//go:build integration

package worker

// Integration test untuk auto-cancel RIDE ORDER lewat worker sungguhan
// (TD-158).
//
// KENAPA TEST INI PERLU (dan kenapa bug TD-158 bisa lolos ke produksi):
//   TestIntegrationRide_AutoCancelExpired di internal/ride/integration_test.go
//   memanggil ride.Service.AutoCancelExpiredOrders, BUKAN worker. Service itu
//   memakai ride.Repository.CancelOrder (internal/ride/repository.go:506) yang
//   TIDAK menulis updated_at. Sementara worker memakai
//   worker.Repository.CancelRideOrder (internal/worker/repository.go:241) yang
//   menulis updated_at = NOW(). Saat ride_orders belum punya kolom tersebut,
//   hanya worker yang meledak dengan SQLSTATE 42703 — jadi test service-layer
//   yang ada selalu hijau dan tidak pernah menangkap regresi ini.
//
//   Test di bawah memanggil worker.CancelRideOrders secara langsung terhadap
//   database sungguhan, sehingga setiap kolom yang ditulis worker (termasuk
//   updated_at) ikut tervalidasi.
//
// Prasyarat: DATABASE_URL menunjuk database yang sudah menjalankan seluruh
// migrations/*.up.sql (termasuk 018_add_ride_orders_updated_at.up.sql).
// Kalau DATABASE_URL kosong, test di-skip — sama seperti harness ride.

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/db"
	"github.com/g-flow/g-flow/internal/wallet"
)

// setupWorkerPool membuat pool ke dev DB dan menyetel ulang saldo wallet
// sistem supaya test idempotent (wallet sistem dipakai escrow dan hanya bisa
// bertambah lintas run).
func setupWorkerPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset, melewati integration test worker")
	}
	pool, err := db.NewDB(db.Config{DatabaseURL: url})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(pool) })

	if _, err := pool.Exec(context.Background(),
		`UPDATE wallets SET balance = 0 WHERE user_id IS NULL`); err != nil {
		t.Fatalf("gagal reset saldo wallet sistem: %v", err)
	}
	return pool
}

// newIntegrationWorker menyusun Worker dengan dependency nyata: pgxpool
// (untuk DB), miniredis tidak dipakai di sini karena CancelRideOrders tidak
// menyentuh Redis (lock Redis hanya dipakai di sweepOnce/Run).
func newIntegrationWorker(t *testing.T, pool *pgxpool.Pool) *Worker {
	t.Helper()
	rdb, err := redis.ParseURL("redis://localhost:6380/0")
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	return NewWorker(NewRepository(pool), pool, rd, wallet.NewLedgerService(pool))
}

// seedExpiredRideOrder membuat satu ride order WALLET berstatus
// SEARCHING_DRIVER yang sudah lewat expires_at, lengkap dengan dana escrow
// yang sudah didebet dari wallet customer (skenario nyata: booking WALLET
// menahan fare di SYSTEM_ESCROW sampai driver ditemukan).
func seedExpiredRideOrder(t *testing.T, pool *pgxpool.Pool) (orderID, walletID uuid.UUID, fare decimal.Decimal) {
	t.Helper()
	ctx := context.Background()

	// User + wallet customer.
	email := "td158" + uuid.NewString() + "@test.com"
	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, user_type, status)
		VALUES ($1, 'x', 'TD-158 Customer', 'customer', 'ACTIVE')
		RETURNING id`, email).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO wallets (user_id, wallet_type, balance, status)
		VALUES ($1, 'CUSTOMER', 0, 'ACTIVE') RETURNING id`, userID).Scan(&walletID))

	fare = decimal.NewFromInt(15000)
	orderID = uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO ride_orders
			(id, customer_id, customer_wallet_id, payment_method, status,
			 pickup_lat, pickup_lng, pickup_address,
			 dropoff_lat, dropoff_lng, dropoff_address,
			 estimated_fare, base_fare, per_km_rate, expires_at, created_at)
		VALUES ($1, $2, $3, 'WALLET', 'SEARCHING_DRIVER',
			-6.20000000, 106.81666667, 'Jl. Test Pickup',
			-6.17500000, 106.82722222, 'Jl. Test Dropoff',
			$4, 10000, 4000, NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '20 minutes')`,
		orderID, userID, walletID, fare)
	require.NoError(t, err)

	// Bukau escrow: DEBIT customer, CREDIT SYSTEM_ESCROW (setara Booking).
	escrowID := systemWalletIDOf(t, pool, "SYSTEM_ESCROW")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) // no-op setelah Commit
	require.NoError(t, wallet.NewLedgerService(pool).CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{WalletID: walletID, EntryType: wallet.EntryDebit, Amount: fare,
			ReferenceID: orderID, ReferenceType: "RIDE_ESCROW",
			Description: "TD-158 seed: fare ditahan di escrow"},
		{WalletID: escrowID, EntryType: wallet.EntryCredit, Amount: fare,
			ReferenceID: orderID, ReferenceType: "RIDE_ESCROW",
			Description: "TD-158 seed: escrow menerima fare"},
	}))
	require.NoError(t, tx.Commit(ctx))
	return orderID, walletID, fare
}

func systemWalletIDOf(t *testing.T, pool *pgxpool.Pool, walletType string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM wallets WHERE wallet_type = $1 AND user_id IS NULL`, walletType).Scan(&id))
	return id
}

