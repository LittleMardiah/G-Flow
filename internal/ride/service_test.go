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
	"github.com/stretchr/testify/require"

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

func (m *mockRepo) ListOrdersByCustomer(ctx context.Context, customerID uuid.UUID, status string, limit, offset int) ([]RideOrder, error) {
	args := m.Called(ctx, customerID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]RideOrder), args.Error(1)
}

func (m *mockRepo) CountOrdersByCustomer(ctx context.Context, customerID uuid.UUID, status string) (int, error) {
	args := m.Called(ctx, customerID, status)
	return args.Int(0), args.Error(1)
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

func (m *mockRepo) GetVoucherByCode(ctx context.Context, code string) (*Voucher, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Voucher), args.Error(1)
}

func (m *mockRepo) GetVoucherByID(ctx context.Context, voucherID uuid.UUID) (*Voucher, error) {
	args := m.Called(ctx, voucherID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Voucher), args.Error(1)
}

func (m *mockRepo) CountUserVoucherUsage(ctx context.Context, q Querier, userID, voucherID uuid.UUID) (int, error) {
	args := m.Called(ctx, q, userID, voucherID)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) InsertUserVoucher(ctx context.Context, q Querier, uv *UserVoucher) error {
	args := m.Called(ctx, q, uv)
	return args.Error(0)
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

func (m *mockRepo) LockDriverUserForAccept(ctx context.Context, q Querier, driverID uuid.UUID) (*Driver, error) {
	args := m.Called(ctx, q, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Driver), args.Error(1)
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

func (m *mockRepo) CancelOrder(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, reason string, fee decimal.Decimal) (bool, error) {
	args := m.Called(ctx, q, orderID, fromStatus, reason, fee)
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

func (m *mockRepo) IncrementOverdueDebt(ctx context.Context, q Querier, customerID uuid.UUID, amount decimal.Decimal) error {
	args := m.Called(ctx, q, customerID, amount)
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
	svcCustomerID     = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svcWalletID       = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	svcEscrowID       = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	svcPlatformID     = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	svcDriverID       = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	svcOrderID        = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	svcDriverWalletID = uuid.MustParse("77777777-7777-7777-7777-777777777777")
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

func svcDriverWallet(balance decimal.Decimal) *RideWallet {
	return &RideWallet{ID: svcDriverWalletID, UserID: svcDriverID, Type: WalletTypeDriver, Balance: balance, Status: "ACTIVE"}
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

// decMatch mencocokkan decimal.Decimal berdasar nilai (bukan representasi):
// shopspring bisa menyimpan nilai sama dengan exponent berbeda (exp 0 vs -2).
func decMatch(want decimal.Decimal) any {
	return mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(want) })
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

// ---- TD-113: GetOrder enrich driver info + masking phone ----

// TestService_GetOrder_EnrichesDriver Order punya driver: GetOrder harus
// mengembalikan name/vehicle apa adanya, phone dalam bentuk MASKED, dan
// TIDAK boleh memutasikan objek order dari repository.
func TestService_GetOrder_EnrichesDriver(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	o := svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID)
	o.DriverName = ptrString("Budi Santoso")
	o.DriverPhone = ptrString("+628123456789")
	o.DriverVehicleType = ptrString("MOTORCYCLE")
	o.DriverVehiclePlate = ptrString("B 1234 ABC")
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(o, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetOrder(context.Background(), svcOrderID)
	require.NoError(t, err)

	// Masking: +62 + 4 digit terakhir disembunyikan -> "+62****6789".
	require.NotNil(t, got.DriverPhone)
	assert.Equal(t, "+62****6789", *got.DriverPhone)
	assert.NotEqual(t, "+628123456789", *got.DriverPhone, "phone mentah tidak boleh bocor ke consumer")

	// Field lain driver diteruskan tanpa modifikasi.
	require.NotNil(t, got.DriverName)
	assert.Equal(t, "Budi Santoso", *got.DriverName)
	require.NotNil(t, got.DriverVehicleType)
	assert.Equal(t, "MOTORCYCLE", *got.DriverVehicleType)
	require.NotNil(t, got.DriverVehiclePlate)
	assert.Equal(t, "B 1234 ABC", *got.DriverVehiclePlate)
	assert.Equal(t, svcDriverID, *got.DriverID)

	// Objek order milik repository tidak boleh termutasi (masking via copy).
	require.NotNil(t, o.DriverPhone)
	assert.Equal(t, "+628123456789", *o.DriverPhone)

	repo.AssertExpectations(t)
}

// TestService_GetOrder_NoDriver Order SEARCHING_DRIVER: driver_id NULL dan
// semua kolom driver NULL. GetOrder harus graceful — tidak error, tidak panic,
// pointer tetap nil.
func TestService_GetOrder_NoDriver(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	o := svcOrder(statusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(o, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetOrder(context.Background(), svcOrderID)
	require.NoError(t, err)

	assert.Nil(t, got.DriverID)
	assert.Nil(t, got.DriverName)
	assert.Nil(t, got.DriverPhone)
	assert.Nil(t, got.DriverVehicleType)
	assert.Nil(t, got.DriverVehiclePlate)
	assert.Equal(t, statusSearchingDriver, got.Status)
	repo.AssertExpectations(t)
}

// TestService_GetOrder_RepoError Error repository diteruskan apa adanya, tanpa
// attempting masking pada order nil.
func TestService_GetOrder_RepoError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(nil, ErrOrderNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetOrder(context.Background(), svcOrderID)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrOrderNotFound)
	repo.AssertExpectations(t)
}

// TestMaskDriverPhone_Table menutupi seluruh cabang maskDriverPhone
// (service.go): prefix +62 / 0 / tanpa prefix, terpotong, kosong, dan nil.
func TestMaskDriverPhone_Table(t *testing.T) {
	cases := []struct {
		name  string
		input *string
		want  *string
	}{
		{"nil", nil, nil},
		{"empty string", ptrString(""), nil},
		{"whitespace only", ptrString("   "), nil},
		{"<=4 digits", ptrString("1234"), ptrString("****")},
		{"+62 normal", ptrString("+628123456789"), ptrString("+62****6789")},
		{"+62 panjang", ptrString("+6281234567890"), ptrString("+62****7890")},
		{"0 normal", ptrString("08123456789"), ptrString("0812****6789")},
		{"tanpa prefix", ptrString("8123456789"), ptrString("****6789")},
		{"+62 pendek (tidak muat)", ptrString("+628123"), ptrString("****8123")},
		{"0 pendek (tidak muat)", ptrString("08123456"), ptrString("****3456")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskDriverPhone(tc.input)
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, *tc.want, *got)
		})
	}
}

// GetRidesHistory

func TestGetRidesHistory_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	orders := []RideOrder{
		*svcHistoryOrder(svcOrderID, statusSettled, PaymentMethodWallet),
		*svcHistoryOrder(uuid.MustParse("77777777-7777-7777-7777-777777777777"), statusCompleted, PaymentMethodCash),
	}
	repo.On("CountOrdersByCustomer", mock.Anything, svcCustomerID, "").Return(3, nil)
	repo.On("ListOrdersByCustomer", mock.Anything, svcCustomerID, "", 20, 0).Return(orders, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, total, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 1, 20, "")
	assert.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, got, 2)
	assert.Equal(t, svcOrderID, got[0].ID)
	repo.AssertExpectations(t)
}

func TestGetRidesHistory_PaginationOffset(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("CountOrdersByCustomer", mock.Anything, svcCustomerID, "").Return(25, nil)
	repo.On("ListOrdersByCustomer", mock.Anything, svcCustomerID, "", 10, 10).Return([]RideOrder{}, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, total, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 2, 10, "")
	assert.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Empty(t, got)
	repo.AssertExpectations(t)
}

func TestGetRidesHistory_StatusFilter(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	order := svcHistoryOrder(svcOrderID, statusCompleted, PaymentMethodWallet)
	repo.On("CountOrdersByCustomer", mock.Anything, svcCustomerID, "COMPLETED").Return(1, nil)
	repo.On("ListOrdersByCustomer", mock.Anything, svcCustomerID, "COMPLETED", 20, 0).Return([]RideOrder{*order}, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, total, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 1, 20, "COMPLETED")
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, got, 1)
	assert.Equal(t, statusCompleted, got[0].Status)
	repo.AssertExpectations(t)
}

