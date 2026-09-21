package worker

import (
	"context"
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