// Package food berisi akses data (repository) untuk modul G-Food (Phase 3).
// Repository hanya bertanggung jawab mengakses database; business logic
// ditangani di Service Layer (3.2). Semua query memakai parameterized statement
// untuk mencegah SQL injection.
//
// Rujukan skema: MIGRATION 005_food_send_schema.up.sql (LOCKED) —
// food_merchants, merchant_menus, merchant_items.
package food

import (
	"context"
	"encoding/json"
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
	ErrMerchantNotFound  = errors.New("merchant not found")
	ErrMenuNotFound      = errors.New("menu not found")
	ErrItemNotFound      = errors.New("item not found")
	ErrFoodOrderNotFound = errors.New("food order not found")
	ErrLockTimeout       = errors.New("food order lock not available (NOWAIT timeout)")
)

// RepoDB adalah subset operasi pool yang dipakai Repository untuk query di
// luar transaksi. Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Querier adalah subset operasi yang bisa dipakai dalam transaksi (pgx.Tx)
// ATAU pool. Method write yang dieksekusi di dalam transaksi merchant
// register / catalog management menerima Querier agar tetap satu unit kerja
// atomik.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// MerchantUser adalah representasi subset baris users yang dibutuhkan untuk
// validasi merchant onboarding (user_type, status).
type MerchantUser struct {
	ID       uuid.UUID
	UserType string
	Status   string
}

// FoodWallet adalah representasi subset baris wallets yang dipakai modul food
// (id, user_id, wallet_type, balance, status).
type FoodWallet struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Type    string
	Balance decimal.Decimal
	Status  string
}

// Merchant adalah representasi baris tabel food_merchants (MIGRATION 005).
// Kolom nullable direpresentasikan sebagai pointer; nil berarti SQL NULL.
type Merchant struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Name         string
	Description  *string
	Category     string
	Latitude     decimal.Decimal
	Longitude    decimal.Decimal
	Address      string
	Phone        *string
	AvgRating    decimal.Decimal
	TotalReviews int
	TotalOrders  int
	OpeningTime  *string
	ClosingTime  *string
	IsOpen       bool
	Status       string
	VerifiedAt   *time.Time
	LogoURL      *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Menu adalah representasi baris tabel merchant_menus (MIGRATION 005).
