// Package send — Service Layer (Phase 3, G-Send).
//
// Service menangani business logic modul G-Send Task 3.5 (ROADMAP 3.5.1):
//   - CreateSendOrder (POST /send-orders): dual-layer idempotency (Redis L1 +
//     PostgreSQL L2 idempotency_cache), validasi (Debt Gate Universal
//     overdue_debt, stops 1-3, weight > 0), kalkulasi fare (base_fare 15.000 +
//     distance_km * 3.000 + weight_surcharge >5kg + insurance_fee), alokasi
//     ongkos multi-stop proporsional per jarak, escrow WALLET (DEBIT customer /
//     CREDIT SYSTEM_ESCROW) dalam satu transaksi dengan insert send_orders +
//     send_order_stops, dan trigger driver matching (async placeholder).
//   - GetSendOrder (GET /send-orders/{id}): detail order + stops dengan
//     pemeriksaan kepemilikan (sender / driver tertunjuk).
//   - UpdateSendOrderStatus (PATCH /send-orders/{id}): transisi status per
//     aktor dengan FOR UPDATE NOWAIT + audit trail send_order_events; cancel
//     WALLET → refund escrow penuh (SEND_REFUND).
//
// Prinsip yang selaras dengan modul food/wallet:
//   - Otorisasi ownership order divalidasi di service, bukan hanya via RBAC role.
//   - Operasi bertransaksi memakai SATU transaksi DB agar order + stops +
//     escrow atomik (tanpa partial state).
//   - Semua query parameterized (tanpa interpolasi string).
//   - Idempotency key wajib & response di-cache (L1 + L2).
package send

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/g-flow/g-flow/internal/wallet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// Constanta domain & bisnis G-Send (ROADMAP 3.5.1 / MIGRATION 005).
const (
	walletTypeCustomer       = "CUSTOMER"
	walletTypeDriver         = "DRIVER"
	walletTypeSystemEscrow   = "SYSTEM_ESCROW"
	walletTypeSystemPlatform = "SYSTEM_PLATFORM"
	userTypeCustomer         = "customer"
	userTypeDriver           = "driver"
	statusActive             = "ACTIVE"

	// Payment method (payment_method_enum).
	PaymentMethodWallet = "WALLET"
	PaymentMethodCash   = "CASH"

	// Status send order (send_order_status_enum).
	sendStatusCreated         = "CREATED"
	sendStatusSearchingDriver = "SEARCHING_DRIVER"
	sendStatusDriverAssigned  = "DRIVER_ASSIGNED"
	sendStatusPickedUp        = "PICKED_UP"
	sendStatusInTransit       = "IN_TRANSIT"
	sendStatusDelivered       = "DELIVERED"
	sendStatusCancelled       = "CANCELLED"
	sendStatusSettled         = "SETTLED"

	// Stop status (send_order_stops.status — CHECK constraint DB, TD-015,
	// diselaraskan dengan ROADMAP 03 3.5.1/3.6.3). Hanya 4 nilai:
	//   PENDING → PICKED_UP → DELIVERED ; CANCELLED
	// Partial stop cancellation TIDAK didukung di MVP (pembatalan seluruh
	// order). Endpoint menerima "DELIVERED" (dan legacy "COMPLETED") yang
	// dinormalisasi menjadi DELIVERED; "PICKED_UP"/legacy "ARRIVED";
	// "CANCELLED"/legacy "SKIPPED".
	stopStatusPending   = "PENDING"
	stopStatusPickedUp  = "PICKED_UP"
	stopStatusDelivered = "DELIVERED"
	stopStatusCancelled = "CANCELLED"

	// Working status driver (users.working_status).
	workingStatusIdle = "IDLE"
	workingStatusBusy = "BUSY"

	// Capacity driver (Task 3.6.1): maksimal 3 order aktif serentak.
	driverMaxActiveOrders = 3

	// Idempotency dual-layer (L1 Redis + L2 idempotency_cache).
	redisKeyPrefix = "idempotency:"
	redisCompleted = "COMPLETED"
	pgProcessing   = "PROCESSING"
	redisTTL       = 24 * time.Hour

	// Reference type double-entry ledger untuk escrow & refund G-Send.
	referenceTypeSendEscrow = "SEND_ESCROW"
	referenceTypeSendRefund = "SEND_REFUND"
	referenceTypeSendSettle = "SEND_SETTLEMENT"

	// Estimasi pickup (ROADMAP 3.5.1 response).
	sendEstimatedPickup = "5 minutes"
)

// Tarif G-Send (ROADMAP 3.5.1 langkah 3):
//   - base_fare: Rp 15.000
//   - rate per km: Rp 3.000
//   - weight_surcharge: jika weight_kg > 5, +Rp 2.000 per kg KELEBIHAN
//   - insurance_fee: jika declared_value > Rp 100.000, 1% dari declared_value
//     (maks Rp 50.000)
var (
	sendBaseFare             = decimal.NewFromInt(15000)
	sendPerKmRate            = decimal.NewFromInt(3000)
	weightSurchargeThreshold = decimal.NewFromInt(5)
	weightSurchargePerKg     = decimal.NewFromInt(2000)
	insuranceThreshold       = decimal.NewFromInt(100000)
	insuranceRate            = decimal.RequireFromString("0.01")
	insuranceMax             = decimal.NewFromInt(50000)

	// Revenue split settlement 3-way (ROADMAP 3.6.4): driver 90%, platform 10%.
	sendPlatformCommissionRate = decimal.RequireFromString("0.10")

	// Ceiling saldo negatif driver (LOGIC_FLOW 5.5): setelah CASH settlement,
	// jika balance < -Rp 50.000 → driver di-SUSPENDED.
	sendMaxDriverNegativeBalance = decimal.NewFromInt(-50000)
)

// Error definitions untuk service layer send.
var (
	ErrNotCustomer            = errors.New("user is not a customer")
	ErrCustomerInactive       = errors.New("customer is not ACTIVE")
	ErrOverdueDebt            = errors.New("customer has overdue debt (order rejected)")
	ErrWalletInactive         = errors.New("customer wallet is not ACTIVE")
	ErrInsufficientBalance    = errors.New("insufficient customer wallet balance for escrow")
	ErrInvalidPaymentMethod   = errors.New("invalid payment method (must be WALLET or CASH)")
	ErrInvalidPackageType     = errors.New("invalid package type (STANDARD, FRAGILE, LIQUID, ELECTRONICS)")
	ErrInvalidWeight          = errors.New("package weight must be greater than zero")
	ErrInvalidDeclaredValue   = errors.New("declared value must not be negative")
	ErrInvalidStopsCount      = errors.New("stops must contain 1-3 delivery stops")
	ErrInvalidRecipient       = errors.New("delivery stop must have an address and valid coordinates")
	ErrInvalidCoordinates     = errors.New("invalid coordinates (latitude/longitude)")
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	ErrIdempotencyInProgress  = errors.New("idempotency key is still processing")
	ErrInvalidCachedResponse  = errors.New("cached idempotency response is invalid")
	ErrNotAllowed             = errors.New("user is not allowed to access this order")
	ErrInvalidStatus          = errors.New("invalid status value")
	ErrInvalidTransition      = errors.New("invalid status transition for current order state")
	ErrDriverNotFound         = errors.New("driver not found or has no DRIVER wallet")

	// Task 3.6 — accept order & driver capacity.
	ErrNotDriver                 = errors.New("user is not a driver")
	ErrDriverInactive            = errors.New("driver is not ACTIVE")
	ErrDriverBusy                = errors.New("driver is busy (working_status not IDLE)")
	ErrInsufficientDriverBalance = errors.New("insufficient driver balance (below min_balance_threshold)")
	ErrDriverCapacityExceeded    = errors.New("driver has reached maximum 3 active orders")
	ErrOrderNotSearching         = errors.New("order is not in SEARCHING_DRIVER state")
	ErrStopsNotDelivered         = errors.New("all stops must be DELIVERED before main order is DELIVERED")
	ErrStopNotFound              = errors.New("send order stop not found")
	ErrInvalidStopStatus         = errors.New("invalid stop status value")
	ErrStopAlreadyDelivered      = errors.New("stop is already DELIVERED")
)

