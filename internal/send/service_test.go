package send

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

func (m *mockRepo) GetCustomer(ctx context.Context, userID uuid.UUID) (*SendCustomer, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendCustomer), args.Error(1)
}

func (m *mockRepo) GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*SendWallet, error) {
	args := m.Called(ctx, userID, walletType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendWallet), args.Error(1)
}

func (m *mockRepo) SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error) {
	args := m.Called(ctx, q, walletType)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *mockRepo) InsertSendOrder(ctx context.Context, q Querier, o *SendOrder) error {
	args := m.Called(ctx, q, o)
	return args.Error(0)
}

func (m *mockRepo) InsertSendOrderStops(ctx context.Context, q Querier, stops []*SendOrderStop) error {
	args := m.Called(ctx, q, stops)
	return args.Error(0)
}

func (m *mockRepo) InsertSendOrderEvent(ctx context.Context, q Querier, e SendOrderEvent) error {
	args := m.Called(ctx, q, e)
	return args.Error(0)
}

func (m *mockRepo) GetSendOrderByID(ctx context.Context, orderID uuid.UUID) (*SendOrder, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendOrder), args.Error(1)
}

func (m *mockRepo) LockSendOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*SendOrder, error) {
	args := m.Called(ctx, q, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendOrder), args.Error(1)
}

func (m *mockRepo) UpdateSendOrderStatus(ctx context.Context, q Querier, orderID uuid.UUID, fromStatus, toStatus string) (bool, error) {
	args := m.Called(ctx, q, orderID, fromStatus, toStatus)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) GetSendOrderStops(ctx context.Context, orderID uuid.UUID) ([]*SendOrderStop, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*SendOrderStop), args.Error(1)
}

func (m *mockRepo) GetDriver(ctx context.Context, driverID uuid.UUID) (*SendDriver, error) {
	args := m.Called(ctx, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendDriver), args.Error(1)
}

func (m *mockRepo) GetDriverActiveSendOrdersCount(ctx context.Context, driverID uuid.UUID) (int, error) {
	args := m.Called(ctx, driverID)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) LockSendOrderForAccept(ctx context.Context, q Querier, orderID uuid.UUID) (*SendOrder, error) {
	args := m.Called(ctx, q, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendOrder), args.Error(1)
}

func (m *mockRepo) LockDriverUserForAccept(ctx context.Context, q Querier, driverID uuid.UUID) error {
	args := m.Called(ctx, q, driverID)
	return args.Error(0)
}

