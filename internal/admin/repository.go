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
	"errors"
	"fmt"
	"strings"
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
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
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

// OrderStatusCount adalah pasangan status order (teks enum) dengan jumlah
// order pada status tersebut (agregasi ride/food/send).
type OrderStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// RecentTransaction adalah satu baris ledger terbaru untuk halaman dashboard
// admin (subset kolom ledger_entries + status turunan is_reversed).
type RecentTransaction struct {
	ID        uuid.UUID
	WalletID  uuid.UUID
	EntryType string
	Amount    decimal.Decimal
	Reference string
	CreatedAt time.Time
	Status    string
	Note      string
}

// CountActiveOrders menghitung jumlah order yang sedang berjalan di semua
// layanan (ride/food/send). Order "aktif" = status bukan terminal, yaitu
// bukan CANCELLED dan bukan SETTLED (SETTLED = settlement final).
func (r *Repository) CountActiveOrders(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM ride_orders WHERE status::text NOT IN ('CANCELLED', 'SETTLED')
			UNION ALL
			SELECT 1 FROM food_orders WHERE status::text NOT IN ('CANCELLED', 'SETTLED')
			UNION ALL
			SELECT 1 FROM send_orders WHERE status::text NOT IN ('CANCELLED', 'SETTLED')
		) t
	`).Scan(&count)
	return count, err
}

// SumTransactionVolume24h menjumlahkan nilai transaksi (sisi DEBIT perganda)
// yang masuk dalam 24 jam terakhir, tidak termasuk entry reversal.
func (r *Repository) SumTransactionVolume24h(ctx context.Context) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM ledger_entries
		WHERE is_reversed = FALSE AND entry_type = 'DEBIT'
		  AND created_at >= NOW() - INTERVAL '24 hours'
	`).Scan(&total)
	return total, err
}

// AvgFare24h menghitung rata-rata ongkos order (ride actual/estimated fare,
// food total_amount, send total_fare) yang dibuat 24 jam terakhir, tidak
// termasuk order yang dibatalkan (CANCELLED).
func (r *Repository) AvgFare24h(ctx context.Context) (decimal.Decimal, error) {
	var avg decimal.Decimal
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(x.fare), 0) FROM (
			SELECT COALESCE(actual_fare, estimated_fare) AS fare
			FROM ride_orders
			WHERE created_at >= NOW() - INTERVAL '24 hours' AND status::text NOT IN ('CANCELLED')
			UNION ALL
			SELECT total_amount FROM food_orders
			WHERE created_at >= NOW() - INTERVAL '24 hours' AND status::text NOT IN ('CANCELLED')
			UNION ALL
			SELECT total_fare FROM send_orders
			WHERE created_at >= NOW() - INTERVAL '24 hours' AND status::text NOT IN ('CANCELLED')
		) x
	`).Scan(&avg)
	return avg, err
}

// RevenueToday menjumlahkan komisi platform (platform_commission) dari order
// yang di-settle hari ini di semua layanan.
func (r *Repository) RevenueToday(ctx context.Context) (decimal.Decimal, error) {
	var revenue decimal.Decimal
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(x.commission), 0) FROM (
			SELECT COALESCE(platform_commission, 0) AS commission
			FROM ride_orders WHERE is_settled = TRUE AND settled_at::date = CURRENT_DATE
			UNION ALL
			SELECT COALESCE(platform_commission, 0) FROM food_orders
			WHERE is_settled = TRUE AND settled_at::date = CURRENT_DATE
			UNION ALL
			SELECT COALESCE(platform_commission, 0) FROM send_orders
			WHERE is_settled = TRUE AND settled_at::date = CURRENT_DATE
		) x
	`).Scan(&revenue)
	return revenue, err
}

