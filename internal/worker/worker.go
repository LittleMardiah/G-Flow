// Package worker — Auto-Cancel Workers untuk G-Food, G-Send & G-Ride
// (Task 3.7 + TD-067).
//
// Lihat dokumentasi package (file types.go / repository.go) untuk arsitektur
// dan alur per-order. Berikut implementasi engine:
//   - Worker.Run(ctx) menjalankan loop ticker 1 menit.
//   - Setiap tick => (1) ambil Redis distributed lock (SET NX), (2) sweep food,
//     (3) sweep send, (4) sweep ride, (5) lepas lock. Jika lock tidak didapat
//     (instance lain sedang berjalan) => skip tick tanpa query DB.
//   - CancelFoodOrders / CancelSendOrders / CancelRideOrders memproses tiap
//     order dalam transaksi terpisah (lock order -> wallets -> cancel -> refund
//     -> audit).
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/g-flow/g-flow/internal/wallet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

// Redis adalah subset operasi Redis yang dipakai Worker untuk distributed
// lock (SET NX + compare-and-del). Dipenuhi oleh *redis.Client (produksi) dan
// miniredis (test).
type Redis interface {
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
}

// DB adalah subset operasi pool untuk memulai transaksi (pgx.Tx) dan
// mengeksekusi DML di luar transaksi (purge idempotency cache). Dipenuhi
// oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Ledger adalah kontrak double-entry ledger yang dibutuhkan Worker.
// Dipenuhi oleh *wallet.LedgerService (internal/wallet/ledger.go).
type Ledger interface {
	CreateLedgerEntries(ctx context.Context, tx pgx.Tx, entries []wallet.LedgerEntry) error
}

// Worker adalah background worker auto-cancel order G-Food & G-Send.
type Worker struct {
	repo   *Repository
	db     DB
	redis  Redis
	ledger Ledger
}

// NewWorker membuat Worker baru dengan dependency injection.
func NewWorker(repo *Repository, db DB, rdb Redis, ledger Ledger) *Worker {
	return &Worker{repo: repo, db: db, redis: rdb, ledger: ledger}
}

// Run menjalankan loop auto-cancel setiap PollInterval sampai ctx selesai.
// Dipanggil sebagai goroutine dari main.go (dengan context cancel untuk
// graceful shutdown).
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	log.Println("worker auto-cancel: loop dimulai (interval 1 menit)")

	// Jalankan sweep pertama segera, tanpa menunggu ticker pertama.
	w.sweepOnce(ctx)

	// Purge idempotency cache kedaluwarsa setiap jam (TD-002).
	lastPurge := time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker auto-cancel: context selesai, hentikan loop")
			return
		case <-ticker.C:
			w.sweepOnce(ctx)
			if time.Since(lastPurge) >= IdempotencyPurgeInterval {
				purged := w.purgeIdempotencyCache(ctx)
				log.Printf("worker purge idempotency: selesai, dihapus=%d", purged)
				lastPurge = time.Now()
			}
		}
	}
}

// sweepOnce menjalankan satu siklus auto-cancel: ambil distributed lock,
// proses food & send, lalu lepas lock.
func (w *Worker) sweepOnce(ctx context.Context) {
	acquired, err := w.acquireLock(ctx)
	if err != nil {
		log.Printf("worker auto-cancel: gagal cek distributed lock: %v", err)
		return
	}
	if !acquired {
		log.Println("worker auto-cancel: distributed lock dipegang instance lain, skip sweep")
		return
	}

	log.Println("worker auto-cancel: distributed lock didapat, mulai sweep")
	foodDone := w.CancelFoodOrders(ctx)
	sendDone := w.CancelSendOrders(ctx)
	rideDone := w.CancelRideOrders(ctx)

	w.releaseLock(ctx)

	log.Printf("worker auto-cancel: sweep selesai (food cancelled=%d, send cancelled=%d, ride cancelled=%d)", foodDone, sendDone, rideDone)
}

