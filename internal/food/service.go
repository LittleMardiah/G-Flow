// Package food — Service Layer (Phase 3, G-Food: Merchant Onboarding,
// Catalog Management, Food Order Creation & Cart).
//
// Service menangani business logic modul G-Food 3.2 & 3.3:
//   - Merchant onboarding (POST /merchants/register): validasi user merchant,
//     buat merchant profile (status PENDING_VERIFICATION) dan wallet MERCHANT
//     dalam satu transaksi atomik.
//   - Merchant profile (GET /merchants/{id}, PATCH /merchants/{id}) dan
//     catalog management (menu & item CRUD) — hanya pemilik merchant.
//   - Catalog discovery publik (GET /merchants, GET /merchants/{id}/items):
//     merchant ACTIVE, filter category/search, urut jarak (earthdistance).
//   - Food order creation (POST /food-orders): dual-layer idempotency
//     (Redis L1 + PostgreSQL L2 idempotency_cache), Debt Gate Universal
//     (overdue_debt), kalkulasi harga (item_subtotal + delivery_fee 20rb +
//     komisi platform 15%), escrow WALLET (lock wallet ORDER BY id ASC) dalam
//     transaksi yang sama dengan pembuatan order.
//   - Status update (PATCH /food-orders/{id}): SELECT ... FOR UPDATE NOWAIT
//   - transisi status per aktor + audit trail food_order_events.
//   - Settlement (Task 3.4): saat driver menandai DELIVERED, service otomatis
//     men-disettle order (DELIVERED → SETTLED) dalam transaksi yang sama —
//     lock wallet ORDER BY id ASC FOR UPDATE, double-entry ledger
//     FOOD_SETTLEMENT via wallet.LedgerService, cash settlement dengan cek
//     balance driver (ceiling -50.000 → SUSPENDED), dan audit trail.
//     DB migration 006 memasang guard trigger (status SETTLED ⇒ is_settled).
//
// Prinsip yang selaras dengan modul ride/wallet:
//   - Otorisasi ownership merchant/order divalidasi di service, bukan hanya
//     via RBAC role.
//   - Operasi bertransaksi memakai satu transaksi DB untuk menghindari
//     partial state (order tanpa escrow).
//   - Semua query parameterized (tanpa interpolasi string).
//   - Idempotency key wajib & response di-cache (L1 + L2).
package food

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

// Constanta domain & bisnis G-Food (MIGRATION 005 / ROADMAP 03).
const (
	WalletTypeMerchant        = "MERCHANT"
	userTypeMerchant          = "merchant"
	statusActive              = "ACTIVE"
	statusPendingVerification = "PENDING_VERIFICATION"

	defaultSeqOrder = 0

	// Task 3.3 — Payment method (payment_method_enum).
	PaymentMethodWallet = "WALLET"
	PaymentMethodCash   = "CASH"

	walletTypeCustomer     = "CUSTOMER"
	walletTypeSystemEscrow = "SYSTEM_ESCROW"

	userTypeCustomer = "customer"

	// Task 3.5.4 — Food driver accept (TD-078). user_type & working_status
	// driver food, mirror internal/send (Task 3.6 / TD-069).
	userTypeDriver    = "driver"
	workingStatusIdle = "IDLE"
	workingStatusBusy = "BUSY"
	driverUserType    = "driver"

	// driverMaxActiveFoodOrders membatasi jumlah food order aktif (status
	// READY_FOR_PICKUP/PICKED_UP/IN_TRANSIT) yang boleh dipegang satu driver
	// (Task 3.5.4 langkah 2; mirror send driverMaxActiveSendOrders=3).
	driverMaxActiveFoodOrders = 3

	// Task 3.5.4 — Food driver accept (TD-078). Masih dalam modul makanan;
	// kolom users.working_status hanya dipakai driver (mirror internal/send

	// Status food order (food_order_status_enum, ROADMAP 3.3/3.4).
	foodStatusCreated        = "CREATED"
	foodStatusConfirmed      = "CONFIRMED"
	foodStatusPreparing      = "PREPARING"
	foodStatusReadyForPickup = "READY_FOR_PICKUP"
	foodStatusPickedUp       = "PICKED_UP"
	foodStatusInTransit      = "IN_TRANSIT"
	foodStatusDelivered      = "DELIVERED"
	foodStatusCancelled      = "CANCELLED"
	foodStatusSettled        = "SETTLED"

	// merchant_status pada food_orders (jalur proses merchant).
	merchantStatusWaiting   = "WAITING"
	merchantStatusConfirmed = "CONFIRMED"
	merchantStatusPreparing = "PREPARING"
	merchantStatusReady     = "READY"

	// Idempotency dual-layer (L1 Redis + L2 idempotency_cache).
	redisKeyPrefix = "idempotency:"
	redisCompleted = "COMPLETED"
	pgProcessing   = "PROCESSING"
	redisTTL       = 24 * time.Hour

	// Reference type double-entry ledger untuk food escrow & refund.
	referenceTypeFoodEscrow = "FOOD_ESCROW"
	referenceTypeFoodRefund = "FOOD_REFUND"

	// Task 3.4 — Settlement & status machine.
	walletTypeDriver         = "DRIVER"
	walletTypeSystemPlatform = "SYSTEM_PLATFORM"
	referenceTypeFoodSettle  = "FOOD_SETTLEMENT"

	// Ceiling saldo negatif driver (LOGIC_FLOW 5.5): jika balance driver
	// < -Rp 50.000 setelah CASH settlement, akun driver di-SUSPENDED.
	maxDriverNegativeBalance = -50000
)

// Tarif G-Food (ROADMAP 3.3.3 langkah 3): delivery fee fixed Rp 20.000 dan
// komisi platform 15% dari item_subtotal.
var (
	foodDeliveryFee            = decimal.NewFromInt(20000)
	foodPlatformCommissionRate = decimal.RequireFromString("0.15")
	foodDriverEarningRate      = decimal.RequireFromString("0.90")
	foodDeliveryCommissionRate = decimal.RequireFromString("0.10")
	foodEstimatedDelivery      = "30 minutes"
)

// Error definitions untuk service layer food.
var (
	ErrNotMerchant           = errors.New("user is not a merchant")
	ErrMerchantInactive      = errors.New("merchant is not ACTIVE")
	ErrMerchantAlreadyExists = errors.New("merchant profile already exists for this user")
	ErrNotMerchantOwner      = errors.New("user is not the owner of this merchant")
	ErrInvalidCoordinates    = errors.New("invalid coordinates (latitude/longitude)")
	ErrInvalidMenu           = errors.New("menu must belong to the merchant")
	ErrInvalidPrice          = errors.New("price must be greater than zero")
	ErrEmptyName             = errors.New("name is required")

	// Task 3.3 — Food Order errors.
	ErrNotCustomer            = errors.New("user is not a customer")
	ErrCustomerInactive       = errors.New("customer is not ACTIVE")
	ErrOverdueDebt            = errors.New("customer has overdue debt (order rejected)")
	ErrWalletInactive         = errors.New("customer wallet is not ACTIVE")
	ErrMerchantWalletNotFound = errors.New("merchant wallet not found")
	ErrInsufficientBalance    = errors.New("insufficient customer wallet balance for escrow")
	ErrInvalidPaymentMethod   = errors.New("invalid payment method (must be WALLET or CASH)")
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	ErrIdempotencyInProgress  = errors.New("idempotency key is still processing")
	ErrInvalidCachedResponse  = errors.New("cached idempotency response is invalid")
	ErrInvalidItem            = errors.New("invalid item (not found, not in merchant, or unavailable)")
	ErrInsufficientStock      = errors.New("item stock is less than requested quantity")
	ErrEmptyItems             = errors.New("order must contain at least one item")
	ErrInvalidDeliveryAddress = errors.New("delivery address is required")
	ErrInvalidStatus          = errors.New("invalid status value")
	ErrInvalidTransition      = errors.New("invalid status transition for current order state")
	ErrNotAllowed             = errors.New("user is not allowed to access this order")
	ErrDriverNotFound         = errors.New("driver not found or has no DRIVER wallet")

	// Task 3.5.4 — Food driver accept (TD-078). Error validasi driver food
	// jalur READY_FOR_PICKUP (mirror internal/send Task 3.6):
	ErrNotDriver              = errors.New("user is not a driver")
	ErrDriverInactive         = errors.New("driver is not ACTIVE")
	ErrDriverBusy             = errors.New("driver working_status is not IDLE")
	ErrDriverCapacityExceeded = errors.New("driver has reached the maximum of 3 active food orders")
	ErrOrderNotReadyForAccept = errors.New("food order is not READY_FOR_PICKUP or already has a driver")
)