// Repo adalah kontrak repository yang dibutuhkan Service. Dipenuhi oleh
// *Repository (internal/send/repository.go); dijadikan interface agar mudah
// di-mock pada unit test.
type Repo interface {
	GetCustomer(ctx context.Context, userID uuid.UUID) (*SendCustomer, error)
	GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*SendWallet, error)
	SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error)

	InsertSendOrder(ctx context.Context, q Querier, o *SendOrder) error
	InsertSendOrderStops(ctx context.Context, q Querier, stops []*SendOrderStop) error
	InsertSendOrderEvent(ctx context.Context, q Querier, e SendOrderEvent) error

	GetSendOrderByID(ctx context.Context, orderID uuid.UUID) (*SendOrder, error)
	LockSendOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*SendOrder, error)
	UpdateSendOrderStatus(ctx context.Context, q Querier, orderID uuid.UUID,
		fromStatus, toStatus string) (bool, error)
	GetSendOrderStops(ctx context.Context, orderID uuid.UUID) ([]*SendOrderStop, error)
	GetSendOrdersBySender(ctx context.Context, senderID uuid.UUID, limit, offset int) ([]*SendOrder, error)
	CountSendOrdersBySender(ctx context.Context, senderID uuid.UUID) (int, error)

	// Task 3.6 — accept order, status machine lanjutan & settlement.
	GetDriver(ctx context.Context, driverID uuid.UUID) (*SendDriver, error)
	GetDriverActiveSendOrdersCount(ctx context.Context, driverID uuid.UUID) (int, error)
	LockSendOrderForAccept(ctx context.Context, q Querier, orderID uuid.UUID) (*SendOrder, error)
	LockDriverUserForAccept(ctx context.Context, q Querier, driverID uuid.UUID) error
	AssignDriverToSendOrder(ctx context.Context, q Querier, orderID uuid.UUID, driverID uuid.UUID) (bool, error)
	UpdateDriverWorkingStatus(ctx context.Context, q Querier, driverID uuid.UUID, status string) error
	MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error
	MarkSendOrderDelivered(ctx context.Context, q Querier, orderID uuid.UUID) (bool, error)
	MarkSendOrderSettled(ctx context.Context, q Querier, orderID uuid.UUID) error
	GetSendOrderStopsByOrderID(ctx context.Context, q Querier, orderID uuid.UUID) ([]*SendOrderStop, error)
	UpdateSendOrderStopStatus(ctx context.Context, q Querier, stopID uuid.UUID, status string, proof *string) (bool, error)
}

// Ledger adalah kontrak double-entry ledger yang dibutuhkan Service.
// Dipenuhi oleh *wallet.LedgerService (internal/wallet/ledger.go).
type Ledger interface {
	CreateLedgerEntries(ctx context.Context, tx pgx.Tx, entries []wallet.LedgerEntry) error
}

// DB adalah subset operasi pool yang dipakai Service. Dipenuhi oleh
// *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Service adalah business logic untuk modul G-Send (order creation & pricing).
type Service struct {
	repo   Repo
	db     DB
	redis  *redis.Client
	ledger Ledger
}

// NewService membuat Service baru dengan dependency injection.
func NewService(repo Repo, db DB, rdb *redis.Client, ledger Ledger) *Service {
	return &Service{repo: repo, db: db, redis: rdb, ledger: ledger}
}

// ---- Task 3.5: Create Send Order (ROADMAP 3.5.1) ----

// SendStopRequest satu stop pengiriman pada request POST /send-orders.
type SendStopRequest struct {
	RecipientName   string
	RecipientPhone  string
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
}

// CreateSendOrderRequest input untuk POST /send-orders.
type CreateSendOrderRequest struct {
	UserID              uuid.UUID
	PickupLat           float64
	PickupLng           float64
	PickupAddress       string
	PackageWeightKg     float64
	PackageDimensionsCm string
	PackageDescription  string
	PackageType         string
	DeclaredValue       decimal.Decimal
	PaymentMethod       string
	VoucherID           *uuid.UUID
	Stops               []SendStopRequest
	IdempotencyKey      string
}

// SendStopResponse satu baris breakdown fare per stop.
type SendStopResponse struct {
	StopNumber    int             `json:"stop_order"`
	RecipientName string          `json:"recipient_name"`
	Address       string          `json:"address"`
	DistanceKm    decimal.Decimal `json:"distance_km"`
	AllocatedFare decimal.Decimal `json:"allocated_fare"`
}

// CreateSendOrderResponse hasil POST /send-orders (ROADMAP 3.5.1 response +
// fare breakdown per stop sesuai API CONTRACT 9.1).
type CreateSendOrderResponse struct {
	ID                   uuid.UUID          `json:"id"`
	Status               string             `json:"status"`
	PackageType          string             `json:"package_type"`
	PaymentMethod        string             `json:"payment_method"`
	TotalDistanceKm      decimal.Decimal    `json:"total_distance_km"`
	BaseFare             decimal.Decimal    `json:"base_fare"`
	WeightSurcharge      decimal.Decimal    `json:"weight_surcharge"`
	InsuranceFee         decimal.Decimal    `json:"insurance_fee"`
	DiscountAmount       decimal.Decimal    `json:"discount_amount"`
	TotalFare            decimal.Decimal    `json:"total_fare"`
	EscrowAmount         decimal.Decimal    `json:"escrow_amount"`
	StopsCount           int                `json:"stops_count"`
	FareBreakdownPerStop []SendStopResponse `json:"fare_breakdown_per_stop"`
	EstimatedPickupTime  string             `json:"estimated_pickup_time"`
}

// stopDistance adalah hasil perhitungan jarak per stop.
type stopDistance struct {
	index int
	km    decimal.Decimal
}