func (m *mockRepo) AssignDriverToSendOrder(ctx context.Context, q Querier, orderID uuid.UUID, driverID uuid.UUID) (bool, error) {
	args := m.Called(ctx, q, orderID, driverID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) UpdateDriverWorkingStatus(ctx context.Context, q Querier, driverID uuid.UUID, status string) error {
	args := m.Called(ctx, q, driverID, status)
	return args.Error(0)
}

func (m *mockRepo) MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error {
	args := m.Called(ctx, q, driverID)
	return args.Error(0)
}

func (m *mockRepo) MarkSendOrderDelivered(ctx context.Context, q Querier, orderID uuid.UUID) (bool, error) {
	args := m.Called(ctx, q, orderID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) MarkSendOrderSettled(ctx context.Context, q Querier, orderID uuid.UUID) error {
	args := m.Called(ctx, q, orderID)
	return args.Error(0)
}

func (m *mockRepo) GetSendOrderStopsByOrderID(ctx context.Context, q Querier, orderID uuid.UUID) ([]*SendOrderStop, error) {
	args := m.Called(ctx, q, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*SendOrderStop), args.Error(1)
}

func (m *mockRepo) UpdateSendOrderStopStatus(ctx context.Context, q Querier, stopID uuid.UUID, status string, proof *string) (bool, error) {
	args := m.Called(ctx, q, stopID, status, proof)
	return args.Bool(0), args.Error(1)
}

type mockLedger struct {
	mock.Mock
}

func (m *mockLedger) CreateLedgerEntries(ctx context.Context, tx pgx.Tx, entries []wallet.LedgerEntry) error {
	args := m.Called(ctx, tx, entries)
	return args.Error(0)
}

// ---- fixtures ----

var (
	fCustID     = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fDriverID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	fCustWallet = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	fEscrowID   = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	fPlatformID = uuid.MustParse("77777777-7777-7777-7777-777777777777")
	fDriverWID  = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	fOrderID    = uuid.MustParse("99999999-9999-9999-9999-999999999999")
	fStop1ID    = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	fStop2ID    = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
)

func fSendCust() *SendCustomer {
	return &SendCustomer{ID: fCustID, UserType: userTypeCustomer, Status: statusActive, OverdueDebt: decimal.Zero}
}

func fSendCustWallet(balance decimal.Decimal) *SendWallet {
	return &SendWallet{ID: fCustWallet, UserID: fCustID, Type: walletTypeCustomer, Balance: balance, Status: statusActive}
}

func fSendDriver() *SendDriver {
	return &SendDriver{ID: fDriverID, UserType: userTypeDriver, Status: statusActive, WorkingStatus: workingStatusIdle, MinBalanceThreshold: decimal.Zero}
}

func fSendDriverWallet(balance decimal.Decimal) *SendWallet {
	return &SendWallet{ID: fDriverWID, UserID: fDriverID, Type: walletTypeDriver, Balance: balance, Status: statusActive}
}

func fSendOrder(status, paymentMethod string, driver *uuid.UUID) *SendOrder {
	o := &SendOrder{
		ID: fOrderID, SenderID: fCustID, DriverID: driver,
		SenderWalletID:  &fCustWallet,
		PackageWeightKg: decimal.NewFromInt(2),
		PickupLat:       decimal.NewFromFloat(-6.2), PickupLng: decimal.NewFromFloat(106.8),
		PickupAddress:      "Jl. Pickup",
		BaseFare:           sendBaseFare,
		DistanceKm:         decimal.NewFromFloat(11.112),
		WeightSurcharge:    decimal.Zero,
		TotalFare:          decimal.NewFromInt(48336),
		DeclaredValue:      decimal.Zero,
		PackageType:        "STANDARD",
		InsuranceFee:       decimal.Zero,
		DiscountAmount:     decimal.Zero,
		PaymentMethod:      paymentMethod,
		Status:             status,
		CreatedAt:          time.Now().Add(-time.Hour),
		PlatformCommission: decimalPtr(decimal.NewFromInt(4834)),
		DriverEarning:      decimalPtr(decimal.NewFromInt(43502)),
	}
	if driver != nil {
		o.DriverWalletID = &fDriverWID
	}
	return o
}

func decimalPtr(d decimal.Decimal) *decimal.Decimal {
	return &d
}

func fSendStop(orderID uuid.UUID, id uuid.UUID, num int, status string) *SendOrderStop {
	return &SendOrderStop{
		ID: id, OrderID: orderID, StopNumber: num,
		DropoffLat: decimal.NewFromFloat(-6.25), DropoffLng: decimal.NewFromFloat(106.8),
		DropoffAddress: "Jl. Stop",
		Status:         status,
	}
}

func fSendReq(paymentMethod, idemKey string) CreateSendOrderRequest {
	return CreateSendOrderRequest{
		UserID:          fCustID,
		PickupLat:       -6.2,
		PickupLng:       106.8,
		PickupAddress:   "Jl. Pickup",
		PackageWeightKg: 2,
		PackageType:     "STANDARD",
		DeclaredValue:   decimal.Zero,
		PaymentMethod:   paymentMethod,
		Stops: []SendStopRequest{
			{RecipientName: "A", RecipientPhone: "0811", DeliveryAddress: "Jl. A", DeliveryLat: -6.25, DeliveryLng: 106.8},
			{RecipientName: "B", RecipientPhone: "0812", DeliveryAddress: "Jl. B", DeliveryLat: -6.3, DeliveryLng: 106.8},
		},
		IdempotencyKey: idemKey,
	}
}

func setupSendDB(mDB pgxmock.PgxPoolIface, idemKey string, owner uuid.UUID) {
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, owner).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, owner).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
}

func sendBeginTx(t *testing.T, mDB pgxmock.PgxPoolIface) pgx.Tx {
	t.Helper()
	mDB.ExpectBegin()
	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	return tx
}

func sendLockWallets(mDB pgxmock.PgxPoolIface, tag string) {
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag(tag))
}

// ---- fare & validation helpers ----

func Test_calculateFare(t *testing.T) {
	req := fSendReq(PaymentMethodCash, "k")
	req.PackageWeightKg = 8                        // kelebihan 3kg * 2000 = 6000
	req.DeclaredValue = decimal.NewFromInt(200000) // 1% = 2000 (under cap)
	base, distCharge, weight, ins, total := (&Service{}).calculateFare(req, decimal.NewFromFloat(10))
	assert.True(t, base.Equal(sendBaseFare))
	assert.True(t, distCharge.Equal(decimal.NewFromInt(30000)))
	assert.True(t, weight.Equal(decimal.NewFromInt(6000)))
	assert.True(t, ins.Equal(decimal.NewFromInt(2000)))
	assert.True(t, total.Equal(decimal.NewFromInt(53000)))
}

func Test_calculateFare_InsuranceCap(t *testing.T) {
	req := fSendReq(PaymentMethodCash, "k")
	req.DeclaredValue = decimal.NewFromInt(6000000) // 1% = 60000 → cap 50000
	_, _, _, ins, total := (&Service{}).calculateFare(req, decimal.Zero)
	assert.True(t, ins.Equal(insuranceMax))
	assert.True(t, total.Equal(sendBaseFare.Add(insuranceMax)))
}

func Test_calculateFare_NoSurcharge(t *testing.T) {
	req := fSendReq(PaymentMethodCash, "k")
	req.PackageWeightKg = 5
	_, _, weight, ins, total := (&Service{}).calculateFare(req, decimal.Zero)
	assert.True(t, weight.IsZero())
	assert.True(t, ins.IsZero())
	assert.True(t, total.Equal(sendBaseFare))
}

