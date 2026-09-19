package ride

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/g-flow/g-flow/internal/wallet"
)

// ---- mocks ----

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) GetCustomer(ctx context.Context, userID uuid.UUID) (*Customer, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Customer), args.Error(1)
}

func (m *mockRepo) GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*RideWallet, error) {
	args := m.Called(ctx, userID, walletType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RideWallet), args.Error(1)
}

func (m *mockRepo) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*RideOrder, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RideOrder), args.Error(1)
}

func (m *mockRepo) InsertOrder(ctx context.Context, q Querier, order *RideOrder) error {
	args := m.Called(ctx, q, order)
	return args.Error(0)
}

func (m *mockRepo) TransitionStatus(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, toStatus string) (bool, error) {
	args := m.Called(ctx, q, orderID, fromStatus, toStatus)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) InsertEvent(ctx context.Context, q Querier, event RideOrderEvent) error {
	args := m.Called(ctx, q, event)
	return args.Error(0)
}

func (m *mockRepo) SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error) {
	args := m.Called(ctx, q, walletType)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *mockRepo) GetDriver(ctx context.Context, driverID uuid.UUID) (*Driver, error) {
	args := m.Called(ctx, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Driver), args.Error(1)
}

func (m *mockRepo) GetDriverBalance(ctx context.Context, driverID uuid.UUID) (decimal.Decimal, error) {
	args := m.Called(ctx, driverID)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

func (m *mockRepo) LockOrderForAccept(ctx context.Context, q Querier, orderID uuid.UUID) error {
	args := m.Called(ctx, q, orderID)
	return args.Error(0)
}

func (m *mockRepo) AssignDriver(ctx context.Context, q Querier, orderID uuid.UUID, driverID uuid.UUID) (bool, error) {
	args := m.Called(ctx, q, orderID, driverID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) MarkDriverBusy(ctx context.Context, q Querier, driverID uuid.UUID) error {
	args := m.Called(ctx, q, driverID)
	return args.Error(0)
}

func (m *mockRepo) LockOrderForUpdate(ctx context.Context, q Querier, orderID uuid.UUID) (*RideOrder, error) {
	args := m.Called(ctx, q, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RideOrder), args.Error(1)
}

func (m *mockRepo) GetExpiredSearchingOrders(ctx context.Context) ([]uuid.UUID, error) {
	args := m.Called(ctx)
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (m *mockRepo) CancelOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, reason string) (bool, error) {
	args := m.Called(ctx, q, orderID, fromStatus, reason)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) CompleteOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus string, actualFare, driverEarning, platformCommission decimal.Decimal) (bool, error) {
	args := m.Called(ctx, q, orderID, fromStatus, actualFare, driverEarning, platformCommission)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) MarkSettled(ctx context.Context, q Querier, orderID uuid.UUID) error {
	args := m.Called(ctx, q, orderID)
	return args.Error(0)
}

func (m *mockRepo) ResetDriverIdle(ctx context.Context, q Querier, driverID uuid.UUID) error {
	args := m.Called(ctx, q, driverID)
	return args.Error(0)
}

func (m *mockRepo) MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error {
	args := m.Called(ctx, q, driverID)
	return args.Error(0)
}

type mockLedger struct {
	mock.Mock
}

func (m *mockLedger) CreateLedgerEntries(_ context.Context, _ pgx.Tx, entries []wallet.LedgerEntry) error {
	args := m.Called(entries)
	return args.Error(0)
}

// ---- fixtures ----

var (
	svcCustomerID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svcWalletID   = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	svcEscrowID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	svcPlatformID = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	svcDriverID   = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	svcOrderID    = uuid.MustParse("66666666-6666-6666-6666-666666666666")
)

func svcCustomer(order decimal.Decimal) *Customer {
	return &Customer{ID: svcCustomerID, UserType: userTypeCustomer, Status: "ACTIVE", OverdueDebt: order}
}

func svcWallet(balance decimal.Decimal) *RideWallet {
	return &RideWallet{ID: svcWalletID, UserID: svcCustomerID, Type: WalletTypeCustomer, Balance: balance, Status: "ACTIVE"}
}

func svcDriver(workingStatus string, threshold decimal.Decimal) *Driver {
	return &Driver{ID: svcDriverID, UserType: userTypeDriver, Status: "ACTIVE", WorkingStatus: workingStatus, MinBalanceThreshold: threshold}
}

func svcValidReq(paymentMethod, idemKey string) BookRideRequest {
	return BookRideRequest{
		UserID:         svcCustomerID,
		PickupLat:      -6.2,
		PickupLng:      106.816666,
		PickupAddress:  "Jl. A",
		DropoffLat:     -6.26,
		DropoffLng:     106.816666,
		DropoffAddress: "Jl. B",
		PaymentMethod:  paymentMethod,
		IdempotencyKey: idemKey,
	}
}

func svcOrder(status string, paymentMethod string, driver *uuid.UUID) *RideOrder {
	return &RideOrder{
		ID:               svcOrderID,
		CustomerID:       svcCustomerID,
		DriverID:         driver,
		CustomerWalletID: &svcWalletID,
		PaymentMethod:    paymentMethod,
		EstimatedFare:    decimal.NewFromInt(50000),
		Status:           status,
	}
}

// bookRideCommon mengatur mock untuk seluruh alur BookRide yang sukses lalu
// memanggil BookRide. Dipakai oleh pengujian escrow WALLET.
func setupBookRideDB(mDB pgxmock.PgxPoolIface, idemKey string) {
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, svcCustomerID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, svcCustomerID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
}

func timeNowFuture() *time.Time {
	t := time.Now().Add(5 * time.Minute)
	return &t
}

// BookRide

func TestBookRide_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-1"
	req := svcValidReq(PaymentMethodWallet, idemKey)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)

	setupBookRideDB(mDB, idemKey)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(svcWalletID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200000)))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, svcCustomerID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.BookRide(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, statusSearchingDriver, resp.Status)
	assert.NotEqual(t, uuid.Nil, resp.OrderID)
	assert.True(t, resp.DistanceKm.IsPositive())
	assert.True(t, resp.EscrowAmount.IsPositive(), "escrow harus > 0 untuk WALLET")
	assert.Equal(t, PaymentMethodWallet, resp.PaymentMethod)

	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestBookRide_Success_Cash(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-cash"
	req := svcValidReq(PaymentMethodCash, idemKey)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.Zero), nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupBookRideDB(mDB, idemKey)
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, svcCustomerID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.BookRide(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, statusSearchingDriver, resp.Status)
	assert.True(t, resp.EscrowAmount.IsZero(), "CASH tidak ada escrow")

	repo.AssertExpectations(t)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestBookRide_InsufficientBalance(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-bal"
	req := svcValidReq(PaymentMethodWallet, idemKey)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(100)), nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)

	setupBookRideDB(mDB, idemKey)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(svcWalletID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100)))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)

	assert.ErrorIs(t, err, ErrInsufficientBalance)
	repo.AssertExpectations(t)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestBookRide_OverdueDebt(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodWallet, "book-key-debt")
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.NewFromInt(30000)), nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)

	assert.ErrorIs(t, err, ErrOverdueDebt)
	repo.AssertExpectations(t)
}