// CreateSendOrder membuat send order paket (ROADMAP 3.5.1):
//
//  1. Validasi input (idempotency key, payment, package, stops 1-3, koordinat).
//  2. Idempotency L1 (Redis) → L2 (PostgreSQL idempotency_cache).
//  3. Validasi customer + Debt Gate Universal (overdue_debt = 0) — berlaku
//     untuk WALLET maupun CASH.
//  4. Hitung jarak total (haversine: pickup → stop1 → stop2 → ...).
//  5. Hitung fare: base_fare 15.000 + distance_km * 3.000 + weight_surcharge
//     (>5kg: 2.000/kg kelebihan) + insurance_fee (declared_value > 100.000:
//     1% maks 50.000); discount (voucher) belum didukung.
//  6. Alokasi ongkos multi-stop proporsional per jarak (allocated_fare; stop
//     terakhir mengambil sisa agar SUM(allocated) == total_fare).
//  7. Satu transaksi: INSERT send_orders (+stops, +event audit) → (WALLET)
//     escrow dengan lock wallet ORDER BY id ASC & double-entry ledger
//     SEND_ESCROW (DEBIT sender / CREDIT SYSTEM_ESCROW). CASH tanpa escrow.
//  8. Trigger driver matching (async) — placeholder untuk sekarang.
//  9. Cache response (L1 + L2 COMPLETED).
func (s *Service) CreateSendOrder(ctx context.Context, req CreateSendOrderRequest) (*CreateSendOrderResponse, error) {
	if err := s.validateSendOrderInput(req); err != nil {
		return nil, err
	}

	// L1: Redis (scope per user agar tidak collision antar user).
	if resp, ok := s.redisGetCachedResp(ctx, req.UserID, req.IdempotencyKey); ok {
		var out CreateSendOrderResponse
		if err := json.Unmarshal(resp, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Validasi customer & Debt Gate Universal.
	cust, err := s.repo.GetCustomer(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if cust.UserType != userTypeCustomer {
		return nil, ErrNotCustomer
	}
	if cust.Status != statusActive {
		return nil, ErrCustomerInactive
	}
	if cust.OverdueDebt.IsPositive() {
		return nil, ErrOverdueDebt
	}

	// Wallet customer (untuk sender_wallet_id & escrow WALLET).
	customerWallet, err := s.repo.GetWalletByUserAndType(ctx, req.UserID, walletTypeCustomer)
	if err != nil {
		return nil, err
	}
	if customerWallet.Status != statusActive {
		return nil, ErrWalletInactive
	}

	// L2: PostgreSQL idempotency.
	if res, err := s.idemAcquire(ctx, req.UserID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if !res.proceed {
		s.redisSet(ctx, req.UserID, req.IdempotencyKey, redisCompleted, res.cached)
		var out CreateSendOrderResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Hitung jarak & alokasikan ke stop.
	totalDistance, stopDistances := s.calculateDistances(req)
	baseFare, _, weightSurcharge, insuranceFee, totalFare := s.calculateFare(req, totalDistance)
	allocated := s.allocateFares(totalFare, stopDistances)

	orderID := uuid.New()
	senderWalletID := customerWallet.ID

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Buat send_orders (status SEARCHING_DRIVER) dalam transaksi.
	dimensions, description := strPtrOrNil(req.PackageDimensionsCm), strPtrOrNil(req.PackageDescription)
	order := &SendOrder{
		ID:                  orderID,
		SenderID:            req.UserID,
		SenderWalletID:      &senderWalletID,
		PackageWeightKg:     decimal.NewFromFloat(req.PackageWeightKg),
		PackageDimensionsCm: dimensions,
		PackageDescription:  description,
		PickupLat:           decimal.NewFromFloat(req.PickupLat),
		PickupLng:           decimal.NewFromFloat(req.PickupLng),
		PickupAddress:       req.PickupAddress,
		BaseFare:            baseFare,
		DistanceKm:          totalDistance,
		WeightSurcharge:     weightSurcharge,
		TotalFare:           totalFare,
		DeclaredValue:       req.DeclaredValue,
		PackageType:         req.PackageType,
		InsuranceFee:        insuranceFee,
		DiscountAmount:      decimal.Zero, // voucher belum didukung (tabel vouchers menyusul)
		VoucherID:           req.VoucherID,
		PaymentMethod:       req.PaymentMethod,
		Status:              sendStatusSearchingDriver,
	}
	if err := s.repo.InsertSendOrder(ctx, tx, order); err != nil {
		return nil, err
	}

	// Insert stops (multi-stop) dalam transaksi yang sama.
	stops := make([]*SendOrderStop, 0, len(req.Stops))
	for i, st := range req.Stops {
		alloc := allocated[i]
		dist := stopDistances[i].km
		stops = append(stops, &SendOrderStop{
			ID:             uuid.New(),
			OrderID:        orderID,
			StopNumber:     i + 1,
			RecipientName:  strPtrOrNil(st.RecipientName),
			RecipientPhone: strPtrOrNil(st.RecipientPhone),
			DropoffLat:     decimal.NewFromFloat(st.DeliveryLat),
			DropoffLng:     decimal.NewFromFloat(st.DeliveryLng),
			DropoffAddress: st.DeliveryAddress,
			DistanceKm:     &dist,
			AllocatedFare:  &alloc,
			Status:         stopStatusPending,
		})
	}
	if err := s.repo.InsertSendOrderStops(ctx, tx, stops); err != nil {
		return nil, err
	}

	// Escrow conditional: hanya payment WALLET.
	if req.PaymentMethod == PaymentMethodWallet {
		if err := s.holdSendEscrow(ctx, tx, order, totalFare, customerWallet.ID); err != nil {
			return nil, err
		}
	}

	// Audit trail pembuatan order (CREATED → SEARCHING_DRIVER).
	from := sendStatusCreated
	if err := s.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
		OrderID:     orderID,
		FromStatus:  &from,
		ToStatus:    sendStatusSearchingDriver,
		TriggeredBy: &req.UserID,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Trigger driver matching (async) — placeholder untuk sekarang
	// (implementasi penuh di Task 3.6: query driver dalam radius 5km).
	s.triggerDriverMatching(ctx, orderID)

	escrowAmount := decimal.Zero
	if req.PaymentMethod == PaymentMethodWallet {
		escrowAmount = totalFare
	}

	response := &CreateSendOrderResponse{
		ID:                   orderID,
		Status:               sendStatusSearchingDriver,
		PackageType:          req.PackageType,
		PaymentMethod:        req.PaymentMethod,
		TotalDistanceKm:      totalDistance,
		BaseFare:             baseFare,
		WeightSurcharge:      weightSurcharge,
		InsuranceFee:         insuranceFee,
		DiscountAmount:       decimal.Zero,
		TotalFare:            totalFare,
		EscrowAmount:         escrowAmount,
		StopsCount:           len(stops),
		FareBreakdownPerStop: toStopResponses(req.Stops, stopDistances, allocated),
		EstimatedPickupTime:  sendEstimatedPickup,
	}
	if err := s.cacheResponse(ctx, req.UserID, req.IdempotencyKey, response); err != nil {
		return nil, err
	}
	return response, nil
}

// ---- Task 3.6: Accept Order (ROADMAP 3.6.1) ----

// AcceptSendOrderRequest input untuk POST /send-orders/{id}/accept.
type AcceptSendOrderRequest struct {
	OrderID        uuid.UUID
	DriverID       uuid.UUID
	IdempotencyKey string
}

// AcceptSendOrderStop satu baris stop pada response accept (ROADMAP 3.6.1
// response: stop_sequence, address, allocated_fare).
type AcceptSendOrderStop struct {
	StopSequence  int             `json:"stop_sequence"`
	Address       string          `json:"address"`
	AllocatedFare decimal.Decimal `json:"allocated_fare"`
}

// AcceptSendOrderResponse hasil POST /send-orders/{id}/accept.
type AcceptSendOrderResponse struct {
	ID     uuid.UUID             `json:"id"`
	Status string                `json:"status"`
	Stops  []AcceptSendOrderStop `json:"stops"`
}

// AcceptSendOrder menetapkan driver ke send order (SEARCHING_DRIVER →
// DRIVER_ASSIGNED) dengan dual-layer idempotency (ROADMAP 3.6.1):
//
//  1. Idempotency L1 (Redis) + L2 (PostgreSQL idempotency_cache).
//  2. Validasi driver: user_type='driver', status='ACTIVE',
//     working_status='IDLE', min_balance_threshold <= balance driver.
//  3. Capacity check: jumlah order aktif (DRIVER_ASSIGNED/PICKED_UP/IN_TRANSIT)
//     harus < 3 (max 3 order serentak).
//  4. Transaksi: lock driver user + order FOR UPDATE NOWAIT, atomic assign
//     (driver_id, driver_wallet_id, status DRIVER_ASSIGNED, assigned_at),
//     users.working_status → BUSY, audit send_order_events.
//  5. Cache response (L1 + L2 COMPLETED).
func (s *Service) AcceptSendOrder(ctx context.Context, req AcceptSendOrderRequest) (*AcceptSendOrderResponse, error) {
	if req.IdempotencyKey == "" {
		return nil, ErrIdempotencyKeyRequired
	}

	// L1: Redis idempotency.
	if resp, ok := s.redisGetCachedResp(ctx, req.DriverID, req.IdempotencyKey); ok {
		var out AcceptSendOrderResponse
		if err := json.Unmarshal(resp, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Validasi driver (status, working_status, threshold).
	driver, err := s.repo.GetDriver(ctx, req.DriverID)
	if err != nil {
		return nil, err
	}
	if driver.UserType != userTypeDriver {
		return nil, ErrNotDriver
	}
	if driver.Status != statusActive {
		return nil, ErrDriverInactive
	}
	if driver.WorkingStatus != workingStatusIdle {
		return nil, ErrDriverBusy
	}

	driverWallet, err := s.repo.GetWalletByUserAndType(ctx, req.DriverID, walletTypeDriver)
	if err != nil {
		return nil, err
	}
	if driverWallet.Status != statusActive {
		return nil, ErrWalletInactive
	}
	if driverWallet.Balance.LessThan(driver.MinBalanceThreshold) {
		return nil, ErrInsufficientDriverBalance
	}

	// Capacity check: max 3 order aktif per driver.
	active, err := s.repo.GetDriverActiveSendOrdersCount(ctx, req.DriverID)
	if err != nil {
		return nil, err
	}
	if active >= driverMaxActiveOrders {
		return nil, ErrDriverCapacityExceeded
	}

	// L2: PostgreSQL idempotency.
	if res, err := s.idemAcquire(ctx, req.DriverID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if !res.proceed {
		s.redisSet(ctx, req.DriverID, req.IdempotencyKey, redisCompleted, res.cached)
		var out AcceptSendOrderResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Lock driver user terlebih dahulu (guard working_status IDLE atomik),
	// lalu lock order FOR UPDATE NOWAIT + guard status SEARCHING_DRIVER.
	if err := s.lockDriverForAccept(ctx, tx, req.DriverID); err != nil {
		return nil, err
	}
	if _, err := s.repo.LockSendOrderForAccept(ctx, tx, req.OrderID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrLockTimeout
		}
		if errors.Is(err, ErrSendOrderNotFound) {
			if _, gErr := s.repo.GetSendOrderByID(ctx, req.OrderID); gErr != nil {
				return nil, gErr
			}
			return nil, ErrOrderNotSearching
		}
		return nil, err
	}

	// Atomic assign: order → DRIVER_ASSIGNED + driver data.
	ok, err := s.repo.AssignDriverToSendOrder(ctx, tx, req.OrderID, req.DriverID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrOrderNotSearching
	}

	// Driver working_status → BUSY.
	if err := s.repo.UpdateDriverWorkingStatus(ctx, tx, req.DriverID, workingStatusBusy); err != nil {
		return nil, err
	}

	// Audit trail SEARCHING_DRIVER → DRIVER_ASSIGNED.
	from := sendStatusSearchingDriver
	if err := s.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
		OrderID:     req.OrderID,
		FromStatus:  &from,
		ToStatus:    sendStatusDriverAssigned,
		TriggeredBy: &req.DriverID,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	stops, err := s.repo.GetSendOrderStops(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}
	resp := &AcceptSendOrderResponse{
		ID:     req.OrderID,
		Status: sendStatusDriverAssigned,
	}
	for _, st := range stops {
		resp.Stops = append(resp.Stops, AcceptSendOrderStop{
			StopSequence:  st.StopNumber,
			Address:       st.DropoffAddress,
			AllocatedFare: derefDecimal(st.AllocatedFare),
		})
	}

	if err := s.cacheResponse(ctx, req.DriverID, req.IdempotencyKey, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// lockDriverForAccept mengunci baris users driver dengan FOR UPDATE NOWAIT
// sekaligus guard atomik status ACTIVE + working_status IDLE. Mengembalikan
// ErrLockTimeout (55P03) bila lock tidak tersedia.
func (s *Service) lockDriverForAccept(ctx context.Context, tx pgx.Tx, driverID uuid.UUID) error {
	err := s.repo.LockDriverUserForAccept(ctx, tx, driverID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return ErrLockTimeout
	}
	return err
}

// derefDecimal mengembalikan nilai decimal dari pointer (nil → decimal.Zero).
func derefDecimal(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}

// calculateDistances menghitung jarak antar titik untuk tiap stop:
// pickup → stop1 → stop2 → ... (rumus haversine). Mengembalikan total jarak
// dan jarak per stop.
func (s *Service) calculateDistances(req CreateSendOrderRequest) (decimal.Decimal, []stopDistance) {
	prevLat, prevLng := req.PickupLat, req.PickupLng
	total := decimal.Zero
	out := make([]stopDistance, 0, len(req.Stops))
	for i, st := range req.Stops {
		km := haversineKm(prevLat, prevLng, st.DeliveryLat, st.DeliveryLng).Round(3)
		total = total.Add(km)
		prevLat, prevLng = st.DeliveryLat, st.DeliveryLng
		out = append(out, stopDistance{index: i, km: km})
	}
	return total.Round(3), out
}

// calculateFare menghitung komponen fare G-Send (ROADMAP 3.5.1 langkah 3):
// total = base_fare (15.000) + distance_km * 3.000 + weight_surcharge +
// insurance_fee. Discount (voucher) belum didukung (0).
func (s *Service) calculateFare(req CreateSendOrderRequest, distance decimal.Decimal) (
	base, distanceCharge, weightSurcharge, insuranceFee, total decimal.Decimal) {
	base = sendBaseFare
	distanceCharge = distance.Mul(sendPerKmRate).Round(2)

	weight := decimal.NewFromFloat(req.PackageWeightKg)
	if weight.GreaterThan(weightSurchargeThreshold) {
		excess := weight.Sub(weightSurchargeThreshold)
		weightSurcharge = excess.Mul(weightSurchargePerKg).Round(2)
	}

	if req.DeclaredValue.GreaterThan(insuranceThreshold) {
		insuranceFee = req.DeclaredValue.Mul(insuranceRate).Round(2)
		if insuranceFee.GreaterThan(insuranceMax) {
			insuranceFee = insuranceMax
		}
	}

	total = base.Add(distanceCharge).Add(weightSurcharge).Add(insuranceFee).Round(2)
	return base, distanceCharge, weightSurcharge, insuranceFee, total
}

// allocateFares mengalokasikan total_fare ke tiap stop secara proporsional
// per jarak (ROADMAP 3.5.1 langkah 4): stop terakhir mengambil sisa agar
// SUM(allocated_fare) == total_fare persis (double rounding aman). Jika total
// jarak nol (degenerate), seluruh fare dialokasikan ke stop terakhir.
func (s *Service) allocateFares(totalFare decimal.Decimal, stopDistances []stopDistance) []decimal.Decimal {
	n := len(stopDistances)
	out := make([]decimal.Decimal, n)
	if n == 0 {
		return out
	}

	totalDist := decimal.Zero
	for _, sd := range stopDistances {
		totalDist = totalDist.Add(sd.km)
	}

	if totalDist.IsZero() {
		out[n-1] = totalFare
		return out
	}

	remaining := totalFare
	for i, sd := range stopDistances {
		if i == n-1 {
			out[i] = remaining
			continue
		}
		alloc := totalFare.Mul(sd.km.Div(totalDist)).Round(2)
		out[i] = alloc
		remaining = remaining.Sub(alloc)
	}
	// Jaga agar stop terakhir tidak negatif akibat pembulatan.
	if remaining.IsNegative() {
		out[n-1] = decimal.Zero
	}
	return out
}

// holdSendEscrow memegang dana customer ke SYSTEM_ESCROW saat order WALLET.
// Urutan lock (LOCKED): baris order sudah ditulis → lock wallets
// ORDER BY id ASC FOR UPDATE → cek saldo → ledger double-entry SEND_ESCROW:
//   - DEBIT sender_wallet
//   - CREDIT SYSTEM_ESCROW
func (s *Service) holdSendEscrow(ctx context.Context, tx pgx.Tx, order *SendOrder, amount decimal.Decimal, senderWalletID uuid.UUID) error {
	escrowID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemEscrow)
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, senderWalletID, escrowID); err != nil {
		return err
	}
	balance, err := getWalletBalanceTx(ctx, tx, senderWalletID)
	if err != nil {
		return err
	}
	if balance.LessThan(amount) {
		return ErrInsufficientBalance
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      senderWalletID,
			EntryType:     wallet.EntryDebit,
			Amount:        amount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeSendEscrow,
			Description:   "SEND_ESCROW - DEBIT sender wallet",
		},
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryCredit,
			Amount:        amount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeSendEscrow,
			Description:   "SEND_ESCROW - CREDIT SYSTEM_ESCROW",
		},
	})
}

// refundSendEscrow mengembalikan dana escrow penuh ke sender saat order
// WALLET dibatalkan (CREATED/SEARCHING_DRIVER). Pasangan entry SEND_REFUND
// membuat total DEBIT == CREDIT per reference_id tetap seimbang. Dipanggil
// dalam transaksi yang sama dengan transisi status.
func (s *Service) refundSendEscrow(ctx context.Context, tx pgx.Tx, order *SendOrder, senderWalletID uuid.UUID) error {
	escrowID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemEscrow)
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, senderWalletID, escrowID); err != nil {
		return err
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        order.TotalFare,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeSendRefund,
			Description:   "SEND_REFUND - escrow release to sender",
		},
		{
			WalletID:      senderWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        order.TotalFare,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeSendRefund,
			Description:   "SEND_REFUND - full refund sender wallet",
		},
	})
}

// validateSendOrderInput memvalidasi input POST /send-orders (ROADMAP 3.5.1
// langkah 1).
func (s *Service) validateSendOrderInput(req CreateSendOrderRequest) error {
	if req.IdempotencyKey == "" {
		return ErrIdempotencyKeyRequired
	}
	if req.PaymentMethod != PaymentMethodWallet && req.PaymentMethod != PaymentMethodCash {
		return ErrInvalidPaymentMethod
	}
	if !validPackageType(req.PackageType) {
		return ErrInvalidPackageType
	}
	if req.PackageWeightKg <= 0 {
		return ErrInvalidWeight
	}
	if req.DeclaredValue.IsNegative() {
		return ErrInvalidDeclaredValue
	}
	if len(req.Stops) < 1 || len(req.Stops) > 3 {
		return ErrInvalidStopsCount
	}
	if !validLatLng(req.PickupLat, req.PickupLng) {
		return ErrInvalidCoordinates
	}
	for _, st := range req.Stops {
		if st.DeliveryAddress == "" {
			return ErrInvalidRecipient
		}
		if !validLatLng(st.DeliveryLat, st.DeliveryLng) {
			return ErrInvalidRecipient
		}
	}
	return nil
}

// validPackageType memeriksa paket valid sesuai package_type_enum.
func validPackageType(t string) bool {
	switch t {
	case "STANDARD", "FRAGILE", "LIQUID", "ELECTRONICS":
		return true
	default:
		return false
	}
}

// toStopResponses menyusun breakdown fare per stop untuk response.
func toStopResponses(req []SendStopRequest, dists []stopDistance, allocs []decimal.Decimal) []SendStopResponse {
	out := make([]SendStopResponse, 0, len(req))
	for i, st := range req {
		out = append(out, SendStopResponse{
			StopNumber:    i + 1,
			RecipientName: st.RecipientName,
			Address:       st.DeliveryAddress,
			DistanceKm:    dists[i].km,
			AllocatedFare: allocs[i],
		})
	}
	return out
}

// ---- Task 3.5: Retrieval & Status Update ----

// SendOrderDetail adalah hasil GET /send-orders/{id}: order + stops.
type SendOrderDetail struct {
	Order *SendOrder
	Stops []*SendOrderStop
}

// GetSendOrder mengambil send order lengkap + stops dengan pemeriksaan
// kepemilikan: sender pemilik atau driver tertunjuk.
func (s *Service) GetSendOrder(ctx context.Context, orderID, userID uuid.UUID) (*SendOrderDetail, error) {
	order, err := s.repo.GetSendOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.SenderID != userID && (order.DriverID == nil || *order.DriverID != userID) {
		return nil, ErrNotAllowed
	}
	stops, err := s.repo.GetSendOrderStops(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return &SendOrderDetail{Order: order, Stops: stops}, nil
}

// GetSendOrderHistory mengembalikan riwayat send order milik sender dengan
// pagination (page mulai 1, page_size 1..50, default 20), urut created_at DESC.
func (s *Service) GetSendOrderHistory(ctx context.Context, senderID uuid.UUID, page, pageSize int) ([]*SendOrder, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	total, err := s.repo.CountSendOrdersBySender(ctx, senderID)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.repo.GetSendOrdersBySender(ctx, senderID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// Actor kinds pada send order.
const (
	actorKindSender = iota
	actorKindDriver
)

// UpdateSendOrderStatusRequest input untuk PATCH /send-orders/{id}.
type UpdateSendOrderStatusRequest struct {
	OrderID uuid.UUID
	UserID  uuid.UUID
	Status  string
	Reason  string
}

// UpdateSendOrderStatusResponse hasil PATCH /send-orders/{id}.
type UpdateSendOrderStatusResponse struct {
	OrderID uuid.UUID `json:"order_id"`
	Status  string    `json:"status"`
}

// UpdateSendOrderStatus memproses PATCH /send-orders/{id} dengan FOR UPDATE
// NOWAIT (Task 3.5 + 3.6):
//
//   - Sender (pemilik order): CREATED/SEARCHING_DRIVER → CANCELLED, dengan
//     refund escrow penuh jika payment WALLET (SEND_REFUND).
//   - Driver tertunjuk: DRIVER_ASSIGNED → PICKED_UP → IN_TRANSIT → DELIVERED.
//     Transisi ke DELIVERED divalidasi semua stops COMPLETED lalu memicu
//     3-way settlement dalam transaksi yang sama (DELIVERED → SETTLED).
//   - CANCELLED oleh driver (emergency, dari DRIVER_ASSIGNED/PICKED_UP/
//     IN_TRANSIT): refund escrow penuh jika WALLET + reset working_status IDLE.
func (s *Service) UpdateSendOrderStatus(ctx context.Context, req UpdateSendOrderStatusRequest) (*UpdateSendOrderStatusResponse, error) {
	if !validSendStatusTarget(req.Status) {
		return nil, ErrInvalidStatus
	}

	order, err := s.repo.GetSendOrderByID(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}
	if order.Status == sendStatusCancelled || order.Status == sendStatusSettled {
		return nil, ErrInvalidTransition
	}

	// Tentukan aktor: sender pemilik atau driver tertunjuk.
	actor := actorKindSender
	switch {
	case order.SenderID == req.UserID:
		actor = actorKindSender
	case order.DriverID != nil && *order.DriverID == req.UserID:
		actor = actorKindDriver
	default:
		return nil, ErrNotAllowed
	}

	if err := validateSendTransition(order, actor, req.Status); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// FOR UPDATE NOWAIT: dua transisi bersamaan tidak bisa saling menimpa.
	locked, err := s.repo.LockSendOrder(ctx, tx, req.OrderID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrLockTimeout
		}
		return nil, err
	}
	if locked.Status != order.Status {
		return nil, ErrInvalidTransition
	}

	switch req.Status {
	case sendStatusCancelled:
		return s.cancelSendOrderTx(ctx, tx, locked, req)
	case sendStatusDelivered:
		return s.deliverSendOrderTx(ctx, tx, locked, req)
	default:
		// PICKED_UP / IN_TRANSIT — transisi murni + audit event.
		from := locked.Status
		ok, err := s.repo.UpdateSendOrderStatus(ctx, tx, req.OrderID, locked.Status, req.Status)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidTransition
		}
		if err := s.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
			OrderID:     req.OrderID,
			FromStatus:  &from,
			ToStatus:    req.Status,
			TriggeredBy: &req.UserID,
		}); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &UpdateSendOrderStatusResponse{OrderID: req.OrderID, Status: req.Status}, nil
	}
}