func Test_allocateFares(t *testing.T) {
	svc := &Service{}
	total := decimal.NewFromInt(48000)
	dists := []stopDistance{{index: 0, km: decimal.NewFromFloat(5.556)}, {index: 1, km: decimal.NewFromFloat(5.556)}}
	allocs := svc.allocateFares(total, dists)
	require.Len(t, allocs, 2)
	sum := allocs[0].Add(allocs[1])
	assert.True(t, sum.Equal(total))
}

func Test_allocateFares_ZeroDistance(t *testing.T) {
	svc := &Service{}
	total := decimal.NewFromInt(15000)
	allocs := svc.allocateFares(total, []stopDistance{{index: 0, km: decimal.Zero}, {index: 1, km: decimal.Zero}})
	require.Len(t, allocs, 2)
	assert.True(t, allocs[0].IsZero())
	assert.True(t, allocs[1].Equal(total))
}

func Test_allocateFares_Empty(t *testing.T) {
	svc := &Service{}
	allocs := svc.allocateFares(decimal.NewFromInt(1000), nil)
	assert.Empty(t, allocs)
}

func Test_calculateDistances(t *testing.T) {
	req := fSendReq(PaymentMethodCash, "k")
	total, dists := (&Service{}).calculateDistances(req)
	require.Len(t, dists, 2)
	assert.True(t, total.IsPositive())
	assert.True(t, dists[0].km.Equal(dists[1].km))
}

func Test_validPackageType(t *testing.T) {
	assert.True(t, validPackageType("STANDARD"))
	assert.True(t, validPackageType("FRAGILE"))
	assert.True(t, validPackageType("LIQUID"))
	assert.True(t, validPackageType("ELECTRONICS"))
	assert.False(t, validPackageType("GLASS"))
}

func Test_normalizeStopStatus(t *testing.T) {
	assert.Equal(t, stopStatusDelivered, normalizeStopStatus("COMPLETED"))
	assert.Equal(t, stopStatusDelivered, normalizeStopStatus("DELIVERED"))
	assert.Equal(t, stopStatusPickedUp, normalizeStopStatus("PICKED_UP"))
	assert.Equal(t, stopStatusPickedUp, normalizeStopStatus("ARRIVED"))
	assert.Equal(t, stopStatusCancelled, normalizeStopStatus("CANCELLED"))
	assert.Equal(t, stopStatusCancelled, normalizeStopStatus("SKIPPED"))
	assert.Equal(t, "", normalizeStopStatus("PENDING"))
	assert.Equal(t, "", normalizeStopStatus("OCI"))
}

func Test_validSendStatusTarget(t *testing.T) {
	assert.True(t, validSendStatusTarget(sendStatusCancelled))
	assert.True(t, validSendStatusTarget(sendStatusPickedUp))
	assert.True(t, validSendStatusTarget(sendStatusInTransit))
	assert.True(t, validSendStatusTarget(sendStatusDelivered))
	assert.False(t, validSendStatusTarget("SETTLED"))
}

func Test_validateSendTransition(t *testing.T) {
	order := fSendOrder(sendStatusCreated, PaymentMethodCash, nil)
	assert.NoError(t, validateSendTransition(order, actorKindSender, sendStatusCancelled))
	assert.ErrorIs(t, validateSendTransition(order, actorKindSender, sendStatusPickedUp), ErrInvalidTransition)

	assigned := fSendOrder(sendStatusDriverAssigned, PaymentMethodCash, &fDriverID)
	assert.NoError(t, validateSendTransition(assigned, actorKindDriver, sendStatusPickedUp))
	assert.NoError(t, validateSendTransition(assigned, actorKindDriver, sendStatusCancelled))
	assert.ErrorIs(t, validateSendTransition(assigned, actorKindDriver, sendStatusDelivered), ErrInvalidTransition)

	delivering := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	assert.NoError(t, validateSendTransition(delivering, actorKindDriver, sendStatusDelivered))
	assert.ErrorIs(t, validateSendTransition(delivering, 99, sendStatusDelivered), ErrNotAllowed)
}

func Test_derefDecimal(t *testing.T) {
	require.True(t, derefDecimal(nil).IsZero())
	v := decimal.NewFromInt(100)
	d := derefDecimal(&v)
	assert.True(t, d.Equal(v))
}

// ---- holdSendEscrow ----

func Test_holdSendEscrow_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)

	svc := NewService(repo, mDB, nil, lgr)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil)
	err = svc.holdSendEscrow(context.Background(), tx, order, decimal.NewFromInt(50000), fCustWallet)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdSendEscrow_SystemWalletError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(uuid.Nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.holdSendEscrow(context.Background(), nil, &SendOrder{}, decimal.NewFromInt(10), fCustWallet)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_holdSendEscrow_LockError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.holdSendEscrow(context.Background(), tx, &SendOrder{}, decimal.NewFromInt(10), fCustWallet)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdSendEscrow_BalanceError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.holdSendEscrow(context.Background(), tx, &SendOrder{}, decimal.NewFromInt(10), fCustWallet)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdSendEscrow_Insufficient(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(10)))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.holdSendEscrow(context.Background(), tx, &SendOrder{}, decimal.NewFromInt(100), fCustWallet)
	assert.ErrorIs(t, err, ErrInsufficientBalance)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdSendEscrow_LedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.holdSendEscrow(context.Background(), tx, &SendOrder{}, decimal.NewFromInt(100), fCustWallet)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- refundSendEscrow ----

