// Package ride — Service Layer (F004: G-Ride Booking & Fare Calculation).
//
// Service menangani business logic untuk booking ride (POST /rides/book):
// validasi Debt Gate Universal, perhitungan fare berbasis jarak haversine,
// escrow (WALLET) dengan locking deadlock-free (ORDER BY id ASC FOR UPDATE),
// dual-layer idempotency (Redis L1 + PostgreSQL L2), dan pembuatan order
// + audit event secara atomik dalam satu transaksi.
//
// Prinsip kunci (selaras dengan modul wallet):
//   - Semua operasi finansial berjalan dalam satu transaksi DB. Order dibuat
//     di transaksi yang SAMA dengan escrow sehingga tidak ada orphan escrow
//     (LOGIC_FLOW §6.1 Ghost Order Prevention) — lebih aman dari alur
//     "deduct lalu create order" yang butuh saga compensation.
//   - Lock hierarchy global: users (jika ada) → ride_orders → wallets.
//     Booking memegang lock wallets ORDER BY id ASC setelah baris order
//     di-tulis (implicit row lock), sesuai standar lock ordering.
//   - Trigger DB `sync_wallet_balance` TIDAK dipanggil manual; ledger insert
//     dalam transaksi yang sudah mengunci wallet otomatis memutakhirkan
//     saldo (double-entry enforcement + DEFERRABLE balance validation).
//   - SET LOCAL statement_timeout = '3000ms' di setiap transaksi.
//   - Idempotency REQUIRED (X-Idempotency-Key) dengan cache response.
package ride

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/g-flow/g-flow/internal/wallet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// Constanta domain & bisnis G-Ride.
const (
	// Enum payment_method_enum (MIGRATION 004): WALLET | CASH | QRIS | CREDIT_CARD.
	// Booking hanya mengizinkan WALLET atau CASH (ROADMAP 02 §2.2).
	PaymentMethodWallet = "WALLET"
	PaymentMethodCash   = "CASH"

	WalletTypeCustomer     = "CUSTOMER"
	WalletTypeSystemEscrow = "SYSTEM_ESCROW"

	// Order expire jika tidak ada driver dalam 15 menit (auto-cancel worker).
	orderTTL = 15 * time.Minute

	// Reference type double-entry ledger untuk escrow ride.
	referenceTypeRideEscrow = "RIDE_ESCROW"

	statusCreated         = "CREATED"
	statusSearchingDriver = "SEARCHING_DRIVER"
	statusDriverAssigned  = "DRIVER_ASSIGNED"
	statusDriverArrived   = "DRIVER_ARRIVED"
	statusTripStarted     = "TRIP_STARTED"
	statusCompleted       = "COMPLETED"
	statusCancelled       = "CANCELLED"
	statusSettled         = "SETTLED"

	userTypeCustomer = "customer"
	userTypeDriver   = "driver"

	workingStatusIdle = "IDLE"
	workingStatusBusy = "BUSY"

	redisKeyPrefix = "idempotency:"
	redisCompleted = "COMPLETED"
	pgProcessing   = "PROCESSING"
	redisTTL       = 24 * time.Hour

	// Wallet types (migration 001: wallet_type_enum).
	WalletTypeDriver         = "DRIVER"
	WalletTypeSystemPlatform = "SYSTEM_PLATFORM"

	// Reference type double-entry ledger untuk settlement & refund ride.
	referenceTypeRideSettlement      = "RIDE_SETTLEMENT"
	referenceTypeRideRefund          = "RIDE_REFUND"
	referenceTypeRideCancellationFee = "RIDE_CANCELLATION_FEE"

	// Reference type untuk selisih fare (shortfall/surplus, TD-069).
	referenceTypeRideFareAdjustment = "RIDE_FARE_ADJUSTMENT"

	// Cancellation reasons (ROADMAP_02 / API_CONTRACT 7.3 / TD-068).
	reasonCustomerCancel  = "CUSTOMER_CANCEL"
	reasonDriverEmergency = "DRIVER_EMERGENCY"
	reasonNoShow          = "NO_SHOW"
	reasonExpired         = "EXPIRED"

	// Cancellation fee (LOGIC_FLOW 2.1 / ROADMAP 02 §2.4 / TD-068):
	//   - cancel setelah DRIVER_ASSIGNED : Rp 5.000 ke driver
	//   - cancel setelah DRIVER_ARRIVED  : Rp 10.000 ke driver
	//   - no-show (driver tiba > 5 menit, customer tidak muncul) : Rp 10.000
	//   - driver emergency cancel        : tanpa penalti (full refund)
	CancellationFeeAssigned = 5000
	CancellationFeeArrived  = 10000

	// Ceiling saldo negatif driver (LOGIC_FLOW 5.5): jika balance driver
	// < -Rp 50.000 setelah CASH settlement, akun driver di-SUSPENDED.
	maxDriverNegativeBalance = -50000

	// Voucher discount (TD-070): order_type_enum untuk layanan ride &
	// default per_user_limit bila kolom tidak diisi (MIGRATION 016).
	orderTypeRide        = "RIDE"
	voucherStatusActive  = "ACTIVE"
	voucherUsageApplied  = "APPLIED"
	voucherPerUserLimit  = 1
	voucherDiscountPct   = "PERCENTAGE"
	voucherDiscountFixed = "FIXED"
)

// Tarif G-Ride (API_CONTRACT 7.1 / ROADMAP 02 §2.2).
var (
	baseFare  = decimal.NewFromInt(10000)
	perKmRate = decimal.NewFromInt(4000)

	// Revenue split settlement (API_CONTRACT 7.3): platform 20%, driver 80%.
	driverShareRate   = decimal.RequireFromString("0.80")
	platformShareRate = decimal.RequireFromString("0.20")
)

// Error definitions untuk service layer booking.
var (
	ErrInvalidCoordinates     = errors.New("invalid coordinates (outside service area)")
	ErrSamePickupDropoff      = errors.New("pickup and dropoff cannot be the same")
	ErrInvalidPaymentMethod   = errors.New("invalid payment method (must be WALLET or CASH)")
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	ErrIdempotencyInProgress  = errors.New("idempotency key is still processing")
	ErrInvalidCachedResponse  = errors.New("cached idempotency response is invalid")

	ErrNotCustomer         = errors.New("user is not a customer")
	ErrCustomerInactive    = errors.New("customer is not ACTIVE")
	ErrOverdueDebt         = errors.New("customer has overdue debt (booking rejected)")
	ErrWalletInactive      = errors.New("customer wallet is not ACTIVE")
	ErrInsufficientBalance = errors.New("insufficient customer wallet balance for escrow")

	ErrNotDriver                 = errors.New("user is not a driver")
	ErrDriverInactive            = errors.New("driver is not ACTIVE")
	ErrDriverBusy                = errors.New("driver is busy (working_status not IDLE)")
	ErrOrderNotSearching         = errors.New("order is not in SEARCHING_DRIVER state")
	ErrInsufficientDriverBalance = errors.New("insufficient driver balance (below min_balance_threshold)")
	ErrLockTimeout               = errors.New("order lock not available (NOWAIT timeout)")

	ErrInvalidStatus     = errors.New("invalid status value")
	ErrInvalidTransition = errors.New("invalid status transition for current order state")
	ErrNotAllowed        = errors.New("user is not allowed to update this order")
	ErrInvalidPagination = errors.New("invalid page or page_size")

	// Voucher discount (TD-070). ErrVoucherNotFound dideklarasikan di
	// repository.go (dikembalikan saat scan baris voucher tidak ditemukan).
	ErrVoucherInvalid     = errors.New("voucher is not valid for ride service")
	ErrVoucherExpired     = errors.New("voucher has expired or not yet valid")
	ErrVoucherMinOrder    = errors.New("order amount does not meet voucher minimum")
	ErrVoucherPerUserLimit = errors.New("voucher per-user usage limit reached")
)

