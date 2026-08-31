package food

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFoodRepo(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewRepository(mDB), mDB
}

func merchantRowValues(id, userID uuid.UUID) []any {
	name := "Warung"
	cat := "food"
	return []any{
		id, userID, name, nil, cat,
		decimal.NewFromFloat(-6.2), decimal.NewFromFloat(106.8), "Jl. A", nil,
		decimal.Zero, 0, 0,
		nil, nil, true, "ACTIVE", nil, nil,
		time.Now(), time.Now(),
	}
}

const nMerchantCols = 20
const nFoodOrderCols = 30

var foodOrderCols = []string{
	"id", "customer_id", "merchant_id", "driver_id",
	"customer_wallet_id", "merchant_wallet_id", "driver_wallet_id",
	"delivery_address", "delivery_lat", "delivery_lng", "special_instructions",
	"item_subtotal", "delivery_fee", "platform_commission", "driver_earning",
	"discount_amount", "voucher_id", "payment_method", "cutlery_included", "total_amount",
	"status", "merchant_status", "merchant_notes",
	"created_at", "confirmed_at", "pickup_at", "delivered_at", "settled_at",
	"is_settled", "is_refunded",
}

var foodItemCols = []string{"id", "order_id", "item_id", "item_name", "item_price", "quantity", "subtotal",
	"options", "options_total", "special_instructions", "created_at"}

var foodMerchantCols = []string{"id", "user_id", "merchant_name", "merchant_description", "category",
	"latitude", "longitude", "address", "phone",
	"avg_rating", "total_reviews", "total_orders",
	"opening_time", "closing_time", "is_open", "status", "verified_at", "logo_url",
	"created_at", "updated_at"}

func foodOrderRowValues(oID, custID, merchID uuid.UUID, driver *uuid.UUID) []any {
	return []any{
		oID, custID, merchID, driver,
		nil, nil, nil,
		"Jl. B", nil, nil, nil,
		decimal.NewFromInt(100000), decimal.NewFromInt(20000), nil, nil,
		decimal.Zero, nil, "WALLET", false, decimal.NewFromInt(120000),
		"CREATED", "WAITING", nil,
		time.Now(), nil, nil, nil, nil,
		false, false,
	}
}

func itemRowValues(id, merchID uuid.UUID) []any {
	return []any{
		id, uuid.Nil, merchID, "Nasi", nil, decimal.NewFromInt(50000), nil,
		100, true, time.Now(), time.Now(),
	}
}

const nItemCols = 11

func menuRowValues(id, merchID uuid.UUID) []any {
	return []any{id, merchID, "Menu", nil, 0, true, time.Now(), time.Now()}
}

// ---- users / wallets basics ----

