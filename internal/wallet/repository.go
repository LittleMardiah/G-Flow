// Package wallet berisi akses data (repository) untuk modul PayPulse Wallet.
// Repository hanya bertanggung jawab mengakses database; business logic
// ditangani di Service Layer (F001). Semua query menggunakan parameterized
// statement untuk mencegah SQL injection.
package wallet

import (
	"context"
	"errors"
	"fmt"
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
	ErrWithdrawalNotFound  = errors.New("withdrawal tidak ditemukan")
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

// GetWalletOwner mengambil user_id pemilik wallet. Mengembalikan
// ErrWalletNotFound jika wallet tidak ada.
func (r *Repository) GetWalletOwner(ctx context.Context, walletID uuid.UUID) (uuid.UUID, error) {
	query := `
		SELECT user_id
		FROM wallets
		WHERE id = $1
	`

	var owner uuid.UUID
	err := r.db.QueryRow(ctx, query, walletID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrWalletNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}

	return owner, nil
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

// UpdateWalletStatus mengubah status wallet dan mengembalikan updated_at
// terbaru. Mengembalikan ErrWalletNotFound jika wallet tidak ditemukan
// (0 baris). Dipakai admin update status wallet (TD-059).
func (r *Repository) UpdateWalletStatus(ctx context.Context, walletID uuid.UUID, newStatus string) (time.Time, error) {
	query := `
		UPDATE wallets
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.db.QueryRow(ctx, query, newStatus, walletID).Scan(&updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrWalletNotFound
	}
	if err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
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

// ListLedgerEntries mengambil riwayat ledger entries milik wallet tertentu,
// diurutkan created_at DESC (terbaru dulu). refType kosong berarti tanpa
// filter reference_type. limit/offset untuk pagination. Query memakai kolom
// yang ter-cover idx_ledger_wallet + idx_ledger_created. Kolom nullable
// di-COALESCE agar hasil deterministic.
func (r *Repository) ListLedgerEntries(ctx context.Context, walletID uuid.UUID, refType string, limit, offset int) ([]LedgerEntry, error) {
	query := `
		SELECT id, wallet_id, entry_type, amount,
		       COALESCE(balance_after, 0) AS balance_after,
		       COALESCE(reference_type, '') AS reference_type,
		       COALESCE(reference_id, '00000000-0000-0000-0000-000000000000') AS reference_id,
		       COALESCE(description, '') AS description,
		       is_reversed,
		       COALESCE(created_at, NOW()) AS created_at
		FROM ledger_entries
		WHERE wallet_id = $1`
	args := []any{walletID}
	if refType != "" {
		query += ` AND reference_type = $2`
		args = append(args, refType)
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]LedgerEntry, 0)
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.WalletID, &e.EntryType, &e.Amount, &e.BalanceAfter,
			&e.ReferenceType, &e.ReferenceID, &e.Description,
			&e.IsReversed, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// CountLedgerEntries menghitung total ledger entries milik wallet untuk
// pagination. refType kosong berarti tanpa filter reference_type.
func (r *Repository) CountLedgerEntries(ctx context.Context, walletID uuid.UUID, refType string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM ledger_entries
		WHERE wallet_id = $1`
	args := []any{walletID}
	if refType != "" {
		query += ` AND reference_type = $2`
		args = append(args, refType)
	}

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// WithdrawalRequest adalah representasi satu baris withdrawal_requests.
// Response DTO memetakan Status PENDING -> PENDING_APPROVAL di handler layer
// (lihat comment migration 020).
type WithdrawalRequest struct {
	ID                uuid.UUID       `json:"id"`
	UserID            uuid.UUID       `json:"user_id"`
	WalletID          uuid.UUID       `json:"wallet_id"`
	Amount            decimal.Decimal `json:"amount"`
	Fee               decimal.Decimal `json:"fee"`
	NetAmount         decimal.Decimal `json:"net_amount"`
	BankName          string          `json:"bank_name"`
	BankAccountNumber string          `json:"bank_account_number"`
	BankAccountName   string          `json:"bank_account_name"`
	Status            string          `json:"status"`
	RequestedAt       time.Time       `json:"requested_at"`
	ProcessedAt       *time.Time      `json:"processed_at,omitempty"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	LedgerEntryID     *uuid.UUID      `json:"ledger_entry_id,omitempty"`
	AdminNotes        *string         `json:"admin_notes,omitempty"`
	RejectionReason   *string         `json:"rejection_reason,omitempty"`
}

const withdrawalColumns = `id, user_id, wallet_id, amount, fee, net_amount,
    bank_name, bank_account_number, bank_account_name, status,
    requested_at, processed_at, completed_at, ledger_entry_id,
    admin_notes, rejection_reason`

// InsertWithdrawal menyimpan pengajuan withdrawal baru (status PENDING).
// Caller WAJIB memastikan wallet aktif + saldo cukup (dilakukan di Service).
func (r *Repository) InsertWithdrawal(ctx context.Context, w *WithdrawalRequest) (*WithdrawalRequest, error) {
	var out WithdrawalRequest
	err := r.db.QueryRow(ctx, `
		INSERT INTO withdrawal_requests (
			user_id, wallet_id, amount, fee, net_amount,
			bank_name, bank_account_number, bank_account_name, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PENDING')
		RETURNING `+withdrawalColumns+`
	`, w.UserID, w.WalletID, w.Amount, w.Fee, w.NetAmount,
		w.BankName, w.BankAccountNumber, w.BankAccountName).Scan(
		&out.ID, &out.UserID, &out.WalletID, &out.Amount, &out.Fee,
		&out.NetAmount, &out.BankName, &out.BankAccountNumber,
		&out.BankAccountName, &out.Status, &out.RequestedAt,
		&out.ProcessedAt, &out.CompletedAt, &out.LedgerEntryID,
		&out.AdminNotes, &out.RejectionReason,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWithdrawalByID membaca satu pengajuan. ErrWithdrawalNotFound bila tidak ada.
func (r *Repository) GetWithdrawalByID(ctx context.Context, id uuid.UUID) (*WithdrawalRequest, error) {
	var out WithdrawalRequest
	err := r.db.QueryRow(ctx, `
		SELECT `+withdrawalColumns+`
		FROM withdrawal_requests WHERE id = $1
	`, id).Scan(
		&out.ID, &out.UserID, &out.WalletID, &out.Amount, &out.Fee,
		&out.NetAmount, &out.BankName, &out.BankAccountNumber,
		&out.BankAccountName, &out.Status, &out.RequestedAt,
		&out.ProcessedAt, &out.CompletedAt, &out.LedgerEntryID,
		&out.AdminNotes, &out.RejectionReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWithdrawalNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListWithdrawalsByUser mengembalikan pengajuan user, terbaru dulu.
// limit <= 0 -> default 20; max 100.
func (r *Repository) ListWithdrawalsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]WithdrawalRequest, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+withdrawalColumns+`
		FROM withdrawal_requests
		WHERE user_id = $1
		ORDER BY requested_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WithdrawalRequest
	for rows.Next() {
		var w WithdrawalRequest
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.WalletID, &w.Amount, &w.Fee,
			&w.NetAmount, &w.BankName, &w.BankAccountNumber,
			&w.BankAccountName, &w.Status, &w.RequestedAt,
			&w.ProcessedAt, &w.CompletedAt, &w.LedgerEntryID,
			&w.AdminNotes, &w.RejectionReason,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// UpdateWithdrawalStatus mengubah status pengajuan (CAS by expectedStatus)
// dan opsional set completed_at/ledger_entry_id. Return rows affected.
// Caller WAJIB menjalankan dalam transaksi + sudah memegang lock via
// GetWithdrawalForUpdate.
func (r *Repository) UpdateWithdrawalStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, expectedStatus, newStatus string, ledgerEntryID *uuid.UUID) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE withdrawal_requests
		SET status = $1::varchar,
		    completed_at = CASE WHEN $1::varchar = 'COMPLETED' THEN NOW() ELSE completed_at END,
		    processed_at = COALESCE(processed_at, NOW()),
		    ledger_entry_id = COALESCE($3, ledger_entry_id)
		WHERE id = $2 AND status = $4
	`, newStatus, id, ledgerEntryID, expectedStatus)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetWithdrawalForUpdate memuat pengajuan dengan FOR UPDATE (dalam transaksi).
func (r *Repository) GetWithdrawalForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*WithdrawalRequest, error) {
	var out WithdrawalRequest
	err := tx.QueryRow(ctx, `
		SELECT `+withdrawalColumns+`
		FROM withdrawal_requests WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&out.ID, &out.UserID, &out.WalletID, &out.Amount, &out.Fee,
		&out.NetAmount, &out.BankName, &out.BankAccountNumber,
		&out.BankAccountName, &out.Status, &out.RequestedAt,
		&out.ProcessedAt, &out.CompletedAt, &out.LedgerEntryID,
		&out.AdminNotes, &out.RejectionReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWithdrawalNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLedgerEntryIDByReference mencari id ledger entry berdasar reference
// (dipakai untuk link ledger_entry_id ke withdrawal_requests).
func (r *Repository) GetLedgerEntryIDByReference(ctx context.Context, tx pgx.Tx, refType string, refID uuid.UUID, entryType string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM ledger_entries
		WHERE reference_type = $1 AND reference_id = $2 AND entry_type = $3
		ORDER BY created_at ASC
		LIMIT 1
	`, refType, refID, entryType).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