func Test_refundSendEscrow_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)

	svc := NewService(repo, mDB, nil, lgr)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil)
	err = svc.refundSendEscrow(context.Background(), tx, order, fCustWallet)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_refundSendEscrow_SystemWalletError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(uuid.Nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.refundSendEscrow(context.Background(), nil, &SendOrder{}, fCustWallet)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_refundSendEscrow_LockError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.refundSendEscrow(context.Background(), tx, &SendOrder{}, fCustWallet)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_refundSendEscrow_LedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.refundSendEscrow(context.Background(), tx, &SendOrder{}, fCustWallet)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- settleSendOrderTx ----

func Test_settleSendOrderTx_NoDriver(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, nil)
	err := svc.settleSendOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrDriverNotFound)
}

func Test_settleSendOrderTx_DriverWalletError(t *testing.T) {
	repo := new(mockRepo)
	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleSendOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_settleSendOrderTx_PlatformError(t *testing.T) {
	repo := new(mockRepo)
	order := fSendOrder(sendStatusDelivered, PaymentMethodWallet, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(uuid.Nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleSendOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_settleSendOrderTx_InvalidPayment(t *testing.T) {
	repo := new(mockRepo)
	order := fSendOrder(sendStatusDelivered, "QRIS", &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleSendOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrInvalidPaymentMethod)
}

func Test_settleSendOrderTx_CashLedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_CashSuspended(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-60000)))
	repo.On("MarkDriverSuspended", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_CashSuspendedError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-60000)))
	repo.On("MarkDriverSuspended", mock.Anything, mock.Anything, fDriverID).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_CashIdle(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-1000)))
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_CashBalanceError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_WalletEscrowError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodWallet, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(uuid.Nil, ErrWalletNotFound)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_WalletLedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodWallet, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_WalletLockError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodWallet, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_WalletSuccess(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodWallet, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_MarkSettledError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.Zero))
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_settleSendOrderTx_EventError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := sendBeginTx(t, mDB)

	order := fSendOrder(sendStatusDelivered, PaymentMethodCash, &fDriverID)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.Zero))
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleSendOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- CreateSendOrder ----

func fCreateSendSvcMocks(repo *mockRepo, lgr *mockLedger, req CreateSendOrderRequest) {
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fSendCustWallet(decimal.NewFromInt(200000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	_ = req
}

func TestCreateSendOrder_ValidationErrors(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.CreateSendOrder(context.Background(), CreateSendOrderRequest{})
	assert.ErrorIs(t, err, ErrIdempotencyKeyRequired)

	req := fSendReq(PaymentMethodWallet, "k")
	req.PaymentMethod = "QRIS"
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidPaymentMethod)

	req = fSendReq(PaymentMethodWallet, "k")
	req.PackageType = "GLASS"
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidPackageType)

	req = fSendReq(PaymentMethodWallet, "k")
	req.PackageWeightKg = 0
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidWeight)

	req = fSendReq(PaymentMethodWallet, "k")
	req.DeclaredValue = decimal.NewFromInt(-1)
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidDeclaredValue)

	req = fSendReq(PaymentMethodWallet, "k")
	req.Stops = nil
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidStopsCount)

	req = fSendReq(PaymentMethodWallet, "k")
	for i := 0; i < 4; i++ {
		req.Stops = append(req.Stops, SendStopRequest{DeliveryAddress: "x", DeliveryLat: -6, DeliveryLng: 106})
	}
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidStopsCount)

	req = fSendReq(PaymentMethodWallet, "k")
	req.PickupLat = 200
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidCoordinates)

	req = fSendReq(PaymentMethodWallet, "k")
	req.Stops[0].DeliveryAddress = ""
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidRecipient)

	req = fSendReq(PaymentMethodWallet, "k")
	req.Stops[1].DeliveryLat = -100
	_, err = svc.CreateSendOrder(context.Background(), req)
	assert.ErrorIs(t, err, ErrInvalidRecipient)
}

func TestCreateSendOrder_CustomerErrors(t *testing.T) {
	repo := new(mockRepo)
	defer repo.AssertExpectations(t)

	repo.On("GetCustomer", mock.Anything, fCustID).Return(nil, ErrUserNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrUserNotFound)

	repo2 := new(mockRepo)
	c := fSendCust()
	c.UserType = userTypeDriver
	repo2.On("GetCustomer", mock.Anything, fCustID).Return(c, nil)
	svc2 := NewService(repo2, nil, nil, new(mockLedger))
	_, err = svc2.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrNotCustomer)

	repo3 := new(mockRepo)
	c3 := fSendCust()
	c3.Status = "SUSPENDED"
	repo3.On("GetCustomer", mock.Anything, fCustID).Return(c3, nil)
	svc3 := NewService(repo3, nil, nil, new(mockLedger))
	_, err = svc3.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrCustomerInactive)

	repo4 := new(mockRepo)
	c4 := fSendCust()
	c4.OverdueDebt = decimal.NewFromInt(1000)
	repo4.On("GetCustomer", mock.Anything, fCustID).Return(c4, nil)
	svc4 := NewService(repo4, nil, nil, new(mockLedger))
	_, err = svc4.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrOverdueDebt)
}