// cancelSendOrderTx meng-cancel send order dalam transaksi yang sudah mengunci
// baris order: set status CANCELLED, refund escrow penuh (WALLET + belum
// SETTLED), reset working_status driver ke IDLE (jika sudah ditunjuk), dan
// mencatat audit event dengan metadata reason.
func (s *Service) cancelSendOrderTx(ctx context.Context, tx pgx.Tx, order *SendOrder, req UpdateSendOrderStatusRequest) (*UpdateSendOrderStatusResponse, error) {
	reason := req.Reason
	if reason == "" {
		reason = "DRIVER_EMERGENCY_CANCEL"
		if order.DriverID == nil || req.UserID != *order.DriverID {
			reason = "SENDER_CANCEL"
		}
	}

	ok, err := s.repo.UpdateSendOrderStatus(ctx, tx, order.ID, order.Status, sendStatusCancelled)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidTransition
	}

	// Refund escrow penuh jika payment WALLET dan belum disettel.
	if order.PaymentMethod == PaymentMethodWallet && !order.IsSettled {
		if order.SenderWalletID == nil {
			return nil, ErrWalletNotFound
		}
		if err := s.refundSendEscrow(ctx, tx, order, *order.SenderWalletID); err != nil {
			return nil, err
		}
	}

	// Driver lepas dari order → kembali IDLE.
	if order.DriverID != nil {
		if err := s.repo.UpdateDriverWorkingStatus(ctx, tx, *order.DriverID, workingStatusIdle); err != nil {
			return nil, err
		}
	}

	event := SendOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &order.Status,
		ToStatus:    sendStatusCancelled,
		TriggeredBy: &req.UserID,
	}
	if reason != "" {
		event.Reason = strPtr(reason)
	}
	if md, err := json.Marshal(map[string]string{"reason": reason}); err == nil {
		event.Metadata = md
	}
	if err := s.repo.InsertSendOrderEvent(ctx, tx, event); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &UpdateSendOrderStatusResponse{OrderID: order.ID, Status: sendStatusCancelled}, nil
}

