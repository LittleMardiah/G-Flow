//go:build integration

// Package wallet_test — integration test withdrawal end-to-end (TD-183).
//
// Menutup jalur DB+service (bukan HTTP handler — sudah di-cover unit):
//  1. RequestWithdrawal happy path (PENDING, saldo belum dipotong).
//  2. RequestWithdrawal insufficient balance → ErrInsufficientBalance.
//  3. RequestWithdrawal wrong user_type → ErrWithdrawalNotEligible.
//  4. ApproveWithdrawal COMPLETED → ledger DEBIT/CREDIT balanced + balance turun.
//  5. ApproveWithdrawal idempotent (2nd call → ErrInvalidStatus, no double debit).
//  6. ApproveWithdrawal REJECTED → saldo tak berubah + no ledger.
//
// Prasyarat: docker-compose (PostgreSQL:15432, Redis:6380) up + migrations.
// Jalankan:
//
//	go test -tags integration ./internal/wallet/ -run TestIntegrationWithdrawal -v
package wallet_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/wallet"
)

// setupWithdrawalEnv membuat driver user (dengan DRIVER wallet bersaldo
// driverBalance) + returns service instance. Redis nil karena idempotency
// L1 tidak di-exercise di test ini (fokus DB + ledger).
func setupWithdrawalEnv(t *testing.T, pool *pgxpool.Pool, driverBalance decimal.Decimal) (*wallet.Service, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	email := "it" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"
	var driverID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, user_type, password_hash, kyc_status, vehicle_plate, license_number)
		VALUES ($1, $2, 'driver', 'x', 'UNVERIFIED', 'B 1234 XYZ', '1234-5678-9012')
		RETURNING id
	`, email, "Integration Driver").Scan(&driverID)
	require.NoError(t, err)

	// CUSTOMER wallet (mirror auth auto-create — tidak dipakai di flow ini,
	// tapi konsisten dengan behavior produksi).
	_, err = pool.Exec(ctx, `INSERT INTO wallets (user_id, wallet_type) VALUES ($1, 'CUSTOMER')`, driverID)
	require.NoError(t, err)

	// DRIVER wallet + saldo.
	var driverWalletID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO wallets (user_id, wallet_type, balance)
		VALUES ($1, 'DRIVER', $2)
		RETURNING id
	`, driverID, driverBalance).Scan(&driverWalletID)
	require.NoError(t, err)

	repo := wallet.NewRepository(pool)
	ledger := wallet.NewLedgerService(pool)
	svc := wallet.NewService(repo, ledger, nil, pool)

	return svc, driverID, driverWalletID
}

func newWithdrawalInput(driverID, walletID uuid.UUID, amount int64) wallet.RequestWithdrawalInput {
	return wallet.RequestWithdrawalInput{
		UserID:            driverID,
		UserType:          "driver",
		WalletID:          walletID,
		Amount:            decimal.NewFromInt(amount),
		BankName:          "BCA",
		BankAccountNumber: "1234567890",
		BankAccountName:   "Integration Driver",
		IdempotencyKey:    uuid.New().String(),
	}
}