func TestCreateSendOrder_WalletErrors(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrWalletNotFound)

	repo2 := new(mockRepo)
	repo2.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	w := fSendCustWallet(decimal.Zero)
	w.Status = "SUSPENDED"
	repo2.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(w, nil)
	svc2 := NewService(repo2, nil, nil, new(mockLedger))
	_, err = svc2.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrWalletInactive)
}

func TestCreateSendOrder_InsertOrderError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-io-err"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	setupSendDB(mDB, idemKey, fCustID)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_InsertStopsError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-is-err"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	setupSendDB(mDB, idemKey, fCustID)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_WalletEscrowSystemError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-esc-sys"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(uuid.Nil, ErrWalletNotFound)

	setupSendDB(mDB, idemKey, fCustID)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_WalletEscrowInsufficient(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-esc-ins"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)

	setupSendDB(mDB, idemKey, fCustID)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100)))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, ErrInsufficientBalance)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_WalletEscrowLedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-esc-led"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	setupSendDB(mDB, idemKey, fCustID)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_EventError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-ev"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	setupSendDB(mDB, idemKey, fCustID)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_CommitError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-commit"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupSendDB(mDB, idemKey, fCustID)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))
	mDB.ExpectCommit().WillReturnError(pgx.ErrTxClosed)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_CacheError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-cache"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupSendDB(mDB, idemKey, fCustID)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, fCustID, pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_Success_Wallet(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-ok-wallet"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(200000)), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupSendDB(mDB, idemKey, fCustID)
	sendLockWallets(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200000)))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, idemKey))
	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, sendStatusSearchingDriver, resp.Status)
	assert.Equal(t, PaymentMethodWallet, resp.PaymentMethod)
	assert.Equal(t, 2, resp.StopsCount)
	assert.Len(t, resp.FareBreakdownPerStop, 2)
	assert.True(t, resp.TotalFare.IsPositive())
	assert.True(t, resp.EscrowAmount.Equal(resp.TotalFare))
	assert.True(t, resp.TotalDistanceKm.IsPositive())
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_Success_Cash(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "cso-ok-cash"

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.Zero), nil)
	repo.On("InsertSendOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderStops", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupSendDB(mDB, idemKey, fCustID)
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodCash, idemKey))
	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, sendStatusSearchingDriver, resp.Status)
	assert.Equal(t, PaymentMethodCash, resp.PaymentMethod)
	assert.True(t, resp.EscrowAmount.IsZero())
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_IdempotentRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cached := CreateSendOrderResponse{ID: fOrderID, Status: sendStatusSearchingDriver, TotalFare: decimal.NewFromInt(48000)}
	cachedJSON, _ := json.Marshal(cached)
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: cachedJSON})
	require.NoError(t, mr.Set(redisKey(fCustID, "k"), string(redisVal)))

	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	resp, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.NoError(t, err)
	assert.Equal(t, sendStatusSearchingDriver, resp.Status)
}

func TestCreateSendOrder_RedisInvalidCached(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: json.RawMessage(`"bad"`)})
	require.NoError(t, mr.Set(redisKey(fCustID, "k"), string(redisVal)))

	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	_, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrInvalidCachedResponse)
}

func TestCreateSendOrder_IdempotencyCompleted(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	body := []byte(`{"id":"` + fOrderID.String() + `","status":"SEARCHING_DRIVER"}`)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(redisCompleted, body, nil))

	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.NoError(t, err)
	assert.Equal(t, sendStatusSearchingDriver, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateSendOrder_IdempotencyInProgress(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	future := time.Now().Add(5 * time.Minute)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", &future))

	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fSendCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fSendCustWallet(decimal.NewFromInt(100000)), nil)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateSendOrder(context.Background(), fSendReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- GetSendOrder ----

func TestGetSendOrder_Errors(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(nil, ErrSendOrderNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.GetSendOrder(context.Background(), fOrderID, fCustID)
	assert.ErrorIs(t, err, ErrSendOrderNotFound)

	repo2 := new(mockRepo)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo2.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	svc2 := NewService(repo2, nil, nil, new(mockLedger))
	_, err = svc2.GetSendOrder(context.Background(), fOrderID, uuid.New())
	assert.ErrorIs(t, err, ErrNotAllowed)

	repo3 := new(mockRepo)
	repo3.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo3.On("GetSendOrderStops", mock.Anything, fOrderID).Return(nil, pgx.ErrNoRows)
	svc3 := NewService(repo3, nil, nil, new(mockLedger))
	_, err = svc3.GetSendOrder(context.Background(), fOrderID, fCustID)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestGetSendOrder_Success(t *testing.T) {
	repo := new(mockRepo)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, &fDriverID)
	stops := []*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusPending)}
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStops", mock.Anything, fOrderID).Return(stops, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	detail, err := svc.GetSendOrder(context.Background(), fOrderID, fDriverID)
	assert.NoError(t, err)
	require.NotNil(t, detail)
	assert.Len(t, detail.Stops, 1)
}

// ---- AcceptSendOrder ----

func fAcceptBaseMocks(repo *mockRepo, mDB pgxmock.PgxPoolIface) {
	repo.On("GetDriver", mock.Anything, fDriverID).Return(fSendDriver(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fSendDriverWallet(decimal.NewFromInt(100000)), nil)
	repo.On("GetDriverActiveSendOrdersCount", mock.Anything, fDriverID).Return(0, nil)
	_ = mDB
}

func TestAcceptSendOrder_Validation(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID})
	assert.ErrorIs(t, err, ErrIdempotencyKeyRequired)
}

