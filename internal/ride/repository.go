// Package ride berisi akses data (repository) untuk modul G-Ride (Phase 2).
// Repository hanya bertanggung jawab mengakses database; business logic
// ditangani di Service Layer (F004). Semua query menggunakan parameterized
// statement untuk mencegah SQL injection.
//
// Rujukan skema: MIGRATION 004_ride_orders.up.sql (LOCKED) — ride_orders,
// ride_order_events, driver_locations.
package ride

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// Error definitions tingkat package.
var (
	ErrCustomerNotFound = errors.New("customer not found")
	ErrWalletNotFound   = errors.New("wallet not found")
	ErrOrderNotFound    = errors.New("ride order not found")
	ErrDriverNotFound   = errors.New("driver not found")
)

// RepoDB adalah subset operasi pool yang dipakai Repository untuk query
// di luar transaksi. Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Querier adalah subset operasi yang bisa dipakai dalam transaksi (pgx.Tx)
// ATAU pool. Method write/read yang dieksekusi di dalam transaksi booking
// menerima Querier agar tetap satu koneksi & satu unit kerja atomik.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Customer adalah representasi baris users yang dibutuhkan untuk validasi
// booking (user_type, status, dan overdue_debt sebagai Debt Gate Universal).
type Customer struct {
	ID          uuid.UUID
	UserType    string
	Status      string
	OverdueDebt decimal.Decimal
}

// RideWallet adalah representasi subset baris wallets yang dipakai booking
// (id, balance, status). Hanya berisi field yang relevan untuk escrow.
type RideWallet struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Type    string
	Balance decimal.Decimal
	Status  string
}

// RideOrder adalah representasi baris tabel ride_orders (MIGRATION 004).
// Kolom nullable direpresentasikan sebagai pointer; nil berarti SQL NULL.
type RideOrder struct {
	ID                 uuid.UUID
	CustomerID         uuid.UUID
	DriverID           *uuid.UUID
	CustomerWalletID   *uuid.UUID
	DriverWalletID     *uuid.UUID
	PickupLat          float64
	PickupLng          float64
	PickupAddress      string
	DropoffLat         float64
	DropoffLng         float64
	DropoffAddress     string
	DistanceKm         decimal.Decimal
	BaseFare           decimal.Decimal
	PerKmRate          decimal.Decimal
	EstimatedFare      decimal.Decimal
	ActualFare         *decimal.Decimal
	SurgeMultiplier    *decimal.Decimal
	TollFee            *decimal.Decimal
	CancellationFee    *decimal.Decimal
	DiscountAmount     *decimal.Decimal
	VoucherID          *uuid.UUID
	PaymentMethod      string
	PlatformCommission *decimal.Decimal
	DriverEarning      *decimal.Decimal
	Status             string
	CancellationReason *string
	CreatedAt          time.Time
	ExpiresAt          *time.Time
	AssignedAt         *time.Time
	PickupAt           *time.Time
	CompletedAt        *time.Time
	SettledAt          *time.Time
	IsSettled          bool
	SettlementNotes    *string
}

// RideOrderEvent adalah representasi baris tabel ride_order_events (audit
// trail transisi status).
type RideOrderEvent struct {
	ID          uuid.UUID
	OrderID     uuid.UUID
	FromStatus  *string
	ToStatus    string
	Reason      *string
	TriggeredBy *uuid.UUID
	Metadata    []byte
}

// Driver adalah representasi subset baris users yang dibutuhkan untuk validasi
// accept order (user_type, status, working_status, min_balance_threshold).
type Driver struct {
	ID                  uuid.UUID
	UserType            string
	Status              string
	WorkingStatus       string
	MinBalanceThreshold decimal.Decimal
}