func walletBalance(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) decimal.Decimal {
	t.Helper()
	var b decimal.Decimal
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT balance FROM wallets WHERE id = $1`, id).Scan(&b))
	return b
}

// TestIntegrationWorker_AutoCancelRideExpired (TD-158) — ride order WALLET yang
// sudah melewati expires_at harus di-cancel jadi CANCELLED/EXPIRED, escrow
// dikembalikan penuh ke wallet customer, updated_at terisi, dan audit event
// tercatat.
func TestIntegrationWorker_AutoCancelRideExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := setupWorkerPool(t)
	orderID, custWallet, fare := seedExpiredRideOrder(t, pool)

	// Sanity: fare ditahan di escrow — saldo customer jadi -fare (debit), dan
	// worker wajib mengembalikannya penuh (jadi 0).
	//
	// Catatan: saldo SYSTEM_ESCROW SENGAJA tidak di-assert secara absolut di
	// sini. Wallet sistem dipakai bersama oleh seluruh integration suite, dan
	// setupWorkerPool/ride setupPool me-reset `wallets.balance` untuk semua
	// user_id IS NULL di awal test. Kalau suite ride dan suite worker berjalan
	// berbarengan, assertion absolut ikut flak. Yang di-assert di sini hanya
	// state milik order test ini (wallet customer + ledger reference_id).
	require.True(t, walletBalance(t, pool, custWallet).Equal(fare.Neg()),
		"fare harus ditahan (debit) dari wallet customer sebelum cancel")

	w := newIntegrationWorker(t, pool)

	// Tanpa updated_at, statement ini mengembalikan error 42703 dan refund
	// ikut ter-rollback — test ini gagal sebelum migration 018.
	//
	// Catatan: DB dev bisa punya ride order SEARCHING_DRIVER kadaluarsa lain
	// dari run sebelumnya, jadi yang diassert adalah ">= 1" (order milik test
	// ini pasti ikut ter-cancel), bukan angka persis.
	cancelled := w.CancelRideOrders(ctx)
	require.GreaterOrEqual(t, cancelled, 1, "order SEARCHING_DRIVER yang expired harus ter-cancel")

	// Order CANCELLED + reason EXPIRED.
	var status, reason string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, cancellation_reason FROM ride_orders WHERE id = $1`, orderID).
		Scan(&status, &reason))
	require.Equal(t, "CANCELLED", status)
	require.Equal(t, "EXPIRED", reason)

	// updated_at terisi — kolom yang jadi akar bug TD-158.
	var updatedAt *interface{}
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT updated_at FROM ride_orders WHERE id = $1`, orderID).Scan(&updatedAt))
	require.NotNil(t, updatedAt, "updated_at harus terisi setelah auto-cancel")

	// Refund penuh: saldo customer balik dari -fare menjadi 0.
	require.True(t, walletBalance(t, pool, custWallet).IsZero(),
		"escrow harus dikembalikan penuh ke customer")

	// Refund untuk order ini menghasilkan tepat 2 entri ledger (DEBIT escrow
	// + CREDIT customer) dengan amount = fare.
	var debits, credits int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE entry_type='DEBIT'),
			count(*) FILTER (WHERE entry_type='CREDIT')
		FROM ledger_entries
		WHERE reference_id = $1 AND reference_type = 'RIDE_REFUND'`, orderID).
		Scan(&debits, &credits))
	require.Equal(t, 1, debits, "tepat 1 DEBIT dari escrow")
	require.Equal(t, 1, credits, "tepat 1 CREDIT ke customer")

	// Audit event EXPIRED tercatat.
	var toStatus, evReason string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT to_status, reason FROM ride_order_events
		WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`, orderID).
		Scan(&toStatus, &evReason))
	require.Equal(t, "CANCELLED", toStatus)
	require.Equal(t, "EXPIRED", evReason)
}

// TestIntegrationWorker_AutoCancelRideExpiredIdempotent (TD-158) — order yang
// SUDAH CANCELLED tidak boleh di-refund dua kali saat worker jalan lagi.
func TestIntegrationWorker_AutoCancelRideExpiredIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := setupWorkerPool(t)
	orderID, custWallet, _ := seedExpiredRideOrder(t, pool)

	w := newIntegrationWorker(t, pool)
	require.GreaterOrEqual(t, w.CancelRideOrders(ctx), 1)
	// Sweep kedua: order milik test ini sudah CANCELLED, jadi tidak lagi
	// memenuhi predicate ExpiredRideOrderIDs dan tidak boleh di-refund ulang.
	require.Equal(t, 0, w.CancelRideOrders(ctx),
		"sweep kedua tidak boleh men-cancel order yang sudah CANCELLED")

	require.True(t, walletBalance(t, pool, custWallet).IsZero(),
		"tidak boleh double refund — saldo harus tetap 0, bukan +fare")

	var refunds int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM ledger_entries
		WHERE reference_id = $1 AND reference_type = 'RIDE_REFUND'`, orderID).Scan(&refunds))
	require.Equal(t, 2, refunds, "tepat 2 entri RIDE_REFUND (1 DEBIT escrow + 1 CREDIT customer)")
}