func TestAcceptSendOrder_DriverErrors(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetDriver", mock.Anything, fDriverID).Return(nil, ErrUserNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrUserNotFound)

	repo2 := new(mockRepo)
	d := fSendDriver()
	d.UserType = userTypeCustomer
	repo2.On("GetDriver", mock.Anything, fDriverID).Return(d, nil)
	svc2 := NewService(repo2, nil, nil, new(mockLedger))
	_, err = svc2.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrNotDriver)

	repo3 := new(mockRepo)
	d3 := fSendDriver()
	d3.Status = "SUSPENDED"
	repo3.On("GetDriver", mock.Anything, fDriverID).Return(d3, nil)
	svc3 := NewService(repo3, nil, nil, new(mockLedger))
	_, err = svc3.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrDriverInactive)

	repo4 := new(mockRepo)
	d4 := fSendDriver()
	d4.WorkingStatus = workingStatusBusy
	repo4.On("GetDriver", mock.Anything, fDriverID).Return(d4, nil)
	svc4 := NewService(repo4, nil, nil, new(mockLedger))
	_, err = svc4.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrDriverBusy)

	repo5 := new(mockRepo)
	repo5.On("GetDriver", mock.Anything, fDriverID).Return(fSendDriver(), nil)
	repo5.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(nil, ErrWalletNotFound)
	svc5 := NewService(repo5, nil, nil, new(mockLedger))
	_, err = svc5.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrWalletNotFound)

	repo6 := new(mockRepo)
	w := fSendDriverWallet(decimal.Zero)
	w.Status = "SUSPENDED"
	repo6.On("GetDriver", mock.Anything, fDriverID).Return(fSendDriver(), nil)
	repo6.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(w, nil)
	svc6 := NewService(repo6, nil, nil, new(mockLedger))
	_, err = svc6.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrWalletInactive)

	repo7 := new(mockRepo)
	d7 := fSendDriver()
	d7.MinBalanceThreshold = decimal.NewFromInt(50000)
	repo7.On("GetDriver", mock.Anything, fDriverID).Return(d7, nil)
	repo7.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.NewFromInt(100)), nil)
	svc7 := NewService(repo7, nil, nil, new(mockLedger))
	_, err = svc7.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrInsufficientDriverBalance)

	repo8 := new(mockRepo)
	repo8.On("GetDriver", mock.Anything, fDriverID).Return(fSendDriver(), nil)
	repo8.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.NewFromInt(100000)), nil)
	repo8.On("GetDriverActiveSendOrdersCount", mock.Anything, fDriverID).Return(0, pgx.ErrNoRows)
	svc8 := NewService(repo8, nil, nil, new(mockLedger))
	_, err = svc8.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	repo9 := new(mockRepo)
	repo9.On("GetDriver", mock.Anything, fDriverID).Return(fSendDriver(), nil)
	repo9.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.NewFromInt(100000)), nil)
	repo9.On("GetDriverActiveSendOrdersCount", mock.Anything, fDriverID).Return(driverMaxActiveOrders, nil)
	svc9 := NewService(repo9, nil, nil, new(mockLedger))
	_, err = svc9.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrDriverCapacityExceeded)
}

func TestAcceptSendOrder_LockDriverTimeout(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).
		Return(&pgconn.PgError{Code: "55P03"})

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrLockTimeout)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_LockDriverPlainError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(errors.New("boom"))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.Error(t, err, "boom")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_LockOrderTimeout(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(nil, &pgconn.PgError{Code: "55P03"})

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrLockTimeout)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_OrderNotFoundThenMissing(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).
		Return(nil, ErrSendOrderNotFound)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(fSendOrder(sendStatusDriverAssigned, PaymentMethodCash, &fDriverID), nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrOrderNotSearching)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_OrderNotFoundGetError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).
		Return(nil, ErrSendOrderNotFound)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(nil, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_LockOrderPlainError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(nil, errors.New("boom"))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.Error(t, err, "boom")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_AssignNotOK(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(false, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrOrderNotSearching)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_AssignError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(false, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_WorkingStatusError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusBusy).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_EventError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusBusy).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_CommitError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusBusy).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit().WillReturnError(pgx.ErrTxClosed)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_StopsAfterCommitError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusBusy).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()
	repo.On("GetSendOrderStops", mock.Anything, fOrderID).Return(nil, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_CacheError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusBusy).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()
	repo.On("GetSendOrderStops", mock.Anything, fOrderID).Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusPending)}, nil)
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs("k", fDriverID, pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_Success(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	fAcceptBaseMocks(repo, mDB)
	setupSendDB(mDB, "k", fDriverID)
	repo.On("LockDriverUserForAccept", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("LockSendOrderForAccept", mock.Anything, mock.Anything, fOrderID).Return(fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil), nil)
	repo.On("AssignDriverToSendOrder", mock.Anything, mock.Anything, fOrderID, fDriverID).Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusBusy).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()
	repo.On("GetSendOrderStops", mock.Anything, fOrderID).Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusPending)}, nil)
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs("k", fDriverID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, sendStatusDriverAssigned, resp.Status)
	assert.Len(t, resp.Stops, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestAcceptSendOrder_IdempotentRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	cached := AcceptSendOrderResponse{ID: fOrderID, Status: sendStatusDriverAssigned}
	cachedJSON, _ := json.Marshal(cached)
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: cachedJSON})
	require.NoError(t, mr.Set(redisKey(fDriverID, "k"), string(redisVal)))

	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	resp, err := svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.NoError(t, err)
	assert.Equal(t, sendStatusDriverAssigned, resp.Status)
}