func TestGetRidesHistory_InvalidPage(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	svc := NewService(repo, lgr, nil, mDB)
	_, _, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 0, 20, "")
	assert.ErrorIs(t, err, ErrInvalidPagination)
	repo.AssertNotCalled(t, "CountOrdersByCustomer", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "ListOrdersByCustomer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetRidesHistory_InvalidPageSize(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	svc := NewService(repo, lgr, nil, mDB)
	_, _, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 1, 0, "")
	assert.ErrorIs(t, err, ErrInvalidPagination)
	_, _, err = svc.GetRidesHistory(context.Background(), svcCustomerID, 1, 51, "")
	assert.ErrorIs(t, err, ErrInvalidPagination)
	repo.AssertNotCalled(t, "CountOrdersByCustomer", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetRidesHistory_CountError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("CountOrdersByCustomer", mock.Anything, svcCustomerID, "").Return(0, errors.New("conn refused"))

	svc := NewService(repo, lgr, nil, mDB)
	got, total, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 1, 20, "")
	assert.ErrorContains(t, err, "conn refused")
	assert.Nil(t, got)
	assert.Equal(t, 0, total)
	repo.AssertNotCalled(t, "ListOrdersByCustomer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetRidesHistory_ListError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	repo.On("CountOrdersByCustomer", mock.Anything, svcCustomerID, "").Return(1, nil)
	repo.On("ListOrdersByCustomer", mock.Anything, svcCustomerID, "", 20, 0).Return(nil, errors.New("conn refused"))

	svc := NewService(repo, lgr, nil, mDB)
	got, total, err := svc.GetRidesHistory(context.Background(), svcCustomerID, 1, 20, "")
	assert.ErrorContains(t, err, "conn refused")
	assert.Nil(t, got)
	assert.Equal(t, 0, total)
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
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, svcDriverID).
		Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
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
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, svcDriverID).
		Return(svcDriver(workingStatusIdle, decimal.Zero), nil).Once()
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(nil).Once()
	repo.On("AssignDriver", mock.Anything, mock.Anything, svcOrderID, svcDriverID).Return(true, nil)
	repo.On("MarkDriverBusy", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Driver kedua: lock tidak tersedia → conflict 503/ErrLockTimeout.
	repo.On("GetDriver", mock.Anything, driver2).Return(
		&Driver{ID: driver2, UserType: userTypeDriver, Status: "ACTIVE", WorkingStatus: workingStatusIdle, MinBalanceThreshold: decimal.Zero}, nil)
	repo.On("GetDriverBalance", mock.Anything, driver2).Return(decimal.NewFromInt(100000), nil)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, driver2).
		Return(svcDriver(workingStatusIdle, decimal.Zero), nil).Once()
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
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, svcDriverID).
		Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
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
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, svcDriverID).
		Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("LockOrderForAccept", mock.Anything, mock.Anything, svcOrderID).Return(nil)
	repo.On("AssignDriver", mock.Anything, mock.Anything, svcOrderID, svcDriverID).Return(false, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrOrderNotSearching)
	repo.AssertExpectations(t)
}

