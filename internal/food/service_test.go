package food

import (
	"context"
	"encoding/json"
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

func (m *mockRepo) GetMerchantUser(ctx context.Context, userID uuid.UUID) (*MerchantUser, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MerchantUser), args.Error(1)
}

func (m *mockRepo) GetMerchantByUserID(ctx context.Context, userID uuid.UUID) (*Merchant, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Merchant), args.Error(1)
}

func (m *mockRepo) GetMerchantByID(ctx context.Context, merchantID uuid.UUID) (*Merchant, error) {
	args := m.Called(ctx, merchantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Merchant), args.Error(1)
}

func (m *mockRepo) InsertMerchant(ctx context.Context, q Querier, mr *Merchant) error {
	args := m.Called(ctx, q, mr)
	return args.Error(0)
}

func (m *mockRepo) InsertMerchantWallet(ctx context.Context, q Querier, userID uuid.UUID) (*FoodWallet, error) {
	args := m.Called(ctx, q, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FoodWallet), args.Error(1)
}

func (m *mockRepo) UpdateMerchant(ctx context.Context, merchantID uuid.UUID,
	name, category *string, openingTime, closingTime *string, isOpen *bool, logoURL *string) (bool, error) {
	args := m.Called(ctx, merchantID, name, category, openingTime, closingTime, isOpen, logoURL)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) GetMenus(ctx context.Context, merchantID uuid.UUID) ([]*Menu, error) {
	args := m.Called(ctx, merchantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Menu), args.Error(1)
}

func (m *mockRepo) GetMenuByID(ctx context.Context, menuID uuid.UUID) (*Menu, error) {
	args := m.Called(ctx, menuID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Menu), args.Error(1)
}

func (m *mockRepo) InsertMenu(ctx context.Context, q Querier, menu *Menu) error {
	args := m.Called(ctx, q, menu)
	return args.Error(0)
}

func (m *mockRepo) UpdateMenu(ctx context.Context, menuID, merchantID uuid.UUID,
	name *string, description *string, sequenceOrder *int, isActive *bool) (bool, error) {
	args := m.Called(ctx, menuID, merchantID, name, description, sequenceOrder, isActive)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) DeleteMenu(ctx context.Context, menuID, merchantID uuid.UUID) (bool, error) {
	args := m.Called(ctx, menuID, merchantID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) GetItems(ctx context.Context, merchantID uuid.UUID) ([]*Item, error) {
	args := m.Called(ctx, merchantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Item), args.Error(1)
}

func (m *mockRepo) GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	args := m.Called(ctx, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Item), args.Error(1)
}

func (m *mockRepo) InsertItem(ctx context.Context, q Querier, it *Item) error {
	args := m.Called(ctx, q, it)
	return args.Error(0)
}

func (m *mockRepo) UpdateItem(ctx context.Context, itemID, merchantID uuid.UUID,
	name *string, description *string, price *decimal.Decimal, imageURL *string,
	stock *int, isAvailable *bool) (bool, error) {
	args := m.Called(ctx, itemID, merchantID, name, description, price, imageURL, stock, isAvailable)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) DeleteItem(ctx context.Context, itemID, merchantID uuid.UUID) (bool, error) {
	args := m.Called(ctx, itemID, merchantID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) GetCustomer(ctx context.Context, userID uuid.UUID) (*FoodCustomer, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FoodCustomer), args.Error(1)
}

func (m *mockRepo) GetWalletByUserAndType(ctx context.Context, userID uuid.UUID, walletType string) (*FoodWallet, error) {
	args := m.Called(ctx, userID, walletType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FoodWallet), args.Error(1)
}

func (m *mockRepo) SearchMerchants(ctx context.Context, category, search *string, lat, lng *float64) ([]*Merchant, error) {
	args := m.Called(ctx, category, search, lat, lng)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Merchant), args.Error(1)
}

func (m *mockRepo) GetAvailableItems(ctx context.Context, merchantID uuid.UUID) ([]*Item, error) {
	args := m.Called(ctx, merchantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Item), args.Error(1)
}

func (m *mockRepo) GetItemsByIDs(ctx context.Context, ids []uuid.UUID) ([]*Item, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Item), args.Error(1)
}

func (m *mockRepo) InsertFoodOrder(ctx context.Context, q Querier, o *FoodOrder) error {
	args := m.Called(ctx, q, o)
	return args.Error(0)
}

func (m *mockRepo) InsertFoodOrderItem(ctx context.Context, q Querier, it *FoodOrderItem) error {
	args := m.Called(ctx, q, it)
	return args.Error(0)
}