// BookRideRequest input untuk operasi booking ride.
type BookRideRequest struct {
	UserID         uuid.UUID
	PickupLat      float64
	PickupLng      float64
	PickupAddress  string
	DropoffLat     float64
	DropoffLng     float64
	DropoffAddress string
	PaymentMethod  string
	IdempotencyKey string
	// VoucherCode kode voucher diskon (opsional, TD-070). Nil/"" = tanpa voucher.
	VoucherCode *string
	// VoucherID id voucher diskon (opsional, alternatif VoucherCode, TD-070).
	// Jika keduanya diisi, dananya konflik → ErrVoucherInvalid.
	VoucherID *uuid.UUID
}

// FareBreakdown rincian perhitungan fare (API_CONTRACT 7.1).
type FareBreakdown struct {
	BaseFare       decimal.Decimal `json:"base_fare"`
	DistanceCharge decimal.Decimal `json:"distance_charge"`
	Total          decimal.Decimal `json:"total"`
}

// BookRideResponse hasil booking ride (API_CONTRACT 7.1).
type BookRideResponse struct {
	OrderID       uuid.UUID       `json:"order_id"`
	Status        string          `json:"status"`
	PickupLat     float64         `json:"pickup_lat"`
	PickupLng     float64         `json:"pickup_lng"`
	DropoffLat    float64         `json:"dropoff_lat"`
	DropoffLng    float64         `json:"dropoff_lng"`
	DistanceKm    decimal.Decimal `json:"distance_km"`
	EstimatedFare decimal.Decimal `json:"estimated_fare"`
	FareBreakdown FareBreakdown   `json:"fare_breakdown"`
	PaymentMethod string          `json:"payment_method"`
	EscrowAmount  decimal.Decimal `json:"escrow_amount"`
	ExpiresAt     time.Time       `json:"expires_at"`
	// Voucher discount (TD-070). DiscountAmount = 0 saat tanpa voucher;
	// VoucherCode nil saat tanpa voucher.
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	VoucherCode    *string         `json:"voucher_code"`
}

// AcceptOrderResponse hasil accept order oleh driver (API_CONTRACT 7.2).
type AcceptOrderResponse struct {
	OrderID    uuid.UUID `json:"order_id"`
	Status     string    `json:"status"`
	DriverID   uuid.UUID `json:"driver_id"`
	AssignedAt time.Time `json:"assigned_at"`
}

// Repo adalah kontrak repository yang dibutuhkan Service. Dipenuhi oleh
// *Repository (internal/ride/repository.go); dijadikan interface agar mudah
// di-mock pada unit test.
type Repo interface {
	GetCustomer(ctx context.Context, userID uuid.UUID) (*Customer, error)
	GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*RideWallet, error)
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (*RideOrder, error)
	ListOrdersByCustomer(ctx context.Context, customerID uuid.UUID, status string, limit, offset int) ([]RideOrder, error)
	CountOrdersByCustomer(ctx context.Context, customerID uuid.UUID, status string) (int, error)
	InsertOrder(ctx context.Context, q Querier, order *RideOrder) error
	TransitionStatus(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, toStatus string) (bool, error)
	InsertEvent(ctx context.Context, q Querier, event RideOrderEvent) error
	SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error)

	GetDriver(ctx context.Context, driverID uuid.UUID) (*Driver, error)
	GetDriverBalance(ctx context.Context, driverID uuid.UUID) (decimal.Decimal, error)
	LockDriverUserForAccept(ctx context.Context, q Querier, driverID uuid.UUID) (*Driver, error)
	LockOrderForAccept(ctx context.Context, q Querier, orderID uuid.UUID) error
	AssignDriver(ctx context.Context, q Querier, orderID uuid.UUID, driverID uuid.UUID) (bool, error)
	MarkDriverBusy(ctx context.Context, q Querier, driverID uuid.UUID) error

	// Voucher discount (TD-070).
	GetVoucherByCode(ctx context.Context, code string) (*Voucher, error)
	GetVoucherByID(ctx context.Context, voucherID uuid.UUID) (*Voucher, error)
	CountUserVoucherUsage(ctx context.Context, q Querier, userID, voucherID uuid.UUID) (int, error)
	InsertUserVoucher(ctx context.Context, q Querier, uv *UserVoucher) error

	// Fase lanjutan F005/F006: transisi status, cancel, auto-cancel & settlement.
	LockOrderForUpdate(ctx context.Context, q Querier, orderID uuid.UUID) (*RideOrder, error)
	GetExpiredSearchingOrders(ctx context.Context) ([]uuid.UUID, error)
	CancelOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, reason string, fee decimal.Decimal) (bool, error)
	CompleteOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus string, actualFare, driverEarning, platformCommission decimal.Decimal) (bool, error)
	MarkSettled(ctx context.Context, q Querier, orderID uuid.UUID) error
	IncrementOverdueDebt(ctx context.Context, q Querier, customerID uuid.UUID, amount decimal.Decimal) error
	ResetDriverIdle(ctx context.Context, q Querier, driverID uuid.UUID) error
	MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error
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
}

// Service adalah business logic untuk modul ride (F004 booking).
type Service struct {
	repo   Repo
	ledger Ledger
	redis  *redis.Client
	db     DB
}

// NewService membuat Service baru dengan dependency injection.
func NewService(repo Repo, ledger Ledger, rdb *redis.Client, db DB) *Service {
	return &Service{repo: repo, ledger: ledger, redis: rdb, db: db}
}