func TestAcceptOrder_LockRevalidatesBusy(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	// Pre-check membaca IDLE, tapi di antara pre-check dan lock user, driver
	// berubah BUSY (di-assign order lain). Re-validasi DI BAWAH LOCK harus
	// menolak (409) — ini mencegah double assign (TD-071).
	repo.On("GetDriver", mock.Anything, svcDriverID).Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
	repo.On("GetDriverBalance", mock.Anything, svcDriverID).Return(decimal.NewFromInt(100000), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, svcDriverID).
		Return(svcDriver(workingStatusBusy, decimal.Zero), nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.AcceptOrder(context.Background(), svcOrderID, svcDriverID)
	assert.ErrorIs(t, err, ErrDriverBusy)
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
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonCustomerCancel, decimal.Zero).Return(true, nil)
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
	// Fee 5.000 tetap dicatat di DB (cancellation_fee) tanpa ledger (cash offline).
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, reasonCustomerCancel, decimal.NewFromInt(5000)).Return(true, nil)
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
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, reasonDriverEmergency, decimal.Zero).Return(true, nil)
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

func TestCancelOrder_FeeCalculationTable(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		reason  string
		wantFee int64
	}{
		{name: "before assign", status: statusSearchingDriver, reason: reasonCustomerCancel, wantFee: 0},
		{name: "created by customer", status: statusCreated, reason: reasonCustomerCancel, wantFee: 0},
		{name: "assigned customer cancel", status: statusDriverAssigned, reason: reasonCustomerCancel, wantFee: 5000},
		{name: "arrived customer cancel", status: statusDriverArrived, reason: reasonCustomerCancel, wantFee: 10000},
		{name: "trip started cancel", status: statusTripStarted, reason: reasonCustomerCancel, wantFee: 10000},
		{name: "no-show", status: statusDriverArrived, reason: reasonNoShow, wantFee: 10000},
		{name: "driver emergency", status: statusDriverAssigned, reason: reasonDriverEmergency, wantFee: 0},
		{name: "expired", status: statusSearchingDriver, reason: reasonExpired, wantFee: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cancellationFeeFor(tc.status, tc.reason)
			assert.True(t, got.Equal(decimal.NewFromInt(tc.wantFee)),
				"fee %s/%s = %v, expect %d", tc.status, tc.reason, got, tc.wantFee)
		})
	}
}