// deliverSendOrderTx menandai send order DELIVERED lalu memicu 3-way
// settlement otomatis dalam transaksi yang sama (Task 3.6.2c/3.6.4). Semua
// stop wajib COMPLETED sebelum order utama boleh DELIVERED.
func (s *Service) deliverSendOrderTx(ctx context.Context, tx pgx.Tx, order *SendOrder, req UpdateSendOrderStatusRequest) (*UpdateSendOrderStatusResponse, error) {
	stops, err := s.repo.GetSendOrderStopsByOrderID(ctx, tx, order.ID)
	if err != nil {
		return nil, err
	}
	for _, st := range stops {
		if st.Status != stopStatusDelivered {
			return nil, ErrStopsNotDelivered
		}
	}

	ok, err := s.repo.MarkSendOrderDelivered(ctx, tx, order.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidTransition
	}

	from := order.Status
	if err := s.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &from,
		ToStatus:    sendStatusDelivered,
		TriggeredBy: &req.UserID,
	}); err != nil {
		return nil, err
	}

	if err := s.settleSendOrderTx(ctx, tx, order); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &UpdateSendOrderStatusResponse{OrderID: order.ID, Status: sendStatusSettled}, nil
}

// ---- Task 3.6: 3-Way Settlement (ROADMAP 3.6.4) ----