// BookRide melakukan booking ride untuk customer.
//
// Alur (ROADMAP 02 §2.2):
//  1. Validasi input (koordinat, payment_method, idempotency key).
//  2. Validasi akun customer: user_type=customer, status=ACTIVE,
//     dan Debt Gate Universal (overdue_debt = 0) — berlaku untuk WALLET
//     maupun CASH.
//  3. Validasi customer wallet = ACTIVE (dibutuhkan sebagai customer_wallet_id).
//  4. Hitung fare: haversine distance → base_fare + distance_km * per_km_rate.
//  5. Idempotency L1 (Redis) → L2 (PostgreSQL idempotency_cache).
//  6. Satu transaksi DB: insert order (CREATED) → transisi ke
//     SEARCHING_DRIVER → audit event → (WALLET) lock customer+escrow wallet
//     ORDER BY id, cek saldo, double-entry escrow ledger.
//  7. Cache response (L1 + L2 COMPLETED).
func (s *Service) BookRide(ctx context.Context, req BookRideRequest) (*BookRideResponse, error) {
	if err := s.validateBookingInput(req); err != nil {
		return nil, err
	}

	// L1: Redis (scope per user agar tidak collision antar user).
	if resp, ok := s.redisGetCachedResp(ctx, req.UserID, req.IdempotencyKey); ok {
		var out BookRideResponse
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
	if cust.Status != "ACTIVE" {
		return nil, ErrCustomerInactive
	}
	if cust.OverdueDebt.IsPositive() {
		return nil, ErrOverdueDebt
	}

	customerWallet, err := s.repo.GetWalletByUserAndType(ctx, req.UserID, WalletTypeCustomer)
	if err != nil {
		return nil, err
	}
	if customerWallet.Status != "ACTIVE" {
		return nil, ErrWalletInactive
	}

	// Hitung fare.
	distanceKm := haversineKm(req.PickupLat, req.PickupLng, req.DropoffLat, req.DropoffLng)
	if !distanceKm.IsPositive() {
		return nil, ErrSamePickupDropoff
	}
	dist := distanceKm.Round(3)
	estimatedFare := baseFare.Add(dist.Mul(perKmRate)).Round(2)

	// Voucher discount (TD-070): resolve → validasi → hitung potongan.
	// Potongan memakai estimatedFare sebagai subtotal (min_order ditegakkan
	// terhadap estimatedFare, bukan total setelah diskon).
	voucherDiscount := decimal.Zero
	var voucher *Voucher
	if (req.VoucherCode != nil && *req.VoucherCode != "") && req.VoucherID != nil {
		return nil, ErrVoucherInvalid
	}
	if req.VoucherCode != nil && *req.VoucherCode != "" {
		v, err := s.repo.GetVoucherByCode(ctx, *req.VoucherCode)
		if err != nil {
			return nil, err
		}
		voucher = v
	} else if req.VoucherID != nil {
		v, err := s.repo.GetVoucherByID(ctx, *req.VoucherID)
		if err != nil {
			return nil, err
		}
		voucher = v
	}
	if voucher != nil {
		if err := validateVoucher(voucher, estimatedFare); err != nil {
			return nil, err
		}
		voucherDiscount = computeVoucherDiscount(voucher, estimatedFare)
	}

	// L2: PostgreSQL idempotency.
	if res, err := s.idemAcquire(ctx, req.UserID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if !res.proceed {
		s.redisSet(ctx, req.UserID, req.IdempotencyKey, redisCompleted, res.cached)
		var out BookRideResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	orderID := uuid.New()
	expiresAt := time.Now().Add(orderTTL)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Pre-check per_user_limit (early exit sebelum menulis order). Trigger DB
	// validate_per_user_limit tetap menjadi guard final (defense-in-depth).
	if voucher != nil && voucherDiscount.IsPositive() {
		usage, err := s.repo.CountUserVoucherUsage(ctx, tx, req.UserID, voucher.ID)
		if err != nil {
			return nil, err
		}
		limit := voucher.PerUserLimit
		if limit <= 0 {
			limit = voucherPerUserLimit
		}
		if usage >= limit {
			return nil, ErrVoucherPerUserLimit
		}
	}

	// Buat order berstatus CREATED (transaksi yang sama dengan escrow).
	order := &RideOrder{
		ID:               orderID,
		CustomerID:       req.UserID,
		CustomerWalletID: &customerWallet.ID,
		PickupLat:        req.PickupLat,
		PickupLng:        req.PickupLng,
		PickupAddress:    req.PickupAddress,
		DropoffLat:       req.DropoffLat,
		DropoffLng:       req.DropoffLng,
		DropoffAddress:   req.DropoffAddress,
		DistanceKm:       dist,
		BaseFare:         baseFare,
		PerKmRate:        perKmRate,
		EstimatedFare:    estimatedFare,
		PaymentMethod:    req.PaymentMethod,
		Status:           statusCreated,
		ExpiresAt:        &expiresAt,
	}
	if voucher != nil {
		if voucherDiscount.IsPositive() {
			order.VoucherID = &voucher.ID
			order.DiscountAmount = &voucherDiscount
		}
	}
	if err := s.repo.InsertOrder(ctx, tx, order); err != nil {
		return nil, err
	}

	// CREATED → SEARCHING_DRIVER (transisi terpisah + audit event).
	if ok, err := s.repo.TransitionStatus(ctx, tx, orderID, statusCreated, statusSearchingDriver); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("ride: gagal transisi order ke SEARCHING_DRIVER")
	}
	if err := s.repo.InsertEvent(ctx, tx, RideOrderEvent{
		OrderID:     orderID,
		FromStatus:  stringPtr(statusCreated),
		ToStatus:    statusSearchingDriver,
		TriggeredBy: &req.UserID,
	}); err != nil {
		return nil, err
	}

	// Escrow conditional: hanya payment WALLET. Escrow dihitung setelah
	// diskon voucher (estimated - discount) sehingga dana yang ditahan = yang
	// benar-benar dibayar customer (TD-070).
	escrowAmount := decimal.Zero
	if req.PaymentMethod == PaymentMethodWallet {
		holding := estimatedFare.Sub(voucherDiscount)
		if holding.IsPositive() {
			escrowAmount, err = s.holdEscrow(ctx, tx, customerWallet, orderID, holding)
			if err != nil {
				return nil, err
			}
		}
	}

	// Catat pemakaian voucher (user_vouchers) dalam transaksi yang sama
	// dengan order. Trigger DB memvalidasi per_user_limit & meng-increment
	// vouchers.used_count; kalau transaksi di-rollback, pemakaian ikut batal.
	if voucher != nil && voucherDiscount.IsPositive() {
		orderIDVal := orderID
		if err := s.repo.InsertUserVoucher(ctx, tx, &UserVoucher{
			UserID:          req.UserID,
			VoucherID:       voucher.ID,
			OrderType:       orderTypeRide,
			OrderID:         &orderIDVal,
			DiscountApplied: voucherDiscount,
			Status:          voucherUsageApplied,
		}); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && (pgErr.Code == "P0001" || pgErr.Code == "23505") {
				return nil, ErrVoucherPerUserLimit
			}
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	resp := &BookRideResponse{
		OrderID:       orderID,
		Status:        statusSearchingDriver,
		PickupLat:     req.PickupLat,
		PickupLng:     req.PickupLng,
		DropoffLat:    req.DropoffLat,
		DropoffLng:    req.DropoffLng,
		DistanceKm:    dist,
		EstimatedFare: estimatedFare,
		FareBreakdown: FareBreakdown{
			BaseFare:       baseFare,
			DistanceCharge: dist.Mul(perKmRate).Round(2),
			Total:          estimatedFare,
		},
		PaymentMethod:  req.PaymentMethod,
		EscrowAmount:   escrowAmount,
		DiscountAmount: voucherDiscount,
		ExpiresAt:      expiresAt,
	}
	if voucher != nil {
		code := voucher.Code
		resp.VoucherCode = &code
	}

	if err := s.cacheResponse(ctx, req.UserID, req.IdempotencyKey, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetOrder mengambil order lengkap berdasarkan id.
func (s *Service) GetOrder(ctx context.Context, orderID uuid.UUID) (*RideOrder, error) {
	return s.repo.GetOrderByID(ctx, orderID)
}

// GetRidesHistory mengembalikan riwayat ride customer dengan pagination
// (page mulai 1, page_size 1..50, default 20), urut created_at DESC, dan
// filter status opsional (exact match, di-uppercase oleh handler).
func (s *Service) GetRidesHistory(ctx context.Context, customerID uuid.UUID, page, pageSize int, status string) ([]RideOrder, int, error) {
	if page < 1 || pageSize < 1 || pageSize > 50 {
		return nil, 0, ErrInvalidPagination
	}
	total, err := s.repo.CountOrdersByCustomer(ctx, customerID, status)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.repo.ListOrdersByCustomer(ctx, customerID, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// AcceptOrder menetapkan driver ke order yang sedang mencari driver
// (SEARCHING_DRIVER → DRIVER_ASSIGNED).
//
// Alur (F005 Driver Dispatch Matching Engine):
//  1. Validasi driver: user_type='driver', status='ACTIVE',
//     working_status='IDLE'.
//  2. Validasi saldo wallet driver (wallet_type='DRIVER') >=
//     min_balance_threshold.
//  3. Transaksi: lock users driver FOR UPDATE NOWAIT (guard ACTIVE+IDLE,
//     ROADMAP 02 §2.3 lock hierarchy: users → ride_orders), lalu lock
//     ride_orders FOR UPDATE NOWAIT + status='SEARCHING_DRIVER'.
//  4. Update order: status='DRIVER_ASSIGNED', driver_id, assigned_at=NOW().
//  5. Update users: working_status='BUSY'.
//  6. Insert audit event ride_order_events (SEARCHING_DRIVER → DRIVER_ASSIGNED).
//
// Error mapping:
//   - driver tidak ditemukan / tidak aktif             -> 422
//   - driver sibuk (working_status != IDLE)           -> 409
//   - order bukan SEARCHING_DRIVER                    -> 409
//   - lock tidak tersedia (NOWAIT)                    -> 503
//   - saldo driver di bawah ambang                     -> 422
func (s *Service) AcceptOrder(ctx context.Context, orderID uuid.UUID, driverID uuid.UUID) (*AcceptOrderResponse, error) {
	driver, err := s.repo.GetDriver(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if driver.UserType != userTypeDriver {
		return nil, ErrNotDriver
	}
	if driver.Status != "ACTIVE" {
		return nil, ErrDriverInactive
	}
	if driver.WorkingStatus != workingStatusIdle {
		return nil, ErrDriverBusy
	}

	balance, err := s.repo.GetDriverBalance(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if balance.LessThan(driver.MinBalanceThreshold) {
		return nil, ErrInsufficientDriverBalance
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Lock user driver DULU dengan FOR UPDATE NOWAIT (ROADMAP 02 §2.3 lock
	// hierarchy: users → ride_orders). Guard atomik ACTIVE+driver; hasil
	// re-validasi working_status DI BAWAH LOCK agar driver yang sama tidak
	// bisa di-assign 2 order secara paralel (double assign).
	lockedDriver, err := s.repo.LockDriverUserForAccept(ctx, tx, driverID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrLockTimeout
		}
		return nil, err
	}
	if lockedDriver.WorkingStatus != workingStatusIdle {
		return nil, ErrDriverBusy
	}

	// Lock order tunggal dengan FOR UPDATE NOWAIT + guard status.
	err = s.repo.LockOrderForAccept(ctx, tx, orderID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrLockTimeout
		}
		if errors.Is(err, ErrOrderNotFound) {
			// Order tidak ditemukan atau bukan SEARCHING_DRIVER.
			if _, gErr := s.repo.GetOrderByID(ctx, orderID); gErr != nil {
				return nil, gErr
			}
			return nil, ErrOrderNotSearching
		}
		return nil, err
	}

	// Update order: DRIVER_ASSIGNED + driver_id + assigned_at=NOW().
	ok, err := s.repo.AssignDriver(ctx, tx, orderID, driverID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrOrderNotSearching
	}

	// Update users: working_status = 'BUSY'.
	if err := s.repo.MarkDriverBusy(ctx, tx, driverID); err != nil {
		return nil, err
	}

	// Audit event SEARCHING_DRIVER → DRIVER_ASSIGNED.
	fromStatus := statusSearchingDriver
	if err := s.repo.InsertEvent(ctx, tx, RideOrderEvent{
		OrderID:     orderID,
		FromStatus:  &fromStatus,
		ToStatus:    statusDriverAssigned,
		TriggeredBy: &driverID,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &AcceptOrderResponse{
		OrderID:    orderID,
		Status:     statusDriverAssigned,
		DriverID:   driverID,
		AssignedAt: time.Now(),
	}, nil
}

// UpdateRideStatusRequest input untuk PATCH /rides/:order_id/status.
// ActualFare opsional (TD-069): fare aktual dari driver saat COMPLETED; jika
// nil/nol → pakai estimated_fare (backward compat).
type UpdateRideStatusRequest struct {
	OrderID    uuid.UUID
	UserID     uuid.UUID
	Status     string
	Reason     string
	ActualFare *decimal.Decimal
}

// UpdateRideStatusResponse hasil update status order (API_CONTRACT 7.3).
// CancellationFee diisi (informasi fee yang dipungut) saat status = CANCELLED.
type UpdateRideStatusResponse struct {
	OrderID         uuid.UUID        `json:"order_id"`
	Status          string           `json:"status"`
	CancellationFee *decimal.Decimal `json:"cancellation_fee"`
}

// UpdateRideStatus memproses PATCH /rides/:order_id/status:
//
//   - Transisi driver: DRIVER_ASSIGNED → DRIVER_ARRIVED → TRIP_STARTED →
//     COMPLETED. COMPLETED otomatis memicu settlement (COMPLETED → SETTLED):
//     WALLET = rilis escrow (driver 80% + platform 20%); CASH = debit komisi
//     platform (20%) dari wallet driver (COD), dengan ceiling negatif -Rp 50.000.
//   - CANCELLED dari status apa pun yang belum final (customer pemilik order
//     atau driver tertunjuk) → refund escrow penuh tanpa penalti, driver
//     kembali IDLE.
//
// Auth: customer (pemilik order) untuk cancel; driver tertunjuk untuk transisi
// status. Authorization divalidasi di service; handler hanya meneruskan
// user_id dari JWT claim. Locking order FOR UPDATE mencegah dua transisi
// bersamaan (cancel vs accept / double-cancel).
func (s *Service) UpdateRideStatus(ctx context.Context, req UpdateRideStatusRequest) (*UpdateRideStatusResponse, error) {
	if !validRideStatusTarget(req.Status) {
		return nil, ErrInvalidStatus
	}

	order, err := s.repo.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}

	// Order sudah final → tidak ada transisi lagi.
	if order.Status == statusCancelled || order.Status == statusSettled {
		return nil, ErrInvalidTransition
	}

	if err := validateStatusTransition(order, req); err != nil {
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

	// Kunci baris order di awal transaksi (FOR UPDATE) agar dua transisi
	// bersamaan tidak saling menimpa.
	locked, err := s.repo.LockOrderForUpdate(ctx, tx, req.OrderID)
	if err != nil {
		return nil, err
	}
	if locked.Status != order.Status {
		return nil, ErrInvalidTransition
	}

	switch req.Status {
	case statusCancelled:
		return s.cancelOrderTx(ctx, tx, locked, req)
	case statusCompleted:
		return s.completeOrderTx(ctx, tx, locked, req)
	default:
		// DRIVER_ARRIVED / TRIP_STARTED — transisi murni + audit event.
		from := locked.Status
		ok, err := s.repo.TransitionStatus(ctx, tx, locked.ID, from, req.Status)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidTransition
		}
		if err := s.repo.InsertEvent(ctx, tx, RideOrderEvent{
			OrderID:     locked.ID,
			FromStatus:  &from,
			ToStatus:    req.Status,
			TriggeredBy: &req.UserID,
		}); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &UpdateRideStatusResponse{OrderID: locked.ID, Status: req.Status}, nil
	}
}

// cancelOrderTx meng-cancel order dalam transaksi yang sudah mengunci baris
// order: set status CANCELLED + cancellation_reason + cancellation_fee.
//
// Skema fee (LOGIC_FLOW 2.1 / ROADMAP 02 §2.4 / TD-068):
//   - cancel SEBELUM DRIVER_ASSIGNED : full refund, tanpa fee.
//   - cancel setelah DRIVER_ASSIGNED  : fee Rp 5.000 → driver, refund
//     (estimated - 5.000) → customer (WALLET).
//   - cancel setelah DRIVER_ARRIVED   : fee Rp 10.000 → driver, refund
//     (estimated - 10.000) → customer (WALLET).
//   - no-show (reason NO_SHOW)       : fee Rp 10.000 → driver. Hanya driver
//     tertunjuk yang sudah DRIVER_ARRIVED.
//   - driver emergency (DRIVER_EMERGENCY): full refund, tanpa penalti.
//
// Guard reason anti-bypass: NO_SHOW & DRIVER_EMERGENCY wajib dari driver
// tertunjuk (mencegah customer memalsukan reason untuk menghindari fee).
// Driver lepas dari order → kembali IDLE. Audit event + metadata reason.
func (s *Service) cancelOrderTx(ctx context.Context, tx pgx.Tx, order *RideOrder, req UpdateRideStatusRequest) (*UpdateRideStatusResponse, error) {
	reason := req.Reason
	if reason == "" {
		if order.DriverID != nil && req.UserID == *order.DriverID {
			reason = reasonDriverEmergency
		} else {
			reason = reasonCustomerCancel
		}
	}

	// Guard reason khusus (re-validasi di bawah lock).
	switch reason {
	case reasonNoShow:
		if order.DriverID == nil || *order.DriverID != req.UserID {
			return nil, ErrNotAllowed
		}
		if order.Status != statusDriverArrived {
			return nil, ErrInvalidTransition
		}
	case reasonDriverEmergency:
		if order.DriverID == nil || *order.DriverID != req.UserID {
			return nil, ErrNotAllowed
		}
	}

	fee := cancellationFeeFor(order.Status, reason)

	ok, err := s.repo.CancelOrder(ctx, tx, order.ID, order.Status, reason, fee)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidTransition
	}

	// WALLET: refund escrow (dengan fee jika ada; tanpa fee = full refund).
	// CASH: tidak ada escrow yang ditahan — fee hanya dicatat di DB.
	if order.PaymentMethod == PaymentMethodWallet {
		if fee.IsPositive() {
			if err := s.refundCancellationFee(ctx, tx, order, fee); err != nil {
				return nil, err
			}
		} else {
			if err := s.refundEscrow(ctx, tx, order); err != nil {
				return nil, err
			}
		}
	}

	// Driver lepas dari order → kembali IDLE.
	if order.DriverID != nil {
		if err := s.repo.ResetDriverIdle(ctx, tx, *order.DriverID); err != nil {
			return nil, err
		}
	}

	// Audit event dengan metadata reason untuk traceability.
	event := RideOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &order.Status,
		ToStatus:    statusCancelled,
		Reason:      &reason,
		TriggeredBy: &req.UserID,
	}
	if md, err := json.Marshal(map[string]string{"reason": reason}); err == nil {
		event.Metadata = md
	}
	if err := s.repo.InsertEvent(ctx, tx, event); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &UpdateRideStatusResponse{OrderID: order.ID, Status: statusCancelled, CancellationFee: &fee}, nil
}

// cancellationFeeFor menghitung cancellation fee berdasar status order &
// reason (LOGIC_FLOW 2.1 / ROADMAP 02 §2.4):
//   - DRIVER_EMERGENCY        -> 0 (full refund, tanpa penalti)
//   - NO_SHOW                 -> Rp 10.000 (driver sudah menunggu di pickup)
//   - DRIVER_ASSIGNED         -> Rp 5.000 (cancel customer)
//   - DRIVER_ARRIVED/TRIP_STARTED -> Rp 10.000 (cancel customer)
//   - sebelum driver ditunjuk -> 0 (full refund)
func cancellationFeeFor(orderStatus, reason string) decimal.Decimal {
	switch reason {
	case reasonNoShow:
		return decimal.NewFromInt(CancellationFeeArrived)
	case reasonDriverEmergency:
		return decimal.Zero
	}
	switch orderStatus {
	case statusDriverAssigned:
		return decimal.NewFromInt(CancellationFeeAssigned)
	case statusDriverArrived, statusTripStarted:
		return decimal.NewFromInt(CancellationFeeArrived)
	default:
		return decimal.Zero
	}
}

// completeOrderTx menandai order COMPLETED lalu memicu settlement otomatis
// dalam transaksi yang sama (COMPLETED → SETTLED). Hanya driver yang sudah
// melewati TRIP_STARTED boleh menyelesaikan order.
//
// TD-069 (delta fare): driver boleh mengirim actual_fare opsional. Jika
// kosong/nol → pakai fareBasis (estimated_fare − discount; TD-070). Jika ada:
//   - Shortfall (actual > fareBasis): debit customer delta; jika saldo
//     customer kurang, kekurangan dari subsidi SYSTEM_PLATFORM (wallet
//     SYSTEM_PLATFORM_SUBSIDY belum ada, TD-132) + record overdue_debt.
//   - Surplus (actual < fareBasis): credit customer delta, debit escrow.
//
// Settlement selalu memakai actual_fare (driver 80%, platform 20%).
func (s *Service) completeOrderTx(ctx context.Context, tx pgx.Tx, order *RideOrder, req UpdateRideStatusRequest) (*UpdateRideStatusResponse, error) {
	actualFare := fareBasis(order)
	if req.ActualFare != nil && req.ActualFare.IsPositive() {
		actualFare = req.ActualFare.Round(2)
	}

	if err := s.applyFareDelta(ctx, tx, order, actualFare); err != nil {
		return nil, err
	}

	commission := actualFare.Mul(platformShareRate).Round(2)
	earning := actualFare.Sub(commission)

	ok, err := s.repo.CompleteOrder(ctx, tx, order.ID, order.Status, actualFare, earning, commission)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidTransition
	}

	from := order.Status
	if err := s.repo.InsertEvent(ctx, tx, RideOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &from,
		ToStatus:    statusCompleted,
		TriggeredBy: &req.UserID,
	}); err != nil {
		return nil, err
	}

	if err := s.settleOrderTx(ctx, tx, order, actualFare, earning, commission); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &UpdateRideStatusResponse{OrderID: order.ID, Status: statusSettled}, nil
}

// applyFareDelta menerapkan selisih actual vs fareBasis (LOGIC_FLOW 2.2
// §2 / TD-069, basis diperbarui utk diskon voucher TD-070). Hanya berlaku
// untuk payment WALLET (ada escrow):
//   - Shortfall (actual > fareBasis): menahan delta ke escrow — DEBIT customer
//     selisih; jika saldo customer kurang, kekurangan ditutup subsidi
//     SYSTEM_PLATFORM + dicatat overdue_debt.
//   - Surplus (actual < fareBasis): refund — DEBIT escrow, CREDIT customer.
func (s *Service) applyFareDelta(ctx context.Context, tx pgx.Tx, order *RideOrder, actualFare decimal.Decimal) error {
	delta := actualFare.Sub(fareBasis(order)).Round(2)
	if delta.IsZero() || order.PaymentMethod != PaymentMethodWallet {
		return nil
	}
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	escrowID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemEscrow)
	if err != nil {
		return err
	}
	if delta.IsNegative() {
		return s.refundSurplusDelta(ctx, tx, order, escrowID, delta.Neg())
	}
	return s.collectShortfallDelta(ctx, tx, order, escrowID, delta)
}

// refundSurplusDelta mengembalikan surplus (estimated − actual) ke customer:
// DEBIT escrow → CREDIT customer wallet.
func (s *Service) refundSurplusDelta(ctx context.Context, tx pgx.Tx, order *RideOrder, escrowID uuid.UUID, surplus decimal.Decimal) error {
	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID); err != nil {
		return err
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        surplus,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideFareAdjustment,
			Description:   "RIDE_FARE_ADJUSTMENT - DEBIT escrow (surplus delta fare)",
		},
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        surplus,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideFareAdjustment,
			Description:   "RIDE_FARE_ADJUSTMENT - CREDIT customer (surplus delta fare)",
		},
	})
}

// collectShortfallDelta menahan selisih shortfall (actual − estimated) ke
// escrow. Debit sebesar saldo customer (maksimal delta); kekurangan ditutup
// subsidi SYSTEM_PLATFORM (fallback sementara, TD-132) dan dicatat sebagai
// overdue_debt customer (LOGIC_FLOW 2.2 §2).
func (s *Service) collectShortfallDelta(ctx context.Context, tx pgx.Tx, order *RideOrder, escrowID uuid.UUID, delta decimal.Decimal) error {
	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID); err != nil {
		return err
	}
	balance, err := getWalletBalanceTx(ctx, tx, *order.CustomerWalletID)
	if err != nil {
		return err
	}
	fromCustomer := delta
	if balance.LessThan(delta) {
		fromCustomer = balance.Round(2)
	}
	shortfall := delta.Sub(fromCustomer)

	entries := make([]wallet.LedgerEntry, 0, 4)
	if fromCustomer.IsPositive() {
		entries = append(entries,
			wallet.LedgerEntry{
				WalletID:      *order.CustomerWalletID,
				EntryType:     wallet.EntryDebit,
				Amount:        fromCustomer,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideFareAdjustment,
				Description:   "RIDE_FARE_ADJUSTMENT - DEBIT customer (shortfall delta)",
			},
			wallet.LedgerEntry{
				WalletID:      escrowID,
				EntryType:     wallet.EntryCredit,
				Amount:        fromCustomer,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideFareAdjustment,
				Description:   "RIDE_FARE_ADJUSTMENT - CREDIT escrow (shortfall delta)",
			},
		)
	}
	if shortfall.IsPositive() {
		// Wallet SYSTEM_PLATFORM_SUBSIDY belum ada (TD-132) — fallback sementara
		// memakai SYSTEM_PLATFORM sebagai sumber subsidi shortfall.
		subsidyID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemPlatform)
		if err != nil {
			return err
		}
		if err := lockWalletsAsc(ctx, tx, subsidyID); err != nil {
			return err
		}
		entries = append(entries,
			wallet.LedgerEntry{
				WalletID:      subsidyID,
				EntryType:     wallet.EntryDebit,
				Amount:        shortfall,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideFareAdjustment,
				Description:   "RIDE_FARE_ADJUSTMENT - DEBIT SYSTEM_PLATFORM (subsidy shortfall)",
			},
			wallet.LedgerEntry{
				WalletID:      escrowID,
				EntryType:     wallet.EntryCredit,
				Amount:        shortfall,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideFareAdjustment,
				Description:   "RIDE_FARE_ADJUSTMENT - CREDIT escrow (shortfall covered by subsidy)",
			},
		)
		if err := s.repo.IncrementOverdueDebt(ctx, tx, order.CustomerID, shortfall); err != nil {
			return err
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, entries)
}