// setupCancelFeeTest membangun mock repo/ledger/db umum untuk cancel ber-fee
// (WALLET): cancel sukses + refund fee + driver reset + audit event.
func setupCancelFeeTest(est decimal.Decimal, status, reason string) (*mockRepo, *mockLedger, pgxmock.PgxPoolIface, *RideOrder) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	drv := svcDriverID
	order := svcOrder(status, PaymentMethodWallet, &drv)
	order.EstimatedFare = est
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcDriverID, WalletTypeDriver).Return(svcDriverWallet(decimal.Zero), nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, svcDriverID).Return(nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectCommit()

	return repo, lgr, mDB, order
}

// assertLedgerFee memverifikasi double-entry pada cancel ber-fee: escrow
// di-DEBIT total estimated; CREDIT driver = fee; CREDIT customer = est - fee.
func assertLedgerFee(t *testing.T, args mock.Arguments, est, fee int64) {
	t.Helper()
	entries, ok := args.Get(0).([]wallet.LedgerEntry)
	assert.True(t, ok, "CreateLedgerEntries harus menerima []wallet.LedgerEntry")
	assert.Len(t, entries, 4)

	var debit, credit, driverCredit, customerCredit decimal.Decimal
	expDriver := decimal.NewFromInt(fee)
	expCustomer := decimal.NewFromInt(est - fee)
	for _, e := range entries {
		if e.EntryType == wallet.EntryDebit {
			debit = debit.Add(e.Amount)
		} else {
			credit = credit.Add(e.Amount)
			if e.WalletID == svcDriverWalletID {
				driverCredit = driverCredit.Add(e.Amount)
			}
			if e.WalletID == svcWalletID {
				customerCredit = customerCredit.Add(e.Amount)
			}
		}
	}
	assert.True(t, debit.Equal(credit), "ledger tidak seimbang: DEBIT=%v CREDIT=%v", debit, credit)
	assert.True(t, debit.Equal(decimal.NewFromInt(est)), "DEBIT escrow harus = estimated %d, got %v", est, debit)
	assert.True(t, driverCredit.Equal(expDriver), "driver harus dikredit %d, got %v", fee, driverCredit)
	assert.True(t, customerCredit.Equal(expCustomer), "customer harus dikredit %d, got %v", est-fee, customerCredit)
}

// Cancel WALLET setelah DRIVER_ASSIGNED → fee 5.000 ke driver, refund
// (estimated - 5.000) ke customer, ledger double-entry seimbang.
func TestCancelOrder_FeeAssignedWallet(t *testing.T) {
	repo, lgr, mDB, _ := setupCancelFeeTest(decimal.NewFromInt(50000), statusDriverAssigned, reasonCustomerCancel)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverAssigned, reasonCustomerCancel,
		decimal.NewFromInt(5000)).Return(true, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertLedgerFee(t, args, 50000, 5000) }).Return(nil)

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled, Reason: reasonCustomerCancel,
	})
	assert.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	assert.NotNil(t, resp.CancellationFee)
	assert.Equal(t, int64(5000), resp.CancellationFee.IntPart())
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// Cancel WALLET setelah DRIVER_ARRIVED → fee 10.000 ke driver.
func TestCancelOrder_FeeArrivedWallet(t *testing.T) {
	repo, lgr, mDB, _ := setupCancelFeeTest(decimal.NewFromInt(50000), statusDriverArrived, reasonCustomerCancel)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverArrived, reasonCustomerCancel,
		decimal.NewFromInt(10000)).Return(true, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertLedgerFee(t, args, 50000, 10000) }).Return(nil)

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled, Reason: reasonCustomerCancel,
	})
	assert.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	assert.NotNil(t, resp.CancellationFee)
	assert.Equal(t, int64(10000), resp.CancellationFee.IntPart())
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TC-INT-RD-005 — No-Show: driver sudah DRIVER_ARRIVED, customer tidak muncul
// → fee 10.000 dari customer, kompensasi penuh ke driver.
func TestCancelOrder_NoShow(t *testing.T) {
	repo, lgr, mDB, _ := setupCancelFeeTest(decimal.NewFromInt(50000), statusDriverArrived, reasonNoShow)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusDriverArrived, reasonNoShow,
		decimal.NewFromInt(10000)).Return(true, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertLedgerFee(t, args, 50000, 10000) }).Return(nil)

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCancelled, Reason: reasonNoShow,
	})
	assert.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	assert.NotNil(t, resp.CancellationFee)
	assert.Equal(t, int64(10000), resp.CancellationFee.IntPart())
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// NO_SHOW hanya boleh dari driver tertunjuk (customer dilarang memicu no-show).
func TestCancelOrder_NoShowNotDriver(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	order := svcOrder(statusDriverArrived, PaymentMethodWallet, &svcDriverID)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled, Reason: reasonNoShow,
	})
	assert.ErrorIs(t, err, ErrNotAllowed)
	repo.AssertNotCalled(t, "CancelOrder")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// NO_SHOW hanya valid saat status DRIVER_ARRIVED (driver menunggu di pickup).