func TestBookRide_Idempotency(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-dup"
	req := svcValidReq(PaymentMethodWallet, idemKey)

	cached := BookRideResponse{
		OrderID:       svcOrderID,
		Status:        statusSearchingDriver,
		PaymentMethod: PaymentMethodWallet,
	}
	cachedJSON, _ := json.Marshal(cached)

	// Simulasi L1 Redis sudah COMPLETED → langsung return cache, no DB.
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: cachedJSON})
	assert.NoError(t, mr.Set(redisKey(svcCustomerID, idemKey), string(redisVal)))

	svc := NewService(repo, lgr, rdb, mDB)
	resp, err := svc.BookRide(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, svcOrderID, resp.OrderID)
	assert.Equal(t, statusSearchingDriver, resp.Status)
	repo.AssertNotCalled(t, "GetCustomer")
	repo.AssertNotCalled(t, "GetWalletByUserAndType")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestBookRide_MissingIdempotency(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := svcValidReq(PaymentMethodWallet, "")
	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrIdempotencyKeyRequired)
	repo.AssertNotCalled(t, "GetCustomer")
}

func TestBookRide_InvalidPaymentMethod(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := svcValidReq("QRIS", "k")
	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidPaymentMethod)
}

func TestBookRide_SamePickupDropoff(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := BookRideRequest{UserID: svcCustomerID, PickupLat: -6.2, PickupLng: 106.8,
		DropoffLat: -6.2, DropoffLng: 106.8, PaymentMethod: PaymentMethodWallet, IdempotencyKey: "k"}
	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrSamePickupDropoff)
}