// settleOrderTx melakukan settlement double-entry ledger (API_CONTRACT 7.3):
//
//	WALLET: DEBIT SYSTEM_ESCROW (actual_fare) → CREDIT driver (80%) + CREDIT
//	        SYSTEM_PLATFORM (20%).
//	CASH  : DEBIT driver wallet (komisi 20% dari actual_fare) → CREDIT
//	        SYSTEM_PLATFORM. Saldo driver boleh negatif (COD) dengan ceiling
//	        -Rp 50.000; jika balance < ceiling, driver di-SUSPENDED.
//
// TD-069: escrow dilepas sebesar actual_fare (bukan estimated), sesuai
// delta yang sudah ditahan/direfund oleh applyFareDelta.
//
// Lalu menandai order SETTLED + audit event COMPLETED → SETTLED.
func (s *Service) settleOrderTx(ctx context.Context, tx pgx.Tx, order *RideOrder, actualFare, earning, commission decimal.Decimal) error {
	var driverWallet *RideWallet
	if order.DriverID != nil {
		w, err := s.repo.GetWalletByUserAndType(ctx, *order.DriverID, WalletTypeDriver)
		if err != nil {
			return err
		}
		driverWallet = w
	}
	if driverWallet == nil {
		return ErrDriverNotFound
	}

	platformID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemPlatform)
	if err != nil {
		return err
	}

	switch order.PaymentMethod {
	case PaymentMethodCash:
		// Customer bayar tunai ke driver; driver menyetor komisi platform.
		// Saldo driver boleh negatif, namun dibatasi ceiling -Rp 50.000.
		if err := lockWalletsAsc(ctx, tx, driverWallet.ID, platformID); err != nil {
			return err
		}
		if err := s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryDebit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideSettlement,
				Description:   "RIDE_SETTLEMENT - platform commission DEBIT driver (CASH)",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideSettlement,
				Description:   "RIDE_SETTLEMENT - platform commission CREDIT (CASH)",
			},
		}); err != nil {
			return err
		}
		balance, err := getWalletBalanceTx(ctx, tx, driverWallet.ID)
		if err != nil {
			return err
		}
		if balance.LessThan(decimal.NewFromInt(maxDriverNegativeBalance)) {
			if err := s.repo.MarkDriverSuspended(ctx, tx, *order.DriverID); err != nil {
				return err
			}
		} else {
			if err := s.repo.ResetDriverIdle(ctx, tx, *order.DriverID); err != nil {
				return err
			}
		}
	case PaymentMethodWallet:
		escrowID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemEscrow)
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
				Amount:        actualFare,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideSettlement,
				Description:   "RIDE_SETTLEMENT - release escrow",
			},
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryCredit,
				Amount:        earning,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideSettlement,
				Description:   "RIDE_SETTLEMENT - driver earning 80%",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeRideSettlement,
				Description:   "RIDE_SETTLEMENT - platform commission 20%",
			},
		}); err != nil {
			return err
		}
		if err := s.repo.ResetDriverIdle(ctx, tx, *order.DriverID); err != nil {
			return err
		}
	default:
		return ErrInvalidPaymentMethod
	}

	if err := s.repo.MarkSettled(ctx, tx, order.ID); err != nil {
		return err
	}

	from := statusCompleted
	if err := s.repo.InsertEvent(ctx, tx, RideOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &from,
		ToStatus:    statusSettled,
		TriggeredBy: order.DriverID,
	}); err != nil {
		return err
	}
	return nil
}