func TestCancelOrder_NoShowWrongStatus(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCancelled, Reason: reasonNoShow,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	repo.AssertNotCalled(t, "CancelOrder")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// Anti-bypass (security): customer yang mengirim reason DRIVER_EMERGENCY
// tidak boleh lolos dari cancellation fee — guard aktor di cancelOrderTx.
func TestCancelOrder_CustomerEmergencyBypass(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, _ := pgxmock.NewPool()

	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID)
	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))

	svc := NewService(repo, lgr, nil, mDB)
	_, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled, Reason: reasonDriverEmergency,
	})
	assert.ErrorIs(t, err, ErrNotAllowed)
	repo.AssertNotCalled(t, "CancelOrder")
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

func TestSettlement_NoDelta_ActualEqualEstimated(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		decMatch(decimal.NewFromInt(50000)), mock.Anything, mock.Anything).Return(true, nil)
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

	actual := decimal.NewFromInt(50000)
	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted, ActualFare: &actual,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettlement_SurplusDelta_Refund(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		decMatch(decimal.NewFromInt(40000)), mock.Anything, mock.Anything).Return(true, nil)
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
	// Delta surplus: lock customer+escrow.
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	// Settlement: lock escrow+driver+platform.
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectCommit()

	actual := decimal.NewFromInt(40000)
	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted, ActualFare: &actual,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettlement_ShortfallDelta_CoveredFromWallet(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		decMatch(decimal.NewFromInt(60000)), mock.Anything, mock.Anything).Return(true, nil)
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
	// Delta shortfall: lock customer+escrow lalu baca balance.
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(80000)))
	// Settlement: lock escrow+driver+platform.
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectCommit()

	actual := decimal.NewFromInt(60000)
	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted, ActualFare: &actual,
	})

	assert.NoError(t, err)
	assert.Equal(t, statusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettlement_ShortfallDelta_Insufficient_SubsidyOverdue(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	drv := svcDriverID
	order := svcOrder(statusTripStarted, PaymentMethodWallet, &drv)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CompleteOrder", mock.Anything, mock.Anything, svcOrderID, statusTripStarted,
		decMatch(decimal.NewFromInt(60000)), mock.Anything, mock.Anything).Return(true, nil)
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
	// Delta shortfall: lock customer+escrow → balance 0 (tidak cukup).
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.Zero))
	// Subsidi: lock SYSTEM_PLATFORM (fallback TD-132).
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 1"))
	repo.On("IncrementOverdueDebt", mock.Anything, mock.Anything, svcCustomerID,
		decMatch(decimal.NewFromInt(10000))).Return(nil)
	// Settlement: lock escrow+driver+platform.
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectCommit()

	actual := decimal.NewFromInt(60000)
	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcDriverID, Status: statusCompleted, ActualFare: &actual,
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
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonExpired, decimal.Zero).Return(true, nil)
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
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, svcDriverID).
		Return(svcDriver(workingStatusIdle, decimal.Zero), nil)
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
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonCustomerCancel, decimal.Zero).Return(false, nil)

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