// Repo adalah kontrak repository yang dibutuhkan Service. Dipenuhi oleh
// *Repository (internal/food/repository.go); dijadikan interface agar mudah
// di-mock pada unit test.
type Repo interface {
	GetMerchantUser(ctx context.Context, userID uuid.UUID) (*MerchantUser, error)
	GetMerchantByUserID(ctx context.Context, userID uuid.UUID) (*Merchant, error)
	GetMerchantByID(ctx context.Context, merchantID uuid.UUID) (*Merchant, error)
	InsertMerchant(ctx context.Context, q Querier, m *Merchant) error
	InsertMerchantWallet(ctx context.Context, q Querier, userID uuid.UUID) (*FoodWallet, error)
	UpdateMerchant(ctx context.Context, merchantID uuid.UUID,
		name, category *string, openingTime, closingTime *string, isOpen *bool, logoURL *string) (bool, error)

	GetMenus(ctx context.Context, merchantID uuid.UUID) ([]*Menu, error)
	GetMenuByID(ctx context.Context, menuID uuid.UUID) (*Menu, error)
	InsertMenu(ctx context.Context, q Querier, m *Menu) error
	UpdateMenu(ctx context.Context, menuID, merchantID uuid.UUID,
		name *string, description *string, sequenceOrder *int, isActive *bool) (bool, error)
	DeleteMenu(ctx context.Context, menuID, merchantID uuid.UUID) (bool, error)

	GetItems(ctx context.Context, merchantID uuid.UUID) ([]*Item, error)
	GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error)
	InsertItem(ctx context.Context, q Querier, it *Item) error
	UpdateItem(ctx context.Context, itemID, merchantID uuid.UUID,
		name *string, description *string, price *decimal.Decimal, imageURL *string,
		stock *int, isAvailable *bool) (bool, error)
	DeleteItem(ctx context.Context, itemID, merchantID uuid.UUID) (bool, error)

	// Task 3.3 — Food Order Creation & Cart.
	GetCustomer(ctx context.Context, userID uuid.UUID) (*FoodCustomer, error)
	GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*FoodWallet, error)
	SearchMerchants(ctx context.Context, category, search *string, lat, lng *float64) ([]*Merchant, error)
	GetAvailableItems(ctx context.Context, merchantID uuid.UUID) ([]*Item, error)
	GetItemsByIDs(ctx context.Context, ids []uuid.UUID) ([]*Item, error)

	InsertFoodOrder(ctx context.Context, q Querier, o *FoodOrder) error
	InsertFoodOrderItem(ctx context.Context, q Querier, it *FoodOrderItem) error
	InsertFoodOrderEvent(ctx context.Context, q Querier, e FoodOrderEvent) error
	GetFoodOrderByID(ctx context.Context, orderID uuid.UUID) (*FoodOrder, error)
	LockFoodOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*FoodOrder, error)
	UpdateFoodOrderStatus(ctx context.Context, q Querier, orderID uuid.UUID,
		fromStatus, toStatus string, merchantStatus *string, isRefunded *bool) (bool, error)
	GetFoodOrderItems(ctx context.Context, orderID uuid.UUID) ([]*FoodOrderItem, error)
	GetFoodOrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*FoodOrder, error)
	CountFoodOrdersByCustomer(ctx context.Context, customerID uuid.UUID) (int, error)
	GetFoodOrdersByMerchant(ctx context.Context, merchantID uuid.UUID, status string, limit, offset int) ([]*FoodOrder, error)
	CountFoodOrdersByMerchant(ctx context.Context, merchantID uuid.UUID, status string) (int, error)
	SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error)

	// Task 3.4 — Settlement & status machine.
	MarkFoodOrderSettled(ctx context.Context, q Querier, orderID uuid.UUID) error
	MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error
	ResetDriverIdle(ctx context.Context, q Querier, driverID uuid.UUID) error

	// Task 3.5.4 — Food driver accept (TD-078). Mirror internal/send Task 3.6.
	GetFoodDriver(ctx context.Context, driverID uuid.UUID) (*FoodDriver, error)
	GetFoodDriverActiveFoodOrdersCount(ctx context.Context, driverID uuid.UUID) (int, error)
	// Mirror internal/send LockDriverUserForAccept (Task 3.6 langkah 3) —
	// SELECT FOR UPDATE NOWAIT + guard ACTIVE + working_status IDLE.
	// Mengembalikan driver yang sukses di-lock (dipakai audit response).
	LockFoodDriverUserForAccept(ctx context.Context, q Querier, driverID uuid.UUID) error
	LockFoodOrderForAccept(ctx context.Context, q Querier, orderID uuid.UUID) (*FoodOrder, error)
	AssignDriverToFoodOrder(ctx context.Context, q Querier, orderID uuid.UUID, driverID uuid.UUID) (bool, error)
	UpdateFoodDriverWorkingStatus(ctx context.Context, q Querier, driverID uuid.UUID, workingStatus string) error
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

// Service adalah business logic untuk modul G-Food (merchant, catalog &
// food order).
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

// ---- Merchant Onboarding (3.2.1) ----

// RegisterMerchantRequest input untuk POST /merchants/register (ROADMAP 3.2.1).
type RegisterMerchantRequest struct {
	UserID              uuid.UUID
	MerchantName        string
	MerchantDescription string
	Category            string
	Address             string
	Latitude            float64
	Longitude           float64
	Phone               string
	OpeningTime         string
	ClosingTime         string
	LogoURL             string
}

// RegisterMerchantResponse hasil onboarding (ROADMAP 3.2.1 response).
type RegisterMerchantResponse struct {
	ID           uuid.UUID `json:"id"`
	MerchantName string    `json:"merchant_name"`
	Status       string    `json:"status"`
	WalletID     uuid.UUID `json:"wallet_id"`
}