// refundEscrow mengembalikan dana escrow penuh ke customer saat order
// dibatalkan (payment WALLET): DEBIT SYSTEM_ESCROW → CREDIT customer wallet.
// Nilai refund = fareBasis (post-diskon) agar tidak over-refund (TD-070).
func (s *Service) refundEscrow(ctx context.Context, tx pgx.Tx, order *RideOrder) error {
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	refund := fareBasis(order)
	escrowID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemEscrow)
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID); err != nil {
		return err
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        refund,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideRefund,
			Description:   "RIDE_REFUND - escrow release to customer",
		},
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        refund,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideRefund,
			Description:   "RIDE_REFUND - full refund customer wallet",
		},
	})
}

// refundCancellationFee melepas escrow saat cancel yang memungut fee (WALLET):
// escrow di-DEBIT total fareBasis, CREDIT driver = cancellation fee
// (reference RIDE_CANCELLATION_FEE), CREDIT customer = fareBasis - fee
// (reference RIDE_REFUND). Double-entry tetap seimbang per reference_id
// (trigger validate_ledger_balance) karena SUM DEBIT = SUM CREDIT.
func (s *Service) refundCancellationFee(ctx context.Context, tx pgx.Tx, order *RideOrder, fee decimal.Decimal) error {
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	if order.DriverID == nil {
		return ErrDriverNotFound
	}
	refundable := fareBasis(order)
	escrowID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemEscrow)
	if err != nil {
		return err
	}
	driverWallet, err := s.repo.GetWalletByUserAndType(ctx, *order.DriverID, WalletTypeDriver)
	if err != nil {
		return err
	}

	// Defensive: fee tidak boleh melebihi refundable sehingga refund >= 0.
	if fee.GreaterThan(refundable) {
		fee = refundable
	}
	customerRefund := refundable.Sub(fee)

	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID, driverWallet.ID); err != nil {
		return err
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        fee,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideCancellationFee,
			Description:   "RIDE_CANCELLATION_FEE - DEBIT escrow (fee to driver)",
		},
		{
			WalletID:      driverWallet.ID,
			EntryType:     wallet.EntryCredit,
			Amount:        fee,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideCancellationFee,
			Description:   "RIDE_CANCELLATION_FEE - CREDIT driver wallet",
		},
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryDebit,
			Amount:        customerRefund,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideRefund,
			Description:   "RIDE_REFUND - escrow release (balance after cancellation fee)",
		},
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        customerRefund,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeRideRefund,
			Description:   "RIDE_REFUND - refund customer minus cancellation fee",
		},
	})
}

