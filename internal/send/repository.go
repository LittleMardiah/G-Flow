// Package send berisi akses data (repository) untuk modul G-Send (Phase 3).
// Repository hanya bertanggung jawab mengakses database; business logic
// ditangani di Service Layer (3.5). Semua query memakai parameterized statement
// untuk mencegah SQL injection.
//
// Rujukan skema: MIGRATION 005_food_send_schema.up.sql (LOCKED) —
// send_orders, send_order_stops, send_order_events.
package send

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
	ErrUserNotFound      = errors.New("user not found")
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrSendOrderNotFound = errors.New("send order not found")
	ErrLockTimeout       = errors.New("send order lock not available (NOWAIT timeout)")
)

// RepoDB adalah subset operasi pool yang dipakai Repository untuk query di
// luar transaksi. Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Querier adalah subset operasi yang bisa dipakai dalam transaksi (pgx.Tx)
// ATAU pool. Method write yang dieksekusi di dalam transaksi order G-Send
// menerima Querier agar tetap satu unit kerja atomik.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// SendCustomer adalah representasi subset baris users yang dibutuhkan untuk
// validasi send order (user_type, status, dan overdue_debt sebagai Debt Gate
// Universal — ROADMAP 3.5.1 langkah 1).
type SendCustomer struct {
	ID          uuid.UUID
	UserType    string
	Status      string
	OverdueDebt decimal.Decimal
}

// SendWallet adalah representasi subset baris wallets yang dipakai modul send
// (id, user_id, wallet_type, balance, status).
type SendWallet struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Type    string
	Balance decimal.Decimal
	Status  string
}

// SendOrder adalah representasi baris tabel send_orders (MIGRATION 005).
// Kolom nullable direpresentasikan sebagai pointer; nil berarti SQL NULL.
type SendOrder struct {
	ID                  uuid.UUID
	SenderID            uuid.UUID
	DriverID            *uuid.UUID
	SenderWalletID      *uuid.UUID
	DriverWalletID      *uuid.UUID
	PackageWeightKg     decimal.Decimal
	PackageDimensionsCm *string
	PackageDescription  *string
	PickupLat           decimal.Decimal
	PickupLng           decimal.Decimal
	PickupAddress       string
	BaseFare            decimal.Decimal
	DistanceKm          decimal.Decimal
	WeightSurcharge     decimal.Decimal
	TotalFare           decimal.Decimal
	DeclaredValue       decimal.Decimal
	PackageType         string
	InsuranceFee        decimal.Decimal
	DiscountAmount      decimal.Decimal
	VoucherID           *uuid.UUID
	PaymentMethod       string
	PlatformCommission  *decimal.Decimal
	DriverEarning       *decimal.Decimal
	Status              string
	DeliveryPhotoURL    *string
	RecipientSignature  []byte
	CreatedAt           time.Time
	AssignedAt          *time.Time
	PickupAt            *time.Time
	DeliveredAt         *time.Time
	SettledAt           *time.Time
	IsSettled           bool
}

// SendOrderStop adalah representasi baris tabel send_order_stops (multi-stop
// dengan alokasi ongkos — Task 3.5).
type SendOrderStop struct {
	ID                 uuid.UUID
	OrderID            uuid.UUID
	StopNumber         int
	RecipientName      *string
	RecipientPhone     *string
	DropoffLat         decimal.Decimal
	DropoffLng         decimal.Decimal
	DropoffAddress     string
	DistanceKm         *decimal.Decimal
	AllocatedFare      *decimal.Decimal
	Status             string
	DeliveryPhotoURL   *string
	RecipientSignature []byte
	ArrivedAt          *time.Time
	CompletedAt        *time.Time
	Notes              *string
}

// SendOrderEvent adalah representasi baris tabel send_order_events (audit
// trail transisi status G-Send).
type SendOrderEvent struct {
	ID          uuid.UUID
	OrderID     uuid.UUID
	FromStatus  *string
	ToStatus    string
	Reason      *string
	TriggeredBy *uuid.UUID
	Metadata    []byte
}

