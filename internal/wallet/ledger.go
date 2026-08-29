// Package wallet — Ledger Service (F002): double-entry bookkeeping.
//
// Prinsip: setiap transaksi finansial menciptakan pasangan DEBIT & CREDIT
// dengan nilai identik. Invariant: SUM(DEBIT) == SUM(CREDIT) per reference_id.
//
// Trigger DB `sync_wallet_balance` (BEFORE INSERT) otomatis memutakhirkan
// `wallets.balance` dan `ledger_entries.balance_after`. Karena itu service
// layer TIDAK memanggil UpdateBalance secara manual untuk operasi ledger.
// Semua insert ledger HARUS dilakukan dalam transaksi yang sudah mengunci
// wallet terkait (ORDER BY id ASC FOR UPDATE) di service layer.
package wallet

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// EntryType constants.
const (
	EntryDebit  = "DEBIT"
	EntryCredit = "CREDIT"
)

// LedgerEntry adalah representasi baris tabel ledger_entries.
type LedgerEntry struct {
	ID            uuid.UUID
	WalletID      uuid.UUID
	EntryType     string
	Amount        decimal.Decimal
	BalanceAfter  decimal.Decimal
	ReferenceID   uuid.UUID
	ReferenceType string
	Description   string
	IsReversed    bool
	CreatedAt     time.Time
}

// LedgerDB adalah subset operasi pool yang dipakai LedgerService untuk
// memulai transaksi sendiri (reversal). Dipenuhi oleh *pgxpool.Pool
// (produksi) dan pgxmock (test).
type LedgerDB interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// LedgerService menyediakan operasi double-entry bookkeeping.
// Menyimpan pool db untuk operasi yang memulai transaksi sendiri (reversal).
type LedgerService struct {
	db LedgerDB
}

// NewLedgerService membuat LedgerService baru.
func NewLedgerService(db LedgerDB) *LedgerService {
	return &LedgerService{db: db}
}

