// Package driver — Service Layer (TD-009: Available Orders endpoint for driver).
//
// Service menggabungkan business logic penyediaan daftar order yang tersedia
// bagi seorang driver:
//   - Membaca lokasi driver dari Redis (L1, hash driver:location:{id}) dengan
//     fallback ke PostgreSQL (driver_locations) — pola dual-layer sama dgn
//     internal/location.
//   - Kapasitas driver (maks 3 order aktif) — kapasitas penuh artinya driver
//     terdaftar tetapi tidak boleh menerima order baru.
//   - Men-query order tersedia dari semua layanan (ride, food, send) dalam
//     radius 5 km dari lokasi driver dan menyeragamkan strukturnya.
package driver

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// Konstanta domain TD-009.
const (
	// Radius pencarian order tersedia (km) — sesuai spesifikasi TD-009 (5km).
	availableOrdersRadiusKm = 5.0

	// Kapasitas maksimal order aktif driver (ROADMAP 3.10 / Task 3.6.1).
	maxActiveOrders = 3

	// Prefix & field Redis tempat lokasi driver (konsisten dgn internal/location).
	redisKeyPrefix = "driver:location:"
	fieldLat       = "lat"
	fieldLng       = "lng"
)

// Tipe order (order_type_enum / label layanan).
const (
	OrderTypeRide = "ride"
	OrderTypeFood = "food"
	OrderTypeSend = "send"
)

// Error definitions tingkat service.
var (
	ErrDriverLocationUnavailable = errors.New("driver location unavailable")
	ErrInvalidCoordinates        = errors.New("invalid coordinates")
)

// Repo adalah kontrak repository yang dibutuhkan Service. Dipenuhi oleh
// *Repository (internal/driver/repository.go); dijadikan interface agar mudah
// di-mock pada unit test.
type Repo interface {
	GetDriverLocation(ctx context.Context, driverID uuid.UUID) (float64, float64, error)
	CountActiveOrders(ctx context.Context, driverID uuid.UUID) (int, error)
	GetAvailableRideOrders(ctx context.Context, lat, lng, radiusKm float64) ([]RideAvailableOrder, error)
	GetAvailableFoodOrders(ctx context.Context, lat, lng, radiusKm float64) ([]FoodAvailableOrder, error)
	GetAvailableSendOrders(ctx context.Context, lat, lng, radiusKm float64) ([]SendAvailableOrder, error)
	GetActiveRideOrders(ctx context.Context, driverID uuid.UUID) ([]ActiveRideOrder, error)
	GetActiveFoodOrders(ctx context.Context, driverID uuid.UUID) ([]ActiveFoodOrder, error)
	GetActiveSendOrders(ctx context.Context, driverID uuid.UUID) ([]ActiveSendOrder, error)
	GetDriverProfile(ctx context.Context, driverID uuid.UUID) (*DriverProfile, error)
}

// RedisClient adalah subset operasi Redis yang dipakai Service untuk membaca
// lokasi driver (L1). Dipenuhi oleh *redis.Client.
type RedisClient interface {
	HGet(ctx context.Context, key string, field string) *redis.StringCmd
}

// AvailableOrder adalah representasi seragam satu order tersedia dari layanan
// manapun (ride/food/send) yang dikembalikan ke aplikasi driver.
type AvailableOrder struct {
	ID            uuid.UUID       `json:"id"`
	Type          string          `json:"type"`
	Status        string          `json:"status"`
	PickupAddress string          `json:"pickup_address,omitempty"`
	DropoffLabel  string          `json:"dropoff_label,omitempty"`
	PickupLat     float64         `json:"pickup_lat,omitempty"`
	PickupLng     float64         `json:"pickup_lng,omitempty"`
	DropoffLat    float64         `json:"dropoff_lat,omitempty"`
	DropoffLng    float64         `json:"dropoff_lng,omitempty"`
	DistanceKm    float64         `json:"distance_km"`
	Earning       decimal.Decimal `json:"earning"`
	Fare          decimal.Decimal `json:"fare"`
	PaymentMethod string          `json:"payment_method"`
	CreatedAt     string          `json:"created_at"`
	MerchantName  string          `json:"merchant_name,omitempty"`
	StopAddress   string          `json:"stop_address,omitempty"`
}

// AvailableOrdersResult adalah hasil panggilan GetAvailableOrders: daftar
// order tersedia + indikator apakah driver masih punya slot kapasitas untuk
// menerima order baru.
type AvailableOrdersResult struct {
	CapacityAvailable bool
	ActiveOrders      int
	MaxActiveOrders   int
	Orders            []AvailableOrder
}