// Repository adalah akses data untuk tabel G-Send.
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan koneksi database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// GetCustomer mengambil data customer untuk validasi send order (user_type,
// status, dan overdue_debt sebagai Debt Gate Universal). Mengembalikan
// ErrUserNotFound jika user tidak ada.
func (r *Repository) GetCustomer(ctx context.Context, userID uuid.UUID) (*SendCustomer, error) {
	var c SendCustomer
	err := r.db.QueryRow(ctx, `
		SELECT id, user_type, status, COALESCE(overdue_debt, 0)
		FROM users
		WHERE id = $1
	`, userID).Scan(&c.ID, &c.UserType, &c.Status, &c.OverdueDebt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetWalletByUserAndType mengambil wallet spesifik user berdasarkan tipe.
// Mengembalikan ErrWalletNotFound jika wallet tidak ada.
func (r *Repository) GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*SendWallet, error) {
	var w SendWallet
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

// SystemWalletID mengambil id wallet sistem (user_id IS NULL) berdasar tipe,
// misal SYSTEM_ESCROW. Mengembalikan ErrWalletNotFound jika belum di-seed.
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

// InsertSendOrder membuat baris send_orders. Status awal di-set langsung
// 'SEARCHING_DRIVER' setelah escrow (ROADMAP 3.5.1 langkah 5-6). Dipanggil
// dalam transaksi yang sama dengan insert stops + escrow agar atomik.
func (r *Repository) InsertSendOrder(ctx context.Context, q Querier, o *SendOrder) error {
	_, err := q.Exec(ctx, `
		INSERT INTO send_orders (
			id, sender_id, driver_id,
			sender_wallet_id, driver_wallet_id,
			package_weight_kg, package_dimensions_cm, package_description,
			pickup_lat, pickup_lng, pickup_address,
			base_fare, distance_km, weight_surcharge, total_fare,
			declared_value, package_type, insurance_fee,
			discount_amount, voucher_id,
			payment_method, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`,
		o.ID,
		o.SenderID,
		o.DriverID,
		o.SenderWalletID,
		o.DriverWalletID,
		o.PackageWeightKg,
		o.PackageDimensionsCm,
		o.PackageDescription,
		o.PickupLat,
		o.PickupLng,
		o.PickupAddress,
		o.BaseFare,
		o.DistanceKm,
		o.WeightSurcharge,
		o.TotalFare,
		o.DeclaredValue,
		o.PackageType,
		o.InsuranceFee,
		o.DiscountAmount,
		o.VoucherID,
		o.PaymentMethod,
		o.Status,
	)
	return err
}

// InsertSendOrderStops membuat baris send_order_stops (loop stops — Task 3.5
// langkah 7). Setiap stop: stop_number, alamat/lat/lng, distance_km dari stop
// sebelumnya, allocated_fare, status PENDING.
func (r *Repository) InsertSendOrderStops(ctx context.Context, q Querier, stops []*SendOrderStop) error {
	for _, st := range stops {
		if _, err := q.Exec(ctx, `
			INSERT INTO send_order_stops (
				id, order_id, stop_number,
				recipient_name, recipient_phone,
				dropoff_lat, dropoff_lng, dropoff_address,
				distance_km, allocated_fare,
				status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`,
			st.ID, st.OrderID, st.StopNumber,
			st.RecipientName, st.RecipientPhone,
			st.DropoffLat, st.DropoffLng, st.DropoffAddress,
			st.DistanceKm, st.AllocatedFare,
			st.Status,
		); err != nil {
			return err
		}
	}
	return nil
}

// InsertSendOrderEvent mencatat audit trail transisi status
// (send_order_events).
func (r *Repository) InsertSendOrderEvent(ctx context.Context, q Querier, e SendOrderEvent) error {
	_, err := q.Exec(ctx, `
		INSERT INTO send_order_events (order_id, from_status, to_status, reason, triggered_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, e.OrderID, e.FromStatus, e.ToStatus, e.Reason, e.TriggeredBy, e.Metadata)
	return err
}

// sendOrderColumns daftar kolom send_orders untuk SELECT lengkap. Dipakai
// bersama oleh GetSendOrderByID dan LockSendOrder agar satu sumber.
const sendOrderColumns = `id, sender_id, driver_id,
	sender_wallet_id, driver_wallet_id,
	package_weight_kg, package_dimensions_cm, package_description,
	pickup_lat, pickup_lng, pickup_address,
	base_fare, distance_km, weight_surcharge, total_fare,
	declared_value, package_type, insurance_fee,
	discount_amount, voucher_id,
	payment_method, platform_commission, driver_earning,
	status, delivery_photo_url, recipient_signature,
	created_at, assigned_at, pickup_at, delivered_at, settled_at,
	is_settled`

// GetSendOrderByID mengambil send order lengkap berdasarkan id.
func (r *Repository) GetSendOrderByID(ctx context.Context, orderID uuid.UUID) (*SendOrder, error) {
	return scanSendOrderRow(r.db.QueryRow(ctx, `SELECT `+sendOrderColumns+` FROM send_orders WHERE id = $1`, orderID))
}

// LockSendOrder mengambil + mengunci baris send_orders dengan SELECT ...
// FOR UPDATE NOWAIT (Task 3.5 status update): dua transisi bersamaan tidak
// bisa saling menimpa. Mengembalikan ErrLockTimeout (SQLSTATE 55P03) jika lock
// tidak tersedia, atau ErrSendOrderNotFound jika order tidak ada.
func (r *Repository) LockSendOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*SendOrder, error) {
	return scanSendOrderRow(q.QueryRow(ctx, `SELECT `+sendOrderColumns+` FROM send_orders WHERE id = $1 FOR UPDATE NOWAIT`, orderID))
}

// UpdateSendOrderStatus mengubah status send order secara atomik (CAS) dengan
// guard WHERE status = $from. Timestamp terkait di-set sekali saat transisi
// yang tepat. Return true jika baris berubah.
func (r *Repository) UpdateSendOrderStatus(ctx context.Context, q Querier, orderID uuid.UUID,
	fromStatus, toStatus string) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE send_orders
		SET status = $3,
		    pickup_at = CASE WHEN $3 = 'PICKED_UP' AND pickup_at IS NULL THEN NOW() ELSE pickup_at END,
		    delivered_at = CASE WHEN $3 = 'DELIVERED' AND delivered_at IS NULL THEN NOW() ELSE delivered_at END,
		    updated_at = NOW()
		WHERE id = $1 AND status = $2
	`, orderID, fromStatus, toStatus)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// GetSendOrderStops mengambil semua stop sebuah send order (urut stop_number).
func (r *Repository) GetSendOrderStops(ctx context.Context, orderID uuid.UUID) ([]*SendOrderStop, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, stop_number,
		       recipient_name, recipient_phone,
		       dropoff_lat, dropoff_lng, dropoff_address,
		       distance_km, allocated_fare, status,
		       delivery_photo_url, recipient_signature,
		       arrived_at, completed_at, notes
		FROM send_order_stops
		WHERE order_id = $1
		ORDER BY stop_number ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stops := make([]*SendOrderStop, 0)
	for rows.Next() {
		var st SendOrderStop
		if err := rows.Scan(
			&st.ID, &st.OrderID, &st.StopNumber,
			&st.RecipientName, &st.RecipientPhone,
			&st.DropoffLat, &st.DropoffLng, &st.DropoffAddress,
			&st.DistanceKm, &st.AllocatedFare, &st.Status,
			&st.DeliveryPhotoURL, &st.RecipientSignature,
			&st.ArrivedAt, &st.CompletedAt, &st.Notes,
		); err != nil {
			return nil, err
		}
		stops = append(stops, &st)
	}
	return stops, rows.Err()
}

// --- scanner send orders ---

func scanSendOrderRow(row pgx.Row) (*SendOrder, error) {
	var o SendOrder
	err := row.Scan(
		&o.ID, &o.SenderID, &o.DriverID,
		&o.SenderWalletID, &o.DriverWalletID,
		&o.PackageWeightKg, &o.PackageDimensionsCm, &o.PackageDescription,
		&o.PickupLat, &o.PickupLng, &o.PickupAddress,
		&o.BaseFare, &o.DistanceKm, &o.WeightSurcharge, &o.TotalFare,
		&o.DeclaredValue, &o.PackageType, &o.InsuranceFee,
		&o.DiscountAmount, &o.VoucherID,
		&o.PaymentMethod, &o.PlatformCommission, &o.DriverEarning,
		&o.Status, &o.DeliveryPhotoURL, &o.RecipientSignature,
		&o.CreatedAt, &o.AssignedAt, &o.PickupAt, &o.DeliveredAt, &o.SettledAt,
		&o.IsSettled,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSendOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