func TestBookRide_InvalidCoordinates(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := BookRideRequest{UserID: svcCustomerID, PickupLat: -200, PickupLng: 0,
		DropoffLat: -6.2, DropoffLng: 106.8, PaymentMethod: PaymentMethodWallet, IdempotencyKey: "k"}
	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidCoordinates)
}

func TestBookRide_NotCustomer(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := svcValidReq(PaymentMethodWallet, "k")
	cust := svcCustomer(decimal.Zero)
	cust.UserType = userTypeDriver
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(cust, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrNotCustomer)
}

func TestBookRide_CustomerInactive(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := svcValidReq(PaymentMethodWallet, "k")
	cust := svcCustomer(decimal.Zero)
	cust.Status = "SUSPENDED"
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(cust, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrCustomerInactive)
}

func TestBookRide_WalletInactive(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := svcValidReq(PaymentMethodWallet, "k")
	w := svcWallet(decimal.Zero)
	w.Status = "SUSPENDED"
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(w, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrWalletInactive)
}

func TestBookRide_TransitionFailed(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodCash, "book-key-tf")
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.Zero), nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(false, nil)

	setupBookRideDB(mDB, req.IdempotencyKey)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// GetOrder

func TestGetOrder(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	o := svcOrder(statusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(o, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetOrder(context.Background(), svcOrderID)
	assert.NoError(t, err)
	assert.Equal(t, svcOrderID, got.ID)
	repo.AssertExpectations(t)
}

// AcceptOrder

func TestAcceptOrder_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(100000), nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(nil)
	repo.On("AssignDriver", mock.Anything, mock.Anything, svcOrderID, svcDriverID).Return(true, nil)
	repo.On("MarkDriverBusy", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, statusDriverAssigned, resp.Status)
	assert.Equal(t, svcDriverID, resp.DriverID)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptOrder_DriverBusy(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusBusy, decimal.Zero), nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrDriverBusy)
	repo.AssertExpectations(t)
}

func TestAcceptOrder_NotDriver(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	d := svcDriver(workingStatusIdle, decimal.Zero)
	d.UserType = userTypeCustomer
	repo.On("GetDriver", mock.Anything, svcDriverID).Return(d, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrNotDriver)
}

func TestAcceptOrder_InactiveDriver(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	d := svcDriver(workingStatusIdle, decimal.Zero)
	d.Status = "SUSPENDED"
	repo.On("GetDriver", mock.Anything, svcDriverID).Return(d, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrDriverInactive)
}

func TestAcceptOrder_InsufficientDriverBalance(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.NewFromInt(50000)), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(10000), nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrInsufficientDriverBalance)
	repo.AssertExpectations(t)
}

// TestAcceptOrder_RaceCondition mensimulasikan 2 driver bersaing: yang pertama
// sukses assign, yang kedua gagal karena lock order tidak tersedia (55P03 →
// ErrLockTimeout / conflict).
func TestAcceptOrder_RaceCondition(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	driver2 := uuid.MustParse("77777777-7777-7777-7777-777777777777")

	// Driver pertama sukses.
	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(100000), nil)
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(nil).Once()
	repo.On("AssignDriver", mock.Anything, mock.Anything, svcOrderID, svcDriverID).Return(true, nil)
	repo.On("MarkDriverBusy", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Driver kedua: lock tidak tersedia → conflict 503/ErrLockTimeout.
	repo.On("GetDriver", mock.Anything, driver2).Return(
		&Driver{ID: driver2, UserType: userTypeDriver, Status: "ACTIVE", WorkingStatus: workingStatusIdle, MinBalanceThreshold: decimal.Zero}, nil)
	repo.On("GetDriverBalance", mock.Anything, driver2).Return(decimal.NewFromInt(100000), nil)
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).
		Return(&pgconn.PgError{Code: "55P03"}).Once()

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectCommit()

	// Transaksi kedua (driver 2): begin + set timeout, lalu gagal di lock.
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)

	// Call 1 (success).
	resp1, err1 := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.NoError(t, err1)
	assert.NotNil(t, resp1)
	assert.Equal(t, statusDriverAssigned, resp1.Status)

	// Call 2 (conflict).
	_, err2 := svc.AcceptOrder(context.Background(), svcOrderID, driver2)
	assert.ErrorIs(t, err2, ErrLockTimeout)

	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptOrder_OrderNotSearching(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(100000), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(ErrOrderNotFound)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID), nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrOrderNotSearching)
	repo.AssertExpectations(t)
}

