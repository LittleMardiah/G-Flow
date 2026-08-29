//go:build integration

// Package wallet_test — integration test tambahan terhadap DB nyata untuk
// menutup fungsi yang tidak terjangkau lewat alur HTTP (repository CRUD,
// ledger reversal, service GetBalance/webhook). Tanpa mock — langsung memakai
// pool PostgreSQL.
package wallet_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/wallet"
)

// TestIntegration_RepoCreateAndRead menutup Repository.Create, GetByID,
// GetByUserIDAndType, GetWalletsByUserID, GetBalance, UpdateBalance,
// UpdateStatus.
func TestIntegration_RepoCreateAndRead(t *testing.T) {
	ctx := context.Background()
	pool := setupPool(t)
	repo := wallet.NewRepository(pool)

	userID, _ := newIntegrationUser(t, ctx, pool, "customer")

	// CUSTOMER wallet sudah dibuat oleh newIntegrationUser.
	cust, err := repo.GetByUserIDAndType(ctx, userID, "CUSTOMER")
	require.NoError(t, err)

	// Repo.Create: buat DRIVER wallet untuk user yang sama.
	created, err := repo.Create(ctx, userID, "DRIVER")
	require.NoError(t, err)
	assert.Equal(t, "DRIVER", created.Type)
	assert.True(t, created.Balance.IsZero())

	// GetByID & GetWalletsByUserID.
	byID, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byID.ID)

	all, err := repo.GetWalletsByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	types := map[string]bool{}
	for _, w := range all {
		types[w.Type] = true
	}
	assert.True(t, types["CUSTOMER"])
	assert.True(t, types["DRIVER"])

	// UpdateBalance (CREDIT) & GetBalance.
	err = repo.UpdateBalance(ctx, cust.ID, decimal.NewFromInt(25000))
	require.NoError(t, err)
	bal, err := repo.GetBalance(ctx, cust.ID)
	require.NoError(t, err)
	assert.True(t, bal.Equal(decimal.NewFromInt(25000)))

	// UpdateStatus.
	err = repo.UpdateStatus(ctx, cust.ID, "FROZEN")
	require.NoError(t, err)
	frozen, err := repo.GetByID(ctx, cust.ID)
	require.NoError(t, err)
	assert.Equal(t, "FROZEN", frozen.Status)
}

// TestIntegration_RepoNotFound memastikan GET yang tidak ada mengembalikan error.
func TestIntegration_RepoNotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupPool(t)
	repo := wallet.NewRepository(pool)

	rnd := uuid.New()
	_, err := repo.GetByID(ctx, rnd)
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)

	_, err = repo.GetByUserIDAndType(ctx, rnd, "CUSTOMER")
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)

	_, err = repo.GetBalance(ctx, rnd)
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)

	// GetWalletsByUserID untuk user tanpa wallet -> slice kosong, bukan error.
	ws, err := repo.GetWalletsByUserID(ctx, rnd)
	require.NoError(t, err)
	assert.Empty(t, ws)
}

// TestIntegration_ReverseLedger melakukan transfer lalu membalikkannya.
func TestIntegration_ReverseLedger(t *testing.T) {
	ctx := context.Background()
	pool := setupPool(t)

	fromUserID, fromWalletID := newIntegrationUser(t, ctx, pool, "customer")
	_, toWalletID := newIntegrationUser(t, ctx, pool, "customer")

	seed := decimal.NewFromInt(100000)
	_, err := pool.Exec(ctx, `UPDATE wallets SET balance = $1 WHERE id = $2`, seed, fromWalletID)
	require.NoError(t, err)

	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	svc := wallet.NewService(repo, ledger, nil, pool)

	amount := decimal.NewFromInt(30000)
	resp, err := svc.Transfer(ctx, wallet.TransferRequest{
		UserID:         fromUserID,
		FromWalletID:   fromWalletID,
		ToWalletID:     toWalletID,
		Amount:         amount,
		IdempotencyKey: uuid.New().String(),
		Description:    "reverse-me",
	})
	require.NoError(t, err)

	afterTransfer, err := repo.GetBalance(ctx, fromWalletID)
	require.NoError(t, err)
	assert.True(t, afterTransfer.Equal(decimal.NewFromInt(70000)), "from setelah transfer=%v", afterTransfer)

	// Balikkan seluruh transaksi ber-reference transferID.
	err = ledger.ReverseLedger(ctx, resp.TransferID)
	require.NoError(t, err)

	fromBal, err := repo.GetBalance(ctx, fromWalletID)
	require.NoError(t, err)
	assert.True(t, fromBal.Equal(decimal.NewFromInt(100000)), "from setelah reversal=%v", fromBal)

	toBal, err := repo.GetBalance(ctx, toWalletID)
	require.NoError(t, err)
	assert.True(t, toBal.IsZero(), "to setelah reversal=%v", toBal)
}

// TestIntegration_ReverseLedger_NotFound memastikan reversal tanpa baris
// mengembalikan ErrWalletNotFound.
func TestIntegration_ReverseLedger_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupPool(t)
	ledger := wallet.NewLedgerService(pool)

	err := ledger.ReverseLedger(ctx, uuid.New())
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

// TestIntegration_ServiceGetBalanceAndWebhook menutup Service.GetBalance dan
// ProcessTopUpWebhook (path idempotent: tidak ada row PENDING -> nil).
func TestIntegration_ServiceGetBalanceAndWebhook(t *testing.T) {
	ctx := context.Background()
	pool := setupPool(t)

	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	svc := wallet.NewService(repo, ledger, nil, pool)

	_, wal := newIntegrationUser(t, ctx, pool, "customer")
	bal, err := svc.GetBalance(ctx, wal)
	require.NoError(t, err)
	assert.True(t, bal.IsZero())

	// Webhook dengan reference acak: tidak ada PENDING -> nil (idempotent).
	err = svc.ProcessTopUpWebhook(ctx, uuid.New())
	require.NoError(t, err)
}