func (m *mockRepo) InsertFoodOrderEvent(ctx context.Context, q Querier, e FoodOrderEvent) error {
	args := m.Called(ctx, q, e)
	return args.Error(0)
}

func (m *mockRepo) GetFoodOrderByID(ctx context.Context, orderID uuid.UUID) (*FoodOrder, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FoodOrder), args.Error(1)
}

func (m *mockRepo) LockFoodOrder(ctx context.Context, q Querier, orderID uuid.UUID) (*FoodOrder, error) {
	args := m.Called(ctx, q, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FoodOrder), args.Error(1)
}

func (m *mockRepo) UpdateFoodOrderStatus(ctx context.Context, q Querier, orderID uuid.UUID,
	fromStatus, toStatus string, merchantStatus *string, isRefunded *bool) (bool, error) {
	args := m.Called(ctx, q, orderID, fromStatus, toStatus, merchantStatus, isRefunded)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) GetFoodOrderItems(ctx context.Context, orderID uuid.UUID) ([]*FoodOrderItem, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FoodOrderItem), args.Error(1)
}

func (m *mockRepo) GetFoodOrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*FoodOrder, error) {
	args := m.Called(ctx, customerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FoodOrder), args.Error(1)
}

func (m *mockRepo) CountFoodOrdersByCustomer(ctx context.Context, customerID uuid.UUID) (int, error) {
	args := m.Called(ctx, customerID)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) GetFoodOrdersByMerchant(ctx context.Context, merchantID uuid.UUID, status string, limit, offset int) ([]*FoodOrder, error) {
	args := m.Called(ctx, merchantID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FoodOrder), args.Error(1)
}

func (m *mockRepo) CountFoodOrdersByMerchant(ctx context.Context, merchantID uuid.UUID, status string) (int, error) {
	args := m.Called(ctx, merchantID, status)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) SystemWalletID(ctx context.Context, q Querier, walletType string) (uuid.UUID, error) {
	args := m.Called(ctx, q, walletType)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *mockRepo) MarkFoodOrderSettled(ctx context.Context, q Querier, orderID uuid.UUID) error {
	args := m.Called(ctx, q, orderID)
	return args.Error(0)
}

func (m *mockRepo) MarkDriverSuspended(ctx context.Context, q Querier, driverID uuid.UUID) error {
	args := m.Called(ctx, q, driverID)
	return args.Error(0)
}

func (m *mockRepo) ResetDriverIdle(ctx context.Context, q Querier, driverID uuid.UUID) error {
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
	fCustID      = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fCustBID     = uuid.MustParse("12121212-1212-1212-1212-121212121212")
	fMerchID     = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	fDriverID    = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	fCustWallet  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	fMerchWallet = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	fEscrowID    = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	fPlatformID  = uuid.MustParse("77777777-7777-7777-7777-777777777777")
	fDriverWID   = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	fOrderID     = uuid.MustParse("99999999-9999-9999-9999-999999999999")
	fItemID      = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
)

func fMerchantUser() *MerchantUser {
	return &MerchantUser{ID: fCustID, UserType: userTypeMerchant, Status: statusActive}
}

func fMerchant() *Merchant {
	return &Merchant{
		ID: fMerchID, UserID: fCustID, Name: "Warung", Category: "food",
		Latitude: decimal.NewFromFloat(-6.2), Longitude: decimal.NewFromFloat(106.8),
		Address: "Jl. A", IsOpen: true, Status: statusActive,
	}
}

func fCust() *FoodCustomer {
	return &FoodCustomer{ID: fCustID, UserType: userTypeCustomer, Status: statusActive, OverdueDebt: decimal.Zero}
}

func fCustWalletSvc(balance decimal.Decimal) *FoodWallet {
	return &FoodWallet{ID: fCustWallet, UserID: fCustID, Type: walletTypeCustomer, Balance: balance, Status: statusActive}
}

func fMerchantWalletDef() *FoodWallet {
	return &FoodWallet{ID: fMerchWallet, UserID: fCustID, Type: WalletTypeMerchant, Balance: decimal.Zero, Status: statusActive}
}

func fDriverWalletDef() *FoodWallet {
	return &FoodWallet{ID: fDriverWID, UserID: fDriverID, Type: walletTypeDriver, Balance: decimal.Zero, Status: statusActive}
}

func fItem() *Item {
	return &Item{ID: fItemID, MenuID: uuid.Nil, MerchantID: fMerchID, Name: "Nasi",
		Price: decimal.NewFromInt(50000), Stock: 100, IsAvailable: true}
}