func TestAcceptOrder_AssignFailed(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(100000), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(nil)
	repo.On("AssignDriver", mock.Anything, mock.Anything, svcOrderID, svcDriverID).Return(false, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrOrderNotSearching)
	repo.AssertExpectations(t)
}

// CancelOrder

func TestCancelOrder_BeforeAssign(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	order := svcOrder(statusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonCustomerCancel).Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID,
		UserID:  svcCustomerID,
		Status:  statusCancelled,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelOrder_AfterAssign(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusDriverAssigned, PaymentMethodCash, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	// CASH → tidak ada refund escrow; driver dicancel (customer) → reset IDLE.
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, reasonCustomerCancel).Return(true, nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	// Order service refunds escrow hanya untuk WALLET → CASH tanpa fee/refund.
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID,
		UserID:  svcCustomerID,
		Status:  statusCancelled,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelOrder_DriverEmergency(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	// Driver asal cancancel tanpa reason → reason otomatis DRIVER_EMERGENCY.
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, reasonDriverEmergency).Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID,
		UserID:  svcDriverID,
		Status:  statusCancelled,
		Reason:  reasonDriverEmergency,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCancelOrder_InvalidTransition(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	order := svcOrder(statusCompleted, PaymentMethodWallet, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertExpectations(t)
}

func TestCancelOrder_NotAllowed(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	drv := svcDriverID
	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: uuid.New(), Status: statusCancelled,
	})
	assert.ErrorIs(t, err, ErrNotAllowed)
	repo.AssertExpectations(t)
}

func TestUpdateRideStatus_InvalidStatus(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: "BOGUS",
	})
	assert.ErrorIs(t, err, ErrInvalidStatus)
	repo.AssertNotCalled(t, "GetOrderByID")
}

func TestUpdateRideStatus_SimpleTransition(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, statusDriverArrived).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusDriverArrived,
	})
	assert.NoError(t, err)
	assert.Equal(t, statusDriverArrived, resp.Status)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// Settlement

func TestSettlement_WalletPayment(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)
	drvWallet := &RideWallet{ID: svcOrderID, UserID: svcDriverID, Type: WalletTypeDriver, Balance: decimal.Zero, Status: "ACTIVE"}
	_ = drvWallet

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)
	repo.On("GetWalletByUserAndType", mock.Anything, svcDriverID, WalletTypeDriver).
		Return(&RideWallet{ID: uuid.New(), UserID: svcDriverID, Type: WalletTypeDriver, Balance: decimal.Zero, Status: "ACTIVE"}, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemPlatform).Return(svcPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("MarkSettled", mock.Anything, mock.Anything, svcOrderID).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettlement_CashPayment_AboveCeiling(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodCash, &drv)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)
	repo.On("GetWalletByUserAndType", mock.Anything, svcDriverID, WalletTypeDriver).
		Return(&RideWallet{ID: uuid.New(), UserID: svcDriverID, Type: WalletTypeDriver, Balance: decimal.NewFromInt(100000), Status: "ACTIVE"}, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemPlatform).Return(svcPlatformID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("MarkSettled", mock.Anything, mock.Anything, svcOrderID).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	// balance setelah komisi masih di atas ceiling (-50.000) → IDLE.
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-40000)))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettlement_CashPayment_BelowCeiling_Suspended(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodCash, &drv)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)
	repo.On("GetWalletByUserAndType", mock.Anything, svcDriverID, WalletTypeDriver).
		Return(&RideWallet{ID: uuid.New(), UserID: svcDriverID, Type: WalletTypeDriver, Balance: decimal.Zero, Status: "ACTIVE"}, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemPlatform).Return(svcPlatformID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("MarkDriverSuspended", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("MarkSettled", mock.Anything, mock.Anything, svcOrderID).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	// balance menembus ceiling negatif → SUSPENDED (bukan ResetDriverIdle).
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-60000)))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// Auto-Cancel Worker

