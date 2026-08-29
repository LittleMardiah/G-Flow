//go:build integration

// Package integration test untuk alur wallet end-to-end terhadap database
// nyata (PostgreSQL). File ini TIDAK ikut kompilasi pada `go test ./...`
// biasa (build tag `integration`); jalankan dengan:
//
//	go test -tags integration ./internal/wallet/ -run Integration -v
//
// Prasyarat: DATABASE_URL menunjuk ke PostgreSQL yang sudah di-migrate
// (jalankan docker-compose + scripts/migrate.sh terlebih dahulu).
package wallet_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/g-flow/g-flow/internal/db"
	"github.com/g-flow/g-flow/internal/wallet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset, melewati integration test")
	}
	pool, err := db.NewDB(db.Config{DatabaseURL: url})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(pool) })

	// Reset saldo wallet sistem ke 0 agar test idempotent. Top-up mendebet
	// SYSTEM_BANK_GATEWAY; tanpa reset saldo negatif akan menumpuk lintas run
	// dan melanggar CHECK balance_non_negative (>= -1jt).
	if _, err := pool.Exec(context.Background(),
		`UPDATE wallets SET balance = 0 WHERE user_id IS NULL`); err != nil {
		t.Fatalf("gagal reset saldo wallet sistem: %v", err)
	}

	return pool
}

func newIntegrationUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userType string) (userID uuid.UUID, walletID uuid.UUID) {
	t.Helper()
	email := "it" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, user_type, password_hash, kyc_status)
		VALUES ($1, $2, $3, $4, 'UNVERIFIED')
		RETURNING id
	`, email, "Integration User", userType, "x").Scan(&userID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, `
		INSERT INTO wallets (user_id, wallet_type)
		VALUES ($1, 'CUSTOMER')
		RETURNING id
	`, userID).Scan(&walletID)
	require.NoError(t, err)

	return userID, walletID
}

func TestIntegrationTopUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool := setupPool(t)

	userID, walletID := newIntegrationUser(t, ctx, pool, "customer")

	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	svc := wallet.NewService(repo, ledger, nil, pool)

	amount := decimal.NewFromInt(500000)
	resp, err := svc.TopUp(ctx, wallet.TopUpRequest{
		UserID:         userID,
		Amount:         amount,
		IdempotencyKey: uuid.New().String(),
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", resp.Status)
	assert.Equal(t, walletID, resp.WalletID)

	balance, err := repo.GetBalance(ctx, walletID)
	require.NoError(t, err)
	assert.True(t, balance.Equal(amount), "balance = %v, want %v", balance, amount)
}

func TestIntegrationTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool := setupPool(t)

	fromUserID, fromWalletID := newIntegrationUser(t, ctx, pool, "customer")
	_, toWalletID := newIntegrationUser(t, ctx, pool, "customer")

	// Isi saldo wallet sumber.
	seed := decimal.NewFromInt(100000)
	_, err := pool.Exec(ctx,
		`UPDATE wallets SET balance = $1 WHERE id = $2`, seed, fromWalletID)
	require.NoError(t, err)

	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	svc := wallet.NewService(repo, ledger, nil, pool)

	transferAmount := decimal.NewFromInt(40000)
	resp, err := svc.Transfer(ctx, wallet.TransferRequest{
		UserID:         fromUserID, // pemilik wallet (dari JWT claim)
		FromWalletID:   fromWalletID,
		ToWalletID:     toWalletID,
		Amount:         transferAmount,
		IdempotencyKey: uuid.New().String(),
		Description:    "integration-transfer",
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, resp.TransferID)

	fromBalance, err := repo.GetBalance(ctx, fromWalletID)
	require.NoError(t, err)
	toBalance, err := repo.GetBalance(ctx, toWalletID)
	require.NoError(t, err)

	// NB: balance di-tegaskan via saldo (trigger DB mengupdate wallets.balance).
	fmt.Printf("fromBalance=%v toBalance=%v\n", fromBalance, toBalance)
	assert.True(t, fromBalance.Equal(decimal.NewFromInt(60000)), "from=%v", fromBalance)
	assert.True(t, toBalance.Equal(decimal.NewFromInt(40000)), "to=%v", toBalance)
}
