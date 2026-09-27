package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/wallet"
)

// mockLedger memenuhi interface Ledger worker (double-entry ledger entries).
type mockLedger struct {
	mock.Mock
}

func (m *mockLedger) CreateLedgerEntries(_ context.Context, _ pgx.Tx, entries []wallet.LedgerEntry) error {
	args := m.Called(entries)
	return args.Error(0)
}

// ---- fixtures ----

var (
	workerOrderID    = uuid.MustParse("a1111111-1111-1111-1111-111111111111")
	workerOrderID2   = uuid.MustParse("a1111111-1111-1111-1111-111111111112")
	workerOrderID3   = uuid.MustParse("a1111111-1111-1111-1111-111111111113")
	workerCustWallet = uuid.MustParse("b2222222-2222-2222-2222-222222222222")
	workerEscrowID   = uuid.MustParse("c3333333-3333-3333-3333-333333333333")
	workerFare       = decimal.NewFromInt(50000)
)

func newTestWorker(t *testing.T, mDB pgxmock.PgxPoolIface) (*Worker, *mockLedger) {
	t.Helper()
	repo := NewRepository(mDB)
	lgr := new(mockLedger)
	return NewWorker(repo, mDB, nil, lgr), lgr
}

// setupLedgerCapture mengatur mockLedger untuk menangkap entries lalu return nil.
func setupLedgerCapture(lgr *mockLedger) *[]wallet.LedgerEntry {
	var entries []wallet.LedgerEntry
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) {
			entries = args.Get(0).([]wallet.LedgerEntry)
		}).
		Return(nil)
	return &entries
}

// seqLedger adalah double Ledger yang mengembalikan error berurutan (satu per
// pemanggilan). Diperlukan untuk skenario multi-order karena testify mock selalu
// memilih ekspektasi pertama yang belum habis, sehingga cannot menyatukan
// "berhasil" dan "gagal" pada satu mockLedger.
type seqLedger struct {
	mu    sync.Mutex
	errs  []error
	calls [][]wallet.LedgerEntry
}

func (s *seqLedger) CreateLedgerEntries(_ context.Context, _ pgx.Tx, entries []wallet.LedgerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := len(s.calls)
	s.calls = append(s.calls, entries)
	if i < len(s.errs) {
		return s.errs[i]
	}
	return nil
}

// assertRefundEntries memvalidasi double-entry refund escrow WALLET.
func assertRefundEntries(t *testing.T, entries *[]wallet.LedgerEntry, escrowID, walletID, orderID uuid.UUID, fare decimal.Decimal, refType string) {
	t.Helper()
	require.NotNil(t, entries)
	require.Len(t, *entries, 2)

	e0 := (*entries)[0]
	assert.Equal(t, wallet.EntryDebit, e0.EntryType)
	assert.Equal(t, escrowID, e0.WalletID)
	assert.Equal(t, fare, e0.Amount)
	assert.Equal(t, orderID, e0.ReferenceID)
	assert.Equal(t, refType, e0.ReferenceType)

	e1 := (*entries)[1]
	assert.Equal(t, wallet.EntryCredit, e1.EntryType)
	assert.Equal(t, walletID, e1.WalletID)
	assert.Equal(t, fare, e1.Amount)
	assert.Equal(t, orderID, e1.ReferenceID)
	assert.Equal(t, refType, e1.ReferenceType)
}

// expectEmptySweep mengatur ekspektasi 3 query ID expired kosong (food → send
// → ride), yaitu sweep yang tidak menemukan order untuk diproses.
func expectEmptySweep(mDB pgxmock.PgxPoolIface) {
	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))
	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))
}

// ---- CancelRideOrders ----