func TestAutoCancelExpiredOrders(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	order := svcOrder(statusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetExpiredSearchingOrders", mock.Anything).Return([]uuid.UUID{svcOrderID}, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil).Once()
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonExpired).Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	cancelled, err := svc.AutoCancelExpiredOrders(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, cancelled)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// helpers

func TestHaversineKm(t *testing.T) {
	d := haversineKm(-6.2, 106.816666, -6.26, 106.816666)
	assert.True(t, d.IsPositive())
	assert.True(t, d.GreaterThan(decimal.NewFromInt(6)), "jarak harus > 6km, got %v", d)
}

func TestLockWalletsAsc(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectBegin()

	tx, err := mDB.Begin(context.Background())
	assert.NoError(t, err)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	err = lockWalletsAsc(context.Background(), tx, uuid.New(), uuid.New())
	assert.NoError(t, err)
	mDB.ExpectRollback()
	assert.NoError(t, tx.Rollback(context.Background()))
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- idempotency helper branches ----

func Test_idemAcquire_Completed(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	body := []byte(`{"ok":true}`)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", svcCustomerID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(redisCompleted, body, nil))

	svc := NewService(new(mockRepo), new(mockLedger), nil, mDB)
	res, err := svc.idemAcquire(context.Background(), svcCustomerID, "k")
	assert.NoError(t, err)
	assert.False(t, res.proceed)
	assert.JSONEq(t, `{"ok":true}`, string(res.cached))
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_idemAcquire_ProcessingFresh(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", svcCustomerID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", timeNowFuture()))

	svc := NewService(new(mockRepo), new(mockLedger), nil, mDB)
	_, err = svc.idemAcquire(context.Background(), svcCustomerID, "k")
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_idemAcquire_InsertConflict(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", svcCustomerID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs("k", svcCustomerID).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	svc := NewService(new(mockRepo), new(mockLedger), nil, mDB)
	_, err = svc.idemAcquire(context.Background(), svcCustomerID, "k")
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_redisGetCachedResp_Success(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	raw := json.RawMessage(`{"status":"ok"}`)
	b, _ := json.Marshal(redisCache{State: redisCompleted, Response: raw})
	assert.NoError(t, mr.Set(redisKey(svcCustomerID, "k"), string(b)))

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), svcCustomerID, "k")
	assert.True(t, ok)
	assert.JSONEq(t, `{"status":"ok"}`, string(resp))
}

func Test_redisGetCachedResp_NotCompleted(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	b, _ := json.Marshal(redisCache{State: pgProcessing, Response: json.RawMessage(`{}`)})
	assert.NoError(t, mr.Set(redisKey(svcCustomerID, "k"), string(b)))

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), svcCustomerID, "k")
	assert.False(t, ok)
	assert.Nil(t, resp)
}

func Test_redisGetCachedResp_NilRedis(t *testing.T) {
	svc := NewService(new(mockRepo), new(mockLedger), nil, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), svcCustomerID, "k")
	assert.False(t, ok)
	assert.Nil(t, resp)
}

func Test_redisSet_NilRedis(t *testing.T) {
	svc := NewService(new(mockRepo), new(mockLedger), nil, nil)
	svc.redisSet(context.Background(), svcCustomerID, "k", redisCompleted, json.RawMessage(`{}`))
	// Tidak panic; nil redis di-skip.
}

func Test_redisSet_WithRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	svc.redisSet(context.Background(), svcCustomerID, "k", redisCompleted, json.RawMessage(`{"a":1}`))
	got, err := mr.Get(redisKey(svcCustomerID, "k"))
	assert.NoError(t, err)
	var c redisCache
	assert.NoError(t, json.Unmarshal([]byte(got), &c))
	assert.Equal(t, redisCompleted, c.State)
	assert.JSONEq(t, `{"a":1}`, string(c.Response))
}