// RegisterMerchant membuat merchant profile + wallet MERCHANT dalam satu
// transaksi untuk user bertipe merchant (ROADMAP 3.2.1):
//
//  1. Validasi user: user_type='merchant', status='ACTIVE'.
//  2. Validasi user belum punya merchant profile (UNIQUE user_id).
//  3. Validasi koordinat & nama.
//  4. Transaksi atomik: INSERT food_merchants (status='PENDING_VERIFICATION',
//     is_open=TRUE) + INSERT wallets (MERCHANT).
func (s *Service) RegisterMerchant(ctx context.Context, req RegisterMerchantRequest) (*RegisterMerchantResponse, error) {
	if req.MerchantName == "" {
		return nil, ErrEmptyName
	}
	if !validLatLng(req.Latitude, req.Longitude) {
		return nil, ErrInvalidCoordinates
	}

	user, err := s.repo.GetMerchantUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if user.UserType != userTypeMerchant {
		return nil, ErrNotMerchant
	}
	if user.Status != statusActive {
		return nil, ErrMerchantInactive
	}

	if existing, err := s.repo.GetMerchantByUserID(ctx, req.UserID); err == nil && existing != nil {
		return nil, ErrMerchantAlreadyExists
	} else if err != nil && !errors.Is(err, ErrMerchantNotFound) {
		return nil, err
	}

	merchantID := uuid.New()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	var desc, phone, opening, closing, logo *string
	if req.MerchantDescription != "" {
		desc = &req.MerchantDescription
	}
	if req.Phone != "" {
		phone = &req.Phone
	}
	if req.OpeningTime != "" {
		opening = &req.OpeningTime
	}
	if req.ClosingTime != "" {
		closing = &req.ClosingTime
	}
	if req.LogoURL != "" {
		logo = &req.LogoURL
	}

	merchant := &Merchant{
		ID:          merchantID,
		UserID:      req.UserID,
		Name:        req.MerchantName,
		Description: desc,
		Category:    req.Category,
		Latitude:    decimal.NewFromFloat(req.Latitude),
		Longitude:   decimal.NewFromFloat(req.Longitude),
		Address:     req.Address,
		Phone:       phone,
		OpeningTime: opening,
		ClosingTime: closing,
		IsOpen:      true,
		Status:      statusPendingVerification,
		LogoURL:     logo,
	}
	if err := s.repo.InsertMerchant(ctx, tx, merchant); err != nil {
		return nil, err
	}

	wallet, err := s.repo.InsertMerchantWallet(ctx, tx, req.UserID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &RegisterMerchantResponse{
		ID:           merchantID,
		MerchantName: req.MerchantName,
		Status:       statusPendingVerification,
		WalletID:     wallet.ID,
	}, nil
}

// GetMerchant mengambil profile merchant lengkap berdasarkan id.
func (s *Service) GetMerchant(ctx context.Context, merchantID uuid.UUID) (*Merchant, error) {
	return s.repo.GetMerchantByID(ctx, merchantID)
}

// GetMerchantForUser mengambil profile merchant milik seorang user (pemilik).
func (s *Service) GetMerchantForUser(ctx context.Context, userID uuid.UUID) (*Merchant, error) {
	return s.repo.GetMerchantByUserID(ctx, userID)
}

// UpdateMerchantRequest input untuk PATCH /merchants/{id} (partial update).
// Hanya pointer non-nil yang diupdate.
type UpdateMerchantRequest struct {
	UserID       uuid.UUID
	MerchantID   uuid.UUID
	MerchantName *string
	Category     *string
	OpeningTime  *string
	ClosingTime  *string
	IsOpen       *bool
	LogoURL      *string
}

// UpdateMerchant memperbarui info merchant. Hanya pemilik (UserID ==
// merchant.user_id) yang boleh. Mengembalikan merchant termutakhir.
func (s *Service) UpdateMerchant(ctx context.Context, req UpdateMerchantRequest) (*Merchant, error) {
	merchant, err := s.repo.GetMerchantByID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}
	if merchant.UserID != req.UserID {
		return nil, ErrNotMerchantOwner
	}

	if _, err := s.repo.UpdateMerchant(ctx, req.MerchantID,
		req.MerchantName, req.Category, req.OpeningTime, req.ClosingTime, req.IsOpen, req.LogoURL); err != nil {
		return nil, err
	}

	return s.repo.GetMerchantByID(ctx, req.MerchantID)
}

// ---- Catalog: Menus ----

// loadOwnedMerchant memuat merchant dan memvalidasi kepemilikan oleh user,
// sekaligus memastikan merchant aktif. Mengembalikan merchant bila sah.
func (s *Service) loadOwnedMerchant(ctx context.Context, merchantID, userID uuid.UUID) (*Merchant, error) {
	merchant, err := s.repo.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if merchant.UserID != userID {
		return nil, ErrNotMerchantOwner
	}
	return merchant, nil
}

// CreateMenuRequest input untuk POST /merchants/{id}/menus.
type CreateMenuRequest struct {
	UserID        uuid.UUID
	MerchantID    uuid.UUID
	Name          string
	Description   string
	SequenceOrder int
}

// MenuResponse representasi menu yang dikembalikan ke client.
type MenuResponse struct {
	ID            uuid.UUID `json:"id"`
	MerchantID    uuid.UUID `json:"merchant_id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description"`
	SequenceOrder int       `json:"sequence_order"`
	IsActive      bool      `json:"is_active"`
}

// CreateMenu membuat menu baru milik merchant (hanya owner). Mengembalikan
// error jika nama duplikat (unique constraint).
func (s *Service) CreateMenu(ctx context.Context, req CreateMenuRequest) (*MenuResponse, error) {
	if req.Name == "" {
		return nil, ErrEmptyName
	}
	if _, err := s.loadOwnedMerchant(ctx, req.MerchantID, req.UserID); err != nil {
		return nil, err
	}

	seq := req.SequenceOrder
	if seq == 0 {
		seq = defaultSeqOrder
	}
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	menu := &Menu{
		ID:            uuid.New(),
		MerchantID:    req.MerchantID,
		Name:          req.Name,
		Description:   desc,
		SequenceOrder: seq,
		IsActive:      true,
	}
	if err := s.repo.InsertMenu(ctx, s.db, menu); err != nil {
		return nil, err
	}
	return toMenuResponse(menu), nil
}

// UpdateMenuRequest input untuk PATCH /merchants/{id}/menus/{menu_id}.
type UpdateMenuRequest struct {
	UserID        uuid.UUID
	MerchantID    uuid.UUID
	MenuID        uuid.UUID
	Name          *string
	Description   *string
	SequenceOrder *int
	IsActive      *bool
}

// UpdateMenu memperbarui menu milik merchant (hanya owner).
func (s *Service) UpdateMenu(ctx context.Context, req UpdateMenuRequest) (*MenuResponse, error) {
	if _, err := s.loadOwnedMerchant(ctx, req.MerchantID, req.UserID); err != nil {
		return nil, err
	}
	if _, err := s.repo.UpdateMenu(ctx, req.MenuID, req.MerchantID,
		req.Name, req.Description, req.SequenceOrder, req.IsActive); err != nil {
		return nil, err
	}
	menu, err := s.repo.GetMenuByID(ctx, req.MenuID)
	if err != nil {
		return nil, err
	}
	return toMenuResponse(menu), nil
}

// DeleteMenu menghapus menu milik merchant (hanya owner).
func (s *Service) DeleteMenu(ctx context.Context, merchantID, menuID, userID uuid.UUID) error {
	if _, err := s.loadOwnedMerchant(ctx, merchantID, userID); err != nil {
		return err
	}
	ok, err := s.repo.DeleteMenu(ctx, menuID, merchantID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMenuNotFound
	}
	return nil
}