func fValidOrderReq(paymentMethod, idemKey string) CreateFoodOrderRequest {
	return CreateFoodOrderRequest{
		UserID: fCustID, MerchantID: fMerchID, DeliveryAddress: "Jl. B",
		PaymentMethod: paymentMethod,
		Items:         []FoodOrderItemRequest{{ItemID: fItemID, Quantity: 2}},
		IdempotencyKey: idemKey,
	}
}

func fFoodOrder(status string, paymentMethod string, driver *uuid.UUID) *FoodOrder {
	return &FoodOrder{
		ID: fOrderID, CustomerID: fCustID, MerchantID: fMerchID, DriverID: driver,
		CustomerWalletID: &fCustWallet, MerchantWalletID: &fMerchWallet,
		ItemSubtotal: decimal.NewFromInt(100000), DeliveryFee: foodDeliveryFee,
		PlatformCommission: decimalPtr(decimal.NewFromInt(15000)), DriverEarning: decimalPtr(decimal.NewFromInt(18000)),
		TotalAmount: decimal.NewFromInt(120000), PaymentMethod: paymentMethod,
		Status: status, MerchantStatus: merchantStatusWaiting,
	}
}

func fCustBOrder(status string, paymentMethod string, driver *uuid.UUID) *FoodOrder {
	o := fFoodOrder(status, paymentMethod, driver)
	o.CustomerID = fCustBID
	return o
}

