//go:build integration

// Package admin — real-DB integration test untuk repository.
// Prasyarat: PostgreSQL ter-migrate (docker-compose) + DATABASE_URL diset.
// Jalankan dengan:
//
//	DATABASE_URL=... go test -tags integration ./internal/admin/ -run TestIntegrationAdmin_Repository -v -count=1
package admin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/db"
)

// setupAdminPool baca DATABASE_URL, skip kalau kosong (mirror pattern ride).
func setupAdminPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset, melewati integration test")
	}
	pool, err := db.NewDB(db.Config{DatabaseURL: url})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(pool) })
	return pool
}

// TestIntegrationAdmin_Repository_UpdateMerchantStatus_Regression42P08 — REGRESSION test untuk 42P08.
// Insert user (user_type='merchant') → insert food_merchants dengan status='PENDING_VERIFICATION'
// Begin tx + panggil repo.UpdateMerchantStatus(ctx, tx, merchantID, "ACTIVE")
// Assert: no error (42P08 would fail here), verified_at NOT NULL
// Commit + verify dengan SELECT status='ACTIVE'
// Cleanup: DELETE merchant + user
func TestIntegrationAdmin_Repository_UpdateMerchantStatus_Regression42P08(t *testing.T) {
	pool := setupAdminPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Cleanup helper
	cleanup := func(merchantID uuid.UUID, userID uuid.UUID) {
		_, _ = pool.Exec(ctx, `DELETE FROM food_merchants WHERE id = $1`, merchantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	}

	userID := uuid.New()
	email := "reg42p08_merchant_" + userID.String() + "@test.local"

	// Insert user
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, user_type, password_hash) VALUES ($1,$2,$3,'merchant',$4)`,
		userID, email, "Test Merchant", "regression42p08hash",
	)
	require.NoError(t, err)

	merchantID := uuid.New()
	// Insert food_merchants dengan status PENDING_VERIFICATION
	_, err = pool.Exec(ctx,
		`INSERT INTO food_merchants (id, user_id, merchant_name, category, latitude, longitude, address, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING_VERIFICATION')`,
		merchantID, userID, "Test Resto 42P08", "FOOD", -6.2088, 106.8456, "Jl. Test 123",
	)
	require.NoError(t, err)
	defer cleanup(merchantID, userID)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	updatedAt, err := repo.UpdateMerchantStatus(ctx, tx, merchantID, "ACTIVE")
	require.NoError(t, err, "should not error with 42P08; expect no error")
	require.False(t, updatedAt.IsZero())

	// Commit
	require.NoError(t, tx.Commit(ctx))

	// Verify status ACTIVE
	var status string
	var verifiedAt *time.Time
	err = pool.QueryRow(ctx,
		`SELECT status, verified_at FROM food_merchants WHERE id = $1`, merchantID,
	).Scan(&status, &verifiedAt)
	require.NoError(t, err)
	require.Equal(t, "ACTIVE", status)
	require.NotNil(t, verifiedAt, "verified_at should NOT be NULL when status becomes ACTIVE")
}

// TestIntegrationAdmin_Repository_UpdateMerchantStatus_NotFound — non-existing merchant:
// Begin tx + panggil dengan random UUID
// Assert: err == pgx.ErrNoRows (bukan 42P08, bukan nil)
func TestIntegrationAdmin_Repository_UpdateMerchantStatus_NotFound(t *testing.T) {
	pool := setupAdminPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	nonExistent := uuid.New()
	_, err = repo.UpdateMerchantStatus(ctx, tx, nonExistent, "ACTIVE")
	require.Error(t, err)
	require.Equal(t, pgx.ErrNoRows, err, "should return pgx.ErrNoRows for non-existing merchant")

	require.NoError(t, tx.Rollback(ctx))
}

// TestIntegrationAdmin_Repository_UpdateMerchantStatus_RejectDoesNotSetVerifiedAt — guard verified_at:
// Insert merchant ACTIVE (sudah verified_at set)
// Begin tx + panggil dengan "CLOSED"
// Assert: status='CLOSED', verified_at UNCHANGED (bukan di-reset)
func TestIntegrationAdmin_Repository_UpdateMerchantStatus_RejectDoesNotSetVerifiedAt(t *testing.T) {
	pool := setupAdminPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	cleanup := func(merchantID uuid.UUID, userID uuid.UUID) {
		_, _ = pool.Exec(ctx, `DELETE FROM food_merchants WHERE id = $1`, merchantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	}

	userID := uuid.New()
	email := "guard_verified_" + userID.String() + "@test.local"

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, user_type, password_hash) VALUES ($1,$2,$3,'merchant',$4)`,
		userID, email, "Test Merchant 2", "guardhash",
	)
	require.NoError(t, err)

	merchantID := uuid.New()
	// Insert ACTIVE merchant with verified_at set
	_, err = pool.Exec(ctx,
		`INSERT INTO food_merchants (id, user_id, merchant_name, category, latitude, longitude, address, status, verified_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'ACTIVE', NOW(), NOW())`,
		merchantID, userID, "Test Resto Guard", "FOOD", -6.2088, 106.8456, "Jl. Test 456",
	)
	require.NoError(t, err)
	defer cleanup(merchantID, userID)

	// Get initial verified_at
	var initialVerifiedAt time.Time
	err = pool.QueryRow(ctx,
		`SELECT verified_at FROM food_merchants WHERE id = $1`, merchantID,
	).Scan(&initialVerifiedAt)
	require.NoError(t, err)
	require.False(t, initialVerifiedAt.IsZero())

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	_, err = repo.UpdateMerchantStatus(ctx, tx, merchantID, "CLOSED")
	require.NoError(t, err)

	require.NoError(t, tx.Commit(ctx))

	// Verify status CLOSED, verified_at unchanged
	var status string
	var verifiedAt time.Time
	err = pool.QueryRow(ctx,
		`SELECT status, verified_at FROM food_merchants WHERE id = $1`, merchantID,
	).Scan(&status, &verifiedAt)
	require.NoError(t, err)
	require.Equal(t, "CLOSED", status)
	require.Equal(t, initialVerifiedAt.UTC().Truncate(time.Second), verifiedAt.UTC().Truncate(time.Second),
		"verified_at should remain unchanged when status is not ACTIVE")
}
