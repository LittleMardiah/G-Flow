package send

import (
	"context"
	"errors"
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

func newSendRepo(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewRepository(mDB), mDB
}

var sendOrderCols = []string{
	"id", "sender_id", "driver_id",
	"sender_wallet_id", "driver_wallet_id",
	"package_weight_kg", "package_dimensions_cm", "package_description",
	"pickup_lat", "pickup_lng", "pickup_address",
	"base_fare", "distance_km", "weight_surcharge", "total_fare",
	"declared_value", "package_type", "insurance_fee",
	"discount_amount", "voucher_id",
	"payment_method", "platform_commission", "driver_earning",
	"status", "delivery_photo_url", "recipient_signature",
	"created_at", "assigned_at", "pickup_at", "delivered_at", "settled_at",
	"is_settled",
}

var sendOrderStopCols = []string{
	"id", "order_id", "stop_number",
	"recipient_name", "recipient_phone",
	"dropoff_lat", "dropoff_lng", "dropoff_address",
	"distance_km", "allocated_fare", "status",
	"delivery_photo_url", "recipient_signature",
	"arrived_at", "completed_at", "notes",
}

func sendOrderRowValues(oID, senderID uuid.UUID, driver *uuid.UUID) []any {
	return []any{
		oID, senderID, driver,
		&fCustWallet, nil,
		decimal.NewFromInt(2), nil, nil,
		decimal.NewFromFloat(-6.2), decimal.NewFromFloat(106.8), "Jl. Pickup",
		sendBaseFare, decimal.NewFromFloat(11.112), decimal.Zero, decimal.NewFromInt(48336),
		decimal.Zero, "STANDARD", decimal.Zero,
		decimal.Zero, nil,
		PaymentMethodWallet, nil, nil,
		sendStatusSearchingDriver, nil, []byte("sig"),
		time.Now(), nil, nil, nil, nil,
		false,
	}
}

func sendOrderStopRowValues(id, orderID uuid.UUID, num int, status string) []any {
	return []any{
		id, orderID, num,
		nil, nil,
		decimal.NewFromFloat(-6.25), decimal.NewFromFloat(106.8), "Jl. Stop",
		nil, nil, status,
		nil, []byte("sig"),
		nil, nil, nil,
	}
}

// ---- users / wallets basics ----

func TestRepo_GetCustomer(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("SELECT id, user_type, status, COALESCE").
		WithArgs(fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_type", "status", "overdue_debt"}).
			AddRow(fCustID, userTypeCustomer, statusActive, decimal.Zero))
	c, err := r.GetCustomer(context.Background(), fCustID)
	assert.NoError(t, err)
	assert.Equal(t, fCustID, c.ID)
	assert.Equal(t, userTypeCustomer, c.UserType)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_type, status, COALESCE").WithArgs(fCustID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetCustomer(context.Background(), fCustID)
	assert.ErrorIs(t, err, ErrUserNotFound)

	mDB.ExpectQuery("SELECT id, user_type, status, COALESCE").WithArgs(fCustID).WillReturnError(errors.New("db down"))
	_, err = r.GetCustomer(context.Background(), fCustID)
	assert.Error(t, err)
}

func TestRepo_GetWalletByUserAndType(t *testing.T) {
	r, mDB := newSendRepo(t)
	cols := []string{"id", "user_id", "wallet_type", "balance", "status"}
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status").
		WithArgs(fCustID, walletTypeCustomer).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(fCustWallet, fCustID, walletTypeCustomer, decimal.NewFromInt(100), statusActive))
	w, err := r.GetWalletByUserAndType(context.Background(), fCustID, walletTypeCustomer)
	assert.NoError(t, err)
	assert.Equal(t, fCustWallet, w.ID)
	assert.Equal(t, walletTypeCustomer, w.Type)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status").WithArgs(fCustID, walletTypeCustomer).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetWalletByUserAndType(context.Background(), fCustID, walletTypeCustomer)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestRepo_SystemWalletID(t *testing.T) {
	r, mDB := newSendRepo(t)
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

func TestRepo_GetDriver(t *testing.T) {
	r, mDB := newSendRepo(t)
	cols := []string{"id", "user_type", "status", "working_status", "min_balance_threshold"}
	mDB.ExpectQuery("SELECT id, user_type, status, working_status").
		WithArgs(fDriverID).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(fDriverID, userTypeDriver, statusActive, workingStatusIdle, decimal.Zero))
	d, err := r.GetDriver(context.Background(), fDriverID)
	assert.NoError(t, err)
	assert.Equal(t, fDriverID, d.ID)
	assert.Equal(t, userTypeDriver, d.UserType)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT id, user_type, status, working_status").WithArgs(fDriverID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetDriver(context.Background(), fDriverID)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestRepo_GetDriverActiveSendOrdersCount(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs(fDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))
	count, err := r.GetDriverActiveSendOrdersCount(context.Background(), fDriverID)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("SELECT COUNT").WithArgs(fDriverID).WillReturnError(errors.New("db down"))
	_, err = r.GetDriverActiveSendOrdersCount(context.Background(), fDriverID)
	assert.Error(t, err)
}