func Test_cachedResponse_InvalidJSON(t *testing.T) {
	// L1 Redis berisi COMPLETED tapi response tidak bisa di-unmarshal ke
	// BookRideResponse → ErrInvalidCachedResponse.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// order_id: int tidak valid untuk uuid.UUID → unmarshal BookRideResponse gagal.
	b, _ := json.Marshal(redisCache{State: redisCompleted, Response: json.RawMessage(`{"order_id": 1}`)})
	assert.NoError(t, mr.Set(redisKey(svcCustomerID, "k"), string(b)))

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	_, err := svc.BookRide(context.Background(), svcValidReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrInvalidCachedResponse)
}

func Test_validLatLng(t *testing.T) {
	assert.True(t, validLatLng(-6.2, 106.8))
	assert.True(t, validLatLng(90, 180))
	assert.False(t, validLatLng(91, 0))
	assert.False(t, validLatLng(0, -181))
}

// Branch tambahan untuk menaikkan coverage service

func TestBookRide_GetCustomerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	req := svcValidReq(PaymentMethodWallet, "k")
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(nil, ErrCustomerNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrCustomerNotFound)
	repo.AssertExpectations(t)
}

func TestBookRide_IdemInProgress(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodWallet, "k")
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)

	// L2: PROCESSING fresh (debounce masa depan) → ErrIdempotencyInProgress.
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", svcCustomerID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", timeNowFuture()))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestBookRide_TransitionError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodCash, "k")
	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.Zero), nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(false, errors.New("db down"))

	setupBookRideDB(mDB, req.IdempotencyKey)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestAcceptOrder_GetDriverError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(nil, ErrDriverNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrDriverNotFound)
	repo.AssertExpectations(t)
}

func TestAcceptOrder_LockOrderGetError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(100000), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(ErrOrderNotFound)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(nil, ErrOrderNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrOrderNotFound)
	repo.AssertExpectations(t)
}

func TestCancelOrder_CancelFailed(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	order := svcOrder(statusSearchingDriver, PaymentMethodCash, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonCustomerCancel).Return(false, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertExpectations(t)
}

func TestUpdateStatus_LockStatusMismatch(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	order := svcOrder(statusDriverArrived, PaymentMethodWallet, &svcDriverID)
	drv := svcDriverID
	order2 := svcOrder(statusTripStarted, PaymentMethodWallet, &drv) // status berubah usai lock
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order2, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusTripStarted,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertExpectations(t)
}

func TestCompleteOrder_SettlementDriverWalletError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodCash, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	repo.On("GetWalletByUserAndType", mock.Anything, svcDriverID, WalletTypeDriver).Return(nil, ErrWalletNotFound)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted,
	})
	assert.ErrorIs(t, err, ErrWalletNotFound)
	repo.AssertExpectations(t)
}

