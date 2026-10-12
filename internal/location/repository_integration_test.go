//go:build integration

package location

// Integration test SetDriverOnlineStatus (TD-188) — real PostgreSQL.
// Prasyarat: DB ter-migrate (docker-compose) + DATABASE_URL diset.
// Jalankan dengan:
//
//	DATABASE_URL=... go test -tags integration ./internal/location/ -run TestIntegrationLocation_SetDriverOnlineStatus -v
//
// Latar belakang: TD-181 membuktikan pgxmock PASS tapi real DB kena 42P08;
// unit test saja tidak cukup. Test ini memvalidasi SQL SetDriverOnlineStatus
// langsung ke DB dev (guard user_type='driver' + status='ACTIVE').

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

// setupLocationPool baca DATABASE_URL, skip kalau kosong (mirror admin/ride).
func setupLocationPool(t *testing.T) *pgxpool.Pool {
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

// insertLocationTestUser menyisipkan user ke tabel users untuk keperluan test
// dan mendaftarkan t.Cleanup DELETE row. Field driver (vehicle_plate,
// license_number) wajib diisi karena CHECK driver_fields_required.
func insertLocationTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userType, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	email := "td188_" + userType + "_" + id.String() + "@test.local"

	var sqlInsert string
	var args []any
	if userType == "driver" {
		sqlInsert = `INSERT INTO users (id, email, name, user_type, status, password_hash, vehicle_plate, license_number)
		             VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
		args = []any{id, email, "Test Driver", userType, status, "td188hash", "B 1234 AB", "1234567890"}
	} else {
		sqlInsert = `INSERT INTO users (id, email, name, user_type, status, password_hash)
		             VALUES ($1,$2,$3,$4,$5,$6)`
		args = []any{id, email, "Test Non-Driver", userType, status, "td188hash"}
	}

	_, err := pool.Exec(ctx, sqlInsert, args...)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id
}

// TestIntegrationLocation_SetDriverOnlineStatus_HappyPath — driver ACTIVE
// di-update is_online=true, last_status_update_at terisi.
func TestIntegrationLocation_SetDriverOnlineStatus_HappyPath(t *testing.T) {
	pool := setupLocationPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	driverID := insertLocationTestUser(t, ctx, pool, "driver", "ACTIVE")

	err := repo.SetDriverOnlineStatus(ctx, driverID, true)
	require.NoError(t, err)

	var isOnline bool
	var lastStatusUpdateAt *time.Time
	err = pool.QueryRow(ctx,
		`SELECT is_online, last_status_update_at FROM users WHERE id = $1`, driverID,
	).Scan(&isOnline, &lastStatusUpdateAt)
	require.NoError(t, err)
	require.True(t, isOnline)
	require.NotNil(t, lastStatusUpdateAt)
	require.False(t, lastStatusUpdateAt.IsZero())
}

// TestIntegrationLocation_SetDriverOnlineStatus_NotFound — id acak, tidak ada
// baris yang terpengaruh -> pgx.ErrNoRows.
func TestIntegrationLocation_SetDriverOnlineStatus_NotFound(t *testing.T) {
	pool := setupLocationPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	err := repo.SetDriverOnlineStatus(ctx, uuid.New(), true)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

// TestIntegrationLocation_SetDriverOnlineStatus_WrongUserType — user customer
// tidak boleh di-update karena guard user_type='driver' -> pgx.ErrNoRows.
func TestIntegrationLocation_SetDriverOnlineStatus_WrongUserType(t *testing.T) {
	pool := setupLocationPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	customerID := insertLocationTestUser(t, ctx, pool, "customer", "ACTIVE")

	err := repo.SetDriverOnlineStatus(ctx, customerID, true)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

// TestIntegrationLocation_SetDriverOnlineStatus_InactiveDriver — driver status
// SUSPENDED tidak boleh di-update karena guard status='ACTIVE' -> pgx.ErrNoRows.
func TestIntegrationLocation_SetDriverOnlineStatus_InactiveDriver(t *testing.T) {
	pool := setupLocationPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	driverID := insertLocationTestUser(t, ctx, pool, "driver", "SUSPENDED")

	err := repo.SetDriverOnlineStatus(ctx, driverID, true)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}
