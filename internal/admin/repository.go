// Package admin — Transaction Reversal & Admin Tooling (Task 4.2.4).
//
// Menyediakan akses data untuk reversal transaksi: membaca ledger entries,
// melakukan balik-balik entry, mengambil saldo wallet, memperbarui saldo,
// membuat log aksi admin, dan memeriksa role user. Paket ini mandiri dari
// internal/wallet agar logika clawback proporsional reversal terpisah jelas
// dan dapat diuji (dependency injection via interface).
package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// DB adalah subset operasi pool yang dipakai Repository & Service.
// Dipenuhi oleh *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// LedgerEntry adalah representasi baris tabel ledger_entries yang dipakai
// algoritma reversal (subset field yang relevan).
type LedgerEntry struct {
	ID            uuid.UUID
	WalletID      uuid.UUID
	EntryType     string // DEBIT | CREDIT
	Amount        decimal.Decimal
	ReferenceType string
	ReferenceID   uuid.UUID
	Description   string
	IsReversed    bool
	CreatedAt     time.Time
}

// WalletBalance adalah representasi saldo sebuah wallet saat ini.
type WalletBalance struct {
	WalletID uuid.UUID
	Balance  decimal.Decimal
}

// Repository adalah akses data untuk modul admin.
type Repository struct {
	db DB
}

// NewRepository membuat Repository baru dengan pool database.
func NewRepository(db DB) *Repository {
	return &Repository{db: db}
}

// GetLedgerEntries mengambil semua entry non-reversed milik reference_id.
// Digunakan untuk memetakan bagian merchant/driver/platform saat clawback.
func (r *Repository) GetLedgerEntries(ctx context.Context, tx pgx.Tx, referenceID uuid.UUID) ([]LedgerEntry, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, wallet_id, entry_type, amount, reference_type, reference_id, description, is_reversed, created_at
		FROM ledger_entries
		WHERE reference_id = $1 AND is_reversed = FALSE
		ORDER BY created_at ASC
	`, referenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.WalletID, &e.EntryType, &e.Amount, &e.ReferenceType,
			&e.ReferenceID, &e.Description, &e.IsReversed, &e.CreatedAt,
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

// GetAllLedgerEntries mengambil SEMUA entry (termasuk is_reversed) milik
// reference_id. Dipakai untuk menampilkan detail transaksi sebelum reversal.
func (r *Repository) GetAllLedgerEntries(ctx context.Context, tx pgx.Tx, referenceID uuid.UUID) ([]LedgerEntry, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, wallet_id, entry_type, amount, reference_type, reference_id, description, is_reversed, created_at
		FROM ledger_entries
		WHERE reference_id = $1
		ORDER BY created_at ASC
	`, referenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.WalletID, &e.EntryType, &e.Amount, &e.ReferenceType,
			&e.ReferenceID, &e.Description, &e.IsReversed, &e.CreatedAt,
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

// GetWalletTypes mengambil pemetaan wallet_id -> wallet_type untuk wallet yang
// terlibat agar service bisa membedakan escrow/merchant/driver/platform.
func (r *Repository) GetWalletTypes(ctx context.Context, tx pgx.Tx, walletIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(walletIDs) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT id, wallet_type FROM wallets WHERE id = ANY($1)
	`, walletIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]string, len(walletIDs))
	for rows.Next() {
		var id uuid.UUID
		var wt string
		if err := rows.Scan(&id, &wt); err != nil {
			return nil, err
		}
		result[id] = wt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// LedgerInsert adalah entry yang akan dimasukkan saat reversal (compensating).
type LedgerInsert struct {
	WalletID      uuid.UUID
	EntryType     string // DEBIT | CREDIT
	Amount        decimal.Decimal
	ReferenceType string
	ReferenceID   uuid.UUID
	Description   string
	CreatedBy     *uuid.UUID
}

// InsertLedgerEntries memasukkan batch compensating entries ke ledger_entries.
// Pemanggil wajib mengunci wallet terkait (GetWalletBalances) dalam transaksi
// yang sama agar trigger sync_wallet_balance aman.
func (r *Repository) InsertLedgerEntries(ctx context.Context, tx pgx.Tx, entries []LedgerInsert) error {
	if len(entries) == 0 {
		return nil
	}
	for _, e := range entries {
		var createdBy any
		if e.CreatedBy != nil {
			createdBy = *e.CreatedBy
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO ledger_entries (wallet_id, entry_type, amount, reference_type, reference_id, description, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, e.WalletID, e.EntryType, e.Amount, e.ReferenceType, e.ReferenceID, e.Description, createdBy); err != nil {
			return err
		}
	}
	return nil
}

// UpdateLedgerReversal menandai seluruh entry reference_id sebagai is_reversed
// = TRUE (invariant immutability: hanya kolom is_reversed yang boleh berubah).
// Trigger `sync_wallet_balance` akan mengembalikan saldo wallet ke nilai
// sebelum entry asli.
func (r *Repository) UpdateLedgerReversal(ctx context.Context, tx pgx.Tx, referenceID uuid.UUID, entryIDs []uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE ledger_entries SET is_reversed = TRUE WHERE id = ANY($1)
	`, entryIDs)
	return err
}