func getBalance(t *testing.T, pool *pgxpool.Pool, walletID uuid.UUID) decimal.Decimal {
	t.Helper()
	var bal decimal.Decimal
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT balance FROM wallets WHERE id = $1`, walletID).Scan(&bal))
	return bal
}

func TestIntegrationWithdrawal_RequestHappyPath(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	svc, driverID, walletID := setupWithdrawalEnv(t, pool, decimal.NewFromInt(500000))

	w, err := svc.RequestWithdrawal(ctx, newWithdrawalInput(driverID, walletID, 100000))
	require.NoError(t, err)
	require.Equal(t, "PENDING", w.Status)
	require.Equal(t, driverID, w.UserID)
	require.Equal(t, "BCA", w.BankName)
	require.True(t, w.Amount.Equal(decimal.NewFromInt(100000)))

	// Saldo belum dipotong (hold hanya cek saldo cukup, potong saat approve).
	bal := getBalance(t, pool, walletID)
	require.True(t, bal.Equal(decimal.NewFromInt(500000)), "balance = %v, want 500000", bal)
}

func TestIntegrationWithdrawal_RequestInsufficientBalance(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	svc, driverID, walletID := setupWithdrawalEnv(t, pool, decimal.NewFromInt(50000))

	_, err := svc.RequestWithdrawal(ctx, newWithdrawalInput(driverID, walletID, 200000))
	require.ErrorIs(t, err, wallet.ErrInsufficientBalance)
}

func TestIntegrationWithdrawal_RequestWrongUserType(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	svc, driverID, walletID := setupWithdrawalEnv(t, pool, decimal.NewFromInt(500000))

	in := newWithdrawalInput(driverID, walletID, 100000)
	in.UserType = "customer"
	_, err := svc.RequestWithdrawal(ctx, in)
	require.ErrorIs(t, err, wallet.ErrWithdrawalNotEligible)
}

func TestIntegrationWithdrawal_ApproveCompleted_LedgerBalanced(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	svc, driverID, walletID := setupWithdrawalEnv(t, pool, decimal.NewFromInt(500000))
	amount := decimal.NewFromInt(100000)

	w, err := svc.RequestWithdrawal(ctx, newWithdrawalInput(driverID, walletID, 100000))
	require.NoError(t, err)

	approved, err := svc.ApproveWithdrawal(ctx, wallet.ApproveWithdrawalInput{
		WithdrawalID: w.ID,
		AdminID:      uuid.New(),
		NewStatus:    "COMPLETED",
	})
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", approved.Status)

	// Saldo turun = amount.
	bal := getBalance(t, pool, walletID)
	require.True(t, bal.Equal(decimal.NewFromInt(400000)), "balance = %v, want 400000", bal)

	// Ledger balanced per reference_id.
	var debitSum, creditSum decimal.Decimal
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN entry_type='DEBIT' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN entry_type='CREDIT' THEN amount ELSE 0 END), 0)
		FROM ledger_entries
		WHERE reference_type = 'WITHDRAWAL' AND reference_id = $1 AND is_reversed = FALSE
	`, w.ID).Scan(&debitSum, &creditSum))
	require.True(t, debitSum.Equal(creditSum), "debit=%v credit=%v", debitSum, creditSum)
	require.True(t, debitSum.Equal(amount), "debit=%v want=%v", debitSum, amount)

	// Ledger entry linked ke withdrawal_requests.
	var ledgerID *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ledger_entry_id FROM withdrawal_requests WHERE id = $1`, w.ID).Scan(&ledgerID))
	require.NotNil(t, ledgerID)
}

func TestIntegrationWithdrawal_ApproveIdempotent(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	svc, driverID, walletID := setupWithdrawalEnv(t, pool, decimal.NewFromInt(500000))

	w, err := svc.RequestWithdrawal(ctx, newWithdrawalInput(driverID, walletID, 100000))
	require.NoError(t, err)

	// Approve pertama OK.
	_, err = svc.ApproveWithdrawal(ctx, wallet.ApproveWithdrawalInput{
		WithdrawalID: w.ID,
		AdminID:      uuid.New(),
		NewStatus:    "COMPLETED",
	})
	require.NoError(t, err)

	// Approve kedua → CAS PENDING gagal → ErrInvalidStatus.
	_, err = svc.ApproveWithdrawal(ctx, wallet.ApproveWithdrawalInput{
		WithdrawalID: w.ID,
		AdminID:      uuid.New(),
		NewStatus:    "COMPLETED",
	})
	require.ErrorIs(t, err, wallet.ErrInvalidStatus)

	// Saldo turun 1x saja.
	bal := getBalance(t, pool, walletID)
	require.True(t, bal.Equal(decimal.NewFromInt(400000)), "balance = %v, want 400000", bal)
}

func TestIntegrationWithdrawal_ApproveRejected_NoLedger(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	svc, driverID, walletID := setupWithdrawalEnv(t, pool, decimal.NewFromInt(500000))

	w, err := svc.RequestWithdrawal(ctx, newWithdrawalInput(driverID, walletID, 100000))
	require.NoError(t, err)

	approved, err := svc.ApproveWithdrawal(ctx, wallet.ApproveWithdrawalInput{
		WithdrawalID: w.ID,
		AdminID:      uuid.New(),
		NewStatus:    "REJECTED",
	})
	require.NoError(t, err)
	require.Equal(t, "REJECTED", approved.Status)

	// Saldo tidak berubah.
	bal := getBalance(t, pool, walletID)
	require.True(t, bal.Equal(decimal.NewFromInt(500000)), "balance = %v, want 500000", bal)

	// Tidak ada ledger WITHDRAWAL.
	var cnt int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ledger_entries
		WHERE reference_type = 'WITHDRAWAL' AND reference_id = $1
	`, w.ID).Scan(&cnt))
	require.Equal(t, 0, cnt)
}