// settleSendOrderTx melakukan 3-way settlement double-entry ledger saat send
// order bertransisi DELIVERED → SETTLED (dipanggil otomatis setelah driver
// menandai DELIVERED / semua stop COMPLETED). Locking wallet
// ORDER BY id ASC FOR UPDATE (deadlock-free, sesuai mandat Lock Hierarchy).
//
//	WALLET: DEBIT SYSTEM_ESCROW (total_fare) → CREDIT driver_wallet
//	        (total_fare * 0.90) + CREDIT SYSTEM_PLATFORM (total_fare * 0.10).
//	        Setelah itu driver kembali IDLE.
//	CASH  : customer bayar tunai ke driver; DEBIT driver_wallet
//	        (total_fare * 0.10 komisi platform) → CREDIT SYSTEM_PLATFORM.
//	        Jika balance driver < -Rp 50.000 → SUSPENDED (ceiling negatif),
//	        jika aman → working_status IDLE.
//
// Lalu menandai send_orders SETTLED (is_settled = TRUE, settled_at = NOW())
// dan menulis audit trail send_order_events (DELIVERED → SETTLED).
//
// Invariant double-entry: SUM(DEBIT) == SUM(CREDIT) per group — diverifikasi
// oleh wallet.LedgerService.CreateLedgerEntries.
func (s *Service) settleSendOrderTx(ctx context.Context, tx pgx.Tx, order *SendOrder) error {
	if order.DriverID == nil {
		return ErrDriverNotFound
	}
	driverWallet, err := s.repo.GetWalletByUserAndType(ctx, *order.DriverID, walletTypeDriver)
	if err != nil {
		return err
	}
	platformID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemPlatform)
	if err != nil {
		return err
	}

	commission := order.TotalFare.Mul(sendPlatformCommissionRate).Round(2)
	earning := order.TotalFare.Sub(commission).Round(2)

	switch order.PaymentMethod {
	case PaymentMethodCash:
		// Driver memegang kas customer; menyetor komisi platform (10%) ke
		// wallet digital. Saldo driver boleh negatif dengan ceiling -50.000.
		if err := lockWalletsAsc(ctx, tx, driverWallet.ID, platformID); err != nil {
			return err
		}
		if err := s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryDebit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeSendSettle,
				Description:   "SEND_SETTLEMENT - platform commission DEBIT driver (CASH)",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeSendSettle,
				Description:   "SEND_SETTLEMENT - platform commission CREDIT (CASH)",
			},
		}); err != nil {
			return err
		}
		balance, err := getWalletBalanceTx(ctx, tx, driverWallet.ID)
		if err != nil {
			return err
		}
		if balance.LessThan(sendMaxDriverNegativeBalance) {
			if err := s.repo.MarkDriverSuspended(ctx, tx, *order.DriverID); err != nil {
				return err
			}
		} else {
			if err := s.repo.UpdateDriverWorkingStatus(ctx, tx, *order.DriverID, workingStatusIdle); err != nil {
				return err
			}
		}
	case PaymentMethodWallet:
		escrowID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemEscrow)
		if err != nil {
			return err
		}
		if err := lockWalletsAsc(ctx, tx, escrowID, driverWallet.ID, platformID); err != nil {
			return err
		}
		if err := s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
			{
				WalletID:      escrowID,
				EntryType:     wallet.EntryDebit,
				Amount:        order.TotalFare,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeSendSettle,
				Description:   "SEND_SETTLEMENT - release escrow",
			},
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryCredit,
				Amount:        earning,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeSendSettle,
				Description:   "SEND_SETTLEMENT - driver earning 90%",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeSendSettle,
				Description:   "SEND_SETTLEMENT - platform commission 10%",
			},
		}); err != nil {
			return err
		}
		if err := s.repo.UpdateDriverWorkingStatus(ctx, tx, *order.DriverID, workingStatusIdle); err != nil {
			return err
		}
	default:
		return ErrInvalidPaymentMethod
	}

	if err := s.repo.MarkSendOrderSettled(ctx, tx, order.ID); err != nil {
		return err
	}

	from := sendStatusDelivered
	if err := s.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &from,
		ToStatus:    sendStatusSettled,
		TriggeredBy: order.DriverID,
	}); err != nil {
		return err
	}
	return nil
}