func TestCancelRideOrders_NoExpiredOrders(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_WalletRefund(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "estimated_fare"}).
			AddRow(workerOrderID, &custWallet, paymentMethodWallet, rideStatusSearchingDriver, workerFare))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerEscrowID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	entries := setupLedgerCapture(lgr)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelRideOrders(context.Background())

	assert.Equal(t, 1, got)
	assertRefundEntries(t, entries, workerEscrowID, workerCustWallet, workerOrderID, workerFare, referenceTypeRideRefund)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_CashNoRefund(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "estimated_fare"}).
			AddRow(workerOrderID, &custWallet, "CASH", rideStatusSearchingDriver, workerFare))
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelRideOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_AlreadyCancelled_Skip(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "estimated_fare"}).
			AddRow(workerOrderID, &custWallet, paymentMethodWallet, rideStatusCancelled, workerFare))

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- CancelRideOrders: error paths (TD-106) ----

// expectRideOrderIDs mengatur ekspektasi query ID ride order expired.
func expectRideOrderIDs(mDB pgxmock.PgxPoolIface, rows *pgxmock.Rows) {
	mDB.ExpectQuery("SELECT id FROM ride_orders").WillReturnRows(rows)
}

// expectRideTxOpen mengatur ekspektasi transaksi sampai baris order ter-lock:
// Begin, SET LOCAL statement_timeout, SELECT ... FOR UPDATE NOWAIT. Parameter
// wallet boleh nil (kolom NULL) untuk menguji guard wallet. Query ID expired
// hanya dieksekusi sekali per sweep, jadi dipisah lewat expectRideOrderIDs.
func expectRideTxOpen(mDB pgxmock.PgxPoolIface, id uuid.UUID, wallet any, payment, status string, fare decimal.Decimal) {
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "estimated_fare"}).
			AddRow(id, wallet, payment, status, fare))
}

// expectRideOneOrder menggabungkan query ID expired (satu order) dengan
// opensource transaksi sampai baris order ter-lock.
func expectRideOneOrder(mDB pgxmock.PgxPoolIface, id uuid.UUID, wallet any, payment, status string, fare decimal.Decimal) {
	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(id))
	expectRideTxOpen(mDB, id, wallet, payment, status, fare)
}

// expectEscrowLookupAndWalletLock mengatur query wallet escrow + lock wallets
// id menaik (dipakai sebelum CreateLedgerEntries untuk order payment WALLET).
func expectEscrowLookupAndWalletLock(mDB pgxmock.PgxPoolIface) {
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerEscrowID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
}