type Menu struct {
	ID            uuid.UUID
	MerchantID    uuid.UUID
	Name          string
	Description   *string
	SequenceOrder int
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Item adalah representasi baris tabel merchant_items (MIGRATION 005).
type Item struct {
	ID          uuid.UUID
	MenuID      uuid.UUID
	MerchantID  uuid.UUID
	Name        string
	Description *string
	Price       decimal.Decimal
	ImageURL    *string
	Stock       int
	IsAvailable bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Repository adalah akses data untuk tabel makanan & merchant.
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan koneksi database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// GetMerchantUser mengambil data user untuk validasi merchant onboarding.
// Mengembalikan ErrUserNotFound jika user tidak ada.
func (r *Repository) GetMerchantUser(ctx context.Context, userID uuid.UUID) (*MerchantUser, error) {
	var u MerchantUser
	err := r.db.QueryRow(ctx, `
		SELECT id, user_type, status
		FROM users
		WHERE id = $1
	`, userID).Scan(&u.ID, &u.UserType, &u.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetCustomer mengambil data customer untuk validasi food order (user_type,
// status, dan overdue_debt sebagai Debt Gate Universal).
func (r *Repository) GetCustomer(ctx context.Context, userID uuid.UUID) (*FoodCustomer, error) {
	var c FoodCustomer
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

// GetMerchantByUserID mengambil merchant milik user (pemilik). Mengembalikan
// ErrMerchantNotFound jika user belum punya merchant profile.
func (r *Repository) GetMerchantByUserID(ctx context.Context, userID uuid.UUID) (*Merchant, error) {
	return scanMerchantRow(r.db.QueryRow(ctx, `SELECT `+merchantColumns+` FROM food_merchants WHERE user_id = $1`, userID))
}

// GetMerchantByID mengambil merchant berdasarkan id. Mengembalikan
// ErrMerchantNotFound jika tidak ada.
func (r *Repository) GetMerchantByID(ctx context.Context, merchantID uuid.UUID) (*Merchant, error) {
	return scanMerchantRow(r.db.QueryRow(ctx, `SELECT `+merchantColumns+` FROM food_merchants WHERE id = $1`, merchantID))
}

// InsertMerchant membuat baris food_merchants berstatus PENDING_VERIFICATION
// dalam transaksi yang sama dengan pembuatan wallet merchant.
func (r *Repository) InsertMerchant(ctx context.Context, q Querier, m *Merchant) error {
	_, err := q.Exec(ctx, `
		INSERT INTO food_merchants (
			id, user_id, merchant_name, merchant_description, category,
			latitude, longitude, address, phone,
			opening_time, closing_time, is_open, status, logo_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		m.ID,
		m.UserID,
		m.Name,
		m.Description,
		m.Category,
		m.Latitude,
		m.Longitude,
		m.Address,
		m.Phone,
		m.OpeningTime,
		m.ClosingTime,
		m.IsOpen,
		m.Status,
		m.LogoURL,
	)
	return err
}

// UpdateMerchant memperbarui field merchant secara parsial (hanya pointer
// non-nil yang diupdate via COALESCE). Dipanggil hanya oleh pemilik merchant
// (ownership divalidasi di service). Return true jika baris berubah.
func (r *Repository) UpdateMerchant(ctx context.Context, merchantID uuid.UUID,
	name, category *string, openingTime, closingTime *string, isOpen *bool, logoURL *string) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE food_merchants
		SET merchant_name = COALESCE($2, merchant_name),
		    category = COALESCE($3, category),
		    opening_time = COALESCE($4, opening_time),
		    closing_time = COALESCE($5, closing_time),
		    is_open = COALESCE($6, is_open),
		    logo_url = COALESCE($7, logo_url),
		    updated_at = NOW()
		WHERE id = $1
	`, merchantID, name, category, openingTime, closingTime, isOpen, logoURL)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// GetWalletByUserAndType mengambil wallet spesifik user berdasarkan tipe.
// Mengembalikan ErrWalletNotFound jika wallet tidak ada.
func (r *Repository) GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*FoodWallet, error) {
	var w FoodWallet
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

// InsertMerchantWallet membuat wallet bertipe MERCHANT untuk user dalam
// transaksi. Mengembalikan wallet yang baru dibuat.
func (r *Repository) InsertMerchantWallet(ctx context.Context, q Querier, userID uuid.UUID) (*FoodWallet, error) {
	var w FoodWallet
	err := q.QueryRow(ctx, `
		INSERT INTO wallets (user_id, wallet_type, balance, status)
		VALUES ($1, 'MERCHANT', 0, 'ACTIVE')
		RETURNING id, user_id, wallet_type, balance, status
	`, userID).Scan(&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// ---- merchant_menus ----

// GetMenus mengambil daftar menu milik merchant (urut sesuai sequence_order).
func (r *Repository) GetMenus(ctx context.Context, merchantID uuid.UUID) ([]*Menu, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, merchant_id, name, description, sequence_order, is_active, created_at, updated_at
		FROM merchant_menus
		WHERE merchant_id = $1
		ORDER BY sequence_order ASC, created_at ASC
	`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := make([]*Menu, 0)
	for rows.Next() {
		m, err := scanMenuRow(rows)
		if err != nil {
			return nil, err
		}
		menus = append(menus, m)
	}
	return menus, rows.Err()
}

// GetMenuByID mengambil menu berdasarkan id. Mengembalikan ErrMenuNotFound.
func (r *Repository) GetMenuByID(ctx context.Context, menuID uuid.UUID) (*Menu, error) {
	return scanMenuRow(r.db.QueryRow(ctx, `
		SELECT id, merchant_id, name, description, sequence_order, is_active, created_at, updated_at
		FROM merchant_menus WHERE id = $1
	`, menuID))
}

// InsertMenu membuat baris merchant_menus. Return false jika nama duplikat
// (constraint UNIQUE merchant_id+name → error 23505).
func (r *Repository) InsertMenu(ctx context.Context, q Querier, m *Menu) error {
	_, err := q.Exec(ctx, `
		INSERT INTO merchant_menus (id, merchant_id, name, description, sequence_order, is_active)
		VALUES ($1, $2, $3, $4, $5, TRUE)
	`, m.ID, m.MerchantID, m.Name, m.Description, m.SequenceOrder)
	return err
}

// UpdateMenu memperbarui field menu secara parsial (pointer non-nil diupdate).
// Return true jika baris berubah; ErrMenuNotFound jika menu bukan milik merchant.
func (r *Repository) UpdateMenu(ctx context.Context, menuID, merchantID uuid.UUID,
	name *string, description *string, sequenceOrder *int, isActive *bool) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE merchant_menus
		SET name = COALESCE($3, name),
		    description = COALESCE($4, description),
		    sequence_order = COALESCE($5, sequence_order),
		    is_active = COALESCE($6, is_active),
		    updated_at = NOW()
		WHERE id = $1 AND merchant_id = $2
	`, menuID, merchantID, name, description, sequenceOrder, isActive)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, ErrMenuNotFound
	}
	return true, nil
}

// DeleteMenu menghapus menu milik merchant. Return true jika baris terhapus.
func (r *Repository) DeleteMenu(ctx context.Context, menuID, merchantID uuid.UUID) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM merchant_menus WHERE id = $1 AND merchant_id = $2
	`, menuID, merchantID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---- merchant_items ----

// GetItems mengambil daftar item milik merchant.
func (r *Repository) GetItems(ctx context.Context, merchantID uuid.UUID) ([]*Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, menu_id, merchant_id, name, description, price, image_url, stock, is_available, created_at, updated_at
		FROM merchant_items
		WHERE merchant_id = $1
		ORDER BY created_at ASC
	`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*Item, 0)
	for rows.Next() {
		it, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetItemByID mengambil item berdasarkan id. Mengembalikan ErrItemNotFound.
func (r *Repository) GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	return scanItemRow(r.db.QueryRow(ctx, `
		SELECT id, menu_id, merchant_id, name, description, price, image_url, stock, is_available, created_at, updated_at
		FROM merchant_items WHERE id = $1
	`, itemID))
}

// InsertItem membuat baris merchant_items.
func (r *Repository) InsertItem(ctx context.Context, q Querier, it *Item) error {
	_, err := q.Exec(ctx, `
		INSERT INTO merchant_items (id, menu_id, merchant_id, name, description, price, image_url, stock, is_available)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, it.ID, it.MenuID, it.MerchantID, it.Name, it.Description, it.Price, it.ImageURL, it.Stock, it.IsAvailable)
	return err
}

// UpdateItem memperbarui field item secara parsial (pointer non-nil diupdate).
// Return true jika baris berubah; ErrItemNotFound jika item bukan milik merchant.
func (r *Repository) UpdateItem(ctx context.Context, itemID, merchantID uuid.UUID,
	name *string, description *string, price *decimal.Decimal, imageURL *string,
	stock *int, isAvailable *bool) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE merchant_items
		SET name = COALESCE($3, name),
		    description = COALESCE($4, description),
		    price = COALESCE($5, price),
		    image_url = COALESCE($6, image_url),
		    stock = COALESCE($7, stock),
		    is_available = COALESCE($8, is_available),
		    updated_at = NOW()
		WHERE id = $1 AND merchant_id = $2
	`, itemID, merchantID, name, description, price, imageURL, stock, isAvailable)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, ErrItemNotFound
	}
	return true, nil
}

// DeleteItem menghapus item milik merchant. Return true jika baris terhapus.
func (r *Repository) DeleteItem(ctx context.Context, itemID, merchantID uuid.UUID) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM merchant_items WHERE id = $1 AND merchant_id = $2
	`, itemID, merchantID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---- scanner helpers ----

const merchantColumns = `id, user_id, merchant_name, merchant_description, category,
	latitude, longitude, address, phone,
	avg_rating, total_reviews, total_orders,
	opening_time, closing_time, is_open, status, verified_at, logo_url,
	created_at, updated_at`

func scanMerchantRow(row pgx.Row) (*Merchant, error) {
	var m Merchant
	err := row.Scan(
		&m.ID, &m.UserID, &m.Name, &m.Description, &m.Category,
		&m.Latitude, &m.Longitude, &m.Address, &m.Phone,
		&m.AvgRating, &m.TotalReviews, &m.TotalOrders,
		&m.OpeningTime, &m.ClosingTime, &m.IsOpen, &m.Status, &m.VerifiedAt, &m.LogoURL,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMerchantNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func scanMenuRow(scanner interface{ Scan(dest ...any) error }) (*Menu, error) {
	var m Menu
	err := scanner.Scan(
		&m.ID, &m.MerchantID, &m.Name, &m.Description, &m.SequenceOrder, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMenuNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func scanItemRow(scanner interface{ Scan(dest ...any) error }) (*Item, error) {
	var it Item
	err := scanner.Scan(
		&it.ID, &it.MenuID, &it.MerchantID, &it.Name, &it.Description, &it.Price, &it.ImageURL,
		&it.Stock, &it.IsAvailable, &it.CreatedAt, &it.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrItemNotFound
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// ---- Catalog discovery (Task 3.3: 3.3.1 & 3.3.2) ----

// FoodCustomer adalah representasi subset baris users yang dibutuhkan untuk
// validasi food order (user_type, status, dan overdue_debt sebagai Debt Gate
// Universal — ROADMAP 3.3.3 langkah 1).
type FoodCustomer struct {
	ID          uuid.UUID
	UserType    string
	Status      string
	OverdueDebt decimal.Decimal
}

// SearchMerchants menelusuri katalog merchant berstatus ACTIVE dengan filter
// opsional (category, search ILIKE pada merchant_name). Jika lat/lng diberikan,
// hasil diurutkan berdasarkan jarak (earthdistance). Semua nilai parameterized;
// bagian ORDER BY hanya dipilih dari dua cabang statis (bukan input user).
func (r *Repository) SearchMerchants(ctx context.Context, category, search *string, lat, lng *float64) ([]*Merchant, error) {
	query := `SELECT ` + merchantColumns + `
		FROM food_merchants
		WHERE status = 'ACTIVE'
		  AND ($1::text IS NULL OR category = $1)
		  AND ($2::text IS NULL OR merchant_name ILIKE '%' || $2 || '%')`

	args := []any{category, search}
	if lat != nil && lng != nil {
		query += `
		  ORDER BY earth_distance(ll_to_earth($3::float8, $4::float8),
		                          ll_to_earth(latitude, longitude)) ASC`
		args = append(args, *lat, *lng)
	} else {
		query += `
		  ORDER BY avg_rating DESC, total_orders DESC`
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	merchants := make([]*Merchant, 0)
	for rows.Next() {
		m, err := scanMerchantRow(rows)
		if err != nil {
			return nil, err
		}
		merchants = append(merchants, m)
	}
	return merchants, rows.Err()
}

// GetAvailableItems mengambil item merchant yang sedang dijual
// (is_available = TRUE) untuk katalog publik (ROADMAP 3.3.2).
func (r *Repository) GetAvailableItems(ctx context.Context, merchantID uuid.UUID) ([]*Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, menu_id, merchant_id, name, description, price, image_url, stock, is_available, created_at, updated_at
		FROM merchant_items
		WHERE merchant_id = $1 AND is_available = TRUE
		ORDER BY created_at ASC
	`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*Item, 0)
	for rows.Next() {
		it, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetItemsByIDs mengambil banyak item sekaligus (untuk validasi & kalkulasi
// keranjang — ROADMAP 3.3.3) berdasarkan id.
func (r *Repository) GetItemsByIDs(ctx context.Context, ids []uuid.UUID) ([]*Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, menu_id, merchant_id, name, description, price, image_url, stock, is_available, created_at, updated_at
		FROM merchant_items
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*Item, 0)
	for rows.Next() {
		it, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// FoodOrder adalah representasi baris tabel food_orders (MIGRATION 005).
// Kolom nullable direpresentasikan sebagai pointer; nil berarti SQL NULL.
type FoodOrder struct {
	ID                  uuid.UUID        `json:"id"`
	CustomerID          uuid.UUID        `json:"customer_id"`
	MerchantID          uuid.UUID        `json:"merchant_id"`
	DriverID            *uuid.UUID       `json:"driver_id,omitempty"`
	CustomerWalletID    *uuid.UUID       `json:"customer_wallet_id,omitempty"`
	MerchantWalletID    *uuid.UUID       `json:"merchant_wallet_id,omitempty"`
	DriverWalletID      *uuid.UUID       `json:"driver_wallet_id,omitempty"`
	DeliveryAddress     string           `json:"delivery_address"`
	DeliveryLat         *decimal.Decimal `json:"delivery_lat,omitempty"`
	DeliveryLng         *decimal.Decimal `json:"delivery_lng,omitempty"`
	SpecialInstructions *string          `json:"special_instructions,omitempty"`
	ItemSubtotal        decimal.Decimal  `json:"item_subtotal"`
	DeliveryFee         decimal.Decimal  `json:"delivery_fee"`
	PlatformCommission  *decimal.Decimal `json:"platform_commission,omitempty"`
	DriverEarning       *decimal.Decimal `json:"driver_earning,omitempty"`
	DiscountAmount      decimal.Decimal  `json:"discount_amount"`
	VoucherID           *uuid.UUID       `json:"voucher_id,omitempty"`
	PaymentMethod       string           `json:"payment_method"`
	CutleryIncluded     bool             `json:"cutlery_included"`
	TotalAmount         decimal.Decimal  `json:"total_amount"`
	Status              string           `json:"status"`
	MerchantStatus      string           `json:"merchant_status"`
	MerchantNotes       *string          `json:"merchant_notes,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	ConfirmedAt         *time.Time       `json:"confirmed_at,omitempty"`
	PickupAt            *time.Time       `json:"pickup_at,omitempty"`
	DeliveredAt         *time.Time       `json:"delivered_at,omitempty"`
	SettledAt           *time.Time       `json:"settled_at,omitempty"`
	IsSettled           bool             `json:"is_settled"`
	IsRefunded          bool             `json:"is_refunded"`
}

// FoodOrderItem adalah representasi baris tabel food_order_items.
type FoodOrderItem struct {
	ID                  uuid.UUID       `json:"id"`
	OrderID             uuid.UUID       `json:"order_id"`
	ItemID              uuid.UUID       `json:"item_id"`
	ItemName            string          `json:"item_name"`
	ItemPrice           decimal.Decimal `json:"item_price"`
	Quantity            int             `json:"quantity"`
	Subtotal            decimal.Decimal `json:"subtotal"`
	Options             json.RawMessage `json:"options"`
	OptionsTotal        decimal.Decimal `json:"options_total"`
	SpecialInstructions *string         `json:"special_instructions,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

// FoodOrderEvent adalah representasi baris tabel food_order_events (audit
// trail transisi status).
type FoodOrderEvent struct {
	ID          uuid.UUID
	OrderID     uuid.UUID
	FromStatus  *string
	ToStatus    string
	Reason      *string
	TriggeredBy *uuid.UUID
	Metadata    []byte
}

// InsertFoodOrder membuat baris food_orders (status awal CREATED untuk CASH
// atau CONFIRMED untuk WALLET; merchant_status selalu WAITING). Dipanggil
// dalam transaksi yang sama dengan escrow agar order + escrow atomik.
func (r *Repository) InsertFoodOrder(ctx context.Context, q Querier, o *FoodOrder) error {
	_, err := q.Exec(ctx, `
		INSERT INTO food_orders (
			id, customer_id, merchant_id,
			customer_wallet_id, merchant_wallet_id,
			delivery_address, delivery_lat, delivery_lng, special_instructions,
			item_subtotal, delivery_fee, platform_commission,
			discount_amount, voucher_id,
			payment_method, total_amount,
			status, merchant_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`,
		o.ID,
		o.CustomerID,
		o.MerchantID,
		o.CustomerWalletID,
		o.MerchantWalletID,
		o.DeliveryAddress,
		o.DeliveryLat,
		o.DeliveryLng,
		o.SpecialInstructions,
		o.ItemSubtotal,
		o.DeliveryFee,
		o.PlatformCommission,
		o.DiscountAmount,
		o.VoucherID,
		o.PaymentMethod,
		o.TotalAmount,
		o.Status,
		o.MerchantStatus,
	)
	return err
}

// InsertFoodOrderItem membuat baris food_order_items (loop items keranjang).
// Options (JSONB) dikirim sebagai []byte yang sudah ter-marshal.
func (r *Repository) InsertFoodOrderItem(ctx context.Context, q Querier, it *FoodOrderItem) error {
	_, err := q.Exec(ctx, `
		INSERT INTO food_order_items (
			id, order_id, item_id, item_name, item_price, quantity, subtotal,
			options, options_total, special_instructions
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		it.ID, it.OrderID, it.ItemID, it.ItemName, it.ItemPrice, it.Quantity, it.Subtotal,
		it.Options, it.OptionsTotal, it.SpecialInstructions,
	)
	return err
}

// InsertFoodOrderEvent mencatat audit trail transisi status
// (food_order_events).
func (r *Repository) InsertFoodOrderEvent(ctx context.Context, q Querier, e FoodOrderEvent) error {
	_, err := q.Exec(ctx, `
		INSERT INTO food_order_events (order_id, from_status, to_status, reason, triggered_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, e.OrderID, e.FromStatus, e.ToStatus, e.Reason, e.TriggeredBy, e.Metadata)
	return err
}

// foodOrderColumns daftar kolom food_orders untuk SELECT lengkap. Dipakai
// bersama oleh GetFoodOrderByID dan LockFoodOrder agar satu sumber.
const foodOrderColumns = `id, customer_id, merchant_id, driver_id,
	customer_wallet_id, merchant_wallet_id, driver_wallet_id,
	delivery_address, delivery_lat, delivery_lng, special_instructions,
	item_subtotal, delivery_fee, platform_commission, driver_earning,
	discount_amount, voucher_id, payment_method, cutlery_included, total_amount,
	status, merchant_status, merchant_notes,
	created_at, confirmed_at, pickup_at, delivered_at, settled_at,
	is_settled, is_refunded`

// GetFoodOrderByID mengambil food order lengkap berdasarkan id.
func (r *Repository) GetFoodOrderByID(ctx context.Context, orderID uuid.UUID) (*FoodOrder, error) {
	return scanFoodOrderRow(r.db.QueryRow(ctx, `SELECT `+foodOrderColumns+` FROM food_orders WHERE id = $1`, orderID))
}

// LockFoodOrder mengambil + mengunci baris food_orders dengan SELECT ...
// FOR UPDATE NOWAIT (ROADMAP 3.3.4): dua transisi bersamaan tidak bisa saling
// menimpa. Mengembalikan ErrLockTimeout (SQLSTATE 55P03) jika lock tidak
// tersedia, atau ErrFoodOrderNotFound jika order tidak ada.
func (r *Repository) LockFoodOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*FoodOrder, error) {
	return scanFoodOrderRow(q.QueryRow(ctx, `SELECT `+foodOrderColumns+` FROM food_orders WHERE id = $1 FOR UPDATE NOWAIT`, orderID))
}

// UpdateFoodOrderStatus mengubah status food order secara atomik (CAS) dengan
// guard WHERE status = $from. Field merchant_status/is_refunded hanya diubah
// bila pointer tidak nil (COALESCE). Timestamp terkait di-set sekali saat
// transisi yang tepat. Return true jika baris berubah.
func (r *Repository) UpdateFoodOrderStatus(ctx context.Context, q Querier, orderID uuid.UUID,
	fromStatus, toStatus string, merchantStatus *string, isRefunded *bool) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE food_orders
		SET status = $3,
		    merchant_status = COALESCE($4, merchant_status),
		    is_refunded = COALESCE($5, is_refunded),
		    confirmed_at = CASE WHEN $3 = 'CONFIRMED' AND confirmed_at IS NULL THEN NOW() ELSE confirmed_at END,
		    pickup_at = CASE WHEN $3 = 'PICKED_UP' AND pickup_at IS NULL THEN NOW() ELSE pickup_at END,
		    delivered_at = CASE WHEN $3 = 'DELIVERED' AND delivered_at IS NULL THEN NOW() ELSE delivered_at END,
		    updated_at = NOW()
		WHERE id = $1 AND status = $2
	`, orderID, fromStatus, toStatus, merchantStatus, isRefunded)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// MarkFoodOrderSettled menandai food order SETTLED + is_settled + settled_at
// (dipakai SETELAH settlement ledger berhasil — Task 3.4). CAS guard status
// DELIVERED agar settlement tidak menimpa transisi lain.
func (r *Repository) MarkFoodOrderSettled(ctx context.Context, q Querier, orderID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE food_orders
		SET status = 'SETTLED', is_settled = TRUE, settled_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'DELIVERED'
	`, orderID)
	return err
}

// ResetDriverIdle menyetel working_status driver kembali ke IDLE (habis
// menyelesaikan delivery food order) — selaras dengan modul ride.
func (r *Repository) ResetDriverIdle(ctx context.Context, q Querier, driverID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE users
		SET working_status = 'IDLE', last_status_update_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, driverID)
	return err
}

// MarkDriverSuspended menyetel status driver menjadi SUSPENDED (dipakai saat
// saldo driver melewati ceiling negatif setelah CASH settlement food order).
func (r *Repository) MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE users
		SET status = 'SUSPENDED', working_status = 'IDLE',
		    last_status_update_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, driverID)
	return err
}

// GetFoodOrderItems mengambil semua item sebuah food order (urut insert).
func (r *Repository) GetFoodOrderItems(ctx context.Context, orderID uuid.UUID) ([]*FoodOrderItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, item_id, item_name, item_price, quantity, subtotal,
		       options, options_total, special_instructions, created_at
		FROM food_order_items
		WHERE order_id = $1
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*FoodOrderItem, 0)
	for rows.Next() {
		var it FoodOrderItem
		if err := rows.Scan(
			&it.ID, &it.OrderID, &it.ItemID, &it.ItemName, &it.ItemPrice, &it.Quantity, &it.Subtotal,
			&it.Options, &it.OptionsTotal, &it.SpecialInstructions, &it.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, &it)
	}
	return items, rows.Err()
}

// GetFoodOrdersByCustomer mengambil riwayat order milik customer (pagination,
// urut terbaru). Dipakai GET /food-orders.
func (r *Repository) GetFoodOrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*FoodOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+foodOrderColumns+`
		FROM food_orders
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]*FoodOrder, 0)
	for rows.Next() {
		o, err := scanFoodOrderRow(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// CountFoodOrdersByCustomer menghitung total order milik customer (untuk
// pagination meta).
func (r *Repository) CountFoodOrdersByCustomer(ctx context.Context, customerID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM food_orders WHERE customer_id = $1
	`, customerID).Scan(&count)
	return count, err
}

// --- scanner food orders ---

func scanFoodOrderRow(row pgx.Row) (*FoodOrder, error) {
	var o FoodOrder
	err := row.Scan(
		&o.ID, &o.CustomerID, &o.MerchantID, &o.DriverID,
		&o.CustomerWalletID, &o.MerchantWalletID, &o.DriverWalletID,
		&o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng, &o.SpecialInstructions,
		&o.ItemSubtotal, &o.DeliveryFee, &o.PlatformCommission, &o.DriverEarning,
		&o.DiscountAmount, &o.VoucherID, &o.PaymentMethod, &o.CutleryIncluded, &o.TotalAmount,
		&o.Status, &o.MerchantStatus, &o.MerchantNotes,
		&o.CreatedAt, &o.ConfirmedAt, &o.PickupAt, &o.DeliveredAt, &o.SettledAt,
		&o.IsSettled, &o.IsRefunded,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFoodOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