// ---- Task 3.6: Stop-Level Updates (ROADMAP 3.6.3, optional MVP) ----

// UpdateSendOrderStopRequest input untuk PATCH /send-orders/{id}/stops/{stop_id}.
type UpdateSendOrderStopRequest struct {
	OrderID          uuid.UUID
	StopID           uuid.UUID
	UserID           uuid.UUID
	Status           string
	DeliveryPhotoURL string
}

// UpdateSendOrderStopResponse hasil update status stop. OrderStatus berisi
// status send_orders SETELAH operasi; bila seluruh stop sudah COMPLETED pada
// operasi ini, order otomatis DELIVERED → SETTLED (auto-transition).
type UpdateSendOrderStopResponse struct {
	OrderID     uuid.UUID `json:"order_id"`
	StopID      uuid.UUID `json:"stop_id"`
	StopStatus  string    `json:"stop_status"`
	OrderStatus string    `json:"order_status"`
	Settled     bool      `json:"settled"`
}

// UpdateSendOrderStop memproses PATCH /send-orders/{id}/stops/{stop_id}
// (multi-stop MVP): driver menandai satu stop sebagai DELIVERED (input
// "DELIVERED" atau legacy "COMPLETED" dinormalisasi) dan menyimpan proof
// delivery_photo_url. Ketika semua stop DELIVERED, order utama otomatis
// bertransisi DELIVERED → SETTLED (3-way settlement) dalam transaksi sama.
func (s *Service) UpdateSendOrderStop(ctx context.Context, req UpdateSendOrderStopRequest) (*UpdateSendOrderStopResponse, error) {
	// Normalisasi status stop (enum 4-nilai TD-015: PENDING/PICKED_UP/
	// DELIVERED/CANCELLED).
	target := normalizeStopStatus(req.Status)
	if target == "" {
		return nil, ErrInvalidStopStatus
	}

	order, err := s.repo.GetSendOrderByID(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}
	if order.DriverID == nil || *order.DriverID != req.UserID {
		return nil, ErrNotAllowed
	}
	if order.Status != sendStatusPickedUp && order.Status != sendStatusInTransit {
		return nil, ErrInvalidTransition
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	locked, err := s.repo.LockSendOrder(ctx, tx, req.OrderID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrLockTimeout
		}
		return nil, err
	}
	if locked.Status != order.Status {
		return nil, ErrInvalidTransition
	}

	var proof *string
	if req.DeliveryPhotoURL != "" {
		proof = strPtr(req.DeliveryPhotoURL)
	}
	ok, err := s.repo.UpdateSendOrderStopStatus(ctx, tx, req.StopID, target, proof)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrStopNotFound
	}

	resp := &UpdateSendOrderStopResponse{
		OrderID:     req.OrderID,
		StopID:      req.StopID,
		StopStatus:  target,
		OrderStatus: locked.Status,
	}

	// Auto-transition: jika target DELIVERED dan semua stop sudah DELIVERED,
	// order utama otomatis DELIVERED → SETTLED dalam transaksi yang sama.
	if target == stopStatusDelivered {
		allCompleted, err := s.allStopsDelivered(ctx, tx, req.OrderID)
		if err != nil {
			return nil, err
		}
		if allCompleted {
			if err := s.autoDeliverSendOrder(ctx, tx, locked); err != nil {
				return nil, err
			}
			resp.OrderStatus = sendStatusSettled
			resp.Settled = true
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return resp, nil
}

// allStopsDelivered memeriksa apakah SEMUA stop sebuah send order berstatus
// DELIVERED (dibaca dalam transaksi berlock).
func (s *Service) allStopsDelivered(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (bool, error) {
	stops, err := s.repo.GetSendOrderStopsByOrderID(ctx, tx, orderID)
	if err != nil {
		return false, err
	}
	if len(stops) == 0 {
		return false, nil
	}
	for _, st := range stops {
		if st.Status != stopStatusDelivered {
			return false, nil
		}
	}
	return true, nil
}

// autoDeliverSendOrder menandai order DELIVERED + audit, lalu memicu 3-way
// settlement — dipanggil saat seluruh stop DELIVERED (stop-level flow).
func (s *Service) autoDeliverSendOrder(ctx context.Context, tx pgx.Tx, order *SendOrder) error {
	ok, err := s.repo.MarkSendOrderDelivered(ctx, tx, order.ID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidTransition
	}

	from := order.Status
	if err := s.repo.InsertSendOrderEvent(ctx, tx, SendOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &from,
		ToStatus:    sendStatusDelivered,
		TriggeredBy: order.DriverID,
	}); err != nil {
		return err
	}
	return s.settleSendOrderTx(ctx, tx, order)
}

// normalizeStopStatus menormalkan status stop yang diterima dari request ke
// enum 4-nilai (TD-015): PENDING, PICKED_UP, DELIVERED, CANCELLED. Nilai
// legacy (COMPLETED→DELIVERED, ARRIVED→PICKED_UP, SKIPPED→CANCELLED) tetap
// diterima untuk kompatibilitas data antarversi. PENDING tidak diset lewat
// endpoint ini (hanya terminal), sehingga mengembalikan "" (invalid).
func normalizeStopStatus(s string) string {
	switch s {
	case stopStatusDelivered, "COMPLETED":
		return stopStatusDelivered
	case stopStatusPickedUp, "ARRIVED":
		return stopStatusPickedUp
	case stopStatusCancelled, "SKIPPED":
		return stopStatusCancelled
	default:
		return ""
	}
}

// validSendStatusTarget memvalidasi nilai status yang boleh dikirim ke
// PATCH /send-orders/{id}. SEARCHING_DRIVER/DRIVER_ASSIGNED dikelola oleh
// flow accept driver (Task 3.6); SETTLED otomatis pada Task 3.6.
func validSendStatusTarget(s string) bool {
	switch s {
	case sendStatusCancelled, sendStatusPickedUp, sendStatusInTransit, sendStatusDelivered:
		return true
	default:
		return false
	}
}

// validateSendTransition memvalidasi transisi status per aktor sesuai state
// machine send order (ROADMAP 3.5/3.6 status flow).
func validateSendTransition(order *SendOrder, actor int, target string) error {
	from := order.Status
	switch actor {
	case actorKindSender:
		if target == sendStatusCancelled && (from == sendStatusCreated || from == sendStatusSearchingDriver) {
			return nil
		}
	case actorKindDriver:
		switch target {
		case sendStatusPickedUp:
			if from == sendStatusDriverAssigned {
				return nil
			}
		case sendStatusInTransit:
			if from == sendStatusPickedUp {
				return nil
			}
		case sendStatusDelivered:
			if from == sendStatusInTransit {
				return nil
			}
		case sendStatusCancelled:
			// Driver emergency cancel dari status apa pun yang belum final
			// (DRIVER_ASSIGNED / PICKED_UP / IN_TRANSIT).
			if from == sendStatusDriverAssigned || from == sendStatusPickedUp || from == sendStatusInTransit {
				return nil
			}
		}
	default:
		return ErrNotAllowed
	}
	return ErrInvalidTransition
}

// ---- driver matching placeholder ----

// triggerDriverMatching adalah placeholder untuk driver matching async
// (F005 / Task 3.6): query driver dalam radius 5km dari pickup, filter
// is_online=TRUE, working_status='IDLE', status='ACTIVE', lalu kirim order.
// Dipanggil SETELAH transaksi commit agar tidak menahan response.
func (s *Service) triggerDriverMatching(ctx context.Context, orderID uuid.UUID) {
	// TODO(Task 3.6): implementasi driver matching engine.
	_ = ctx
	_ = orderID
}

// ---- idempotency dual-layer (selaras dengan modul food/ride/wallet) ----

// redisKey membangun namespace Redis per user: idempotency:{userID}:{key}.
func redisKey(userID uuid.UUID, key string) string {
	return redisKeyPrefix + userID.String() + ":" + key
}

// redisCache adalah bentuk yang disimpan di Redis L1.
type redisCache struct {
	State    string          `json:"state"`
	Response json.RawMessage `json:"response"`
}

// redisGetCachedResp mengecek L1 Redis. Mengembalikan raw response JSON jika
// state COMPLETED.
func (s *Service) redisGetCachedResp(ctx context.Context, userID uuid.UUID, key string) (json.RawMessage, bool) {
	if s.redis == nil {
		return nil, false
	}
	val, err := s.redis.Get(ctx, redisKey(userID, key)).Result()
	if err != nil {
		return nil, false
	}
	var cached redisCache
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return nil, false
	}
	if cached.State != redisCompleted || len(cached.Response) == 0 {
		return nil, false
	}
	return cached.Response, true
}