func TestCancelRideOrders_QueryError_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnError(errors.New("conn refused"))

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_QueryRowsErr_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).
			AddRow(workerOrderID).
			CloseError(errors.New("connection reset by peer")))

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_BeginError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin().WillReturnError(errors.New("begin failed"))

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_StatementTimeoutSetupError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnError(errors.New("permission denied"))
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_LockTimeout55P03_SkipThenContinue(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	// Order 1: lock NOWAIT ditolak instance lain (55P03) → skip.
	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).
		AddRow(workerOrderID).AddRow(workerOrderID2))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID).
		WillReturnError(&pgconn.PgError{Code: "55P03"})
	mDB.ExpectRollback()

	// Order 2: CASH (tanpa refund) → sukses, sweep tidak boleh berhenti.
	expectRideTxOpen(mDB, workerOrderID2, nil, "CASH", rideStatusSearchingDriver, workerFare)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID2, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelRideOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_LockGenericError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID).
		WillReturnError(&pgconn.PgError{Code: "57014", Message: "query canceled"})
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_LockNoRows_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_RefundLedgerError_RollbackAndContinue(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).
		AddRow(workerOrderID).AddRow(workerOrderID2))

	// Order 1: escrow reached, ledger gagal → rollback, tidak ada partial.
	expectRideTxOpen(mDB, workerOrderID, &custWallet, paymentMethodWallet, rideStatusSearchingDriver, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Return(errors.New("insufficient escrow balance")).Once()
	mDB.ExpectRollback()

	// Order 2: CASH → sukses ⇒ rollback order 1 tidak meng-spread ke order 2.
	expectRideTxOpen(mDB, workerOrderID2, nil, "CASH", rideStatusSearchingDriver, workerFare)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID2, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelRideOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_RefundNoCustomerWallet_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	// customer_wallet_id NULL pada order WALLET → ErrWalletNotFound.
	expectRideOneOrder(mDB, workerOrderID, nil, paymentMethodWallet, rideStatusSearchingDriver, workerFare)
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_SystemWalletIDNoRows_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectRideOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, rideStatusSearchingDriver, workerFare)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_SystemWalletIDQueryError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectRideOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, rideStatusSearchingDriver, workerFare)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnError(errors.New("conn reset"))
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_LockWalletsError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectRideOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, rideStatusSearchingDriver, workerFare)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerEscrowID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: "40P01", Message: "deadlock detected"})
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_CASFalse_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectRideOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, rideStatusSearchingDriver, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	setupLedgerCapture(lgr)
	// CAS guard: UPDATE tidak mengubah baris (order berubah status di detik
	// yang sama) → 0 baris, transaksi rollback supaya refund tidak orphan.
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_CASExecError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOneOrder(mDB, workerOrderID, nil, "CASH", rideStatusSearchingDriver, workerFare)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID, rideStatusSearchingDriver).
		WillReturnError(errors.New("serialization failure"))
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_EventInsertError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOneOrder(mDB, workerOrderID, nil, "CASH", rideStatusSearchingDriver, workerFare)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: "23505", Message: "duplicate key"})
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelRideOrders_CommitError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectRideOneOrder(mDB, workerOrderID, nil, "CASH", rideStatusSearchingDriver, workerFare)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(workerOrderID, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit().WillReturnError(errors.New("commit failed"))
	mDB.ExpectRollback()

	got := w.CancelRideOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- CancelFoodOrders ----

func TestCancelFoodOrders_WalletRefund(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, is_refunded, is_settled, total_amount").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "is_refunded", "is_settled", "total_amount"}).
			AddRow(workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, false, false, workerFare))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerEscrowID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	entries := setupLedgerCapture(lgr)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelFoodOrders(context.Background())

	assert.Equal(t, 1, got)
	assertRefundEntries(t, entries, workerEscrowID, workerCustWallet, workerOrderID, workerFare, referenceTypeFoodRefund)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- CancelFoodOrders: error paths (TD-106) ----

// expectFoodOrderIDs mengatur ekspektasi query ID food order expired.
func expectFoodOrderIDs(mDB pgxmock.PgxPoolIface, rows *pgxmock.Rows) {
	mDB.ExpectQuery("SELECT id FROM food_orders").WillReturnRows(rows)
}

// expectFoodTxOpen mengatur ekspektasi transaksi sampai baris order ter-lock.
func expectFoodTxOpen(mDB pgxmock.PgxPoolIface, id uuid.UUID, wallet any, payment, status string, isRefunded, isSettled bool, total decimal.Decimal) {
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, is_refunded, is_settled, total_amount").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "is_refunded", "is_settled", "total_amount"}).
			AddRow(id, wallet, payment, status, isRefunded, isSettled, total))
}

// expectFoodOneOrder menggabungkan query ID expired dengan transaksi sampai lock.
func expectFoodOneOrder(mDB pgxmock.PgxPoolIface, id uuid.UUID, wallet any, payment, status string, isRefunded, isSettled bool, total decimal.Decimal) {
	expectFoodOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(id))
	expectFoodTxOpen(mDB, id, wallet, payment, status, isRefunded, isSettled, total)
}