// GetMenus mengambil daftar menu milik merchant (hanya owner).
func (s *Service) GetMenus(ctx context.Context, merchantID, userID uuid.UUID) ([]*MenuResponse, error) {
	if _, err := s.loadOwnedMerchant(ctx, merchantID, userID); err != nil {
		return nil, err
	}
	menus, err := s.repo.GetMenus(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	out := make([]*MenuResponse, 0, len(menus))
	for _, m := range menus {
		out = append(out, toMenuResponse(m))
	}
	return out, nil
}

// ---- Catalog: Items ----

// CreateItemRequest input untuk POST /merchants/{id}/items (ROADMAP 3.2.3).
type CreateItemRequest struct {
	UserID      uuid.UUID
	MerchantID  uuid.UUID
	MenuID      uuid.UUID
	Name        string
	Description string
	Price       decimal.Decimal
	ImageURL    string
	Stock       int
	IsAvailable bool
}

// ItemResponse representasi item yang dikembalikan ke client.
type ItemResponse struct {
	ID          uuid.UUID       `json:"id"`
	MenuID      uuid.UUID       `json:"menu_id"`
	MerchantID  uuid.UUID       `json:"merchant_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Price       decimal.Decimal `json:"price"`
	ImageURL    *string         `json:"image_url"`
	Stock       int             `json:"stock"`
	IsAvailable bool            `json:"is_available"`
}

// CreateItem membuat item baru milik merchant (hanya owner). Memvalidasi menu
// milik merchant dan harga > 0.
func (s *Service) CreateItem(ctx context.Context, req CreateItemRequest) (*ItemResponse, error) {
	if req.Name == "" {
		return nil, ErrEmptyName
	}
	if !req.Price.IsPositive() {
		return nil, ErrInvalidPrice
	}
	if _, err := s.loadOwnedMerchant(ctx, req.MerchantID, req.UserID); err != nil {
		return nil, err
	}

	// Menu harus milik merchant yang sama.
	menu, err := s.repo.GetMenuByID(ctx, req.MenuID)
	if err != nil {
		return nil, err
	}
	if menu.MerchantID != req.MerchantID {
		return nil, ErrInvalidMenu
	}

	var desc, img *string
	if req.Description != "" {
		desc = &req.Description
	}
	if req.ImageURL != "" {
		img = &req.ImageURL
	}
	stock := req.Stock
	if stock == 0 {
		stock = 999
	}

	item := &Item{
		ID:          uuid.New(),
		MenuID:      req.MenuID,
		MerchantID:  req.MerchantID,
		Name:        req.Name,
		Description: desc,
		Price:       req.Price,
		ImageURL:    img,
		Stock:       stock,
		IsAvailable: req.IsAvailable,
	}
	if err := s.repo.InsertItem(ctx, s.db, item); err != nil {
		return nil, err
	}
	return toItemResponse(item), nil
}

// UpdateItemRequest input untuk PATCH /merchants/{id}/items/{item_id}.
type UpdateItemRequest struct {
	UserID      uuid.UUID
	MerchantID  uuid.UUID
	ItemID      uuid.UUID
	Name        *string
	Description *string
	Price       *decimal.Decimal
	ImageURL    *string
	Stock       *int
	IsAvailable *bool
}

// UpdateItem memperbarui item milik merchant (hanya owner).
func (s *Service) UpdateItem(ctx context.Context, req UpdateItemRequest) (*ItemResponse, error) {
	if req.Price != nil && !req.Price.IsPositive() {
		return nil, ErrInvalidPrice
	}
	if _, err := s.loadOwnedMerchant(ctx, req.MerchantID, req.UserID); err != nil {
		return nil, err
	}
	if _, err := s.repo.UpdateItem(ctx, req.ItemID, req.MerchantID,
		req.Name, req.Description, req.Price, req.ImageURL, req.Stock, req.IsAvailable); err != nil {
		return nil, err
	}
	item, err := s.repo.GetItemByID(ctx, req.ItemID)
	if err != nil {
		return nil, err
	}
	return toItemResponse(item), nil
}

// DeleteItem menghapus item milik merchant (hanya owner).
func (s *Service) DeleteItem(ctx context.Context, merchantID, itemID, userID uuid.UUID) error {
	if _, err := s.loadOwnedMerchant(ctx, merchantID, userID); err != nil {
		return err
	}
	ok, err := s.repo.DeleteItem(ctx, itemID, merchantID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotFound
	}
	return nil
}

// GetItems mengambil daftar item milik merchant (hanya owner).
func (s *Service) GetItems(ctx context.Context, merchantID, userID uuid.UUID) ([]*ItemResponse, error) {
	if _, err := s.loadOwnedMerchant(ctx, merchantID, userID); err != nil {
		return nil, err
	}
	items, err := s.repo.GetItems(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	out := make([]*ItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toItemResponse(it))
	}
	return out, nil
}

// ---- helpers ----

func validLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

func toMenuResponse(m *Menu) *MenuResponse {
	return &MenuResponse{
		ID:            m.ID,
		MerchantID:    m.MerchantID,
		Name:          m.Name,
		Description:   m.Description,
		SequenceOrder: m.SequenceOrder,
		IsActive:      m.IsActive,
	}
}

func toItemResponse(it *Item) *ItemResponse {
	return &ItemResponse{
		ID:          it.ID,
		MenuID:      it.MenuID,
		MerchantID:  it.MerchantID,
		Name:        it.Name,
		Description: it.Description,
		Price:       it.Price,
		ImageURL:    it.ImageURL,
		Stock:       it.Stock,
		IsAvailable: it.IsAvailable,
	}
}

// ---- Task 3.3: Catalog Discovery (Catalog Search & Retrieval) ----

// MerchantSearchResult adalah satu merchant pada hasil GET /merchants,
// dilengkapi distance_km opsional bila lat/lng dikirim.
type MerchantSearchResult struct {
	Merchant   *Merchant
	DistanceKm *decimal.Decimal
}

// SearchMerchants menelusuri katalog merchant ACTIVE (ROADMAP 3.3.1).
// Filter opsional: category, search (ILIKE merchant_name), dan sortir jarak
// bila lat/lng valid diberikan (earthdistance di DB).
func (s *Service) SearchMerchants(ctx context.Context, category, search string, lat, lng float64) ([]*MerchantSearchResult, error) {
	var catPtr, searchPtr *string
	if category != "" {
		catPtr = &category
	}
	if search != "" {
		searchPtr = &search
	}

	var latPtr, lngPtr *float64
	if validLatLng(lat, lng) {
		latPtr, lngPtr = &lat, &lng
	}

	merchants, err := s.repo.SearchMerchants(ctx, catPtr, searchPtr, latPtr, lngPtr)
	if err != nil {
		return nil, err
	}

	out := make([]*MerchantSearchResult, 0, len(merchants))
	for _, m := range merchants {
		res := &MerchantSearchResult{Merchant: m}
		if latPtr != nil {
			d := haversineKm(lat, lng, m.Latitude.InexactFloat64(), m.Longitude.InexactFloat64())
			res.DistanceKm = decimalPtr(d.Round(3))
		}
		out = append(out, res)
	}
	return out, nil
}

// GetMerchantItems mengambil katalog item sebuah merchant (ROADMAP 3.3.2).
//   - Pemilik merchant (userID sesuai merchant.user_id): seluruh item
//     (termasuk yang tidak available) — menggantikan GET /:id/items Task 3.2.
//   - Selain pemilik / anonymous: hanya item is_available=TRUE dari merchant
//     ACTIVE.
func (s *Service) GetMerchantItems(ctx context.Context, merchantID, userID uuid.UUID) ([]*ItemResponse, error) {
	merchant, err := s.repo.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	var items []*Item
	if userID != uuid.Nil && merchant.UserID == userID {
		items, err = s.repo.GetItems(ctx, merchantID)
	} else {
		if merchant.Status != statusActive {
			return nil, ErrMerchantInactive
		}
		items, err = s.repo.GetAvailableItems(ctx, merchantID)
	}
	if err != nil {
		return nil, err
	}

	out := make([]*ItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toItemResponse(it))
	}
	return out, nil
}

// ---- Task 3.3: Food Order Creation (ROADMAP 3.3.3) ----

// FoodOrderItemRequest satu baris item pada keranjang (cart).
type FoodOrderItemRequest struct {
	ItemID              uuid.UUID
	Quantity            int
	Options             map[string]string
	SpecialInstructions string
}

// CreateFoodOrderRequest input untuk POST /food-orders.
type CreateFoodOrderRequest struct {
	UserID          uuid.UUID
	MerchantID      uuid.UUID
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
	PaymentMethod   string
	Items           []FoodOrderItemRequest
	IdempotencyKey  string
	VoucherID       *uuid.UUID
}

// CreateFoodOrderResponse hasil POST /food-orders (ROADMAP 3.3.3 response).
type CreateFoodOrderResponse struct {
	ID                    uuid.UUID       `json:"id"`
	Status                string          `json:"status"`
	ItemSubtotal          decimal.Decimal `json:"item_subtotal"`
	DeliveryFee           decimal.Decimal `json:"delivery_fee"`
	DiscountAmount        decimal.Decimal `json:"discount_amount"`
	TotalAmount           decimal.Decimal `json:"total_amount"`
	EstimatedDeliveryTime string          `json:"estimated_delivery_time"`
}

// itemRecord hasil lookup item keranjang + subtotal per item.
type itemRecord struct {
	item     *Item
	qty      int
	subtotal decimal.Decimal
	options  map[string]string
	note     string
}

// CreateFoodOrder membuat food order dari keranjang (ROADMAP 3.3.3):
//
//  1. Validasi input (idempotency key, payment method, alamat, items).
//  2. Idempotency L1 (Redis) → L2 (PostgreSQL idempotency_cache).
//  3. Validasi customer + Debt Gate Universal (overdue_debt = 0) — berlaku
//     untuk WALLET maupun CASH.
//  4. Validasi merchant ACTIVE + wallet customer/merchant.
//  5. Validasi semua item: milik merchant, is_available, stock >= quantity.
//  6. Hitung harga: item_subtotal → delivery_fee (20rb) → komisi platform
//     (15%) → total_amount (voucher belum didukung di tugas ini).
//  7. Satu transaksi: INSERT food_orders (+items) → (WALLET) escrow dengan
//     lock wallet ORDER BY id ASC & double-entry ledger FOOD_ESCROW.
//  8. Cache response (L1 + L2 COMPLETED).
func (s *Service) CreateFoodOrder(ctx context.Context, req CreateFoodOrderRequest) (*CreateFoodOrderResponse, error) {
	if err := s.validateFoodOrderInput(req); err != nil {
		return nil, err
	}

	// L1: Redis (scope per user agar tidak collision antar user).
	if resp, ok := s.redisGetCachedResp(ctx, req.UserID, req.IdempotencyKey); ok {
		var out CreateFoodOrderResponse
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

	customerWallet, err := s.repo.GetWalletByUserAndType(ctx, req.UserID, walletTypeCustomer)
	if err != nil {
		return nil, err
	}
	if customerWallet.Status != statusActive {
		return nil, ErrWalletInactive
	}

	// Merchant harus ACTIVE.
	merchant, err := s.repo.GetMerchantByID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}
	if merchant.Status != statusActive {
		return nil, ErrMerchantInactive
	}

	merchantWallet, err := s.repo.GetWalletByUserAndType(ctx, merchant.UserID, WalletTypeMerchant)
	if err != nil {
		return nil, err
	}
	if merchantWallet.Status != statusActive {
		return nil, ErrMerchantWalletNotFound
	}

	// Muat item & kalkulasi subtotal.
	records, itemSubtotal, err := s.buildOrderRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	if !itemSubtotal.IsPositive() {
		return nil, ErrEmptyItems
	}

	// L2: PostgreSQL idempotency.
	if res, err := s.idemAcquire(ctx, req.UserID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if !res.proceed {
		s.redisSet(ctx, req.UserID, req.IdempotencyKey, redisCompleted, res.cached)
		var out CreateFoodOrderResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Kalkulasi harga.
	deliveryFee := foodDeliveryFee
	platformCommission := itemSubtotal.Mul(foodPlatformCommissionRate).Round(2)
	discountAmount := decimal.Zero // voucher belum didukung (tabel vouchers menyusul Task 3.4+)
	totalAmount := itemSubtotal.Add(deliveryFee).Sub(discountAmount).Round(2)

	initialStatus := foodStatusCreated
	if req.PaymentMethod == PaymentMethodWallet {
		initialStatus = foodStatusConfirmed // pembayaran sudah diamankan escrow
	}

	var dLat, dLng *decimal.Decimal
	if req.DeliveryLat != 0 || req.DeliveryLng != 0 {
		dLat = decimalPtr(decimal.NewFromFloat(req.DeliveryLat))
		dLng = decimalPtr(decimal.NewFromFloat(req.DeliveryLng))
	}

	var specialNote *string
	for _, it := range req.Items {
		if it.SpecialInstructions != "" {
			note := it.SpecialInstructions
			specialNote = &note
			break
		}
	}

	orderID := uuid.New()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	order := &FoodOrder{
		ID:                  orderID,
		CustomerID:          req.UserID,
		MerchantID:          req.MerchantID,
		CustomerWalletID:    &customerWallet.ID,
		MerchantWalletID:    &merchantWallet.ID,
		DeliveryAddress:     req.DeliveryAddress,
		DeliveryLat:         dLat,
		DeliveryLng:         dLng,
		SpecialInstructions: specialNote,
		ItemSubtotal:        itemSubtotal,
		DeliveryFee:         deliveryFee,
		PlatformCommission:  decimalPtr(platformCommission),
		DiscountAmount:      discountAmount,
		VoucherID:           req.VoucherID,
		PaymentMethod:       req.PaymentMethod,
		TotalAmount:         totalAmount,
		Status:              initialStatus,
		MerchantStatus:      merchantStatusWaiting,
	}
	if err := s.repo.InsertFoodOrder(ctx, tx, order); err != nil {
		return nil, err
	}

	// Insert item keranjang (order + items atomik).
	for _, rec := range records {
		optionsJSON, _ := json.Marshal(rec.options)
		options := json.RawMessage(optionsJSON)
		if rec.options == nil {
			options = json.RawMessage(`{}`)
		}
		if err := s.repo.InsertFoodOrderItem(ctx, tx, &FoodOrderItem{
			ID:                  uuid.New(),
			OrderID:             orderID,
			ItemID:              rec.item.ID,
			ItemName:            rec.item.Name,
			ItemPrice:           rec.item.Price,
			Quantity:            rec.qty,
			Subtotal:            rec.subtotal,
			Options:             options,
			OptionsTotal:        decimal.Zero, // opsi item MVP disimpan mentah, dananya opsional
			SpecialInstructions: strPtrOrNil(rec.note),
		}); err != nil {
			return nil, err
		}
	}

	// Escrow conditional: hanya payment WALLET.
	if req.PaymentMethod == PaymentMethodWallet {
		if err := s.holdFoodEscrow(ctx, tx, order, totalAmount); err != nil {
			return nil, err
		}
	}

	// Audit trail pembuatan order.
	if err := s.repo.InsertFoodOrderEvent(ctx, tx, FoodOrderEvent{
		OrderID:     orderID,
		ToStatus:    initialStatus,
		TriggeredBy: &req.UserID,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	resp := &CreateFoodOrderResponse{
		ID:                    orderID,
		Status:                initialStatus,
		ItemSubtotal:          itemSubtotal,
		DeliveryFee:           deliveryFee,
		DiscountAmount:        discountAmount,
		TotalAmount:           totalAmount,
		EstimatedDeliveryTime: foodEstimatedDelivery,
	}
	if err := s.cacheResponse(ctx, req.UserID, req.IdempotencyKey, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// buildOrderRecords memuat item dari keranjang dan memvalidasi kepemilikan
// merchant, ketersediaan, dan stok; sekaligus menghitung item_subtotal.
func (s *Service) buildOrderRecords(ctx context.Context, req CreateFoodOrderRequest) ([]itemRecord, decimal.Decimal, error) {
	ids := make([]uuid.UUID, 0, len(req.Items))
	for _, it := range req.Items {
		ids = append(ids, it.ItemID)
	}
	items, err := s.repo.GetItemsByIDs(ctx, ids)
	if err != nil {
		return nil, decimal.Zero, err
	}
	byID := make(map[uuid.UUID]*Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}

	records := make([]itemRecord, 0, len(req.Items))
	subtotal := decimal.Zero
	for _, li := range req.Items {
		it, ok := byID[li.ItemID]
		if !ok {
			return nil, decimal.Zero, ErrInvalidItem
		}
		if it.MerchantID != req.MerchantID {
			return nil, decimal.Zero, ErrInvalidItem
		}
		if !it.IsAvailable {
			return nil, decimal.Zero, ErrInvalidItem
		}
		if it.Stock < li.Quantity {
			return nil, decimal.Zero, ErrInsufficientStock
		}
		line := it.Price.Mul(decimal.NewFromInt(int64(li.Quantity)))
		subtotal = subtotal.Add(line)
		records = append(records, itemRecord{
			item:     it,
			qty:      li.Quantity,
			subtotal: line,
			options:  li.Options,
			note:     li.SpecialInstructions,
		})
	}
	return records, subtotal, nil
}

// validateFoodOrderInput memvalidasi input POST /food-orders.
func (s *Service) validateFoodOrderInput(req CreateFoodOrderRequest) error {
	if req.IdempotencyKey == "" {
		return ErrIdempotencyKeyRequired
	}
	if req.PaymentMethod != PaymentMethodWallet && req.PaymentMethod != PaymentMethodCash {
		return ErrInvalidPaymentMethod
	}
	if req.DeliveryAddress == "" {
		return ErrInvalidDeliveryAddress
	}
	if len(req.Items) == 0 {
		return ErrEmptyItems
	}
	for _, it := range req.Items {
		if it.Quantity <= 0 {
			return ErrInvalidItem
		}
	}
	if req.DeliveryLat != 0 || req.DeliveryLng != 0 {
		if !validLatLng(req.DeliveryLat, req.DeliveryLng) {
			return ErrInvalidCoordinates
		}
	}
	return nil
}

// holdFoodEscrow memegang dana customer ke SYSTEM_ESCROW saat order WALLET.
// Urutan lock (LOCKED): baris order sudah ditulis → lock wallets
// ORDER BY id ASC FOR UPDATE → cek saldo → ledger double-entry FOOD_ESCROW:
//   - DEBIT customer_wallet
//   - CREDIT SYSTEM_ESCROW
func (s *Service) holdFoodEscrow(ctx context.Context, tx pgx.Tx, order *FoodOrder, amount decimal.Decimal) error {
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	escrowID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemEscrow)
	if err != nil {
		return err
	}
	if err := lockWalletsAsc(ctx, tx, *order.CustomerWalletID, escrowID); err != nil {
		return err
	}
	balance, err := getWalletBalanceTx(ctx, tx, *order.CustomerWalletID)
	if err != nil {
		return err
	}
	if balance.LessThan(amount) {
		return ErrInsufficientBalance
	}
	return s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryDebit,
			Amount:        amount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeFoodEscrow,
			Description:   "FOOD_ESCROW - DEBIT customer wallet",
		},
		{
			WalletID:      escrowID,
			EntryType:     wallet.EntryCredit,
			Amount:        amount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeFoodEscrow,
			Description:   "FOOD_ESCROW - CREDIT SYSTEM_ESCROW",
		},
	})
}

// refundFoodEscrow mengembalikan dana escrow ke customer saat order WALLET
// dibatalkan (CREATED/CONFIRMED). Pasangan entry FOOD_REFUND membuat total
// DEBIT == CREDIT per reference_id tetap seimbang. Dipanggil dalam transaksi
// yang sama dengan transisi status.
func (s *Service) refundFoodEscrow(ctx context.Context, tx pgx.Tx, order *FoodOrder) error {
	if order.CustomerWalletID == nil {
		return ErrWalletNotFound
	}
	escrowID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemEscrow)
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
			Amount:        order.TotalAmount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeFoodRefund,
			Description:   "FOOD_REFUND - escrow release to customer",
		},
		{
			WalletID:      *order.CustomerWalletID,
			EntryType:     wallet.EntryCredit,
			Amount:        order.TotalAmount,
			ReferenceID:   order.ID,
			ReferenceType: referenceTypeFoodRefund,
			Description:   "FOOD_REFUND - full refund customer wallet",
		},
	})
}

// ---- Task 3.4: Settlement (ROADMAP 3.4, F009) ----

// settleFoodOrderTx melakukan 4-way settlement double-entry ledger saat food
// order bertransisi DELIVERED → SETTLED (otomatis dipanggil setelah driver
// menandai DELIVERED). Locking wallet ORDER BY id ASC FOR UPDATE (deadlock-free).
//
//	WALLET: DEBIT SYSTEM_ESCROW (total_amount) → CREDIT merchant_wallet
//	        (item_subtotal - platform_commission) + CREDIT driver_wallet
//	        (driver_earning = delivery_fee * 0.9) + CREDIT SYSTEM_PLATFORM
//	        (platform_commission + delivery_fee * 0.1).
//	CASH  : customer bayar tunai ke driver; driver menyetor merchant_share +
//	        komisi platform ke wallet digital:
//	        DEBIT driver_wallet → CREDIT merchant_wallet (merchant_share)
//	        DEBIT driver_wallet → CREDIT platform_wallet (komisi item + delivery).
//	        Saldo driver boleh negatif (ceiling -Rp 50.000); jika balance
//	        < ceiling, driver di-SUSPENDED.
//
// Lalu menandai order SETTLED (is_settled = TRUE, settled_at = NOW()) dan
// menulis audit trail food_order_events (DELIVERED → SETTLED).
//
// Invariant double-entry: SUM(DEBIT) == SUM(CREDIT) per group — diverifikasi
// oleh wallet.LedgerService.CreateLedgerEntries.
func (s *Service) settleFoodOrderTx(ctx context.Context, tx pgx.Tx, order *FoodOrder) error {
	if order.DriverID == nil {
		return ErrDriverNotFound
	}
	driverWallet, err := s.repo.GetWalletByUserAndType(ctx, *order.DriverID, walletTypeDriver)
	if err != nil {
		return err
	}
	if order.MerchantWalletID == nil {
		return ErrWalletNotFound
	}
	platformID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemPlatform)
	if err != nil {
		return err
	}

	commission := decimal.Zero
	if order.PlatformCommission != nil {
		commission = order.PlatformCommission.Round(2)
	}
	merchantEarning := order.ItemSubtotal.Sub(commission).Round(2)
	deliveryCommission := order.DeliveryFee.Mul(foodDeliveryCommissionRate).Round(2)
	driverEarning := order.DeliveryFee.Mul(foodDriverEarningRate).Round(2)
	if order.DriverEarning != nil {
		driverEarning = order.DriverEarning.Round(2)
	}

	switch order.PaymentMethod {
	case PaymentMethodCash:
		// Driver memegang kas customer; menyetor bagian merchant + komisi
		// platform ke wallet digital.
		if err := lockWalletsAsc(ctx, tx, driverWallet.ID, *order.MerchantWalletID, platformID); err != nil {
			return err
		}
		if err := s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryDebit,
				Amount:        merchantEarning,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - merchant share DEBIT driver (CASH)",
			},
			{
				WalletID:      *order.MerchantWalletID,
				EntryType:     wallet.EntryCredit,
				Amount:        merchantEarning,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - merchant share CREDIT (CASH)",
			},
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryDebit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - food commission DEBIT driver (CASH)",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        commission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - food commission CREDIT (CASH)",
			},
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryDebit,
				Amount:        deliveryCommission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - delivery commission DEBIT driver (CASH)",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        deliveryCommission,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - delivery commission CREDIT (CASH)",
			},
		}); err != nil {
			return err
		}

		// Cek saldo driver SETELAH distribusi; jika menembus ceiling negatif
		// -Rp 50.000 → driver di-SUSPENDED (LOGIC_FLOW 5.5).
		balance, err := getWalletBalanceTx(ctx, tx, driverWallet.ID)
		if err != nil {
			return err
		}
		if balance.LessThan(decimal.NewFromInt(maxDriverNegativeBalance)) {
			if err := s.repo.MarkDriverSuspended(ctx, tx, *order.DriverID); err != nil {
				return err
			}
		} else if err := s.repo.ResetDriverIdle(ctx, tx, *order.DriverID); err != nil {
			return err
		}
	case PaymentMethodWallet:
		escrowID, err := s.repo.SystemWalletID(ctx, tx, walletTypeSystemEscrow)
		if err != nil {
			return err
		}
		// 4-way: escrow (DEBIT) → merchant + driver + platform (CREDIT).
		// Lock seluruh wallet dengan ORDER BY id ASC untuk mencegah deadlock.
		if err := lockWalletsAsc(ctx, tx, escrowID, *order.MerchantWalletID, driverWallet.ID, platformID); err != nil {
			return err
		}
		if err := s.ledger.CreateLedgerEntries(ctx, tx, []wallet.LedgerEntry{
			{
				WalletID:      escrowID,
				EntryType:     wallet.EntryDebit,
				Amount:        order.TotalAmount,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - release escrow to merchant/driver/platform",
			},
			{
				WalletID:      *order.MerchantWalletID,
				EntryType:     wallet.EntryCredit,
				Amount:        merchantEarning,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - merchant earning CREDIT",
			},
			{
				WalletID:      driverWallet.ID,
				EntryType:     wallet.EntryCredit,
				Amount:        driverEarning,
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - driver earning 90% delivery fee CREDIT",
			},
			{
				WalletID:      platformID,
				EntryType:     wallet.EntryCredit,
				Amount:        commission.Add(deliveryCommission),
				ReferenceID:   order.ID,
				ReferenceType: referenceTypeFoodSettle,
				Description:   "FOOD_SETTLEMENT - platform commission (food + delivery) CREDIT",
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

	// Tandai SETTLED + audit trail DELIVERED → SETTLED.
	if err := s.repo.MarkFoodOrderSettled(ctx, tx, order.ID); err != nil {
		return err
	}

	from := foodStatusDelivered
	if err := s.repo.InsertFoodOrderEvent(ctx, tx, FoodOrderEvent{
		OrderID:     order.ID,
		FromStatus:  &from,
		ToStatus:    foodStatusSettled,
		TriggeredBy: order.DriverID,
	}); err != nil {
		return err
	}
	return nil
}

// ---- Task 3.3: Status Update (ROADMAP 3.3.4) ----

// UpdateFoodOrderStatusRequest input untuk PATCH /food-orders/{id}.
type UpdateFoodOrderStatusRequest struct {
	OrderID uuid.UUID
	UserID  uuid.UUID
	Status  string
	Reason  string
}

// UpdateFoodOrderStatusResponse hasil PATCH /food-orders/{id}.
type UpdateFoodOrderStatusResponse struct {
	OrderID uuid.UUID `json:"order_id"`
	Status  string    `json:"status"`
}

// Actor kinds pada order.
const (
	actorKindCustomer = iota
	actorKindMerchant
	actorKindDriver
)

// UpdateFoodOrderStatus memproses PATCH /food-orders/{id} dengan FOR UPDATE
// NOWAIT (ROADMAP 3.3.4):
//
//   - Customer (pemilik order): CREATED/CONFIRMED → CANCELLED, dengan refund
//     escrow penuh jika payment WALLET (is_refunded = TRUE).
//   - Merchant (pemilik merchant): aksi di-gate oleh merchant_status (bukan
//     status order) — WAITING → CONFIRMED (confirm), WAITING → CANCELLED
//     (reject + refund WALLET), CONFIRMED → PREPARING → READY. WALLET order
//     lahir status CONFIRMED tapi merchant_status WAITING, sehingga merchant
//     tetap harus confirm dulu sebelum PREPARING (merchant_status ikut maju).
//   - Driver tertunjuk: READY_FOR_PICKUP → PICKED_UP → IN_TRANSIT →
//     DELIVERED; status DELIVERED otomatis memicu settlement (Task 3.4):
//     order langsung menjadi SETTLED dalam transaksi yang sama
//     (settleFoodOrderTx — 4-way WALLET / CASH).
//
// Settlement (DELIVERED → SETTLED) otomatis diimplementasi di Task 3.4.
func (s *Service) UpdateFoodOrderStatus(ctx context.Context, req UpdateFoodOrderStatusRequest) (*UpdateFoodOrderStatusResponse, error) {
	if !validFoodStatusTarget(req.Status) {
		return nil, ErrInvalidStatus
	}

	order, err := s.repo.GetFoodOrderByID(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}
	if order.Status == foodStatusCancelled || order.Status == foodStatusSettled {
		return nil, ErrInvalidTransition
	}

	// Tentukan aktor: customer, merchant owner, atau driver tertunjuk.
	merchant, err := s.repo.GetMerchantByID(ctx, order.MerchantID)
	if err != nil {
		return nil, err
	}
	actor := actorKindCustomer
	switch {
	case order.CustomerID == req.UserID:
		actor = actorKindCustomer
	case merchant.UserID == req.UserID:
		actor = actorKindMerchant
	case order.DriverID != nil && *order.DriverID == req.UserID:
		actor = actorKindDriver
	default:
		return nil, ErrNotAllowed
	}

	if err := validateFoodTransition(order, actor, req.Status); err != nil {
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
	locked, err := s.repo.LockFoodOrder(ctx, tx, req.OrderID)
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

	// merchant_status hanya maju pada transisi yang dilakukan merchant.
	var merchantStatus *string
	switch actor {
	case actorKindMerchant:
		switch req.Status {
		case foodStatusConfirmed:
			merchantStatus = strPtr(merchantStatusConfirmed)
		case foodStatusPreparing:
			merchantStatus = strPtr(merchantStatusPreparing)
		case foodStatusReadyForPickup:
			merchantStatus = strPtr(merchantStatusReady)
		}
	}

	// Refund escrow + tandai is_refunded saat customer membatalkan order
	// WALLET yang belum disettel / belum di-refund.
	var isRefunded *bool
	needRefund := false
	if req.Status == foodStatusCancelled && locked.PaymentMethod == PaymentMethodWallet &&
		!locked.IsRefunded && !locked.IsSettled {
		t := true
		isRefunded = &t
		needRefund = true
	}

	ok, err := s.repo.UpdateFoodOrderStatus(ctx, tx, req.OrderID, locked.Status, req.Status, merchantStatus, isRefunded)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidTransition
	}

	if needRefund {
		if err := s.refundFoodEscrow(ctx, tx, locked); err != nil {
			return nil, err
		}
	}

	fromStatus := locked.Status
	event := FoodOrderEvent{
		OrderID:     req.OrderID,
		FromStatus:  &fromStatus,
		ToStatus:    req.Status,
		TriggeredBy: &req.UserID,
	}
	if req.Reason != "" {
		event.Reason = strPtr(req.Reason)
	}
	if err := s.repo.InsertFoodOrderEvent(ctx, tx, event); err != nil {
		return nil, err
	}

	// Driver emergency cancel membebaskan driver: kembalikan working_status ke
	// IDLE (konsisten dengan send.cancelSendOrderTx) agar driver yang
	// membatalkan order tidak terkunci BUSY untuk order berikutnya.
	if actor == actorKindDriver && req.Status == foodStatusCancelled {
		if err := s.repo.ResetDriverIdle(ctx, tx, *order.DriverID); err != nil {
			return nil, err
		}
	}

	// Task 3.4 — Settlement otomatis: ketika driver menandai DELIVERED, order
	// langsung disettel (DELIVERED → SETTLED) dalam transaksi yang sama agar
	// order + distribusi dana atomik. Response status akhir = SETTLED.
	respStatus := req.Status
	if req.Status == foodStatusDelivered {
		if err := s.settleFoodOrderTx(ctx, tx, locked); err != nil {
			return nil, err
		}
		respStatus = foodStatusSettled
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &UpdateFoodOrderStatusResponse{OrderID: req.OrderID, Status: respStatus}, nil
}

// validFoodStatusTarget memvalidasi nilai status yang boleh dikirim ke
// PATCH /food-orders/{id}.
func validFoodStatusTarget(s string) bool {
	switch s {
	case foodStatusConfirmed, foodStatusPreparing, foodStatusReadyForPickup,
		foodStatusPickedUp, foodStatusInTransit, foodStatusDelivered, foodStatusCancelled:
		return true
	default:
		return false
	}
}

// validateFoodTransition memvalidasi transisi status per aktor sesuai state
// machine food order (ROADMAP 3.4 status flow + batasan Task 3.3). Aksi
// merchant di-gate oleh merchant_status (bukan status order) supaya order
// WALLET yang lahir status CONFIRMED tetap bisa di-confirm/di-reject merchant.
func validateFoodTransition(order *FoodOrder, actor int, target string) error {
	from := order.Status
	ms := order.MerchantStatus
	switch actor {
	case actorKindCustomer:
		if target == foodStatusCancelled && (from == foodStatusCreated || from == foodStatusConfirmed) {
			return nil
		}
	case actorKindMerchant:
		switch target {
		case foodStatusConfirmed:
			if ms == merchantStatusWaiting {
				return nil
			}
		case foodStatusCancelled:
			if ms == merchantStatusWaiting {
				return nil
			}
		case foodStatusPreparing:
			if ms == merchantStatusConfirmed {
				return nil
			}
		case foodStatusReadyForPickup:
			if ms == merchantStatusPreparing {
				return nil
			}
		}
	case actorKindDriver:
		switch target {
		case foodStatusPickedUp:
			if from == foodStatusReadyForPickup {
				return nil
			}
		case foodStatusInTransit:
			if from == foodStatusPickedUp {
				return nil
			}
		case foodStatusDelivered:
			if from == foodStatusInTransit {
				return nil
			}
		case foodStatusCancelled:
			if from == foodStatusReadyForPickup || from == foodStatusPickedUp {
				return nil
			}
		}
	default:
		return ErrNotAllowed
	}
	return ErrInvalidTransition
}

// ---- Task 3.3: Retrieval & History ----

// AcceptFoodOrderRequest — request driver food accept (TD-078 / Task 3.5.4 &
// 3.6.4). OrderID order yang di-accept, DriverID driver tertunjuk, dan
// IdempotencyKey wajib di header X-Idempotency-Key (L1 redis + L2 gateway).
// Mirror internal/send AcceptSendOrderRequest (Task 3.6 / TD-069).
type AcceptFoodOrderRequest struct {
	OrderID        uuid.UUID
	DriverID       uuid.UUID
	IdempotencyKey string
}

// AcceptFoodOrderResponse — hasil setelah driver ditetapkan. Status tetap
// READY_FOR_PICKUP (tidak ada DRIVER_ASSIGNED pada enum food_order_status);
// assignment hanya menyetel driver_id + driver_wallet_id + updated_at.
// Mirror internal/send AcceptSendOrderResponse (Task 3.6 / TD-069).
type AcceptFoodOrderResponse struct {
	ID       uuid.UUID `json:"id"`
	DriverID uuid.UUID `json:"driver_id"`
	Status   string    `json:"status"`
}

// AcceptFoodOrder menerima (assign) food driver ke food order berstatus
// READY_FOR_PICKUP (TD-078, Task 3.5.4). Membutuhkan X-Idempotency-Key yang
// wajib; alur idempotency L2 + lock FOR UPDATE NOWAIT + CAS assign mirrored
// dari internal/send AcceptSendOrder (Task 3.6, TD-069).
//
// State machine food (migration 005/009) TIDAK punya status DRIVER_ASSIGNED —
// assignment hanya menyetel driver_id, driver_wallet_id, updated_at=NOW()
// tanpa mengubah status order (tetap READY_FOR_PICKUP; dokumen: food order
// tanpa transisi status ketika driver accept). Driver yang sudah BUSY / punya
// ≥ driverMaxActiveFoodOrders order aktif tidak boleh accept.
func (s *Service) AcceptFoodOrder(ctx context.Context, req AcceptFoodOrderRequest) (*AcceptFoodOrderResponse, error) {
	if req.IdempotencyKey == "" {
		return nil, ErrIdempotencyKeyRequired
	}

	// L1: cache respon idempotency per driver (redisGetCachedResp pengembalian
	// dari response cache hasil accept sebelumnya).
	if cached, ok := s.redisGetCachedResp(ctx, req.DriverID, req.IdempotencyKey); ok {
		var out AcceptFoodOrderResponse
		if err := json.Unmarshal(cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Validasi driver food (user_type=driver, status=ACTIVE, working_status=IDLE).
	driver, err := s.repo.GetFoodDriver(ctx, req.DriverID)
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

	// L2: idempotency gateway — cegah duplikat accept saat request sama masuk.
	res, err := s.idemAcquire(ctx, req.DriverID, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if !res.proceed {
		s.redisSet(ctx, req.DriverID, req.IdempotencyKey, redisCompleted, res.cached)
		var out AcceptFoodOrderResponse
		if err := json.Unmarshal(res.cached, &out); err != nil {
			return nil, ErrInvalidCachedResponse
		}
		return &out, nil
	}

	// Kapasitas aktif driver: jumlah food order aktif (READY_FOR_PICKUP /
	// PICKED_UP / IN_TRANSIT) milik driver harus < driverMaxActiveFoodOrders.
	active, err := s.repo.GetFoodDriverActiveFoodOrdersCount(ctx, req.DriverID)
	if err != nil {
		return nil, err
	}
	if active >= driverMaxActiveFoodOrders {
		return nil, ErrDriverCapacityExceeded
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return nil, err
	}

	// Lock driver user (guard working_status IDLE) lalu lock food order
	// FOR UPDATE NOWAIT + guard READY_FOR_PICKUP + driver_id IS NULL.
	if err := s.repo.LockFoodDriverUserForAccept(ctx, tx, req.DriverID); err != nil {
		return nil, err
	}
	if _, err := s.repo.LockFoodOrderForAccept(ctx, tx, req.OrderID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrLockTimeout
		}
		if errors.Is(err, ErrFoodOrderNotFound) {
			found, gErr := s.repo.GetFoodOrderByID(ctx, req.OrderID)
			if gErr != nil {
				return nil, gErr
			}
			if found.Status != foodStatusReadyForPickup || found.DriverID != nil {
				return nil, ErrOrderNotReadyForAccept
			}
			return nil, ErrFoodOrderNotFound
		}
		return nil, err
	}

	// CAS assign driver: UPDATE ... WHERE status='READY_FOR_PICKUP' AND
	// driver_id IS NULL. False → order sudah terassign / bukan READY lagi.
	ok, err := s.repo.AssignDriverToFoodOrder(ctx, tx, req.OrderID, req.DriverID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrOrderNotReadyForAccept
	}

	// Driver working_status → BUSY (Task 3.5.4 langkah 4).
	if err := s.repo.UpdateFoodDriverWorkingStatus(ctx, tx, req.DriverID, workingStatusBusy); err != nil {
		return nil, err
	}

	// Audit trail: order status tetap READY_FOR_PICKUP, merekam penugasan
	// driver lewat food_order_events (from=to=READY_FOR_PICKUP).
	from := foodStatusReadyForPickup
	if err := s.repo.InsertFoodOrderEvent(ctx, tx, FoodOrderEvent{
		OrderID:     req.OrderID,
		FromStatus:  &from,
		ToStatus:    foodStatusReadyForPickup,
		TriggeredBy: &req.DriverID,
		Metadata:    json.RawMessage(`{"action":"driver_accept"}`),
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	resp := &AcceptFoodOrderResponse{
		ID:       req.OrderID,
		DriverID: req.DriverID,
		Status:   foodStatusReadyForPickup,
	}

	if err := s.cacheResponse(ctx, req.DriverID, req.IdempotencyKey, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ---- Task 3.3: Retrieval & History ----

// GetFoodOrder mengambil food order lengkap dengan pemeriksaan kepemilikan:
// customer pemilik, merchant owner, atau driver tertunjuk.
func (s *Service) GetFoodOrder(ctx context.Context, orderID, userID uuid.UUID) (*FoodOrder, error) {
	order, err := s.repo.GetFoodOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	merchant, err := s.repo.GetMerchantByID(ctx, order.MerchantID)
	if err != nil {
		return nil, err
	}
	if order.CustomerID == userID || merchant.UserID == userID ||
		(order.DriverID != nil && *order.DriverID == userID) {
		return order, nil
	}
	return nil, ErrNotAllowed
}

// GetFoodOrderItems mengambil daftar item sebuah food order (untuk detail).
func (s *Service) GetFoodOrderItems(ctx context.Context, orderID uuid.UUID) ([]*FoodOrderItem, error) {
	return s.repo.GetFoodOrderItems(ctx, orderID)
}

// GetFoodOrderHistory mengembalikan riwayat order customer dengan pagination
// (page mulai 1, page_size 1..50, default 20).
func (s *Service) GetFoodOrderHistory(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*FoodOrder, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	total, err := s.repo.CountFoodOrdersByCustomer(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.repo.GetFoodOrdersByCustomer(ctx, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// GetMerchantOrders mengembalikan daftar food order milik merchant dengan
// pagination (page mulai 1, page_size 1..50, default 20) dan filter status
// opsional (status order ATAU merchant_status). Hanya merchant pemilik yang
// bisa mengakses: merchant di-resolve dari userID (GetMerchantByUserID), lalu
// merchantID di URL harus cocok — selain itu ErrNotMerchantOwner (403).
func (s *Service) GetMerchantOrders(ctx context.Context, userID, merchantID uuid.UUID, status string, page, pageSize int) ([]*FoodOrder, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	merchant, err := s.repo.GetMerchantByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if merchant.ID != merchantID {
		return nil, 0, ErrNotMerchantOwner
	}
	total, err := s.repo.CountFoodOrdersByMerchant(ctx, merchantID, status)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.repo.GetFoodOrdersByMerchant(ctx, merchantID, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ---- idempotency dual-layer (selaras dengan modul ride/wallet) ----

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

// decimalPtr mengembalikan pointer decimal.
func decimalPtr(d decimal.Decimal) *decimal.Decimal {
	return &d
}