func setupFoodDB(mDB pgxmock.PgxPoolIface, idemKey string, owner uuid.UUID) {
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

// RegisterMerchant

func TestRegisterMerchant_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	req := RegisterMerchantRequest{
		UserID: fCustID, MerchantName: "Warung Baru", Category: "food",
		Latitude: -6.2, Longitude: 106.8, Address: "Jl. X",
	}
	repo.On("GetMerchantUser", mock.Anything, fCustID).Return(fMerchantUser(), nil)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(nil, ErrMerchantNotFound)
	repo.On("InsertMerchant", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertMerchantWallet", mock.Anything, mock.Anything, fCustID).Return(&FoodWallet{ID: uuid.New()}, nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.RegisterMerchant(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, statusPendingVerification, resp.Status)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRegisterMerchant_EmptyName(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.RegisterMerchant(context.Background(), RegisterMerchantRequest{})
	assert.ErrorIs(t, err, ErrEmptyName)
}

func TestRegisterMerchant_InvalidCoords(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.RegisterMerchant(context.Background(), RegisterMerchantRequest{MerchantName: "X", Latitude: -200})
	assert.ErrorIs(t, err, ErrInvalidCoordinates)
}

func TestRegisterMerchant_NotMerchant(t *testing.T) {
	repo := new(mockRepo)
	u := fMerchantUser()
	u.UserType = userTypeCustomer
	repo.On("GetMerchantUser", mock.Anything, fCustID).Return(u, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.RegisterMerchant(context.Background(), RegisterMerchantRequest{UserID: fCustID, MerchantName: "X", Latitude: -6.2, Longitude: 106.8})
	assert.ErrorIs(t, err, ErrNotMerchant)
}

func TestRegisterMerchant_MerchantInactive(t *testing.T) {
	repo := new(mockRepo)
	u := fMerchantUser()
	u.Status = "SUSPENDED"
	repo.On("GetMerchantUser", mock.Anything, fCustID).Return(u, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.RegisterMerchant(context.Background(), RegisterMerchantRequest{UserID: fCustID, MerchantName: "X", Latitude: -6.2, Longitude: 106.8})
	assert.ErrorIs(t, err, ErrMerchantInactive)
}

func TestRegisterMerchant_AlreadyExists(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantUser", mock.Anything, fCustID).Return(fMerchantUser(), nil)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(fMerchant(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.RegisterMerchant(context.Background(), RegisterMerchantRequest{UserID: fCustID, MerchantName: "X", Latitude: -6.2, Longitude: 106.8})
	assert.ErrorIs(t, err, ErrMerchantAlreadyExists)
}

// UpdateMerchant & catalog

func TestUpdateMerchant_Success(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("UpdateMerchant", mock.Anything, fMerchID, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	svc := NewService(repo, nil, nil, new(mockLedger))
	m, err := svc.UpdateMerchant(context.Background(), UpdateMerchantRequest{UserID: fCustID, MerchantID: fMerchID})
	assert.NoError(t, err)
	assert.Equal(t, fMerchID, m.ID)
	repo.AssertExpectations(t)
}

func TestUpdateMerchant_NotOwner(t *testing.T) {
	repo := new(mockRepo)
	m := fMerchant()
	m.UserID = uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(m, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateMerchant(context.Background(), UpdateMerchantRequest{UserID: fCustID, MerchantID: fMerchID})
	assert.ErrorIs(t, err, ErrNotMerchantOwner)
}

func TestUpdateMerchant_RepoError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("UpdateMerchant", mock.Anything, fMerchID, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(false, pgx.ErrNoRows)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateMerchant(context.Background(), UpdateMerchantRequest{UserID: fCustID, MerchantID: fMerchID})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestCreateMenu_Success(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("InsertMenu", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	m, err := svc.CreateMenu(context.Background(), CreateMenuRequest{UserID: fCustID, MerchantID: fMerchID, Name: "Menu"})
	assert.NoError(t, err)
	assert.Equal(t, "Menu", m.Name)
	repo.AssertExpectations(t)
}

func TestCreateMenu_EmptyName(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.CreateMenu(context.Background(), CreateMenuRequest{UserID: fCustID, MerchantID: fMerchID})
	assert.ErrorIs(t, err, ErrEmptyName)
}

func TestCreateMenu_NotOwner(t *testing.T) {
	repo := new(mockRepo)
	m := fMerchant()
	m.UserID = uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(m, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateMenu(context.Background(), CreateMenuRequest{UserID: fCustID, MerchantID: fMerchID, Name: "Menu"})
	assert.ErrorIs(t, err, ErrNotMerchantOwner)
}

func TestUpdateMenu_Success(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("UpdateMenu", mock.Anything, mock.Anything, fMerchID, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("GetMenuByID", mock.Anything, mock.Anything).Return(&Menu{ID: uuid.New(), MerchantID: fMerchID, Name: "M"}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	m, err := svc.UpdateMenu(context.Background(), UpdateMenuRequest{UserID: fCustID, MerchantID: fMerchID, MenuID: uuid.New()})
	assert.NoError(t, err)
	assert.Equal(t, "M", m.Name)
	repo.AssertExpectations(t)
}

func TestDeleteMenu_Success(t *testing.T) {
	repo := new(mockRepo)
	menuID := uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("DeleteMenu", mock.Anything, menuID, fMerchID).Return(true, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.DeleteMenu(context.Background(), fMerchID, menuID, fCustID)
	assert.NoError(t, err)
}

func TestDeleteMenu_NotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("DeleteMenu", mock.Anything, uuid.Nil, fMerchID).Return(false, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.DeleteMenu(context.Background(), fMerchID, uuid.Nil, fCustID)
	assert.ErrorIs(t, err, ErrMenuNotFound)
}

func TestGetMenus_Owner(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetMenus", mock.Anything, fMerchID).Return([]*Menu{{ID: uuid.New(), MerchantID: fMerchID, Name: "M"}}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	menus, err := svc.GetMenus(context.Background(), fMerchID, fCustID)
	assert.NoError(t, err)
	assert.Len(t, menus, 1)
	repo.AssertExpectations(t)
}

func TestGetMenus_NotOwner(t *testing.T) {
	repo := new(mockRepo)
	m := fMerchant()
	m.UserID = uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(m, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.GetMenus(context.Background(), fMerchID, fCustID)
	assert.ErrorIs(t, err, ErrNotMerchantOwner)
}

func TestCreateItem_Success(t *testing.T) {
	repo := new(mockRepo)
	menuID := uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetMenuByID", mock.Anything, menuID).Return(&Menu{ID: menuID, MerchantID: fMerchID}, nil)
	repo.On("InsertItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	it, err := svc.CreateItem(context.Background(), CreateItemRequest{
		UserID: fCustID, MerchantID: fMerchID, MenuID: menuID, Name: "Nasi", Price: decimal.NewFromInt(10000),
	})
	assert.NoError(t, err)
	assert.Equal(t, 999, it.Stock)
	repo.AssertExpectations(t)
}

func TestCreateItem_InvalidPrice(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.CreateItem(context.Background(), CreateItemRequest{Name: "X", Price: decimal.Zero})
	assert.ErrorIs(t, err, ErrInvalidPrice)
}

func TestCreateItem_InvalidMenu(t *testing.T) {
	repo := new(mockRepo)
	menuID := uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetMenuByID", mock.Anything, menuID).Return(&Menu{ID: menuID, MerchantID: uuid.New()}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateItem(context.Background(), CreateItemRequest{
		UserID: fCustID, MerchantID: fMerchID, MenuID: menuID, Name: "Nasi", Price: decimal.NewFromInt(10000),
	})
	assert.ErrorIs(t, err, ErrInvalidMenu)
}

func TestUpdateItem_Success(t *testing.T) {
	repo := new(mockRepo)
	itemID := uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{ID: itemID, MerchantID: fMerchID}, nil)
	repo.On("UpdateItem", mock.Anything, itemID, fMerchID, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	it, err := svc.UpdateItem(context.Background(), UpdateItemRequest{UserID: fCustID, MerchantID: fMerchID, ItemID: itemID})
	assert.NoError(t, err)
	assert.Equal(t, itemID, it.ID)
	repo.AssertExpectations(t)
}

func TestDeleteItem_Success(t *testing.T) {
	repo := new(mockRepo)
	itemID := uuid.New()
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("DeleteItem", mock.Anything, itemID, fMerchID).Return(true, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.DeleteItem(context.Background(), fMerchID, itemID, fCustID)
	assert.NoError(t, err)
}

func TestGetItems_Owner(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetItems", mock.Anything, fMerchID).Return([]*Item{fItem()}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	items, err := svc.GetItems(context.Background(), fMerchID, fCustID)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	repo.AssertExpectations(t)
}

func TestGetMerchantItems_Owner(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetItems", mock.Anything, fMerchID).Return([]*Item{fItem()}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	items, err := svc.GetMerchantItems(context.Background(), fMerchID, fCustID)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	repo.AssertExpectations(t)
}

func TestGetMerchantItems_PublicActive(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetAvailableItems", mock.Anything, fMerchID).Return([]*Item{fItem()}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	items, err := svc.GetMerchantItems(context.Background(), fMerchID, uuid.Nil)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	repo.AssertExpectations(t)
}

func TestGetMerchantItems_PublicInactive(t *testing.T) {
	repo := new(mockRepo)
	m := fMerchant()
	m.Status = "PENDING_VERIFICATION"
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(m, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.GetMerchantItems(context.Background(), fMerchID, uuid.Nil)
	assert.ErrorIs(t, err, ErrMerchantInactive)
}

func TestSearchMerchants_Success(t *testing.T) {
	repo := new(mockRepo)
	repo.On("SearchMerchants", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*Merchant{fMerchant()}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	res, err := svc.SearchMerchants(context.Background(), "", "", -6.2, 106.8)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.NotNil(t, res[0].DistanceKm)
	repo.AssertExpectations(t)
}

func TestSearchMerchants_NoDistance(t *testing.T) {
	repo := new(mockRepo)
	repo.On("SearchMerchants", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*Merchant{fMerchant()}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	res, err := svc.SearchMerchants(context.Background(), "", "", 200, 0)
	assert.NoError(t, err)
	assert.Nil(t, res[0].DistanceKm)
}

// CreateFoodOrder — WALLET & CASH

func TestCreateFoodOrder_Success_Wallet(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "food-wallet-1"
	req := fValidOrderReq(PaymentMethodWallet, idemKey)

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.NewFromInt(200000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupFoodDB(mDB, idemKey, fCustID)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200000)))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.CreateFoodOrder(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, foodStatusConfirmed, resp.Status)
	assert.True(t, resp.ItemSubtotal.Equal(decimal.NewFromInt(100000)))
	assert.True(t, resp.TotalAmount.GreaterThan(resp.ItemSubtotal), "termasuk delivery fee")
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_Success_Cash(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	idemKey := "food-cash-1"
	req := fValidOrderReq(PaymentMethodCash, idemKey)

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.Zero), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupFoodDB(mDB, idemKey, fCustID)
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.CreateFoodOrder(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, foodStatusCreated, resp.Status)
	assert.True(t, resp.TotalAmount.IsPositive())
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_ValidationErrors(t *testing.T) {
	repo := new(mockRepo)
	svc := NewService(repo, nil, nil, new(mockLedger))

	_, err := svc.CreateFoodOrder(context.Background(), CreateFoodOrderRequest{})
	assert.ErrorIs(t, err, ErrIdempotencyKeyRequired)

	_, err = svc.CreateFoodOrder(context.Background(), CreateFoodOrderRequest{IdempotencyKey: "k", PaymentMethod: "QRIS"})
	assert.ErrorIs(t, err, ErrInvalidPaymentMethod)

	_, err = svc.CreateFoodOrder(context.Background(), CreateFoodOrderRequest{IdempotencyKey: "k", PaymentMethod: PaymentMethodWallet})
	assert.ErrorIs(t, err, ErrInvalidDeliveryAddress)

	_, err = svc.CreateFoodOrder(context.Background(), CreateFoodOrderRequest{IdempotencyKey: "k", PaymentMethod: PaymentMethodWallet, DeliveryAddress: "a"})
	assert.ErrorIs(t, err, ErrEmptyItems)

	_, err = svc.CreateFoodOrder(context.Background(), CreateFoodOrderRequest{
		IdempotencyKey: "k", PaymentMethod: PaymentMethodWallet, DeliveryAddress: "a",
		Items: []FoodOrderItemRequest{{ItemID: fItemID, Quantity: 0}},
	})
	assert.ErrorIs(t, err, ErrInvalidItem)
}

func TestCreateFoodOrder_OverdueDebt(t *testing.T) {
	repo := new(mockRepo)
	c := fCust()
	c.OverdueDebt = decimal.NewFromInt(50000)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(c, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrOverdueDebt)
}

func TestCreateFoodOrder_CustomerInactive(t *testing.T) {
	repo := new(mockRepo)
	c := fCust()
	c.Status = "SUSPENDED"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(c, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrCustomerInactive)
}

func TestCreateFoodOrder_NotCustomer(t *testing.T) {
	repo := new(mockRepo)
	c := fCust()
	c.UserType = userTypeMerchant
	repo.On("GetCustomer", mock.Anything, fCustID).Return(c, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrNotCustomer)
}

func TestCreateFoodOrder_WalletInactive(t *testing.T) {
	repo := new(mockRepo)
	w := fCustWalletSvc(decimal.Zero)
	w.Status = "SUSPENDED"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(w, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrWalletInactive)
}

func TestCreateFoodOrder_MerchantInactive(t *testing.T) {
	repo := new(mockRepo)
	m := fMerchant()
	m.Status = "PENDING_VERIFICATION"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(m, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrMerchantInactive)
}

func TestCreateFoodOrder_MerchantWalletInactive(t *testing.T) {
	repo := new(mockRepo)
	mw := fMerchantWalletDef()
	mw.Status = "SUSPENDED"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).Return(mw, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrMerchantWalletNotFound)
}

func TestCreateFoodOrder_InvalidItem(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrInvalidItem)
}

func TestCreateFoodOrder_InsufficientStock(t *testing.T) {
	repo := new(mockRepo)
	it := fItem()
	it.Stock = 1
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{it}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrInsufficientStock)
}

func TestCreateFoodOrder_InsufficientBalance(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).Return(fCustWalletSvc(decimal.NewFromInt(100)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)

	setupFoodDB(mDB, "k", fCustID)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100)))

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrInsufficientBalance)
	lgr.AssertNotCalled(t, "CreateLedgerEntries")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_IdempotentRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cached := CreateFoodOrderResponse{Status: foodStatusConfirmed, TotalAmount: decimal.NewFromInt(120000)}
	cachedJSON, _ := json.Marshal(cached)
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: cachedJSON})
	assert.NoError(t, mr.Set(redisKey(fCustID, "k"), string(redisVal)))

	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	resp, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.NoError(t, err)
	assert.Equal(t, foodStatusConfirmed, resp.Status)
}

// UpdateFoodOrderStatus — merchant / driver / customer / settlement

func TestUpdateFoodOrderStatus_MerchantConfirm(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	// order punya customer B; merchant dimiliki fCustID → aktor merchant.
	order := fCustBOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusCreated, foodStatusConfirmed, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusConfirmed,
	})
	assert.NoError(t, err)
	assert.Equal(t, foodStatusConfirmed, resp.Status)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_DriverPickedUp(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	d := fDriverID
	order := fFoodOrder(foodStatusReadyForPickup, PaymentMethodCash, &d)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusReadyForPickup, foodStatusPickedUp, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fDriverID, Status: foodStatusPickedUp,
	})
	assert.NoError(t, err)
	assert.Equal(t, foodStatusPickedUp, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_CustomerCancelWallet(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	order := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusConfirmed, foodStatusCancelled, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.NoError(t, err)
	assert.Equal(t, foodStatusCancelled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_Deliver_SettlementWallet(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	d := fDriverID
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &d)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusInTransit, foodStatusDelivered, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkFoodOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 4"))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fDriverID, Status: foodStatusDelivered,
	})
	assert.NoError(t, err)
	assert.Equal(t, foodStatusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_Deliver_SettlementCash_Suspended(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	d := fDriverID
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &d)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusInTransit, foodStatusDelivered, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("MarkDriverSuspended", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkFoodOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-60000)))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fDriverID, Status: foodStatusDelivered,
	})
	assert.NoError(t, err)
	assert.Equal(t, foodStatusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_Deliver_SettlementCash_Reset(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	d := fDriverID
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &d)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusInTransit, foodStatusDelivered, mock.Anything, mock.Anything).Return(true, nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).Return(fPlatformID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkFoodOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	mDB.ExpectQuery("SELECT balance FROM wallets").WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(0)))
	mDB.ExpectCommit()

	svc := NewService(repo, mDB, nil, lgr)
	resp, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fDriverID, Status: foodStatusDelivered,
	})
	assert.NoError(t, err)
	assert.Equal(t, foodStatusSettled, resp.Status)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_InvalidStatus(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{Status: "BOGUS"})
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestUpdateFoodOrderStatus_NotAllowed(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: uuid.New(), Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrNotAllowed)
}