func TestRepo_GetMerchantUser(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectQuery("SELECT id, user_type, status").
		WithArgs(fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_type", "status"}).
			AddRow(fCustID, "merchant", "ACTIVE"))
	u, err := r.GetMerchantUser(context.Background(), fCustID)
	assert.NoError(t, err)
	assert.Equal(t, fCustID, u.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_type, status").WithArgs(fCustID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetMerchantUser(context.Background(), fCustID)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestRepo_GetCustomer(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectQuery("SELECT id, user_type, status, COALESCE").
		WithArgs(fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_type", "status", "overdue_debt"}).
			AddRow(fCustID, "customer", "ACTIVE", decimal.Zero))
	c, err := r.GetCustomer(context.Background(), fCustID)
	assert.NoError(t, err)
	assert.Equal(t, "customer", c.UserType)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_type, status, COALESCE").WithArgs(fCustID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetCustomer(context.Background(), fCustID)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestRepo_SystemWalletID(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(walletTypeSystemEscrow).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(fEscrowID))
	id, err := r.SystemWalletID(context.Background(), mDB, walletTypeSystemEscrow)
	assert.NoError(t, err)
	assert.Equal(t, fEscrowID, id)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").WithArgs(walletTypeSystemEscrow).WillReturnError(pgx.ErrNoRows)
	_, err = r.SystemWalletID(context.Background(), mDB, walletTypeSystemEscrow)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestRepo_GetWalletByUserAndType(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status").
		WithArgs(fCustID, walletTypeCustomer).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status"}).
			AddRow(fCustWallet, fCustID, walletTypeCustomer, decimal.NewFromInt(100), "ACTIVE"))
	w, err := r.GetWalletByUserAndType(context.Background(), fCustID, walletTypeCustomer)
	assert.NoError(t, err)
	assert.Equal(t, fCustWallet, w.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status").WithArgs(fCustID, walletTypeCustomer).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetWalletByUserAndType(context.Background(), fCustID, walletTypeCustomer)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestRepo_InsertMerchantWallet(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectQuery("INSERT INTO wallets").
		WithArgs(fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status"}).
			AddRow(fMerchWallet, fCustID, WalletTypeMerchant, decimal.Zero, "ACTIVE"))
	w, err := r.InsertMerchantWallet(context.Background(), mDB, fCustID)
	assert.NoError(t, err)
	assert.Equal(t, fMerchWallet, w.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_InsertMerchant(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("INSERT INTO food_merchants").
		WithArgs(fMerchID, fCustID, "Warung", pgxmock.AnyArg(), "food", decimal.NewFromFloat(-6.2), decimal.NewFromFloat(106.8),
			"Jl. A", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), true, "ACTIVE", pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	m := &Merchant{ID: fMerchID, UserID: fCustID, Name: "Warung", Category: "food",
		Latitude: decimal.NewFromFloat(-6.2), Longitude: decimal.NewFromFloat(106.8), Address: "Jl. A",
		IsOpen: true, Status: "ACTIVE"}
	err := r.InsertMerchant(context.Background(), mDB, m)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- merchant getters ----

func TestRepo_GetMerchantByID(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := foodMerchantCols
	mDB.ExpectQuery("SELECT id, user_id, merchant_name").
		WithArgs(fMerchID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(merchantRowValues(fMerchID, fCustID)...))
	m, err := r.GetMerchantByID(context.Background(), fMerchID)
	assert.NoError(t, err)
	assert.Equal(t, fMerchID, m.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_id, merchant_name").WithArgs(fMerchID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetMerchantByID(context.Background(), fMerchID)
	assert.ErrorIs(t, err, ErrMerchantNotFound)
}

func TestRepo_GetMerchantByUserID(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := foodMerchantCols
	mDB.ExpectQuery("SELECT id, user_id, merchant_name").
		WithArgs(fCustID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(merchantRowValues(fMerchID, fCustID)...))
	m, err := r.GetMerchantByUserID(context.Background(), fCustID)
	assert.NoError(t, err)
	assert.Equal(t, fMerchID, m.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_UpdateMerchant(t *testing.T) {
	r, mDB := newFoodRepo(t)
	name := "Warung 2"
	mDB.ExpectExec("UPDATE food_merchants").
		WithArgs(fMerchID, &name, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.UpdateMerchant(context.Background(), fMerchID, &name, nil, nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE food_merchants").WithArgs(fMerchID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	ok, err = r.UpdateMerchant(context.Background(), fMerchID, nil, nil, nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.False(t, ok)
}

// ---- menus ----

func TestRepo_GetMenus(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := []string{"id", "merchant_id", "name", "description", "sequence_order", "is_active", "created_at", "updated_at"}
	mDB.ExpectQuery("SELECT id, merchant_id, name").
		WithArgs(fMerchID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(menuRowValues(uuid.New(), fMerchID)...).AddRow(menuRowValues(uuid.New(), fMerchID)...))
	menus, err := r.GetMenus(context.Background(), fMerchID)
	assert.NoError(t, err)
	assert.Len(t, menus, 2)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetMenuByID(t *testing.T) {
	r, mDB := newFoodRepo(t)
	menuID := uuid.New()
	cols := []string{"id", "merchant_id", "name", "description", "sequence_order", "is_active", "created_at", "updated_at"}
	mDB.ExpectQuery("SELECT id, merchant_id, name").
		WithArgs(menuID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(menuRowValues(menuID, fMerchID)...))
	m, err := r.GetMenuByID(context.Background(), menuID)
	assert.NoError(t, err)
	assert.Equal(t, menuID, m.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, merchant_id, name").WithArgs(menuID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetMenuByID(context.Background(), menuID)
	assert.ErrorIs(t, err, ErrMenuNotFound)
}

func TestRepo_InsertMenu(t *testing.T) {
	r, mDB := newFoodRepo(t)
	menuID := uuid.New()
	mDB.ExpectExec("INSERT INTO merchant_menus").
		WithArgs(menuID, fMerchID, "Menu", pgxmock.AnyArg(), 0).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertMenu(context.Background(), mDB, &Menu{ID: menuID, MerchantID: fMerchID, Name: "Menu"})
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_UpdateMenu(t *testing.T) {
	r, mDB := newFoodRepo(t)
	menuID := uuid.New()
	name := "M2"
	mDB.ExpectExec("UPDATE merchant_menus").
		WithArgs(menuID, fMerchID, &name, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.UpdateMenu(context.Background(), menuID, fMerchID, &name, nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE merchant_menus").
		WithArgs(menuID, fMerchID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	_, err = r.UpdateMenu(context.Background(), menuID, fMerchID, nil, nil, nil, nil)
	assert.ErrorIs(t, err, ErrMenuNotFound)
}

func TestRepo_DeleteMenu(t *testing.T) {
	r, mDB := newFoodRepo(t)
	menuID := uuid.New()
	mDB.ExpectExec("DELETE FROM merchant_menus").
		WithArgs(menuID, fMerchID).
		WillReturnResult(pgconn.NewCommandTag("DELETE 1"))
	ok, err := r.DeleteMenu(context.Background(), menuID, fMerchID)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- items ----

func TestRepo_GetItems(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := []string{"id", "menu_id", "merchant_id", "name", "description", "price", "image_url", "stock", "is_available", "created_at", "updated_at"}
	mDB.ExpectQuery("SELECT id, menu_id, merchant_id, name").
		WithArgs(fMerchID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(itemRowValues(fItemID, fMerchID)...))
	items, err := r.GetItems(context.Background(), fMerchID)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetItemByID(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := []string{"id", "menu_id", "merchant_id", "name", "description", "price", "image_url", "stock", "is_available", "created_at", "updated_at"}
	mDB.ExpectQuery("SELECT id, menu_id, merchant_id, name").
		WithArgs(fItemID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(itemRowValues(fItemID, fMerchID)...))
	it, err := r.GetItemByID(context.Background(), fItemID)
	assert.NoError(t, err)
	assert.Equal(t, fItemID, it.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, menu_id, merchant_id, name").WithArgs(fItemID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetItemByID(context.Background(), fItemID)
	assert.ErrorIs(t, err, ErrItemNotFound)
}

func TestRepo_InsertItem(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("INSERT INTO merchant_items").
		WithArgs(fItemID, uuid.Nil, fMerchID, "Nasi", pgxmock.AnyArg(), decimal.NewFromInt(50000), pgxmock.AnyArg(), 100, true).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertItem(context.Background(), mDB, &Item{
		ID: fItemID, MenuID: uuid.Nil, MerchantID: fMerchID, Name: "Nasi",
		Price: decimal.NewFromInt(50000), Stock: 100, IsAvailable: true,
	})
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_UpdateItem(t *testing.T) {
	r, mDB := newFoodRepo(t)
	stock := 5
	mDB.ExpectExec("UPDATE merchant_items").
		WithArgs(fItemID, fMerchID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), &stock, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.UpdateItem(context.Background(), fItemID, fMerchID, nil, nil, nil, nil, &stock, nil)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE merchant_items").
		WithArgs(fItemID, fMerchID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	_, err = r.UpdateItem(context.Background(), fItemID, fMerchID, nil, nil, nil, nil, nil, nil)
	assert.ErrorIs(t, err, ErrItemNotFound)
}

func TestRepo_DeleteItem(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("DELETE FROM merchant_items").
		WithArgs(fItemID, fMerchID).
		WillReturnResult(pgconn.NewCommandTag("DELETE 1"))
	ok, err := r.DeleteItem(context.Background(), fItemID, fMerchID)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- discovery ----

func TestRepo_SearchMerchants(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := foodMerchantCols
	search := "war"
	lat, lng := -6.2, 106.8
	mDB.ExpectQuery("FROM food_merchants").
		WithArgs(pgxmock.AnyArg(), &search, lat, lng).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(merchantRowValues(fMerchID, fCustID)...).AddRow(merchantRowValues(uuid.New(), fCustID)...))
	merchants, err := r.SearchMerchants(context.Background(), nil, &search, &lat, &lng)
	assert.NoError(t, err)
	assert.Len(t, merchants, 2)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetAvailableItems(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := []string{"id", "menu_id", "merchant_id", "name", "description", "price", "image_url", "stock", "is_available", "created_at", "updated_at"}
	mDB.ExpectQuery("SELECT id, menu_id, merchant_id, name").
		WithArgs(fMerchID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(itemRowValues(fItemID, fMerchID)...))
	items, err := r.GetAvailableItems(context.Background(), fMerchID)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetItemsByIDs(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := []string{"id", "menu_id", "merchant_id", "name", "description", "price", "image_url", "stock", "is_available", "created_at", "updated_at"}
	ids := []uuid.UUID{fItemID}
	mDB.ExpectQuery("SELECT id, menu_id, merchant_id, name").
		WithArgs(ids).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(itemRowValues(fItemID, fMerchID)...))
	items, err := r.GetItemsByIDs(context.Background(), ids)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- food orders ----

func TestRepo_InsertFoodOrder(t *testing.T) {
	r, mDB := newFoodRepo(t)
	o := fFoodOrder(foodStatusCreated, PaymentMethodWallet, nil)
	comm := decimal.NewFromInt(15000)
	o.DiscountAmount = decimal.Zero
	mDB.ExpectExec("INSERT INTO food_orders").
		WithArgs(o.ID, o.CustomerID, o.MerchantID, &fCustWallet, &fMerchWallet,
			"", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), decimal.NewFromInt(100000), foodDeliveryFee, &comm,
			decimal.Zero, pgxmock.AnyArg(), "WALLET", decimal.NewFromInt(120000), "CREATED", "WAITING").
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertFoodOrder(context.Background(), mDB, o)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_InsertFoodOrderItem(t *testing.T) {
	r, mDB := newFoodRepo(t)
	it := &FoodOrderItem{OrderID: fOrderID, ItemID: fItemID, ItemName: "Nasi",
		ItemPrice: decimal.NewFromInt(50000), Quantity: 2, Subtotal: decimal.NewFromInt(100000),
		Options: json.RawMessage(`{}`), OptionsTotal: decimal.Zero}
	mDB.ExpectExec("INSERT INTO food_order_items").
		WithArgs(uuid.Nil, fOrderID, fItemID, "Nasi", decimal.NewFromInt(50000), 2, decimal.NewFromInt(100000),
			json.RawMessage(`{}`), decimal.Zero, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertFoodOrderItem(context.Background(), mDB, it)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_InsertFoodOrderEvent(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("INSERT INTO food_order_events").
		WithArgs(fOrderID, pgxmock.AnyArg(), "CREATED", pgxmock.AnyArg(), &fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertFoodOrderEvent(context.Background(), mDB, FoodOrderEvent{OrderID: fOrderID, ToStatus: "CREATED", TriggeredBy: &fCustID})
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetFoodOrderByID(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := foodOrderCols
	d := fDriverID
	mDB.ExpectQuery("FROM food_orders WHERE id").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(foodOrderRowValues(fOrderID, fCustID, fMerchID, &d)...))
	o, err := r.GetFoodOrderByID(context.Background(), fOrderID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, o.ID)
	assert.Equal(t, fCustID, o.CustomerID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FROM food_orders WHERE id").WithArgs(fOrderID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetFoodOrderByID(context.Background(), fOrderID)
	assert.ErrorIs(t, err, ErrFoodOrderNotFound)
}

func TestRepo_LockFoodOrder(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := foodOrderCols
	mDB.ExpectQuery("FOR UPDATE NOWAIT").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(foodOrderRowValues(fOrderID, fCustID, fMerchID, nil)...))
	o, err := r.LockFoodOrder(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, o.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FOR UPDATE NOWAIT").WithArgs(fOrderID).WillReturnError(&pgconn.PgError{Code: "55P03"})
	_, err = r.LockFoodOrder(context.Background(), mDB, fOrderID)
	var pgErr *pgconn.PgError
	assert.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "55P03", pgErr.Code)
}

func TestRepo_UpdateFoodOrderStatus(t *testing.T) {
	r, mDB := newFoodRepo(t)
	ms := "CONFIRMED"
	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(fOrderID, "CREATED", "CONFIRMED", &ms, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.UpdateFoodOrderStatus(context.Background(), mDB, fOrderID, "CREATED", "CONFIRMED", &ms, nil)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE food_orders").
		WithArgs(fOrderID, "CREATED", "CONFIRMED", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	ok, err = r.UpdateFoodOrderStatus(context.Background(), mDB, fOrderID, "CREATED", "CONFIRMED", nil, nil)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestRepo_MarkFoodOrderSettled(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("SET status = 'SETTLED'").
		WithArgs(fOrderID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	err := r.MarkFoodOrderSettled(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_ResetDriverIdle(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("SET working_status = 'IDLE'").
		WithArgs(fDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	err := r.ResetDriverIdle(context.Background(), mDB, fDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_MarkDriverSuspended(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectExec("SET status = 'SUSPENDED'").
		WithArgs(fDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	err := r.MarkDriverSuspended(context.Background(), mDB, fDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetFoodOrderItems(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := []string{"id", "order_id", "item_id", "item_name", "item_price", "quantity", "subtotal",
		"options", "options_total", "special_instructions", "created_at"}
	mDB.ExpectQuery("SELECT id, order_id, item_id, item_name").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(uuid.New(), fOrderID, fItemID, "Nasi", decimal.NewFromInt(50000), 2, decimal.NewFromInt(100000),
				json.RawMessage(`{}`), decimal.Zero, nil, time.Now()))
	items, err := r.GetFoodOrderItems(context.Background(), fOrderID)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_GetFoodOrdersByCustomer(t *testing.T) {
	r, mDB := newFoodRepo(t)
	cols := foodOrderCols
	mDB.ExpectQuery("FROM food_orders WHERE customer_id").
		WithArgs(fCustID, 20, 0).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(foodOrderRowValues(fOrderID, fCustID, fMerchID, nil)...))
	orders, err := r.GetFoodOrdersByCustomer(context.Background(), fCustID, 20, 0)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepo_CountFoodOrdersByCustomer(t *testing.T) {
	r, mDB := newFoodRepo(t)
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs(fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(5))
	count, err := r.CountFoodOrdersByCustomer(context.Background(), fCustID)
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