// ---- accept order (Task 3.6.1) ----

func TestRepo_LockSendOrderForAccept(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("status = 'SEARCHING_DRIVER' FOR UPDATE NOWAIT").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(sendOrderCols).AddRow(sendOrderRowValues(fOrderID, fCustID, nil)...))
	o, err := r.LockSendOrderForAccept(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, o.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("status = 'SEARCHING_DRIVER' FOR UPDATE NOWAIT").WithArgs(fOrderID).WillReturnError(pgx.ErrNoRows)
	_, err = r.LockSendOrderForAccept(context.Background(), mDB, fOrderID)
	assert.ErrorIs(t, err, ErrSendOrderNotFound)

	mDB.ExpectQuery("status = 'SEARCHING_DRIVER' FOR UPDATE NOWAIT").WithArgs(fOrderID).WillReturnError(&pgconn.PgError{Code: "55P03"})
	_, err = r.LockSendOrderForAccept(context.Background(), mDB, fOrderID)
	var pgErr *pgconn.PgError
	assert.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "55P03", pgErr.Code)
}

func TestRepo_LockDriverUserForAccept(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("FOR UPDATE NOWAIT").
		WithArgs(fDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(fDriverID))
	err := r.LockDriverUserForAccept(context.Background(), mDB, fDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FOR UPDATE NOWAIT").WithArgs(fDriverID).WillReturnError(pgx.ErrNoRows)
	err = r.LockDriverUserForAccept(context.Background(), mDB, fDriverID)
	assert.ErrorIs(t, err, ErrUserNotFound)

	mDB.ExpectQuery("FOR UPDATE NOWAIT").WithArgs(fDriverID).WillReturnError(&pgconn.PgError{Code: "55P03"})
	err = r.LockDriverUserForAccept(context.Background(), mDB, fDriverID)
	var pgErr *pgconn.PgError
	assert.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "55P03", pgErr.Code)
}