// Repository adalah akses data untuk tabel ride_orders & tabel pendukung.
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan koneksi database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// GetCustomer mengambil data customer untuk validasi booking.
// Mengembalikan ErrCustomerNotFound jika user tidak ada.
func (r *Repository) GetCustomer(ctx context.Context, userID uuid.UUID) (*Customer, error) {
	var c Customer
	err := r.db.QueryRow(ctx, `
		SELECT id, user_type, status, COALESCE(overdue_debt, 0)
		FROM users
		WHERE id = $1
	`, userID).Scan(&c.ID, &c.UserType, &c.Status, &c.OverdueDebt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCustomerNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetWalletByUserAndType mengambil wallet spesifik user berdasarkan tipe.
// Mengembalikan ErrWalletNotFound jika wallet tidak ada.
func (r *Repository) GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*RideWallet, error) {
	var w RideWallet
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, wallet_type, balance, status
		FROM wallets
		WHERE user_id = $1 AND wallet_type = $2
	`, userID, walletType).Scan(&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// InsertOrder membuat baris ride_orders berstatus CREATED. Dipanggil di dalam
// transaksi booking sehingga order + escrow bersifat atomik.
func (r *Repository) InsertOrder(ctx context.Context, q Querier, o *RideOrder) error {
	_, err := q.Exec(ctx, `
		INSERT INTO ride_orders (
			id, customer_id, customer_wallet_id,
			pickup_lat, pickup_lng, pickup_address,
			dropoff_lat, dropoff_lng, dropoff_address,
			distance_km, base_fare, per_km_rate, estimated_fare,
			payment_method, status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`,
		o.ID,
		o.CustomerID,
		o.CustomerWalletID,
		o.PickupLat,
		o.PickupLng,
		o.PickupAddress,
		o.DropoffLat,
		o.DropoffLng,
		o.DropoffAddress,
		o.DistanceKm,
		o.BaseFare,
		o.PerKmRate,
		o.EstimatedFare,
		o.PaymentMethod,
		o.Status,
		o.ExpiresAt,
	)
	return err
}

// TransitionStatus mengubah status order secara atomik (CAS) dengan guard
// WHERE status = $from untuk mencegah transisi status yang keluar urutan.
// Return true jika baris berubah.
func (r *Repository) TransitionStatus(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, toStatus string) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE ride_orders
		SET status = $1
		WHERE id = $2 AND status = $3
	`, toStatus, orderID, fromStatus)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// InsertEvent mencatat audit trail transisi status (ride_order_events).
func (r *Repository) InsertEvent(ctx context.Context, q Querier, e RideOrderEvent) error {
	_, err := q.Exec(ctx, `
		INSERT INTO ride_order_events (order_id, from_status, to_status, reason, triggered_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, e.OrderID, e.FromStatus, e.ToStatus, e.Reason, e.TriggeredBy, e.Metadata)
	return err
}

// SystemWalletID mengambil id wallet sistem (user_id IS NULL) berdasar tipe,
// misal SYSTEM_ESCROW atau SYSTEM_PLATFORM. Mengembalikan ErrWalletNotFound
// jika wallet sistem belum di-seed.
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

// rideOrderColumns daftar kolom ride_orders untuk SELECT lengkap. Dipakai
// bersama oleh GetOrderByID dan LockOrderForUpdate agar tetap satu sumber.
const rideOrderColumns = `id, customer_id, driver_id,
	customer_wallet_id, driver_wallet_id,
	pickup_lat, pickup_lng, pickup_address,
	dropoff_lat, dropoff_lng, dropoff_address,
	distance_km, base_fare, per_km_rate, estimated_fare, actual_fare,
	surge_multiplier, toll_fee, cancellation_fee, discount_amount, voucher_id,
	payment_method, platform_commission, driver_earning,
	status, cancellation_reason,
	created_at, expires_at, assigned_at, pickup_at, completed_at, settled_at,
	is_settled, settlement_notes`

// scanOrderRow memindahkan satu baris ride_orders ke *RideOrder.
// Mengembalikan ErrOrderNotFound jika tidak ada baris.
func scanOrderRow(row pgx.Row) (*RideOrder, error) {
	var o RideOrder
	err := row.Scan(
		&o.ID,
		&o.CustomerID, &o.DriverID,
		&o.CustomerWalletID, &o.DriverWalletID,
		&o.PickupLat, &o.PickupLng, &o.PickupAddress,
		&o.DropoffLat, &o.DropoffLng, &o.DropoffAddress,
		&o.DistanceKm, &o.BaseFare, &o.PerKmRate, &o.EstimatedFare, &o.ActualFare,
		&o.SurgeMultiplier, &o.TollFee, &o.CancellationFee, &o.DiscountAmount, &o.VoucherID,
		&o.PaymentMethod, &o.PlatformCommission, &o.DriverEarning,
		&o.Status, &o.CancellationReason,
		&o.CreatedAt, &o.ExpiresAt, &o.AssignedAt, &o.PickupAt, &o.CompletedAt, &o.SettledAt,
		&o.IsSettled, &o.SettlementNotes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// GetOrderByID mengambil order lengkap berdasarkan id.
// Mengembalikan ErrOrderNotFound jika tidak ada.
func (r *Repository) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*RideOrder, error) {
	return scanOrderRow(r.db.QueryRow(ctx, `SELECT `+rideOrderColumns+` FROM ride_orders WHERE id = $1`, orderID))
}

// LockOrderForUpdate mengambil + mengunci baris ride_orders dengan
// SELECT ... FOR UPDATE (tanpa guard status; pemanggil yang memvalidasi
// transisi). Dipakai oleh UpdateRideStatus, CancelRide, Auto-Cancel Worker
// dan Settlement agar dua transisi bersamaan tidak saling menimpa.
// Mengembalikan ErrOrderNotFound jika order tidak ada.
func (r *Repository) LockOrderForUpdate(ctx context.Context, q Querier, orderID uuid.UUID) (*RideOrder, error) {
	return scanOrderRow(q.QueryRow(ctx, `SELECT `+rideOrderColumns+` FROM ride_orders WHERE id = $1 FOR UPDATE`, orderID))
}

// GetExpiredSearchingOrders mengembalikan ID order berstatus SEARCHING_DRIVER
// yang sudah melewati expires_at (dipakai Auto-Cancel Worker). Partial index
// idx_ride_expires mempercepat pencarian ini.
func (r *Repository) GetExpiredSearchingOrders(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id FROM ride_orders
		WHERE status = 'SEARCHING_DRIVER'
		  AND expires_at IS NOT NULL
		  AND expires_at < NOW()
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

// CancelOrder menandai order CANCELLED + cancellation_reason secara atomik
// (CAS) dengan guard status. Return true jika baris berubah.
func (r *Repository) CancelOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, reason string) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE ride_orders
		SET status = 'CANCELLED', cancellation_reason = $3
		WHERE id = $1 AND status = $2
	`, orderID, fromStatus, reason)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CompleteOrder menandai order COMPLETED sekaligus mengisi actual_fare,
// driver_earning dan platform_commission (CAS dengan guard status).
func (r *Repository) CompleteOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus string,
	actualFare, driverEarning, platformCommission decimal.Decimal) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE ride_orders
		SET status = 'COMPLETED',
		    actual_fare = $3, driver_earning = $4, platform_commission = $5,
		    completed_at = NOW()
		WHERE id = $1 AND status = $2
	`, orderID, fromStatus, actualFare, driverEarning, platformCommission)
	return tag.RowsAffected() > 0, err
}

// MarkSettled menandai order SETTLED + is_settled (dipakai setelat settlement
// ledger berhasil).
func (r *Repository) MarkSettled(ctx context.Context, q Querier, orderID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE ride_orders
		SET status = 'SETTLED', is_settled = TRUE, settled_at = NOW()
		WHERE id = $1 AND status = 'COMPLETED'
	`, orderID)
	return err
}

