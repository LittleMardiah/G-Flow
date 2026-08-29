// Package wallet berisi akses data (repository) untuk modul PayPulse Wallet.
// Repository hanya bertanggung jawab mengakses database; business logic
// ditangani di Service Layer (F001). Semua query menggunakan parameterized
// statement untuk mencegah SQL injection.
package wallet

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
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrWalletAlreadyExists = errors.New("wallet already exists for this user")
)

// Wallet adalah representasi baris tabel wallets.
type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Balance   decimal.Decimal
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RepoDB adalah subset operasi pool yang dipakai Repository. Dipenuhi oleh
// *pgxpool.Pool (produksi) dan pgxmock (test).
type RepoDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Repository adalah akses data untuk tabel wallets.
type Repository struct {
	db RepoDB
}

// NewRepository membuat Repository baru dengan pool database.
func NewRepository(db RepoDB) *Repository {
	return &Repository{db: db}
}

// Create membuat wallet baru dengan saldo 0 dan status ACTIVE.
// Mengembalikan ErrWalletAlreadyExists jika user sudah memiliki wallet
// (constraint unique (user_id, wallet_type)).
func (r *Repository) Create(ctx context.Context, userID uuid.UUID, walletType string) (*Wallet, error) {
	query := `
		INSERT INTO wallets (user_id, wallet_type, balance, status)
		VALUES ($1, $2, 0, 'ACTIVE')
		RETURNING id, user_id, wallet_type, balance, status, created_at, updated_at
	`

	var w Wallet
	err := r.db.QueryRow(ctx, query, userID, walletType).Scan(
		&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrWalletAlreadyExists
		}
		return nil, err
	}

	return &w, nil
}

// GetByID mengambil wallet berdasarkan id.
// Mengembalikan ErrWalletNotFound jika tidak ada.
func (r *Repository) GetByID(ctx context.Context, walletID uuid.UUID) (*Wallet, error) {
	query := `
		SELECT id, user_id, wallet_type, balance, status, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`

	var w Wallet
	err := r.db.QueryRow(ctx, query, walletID).Scan(
		&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}

	return &w, nil
}

// GetWalletsByUserID mengambil semua wallet milik user (bisa lebih dari satu
// karena constraint UNIQUE(user_id, wallet_type)). Mengembalikan slice kosong
// jika user belum punya wallet (bukan error), karena representasi normal.
func (r *Repository) GetWalletsByUserID(ctx context.Context, userID uuid.UUID) ([]*Wallet, error) {
	query := `
		SELECT id, user_id, wallet_type, balance, status, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wallets := make([]*Wallet, 0)
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, err
		}
		wallets = append(wallets, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return wallets, nil
}

// GetByUserIDAndType mengambil wallet spesifik user berdasarkan tipe
// (misal: CUSTOMER, DRIVER, MERCHANT). Mengembalikan ErrWalletNotFound
// jika tidak ditemukan.
func (r *Repository) GetByUserIDAndType(ctx context.Context, userID uuid.UUID, walletType string) (*Wallet, error) {
	query := `
		SELECT id, user_id, wallet_type, balance, status, created_at, updated_at
		FROM wallets
		WHERE user_id = $1 AND wallet_type = $2
	`

	var w Wallet
	err := r.db.QueryRow(ctx, query, userID, walletType).Scan(
		&w.ID, &w.UserID, &w.Type, &w.Balance, &w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}

	return &w, nil
}

// UpdateStatus mengubah status wallet.
// Mengembalikan ErrWalletNotFound jika wallet tidak ditemukan (0 baris).
func (r *Repository) UpdateStatus(ctx context.Context, walletID uuid.UUID, status string) error {
	query := `
		UPDATE wallets
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	tag, err := r.db.Exec(ctx, query, status, walletID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWalletNotFound
	}

	return nil
}

// UpdateBalance menambah/mengurangi saldo wallet secara atomik.
// amount positif = CREDIT, negatif = DEBIT. Mengembalikan
// ErrWalletNotFound jika wallet tidak ditemukan (0 baris).
func (r *Repository) UpdateBalance(ctx context.Context, walletID uuid.UUID, amount decimal.Decimal) error {
	query := `
		UPDATE wallets
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2
	`

	tag, err := r.db.Exec(ctx, query, amount, walletID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWalletNotFound
	}

	return nil
}

// GetBalance mengambil saldo wallet.
// Mengembalikan ErrWalletNotFound jika wallet tidak ada.
func (r *Repository) GetBalance(ctx context.Context, walletID uuid.UUID) (decimal.Decimal, error) {
	query := `
		SELECT balance
		FROM wallets
		WHERE id = $1
	`

	var balance decimal.Decimal
	err := r.db.QueryRow(ctx, query, walletID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, ErrWalletNotFound
	}
	if err != nil {
		return decimal.Zero, err
	}

	return balance, nil
}
