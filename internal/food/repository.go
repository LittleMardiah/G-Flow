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
	ID                uuid.UUID
	UserID            uuid.UUID
	Name              string
	Description       *string
	Category          string
	Latitude          decimal.Decimal
	Longitude         decimal.Decimal
	Address           string
	Phone             *string
	AvgRating         decimal.Decimal
	TotalReviews      int
	TotalOrders       int
	OpeningTime       *string
	ClosingTime       *string
	IsOpen            bool
	Status            string
	VerifiedAt        *time.Time
	LogoURL           *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