func TestUpdateFoodOrderStatus_InvalidTransition(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID, foodStatusCreated, foodStatusCancelled, mock.Anything, mock.Anything).Return(false, nil)
	mDB, _ := pgxmock.NewPool()
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestUpdateFoodOrderStatus_LockTimeout(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(nil, &pgconn.PgError{Code: "55P03"})
	mDB, _ := pgxmock.NewPool()
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrLockTimeout)
}

// GetFoodOrder / Items / History

func TestGetFoodOrder_AsCustomer(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	got, err := svc.GetFoodOrder(context.Background(), fOrderID, fCustID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, got.ID)
}

func TestGetFoodOrder_AsDriver(t *testing.T) {
	repo := new(mockRepo)
	d := fDriverID
	order := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, &d)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	got, err := svc.GetFoodOrder(context.Background(), fOrderID, fDriverID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, got.ID)
}

func TestGetFoodOrder_NotAllowed(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.GetFoodOrder(context.Background(), fOrderID, uuid.New())
	assert.ErrorIs(t, err, ErrNotAllowed)
}

func TestGetFoodOrderItems(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetFoodOrderItems", mock.Anything, fOrderID).Return([]*FoodOrderItem{}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	items, err := svc.GetFoodOrderItems(context.Background(), fOrderID)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestGetFoodOrderHistory(t *testing.T) {
	repo := new(mockRepo)
	repo.On("CountFoodOrdersByCustomer", mock.Anything, fCustID).Return(2, nil)
	repo.On("GetFoodOrdersByCustomer", mock.Anything, fCustID, 20, 0).Return([]*FoodOrder{fFoodOrder(foodStatusSettled, PaymentMethodWallet, nil)}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	orders, total, err := svc.GetFoodOrderHistory(context.Background(), fCustID, 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, orders, 1)
	repo.AssertExpectations(t)
}

func TestGetFoodOrderHistory_Paging(t *testing.T) {
	repo := new(mockRepo)
	repo.On("CountFoodOrdersByCustomer", mock.Anything, fCustID).Return(1, nil)
	repo.On("GetFoodOrdersByCustomer", mock.Anything, fCustID, 20, 40).Return([]*FoodOrder{}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	orders, total, err := svc.GetFoodOrderHistory(context.Background(), fCustID, 3, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Empty(t, orders)
}

func TestGetMerchantOrders_OwnerOK(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(fMerchant(), nil)
	repo.On("CountFoodOrdersByMerchant", mock.Anything, fMerchID, "WAITING").Return(1, nil)
	repo.On("GetFoodOrdersByMerchant", mock.Anything, fMerchID, "WAITING", 20, 0).
		Return([]*FoodOrder{fFoodOrder(foodStatusCreated, PaymentMethodWallet, nil)}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	orders, total, err := svc.GetMerchantOrders(context.Background(), fCustID, fMerchID, "WAITING", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, orders, 1)
	repo.AssertExpectations(t)
}

func TestGetMerchantOrders_NotOwner(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).
		Return(&Merchant{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), UserID: fCustID}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, _, err := svc.GetMerchantOrders(context.Background(), fCustID, fMerchID, "", 1, 20)
	assert.ErrorIs(t, err, ErrNotMerchantOwner)
}

func TestGetMerchantOrders_MerchantNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(nil, ErrMerchantNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, _, err := svc.GetMerchantOrders(context.Background(), fCustID, fMerchID, "", 1, 20)
	assert.ErrorIs(t, err, ErrMerchantNotFound)
}

func TestGetMerchantOrders_Empty(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(fMerchant(), nil)
	repo.On("CountFoodOrdersByMerchant", mock.Anything, fMerchID, "").Return(0, nil)
	repo.On("GetFoodOrdersByMerchant", mock.Anything, fMerchID, "", 20, 0).Return([]*FoodOrder{}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	orders, total, err := svc.GetMerchantOrders(context.Background(), fCustID, fMerchID, "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, orders)
}

func TestGetMerchantOrders_Paging(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(fMerchant(), nil)
	repo.On("CountFoodOrdersByMerchant", mock.Anything, fMerchID, "DELIVERED").Return(1, nil)
	repo.On("GetFoodOrdersByMerchant", mock.Anything, fMerchID, "DELIVERED", 20, 40).Return([]*FoodOrder{}, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	orders, total, err := svc.GetMerchantOrders(context.Background(), fCustID, fMerchID, "DELIVERED", 3, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Empty(t, orders)
}

// helper coverage

func TestFood_Helpers(t *testing.T) {
	assert.True(t, validLatLng(-6.2, 106.8))
	assert.False(t, validLatLng(91, 0))

	assert.True(t, validFoodStatusTarget(foodStatusConfirmed))
	assert.False(t, validFoodStatusTarget("BOGUS"))

	p := decimal.NewFromInt(1)
	got := decimalPtr(p)
	assert.Equal(t, p, *got)

	assert.Nil(t, strPtrOrNil(""))
	assert.NotNil(t, strPtrOrNil("x"))

	s := strPtr("a")
	assert.NotNil(t, s)

	d := haversineKm(-6.2, 106.8, -6.26, 106.8)
	assert.True(t, d.IsPositive())
}

func TestValidateFoodTransition(t *testing.T) {
	o := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)

	assert.Error(t, validateFoodTransition(o, actorKindCustomer, foodStatusPreparing))
	assert.NoError(t, validateFoodTransition(o, actorKindCustomer, foodStatusCancelled))

	o.Status = foodStatusConfirmed
	assert.NoError(t, validateFoodTransition(o, actorKindMerchant, foodStatusPreparing))
	assert.Error(t, validateFoodTransition(o, actorKindMerchant, foodStatusPickedUp))

	o.Status = foodStatusReadyForPickup
	assert.NoError(t, validateFoodTransition(o, actorKindDriver, foodStatusPickedUp))
	assert.NoError(t, validateFoodTransition(o, actorKindDriver, foodStatusCancelled))
	o.Status = foodStatusPickedUp
	assert.NoError(t, validateFoodTransition(o, actorKindDriver, foodStatusInTransit))
	o.Status = foodStatusInTransit
	assert.NoError(t, validateFoodTransition(o, actorKindDriver, foodStatusDelivered))
	assert.Error(t, validateFoodTransition(o, actorKindDriver, foodStatusConfirmed))

	assert.ErrorIs(t, validateFoodTransition(o, 99, foodStatusCancelled), ErrNotAllowed)
}

func TestGetMerchant_And_ForUser(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	m, err := svc.GetMerchant(context.Background(), fMerchID)
	assert.NoError(t, err)
	assert.Equal(t, fMerchID, m.ID)

	repo.On("GetMerchantByUserID", mock.Anything, fCustID).Return(fMerchant(), nil)
	m2, err := svc.GetMerchantForUser(context.Background(), fCustID)
	assert.NoError(t, err)
	assert.Equal(t, fMerchID, m2.ID)
}

func Test_idemAcquire_Completed(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	body := []byte(`{"ok":true}`)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(redisCompleted, body, nil))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	res, err := svc.idemAcquire(context.Background(), fCustID, "k")
	assert.NoError(t, err)
	assert.False(t, res.proceed)
	assert.JSONEq(t, `{"ok":true}`, string(res.cached))
}

func Test_idemAcquire_ProcessingFresh(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	future := time.Now().Add(5 * time.Minute)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", &future))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	_, err = svc.idemAcquire(context.Background(), fCustID, "k")
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
}

func Test_idemAcquire_ProcessingStale(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	past := time.Now().Add(-5 * time.Minute)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", &past))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	res, err := svc.idemAcquire(context.Background(), fCustID, "k")
	assert.NoError(t, err)
	assert.True(t, res.proceed)
}

func Test_idemAcquire_InsertConflict(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs("k", fCustID).
		WillReturnError(&pgconn.PgError{Code: "23505"})
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	_, err = svc.idemAcquire(context.Background(), fCustID, "k")
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
}

func Test_redisGetCachedResp_NilRedis(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	resp, ok := svc.redisGetCachedResp(context.Background(), fCustID, "k")
	assert.False(t, ok)
	assert.Nil(t, resp)
}

func Test_redisGetCachedResp_Completed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: json.RawMessage(`{"x":1}`)})
	assert.NoError(t, mr.Set(redisKey(fCustID, "k"), string(redisVal)))

	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	resp, ok := svc.redisGetCachedResp(context.Background(), fCustID, "k")
	assert.True(t, ok)
	assert.JSONEq(t, `{"x":1}`, string(resp))
}

func Test_redisSet_NilRedis(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	svc.redisSet(context.Background(), fCustID, "k", redisCompleted, json.RawMessage(`{}`))
}

func Test_refundFoodEscrow_NoWallet(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	o.CustomerWalletID = nil
	err := svc.refundFoodEscrow(context.Background(), nil, o)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}