// CreateLedgerEntries melakukan batch insert ke tabel ledger_entries.
//
// VALIDASI DOUBLE-ENTRY: sebelum insert, memastikan SUM(DEBIT) == SUM(CREDIT)
// untuk grup entries tersebut. Mengembalikan error jika tidak seimbang.
//
// Catatan: fungsi ini TIDAK melakukan lock wallet — pemanggil (service layer)
// wajib mengunci semua wallet terkait (ORDER BY id ASC FOR UPDATE) dalam
// transaksi yang sama sebelum memanggil ini, agar trigger balance aman.
func (s *LedgerService) CreateLedgerEntries(ctx context.Context, tx pgx.Tx, entries []LedgerEntry) error {
	if len(entries) < 2 {
		return errors.New("ledger: minimal 2 entries (DEBIT dan CREDIT)")
	}

	var totalDebit, totalCredit decimal.Decimal
	for _, e := range entries {
		if e.EntryType != EntryDebit && e.EntryType != EntryCredit {
			return fmt.Errorf("ledger: entry_type tidak valid: %q", e.EntryType)
		}
		if !e.Amount.IsPositive() {
			return errors.New("ledger: amount harus positif")
		}
		switch e.EntryType {
		case EntryDebit:
			totalDebit = totalDebit.Add(e.Amount)
		case EntryCredit:
			totalCredit = totalCredit.Add(e.Amount)
		}
	}

	if !totalDebit.Equal(totalCredit) {
		return fmt.Errorf(
			"ledger: imbalance DEBIT(%s) != CREDIT(%s)",
			totalDebit.String(), totalCredit.String(),
		)
	}

	const query = `
		INSERT INTO ledger_entries
			(wallet_id, entry_type, amount, reference_type, reference_id, description)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(
			query,
			e.WalletID,
			e.EntryType,
			e.Amount,
			e.ReferenceType,
			e.ReferenceID,
			e.Description,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for range entries {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("ledger: gagal insert entry: %w", err)
		}
	}

	return nil
}

// ReverseLedger membalikkan seluruh transaksi yang ditandai reference_id.
//
// Alur:
//  1. Mulai transaksi baru.
//  2. Ambil semua entry non-reversed milik reference_id.
//  3. Kunci wallet yang terlibat (urutan asc) untuk menjaga konsistensi.
//  4. Buat compensating entries (entry_type dibalik) dengan description REVERSAL.
//  5. Tandai entry asli is_reversed = TRUE.
//  6. Commit.
//
// Trigger immutability mengizinkan update hanya pada kolom is_reversed.
func (s *LedgerService) ReverseLedger(ctx context.Context, referenceID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("ledger: gagal begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3000ms'"); err != nil {
		return err
	}

	// Ambil entry asli (belum di-reverse) milik reference ini.
	rows, err := tx.Query(ctx, `
		SELECT id, wallet_id, entry_type, amount, reference_type, reference_id, description
		FROM ledger_entries
		WHERE reference_id = $1 AND is_reversed = FALSE
	`, referenceID)
	if err != nil {
		return fmt.Errorf("ledger: gagal query entries: %w", err)
	}

	type original struct {
		id            uuid.UUID
		walletID      uuid.UUID
		entryType     string
		amount        decimal.Decimal
		referenceType string
		referenceID   uuid.UUID
		description   string
	}
	var originals []original
	walletSet := make(map[uuid.UUID]struct{})

	for rows.Next() {
		var o original
		if err := rows.Scan(
			&o.id, &o.walletID, &o.entryType, &o.amount,
			&o.referenceType, &o.referenceID, &o.description,
		); err != nil {
			rows.Close()
			return err
		}
		originals = append(originals, o)
		walletSet[o.walletID] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(originals) == 0 {
		return ErrWalletNotFound
	}

	// Seimbangkan dulu: jumlah DEBIT harus = CREDIT sebelum reversal.
	var td, tc decimal.Decimal
	for _, o := range originals {
		if o.entryType == EntryDebit {
			td = td.Add(o.amount)
		} else {
			tc = tc.Add(o.amount)
		}
	}
	if !td.Equal(tc) {
		return fmt.Errorf("ledger: reversal dibatalkan, imbalance DEBIT(%s) != CREDIT(%s)", td.String(), tc.String())
	}

	// Kunci wallet terlibat dengan urutan deterministik.
	walletIDs := make([]uuid.UUID, 0, len(walletSet))
	for id := range walletSet {
		walletIDs = append(walletIDs, id)
	}
	sort.Slice(walletIDs, func(i, j int) bool { return walletIDs[i].String() < walletIDs[j].String() })

	_, err = tx.Exec(ctx, `
		SELECT id FROM wallets WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE
	`, walletIDs)
	if err != nil {
		return fmt.Errorf("ledger: gagal lock wallets: %w", err)
	}

	// Balikkan masing-masing entry asli dengan menandai is_reversed = TRUE.
	// Trigger `sync_wallet_balance` (BEFORE UPDATE OF is_reversed) otomatis
	// mengembalikan saldo wallet ke nilai sebelum entry. Menandai is_reversed
	// SUDAH membalik saldo, sehingga tanpa memasukkan compensating entries
	// (yang akan menggandakan pembalikan — Bug: balance menjadi 2x dari asal).
	batch := &pgx.Batch{}
	for _, o := range originals {
		batch.Queue(
			`UPDATE ledger_entries SET is_reversed = TRUE WHERE id = $1`,
			o.id,
		)
	}

	br := tx.SendBatch(ctx, batch)

	for range originals {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("ledger: gagal eksekusi reversal: %w", err)
		}
	}

	// Lepas koneksi batch SEBELUM commit. Tampa ini, tx.Commit memakai koneksi
	// yang masih dipegang batch -> error "conn busy".
	if err := br.Close(); err != nil {
		return fmt.Errorf("ledger: gagal menutup batch reversal: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ledger: gagal commit reversal: %w", err)
	}

	return nil
}