// ResetDriverIdle menyetel working_status driver kembali ke IDLE (habis
// selesai bertugas atau cancel).
func (r *Repository) ResetDriverIdle(ctx context.Context, q Querier, driverID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE users
		SET working_status = 'IDLE', last_status_update_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, driverID)
	return err
}

// MarkDriverSuspended menyetel status driver menjadi SUSPENDED (dipakai saat
// saldo driver melewati ceiling negatif setelah CASH settlement).
func (r *Repository) MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE users
		SET status = 'SUSPENDED', working_status = 'IDLE',
		    last_status_update_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, driverID)
	return err
}

// GetDriver mengambil data driver untuk validasi accept order.
// Mengembalikan ErrDriverNotFound jika user tidak ada.
func (r *Repository) GetDriver(ctx context.Context, driverID uuid.UUID) (*Driver, error) {
	var d Driver
	err := r.db.QueryRow(ctx, `
		SELECT id, user_type, status, working_status, COALESCE(min_balance_threshold, 0)
		FROM users
		WHERE id = $1
	`, driverID).Scan(&d.ID, &d.UserType, &d.Status, &d.WorkingStatus, &d.MinBalanceThreshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDriverNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// GetDriverBalance mengambil saldo wallet driver (wallet_type='DRIVER').
// Mengembalikan ErrWalletNotFound jika wallet driver belum ada.
func (r *Repository) GetDriverBalance(ctx context.Context, driverID uuid.UUID) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := r.db.QueryRow(ctx, `
		SELECT balance FROM wallets
		WHERE user_id = $1 AND wallet_type = 'DRIVER'
	`, driverID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, ErrWalletNotFound
	}
	if err != nil {
		return decimal.Zero, err
	}
	return balance, nil
}

// LockDriverUserForAccept mengunci baris users driver dengan SELECT FOR UPDATE
// NOWAIT (ROADMAP 02 §2.3 Locking Order Standard: users → ride_orders),
// sekaligus membaca working_status & min_balance_threshold di bawah lock.
// Guard user_type='driver' AND status='ACTIVE' menolak non-driver atau driver
// yang tidak aktif. Mengembalikan ErrDriverNotFound jika baris tidak ada, atau
// pgconn.PgError (SQLSTATE 55P03) jika lock tidak tersedia.
func (r *Repository) LockDriverUserForAccept(ctx context.Context, q Querier, driverID uuid.UUID) (*Driver, error) {
	var d Driver
	err := q.QueryRow(ctx, `
		SELECT id, user_type, status, working_status, COALESCE(min_balance_threshold, 0)
		FROM users
		WHERE id = $1 AND user_type = 'driver' AND status = 'ACTIVE'
		FOR UPDATE NOWAIT
	`, driverID).Scan(&d.ID, &d.UserType, &d.Status, &d.WorkingStatus, &d.MinBalanceThreshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDriverNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// LockOrderForAccept mengunci baris ride_orders dengan SELECT FOR UPDATE
// NOWAIT selama driver accept order. Memastikan status masih SEARCHING_DRIVER.
// Mengembalikan pgconn.PgError (SQLSTATE 55P03) jika lock tidak tersedia, dan
// ErrOrderNotFound jika order/status tidak cocok.
func (r *Repository) LockOrderForAccept(ctx context.Context, q Querier, orderID uuid.UUID) error {
	err := q.QueryRow(ctx, `
		SELECT 1 FROM ride_orders
		WHERE id = $1 AND status = 'SEARCHING_DRIVER'
		FOR UPDATE NOWAIT
	`, orderID).Scan(new(int))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOrderNotFound
	}
	return err
}

// AssignDriver menetapkan driver ke order: status → DRIVER_ASSIGNED,
// driver_id & assigned_at diisi. Return false jika order tidak dalam
// state SEARCHING_DRIVER (guard CAS double-check setelah lock).
func (r *Repository) AssignDriver(ctx context.Context, q Querier, orderID uuid.UUID, driverID uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE ride_orders
		SET status = 'DRIVER_ASSIGNED', driver_id = $2, assigned_at = NOW()
		WHERE id = $1 AND status = 'SEARCHING_DRIVER'
	`, orderID, driverID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// MarkDriverBusy menyetel working_status driver menjadi BUSY.
func (r *Repository) MarkDriverBusy(ctx context.Context, q Querier, driverID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE users
		SET working_status = 'BUSY', last_status_update_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, driverID)
	return err
}