// redisSet menulis nilai ke Redis (state COMPLETED) dengan TTL 24 jam.
func (s *Service) redisSet(ctx context.Context, userID uuid.UUID, key string, state string, raw json.RawMessage) {
	if s.redis == nil {
		return
	}
	cached := redisCache{State: state, Response: raw}
	if b, err := json.Marshal(cached); err == nil {
		s.redis.Set(ctx, redisKey(userID, key), b, redisTTL)
	}
}

// idemResult hasil idempotency L2.
type idemResult struct {
	cached  json.RawMessage
	proceed bool
}

// idemAcquire adalah logika L2 idempotency (idempotency_cache).
//
//   - Tidak ditemukan  -> insert PROCESSING (debounce 5 menit) -> proceed.
//   - COMPLETED        -> kembalikan response tersimpan (proceed=false).
//   - PROCESSING stale -> proceed (anggap requester sebelumnya crash).
//   - PROCESSING fresh -> error ErrIdempotencyInProgress.
func (s *Service) idemAcquire(ctx context.Context, owner uuid.UUID, key string) (idemResult, error) {
	var st string
	var body []byte
	var debounce *time.Time

	qErr := s.db.QueryRow(ctx, `
		SELECT state, response_body, debounce_at FROM idempotency_cache
		WHERE key = $1 AND user_id = $2
	`, key, owner).Scan(&st, &body, &debounce)

	if errors.Is(qErr, pgx.ErrNoRows) {
		if err := s.pgInsertProcessing(ctx, owner, key); err != nil {
			return idemResult{}, err
		}
		return idemResult{proceed: true}, nil
	}
	if qErr != nil {
		return idemResult{}, qErr
	}

	switch st {
	case redisCompleted:
		return idemResult{cached: json.RawMessage(body)}, nil
	case pgProcessing:
		if debounce != nil && debounce.Before(time.Now()) {
			return idemResult{proceed: true}, nil
		}
		return idemResult{}, ErrIdempotencyInProgress
	default:
		return idemResult{proceed: true}, nil
	}
}

// pgInsertProcessing menulis baris PROCESSING (L2). Jika key sudah ada
// (kompetisi user lain), return ErrIdempotencyInProgress.
func (s *Service) pgInsertProcessing(ctx context.Context, owner uuid.UUID, key string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO idempotency_cache
			(key, user_id, response_body, status_code, state, debounce_at, expires_at)
		VALUES ($1, $2, '{}'::jsonb, 0, 'PROCESSING', NOW() + INTERVAL '5 minutes', NOW() + INTERVAL '24 hours')
	`, key, owner)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrIdempotencyInProgress
		}
		return err
	}
	return nil
}

// cacheResponse menulis hasil sukses ke Redis L1 dan update L2 COMPLETED.
func (s *Service) cacheResponse(ctx context.Context, owner uuid.UUID, key string, resp any) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	s.redisSet(ctx, owner, key, redisCompleted, raw)
	if _, err := s.db.Exec(ctx, `
		UPDATE idempotency_cache
		SET state = 'COMPLETED', debounce_at = NULL, response_body = $3, status_code = 201
		WHERE key = $1 AND user_id = $2
	`, key, owner, raw); err != nil {
		return err
	}
	return nil
}

// ---- helpers internal ----

// lockWalletsAsc mengunci sejumlah wallet dengan urutan id menaik dalam satu
// statement (ORDER BY id ASC FOR UPDATE) untuk mencegah deadlock.
func lockWalletsAsc(ctx context.Context, tx pgx.Tx, walletIDs ...uuid.UUID) error {
	ids := make([]uuid.UUID, len(walletIDs))
	copy(ids, walletIDs)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	_, err := tx.Exec(ctx, `
		SELECT id FROM wallets WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE
	`, ids)
	return err
}

// getWalletBalanceTx membaca saldo wallet dalam transaksi yang sudah terkunci.
func getWalletBalanceTx(ctx context.Context, tx pgx.Tx, walletID uuid.UUID) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := tx.QueryRow(ctx, `
		SELECT balance FROM wallets WHERE id = $1
	`, walletID).Scan(&balance)
	return balance, err
}

// haversineKm menghitung jarak geodesik (km) antara dua koordinat dengan
// rumus haversine (earth radius 6371 km).
func haversineKm(lat1, lng1, lat2, lng2 float64) decimal.Decimal {
	const earthRadiusKm = 6371.0
	degToRad := math.Pi / 180

	dLat := (lat2 - lat1) * degToRad
	dLng := (lng2 - lng1) * degToRad

	lat1Rad := lat1 * degToRad
	lat2Rad := lat2 * degToRad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Asin(math.Sqrt(a))

	return decimal.NewFromFloat(earthRadiusKm * c)
}

func validLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// strPtr mengembalikan pointer string untuk kolom nullable.
func strPtr(s string) *string {
	return &s
}

// strPtrOrNil mengembalikan pointer string, atau nil bila string kosong.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
