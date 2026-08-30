// Package worker — Background Workers (Phase 3, Task 3.7).
//
// Package worker berisi background worker untuk Auto-Cancel Order makanan
// (G-Food) dan kiriman paket (G-Send), sesuai ROADMAP 03 §3.7. Worker berjalan
// sebagai goroutine dari cmd/api/main.go dengan interval 1 menit, memakai
// Redis distributed lock (SET NX) agar aman berjalan di banyak instance
// (single-writer per sweep).
//
// Setiap order yang timeout di-cancel dalam transaksi PostgreSQL terpisah
// dengan pola yang sama seperti ride AutoCancelExpiredOrders (Task 2.7):
//   - Lock baris order (FOR UPDATE … NOWAIT) lalu wallets (ORDER BY id ASC).
//   - Jika payment_method = 'WALLET' dan belum di-refund: reverse escrow
//     (double-entry refund via wallet.LedgerService).
//   - Update status = 'CANCELLED' + cancellation_reason = 'EXPIRED'.
//   - Insert audit event (food_order_events / send_order_events).
//   - Idempoten: guard status (CAS) + flag is_refunded memastikan tidak ada
//     double-refund.
package worker

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RepoDB adalah subset operasi pool yang dipakai Repository untuk query di
// luar transaksi. Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Querier adalah subset operasi yang bisa dipakai dalam transaksi (pgx.Tx)
// ATAU pool. Method write dieksekusi di dalam transaksi agar atomik.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository adalah akses data untuk worker auto-cancel. Query dibuat
// parameterized untuk mencegah SQL injection.
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan koneksi database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// ---- Food orders ----

// ExpiredFoodOrderIDs mengembalikan ID food order berstatus 'CREATED' yang
// sudah melewati batas waktu cancel (created_at < NOW() - 15 menit). Partial
// index idx_food_orders_auto_cancel mempercepat query ini.
func (r *Repository) ExpiredFoodOrderIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id FROM food_orders
		WHERE status = 'CREATED'
		  AND created_at < NOW() - INTERVAL '15 minutes'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LockFoodOrder mengambil + mengunci baris food_orders dengan SELECT ... FOR
// UPDATE NOWAIT. Mengembalikan ErrLockTimeout (SQLSTATE 55P03) jika lock tidak
// tersedia.
func (r *Repository) LockFoodOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*FoodOrder, error) {
	return scanFoodOrderRow(q.QueryRow(ctx, `
		SELECT id, customer_wallet_id, payment_method, status, is_refunded, is_settled, total_amount
		FROM food_orders WHERE id = $1 FOR UPDATE NOWAIT
	`, orderID))
}

// CancelFoodOrder menandai food order CANCELLED + cancellation_reason='EXPIRED'
// + is_refunded secara atomik (CAS) dengan guard status 'CREATED'. Return true
// jika baris berubah. isRefunded dipakai untuk idempotensi refund (di-set TRUE
// hanya saat WALLET berhasil di-refund).
func (r *Repository) CancelFoodOrder(ctx context.Context, q Querier, orderID uuid.UUID, isRefunded bool) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE food_orders
		SET status = 'CANCELLED',
		    cancellation_reason = 'EXPIRED',
		    is_refunded = COALESCE($3, is_refunded),
		    updated_at = NOW()
		WHERE id = $1 AND status = $2
	`, orderID, foodStatusCreated, boolPtr(isRefunded))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// InsertFoodOrderEvent mencatat audit trail transisi status food order.
func (r *Repository) InsertFoodOrderEvent(ctx context.Context, q Querier, e FoodOrderEvent) error {
	_, err := q.Exec(ctx, `
		INSERT INTO food_order_events (order_id, from_status, to_status, reason, triggered_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, e.OrderID, e.FromStatus, e.ToStatus, e.Reason, e.TriggeredBy, e.Metadata)
	return err
}

// SystemWalletID mengambil id wallet sistem (user_id IS NULL) berdasar tipe,
// misal SYSTEM_ESCROW.
func (r *Repository) SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		SELECT id FROM wallets WHERE wallet_type = $1 AND user_id IS NULL
	`, walletType).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrWalletNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ---- Send orders ----

// ExpiredSendOrderIDs mengembalikan ID send order berstatus 'SEARCHING_DRIVER'
// yang sudah melewati batas waktu cancel (created_at < NOW() - 10 menit).
// Partial index idx_send_orders_auto_cancel mempercepat query ini.
func (r *Repository) ExpiredSendOrderIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id FROM send_orders
		WHERE status = 'SEARCHING_DRIVER'
		  AND created_at < NOW() - INTERVAL '10 minutes'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LockSendOrder mengambil + mengunci baris send_orders dengan SELECT ... FOR
// UPDATE NOWAIT. Mengembalikan ErrLockTimeout (SQLSTATE 55P03) jika lock tidak
// tersedia.
func (r *Repository) LockSendOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*SendOrder, error) {
	return scanSendOrderRow(q.QueryRow(ctx, `
		SELECT id, sender_wallet_id, payment_method, status, is_settled, total_fare
		FROM send_orders WHERE id = $1 FOR UPDATE NOWAIT
	`, orderID))
}

// CancelSendOrder menandai send order CANCELLED + cancellation_reason='EXPIRED'
// secara atomik (CAS) dengan guard status 'SEARCHING_DRIVER'. Return true jika
// baris berubah.
func (r *Repository) CancelSendOrder(ctx context.Context, q Querier, orderID uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE send_orders
		SET status = 'CANCELLED',
		    cancellation_reason = 'EXPIRED',
		    updated_at = NOW()
		WHERE id = $1 AND status = $2
	`, orderID, sendStatusSearchingDriver)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// InsertSendOrderEvent mencatat audit trail transisi status send order.
func (r *Repository) InsertSendOrderEvent(ctx context.Context, q Querier, e SendOrderEvent) error {
	_, err := q.Exec(ctx, `
		INSERT INTO send_order_events (order_id, from_status, to_status, reason, triggered_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, e.OrderID, e.FromStatus, e.ToStatus, e.Reason, e.TriggeredBy, e.Metadata)
	return err
}

// ---- scanner helpers ----

func scanFoodOrderRow(row pgx.Row) (*FoodOrder, error) {
	var o FoodOrder
	err := row.Scan(
		&o.ID, &o.CustomerWalletID, &o.PaymentMethod, &o.Status,
		&o.IsRefunded, &o.IsSettled, &o.TotalAmount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFoodOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func scanSendOrderRow(row pgx.Row) (*SendOrder, error) {
	var o SendOrder
	err := row.Scan(
		&o.ID, &o.SenderWalletID, &o.PaymentMethod, &o.Status,
		&o.IsSettled, &o.TotalFare,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSendOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func boolPtr(b bool) *bool { return &b }