func TestRepo_AssignDriverToSendOrder(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(fOrderID, fDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.AssignDriverToSendOrder(context.Background(), mDB, fOrderID, fDriverID)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE send_orders").WithArgs(fOrderID, fDriverID).WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	ok, err = r.AssignDriverToSendOrder(context.Background(), mDB, fOrderID, fDriverID)
	assert.NoError(t, err)
	assert.False(t, ok)

	mDB.ExpectExec("UPDATE send_orders").WithArgs(fOrderID, fDriverID).WillReturnError(errors.New("db down"))
	_, err = r.AssignDriverToSendOrder(context.Background(), mDB, fOrderID, fDriverID)
	assert.Error(t, err)
}

func TestRepo_UpdateDriverWorkingStatus(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectExec("UPDATE users").
		WithArgs(fDriverID, workingStatusBusy).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	err := r.UpdateDriverWorkingStatus(context.Background(), mDB, fDriverID, workingStatusBusy)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE users").WithArgs(fDriverID, workingStatusBusy).WillReturnError(errors.New("db down"))
	err = r.UpdateDriverWorkingStatus(context.Background(), mDB, fDriverID, workingStatusBusy)
	assert.Error(t, err)
}

func TestRepo_MarkDriverSuspended(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectExec("SET status = 'SUSPENDED'").
		WithArgs(fDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	err := r.MarkDriverSuspended(context.Background(), mDB, fDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("SET status = 'SUSPENDED'").WithArgs(fDriverID).WillReturnError(errors.New("db down"))
	err = r.MarkDriverSuspended(context.Background(), mDB, fDriverID)
	assert.Error(t, err)
}

func TestRepo_MarkSendOrderDelivered(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectExec("SET status = 'DELIVERED'").
		WithArgs(fOrderID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.MarkSendOrderDelivered(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("SET status = 'DELIVERED'").WithArgs(fOrderID).WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	ok, err = r.MarkSendOrderDelivered(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.False(t, ok)

	mDB.ExpectExec("SET status = 'DELIVERED'").WithArgs(fOrderID).WillReturnError(errors.New("db down"))
	_, err = r.MarkSendOrderDelivered(context.Background(), mDB, fOrderID)
	assert.Error(t, err)
}

func TestRepo_MarkSendOrderSettled(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectExec("SET status = 'SETTLED'").
		WithArgs(fOrderID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	err := r.MarkSendOrderSettled(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("SET status = 'SETTLED'").WithArgs(fOrderID).WillReturnError(errors.New("db down"))
	err = r.MarkSendOrderSettled(context.Background(), mDB, fOrderID)
	assert.Error(t, err)
}

// ---- stops ----

func TestRepo_GetSendOrderStopsByOrderID(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("FROM send_order_stops").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(sendOrderStopCols).
			AddRow(sendOrderStopRowValues(fStop1ID, fOrderID, 1, stopStatusPending)...).
			AddRow(sendOrderStopRowValues(fStop2ID, fOrderID, 2, stopStatusDelivered)...))
	stops, err := r.GetSendOrderStopsByOrderID(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.Len(t, stops, 2)
	assert.Equal(t, fStop1ID, stops[0].ID)
	assert.Equal(t, stopStatusDelivered, stops[1].Status)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FROM send_order_stops").WithArgs(fOrderID).WillReturnRows(pgxmock.NewRows(sendOrderStopCols))
	stops, err = r.GetSendOrderStopsByOrderID(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.Len(t, stops, 0)

	mDB.ExpectQuery("FROM send_order_stops").WithArgs(fOrderID).WillReturnError(errors.New("db down"))
	_, err = r.GetSendOrderStopsByOrderID(context.Background(), mDB, fOrderID)
	assert.Error(t, err)
}

func TestRepo_UpdateSendOrderStopStatus(t *testing.T) {
	r, mDB := newSendRepo(t)
	proof := "https://img/1.jpg"
	var nilProof *string
	mDB.ExpectExec("UPDATE send_order_stops").
		WithArgs(fStop1ID, stopStatusDelivered, &proof).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.UpdateSendOrderStopStatus(context.Background(), mDB, fStop1ID, stopStatusDelivered, &proof)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE send_order_stops").WithArgs(fStop1ID, "ARRIVED", nilProof).WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	ok, err = r.UpdateSendOrderStopStatus(context.Background(), mDB, fStop1ID, "ARRIVED", nilProof)
	assert.NoError(t, err)
	assert.False(t, ok)

	mDB.ExpectExec("UPDATE send_order_stops").WithArgs(fStop1ID, stopStatusDelivered, nilProof).WillReturnError(errors.New("db down"))
	_, err = r.UpdateSendOrderStopStatus(context.Background(), mDB, fStop1ID, stopStatusDelivered, nilProof)
	assert.Error(t, err)
}

// ---- inserts ----

func TestRepo_InsertSendOrder(t *testing.T) {
	r, mDB := newSendRepo(t)
	o := &SendOrder{
		ID: fOrderID, SenderID: fCustID, DriverID: nil,
		SenderWalletID: &fCustWallet, DriverWalletID: nil,
		PackageWeightKg:     decimal.NewFromInt(2),
		PackageDimensionsCm: nil, PackageDescription: nil,
		PickupLat: decimal.NewFromFloat(-6.2), PickupLng: decimal.NewFromFloat(106.8),
		PickupAddress:   "Jl. Pickup",
		BaseFare:        sendBaseFare,
		DistanceKm:      decimal.NewFromFloat(11.112),
		WeightSurcharge: decimal.Zero,
		TotalFare:       decimal.NewFromInt(48336),
		DeclaredValue:   decimal.Zero,
		PackageType:     "STANDARD",
		InsuranceFee:    decimal.Zero,
		DiscountAmount:  decimal.Zero,
		VoucherID:       nil,
		PaymentMethod:   PaymentMethodWallet,
		Status:          sendStatusSearchingDriver,
	}
	mDB.ExpectExec("INSERT INTO send_orders").
		WithArgs(o.ID, o.SenderID, o.DriverID, o.SenderWalletID, o.DriverWalletID,
			o.PackageWeightKg, o.PackageDimensionsCm, o.PackageDescription,
			o.PickupLat, o.PickupLng, o.PickupAddress,
			o.BaseFare, o.DistanceKm, o.WeightSurcharge, o.TotalFare,
			o.DeclaredValue, o.PackageType, o.InsuranceFee,
			o.DiscountAmount, o.VoucherID,
			o.PaymentMethod, o.Status).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertSendOrder(context.Background(), mDB, o)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("INSERT INTO send_orders").WillReturnError(errors.New("db down"))
	err = r.InsertSendOrder(context.Background(), mDB, o)
	assert.Error(t, err)
}

func TestRepo_InsertSendOrderStops(t *testing.T) {
	r, mDB := newSendRepo(t)
	stops := []*SendOrderStop{
		fSendStop(fOrderID, fStop1ID, 1, stopStatusPending),
		fSendStop(fOrderID, fStop2ID, 2, stopStatusPending),
	}
	mDB.ExpectExec("INSERT INTO send_order_stops").
		WithArgs(stops[0].ID, stops[0].OrderID, stops[0].StopNumber, stops[0].RecipientName, stops[0].RecipientPhone,
			stops[0].DropoffLat, stops[0].DropoffLng, stops[0].DropoffAddress,
			stops[0].DistanceKm, stops[0].AllocatedFare, stops[0].Status).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectExec("INSERT INTO send_order_stops").
		WithArgs(stops[1].ID, stops[1].OrderID, stops[1].StopNumber, stops[1].RecipientName, stops[1].RecipientPhone,
			stops[1].DropoffLat, stops[1].DropoffLng, stops[1].DropoffAddress,
			stops[1].DistanceKm, stops[1].AllocatedFare, stops[1].Status).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertSendOrderStops(context.Background(), mDB, stops)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("INSERT INTO send_order_stops").WillReturnError(errors.New("db down"))
	err = r.InsertSendOrderStops(context.Background(), mDB, stops[:1])
	assert.Error(t, err)
}

func TestRepo_InsertSendOrderEvent(t *testing.T) {
	r, mDB := newSendRepo(t)
	e := SendOrderEvent{
		OrderID:     fOrderID,
		FromStatus:  nil,
		ToStatus:    sendStatusCancelled,
		Reason:      nil,
		TriggeredBy: &fCustID,
		Metadata:    []byte(`{"reason":"test"}`),
	}
	mDB.ExpectExec("INSERT INTO send_order_events").
		WithArgs(e.OrderID, e.FromStatus, e.ToStatus, e.Reason, e.TriggeredBy, e.Metadata).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	err := r.InsertSendOrderEvent(context.Background(), mDB, e)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("INSERT INTO send_order_events").WillReturnError(errors.New("db down"))
	err = r.InsertSendOrderEvent(context.Background(), mDB, e)
	assert.Error(t, err)
}

// ---- send order getters ----

func TestRepo_GetSendOrderByID(t *testing.T) {
	r, mDB := newSendRepo(t)
	d := fDriverID
	mDB.ExpectQuery("FROM send_orders WHERE id").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(sendOrderCols).AddRow(sendOrderRowValues(fOrderID, fCustID, &d)...))
	o, err := r.GetSendOrderByID(context.Background(), fOrderID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, o.ID)
	assert.Equal(t, fCustID, o.SenderID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FROM send_orders WHERE id").WithArgs(fOrderID).WillReturnError(pgx.ErrNoRows)
	_, err = r.GetSendOrderByID(context.Background(), fOrderID)
	assert.ErrorIs(t, err, ErrSendOrderNotFound)
}

func TestRepo_LockSendOrder(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("FOR UPDATE NOWAIT").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(sendOrderCols).AddRow(sendOrderRowValues(fOrderID, fCustID, nil)...))
	o, err := r.LockSendOrder(context.Background(), mDB, fOrderID)
	assert.NoError(t, err)
	assert.Equal(t, fOrderID, o.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FOR UPDATE NOWAIT").WithArgs(fOrderID).WillReturnError(&pgconn.PgError{Code: "55P03"})
	_, err = r.LockSendOrder(context.Background(), mDB, fOrderID)
	var pgErr *pgconn.PgError
	assert.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "55P03", pgErr.Code)

	mDB.ExpectQuery("FOR UPDATE NOWAIT").WithArgs(fOrderID).WillReturnError(pgx.ErrNoRows)
	_, err = r.LockSendOrder(context.Background(), mDB, fOrderID)
	assert.ErrorIs(t, err, ErrSendOrderNotFound)
}

func TestRepo_UpdateSendOrderStatus(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(fOrderID, sendStatusDriverAssigned, sendStatusPickedUp).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	ok, err := r.UpdateSendOrderStatus(context.Background(), mDB, fOrderID, sendStatusDriverAssigned, sendStatusPickedUp)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(fOrderID, sendStatusDriverAssigned, sendStatusPickedUp).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	ok, err = r.UpdateSendOrderStatus(context.Background(), mDB, fOrderID, sendStatusDriverAssigned, sendStatusPickedUp)
	assert.NoError(t, err)
	assert.False(t, ok)

	mDB.ExpectExec("UPDATE send_orders").
		WithArgs(fOrderID, sendStatusDriverAssigned, sendStatusPickedUp).
		WillReturnError(errors.New("db down"))
	_, err = r.UpdateSendOrderStatus(context.Background(), mDB, fOrderID, sendStatusDriverAssigned, sendStatusPickedUp)
	assert.Error(t, err)
}

func TestRepo_GetSendOrderStops(t *testing.T) {
	r, mDB := newSendRepo(t)
	mDB.ExpectQuery("FROM send_order_stops").
		WithArgs(fOrderID).
		WillReturnRows(pgxmock.NewRows(sendOrderStopCols).
			AddRow(sendOrderStopRowValues(fStop1ID, fOrderID, 1, stopStatusPending)...).
			AddRow(sendOrderStopRowValues(fStop2ID, fOrderID, 2, stopStatusDelivered)...))
	stops, err := r.GetSendOrderStops(context.Background(), fOrderID)
	assert.NoError(t, err)
	assert.Len(t, stops, 2)
	assert.Equal(t, 1, stops[0].StopNumber)
	assert.Equal(t, stopStatusDelivered, stops[1].Status)
	assert.NoError(t, mDB.ExpectationsWereMet())

	mDB.ExpectQuery("FROM send_order_stops").WithArgs(fOrderID).WillReturnRows(pgxmock.NewRows(sendOrderStopCols))
	stops, err = r.GetSendOrderStops(context.Background(), fOrderID)
	assert.NoError(t, err)
	assert.Len(t, stops, 0)

	mDB.ExpectQuery("FROM send_order_stops").WithArgs(fOrderID).WillReturnError(errors.New("db down"))
	_, err = r.GetSendOrderStops(context.Background(), fOrderID)
	assert.Error(t, err)
}
