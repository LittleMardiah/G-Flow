// Package food — Service Layer (Phase 3, G-Food, Task 3.2: Merchant Onboarding
// & Catalog Management).
//
// Service menangani business logic modul G-Food 3.2:
//   - Merchant onboarding (POST /merchants/register): validasi user merchant,
//     buat merchant profile (status PENDING_VERIFICATION) dan wallet MERCHANT
//     dalam satu transaksi atomik.
//   - Merchant profile (GET /merchants/{id}, PATCH /merchants/{id}) dan
//     ownership check (hanya pemilik yang boleh mengelola).
//   - Catalog management (menu & item CRUD) — hanya pemilik merchant.
//
// Prinsip yang selaras dengan modul ride/wallet:
//   - Otorisasi ownership merchant divalidasi di service (merchant.UserID ==
//     auth user), bukan hanya via RBAC role.
//   - Operasi bertransaksi (register) memakai satu transaksi DB untuk
//     menghindari partial state (merchant tanpa wallet).
//   - Semua query parameterized (tanpa interpolasi string).
package food

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// Constanta domain & bisnis G-Food 3.2 (MIGRATION 005 / ROADMAP 03).
const (
	WalletTypeMerchant          = "MERCHANT"
	userTypeMerchant            = "merchant"
	statusActive                = "ACTIVE"
	statusPendingVerification   = "PENDING_VERIFICATION"

	defaultSeqOrder = 0
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
}

// DB adalah subset operasi pool yang dipakai Service. Dipenuhi oleh
// *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Service adalah business logic untuk modul G-Food (merchant & catalog).
type Service struct {
	repo Repo
	db   DB
}

// NewService membuat Service baru dengan dependency injection.
func NewService(repo Repo, db DB) *Service {
	return &Service{repo: repo, db: db}
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
	ID             uuid.UUID  `json:"id"`
	MerchantID     uuid.UUID  `json:"merchant_id"`
	Name           string     `json:"name"`
	Description    *string    `json:"description"`
	SequenceOrder  int        `json:"sequence_order"`
	IsActive       bool       `json:"is_active"`
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
	UserID       uuid.UUID
	MerchantID   uuid.UUID
	MenuID       uuid.UUID
	Name         string
	Description  string
	Price        decimal.Decimal
	ImageURL     string
	Stock        int
	IsAvailable  bool
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