// AutoCancelExpiredOrders (Auto-Cancel Worker) membatalkan order yang masih
// SEARCHING_DRIVER dan sudah melewati expires_at (TTL 15 menit). Setiap order
// ditangani dalam transaksi terpisah (lock FOR UPDATE → CANCELLED reason
// EXPIRED → refund escrow → audit event). Mengembalikan jumlah order yang
// berhasil di-cancel. Worker ini dipanggil by cron/scheduler; pada test
// dipanggil manual.
func (s *Service) AutoCancelExpiredOrders(ctx context.Context) (int, error) {
	ids, err := s.repo.GetExpiredSearchingOrders(ctx)
	if err != nil {
		return 0, err
	}

	cancelled := 0
	for _, id := range ids {
		order, err := s.repo.GetOrderByID(ctx, id)
		if err != nil {
			continue
		}
		if order.Status != statusSearchingDriver {
			continue
		}

		tx, err := s.db.Begin(ctx)
		if err != nil {
			return cancelled, err
		}
		txErr := func() error {
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
				return err
			}
			locked, err := s.repo.LockOrderForUpdate(ctx, tx, id)
			if err != nil {
				return err
			}
			if locked.Status != statusSearchingDriver {
				return ErrInvalidTransition
			}
			ok, err := s.repo.CancelOrder(ctx, tx, id, statusSearchingDriver, reasonExpired, decimal.Zero)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidTransition
			}
			if locked.PaymentMethod == PaymentMethodWallet {
				if err := s.refundEscrow(ctx, tx, locked); err != nil {
					return err
				}
			}
			event := RideOrderEvent{
				OrderID:    id,
				FromStatus: &locked.Status,
				ToStatus:   statusCancelled,
				Reason:     stringPtr(reasonExpired),
			}
			if md, err := json.Marshal(map[string]string{"reason": reasonExpired}); err == nil {
				event.Metadata = md
			}
			if err := s.repo.InsertEvent(ctx, tx, event); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}()
		if txErr != nil {
			continue
		}
		cancelled++
	}
	return cancelled, nil
}

