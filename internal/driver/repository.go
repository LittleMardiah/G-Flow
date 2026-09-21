// Package driver berisi akses data (repository) untuk endpoint "available
// orders" driver (TD-009). Repository bertanggung jawab membaca lokasi driver
// dari tabel driver_locations, menghitung jumlah order aktif seorang driver
// (capacity check, maks 3), dan men-query order yang tersedia dari semua
// layanan (ride, food, send) beserta jarak geodesik dari lokasi driver.
//
// Jarak dihitung di SQL memakai extension earthdistance (ll_to_earth) —
// konsisten dengan internal/location. Rujukan skema: DATABASE SCHEMA v10.4.
package driver

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Error definitions tingkat package.
var (
	ErrDriverLocationNotFound = errors.New("driver location not found")
	ErrNoLocation             = errors.New("driver location unavailable")
)

// RepoDB adalah subset operasi pool yang dipakai Repository untuk query.
// Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// ActiveRideOrder adalah representasi satu ride order yang sedang dikerjakan
// seorang driver (TD-077 A2): status aktif DRIVER_ASSIGNED / DRIVER_ARRIVED /
// TRIP_STARTED. Set status aktif sama persis dengan CountActiveOrders.
type ActiveRideOrder struct {
	ID             uuid.UUID
	DriverID       uuid.UUID
	PickupAddress  string
	PickupLat      float64
	PickupLng      float64
	DropoffAddress string
	DropoffLat     float64
	DropoffLng     float64
	DistanceKm     float64
	EstimatedFare  decimal.Decimal
	PaymentMethod  string
	Status         string
	CreatedAt      string
}

// ActiveFoodOrder adalah representasi satu food order yang sedang dikerjakan
// seorang driver (TD-077 A2): status aktif READY_FOR_PICKUP / PICKED_UP /
// IN_TRANSIT (konsisten dengan CountActiveOrders & kActiveFoodStatuses app).
// Data merchant digabung dari food_merchants agar driver tahu lokasi pickup.
type ActiveFoodOrder struct {
	ID              uuid.UUID
	DriverID        uuid.UUID
	MerchantName    string
	MerchantAddress string
	MerchantLat     float64
	MerchantLng     float64
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
	DeliveryFee     decimal.Decimal
	TotalAmount     decimal.Decimal
	PaymentMethod   string
	Status          string
	CreatedAt       string
}

// ActiveSendOrder adalah representasi satu send order yang sedang dikerjakan
// seorang driver (TD-077 A2): status aktif DRIVER_ASSIGNED / PICKED_UP /
// IN_TRANSIT. Alamat stop pertama disertakan agar driver tahu tujuan awal.
type ActiveSendOrder struct {
	ID               uuid.UUID
	DriverID         uuid.UUID
	PickupAddress    string
	PickupLat        float64
	PickupLng        float64
	DistanceKm       float64
	TotalFare        decimal.Decimal
	PaymentMethod    string
	Status           string
	CreatedAt        string
	FirstStopAddress *string
}

// RideAvailableOrder adalah representasi satu ride order yang tersedia untuk
// driver (belum ada driver tertunjuk & berada pada status menunggu driver).
type RideAvailableOrder struct {
	ID             uuid.UUID
	CustomerID     uuid.UUID
	PickupAddress  string
	PickupLat      float64
	PickupLng      float64
	DropoffAddress string
	DropoffLat     float64
	DropoffLng     float64
	EstimatedFare  decimal.Decimal
	PaymentMethod  string
	Status         string
	DistanceKm     float64
	CreatedAt      string
}

// FoodAvailableOrder adalah representasi satu food order yang tersedia untuk
// driver (belum ada driver tertunjuk & status siap dicari driver).
type FoodAvailableOrder struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	MerchantID      uuid.UUID
	MerchantName    string
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
	PaymentMethod   string
	TotalAmount     decimal.Decimal
	Status          string
	DistanceKm      float64
	CreatedAt       string
}

