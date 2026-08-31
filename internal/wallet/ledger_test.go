package wallet

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestCreateLedgerEntries_Imbalance: SUM(DEBIT) != SUM(CREDIT) -> error
// sebelum ada akses database (tx tidak dipakai, cukup passing nil).
func TestCreateLedgerEntries_Imbalance(t *testing.T) {
	refID := uuid.New()
	ls := NewLedgerService(nil)

	err := ls.CreateLedgerEntries(context.Background(), nil, []LedgerEntry{
		{WalletID: testWalletID, EntryType: EntryDebit, Amount: decimal.NewFromInt(100),
			ReferenceID: refID, ReferenceType: referenceTypeTransfer},
		{WalletID: testWalletID, EntryType: EntryCredit, Amount: decimal.NewFromInt(50),
			ReferenceID: refID, ReferenceType: referenceTypeTransfer},
	})

	assert.ErrorContains(t, err, "imbalance")
}

// TestCreateLedgerEntries_Success: batch insert dua entry (DEBIT + CREDIT)
// seimbang -> nil. Menguji success path SendBatch.
func TestCreateLedgerEntries_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()

	mDB.ExpectBegin()
	batch := mDB.ExpectBatch()
	batch.ExpectExec("INSERT INTO ledger_entries").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	batch.ExpectExec("INSERT INTO ledger_entries").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))

	tx, err := mDB.Begin(context.Background())
	assert.NoError(t, err)
	defer tx.Rollback(context.Background())

	ls := NewLedgerService(mDB)
	err = ls.CreateLedgerEntries(context.Background(), tx, []LedgerEntry{
		{WalletID: testWalletID, EntryType: EntryDebit, Amount: decimal.NewFromInt(100),
			ReferenceID: refID, ReferenceType: referenceTypeTransfer},
		{WalletID: testWalletID, EntryType: EntryCredit, Amount: decimal.NewFromInt(100),
			ReferenceID: refID, ReferenceType: referenceTypeTransfer},
	})

	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestCreateLedgerEntries_TooFewEntries: < 2 entry -> error sebelum DB.
func TestCreateLedgerEntries_TooFewEntries(t *testing.T) {
	ls := NewLedgerService(nil)

	err := ls.CreateLedgerEntries(context.Background(), nil, []LedgerEntry{
		{WalletID: testWalletID, EntryType: EntryDebit, Amount: decimal.NewFromInt(100)},
	})

	assert.ErrorContains(t, err, "minimal 2 entries")
}

// TestCreateLedgerEntries_InvalidEntryType: entry_type bukan DEBIT/CREDIT -> error.
func TestCreateLedgerEntries_InvalidEntryType(t *testing.T) {
	ls := NewLedgerService(nil)

	err := ls.CreateLedgerEntries(context.Background(), nil, []LedgerEntry{
		{WalletID: testWalletID, EntryType: "HOLD", Amount: decimal.NewFromInt(100)},
		{WalletID: testWalletID, EntryType: EntryCredit, Amount: decimal.NewFromInt(100)},
	})

	assert.ErrorContains(t, err, "entry_type tidak valid")
}

// TestCreateLedgerEntries_NonPositiveAmount: amount <= 0 -> error sebelum DB.
func TestCreateLedgerEntries_NonPositiveAmount(t *testing.T) {
	ls := NewLedgerService(nil)

	err := ls.CreateLedgerEntries(context.Background(), nil, []LedgerEntry{
		{WalletID: testWalletID, EntryType: EntryDebit, Amount: decimal.Zero},
		{WalletID: testWalletID, EntryType: EntryCredit, Amount: decimal.NewFromInt(100)},
	})

	assert.ErrorContains(t, err, "amount harus positif")
}

// TestCreateLedgerEntries_BatchExecError: error saat eksekusi batch insert ->
// error dibungkus "gagal insert entry".
func TestCreateLedgerEntries_BatchExecError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()

	mDB.ExpectBegin()
	batch := mDB.ExpectBatch()
	batch.ExpectExec("INSERT INTO ledger_entries").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	batch.ExpectExec("INSERT INTO ledger_entries").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("db down"))

	tx, err := mDB.Begin(context.Background())
	assert.NoError(t, err)
	defer tx.Rollback(context.Background())

	ls := NewLedgerService(mDB)
	err = ls.CreateLedgerEntries(context.Background(), tx, []LedgerEntry{
		{WalletID: testWalletID, EntryType: EntryDebit, Amount: decimal.NewFromInt(100),
			ReferenceID: refID, ReferenceType: referenceTypeTransfer},
		{WalletID: testWalletID, EntryType: EntryCredit, Amount: decimal.NewFromInt(100),
			ReferenceID: refID, ReferenceType: referenceTypeTransfer},
	})

	assert.ErrorContains(t, err, "ledger: gagal insert entry")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_Success: dua entry seimbang, lock wallets, update