// ---- Voucher discount (TD-070) ----

var svcVoucherID = uuid.MustParse("88888888-8888-8888-8888-888888888888")

// svcVoucher membangun fixture voucher dengan window waktu aktif saat ini.
func svcVoucher(discountType string, value decimal.Decimal, maxDiscount *decimal.Decimal, minOrder decimal.Decimal, perUserLimit int) *Voucher {
	return &Voucher{
		ID:                 svcVoucherID,
		Code:               "RIDE20",
		Name:               "Diskon Ride 20%",
		DiscountType:       discountType,
		DiscountValue:      value,
		MaxDiscount:        maxDiscount,
		ApplicableServices: []string{"RIDE"},
		MinOrderAmount:     minOrder,
		PerUserLimit:       perUserLimit,
		UsedCount:          1,
		ValidFrom:          time.Now().Add(-time.Hour),
		ValidTo:            time.Now().Add(24 * time.Hour),
		Status:             "ACTIVE",
	}
}

// assertEscrowEntries memverifikasi double-entry holdEscrow: DEBIT customer =
// escrow & CREDIT SYSTEM_ESCROW = escrow (jumlah sama, TD-070 post-diskon).
func assertEscrowEntries(t *testing.T, args mock.Arguments, want decimal.Decimal) {
	t.Helper()
	entries, ok := args.Get(0).([]wallet.LedgerEntry)
	assert.True(t, ok, "CreateLedgerEntries harus menerima []wallet.LedgerEntry")
	assert.Len(t, entries, 2)
	assert.Equal(t, wallet.EntryDebit, entries[0].EntryType)
	assert.Equal(t, wallet.EntryCredit, entries[1].EntryType)
	assert.True(t, entries[0].Amount.Equal(want), "DEBIT customer = %v want %v", entries[0].Amount, want)
	assert.True(t, entries[1].Amount.Equal(want), "CREDIT escrow = %v want %v", entries[1].Amount, want)
}

// assertFullRefundEntries memverifikasi refundEscrow: escrow di-DEBIT &
// customer di-CREDIT sebesar fareBasis (post-diskon), bukan estimated penuh.
func assertFullRefundEntries(t *testing.T, args mock.Arguments, want decimal.Decimal) {
	t.Helper()
	entries, ok := args.Get(0).([]wallet.LedgerEntry)
	assert.True(t, ok, "CreateLedgerEntries harus menerima []wallet.LedgerEntry")
	assert.Len(t, entries, 2)
	assert.Equal(t, wallet.EntryDebit, entries[0].EntryType)
	assert.Equal(t, wallet.EntryCredit, entries[1].EntryType)
	assert.True(t, entries[0].Amount.Equal(want), "DEBIT escrow = %v want %v", entries[0].Amount, want)
	assert.True(t, entries[1].Amount.Equal(want), "CREDIT customer = %v want %v", entries[1].Amount, want)
}

func svcBookingFare(req BookRideRequest) (dist, est decimal.Decimal) {
	dist = haversineKm(req.PickupLat, req.PickupLng, req.DropoffLat, req.DropoffLng).Round(3)
	est = baseFare.Add(dist.Mul(perKmRate)).Round(2)
	return dist, est
}