// DriverActiveOrder adalah satu order aktif milik driver (TD-077 A2 GET
// /drivers/orders). `type` memisahkan ride/food/send; field spesifik per-tipe
// (pickup/dropoff/fare) hanya terisi untuk tipe terkait (omitempty). Naming
// mengikuti konvensi response existing: ride = estimated_fare, food =
// total_amount/delivery_fee, send = total_fare (API CONTRACT / DTO detail).
type DriverActiveOrder struct {
	Type     string    `json:"type"`
	OrderID  uuid.UUID `json:"order_id"`
	Status   string    `json:"status"`
	DriverID uuid.UUID `json:"driver_id,omitempty"`
	// Ride.
	PickupAddress  string          `json:"pickup_address,omitempty"`
	PickupLat      float64         `json:"pickup_lat,omitempty"`
	PickupLng      float64         `json:"pickup_lng,omitempty"`
	DropoffAddress string          `json:"dropoff_address,omitempty"`
	DropoffLat     float64         `json:"dropoff_lat,omitempty"`
	DropoffLng     float64         `json:"dropoff_lng,omitempty"`
	DistanceKm     float64         `json:"distance_km,omitempty"`
	EstimatedFare  decimal.Decimal `json:"estimated_fare,omitempty"`
	// Food.
	MerchantName    string          `json:"merchant_name,omitempty"`
	MerchantAddress string          `json:"merchant_address,omitempty"`
	MerchantLat     float64         `json:"merchant_lat,omitempty"`
	MerchantLng     float64         `json:"merchant_lng,omitempty"`
	DeliveryAddress string          `json:"delivery_address,omitempty"`
	DeliveryLat     float64         `json:"delivery_lat,omitempty"`
	DeliveryLng     float64         `json:"delivery_lng,omitempty"`
	DeliveryFee     decimal.Decimal `json:"delivery_fee,omitempty"`
	TotalAmount     decimal.Decimal `json:"total_amount,omitempty"`
	// Send.
	TotalFare        decimal.Decimal `json:"total_fare,omitempty"`
	FirstStopAddress string          `json:"first_stop_address,omitempty"`
	// Common.
	PaymentMethod string `json:"payment_method"`
	CreatedAt     string `json:"created_at"`
}

// ActiveOrdersResult adalah hasil panggilan GetActiveOrders: daftar order
// aktif milik driver (ride + food + send) yang harus dikerjakan.
type ActiveOrdersResult struct {
	Orders []DriverActiveOrder
}

type DriverProfile struct {
	DriverID          uuid.UUID
	Name              string
	Email             string
	Phone             *string
	VehicleType       *string
	VehiclePlate      *string
	LicenseNumber     *string
	LicenseExpiry     *string
	Status            string
	RatingAvg         float64
	TotalRides        int64
	BankName          *string
	BankAccountNumber *string
}

// Service adalah business logic modul driver (available orders).
type Service struct {
	repo Repo
	rdb  RedisClient
}

// NewService membuat Service baru dengan dependency injection.
// rdb boleh nil (graceful degradation: lokasi dibaca dari DB saja).
func NewService(repo Repo, rdb RedisClient) *Service {
	return &Service{repo: repo, rdb: rdb}
}