// SendAvailableOrder adalah representasi satu send order yang tersedia untuk
// driver (belum ada driver tertunjuk & menunggu driver).
type SendAvailableOrder struct {
	ID               uuid.UUID
	SenderID         uuid.UUID
	PickupAddress    string
	PickupLat        float64
	PickupLng        float64
	FirstStopAddress string
	PaymentMethod    string
	TotalFare        decimal.Decimal
	Status           string
	DistanceKm       float64
	CreatedAt        string
}

// Repository adalah akses data untuk modul driver (available orders).
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan koneksi database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// GetDriverLocation membaca lokasi driver terbaru dari tabel driver_locations.
// Mengembalikan ErrDriverLocationNotFound jika driver belum pernah melaporkan
// lokasi (tidak ada baris di driver_locations).
func (r *Repository) GetDriverLocation(ctx context.Context, driverID uuid.UUID) (float64, float64, error) {
	var lat, lng float64
	err := r.db.QueryRow(ctx, `
		SELECT current_lat, current_lng
		FROM driver_locations
		WHERE driver_id = $1
		ORDER BY updated_at DESC
		LIMIT 1
	`, driverID).Scan(&lat, &lng)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, ErrDriverLocationNotFound
	}
	if err != nil {
		return 0, 0, err
	}
	return lat, lng, nil
}