// CountOrdersByStatus menghitung jumlah order per status (label enum di-cast
// ke teks lalu digabung antar layanan; label yang sama dijumlahkan).
func (r *Repository) CountOrdersByStatus(ctx context.Context) ([]OrderStatusCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT status::text, COUNT(*) FROM (
			SELECT status::text AS status FROM ride_orders
			UNION ALL
			SELECT status::text AS status FROM food_orders
			UNION ALL
			SELECT status::text AS status FROM send_orders
		) t
		GROUP BY status::text ORDER BY status::text
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderStatusCount
	for rows.Next() {
		var s OrderStatusCount
		if err := rows.Scan(&s.Status, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListRecentTransactions mengambil daftar transaksi ledger terbaru (pagination)
// beserta total seluruh baris. Status diturunkan dari is_reversed:
// "REVERSED" kalau sudah di-reverse, selain itu "ACTIVE". Field reference
// dibentuk dari reference_type + "/" + reference_id (mengikuti pola LedgerItem).
func (r *Repository) ListRecentTransactions(ctx context.Context, limit, offset int) ([]RecentTransaction, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM ledger_entries`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, wallet_id, entry_type, amount,
		       COALESCE(reference_type, '') || '/' || COALESCE(reference_id::text, '') AS reference,
		       created_at,
		       CASE WHEN is_reversed THEN 'REVERSED' ELSE 'ACTIVE' END AS status,
		       COALESCE(description, '') AS note
		FROM ledger_entries
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []RecentTransaction
	for rows.Next() {
		var t RecentTransaction
		if err := rows.Scan(&t.ID, &t.WalletID, &t.EntryType, &t.Amount, &t.Reference, &t.CreatedAt, &t.Status, &t.Note); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// LedgerFilter adalah filter daftar / export ledger admin (STEP A3, E1 & E3).
// DateFrom/DateTo bernilai nil bila filter tanggal tidak dipakai. Limit/Offset
// hanya dipakai untuk query ber-pagination (ListLedger); export memakai semua
// baris yang cocok.
type LedgerFilter struct {
	Offset     int
	Limit      int
	DateFrom   *time.Time
	DateTo     *time.Time
	WalletType string
	EntryType  string
	Search     string
}

const ledgerEntrySelect = `
	SELECT le.id, le.wallet_id, le.entry_type, le.amount,
	       COALESCE(le.reference_type, '') || '/' || COALESCE(le.reference_id::text, '') AS reference,
	       le.created_at,
	       CASE WHEN le.is_reversed THEN 'REVERSED' ELSE 'ACTIVE' END AS status,
	       COALESCE(le.description, '') AS note
	FROM ledger_entries le
	LEFT JOIN wallets w ON w.id = le.wallet_id`

// ledgerFilterWhere membangun fragment WHERE (+ optional LIMIT/OFFSET) untuk
// query ledger admin dengan placeholder dinamis sesuai filter yang terisi.
func ledgerFilterWhere(f LedgerFilter, paginate bool) (where, order string, args []any) {
	conds := []string{"TRUE"}
	idx := 1
	if f.DateFrom != nil {
		conds = append(conds, fmt.Sprintf("le.created_at >= $%d", idx))
		args = append(args, *f.DateFrom)
		idx++
	}
	if f.DateTo != nil {
		conds = append(conds, fmt.Sprintf("le.created_at <= $%d", idx))
		args = append(args, *f.DateTo)
		idx++
	}
	if f.WalletType != "" {
		conds = append(conds, fmt.Sprintf("w.wallet_type::text = $%d", idx))
		args = append(args, f.WalletType)
		idx++
	}
	if f.EntryType != "" {
		conds = append(conds, fmt.Sprintf("le.entry_type::text = $%d", idx))
		args = append(args, f.EntryType)
		idx++
	}
	if f.Search != "" {
		conds = append(conds, fmt.Sprintf(
			"(le.reference_type ILIKE '%%' || $%d || '%%' OR le.reference_id::text ILIKE '%%' || $%d || '%%' OR COALESCE(le.description, '') ILIKE '%%' || $%d || '%%')",
			idx, idx, idx,
		))
		args = append(args, f.Search, f.Search, f.Search)
		idx++
	}
	where = strings.Join(conds, " AND ")
	if paginate {
		order = fmt.Sprintf(" ORDER BY le.created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, f.Limit, f.Offset)
	} else {
		order = " ORDER BY le.created_at DESC"
	}
	return where, order, args
}

// scanRecentTransactions membaca baris RecentTransaction dari pgx.Rows.
func scanRecentTransactions(rows pgx.Rows) ([]RecentTransaction, error) {
	var out []RecentTransaction
	for rows.Next() {
		var t RecentTransaction
		if err := rows.Scan(&t.ID, &t.WalletID, &t.EntryType, &t.Amount, &t.Reference, &t.CreatedAt, &t.Status, &t.Note); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListLedger mengambil daftar ledger entries sesuai filter (pagination). JOIN
// wallets hanya untuk filter wallet_type (kolom ada di tabel wallets, bukan
// ledger_entries).
func (r *Repository) ListLedger(ctx context.Context, f LedgerFilter) ([]RecentTransaction, error) {
	where, order, args := ledgerFilterWhere(f, true)
	rows, err := r.db.Query(ctx, ledgerEntrySelect+" WHERE "+where+order, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecentTransactions(rows)
}

// CountLedger menghitung total baris ledger sesuai filter (untuk total_count).
func (r *Repository) CountLedger(ctx context.Context, f LedgerFilter) (int64, error) {
	where, _, args := ledgerFilterWhere(f, false)
	var total int64
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM ledger_entries le LEFT JOIN wallets w ON w.id = le.wallet_id WHERE "+where,
		args...,
	).Scan(&total)
	return total, err
}

// ListLedgerForExport mengambil SEMUA ledger entries sesuai filter tanpa
// pagination (untuk export CSV, E3).
func (r *Repository) ListLedgerForExport(ctx context.Context, f LedgerFilter) ([]RecentTransaction, error) {
	where, order, args := ledgerFilterWhere(f, false)
	rows, err := r.db.Query(ctx, ledgerEntrySelect+" WHERE "+where+order, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecentTransactions(rows)
}

// LedgerTotals adalah hasil verifikasi ledger milik sebuah wallet: saldo saat
// ini (wallets.balance) vs agregat DEBIT/CREDIT entries non-reversed.
type LedgerTotals struct {
	WalletBalance decimal.Decimal
	TotalDebit    decimal.Decimal
	TotalCredit   decimal.Decimal
}

// VerifyWalletLedger membaca saldo wallet + total DEBIT/CREDIT ledger-nya.
// Wallet tidak ada -> ErrWalletNotFound.
func (r *Repository) VerifyWalletLedger(ctx context.Context, walletID uuid.UUID) (*LedgerTotals, error) {
	balance := decimal.Zero
	err := r.db.QueryRow(ctx, `SELECT balance FROM wallets WHERE id = $1`, walletID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}

	var debit, credit decimal.Decimal
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount) FILTER (WHERE entry_type = 'DEBIT'), 0),
		       COALESCE(SUM(amount) FILTER (WHERE entry_type = 'CREDIT'), 0)
		FROM ledger_entries
		WHERE wallet_id = $1 AND is_reversed = FALSE
	`, walletID).Scan(&debit, &credit)
	if err != nil {
		return nil, err
	}
	return &LedgerTotals{WalletBalance: balance, TotalDebit: debit, TotalCredit: credit}, nil
}