// acquireLock mencoba mengambil Redis distributed lock (SET NX) dengan TTL.
// Mengembalikan true jika lock berhasil didapat, false jika sudah dipegang.
func (w *Worker) acquireLock(ctx context.Context) (bool, error) {
	if w.redis == nil {
		// Tanpa Redis, single-instance dianggap pemilik lock (graceful).
		return true, nil
	}
	return w.redis.SetNX(ctx, lockKey, lockOwnerToken, LockTTL).Result()
}

// releaseLock melepas distributed lock HANYA jika masih dimiliki instance ini
// (compare-and-del via Lua). Mencegah instance lain secara tidak sengaja
// menghapus lock milik instance berjalan bila TTL sudah kedaluwarsa.
func (w *Worker) releaseLock(ctx context.Context) {
	if w.redis == nil {
		return
	}
	const compareAndDel = `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	if err := w.redis.Eval(ctx, compareAndDel, []string{lockKey}, lockOwnerToken).Err(); err != nil {
		log.Printf("worker auto-cancel: gagal lepas distributed lock: %v", err)
	}
}

// CancelFoodOrders membatalkan FFood order 'CREATED' yang timeout (>15 menit).
// Mengembalikan jumlah order yang berhasil di-cancel. Idempoten: guard status
// CAS + is_refunded memastikan tidak ada double-refund.
func (w *Worker) CancelFoodOrders(ctx context.Context) int {
	ids, err := w.repo.ExpiredFoodOrderIDs(ctx)
	if err != nil {
		log.Printf("worker auto-cancel: gagal query food orders expired: %v", err)
		return 0
	}

	cancelled := 0
	for _, id := range ids {
		if err := w.cancelOneFoodOrder(ctx, id); err != nil {
			log.Printf("worker auto-cancel: gagal cancel food order %s: %v", id, err)
			continue
		}
		cancelled++
	}
	return cancelled
}

// cancelOneFoodOrder memproses satu food order dalam transaksi terpisah.
func (w *Worker) cancelOneFoodOrder(ctx context.Context, id uuid.UUID) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return err
	}

	// Lock baris order (FOR UPDATE NOWAIT) lalu guard status.
	locked, err := w.repo.LockFoodOrder(ctx, tx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return ErrLockTimeout
		}
		return err
	}
	if locked.Status != foodStatusCreated {
		return ErrInvalidTransition
	}

	// Refund escrow hanya untuk payment WALLET, belum settled, dan belum
	// di-refund (idempoten).
	refunded := false
	if locked.PaymentMethod == paymentMethodWallet && !locked.IsSettled && !locked.IsRefunded {
		if err := w.refundFoodEscrow(ctx, tx, locked); err != nil {
			return err
		}
		refunded = true
	}

	// CAS cancel: guard status 'CREATED', hapus milik worker bila refund.
	ok, err := w.repo.CancelFoodOrder(ctx, tx, id, refunded)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidTransition
	}

	// Audit event (reason EXPIRED + metadata).
	if err := w.repo.InsertFoodOrderEvent(ctx, tx, FoodOrderEvent{
		OrderID:    id,
		FromStatus: strPtr(locked.Status),
		ToStatus:   foodStatusCancelled,
		Reason:     strPtr(cancellationReasonExpired),
		Metadata:   jsonMetadata(cancellationReasonExpired),
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// refundFoodEscrow mengembalikan dana escrow penuh (FOOD_REFUND) ke customer
// wallet untuk food order WALLET yang di-cancel. Lock wallets ORDER BY id ASC
// (deadlock-free, sesuai Mandat Lock Hierarchy).
func (w *Worker) refundFoodEscrow(ctx context.Context, tx pgx.Tx, order *FoodOrder) error {
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	escrowID, err := w.repo.SystemWalletID(ctx, tx, "SYSTEM_ESCROW")
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID); err != nil {
		return err
	}
	return w.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        order.TotalAmount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeFoodRefund,
			Description:   "FOOD_REFUND - escrow release to customer (auto-cancel EXPIRED)",
		},
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        order.TotalAmount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeFoodRefund,
			Description:   "FOOD_REFUND - full refund customer wallet (auto-cancel EXPIRED)",
		},
	})
}

// CancelSendOrders membatalkan send order 'SEARCHING_DRIVER' yang timeout
// (>10 menit). Mengembalikan jumlah order yang berhasil di-cancel. Idempoten:
// guard status CAS mencegah double-refund.
func (w *Worker) CancelSendOrders(ctx context.Context) int {
	ids, err := w.repo.ExpiredSendOrderIDs(ctx)
	if err != nil {
		log.Printf("worker auto-cancel: gagal query send orders expired: %v", err)
		return 0
	}

	cancelled := 0
	for _, id := range ids {
		if err := w.cancelOneSendOrder(ctx, id); err != nil {
			log.Printf("worker auto-cancel: gagal cancel send order %s: %v", id, err)
			continue
		}
		cancelled++
	}
	return cancelled
}

// cancelOneSendOrder memproses satu send order dalam transaksi terpisah.
func (w *Worker) cancelOneSendOrder(ctx context.Context, id uuid.UUID) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return err
	}

	// Lock baris order (FOR UPDATE NOWAIT) lalu guard status.
	locked, err := w.repo.LockSendOrder(ctx, tx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return ErrLockTimeout
		}
		return err
	}
	if locked.Status != sendStatusSearchingDriver {
		return ErrInvalidTransition
	}

	// Refund escrow hanya untuk payment WALLET dan belum settled.
	if locked.PaymentMethod == paymentMethodWallet && !locked.IsSettled {
		if err := w.refundSendEscrow(ctx, tx, locked); err != nil {
			return err
		}
	}

	// CAS cancel: guard status 'SEARCHING_DRIVER'.
	ok, err := w.repo.CancelSendOrder(ctx, tx, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidTransition
	}

	// Audit event (reason EXPIRED + metadata).
	if err := w.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
		OrderID:    id,
		FromStatus: strPtr(locked.Status),
		ToStatus:   sendStatusCancelled,
		Reason:     strPtr(cancellationReasonExpired),
		Metadata:   jsonMetadata(cancellationReasonExpired),
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// refundSendEscrow mengembalikan dana escrow penuh (SEND_REFUND) ke sender
// wallet untuk send order WALLET yang di-cancel. Lock wallets ORDER BY id ASC
// (deadlock-free).
func (w *Worker) refundSendEscrow(ctx context.Context, tx pgx.Tx, order *SendOrder) error {
	if order.SenderWalletID == nil {
		return ErrWalletNotFound
	}
	escrowID, err := w.repo.SystemWalletID(ctx, tx, "SYSTEM_ESCROW")
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, *order.SenderWalletID, escrowID); err != nil {
		return err
	}
	return w.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        order.TotalFare,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeSendRefund,
			Description:   "SEND_REFUND - escrow release to sender (auto-cancel EXPIRED)",
		},
		{
			WalletID:      *order.SenderWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        order.TotalFare,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeSendRefund,
			Description:   "SEND_REFUND - full refund sender wallet (auto-cancel EXPIRED)",
		},
	})
}

// CancelRideOrders membatalkan ride order 'SEARCHING_DRIVER' yang sudah
// melewati expires_at (TTL 15 menit, TD-067). Mengembalikan jumlah order yang
// berhasil di-cancel. Idempoten: guard status CAS mencegah double-refund.
func (w *Worker) CancelRideOrders(ctx context.Context) int {
	ids, err := w.repo.ExpiredRideOrderIDs(ctx)
	if err != nil {
		log.Printf("worker auto-cancel: gagal query ride orders expired: %v", err)
		return 0
	}

	cancelled := 0
	for _, id := range ids {
		if err := w.cancelOneRideOrder(ctx, id); err != nil {
			log.Printf("worker auto-cancel: gagal cancel ride order %s: %v", id, err)
			continue
		}
		cancelled++
	}
	return cancelled
}

// cancelOneRideOrder memproses satu ride order dalam transaksi terpisah.
func (w *Worker) cancelOneRideOrder(ctx context.Context, id uuid.UUID) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return err
	}

	// Lock baris order (FOR UPDATE NOWAIT) lalu guard status.
	locked, err := w.repo.LockRideOrder(ctx, tx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return ErrLockTimeout
		}
		return err
	}
	if locked.Status != rideStatusSearchingDriver {
		return ErrInvalidTransition
	}

	// Refund escrow hanya untuk payment WALLET.
	if locked.PaymentMethod == paymentMethodWallet {
		if err := w.refundRideEscrow(ctx, tx, locked); err != nil {
			return err
		}
	}

	// CAS cancel: guard status 'SEARCHING_DRIVER'.
	ok, err := w.repo.CancelRideOrder(ctx, tx, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidTransition
	}

	// Audit event (reason EXPIRED + metadata).
	if err := w.repo.InsertRideOrderEvent(ctx, tx, RideOrderEvent{
		OrderID:    id,
		FromStatus: strPtr(locked.Status),
		ToStatus:   rideStatusCancelled,
		Reason:     strPtr(cancellationReasonExpired),
		Metadata:   jsonMetadata(cancellationReasonExpired),
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// refundRideEscrow mengembalikan dana escrow penuh (RIDE_REFUND) ke customer
// wallet untuk ride order WALLET yang di-cancel. Lock wallets ORDER BY id ASC
// (deadlock-free, sesuai Mandat Lock Hierarchy).
func (w *Worker) refundRideEscrow(ctx context.Context, tx pgx.Tx, order *RideOrder) error {
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	escrowID, err := w.repo.SystemWalletID(ctx, tx, "SYSTEM_ESCROW")
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID); err != nil {
		return err
	}
	return w.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        order.EstimatedFare,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideRefund,
			Description:   "RIDE_REFUND - escrow release to customer (auto-cancel EXPIRED)",
		},
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        order.EstimatedFare,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideRefund,
			Description:   "RIDE_REFUND - full refund customer wallet (auto-cancel EXPIRED)",
		},
	})
}

// purgeIdempotencyCache menghapus baris idempotency_cache yang sudah kedaluwarsa
// (expires_at < NOW()). Membatasi 1000 baris per eksekusi agar tidak memblokir
// DB terlalu lama; dipanggil berkala setiap jam sebagai background task.
// PostgreSQL tidak mendukung LIMIT pada DELETE langsung, sehingga batas
// diterapkan lewat subquery ctid. Mengembalikan jumlah baris yang dihapus.
func (w *Worker) purgeIdempotencyCache(ctx context.Context) int {
	tag, err := w.db.Exec(ctx, `
		DELETE FROM idempotency_cache
		WHERE ctid IN (
			SELECT ctid FROM idempotency_cache
			WHERE expires_at < NOW()
			LIMIT 1000
		)
	`)
	if err != nil {
		log.Printf("worker purge idempotency: gagal menghapus cache kedaluwarsa: %v", err)
		return 0
	}
	return int(tag.RowsAffected())
}

// 

// lockWalletsAsc mengunci sejumlah wallet dengan urutan id menaik dalam satu
// statement. ORDER BY id ASC menghindari deadlock antar transaksi.
func lockWalletsAsc(ctx context.Context, tx pgx.Tx, walletIDs ...uuid.UUID) error {
	ids := make([]uuid.UUID, len(walletIDs))
	copy(ids, walletIDs)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	_, err := tx.Exec(ctx, `
		SELECT id FROM wallets WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE
	`, ids)
	return err
}

// jsonMetadata membangun metadata audit event berisi reason EXPIRED.
func jsonMetadata(reason string) []byte {
	md, _ := json.Marshal(map[string]string{"reason": reason})
	return md
}

// strPtr mengembalikan pointer string untuk kolom nullable.
func strPtr(s string) *string { return &s }