func TestRefundEscrow_NoCustomerWallet(t *testing.T) {
	svc := NewService(new(mockRepo), new(mockLedger), nil, nil)
	order := &RideOrder{ID: svcOrderID, PaymentMethod: PaymentMethodWallet, EstimatedFare: decimal.NewFromInt(10000)} // CustomerWalletID nil
	err := svc.refundEscrow(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_idemAcquire_QueryError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", svcCustomerID).
		WillReturnError(errors.New("conn refused"))

	svc := NewService(new(mockRepo), new(mockLedger), nil, mDB)
	_, err = svc.idemAcquire(context.Background(), svcCustomerID, "k")
	assert.EqualError(t, err, "conn refused")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_pgInsertProcessing_OtherError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs("k", svcCustomerID).
		WillReturnError(errors.New("conn refused"))

	svc := NewService(new(mockRepo), new(mockLedger), nil, mDB)
	err = svc.pgInsertProcessing(context.Background(), svcCustomerID, "k")
	assert.EqualError(t, err, "conn refused")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_cacheResponse_UpdateError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs("k", svcCustomerID, pgxmock.AnyArg()).
		WillReturnError(errors.New("conn refused"))

	svc := NewService(new(mockRepo), new(mockLedger), nil, mDB)
	err = svc.cacheResponse(context.Background(), svcCustomerID, "k", &BookRideResponse{})
	assert.EqualError(t, err, "conn refused")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_updateStatus_SimpleTransitionFailed(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, statusDriverArrived).Return(false, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusDriverArrived,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertExpectations(t)
}

func Test_updateStatus_OrderFinal(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	order := svcOrder(statusSettled, PaymentMethodWallet, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertExpectations(t)
}

func Test_updateStatus_CompletedWrongActor(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	drv := svcDriverID
	order := svcOrder(statusDriverArrived, PaymentMethodWallet, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)

	svc := NewService(repo, lgr, nil, mDB)
	// status bukan TRIP_STARTED → ErrInvalidTransition.
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: uuid.New(), Status: statusCompleted,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertExpectations(t)
}

func Test_validateStatusTransition_TripStartedNotAllowed(t *testing.T) {
	drv := svcDriverID
	order := svcOrder(statusDriverArrived, PaymentMethodWallet, &drv)
	err := validateStatusTransition(order, UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: uuid.New(), Status: statusTripStarted,
	})
	assert.ErrorIs(t, err, ErrNotAllowed)
}

func Test_validateStatusTransition_CompletedNotAllowed(t *testing.T) {
	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)
	err := validateStatusTransition(order, UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: uuid.New(), Status: statusCompleted,
	})
	assert.ErrorIs(t, err, ErrNotAllowed)
}

func Test_validateStatusTransition_CancelledCompleted(t *testing.T) {
	order := svcOrder(statusCompleted, PaymentMethodWallet, nil)
	err := validateStatusTransition(order, UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestSettlement_WalletPayment_SystemWalletErr(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	repo.On("GetWalletByUserAndType", mock.Anything, svcDriverID, WalletTypeDriver).
		Return(&RideWallet{ID: uuid.New(), UserID: svcDriverID, Type: WalletTypeDriver}, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemPlatform).Return(svcPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(uuid.Nil, ErrWalletNotFound)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted,
	})
	assert.ErrorIs(t, err, ErrWalletNotFound)
	repo.AssertExpectations(t)
}

func TestAutoCancel_NonSearchingSkip(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetExpiredSearchingOrders", mock.Anything).Return([]uuid.UUID{svcOrderID}, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(svcOrder(statusDriverAssigned, PaymentMethodWallet, nil), nil)

	svc := NewService(repo, lgr, nil, mDB)
	cancelled, err := svc.AutoCancelExpiredOrders(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, cancelled)
	repo.AssertExpectations(t)
}

func TestAutoCancel_GetOrderErrSkip(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetExpiredSearchingOrders", mock.Anything).Return([]uuid.UUID{svcOrderID}, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(nil, ErrOrderNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	cancelled, err := svc.AutoCancelExpiredOrders(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, cancelled)
	repo.AssertExpectations(t)
}

func TestAutoCancel_BeginErr(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetExpiredSearchingOrders", mock.Anything).Return([]uuid.UUID{svcOrderID}, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(svcOrder(statusSearchingDriver, PaymentMethodWallet, nil), nil)
	mDB.ExpectBegin().WillReturnError(errors.New("begin failed"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.AutoCancelExpiredOrders(context.Background())
	assert.EqualError(t, err, "begin failed")
	repo.AssertExpectations(t)
}

func TestHoldEscrow_SystemWalletErr(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(uuid.Nil, ErrWalletNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.holdEscrow(context.Background(), nil, svcWallet(decimal.NewFromInt(200000)), svcOrderID, decimal.NewFromInt(10000))
	assert.ErrorIs(t, err, ErrWalletNotFound)
	repo.AssertExpectations(t)
}

func TestHoldEscrow_InsufficientAfterLock(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(svcWalletID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100)))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.holdEscrow(context.Background(), mDB, svcWallet(decimal.NewFromInt(100)), svcOrderID, decimal.NewFromInt(10000))
	assert.ErrorIs(t, err, ErrInsufficientBalance)
	repo.AssertExpectations(t)
}