// GetAvailableOrders mengembalikan daftar order tersedia untuk driver dalam
// radius 5 km beserta status kapasitas. Alur:
//  1. Baca lokasi driver (Redis L1 → fallback repository DB).
//  2. Hitung jumlah order aktif driver untuk capacity check (maks 3).
//  3. Query order tersedia dari ride, food, dan send dalam radius 5 km.
func (s *Service) GetAvailableOrders(ctx context.Context, driverID uuid.UUID) (*AvailableOrdersResult, error) {
	lat, lng, err := s.driverLocation(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if !validLatLng(lat, lng) {
		return nil, ErrInvalidCoordinates
	}

	activeOrders, err := s.repo.CountActiveOrders(ctx, driverID)
	if err != nil {
		return nil, err
	}

	res := &AvailableOrdersResult{
		ActiveOrders:      activeOrders,
		MaxActiveOrders:   maxActiveOrders,
		CapacityAvailable: activeOrders < maxActiveOrders,
		Orders:            []AvailableOrder{},
	}

	rides, err := s.repo.GetAvailableRideOrders(ctx, lat, lng, availableOrdersRadiusKm)
	if err != nil {
		return nil, err
	}
	foods, err := s.repo.GetAvailableFoodOrders(ctx, lat, lng, availableOrdersRadiusKm)
	if err != nil {
		return nil, err
	}
	sends, err := s.repo.GetAvailableSendOrders(ctx, lat, lng, availableOrdersRadiusKm)
	if err != nil {
		return nil, err
	}

	for _, o := range rides {
		res.Orders = append(res.Orders, fromRide(o))
	}
	for _, o := range foods {
		res.Orders = append(res.Orders, fromFood(o))
	}
	for _, o := range sends {
		res.Orders = append(res.Orders, fromSend(o))
	}

	return res, nil
}

// GetActiveOrders mengembalikan daftar order aktif milik driver (TD-077 A2):
// union ride + food + send pada status aktif yang sama dengan kapasitas
// CountActiveOrders (maks 3). Urutan hasil ride → food → send (konsisten
// dgn GetAvailableOrders). Tidak butuh lokasi — langsung query by driver_id.
func (s *Service) GetActiveOrders(ctx context.Context, driverID uuid.UUID) (*ActiveOrdersResult, error) {
	rides, err := s.repo.GetActiveRideOrders(ctx, driverID)
	if err != nil {
		return nil, err
	}
	foods, err := s.repo.GetActiveFoodOrders(ctx, driverID)
	if err != nil {
		return nil, err
	}
	sends, err := s.repo.GetActiveSendOrders(ctx, driverID)
	if err != nil {
		return nil, err
	}

	res := &ActiveOrdersResult{Orders: []DriverActiveOrder{}}
	for _, o := range rides {
		res.Orders = append(res.Orders, fromActiveRide(o))
	}
	for _, o := range foods {
		res.Orders = append(res.Orders, fromActiveFood(o))
	}
	for _, o := range sends {
		res.Orders = append(res.Orders, fromActiveSend(o))
	}
	return res, nil
}

func (s *Service) GetDriverProfile(ctx context.Context, driverID uuid.UUID) (*DriverProfile, error) {
	return s.repo.GetDriverProfile(ctx, driverID)
}

// driverLocation membaca lokasi driver: Redis L1 (hash lat/lng) → jika tidak
// tersedia/gagal, fallback ke repository.GetDriverLocation (PostgreSQL).
// Mengembalikan ErrDriverLocationUnavailable bila kedua sumber tidak tersedia.
func (s *Service) driverLocation(ctx context.Context, driverID uuid.UUID) (float64, float64, error) {
	if s.rdb != nil {
		if lat, lng, ok := s.readRedisLocation(ctx, driverID); ok {
			return lat, lng, nil
		}
	}

	if lat, lng, err := s.repo.GetDriverLocation(ctx, driverID); err == nil {
		return lat, lng, nil
	}

	return 0, 0, ErrDriverLocationUnavailable
}

// readRedisLocation membaca koordinat driver dari hash Redis
// driver:location:{driverID}. Mengembalikan ok=false bila key/field tidak ada
// atau parsing gagal.
func (s *Service) readRedisLocation(ctx context.Context, driverID uuid.UUID) (float64, float64, bool) {
	key := redisKeyPrefix + driverID.String()
	latStr, err := s.rdb.HGet(ctx, key, fieldLat).Result()
	if err != nil {
		return 0, 0, false
	}
	lngStr, err := s.rdb.HGet(ctx, key, fieldLng).Result()
	if err != nil {
		return 0, 0, false
	}
	lat, errLat := strconv.ParseFloat(latStr, 64)
	lng, errLng := strconv.ParseFloat(lngStr, 64)
	if errLat != nil || errLng != nil {
		return 0, 0, false
	}
	return lat, lng, true
}

// fromRide memetakan RideAvailableOrder ke bentuk seragam AvailableOrder.
func fromRide(o RideAvailableOrder) AvailableOrder {
	return AvailableOrder{
		ID:            o.ID,
		Type:          OrderTypeRide,
		Status:        o.Status,
		PickupAddress: o.PickupAddress,
		DropoffLabel:  o.DropoffAddress,
		PickupLat:     o.PickupLat,
		PickupLng:     o.PickupLng,
		DropoffLat:    o.DropoffLat,
		DropoffLng:    o.DropoffLng,
		DistanceKm:    o.DistanceKm,
		Earning:       o.EstimatedFare.Mul(decimal.NewFromFloat(0.80)).Round(2),
		Fare:          o.EstimatedFare,
		PaymentMethod: o.PaymentMethod,
		CreatedAt:     o.CreatedAt,
	}
}

// fromFood memetakan FoodAvailableOrder ke bentuk seragam AvailableOrder.
func fromFood(o FoodAvailableOrder) AvailableOrder {
	return AvailableOrder{
		ID:            o.ID,
		Type:          OrderTypeFood,
		Status:        o.Status,
		PickupAddress: o.MerchantName,
		DropoffLabel:  o.DeliveryAddress,
		PickupLat:     o.DeliveryLat,
		PickupLng:     o.DeliveryLng,
		DistanceKm:    o.DistanceKm,
		Earning:       o.TotalAmount,
		Fare:          o.TotalAmount,
		PaymentMethod: o.PaymentMethod,
		CreatedAt:     o.CreatedAt,
		MerchantName:  o.MerchantName,
	}
}

// fromSend memetakan SendAvailableOrder ke bentuk seragam AvailableOrder.
func fromSend(o SendAvailableOrder) AvailableOrder {
	return AvailableOrder{
		ID:            o.ID,
		Type:          OrderTypeSend,
		Status:        o.Status,
		PickupAddress: o.PickupAddress,
		DropoffLabel:  o.FirstStopAddress,
		PickupLat:     o.PickupLat,
		PickupLng:     o.PickupLng,
		DistanceKm:    o.DistanceKm,
		Earning:       o.TotalFare.Mul(decimal.NewFromFloat(0.90)).Round(2),
		Fare:          o.TotalFare,
		PaymentMethod: o.PaymentMethod,
		CreatedAt:     o.CreatedAt,
		StopAddress:   o.FirstStopAddress,
	}
}

// fromActiveRide memetakan ActiveRideOrder ke DTO order aktif (TD-077 A2).
func fromActiveRide(o ActiveRideOrder) DriverActiveOrder {
	return DriverActiveOrder{
		Type:           OrderTypeRide,
		OrderID:        o.ID,
		Status:         o.Status,
		DriverID:       o.DriverID,
		PickupAddress:  o.PickupAddress,
		PickupLat:      o.PickupLat,
		PickupLng:      o.PickupLng,
		DropoffAddress: o.DropoffAddress,
		DropoffLat:     o.DropoffLat,
		DropoffLng:     o.DropoffLng,
		DistanceKm:     o.DistanceKm,
		EstimatedFare:  o.EstimatedFare,
		PaymentMethod:  o.PaymentMethod,
		CreatedAt:      o.CreatedAt,
	}
}

// fromActiveFood memetakan ActiveFoodOrder ke DTO order aktif (TD-077 A2).
func fromActiveFood(o ActiveFoodOrder) DriverActiveOrder {
	return DriverActiveOrder{
		Type:            OrderTypeFood,
		OrderID:         o.ID,
		Status:          o.Status,
		DriverID:        o.DriverID,
		MerchantName:    o.MerchantName,
		MerchantAddress: o.MerchantAddress,
		MerchantLat:     o.MerchantLat,
		MerchantLng:     o.MerchantLng,
		DeliveryAddress: o.DeliveryAddress,
		DeliveryLat:     o.DeliveryLat,
		DeliveryLng:     o.DeliveryLng,
		DeliveryFee:     o.DeliveryFee,
		TotalAmount:     o.TotalAmount,
		PaymentMethod:   o.PaymentMethod,
		CreatedAt:       o.CreatedAt,
	}
}

// fromActiveSend memetakan ActiveSendOrder ke DTO order aktif (TD-077 A2).
func fromActiveSend(o ActiveSendOrder) DriverActiveOrder {
	order := DriverActiveOrder{
		Type:          OrderTypeSend,
		OrderID:       o.ID,
		Status:        o.Status,
		DriverID:      o.DriverID,
		PickupAddress: o.PickupAddress,
		PickupLat:     o.PickupLat,
		PickupLng:     o.PickupLng,
		DistanceKm:    o.DistanceKm,
		TotalFare:     o.TotalFare,
		PaymentMethod: o.PaymentMethod,
		CreatedAt:     o.CreatedAt,
	}
	if o.FirstStopAddress != nil {
		order.FirstStopAddress = *o.FirstStopAddress
	}
	return order
}

// validLatLng memastikan koordinat berada pada rentang geografis valid.
func validLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}