func TestCancelFoodOrders_QueryError_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnError(errors.New("conn refused"))

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_QueryRowsErr_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).
			AddRow(workerOrderID).
			CloseError(errors.New("connection reset by peer")))

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_BeginError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectFoodOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin().WillReturnError(errors.New("begin failed"))

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_LockTimeout55P03_SkipThenContinue(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectFoodOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).
		AddRow(workerOrderID).AddRow(workerOrderID2))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, is_refunded, is_settled, total_amount").
		WithArgs(workerOrderID).
		WillReturnError(&pgconn.PgError{Code: "55P03"})
	mDB.ExpectRollback()

	// Order 2 sukses → 1st order yang lock-nya gagal tidak menghentikan sweep.
	expectFoodTxOpen(mDB, workerOrderID2, nil, "CASH", foodStatusCreated, false, false, workerFare)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID2, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelFoodOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_RefundLedgerError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectFoodOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, false, false, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Return(errors.New("ledger insert failed")).Once()
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	// Tidak ada INSERT event / UPDATE cancel yang lolos: transaksi rollback
	// penuh, jadi tidak ada partial state.
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_RefundSkipped_WhenAlreadyRefunded(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	// is_refunded=TRUE → refund escrow di-skip (idempoten anti double-refund),
	// sehingga flag CAS harus FALSE.
	expectFoodOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, true, false, workerFare)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelFoodOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_RefundSkipped_WhenSettled(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	// is_settled=TRUE → dana sudah masuk merchant, tidak ada escrow di-refund.
	expectFoodOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, false, true, workerFare)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelFoodOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_RefundNoCustomerWallet_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectFoodOneOrder(mDB, workerOrderID, nil, paymentMethodWallet, foodStatusCreated, false, false, workerFare)
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_CASFalse_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectFoodOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, false, false, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	setupLedgerCapture(lgr)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_EventInsertError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectFoodOneOrder(mDB, workerOrderID, nil, "CASH", foodStatusCreated, false, false, workerFare)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("event insert failed"))
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- CancelSendOrders ----

func TestCancelSendOrders_WalletRefund(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	senderWallet := workerCustWallet
	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, sender_wallet_id, payment_method, status, is_settled, total_fare").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "sender_wallet_id", "payment_method", "status", "is_settled", "total_fare"}).
			AddRow(workerOrderID, &senderWallet, paymentMethodWallet, sendStatusSearchingDriver, false, workerFare))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(workerEscrowID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	entries := setupLedgerCapture(lgr)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(workerOrderID, sendStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO send_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelSendOrders(context.Background())

	assert.Equal(t, 1, got)
	assertRefundEntries(t, entries, workerEscrowID, workerCustWallet, workerOrderID, workerFare, referenceTypeSendRefund)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- CancelSendOrders: error paths (TD-106) ----

// expectSendOrderIDs mengatur ekspektasi query ID send order expired.
func expectSendOrderIDs(mDB pgxmock.PgxPoolIface, rows *pgxmock.Rows) {
	mDB.ExpectQuery("SELECT id FROM send_orders").WillReturnRows(rows)
}

// expectSendTxOpen mengatur ekspektasi transaksi sampai baris order ter-lock.
func expectSendTxOpen(mDB pgxmock.PgxPoolIface, id uuid.UUID, wallet any, payment, status string, isSettled bool, fare decimal.Decimal) {
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, sender_wallet_id, payment_method, status, is_settled, total_fare").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "sender_wallet_id", "payment_method", "status", "is_settled", "total_fare"}).
			AddRow(id, wallet, payment, status, isSettled, fare))
}

// expectSendOneOrder menggabungkan query ID expired dengan transaksi sampai lock.
func expectSendOneOrder(mDB pgxmock.PgxPoolIface, id uuid.UUID, wallet any, payment, status string, isSettled bool, fare decimal.Decimal) {
	expectSendOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(id))
	expectSendTxOpen(mDB, id, wallet, payment, status, isSettled, fare)
}