// BookRide WALLET memakai voucher PERCENTAGE: diskon = estimated × 20/100,
// escrow = estimated − diskon, user_vouchers dicatat APPLIED, response
// menampilkan discount_amount + voucher_code.
func TestBookRide_VoucherPercentage(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-vpct"
	req := svcValidReq(PaymentMethodWallet, idemKey)
	code := "RIDE20"
	req.VoucherCode = &code

	_, est := svcBookingFare(req)
	voucher := svcVoucher(voucherDiscountPct, decimal.NewFromInt(20), nil, decimal.Zero, 1)
	discount := est.Mul(decimal.NewFromInt(20)).Div(decimal.NewFromInt(100)).Round(2)
	escrow := est.Sub(discount)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("GetVoucherByCode", mock.Anything, code).Return(voucher, nil)
	repo.On("CountUserVoucherUsage", mock.Anything, mock.Anything, svcCustomerID, voucher.ID).Return(0, nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			order := args.Get(2).(*RideOrder)
			require.NotNil(t, order.VoucherID)
			require.NotNil(t, order.DiscountAmount)
			assert.Equal(t, voucher.ID, *order.VoucherID)
			assert.True(t, order.DiscountAmount.Equal(discount), "discount_amount = %v want %v", *order.DiscountAmount, discount)
		}).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	repo.On("InsertUserVoucher", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			uv := args.Get(2).(*UserVoucher)
			assert.Equal(t, voucher.ID, uv.VoucherID)
			assert.Equal(t, orderTypeRide, uv.OrderType)
			assert.Equal(t, voucherUsageApplied, uv.Status)
			assert.True(t, uv.DiscountApplied.Equal(discount), "discount_applied = %v want %v", uv.DiscountApplied, discount)
		}).Return(nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertEscrowEntries(t, args, escrow) }).Return(nil)

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

	require.NoError(t, err)
	require.NotNil(t, resp.VoucherCode)
	assert.Equal(t, code, *resp.VoucherCode)
	assert.True(t, resp.DiscountAmount.Equal(discount), "discount_amount = %v want %v", resp.DiscountAmount, discount)
	assert.True(t, resp.EscrowAmount.Equal(escrow), "escrow = %v want %v", resp.EscrowAmount, escrow)
	assert.True(t, resp.EstimatedFare.Equal(est))
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// BookRide WALLET memakai voucher FIXED via voucher_id: diskon flat = value,
// escrow = estimated − 10.000.
func TestBookRide_VoucherFixed(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-vfix"
	req := svcValidReq(PaymentMethodWallet, idemKey)
	req.VoucherID = &svcVoucherID

	_, est := svcBookingFare(req)
	voucher := svcVoucher(voucherDiscountFixed, decimal.NewFromInt(10000), nil, decimal.Zero, 1)
	discount := decimal.NewFromInt(10000)
	escrow := est.Sub(discount)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("GetVoucherByID", mock.Anything, svcVoucherID).Return(voucher, nil)
	repo.On("CountUserVoucherUsage", mock.Anything, mock.Anything, svcCustomerID, voucher.ID).Return(0, nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			order := args.Get(2).(*RideOrder)
			require.NotNil(t, order.DiscountAmount)
			assert.True(t, order.DiscountAmount.Equal(discount), "discount_amount = %v want %v", *order.DiscountAmount, discount)
		}).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	repo.On("InsertUserVoucher", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertEscrowEntries(t, args, escrow) }).Return(nil)

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

	require.NoError(t, err)
	assert.True(t, resp.DiscountAmount.Equal(discount), "discount_amount = %v want %v", resp.DiscountAmount, discount)
	assert.True(t, resp.EscrowAmount.Equal(escrow), "escrow = %v want %v", resp.EscrowAmount, escrow)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// Voucher sudah lewat valid_to → ErrVoucherExpired; tidak ada order dibuat.
func TestBookRide_VoucherExpired(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodWallet, "book-key-vexp")
	code := "RIDE20"
	req.VoucherCode = &code

	voucher := svcVoucher(voucherDiscountPct, decimal.NewFromInt(20), nil, decimal.Zero, 1)
	voucher.ValidFrom = time.Now().Add(-48 * time.Hour)
	voucher.ValidTo = time.Now().Add(-24 * time.Hour)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("GetVoucherByCode", mock.Anything, code).Return(voucher, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)

	assert.ErrorIs(t, err, ErrVoucherExpired)
	repo.AssertNotCalled(t, "InsertOrder")
	repo.AssertExpectations(t)
}

// Estimated fare di bawah min_order_amount → ErrVoucherMinOrder.
func TestBookRide_VoucherMinOrder(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodWallet, "book-key-vmin")
	code := "RIDE20"
	req.VoucherCode = &code

	_, est := svcBookingFare(req)
	voucher := svcVoucher(voucherDiscountFixed, decimal.NewFromInt(10000), nil, est.Add(decimal.NewFromInt(1)), 1)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("GetVoucherByCode", mock.Anything, code).Return(voucher, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)

	assert.ErrorIs(t, err, ErrVoucherMinOrder)
	repo.AssertNotCalled(t, "InsertOrder")
	repo.AssertExpectations(t)
}