// ---- helpers transisi status ----

// validRideStatusTarget memvalidasi nilai status yang boleh dikirim ke
// PATCH /rides/:order_id/status (API_CONTRACT 7.3).
func validRideStatusTarget(s string) bool {
	switch s {
	case statusCancelled, statusDriverArrived, statusTripStarted, statusCompleted:
		return true
	default:
		return false
	}
}

// validateStatusTransition memvalidasi transisi + otorisasi aktor sesuai
// state machine order (DATABASE_SCHEMA / API_CONTRACT 7.3).
func validateStatusTransition(order *RideOrder, req UpdateRideStatusRequest) error {
	switch req.Status {
	case statusCancelled:
		if order.Status == statusCompleted {
			return ErrInvalidTransition
		}
		if req.UserID == order.CustomerID {
			return nil
		}
		if order.DriverID != nil && req.UserID == *order.DriverID {
			return nil
		}
		return ErrNotAllowed
	case statusDriverArrived:
		if order.Status != statusDriverAssigned {
			return ErrInvalidTransition
		}
		return requireAssignedDriver(order, req.UserID)
	case statusTripStarted:
		if order.Status != statusDriverArrived {
			return ErrInvalidTransition
		}
		return requireAssignedDriver(order, req.UserID)
	case statusCompleted:
		if order.Status != statusTripStarted {
			return ErrInvalidTransition
		}
		return requireAssignedDriver(order, req.UserID)
	default:
		return ErrInvalidStatus
	}
}