// GetWalletBalances mengambil saldo beberapa wallet sekaligus (FOR UPDATE,
// urutan deterministik untuk mencegah deadlock saat multi-wallet reversal).
func (r *Repository) GetWalletBalances(ctx context.Context, tx pgx.Tx, walletIDs []uuid.UUID) ([]WalletBalance, error) {
	if len(walletIDs) == 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT id, balance FROM wallets WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE
	`, walletIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []WalletBalance
	for rows.Next() {
		var b WalletBalance
		if err := rows.Scan(&b.WalletID, &b.Balance); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return balances, nil
}

// UpdateWalletBalance menambah/mengurangi saldo wallet secara eksplisit
// (amount positif = CREDIT, negatif = DEBIT). Dipakai untuk auto-sweep dan
// penyesuaian yang tidak melalui batch ledger trigger.
func (r *Repository) UpdateWalletBalance(ctx context.Context, tx pgx.Tx, walletID uuid.UUID, delta decimal.Decimal) error {
	_, err := tx.Exec(ctx, `
		UPDATE wallets SET balance = balance + $1, updated_at = NOW() WHERE id = $2
	`, delta, walletID)
	return err
}

// CreateAdminActionLog menulis baris audit aksi admin (reversal, dsb).
func (r *Repository) CreateAdminActionLog(ctx context.Context, tx pgx.Tx, adminID uuid.UUID, action, entityType string, entityID, referenceID *uuid.UUID, details map[string]any) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return err
	}
	var eID, rID any
	if entityID != nil {
		eID = *entityID
	}
	if referenceID != nil {
		rID = *referenceID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_action_logs (admin_id, action, entity_type, entity_id, reference_id, details)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, adminID, action, entityType, eID, rID, detailsJSON)
	return err
}

// LoginUser adalah baris hasil query login (subset kolom users).
type LoginUser struct {
	UserID   uuid.UUID
	Email    string
	UserType string
	Status   string
	Hash     string
}

// GetLoginUser mengambil data login user berdasarkan email.
// Mengembalikan pgx.ErrNoRows jika email tidak terdaftar.
func (r *Repository) GetLoginUser(ctx context.Context, email string) (LoginUser, error) {
	var u LoginUser
	err := r.db.QueryRow(ctx,
		`SELECT id, email, user_type, status, password_hash FROM users WHERE email = $1`,
		email,
	).Scan(&u.UserID, &u.Email, &u.UserType, &u.Status, &u.Hash)
	return u, err
}

// GetUserRole mengambil user_type dari users berdasarkan id user.
// Mengembalikan error jika admin tidak ditemukan.
func (r *Repository) GetUserRole(ctx context.Context, adminID uuid.UUID) (string, error) {
	var role string
	err := r.db.QueryRow(ctx,
		`SELECT user_type FROM users WHERE id = $1`, adminID,
	).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

// SystemWalletID mengambil id wallet sistem (user_id IS NULL) berdasar tipe,
// misal SYSTEM_ESCROW / SYSTEM_RECEIVABLE_OVERDRAFT.
func (r *Repository) SystemWalletID(ctx context.Context, tx pgx.Tx, walletType string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM wallets WHERE wallet_type = $1 AND user_id IS NULL
	`, walletType).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, ErrWalletNotFound
	}
	return id, err
}