// Usage sudah menyentuh per_user_limit → ErrVoucherPerUserLimit. Pre-check
// dilakukan di dalam transaksi sebelum order dibuat (early exit).
func TestBookRide_VoucherPerUserLimit(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-vlim"
	req := svcValidReq(PaymentMethodWallet, idemKey)
	code := "RIDE20"
	req.VoucherCode = &code

	voucher := svcVoucher(voucherDiscountPct, decimal.NewFromInt(20), nil, decimal.Zero, 1)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("GetVoucherByCode", mock.Anything, code).Return(voucher, nil)
	repo.On("CountUserVoucherUsage", mock.Anything, mock.Anything, svcCustomerID, voucher.ID).Return(1, nil)

	setupBookRideDB(mDB, idemKey)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)

	assert.ErrorIs(t, err, ErrVoucherPerUserLimit)
	repo.AssertNotCalled(t, "InsertOrder")
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// PERCENTAGE dengan max_discount: diskon dibatasi (est×50% = jauh di atas
// 5.000) → discount_amount = 5.000, escrow = est − 5.000.
func TestBookRide_VoucherMaxDiscountCap(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "book-key-vcap"
	req := svcValidReq(PaymentMethodWallet, idemKey)
	code := "RIDE50"
	req.VoucherCode = &code

	_, est := svcBookingFare(req)
	maxDisc := decimal.NewFromInt(5000)
	voucher := svcVoucher(voucherDiscountPct, decimal.NewFromInt(50), &maxDisc, decimal.Zero, 1)
	discount := decimal.NewFromInt(5000)
	escrow := est.Sub(discount)

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)
	repo.On("GetVoucherByCode", mock.Anything, code).Return(voucher, nil)
	repo.On("CountUserVoucherUsage", mock.Anything, mock.Anything, svcCustomerID, voucher.ID).Return(0, nil)
	repo.On("InsertOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("TransitionStatus", mock.Anything, mock.Anything, mock.Anything, statusCreated, statusSearchingDriver).Return(true, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	repo.On("InsertUserVoucher", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertEscrowEntries(t, args, escrow) }).Return(nil)

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

	require.NoError(t, err)
	assert.True(t, resp.DiscountAmount.Equal(discount), "discount_amount = %v want %v", resp.DiscountAmount, discount)
	assert.True(t, resp.EscrowAmount.Equal(escrow), "escrow = %v want %v", resp.EscrowAmount, escrow)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// VoucherCode + VoucherID diisi keduanya → ErrVoucherInvalid.
func TestBookRide_VoucherBothFields(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := svcValidReq(PaymentMethodWallet, "book-key-vboth")
	code := "RIDE20"
	req.VoucherCode = &code
	req.VoucherID = &svcVoucherID

	repo.On("GetCustomer", mock.Anything, svcCustomerID).Return(svcCustomer(decimal.Zero), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, svcCustomerID, WalletTypeCustomer).Return(svcWallet(decimal.NewFromInt(200000)), nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.BookRide(context.Background(), req)

	assert.ErrorIs(t, err, ErrVoucherInvalid)
	repo.AssertNotCalled(t, "GetVoucherByCode")
	repo.AssertNotCalled(t, "GetVoucherByID")
	repo.AssertExpectations(t)
}

// Cancel WALLET tanpa fee untuk order ber-voucher: refund escrow = fareBasis
// (estimated − discount), bukan estimated penuh (anti over-refund).
func TestCancelOrder_VoucherRefundDiscounted(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	discount := decimal.NewFromInt(10000)
	order := svcOrder(statusSearchingDriver, PaymentMethodWallet, nil)
	order.DiscountAmount = &discount
	refund := fareBasis(order)

	repo.On("GetOrderByID", mock.Anything, svcOrderID).Return(order, nil)
	repo.On("LockOrderForUpdate", mock.Anything, mock.Anything, svcOrderID).Return(order, nil)
	repo.On("CancelOrder", mock.Anything, mock.Anything, svcOrderID, statusSearchingDriver, reasonCustomerCancel, decimal.Zero).Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, WalletTypeSystemEscrow).Return(svcEscrowID, nil)
	repo.On("InsertEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Run(func(args mock.Arguments) { assertFullRefundEntries(t, args, refund) }).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectCommit()

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.UpdateRideStatus(context.Background(), UpdateRideStatusRequest{
		OrderID: svcOrderID, UserID: svcCustomerID, Status: statusCancelled, Reason: reasonCustomerCancel,
	})
	require.NoError(t, err)
	assert.Equal(t, statusCancelled, resp.Status)
	require.NotNil(t, resp.CancellationFee)
	assert.True(t, resp.CancellationFee.IsZero(), "fee harus 0 untuk cancel sebelum driver assigned")
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}