func TestCancelSendOrders_QueryError_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnError(errors.New("conn refused"))

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_QueryRowsErr_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).
			AddRow(workerOrderID).
			CloseError(errors.New("connection reset by peer")))

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_BeginError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin().WillReturnError(errors.New("begin failed"))

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_LockTimeout55P03_SkipThenContinue(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).
		AddRow(workerOrderID).AddRow(workerOrderID2))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, sender_wallet_id, payment_method, status, is_settled, total_fare").
		WithArgs(workerOrderID).
		WillReturnError(&pgconn.PgError{Code: "55P03"})
	mDB.ExpectRollback()

	expectSendTxOpen(mDB, workerOrderID2, nil, "CASH", sendStatusSearchingDriver, false, workerFare)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(workerOrderID2, sendStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO send_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelSendOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_RefundLedgerError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	senderWallet := workerCustWallet
	expectSendOneOrder(mDB, workerOrderID, &senderWallet, paymentMethodWallet, sendStatusSearchingDriver, false, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Return(errors.New("ledger insert failed")).Once()
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_RefundSkipped_WhenSettled(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	senderWallet := workerCustWallet
	expectSendOneOrder(mDB, workerOrderID, &senderWallet, paymentMethodWallet, sendStatusSearchingDriver, true, workerFare)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(workerOrderID, sendStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO send_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelSendOrders(context.Background())

	assert.Equal(t, 1, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_RefundNoSenderWallet_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOneOrder(mDB, workerOrderID, nil, paymentMethodWallet, sendStatusSearchingDriver, false, workerFare)
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_CASFalse_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	senderWallet := workerCustWallet
	expectSendOneOrder(mDB, workerOrderID, &senderWallet, paymentMethodWallet, sendStatusSearchingDriver, false, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	setupLedgerCapture(lgr)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(workerOrderID, sendStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertExpectations(t)
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_EventInsertError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOneOrder(mDB, workerOrderID, nil, "CASH", sendStatusSearchingDriver, false, workerFare)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(workerOrderID, sendStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO send_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("event insert failed"))
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_CASExecError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectFoodOneOrder(mDB, workerOrderID, nil, "CASH", foodStatusCreated, false, false, workerFare)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnError(errors.New("deadlock detected"))
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_StatementTimeoutSetupError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectFoodOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnError(errors.New("permission denied"))
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_SystemWalletIDQueryError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	custWallet := workerCustWallet
	expectFoodOneOrder(mDB, workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, false, false, workerFare)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnError(errors.New("conn reset"))
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_LockRowScanError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	// Kolom id berisi nilai yang tidak bisa di-scan ke uuid → error sebelum
	// guard status, transaksi rollback.
	expectFoodOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, is_refunded, is_settled, total_amount").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "customer_wallet_id", "payment_method", "status", "is_refunded", "is_settled", "total_amount"}).
			AddRow(42, nil, paymentMethodWallet, foodStatusCreated, false, false, workerFare))
	mDB.ExpectRollback()

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_ExpiredIDsRowScanError_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(42))

	got := w.CancelFoodOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_CASExecError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOneOrder(mDB, workerOrderID, nil, "CASH", sendStatusSearchingDriver, false, workerFare)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(workerOrderID, sendStatusSearchingDriver).
		WillReturnError(errors.New("deadlock detected"))
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_StatementTimeoutSetupError_SkipsOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnError(errors.New("permission denied"))
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_SystemWalletIDQueryError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	senderWallet := workerCustWallet
	expectSendOneOrder(mDB, workerOrderID, &senderWallet, paymentMethodWallet, sendStatusSearchingDriver, false, workerFare)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs("SYSTEM_ESCROW").
		WillReturnError(errors.New("conn reset"))
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_LockRowScanError_Rollback(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	expectSendOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).AddRow(workerOrderID))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, sender_wallet_id, payment_method, status, is_settled, total_fare").
		WithArgs(workerOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "sender_wallet_id", "payment_method", "status", "is_settled", "total_fare"}).
			AddRow(42, nil, paymentMethodWallet, sendStatusSearchingDriver, false, workerFare))
	mDB.ExpectRollback()

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelSendOrders_ExpiredIDsRowScanError_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(42))

	got := w.CancelSendOrders(context.Background())

	assert.Zero(t, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Multi-order partial failure (TD-106) ----

// expectRideCancelTail mengatur ekspektasi CAS cancel + audit event + commit
// untuk satu ride order CASH (tanpa refund).
func expectRideCancelTail(mDB pgxmock.PgxPoolIface, id uuid.UUID) {
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(id, rideStatusSearchingDriver).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()
}

func TestCancelRideOrders_PartialFailure_ContinuesToRemainingOrders(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w, lgr := newTestWorker(t, mDB)

	// 3 order expired: #1 sukses, #2 gagal (lock NOWAIT 55P03), #3 sukses.
	expectRideOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).
		AddRow(workerOrderID).AddRow(workerOrderID2).AddRow(workerOrderID3))

	expectRideTxOpen(mDB, workerOrderID, nil, "CASH", rideStatusSearchingDriver, workerFare)
	expectRideCancelTail(mDB, workerOrderID)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id, customer_wallet_id, payment_method, status, estimated_fare").
		WithArgs(workerOrderID2).
		WillReturnError(&pgconn.PgError{Code: "55P03"})
	mDB.ExpectRollback()

	expectRideTxOpen(mDB, workerOrderID3, nil, "CASH", rideStatusSearchingDriver, workerFare)
	expectRideCancelTail(mDB, workerOrderID3)

	got := w.CancelRideOrders(context.Background())

	// Order #2 yang gagal tidak menghentikan sweep dan tidak menggagalkan
	// order #1/#3 → 2 order diproses, tidak ada panic.
	assert.Equal(t, 2, got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelFoodOrders_PartialFailure_ContinuesToRemainingOrders(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	// Panggilan ledger #1 sukses, #2 gagal, #3 sukses.
	lgr := &seqLedger{errs: []error{nil, errors.New("ledger insert failed"), nil}}
	w := NewWorker(NewRepository(mDB), mDB, nil, lgr)

	// 3 order expired: #1 refund sukses, #2 ledger error (rollback), #3 sukses.
	custWallet := workerCustWallet
	expectFoodOrderIDs(mDB, pgxmock.NewRows([]string{"id"}).
		AddRow(workerOrderID).AddRow(workerOrderID2).AddRow(workerOrderID3))

	expectFoodTxOpen(mDB, workerOrderID, &custWallet, paymentMethodWallet, foodStatusCreated, false, false, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	expectFoodTxOpen(mDB, workerOrderID2, &custWallet, paymentMethodWallet, foodStatusCreated, false, false, workerFare)
	expectEscrowLookupAndWalletLock(mDB)
	mDB.ExpectRollback()

	expectFoodTxOpen(mDB, workerOrderID3, nil, "CASH", foodStatusCreated, false, false, workerFare)
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(workerOrderID3, foodStatusCreated, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	got := w.CancelFoodOrders(context.Background())

	assert.Equal(t, 2, got)
	// Order #2 gagal di ledger → tidak ada partial: order #1 & #3 tetap commit.
	require.Len(t, lgr.calls, 2)
	assertRefundEntries(t, &lgr.calls[0], workerEscrowID, workerCustWallet, workerOrderID, workerFare, referenceTypeFoodRefund)
	assertRefundEntries(t, &lgr.calls[1], workerEscrowID, workerCustWallet, workerOrderID2, workerFare, referenceTypeFoodRefund)
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Distributed lock ----

func TestWorker_DistributedLockHeldByAnother_NoOp(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	lgr := new(mockLedger)
	w := NewWorker(NewRepository(mDB), mDB, rdb, lgr)

	// Instance lain memegang lock → SetNX gagal → sweep skip tanpa query DB.
	require.NoError(t, mr.Set(lockKey, "another-instance"))

	w.sweepOnce(context.Background())

	got, err := mr.Get(lockKey)
	require.NoError(t, err)
	assert.Equal(t, "another-instance", got)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestWorker_SweepOnce_NilRedis_GracefulSingleInstance(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	lgr := new(mockLedger)
	w := NewWorker(NewRepository(mDB), mDB, nil, lgr)

	// Tanpa Redis, single-instance dianggap pemilik lock: semua sweep berjalan
	// (food → send → ride), semua kosong → return 0.
	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))
	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	w.sweepOnce(context.Background())

	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestWorker_AcquireLock_RedisUnavailable(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), DialTimeout: 200 * time.Millisecond})
	defer rdb.Close()
	mr.Close() // Redis down → SetNX error → acquire gagal.

	mDB, _ := pgxmock.NewPool()
	lgr := new(mockLedger)
	w := NewWorker(NewRepository(mDB), mDB, rdb, lgr)

	ok, err := w.acquireLock(context.Background())

	assert.False(t, ok)
	assert.Error(t, err)
	// Lock gagal/error → sweepOnce tidak mengeksekusi DB.
	w.sweepOnce(context.Background())
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestWorker_ReleaseLock_DeletesOwnLockAfterSweep(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	expectEmptySweep(mDB)
	w := NewWorker(NewRepository(mDB), mDB, rdb, new(mockLedger))

	// Lock bebas → sweep berjalan penuh (food → send → ride), lalu lock
	// dilepas dengan compare-and-del sehingga instance lain tidak tersesat.
	w.sweepOnce(context.Background())

	assert.False(t, mr.Exists(lockKey), "lock harus dilepas setelah sweep selesai")
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestWorker_ReleaseLock_RedisError_DoesNotPanic(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), DialTimeout: 200 * time.Millisecond})
	defer rdb.Close()
	mr.Close() // Redis down → Lua Eval error saat compare-and-del.

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w := NewWorker(NewRepository(mDB), mDB, rdb, new(mockLedger))

	assert.NotPanics(t, func() { w.releaseLock(context.Background()) })
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Purge idempotency cache (TD-106) ----

func TestWorker_PurgeIdempotencyCache_ReturnsDeletedRows(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w := NewWorker(NewRepository(mDB), mDB, nil, new(mockLedger))

	mDB.ExpectExec("DELETE FROM idempotency_cache").
		WillReturnResult(pgconn.NewCommandTag("DELETE 7"))

	assert.Equal(t, 7, w.purgeIdempotencyCache(context.Background()))
	require.NoError(t, mDB.ExpectationsWereMet())
}

func TestWorker_PurgeIdempotencyCache_ExecError_ReturnsZero(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	w := NewWorker(NewRepository(mDB), mDB, nil, new(mockLedger))

	mDB.ExpectExec("DELETE FROM idempotency_cache").
		WillReturnError(&pgconn.PgError{Code: "40P01", Message: "deadlock detected"})

	assert.Zero(t, w.purgeIdempotencyCache(context.Background()))
	require.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Run loop (TD-106) ----

// purgeSignalDB meneruskan operasi ke pgxmock lalu memberi sinyal saat purge
// idempotency dieksekusi, sehingga test Run bisa menentukan kapan context
// dibatalkan tanpa menebak interval sleep.
type purgeSignalDB struct {
	pgxmock.PgxPoolIface
	purged chan struct{}
	once   sync.Once
}

func (d *purgeSignalDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := d.PgxPoolIface.Exec(ctx, sql, args...)
	if err == nil && strings.Contains(sql, "idempotency_cache") {
		d.once.Do(func() { close(d.purged) })
	}
	return tag, err
}

func TestWorker_Run_PurgesExpiredIdempotency_StopsOnContextCancel(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	sdb := &purgeSignalDB{PgxPoolIface: mDB, purged: make(chan struct{})}
	lgr := new(mockLedger)
	w := NewWorker(NewRepository(sdb), sdb, nil, lgr)

	origPoll, origPurge := PollInterval, IdempotencyPurgeInterval
	PollInterval = 5 * time.Millisecond
	IdempotencyPurgeInterval = time.Millisecond
	t.Cleanup(func() { PollInterval, IdempotencyPurgeInterval = origPoll, origPurge })

	// Sweep pertama dijalankan langsung oleh Run sebelum tick pertama.
	expectEmptySweep(mDB)
	// Sweep pada tick berikutnya: jumlah tick tidak deterministik.
	mDB.ExpectQuery("SELECT id FROM food_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"})).Maybe()
	mDB.ExpectQuery("SELECT id FROM send_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"})).Maybe()
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"})).Maybe()
	// Purge wajib jalan minimal sekali (interval dipercepat di test ini).
	mDB.ExpectExec("DELETE FROM idempotency_cache").
		WillReturnResult(pgconn.NewCommandTag("DELETE 42"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	select {
	case <-sdb.purged:
	case <-time.After(10 * time.Second):
		t.Fatal("loop worker tidak menjalankan purge idempotency")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Worker.Run tidak berhenti setelah context dibatalkan")
	}

	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	require.NoError(t, mDB.ExpectationsWereMet())
}