func TestAcceptSendOrder_IdempotencyInProgress(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	future := time.Now().Add(5 * time.Minute)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", &future))

	repo := new(mockRepo)
	repo.On("GetDriver", mock.Anything, fDriverID).Return(fSendDriver(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.NewFromInt(100000)), nil)
	repo.On("GetDriverActiveSendOrdersCount", mock.Anything, fDriverID).Return(0, nil)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.AcceptSendOrder(context.Background(), AcceptSendOrderRequest{OrderID: fOrderID, DriverID: fDriverID, IdempotencyKey: "k"})
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- UpdateSendOrderStatus ----

func TestUpdateSendOrderStatus_InvalidTarget(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: "SETTLED"})
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestUpdateSendOrderStatus_Errors(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(nil, ErrSendOrderNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrSendOrderNotFound)

	repo2 := new(mockRepo)
	repo2.On("GetSendOrderByID", mock.Anything, fOrderID).Return(fSendOrder(sendStatusCancelled, PaymentMethodCash, nil), nil)
	svc2 := NewService(repo2, nil, nil, new(mockLedger))
	_, err = svc2.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrInvalidTransition)

	repo3 := new(mockRepo)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo3.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	svc3 := NewService(repo3, nil, nil, new(mockLedger))
	_, err = svc3.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: uuid.New(), Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrNotAllowed)

	repo4 := new(mockRepo)
	repo4.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	svc4 := NewService(repo4, nil, nil, new(mockLedger))
	_, err = svc4.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusPickedUp})
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestUpdateSendOrderStatus_BeginError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusCreated, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin().WillReturnError(pgx.ErrTxClosed)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_SetTimeoutError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusCreated, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_LockTimeout(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(nil, &pgconn.PgError{Code: "55P03"})

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrLockTimeout)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_LockPlainError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(nil, errors.New("boom"))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.Error(t, err, "boom")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_LockedStatusChanged(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	locked := fSendOrder(sendStatusDriverAssigned, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(locked, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelUpdateError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(false, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelNotOK(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(false, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelWalletNoSenderWallet(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil)
	order.SenderWalletID = nil
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(true, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelRefundLedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelDriverIdleError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusInTransit, sendStatusCancelled).
		Return(true, nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(pgx.ErrNoRows)
	_ = lgr

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelEventError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelCommitError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit().WillReturnError(pgx.ErrTxClosed)
	_ = lgr

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelSuccessSender(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusSearchingDriver, sendStatusCancelled).
		Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fCustID, Status: sendStatusCancelled})
	assert.NoError(t, err)
	assert.Equal(t, sendStatusCancelled, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_CancelSuccessDriver(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusInTransit, sendStatusCancelled).
		Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusCancelled, Reason: "emergency"})
	assert.NoError(t, err)
	assert.Equal(t, sendStatusCancelled, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_PlainTransition(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusDriverAssigned, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusDriverAssigned, sendStatusPickedUp).
		Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusPickedUp})
	assert.NoError(t, err)
	assert.Equal(t, sendStatusPickedUp, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_PlainTransitionUpdateNotOK(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusDriverAssigned, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStatus", mock.Anything, mock.Anything, fOrderID, sendStatusDriverAssigned, sendStatusPickedUp).
		Return(false, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusPickedUp})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverStopsError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).Return(nil, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverStopsNotCompleted(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusPending)}, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.ErrorIs(t, err, ErrStopsNotDelivered)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverMarkError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(false, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverMarkNotOK(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(false, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverEventError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverSettleError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(nil, ErrWalletNotFound)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStatus_DeliverSuccessCash(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	sendLockWallets(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.Zero))
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateSendOrderStatus(context.Background(), UpdateSendOrderStatusRequest{OrderID: fOrderID, UserID: fDriverID, Status: sendStatusDelivered})
	assert.NoError(t, err)
	assert.Equal(t, sendStatusSettled, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- UpdateSendOrderStop ----

func TestUpdateSendOrderStop_InvalidStatus(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "PENDING"})
	assert.ErrorIs(t, err, ErrInvalidStopStatus)
}

func TestUpdateSendOrderStop_Errors(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(nil, ErrSendOrderNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, ErrSendOrderNotFound)

	repo2 := new(mockRepo)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, nil)
	repo2.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	svc2 := NewService(repo2, nil, nil, new(mockLedger))
	_, err = svc2.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, ErrNotAllowed)

	repo3 := new(mockRepo)
	order3 := fSendOrder(sendStatusSearchingDriver, PaymentMethodCash, &fDriverID)
	repo3.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order3, nil)
	svc3 := NewService(repo3, nil, nil, new(mockLedger))
	_, err = svc3.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestUpdateSendOrderStop_LockTimeout(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(nil, &pgconn.PgError{Code: "55P03"})

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, ErrLockTimeout)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_LockedStatusChanged(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	locked := fSendOrder(sendStatusInTransit, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(locked, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_UpdateError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop1ID, stopStatusDelivered, mock.Anything).
		Return(false, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_StopNotFound(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop1ID, stopStatusDelivered, mock.Anything).
		Return(false, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, ErrStopNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_AllStopsError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop1ID, stopStatusDelivered, mock.Anything).
		Return(true, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).Return(nil, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "DELIVERED", DeliveryPhotoURL: "http://photo"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_SuccessNoAuto(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop1ID, stopStatusDelivered, mock.Anything).
		Return(true, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{
			fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered),
			fSendStop(fOrderID, fStop2ID, 2, stopStatusPending),
		}, nil)
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "DELIVERED"})
	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, stopStatusDelivered, resp.StopStatus)
	assert.False(t, resp.Settled)
	assert.Equal(t, sendStatusPickedUp, resp.OrderStatus)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_AutoDeliverMarkError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop1ID, stopStatusDelivered, mock.Anything).
		Return(true, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(false, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_AutoDeliverEventError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusPickedUp, PaymentMethodCash, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop1ID, stopStatusDelivered, mock.Anything).
		Return(true, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered)}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop1ID, UserID: fDriverID, Status: "COMPLETED"})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateSendOrderStop_AutoDeliverSuccess(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fSendOrder(sendStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo.On("GetSendOrderByID", mock.Anything, fOrderID).Return(order, nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockSendOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateSendOrderStopStatus", mock.Anything, mock.Anything, fStop2ID, stopStatusDelivered, mock.Anything).
		Return(true, nil)
	repo.On("GetSendOrderStopsByOrderID", mock.Anything, mock.Anything, fOrderID).
		Return([]*SendOrderStop{
			fSendStop(fOrderID, fStop1ID, 1, stopStatusDelivered),
			fSendStop(fOrderID, fStop2ID, 2, stopStatusDelivered),
		}, nil)
	repo.On("MarkSendOrderDelivered", mock.Anything, mock.Anything, fOrderID).Return(true, nil)
	repo.On("InsertSendOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fSendDriverWallet(decimal.Zero), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	sendLockWallets(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.Anything, mock.Anything, mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("UpdateDriverWorkingStatus", mock.Anything, mock.Anything, fDriverID, workingStatusIdle).Return(nil)
	repo.On("MarkSendOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateSendOrderStop(context.Background(), UpdateSendOrderStopRequest{OrderID: fOrderID, StopID: fStop2ID, UserID: fDriverID, Status: "DELIVERED"})
	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Settled)
	assert.Equal(t, sendStatusSettled, resp.OrderStatus)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- redis helpers ----

func Test_redisSet_RedisGet(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	svc.redisSet(context.Background(), fCustID, "w", redisCompleted, json.RawMessage(`{"a":1}`))
	resp, ok := svc.redisGetCachedResp(context.Background(), fCustID, "w")
	assert.True(t, ok)
	assert.JSONEq(t, `{"a":1}`, string(resp))
}

func Test_redisGetCachedResp_MissingKey(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	_, ok := svc.redisGetCachedResp(context.Background(), fCustID, "nope")
	assert.False(t, ok)
}

func Test_redisGetCachedResp_NilRedis(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, ok := svc.redisGetCachedResp(context.Background(), fCustID, "k")
	assert.False(t, ok)
}

func Test_idemAcquire_StaleProcessing(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	past := time.Now().Add(-6 * time.Minute)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", &past))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	res, err := svc.idemAcquire(context.Background(), fCustID, "k")
	assert.NoError(t, err)
	assert.True(t, res.proceed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_idemAcquire_UnknownStateProceeds(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow("WEIRD", "{}", nil))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	res, err := svc.idemAcquire(context.Background(), fCustID, "k")
	assert.NoError(t, err)
	assert.True(t, res.proceed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_idemAcquire_QueryError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs("k", fCustID).
		WillReturnError(&pgconn.PgError{Code: "23505"})
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	_, err = svc.idemAcquire(context.Background(), fCustID, "k")
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_cacheResponse_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs("k", fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	err = svc.cacheResponse(context.Background(), fCustID, "k", map[string]string{"x": "1"})
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_haversineKm(t *testing.T) {
	d := haversineKm(-6.2, 106.8, -6.2, 106.8)
	assert.True(t, d.LessThan(decimal.NewFromFloat(0.001)))
	d2 := haversineKm(-6.2, 106.8, -6.25, 106.8)
	assert.True(t, d2.GreaterThan(decimal.NewFromFloat(5)))
}