// requireAssignedDriver memastikan aktor adalah driver yang ditunjuk.
func requireAssignedDriver(order *RideOrder, actor uuid.UUID) error {
	if order.DriverID == nil || *order.DriverID != actor {
		return ErrNotAllowed
	}
	return nil
}

// holdEscrow memegang dana customer ke SYSTEM_ESCROW saat booking (WALLET).
//
// Urutan lock (LOCKED): baris order sudah di-tulis → lock wallets
// ORDER BY id ASC FOR UPDATE → cek saldo terkini → ledger double-entry:
//   - DEBIT customer_wallet (RIDE_ESCROW)
//   - CREDIT SYSTEM_ESCROW  (RIDE_ESCROW)
func (s *Service) holdEscrow(ctx context.Context, tx pgx.Tx, customerWallet *RideWallet, orderID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	escrowID, err := s.repo.SystemWalletID(ctx, tx, WalletTypeSystemEscrow)
	if err != nil {
		return decimal.Zero, err
	}

	// Deadlock-free: lock kedua wallet urutan ascending lalu reload balance.
	if err := lockWalletsAsc(ctx, tx, customerWallet.ID, escrowID); err != nil {
		return decimal.Zero, err
	}
	balance, err := getWalletBalanceTx(ctx, tx, customerWallet.ID)
	if err != nil {
		return decimal.Zero, err
	}
	if balance.LessThan(amount) {
		return decimal.Zero, ErrInsufficientBalance
	}

	entries := []wallet.LedgerEntry{
		{
			WalletID:      customerWallet.ID,
			EntryType:     wallet.EntryDebit,
			Amount:        amount,
			ReferenceID:   orderID,
			ReferenceType: referenceTypeRideEscrow,
			Description:   "RIDE_ESCROW - DEBIT customer wallet",
		},
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryCredit,
			Amount:        amount,
			ReferenceID:   orderID,
			ReferenceType: referenceTypeRideEscrow,
			Description:   "RIDE_ESCROW - CREDIT SYSTEM_ESCROW",
		},
	}
	if err := s.ledger.CreateLedgerEntries(ctx, tx, entries); err != nil {
		return decimal.Zero, err
	}

	return amount, nil
}

// ---- helpers internal ----

// validateBookingInput memvalidasi input booking sebelum diproses.
func (s *Service) validateBookingInput(req BookRideRequest) error {
	if req.IdempotencyKey == "" {
		return ErrIdempotencyKeyRequired
	}
	if req.PaymentMethod != PaymentMethodWallet && req.PaymentMethod != PaymentMethodCash {
		return ErrInvalidPaymentMethod
	}
	if !validLatLng(req.PickupLat, req.PickupLng) || !validLatLng(req.DropoffLat, req.DropoffLng) {
		return ErrInvalidCoordinates
	}
	if req.PickupLat == req.DropoffLat && req.PickupLng == req.DropoffLng {
		return ErrSamePickupDropoff
	}
	return nil
}

// validLatLng memastikan koordinat berada pada rentang geografis yang valid.
func validLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// validateVoucher memvalidasi kelayakan voucher untuk order ride ini
// (TD-070). Urutan pemeriksaan menentukan error yang dikembalikan:
// status -> window waktu -> min_order -> layanan yang didukung.
func validateVoucher(v *Voucher, estimatedFare decimal.Decimal) error {
	if v.Status != voucherStatusActive {
		return ErrVoucherInvalid
	}
	now := time.Now()
	if now.Before(v.ValidFrom) || now.After(v.ValidTo) {
		return ErrVoucherExpired
	}
	if estimatedFare.LessThan(v.MinOrderAmount) {
		return ErrVoucherMinOrder
	}
	for _, svc := range v.ApplicableServices {
		if strings.EqualFold(svc, orderTypeRide) {
			return nil
		}
	}
	return ErrVoucherInvalid
}

// computeVoucherDiscount menghitung besar potongan (TD-070):
//   - PERCENTAGE: estimatedFare × (discount_value/100), dibatasi max_discount
//     jika diisi, dan tidak boleh melebihi estimatedFare.
//   - FIXED: discount_value, dibatasi estimatedFare (tidak boleh > subtotal).
//
// Potongan minimal: selalu non-negatif; nilai 0 berarti tanpa efek.
func computeVoucherDiscount(v *Voucher, estimatedFare decimal.Decimal) decimal.Decimal {
	var discount decimal.Decimal
	switch v.DiscountType {
	case voucherDiscountPct:
		pct := v.DiscountValue.Div(decimal.NewFromInt(100))
		discount = estimatedFare.Mul(pct).Round(2)
		if v.MaxDiscount != nil && !v.MaxDiscount.IsZero() && discount.GreaterThan(*v.MaxDiscount) {
			discount = v.MaxDiscount.Round(2)
		}
	case voucherDiscountFixed:
		discount = v.DiscountValue.Round(2)
	}
	if discount.IsNegative() {
		return decimal.Zero
	}
	if discount.GreaterThan(estimatedFare) {
		return estimatedFare
	}
	return discount
}

// fareBasis mengembalikan besar fare yang menjadi dasar escrow/refund/
// settlement (rantai uang customer) setelah diskon voucher (TD-070):
// estimated_fare - discount_amount, minimal 0. Tanpa voucher (= 0) → equal
// estimated_fare, sehingga perilaku lama (TD-069) tidak berubah.
func fareBasis(order *RideOrder) decimal.Decimal {
	amount := order.EstimatedFare
	if order.DiscountAmount != nil && order.DiscountAmount.IsPositive() {
		amount = amount.Sub(*order.DiscountAmount)
	}
	if amount.IsNegative() {
		return decimal.Zero
	}
	return amount
}

// haversineKm menghitung jarak geodesik (km) antara dua koordinat dengan
// rumus haversine (earth radius 6371 km). Sama dengan tujuan
// earthdistance.earth_distance(ll_to_earth(...))/1000 yang dipakai layer DB
// untuk driver matching (F005).
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

// lockWalletsAsc mengunci sejumlah wallet dengan urutan id menaik dalam satu
// statement. ORDER BY id ASC membuat PostgreSQL mengambil row lock berurutan,
// sehingga mencegah deadlock antar transaksi yang mengunci wallet sama.
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

func stringPtr(s string) *string {
	return &s
}

// ---- idempotency dual-layer (selaras dengan modul wallet) ----

// redisKey membangun namespace Redis per user: idempotency:{userID}:{key}.
func redisKey(userID uuid.UUID, key string) string {
	return redisKeyPrefix + userID.String() + ":" + key
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

// idemResult hasil idempotency L2.
type idemResult struct {
	cached  json.RawMessage
	proceed bool
}

// idemAcquire adalah logika L2 idempotency (idempotency_cache).
//
//   - Tidak ditemukan -> insert PROCESSING (debounce 5 menit) -> proceed.
//   - COMPLETED       -> kembalikan response tersimpan (proceed=false).
//   - PROCESSING stale-> proceed (anggap requester sebelumnya crash).
//   - PROCESSING fresh-> error ErrIdempotencyInProgress.
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

// redisCache adalah bentuk yang disimpan di Redis L1.
type redisCache struct {
	State    string          `json:"state"`
	Response json.RawMessage `json:"response"`
}