// is_reversed, commit -> nil.
func TestReverseLedger_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()
	walletA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	walletB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	// Ambil 2 entry (DEBIT & CREDIT) milik reference.
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_id", "entry_type", "amount", "reference_type", "reference_id", "description"}).
			AddRow(uuid.New(), walletA, EntryDebit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "d").
			AddRow(uuid.New(), walletB, EntryCredit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "c"))
	// Lock wallets.
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	// Batch update is_reversed.
	batch := mDB.ExpectBatch()
	batch.ExpectExec("UPDATE ledger_entries SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	batch.ExpectExec("UPDATE ledger_entries SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit()

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_Imbalance: total DEBIT != CREDIT -> error reversal dibatalkan.
func TestReverseLedger_Imbalance(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()

	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_id", "entry_type", "amount", "reference_type", "reference_id", "description"}).
			AddRow(uuid.New(), testWalletID, EntryDebit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "d").
			AddRow(uuid.New(), testWalletID, EntryCredit, decimal.NewFromInt(50), referenceTypeTransfer, refID, "c"))
	mDB.ExpectRollback()

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.ErrorContains(t, err, "reversal dibatalkan")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_BeginTxError: error saat begin tx -> error dibungkus.
func TestReverseLedger_BeginTxError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectBeginTx(pgx.TxOptions{}).
		WillReturnError(errors.New("conn refused"))

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), uuid.New())

	assert.ErrorContains(t, err, "ledger: gagal begin tx")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_QueryError: query entries gagal -> error dibungkus.
func TestReverseLedger_QueryError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()

	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnError(errors.New("query failed"))
	mDB.ExpectRollback()

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.ErrorContains(t, err, "ledger: gagal query entries")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_CommitError: commit gagal -> error dibungkus.
func TestReverseLedger_CommitError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()
	walletA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	walletB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_id", "entry_type", "amount", "reference_type", "reference_id", "description"}).
			AddRow(uuid.New(), walletA, EntryDebit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "d").
			AddRow(uuid.New(), walletB, EntryCredit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "c"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	batch := mDB.ExpectBatch()
	batch.ExpectExec("UPDATE ledger_entries SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	batch.ExpectExec("UPDATE ledger_entries SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit().WillReturnError(errors.New("commit failed"))

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.ErrorContains(t, err, "ledger: gagal commit reversal")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_NotFound: tidak ada entry non-reversed milik reference ->
// ErrWalletNotFound.
func TestReverseLedger_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()
	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))
	mDB.ExpectRollback()

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_LockWalletError: lock wallets gagal -> error dibungkus.
func TestReverseLedger_LockWalletError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()
	walletA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	walletB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_id", "entry_type", "amount", "reference_type", "reference_id", "description"}).
			AddRow(uuid.New(), walletA, EntryDebit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "d").
			AddRow(uuid.New(), walletB, EntryCredit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "c"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(errors.New("lock failed"))
	mDB.ExpectRollback()

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.ErrorContains(t, err, "ledger: gagal lock wallets")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseLedger_ScanError: scanning baris entry gagal -> error mentah.
func TestReverseLedger_ScanError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	refID := uuid.New()

	mDB.ExpectBeginTx(pgx.TxOptions{})
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WillReturnResult(pgconn.NewCommandTag("SET"))
	// Kolom amount diisi string, bukan decimal -> scan gagal.
	mDB.ExpectQuery("SELECT id, wallet_id, entry_type").
		WithArgs(refID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_id", "entry_type", "amount", "reference_type", "reference_id", "description"}).
			AddRow(uuid.New(), uuid.New(), EntryDebit, "not-a-decimal", referenceTypeTransfer, refID, "d").
			AddRow(uuid.New(), uuid.New(), EntryCredit, decimal.NewFromInt(100), referenceTypeTransfer, refID, "c"))
	mDB.ExpectRollback()

	ls := NewLedgerService(mDB)
	err = ls.ReverseLedger(context.Background(), refID)

	assert.Error(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}
