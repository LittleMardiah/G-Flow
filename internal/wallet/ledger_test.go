package wallet

import (
	"context"
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