// CountActiveOrders menghitung jumlah order aktif seorang driver (capacity
// check, maks 3) melintasi ride, food, dan send pada status yang sedang
// dijalankan (belum selesai/dibatalkan).
func (r *Repository) CountActiveOrders(ctx context.Context, driverID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM ride_orders
			  WHERE driver_id = $1
			    AND status IN ('DRIVER_ASSIGNED', 'DRIVER_ARRIVED', 'TRIP_STARTED'))
		  + (SELECT COUNT(*) FROM food_orders
			  WHERE driver_id = $1
			    AND status IN ('READY_FOR_PICKUP', 'PICKED_UP', 'IN_TRANSIT'))
		  + (SELECT COUNT(*) FROM send_orders
			  WHERE driver_id = $1
			    AND status IN ('DRIVER_ASSIGNED', 'PICKED_UP', 'IN_TRANSIT'))
	`, driverID).Scan(&count)
	return count, err
}

// GetActiveRideOrders men-query ride order aktif milik driver (TD-077 A2).
// Filter status sama persis dengan CountActiveOrders (DRIVER_ASSIGNED /
// DRIVER_ARRIVED / TRIP_STARTED) sehingga jumlah item == kapasitas yang
// terpakai. Order CANCELLED/COMPLETED/SETTLED tidak ikut.
func (r *Repository) GetActiveRideOrders(ctx context.Context, driverID uuid.UUID) ([]ActiveRideOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, driver_id, pickup_address, pickup_lat, pickup_lng,
		       dropoff_address, dropoff_lat, dropoff_lng,
		       distance_km::float8, estimated_fare, payment_method, status,
		       (created_at AT TIME ZONE 'UTC')::text
		FROM ride_orders
		WHERE driver_id = $1
		  AND status IN ('DRIVER_ASSIGNED', 'DRIVER_ARRIVED', 'TRIP_STARTED')
		ORDER BY created_at ASC
	`, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []ActiveRideOrder
	for rows.Next() {
		var o ActiveRideOrder
		if err := rows.Scan(&o.ID, &o.DriverID, &o.PickupAddress, &o.PickupLat, &o.PickupLng,
			&o.DropoffAddress, &o.DropoffLat, &o.DropoffLng,
			&o.DistanceKm, &o.EstimatedFare, &o.PaymentMethod, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetActiveFoodOrders men-query food order aktif milik driver (TD-077 A2).
// Filter status sama persis dengan CountActiveOrders (READY_FOR_PICKUP /
// PICKED_UP / IN_TRANSIT). Titik pickup = lokasi merchant (food_merchants);
// delivery_lat/lng bersifat nullable sehingga di-COALESCE ke 0.
func (r *Repository) GetActiveFoodOrders(ctx context.Context, driverID uuid.UUID) ([]ActiveFoodOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT fo.id, fo.driver_id, fm.merchant_name, fm.address,
		       fm.latitude::float8, fm.longitude::float8,
		       fo.delivery_address,
		       COALESCE(fo.delivery_lat, 0)::float8, COALESCE(fo.delivery_lng, 0)::float8,
		       fo.delivery_fee, fo.total_amount, fo.payment_method, fo.status,
		       (fo.created_at AT TIME ZONE 'UTC')::text
		FROM food_orders fo
		JOIN food_merchants fm ON fm.id = fo.merchant_id
		WHERE fo.driver_id = $1
		  AND fo.status IN ('READY_FOR_PICKUP', 'PICKED_UP', 'IN_TRANSIT')
		ORDER BY fo.created_at ASC
	`, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []ActiveFoodOrder
	for rows.Next() {
		var o ActiveFoodOrder
		if err := rows.Scan(&o.ID, &o.DriverID, &o.MerchantName, &o.MerchantAddress,
			&o.MerchantLat, &o.MerchantLng,
			&o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng,
			&o.DeliveryFee, &o.TotalAmount, &o.PaymentMethod, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetActiveSendOrders men-query send order aktif milik driver (TD-077 A2).
// Filter status sama persis dengan CountActiveOrders (DRIVER_ASSIGNED /
// PICKED_UP / IN_TRANSIT). Menyertakan alamat stop pertama (indeks stop
// terkecil) agar driver tahu tujuan awal.
func (r *Repository) GetActiveSendOrders(ctx context.Context, driverID uuid.UUID) ([]ActiveSendOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT so.id, so.driver_id, so.pickup_address,
		       so.pickup_lat::float8, so.pickup_lng::float8,
		       so.distance_km::float8, so.total_fare, so.payment_method, so.status,
		       (so.created_at AT TIME ZONE 'UTC')::text,
		       (SELECT ss.dropoff_address
		          FROM send_order_stops ss
		         WHERE ss.order_id = so.id
		         ORDER BY ss.stop_number ASC
		         LIMIT 1) AS first_stop_address
		FROM send_orders so
		WHERE so.driver_id = $1
		  AND so.status IN ('DRIVER_ASSIGNED', 'PICKED_UP', 'IN_TRANSIT')
		ORDER BY so.created_at ASC
	`, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []ActiveSendOrder
	for rows.Next() {
		var o ActiveSendOrder
		if err := rows.Scan(&o.ID, &o.DriverID, &o.PickupAddress,
			&o.PickupLat, &o.PickupLng,
			&o.DistanceKm, &o.TotalFare, &o.PaymentMethod, &o.Status, &o.CreatedAt,
			&o.FirstStopAddress); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetAvailableRideOrders men-query ride order yang tersedia (status menunggu
// driver, belum ada driver) dalam radius radiusKm dari (lat,lng), jarak
// geodesik dihitung via earthdistance.
func (r *Repository) GetAvailableRideOrders(ctx context.Context, lat, lng, radiusKm float64) ([]RideAvailableOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, customer_id, pickup_address, pickup_lat, pickup_lng,
		       dropoff_address, dropoff_lat, dropoff_lng,
		       estimated_fare, payment_method, status,
		       ROUND(
		         earth_distance(ll_to_earth($1, $2), ll_to_earth(pickup_lat, pickup_lng)) / 1000.0,
		         3
		       )::float8 AS distance_km,
		       (created_at AT TIME ZONE 'UTC')::text
		FROM ride_orders
		WHERE status IN ('CREATED', 'SEARCHING_DRIVER')
		  AND driver_id IS NULL
		  AND earth_distance(ll_to_earth($1, $2), ll_to_earth(pickup_lat, pickup_lng)) / 1000.0 <= $3
		ORDER BY distance_km ASC, created_at ASC
	`, lat, lng, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []RideAvailableOrder
	for rows.Next() {
		var o RideAvailableOrder
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.PickupAddress, &o.PickupLat, &o.PickupLng,
			&o.DropoffAddress, &o.DropoffLat, &o.DropoffLng,
			&o.EstimatedFare, &o.PaymentMethod, &o.Status, &o.DistanceKm, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetAvailableFoodOrders men-query food order yang tersedia (belum ada driver)
// dalam radius radiusKm dari (lat,lng). Titik acuan jarak adalah lokasi
// merchant (titik pickup driver), digabung dari tabel food_merchants.
func (r *Repository) GetAvailableFoodOrders(ctx context.Context, lat, lng, radiusKm float64) ([]FoodAvailableOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT fo.id, fo.customer_id, fo.merchant_id, fm.merchant_name,
		       fo.delivery_address, fo.delivery_lat, fo.delivery_lng,
		       fo.payment_method, fo.total_amount, fo.status,
		       ROUND(
		         earth_distance(ll_to_earth($1, $2), ll_to_earth(fm.latitude, fm.longitude)) / 1000.0,
		         3
		       )::float8 AS distance_km,
		       (fo.created_at AT TIME ZONE 'UTC')::text
		FROM food_orders fo
		JOIN food_merchants fm ON fm.id = fo.merchant_id
		WHERE fo.status IN ('CREATED', 'CONFIRMED', 'PREPARING', 'READY_FOR_PICKUP')
		  AND fo.driver_id IS NULL
		  AND earth_distance(ll_to_earth($1, $2), ll_to_earth(fm.latitude, fm.longitude)) / 1000.0 <= $3
		ORDER BY distance_km ASC, fo.created_at ASC
	`, lat, lng, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []FoodAvailableOrder
	for rows.Next() {
		var o FoodAvailableOrder
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.MerchantID, &o.MerchantName,
			&o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng,
			&o.PaymentMethod, &o.TotalAmount, &o.Status, &o.DistanceKm, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetAvailableSendOrders men-query send order yang tersedia (belum ada driver
// & menunggu driver) dalam radius radiusKm dari (lat,lng). Menyertakan alamat
// stop pertama (indeks stop terkecil) agar driver tahu tujuan awal.
func (r *Repository) GetAvailableSendOrders(ctx context.Context, lat, lng, radiusKm float64) ([]SendAvailableOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT so.id, so.sender_id, so.pickup_address, so.pickup_lat, so.pickup_lng,
		       (SELECT ss.dropoff_address
		          FROM send_order_stops ss
		         WHERE ss.order_id = so.id
		         ORDER BY ss.stop_number ASC
		         LIMIT 1) AS first_stop_address,
		       so.payment_method, so.total_fare, so.status,
		       ROUND(
		         earth_distance(ll_to_earth($1, $2), ll_to_earth(so.pickup_lat, so.pickup_lng)) / 1000.0,
		         3
		       )::float8 AS distance_km,
		       (so.created_at AT TIME ZONE 'UTC')::text
		FROM send_orders so
		WHERE so.status IN ('CREATED', 'SEARCHING_DRIVER')
		  AND so.driver_id IS NULL
		  AND earth_distance(ll_to_earth($1, $2), ll_to_earth(so.pickup_lat, so.pickup_lng)) / 1000.0 <= $3
		ORDER BY distance_km ASC, so.created_at ASC
	`, lat, lng, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []SendAvailableOrder
	for rows.Next() {
		var o SendAvailableOrder
		if err := rows.Scan(&o.ID, &o.SenderID, &o.PickupAddress, &o.PickupLat, &o.PickupLng,
			&o.FirstStopAddress,
			&o.PaymentMethod, &o.TotalFare, &o.Status, &o.DistanceKm, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
