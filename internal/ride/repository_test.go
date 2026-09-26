package ride

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

var (
	repoOrderID  = uuid.MustParse("81111111-1111-1111-1111-111111111111")
	repoCustID   = uuid.MustParse("82222222-2222-2222-2222-222222222222")
	repoDriverID = uuid.MustParse("83333333-3333-3333-3333-333333333333")
	repoWalletID = uuid.MustParse("84444444-4444-4444-4444-444444444444")
	repoEscrowID = uuid.MustParse("85555555-5555-5555-5555-555555555555")
)

// ptrString helper pembungkus literal string jadi *string (dipakai test
// kolom driver nullable).
func ptrString(s string) *string { return &s }

// orderColumnNames daftar kolom hasil SELECT ride_orders tanpa JOIN (34).
var orderColumnNames = []string{
	"id", "customer_id", "driver_id",
	"customer_wallet_id", "driver_wallet_id",
	"pickup_lat", "pickup_lng", "pickup_address",
	"dropoff_lat", "dropoff_lng", "dropoff_address",
	"distance_km", "base_fare", "per_km_rate", "estimated_fare", "actual_fare",
	"surge_multiplier", "toll_fee", "cancellation_fee", "discount_amount", "voucher_id",
	"payment_method", "platform_commission", "driver_earning",
	"status", "cancellation_reason",
	"created_at", "expires_at", "assigned_at", "pickup_at", "completed_at", "settled_at",
	"is_settled", "settlement_notes",
}

// orderWithDriverColumnNames = orderColumnNames + 4 kolom users yang di-LEFT
// JOIN GetOrderByID (users.name/phone/vehicle_type/vehicle_plate) → 38 kolom.
// Urutannya WAJIB sama dengan append di scanOrderValues(repository.go).
var orderWithDriverColumnNames = append(append([]string{}, orderColumnNames...),
	"driver_name", "driver_phone", "vehicle_type", "vehicle_plate")

// orderRowValues urutan nilai yang cocok dengan orderColumnNames.
func orderRowValues(o *RideOrder) []any {
	return []any{
		o.ID, o.CustomerID, o.DriverID,
		o.CustomerWalletID, o.DriverWalletID,
		o.PickupLat, o.PickupLng, o.PickupAddress,
		o.DropoffLat, o.DropoffLng, o.DropoffAddress,
		o.DistanceKm, o.BaseFare, o.PerKmRate, o.EstimatedFare, o.ActualFare,
		o.SurgeMultiplier, o.TollFee, o.CancellationFee, o.DiscountAmount, o.VoucherID,
		o.PaymentMethod, o.PlatformCommission, o.DriverEarning,
		o.Status, o.CancellationReason,
		o.CreatedAt, o.ExpiresAt, o.AssignedAt, o.PickupAt, o.CompletedAt, o.SettledAt,
		o.IsSettled, o.SettlementNotes,
	}
}

// orderRowSet membangun pgxmock rows untuk kolom lengkap ride_orders.
func orderRowSet(o *RideOrder) *pgxmock.Rows {
	return pgxmock.NewRows(orderColumnNames).AddRow(orderRowValues(o)...)
}

// orderRowSetWithDriver membangun pgxmock rows 38 kolom (ride_orders LEFT JOIN
// users) sesuai yang diminta scanOrderWithDriverRow.
func orderRowSetWithDriver(o *RideOrder) *pgxmock.Rows {
	return pgxmock.NewRows(orderWithDriverColumnNames).
		AddRow(append(orderRowValues(o), o.DriverName, o.DriverPhone, o.DriverVehicleType, o.DriverVehiclePlate)...)
}

func repoOrder(status string, driver *uuid.UUID) *RideOrder {
	exp := time.Now().Add(15 * time.Minute)
	return &RideOrder{
		ID:               repoOrderID,
		CustomerID:       repoCustID,
		DriverID:         driver,
		CustomerWalletID: &repoWalletID,
		PickupLat:        -6.2,
		PickupLng:        106.816666,
		DropoffLat:       -6.26,
		DropoffLng:       106.816666,
		DistanceKm:       decimal.NewFromFloat(6.7),
		BaseFare:         decimal.NewFromInt(10000),
		PerKmRate:        decimal.NewFromInt(4000),
		EstimatedFare:    decimal.NewFromInt(36800),
		PaymentMethod:    PaymentMethodWallet,
		Status:           status,
		CreatedAt:        time.Now(),
		ExpiresAt:        &exp,
		IsSettled:        false,
	}
}

func TestRepository_GetCustomer_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id, user_type, status, COALESCE\\(overdue_debt").
		WithArgs(repoCustID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_type", "status", "overdue_debt"}).
			AddRow(repoCustID, userTypeCustomer, "ACTIVE", decimal.Zero))

	repo := NewRepository(mDB)
	c, err := repo.GetCustomer(context.Background(), repoCustID)
	assert.NoError(t, err)
	assert.Equal(t, userTypeCustomer, c.UserType)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetCustomer_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id, user_type, status").
		WithArgs(repoCustID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	_, err = repo.GetCustomer(context.Background(), repoCustID)
	assert.ErrorIs(t, err, ErrCustomerNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetWalletByUserAndType_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status").
		WithArgs(repoCustID, WalletTypeCustomer).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status"}).
			AddRow(repoWalletID, repoCustID, WalletTypeCustomer, decimal.NewFromInt(100000), "ACTIVE"))

	repo := NewRepository(mDB)
	w, err := repo.GetWalletByUserAndType(context.Background(), repoCustID, WalletTypeCustomer)
	assert.NoError(t, err)
	assert.Equal(t, repoWalletID, w.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetWalletByUserAndType_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id, user_id, wallet_type").
		WithArgs(repoCustID, WalletTypeCustomer).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	_, err = repo.GetWalletByUserAndType(context.Background(), repoCustID, WalletTypeCustomer)
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_InsertOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("INSERT INTO ride_orders").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))

	repo := NewRepository(mDB)
	err = repo.InsertOrder(context.Background(), mDB, repoOrder(statusCreated, nil))
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_TransitionStatus_True(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(statusSearchingDriver, repoOrderID, statusCreated).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	ok, err := repo.TransitionStatus(context.Background(), mDB, repoOrderID, statusCreated, statusSearchingDriver)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_TransitionStatus_False(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(statusSearchingDriver, repoOrderID, statusCreated).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewRepository(mDB)
	ok, err := repo.TransitionStatus(context.Background(), mDB, repoOrderID, statusCreated, statusSearchingDriver)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_InsertEvent(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	from := statusCreated
	mDB.ExpectExec("INSERT INTO ride_order_events").
		WithArgs(repoOrderID, pgxmock.AnyArg(), statusSearchingDriver, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))

	repo := NewRepository(mDB)
	err = repo.InsertEvent(context.Background(), mDB, RideOrderEvent{
		OrderID: repoOrderID, FromStatus: &from, ToStatus: statusSearchingDriver, TriggeredBy: &repoCustID,
	})
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_SystemWalletID_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemEscrow).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoEscrowID))

	repo := NewRepository(mDB)
	id, err := repo.SystemWalletID(context.Background(), mDB, WalletTypeSystemEscrow)
	assert.NoError(t, err)
	assert.Equal(t, repoEscrowID, id)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_SystemWalletID_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemEscrow).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	id, err := repo.SystemWalletID(context.Background(), mDB, WalletTypeSystemEscrow)
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.Equal(t, uuid.Nil, id)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetOrderByID_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	o := repoOrder(statusSearchingDriver, nil)
	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnRows(orderRowSetWithDriver(o))

	repo := NewRepository(mDB)
	got, err := repo.GetOrderByID(context.Background(), repoOrderID)
	assert.NoError(t, err)
	assert.Equal(t, repoOrderID, got.ID)
	assert.Equal(t, statusSearchingDriver, got.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetOrderByID_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	_, err = repo.GetOrderByID(context.Background(), repoOrderID)
	assert.ErrorIs(t, err, ErrOrderNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- TD-113: GetOrderByID LEFT JOIN users untuk info driver ----

// TestRepo_GetOrderByID_WithDriver memverifikasi query meng-JOIN users
// (bukan hanya ride_orders) dan 4 kolom driver ter-scan ke RideOrder.
func TestRepo_GetOrderByID_WithDriver(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	o := repoOrder(statusDriverAssigned, &repoDriverID)
	o.DriverName = ptrString("Budi Santoso")
	o.DriverPhone = ptrString("+628123456789")
	o.DriverVehicleType = ptrString("MOTORCYCLE")
	o.DriverVehiclePlate = ptrString("B 1234 ABC")

	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnRows(orderRowSetWithDriver(o))

	repo := NewRepository(mDB)
	got, err := repo.GetOrderByID(context.Background(), repoOrderID)
	require.NoError(t, err)

	assert.Equal(t, repoDriverID, *got.DriverID)
	require.NotNil(t, got.DriverName)
	assert.Equal(t, "Budi Santoso", *got.DriverName)
	require.NotNil(t, got.DriverPhone)
	// PENTING: repository TIDAK melakukan masking — itu urusan service.GetOrder.
	assert.Equal(t, "+628123456789", *got.DriverPhone)
	require.NotNil(t, got.DriverVehicleType)
	assert.Equal(t, "MOTORCYCLE", *got.DriverVehicleType)
	require.NotNil(t, got.DriverVehiclePlate)
	assert.Equal(t, "B 1234 ABC", *got.DriverVehiclePlate)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepo_GetOrderByID_NoDriver Order SEARCHING_DRIVER: driver_id NULL →
// kolom users NULL semua (LEFT JOIN tidak match) → pointer driver nil, bukan error.
func TestRepo_GetOrderByID_NoDriver(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	o := repoOrder(statusSearchingDriver, nil)

	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnRows(orderRowSetWithDriver(o))

	repo := NewRepository(mDB)
	got, err := repo.GetOrderByID(context.Background(), repoOrderID)
	require.NoError(t, err)

	assert.Nil(t, got.DriverID)
	assert.Nil(t, got.DriverName)
	assert.Nil(t, got.DriverPhone)
	assert.Nil(t, got.DriverVehicleType)
	assert.Nil(t, got.DriverVehiclePlate)
	assert.Equal(t, statusSearchingDriver, got.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepo_GetOrderByID_JoinQueryShape mengunci bentuk SQL: wajib LEFT JOIN
// users (bukan INNER — order tanpa driver harus tetap terbaca) dan wajib
// men-select 4 kolom driver. Guard terhadap regresi ke SELECT ride_orders saja.
func TestRepo_GetOrderByID_JoinQueryShape(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	o := repoOrder(statusDriverAssigned, &repoDriverID)

	mDB.ExpectQuery("(?s)LEFT JOIN users ON users.id = ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnRows(orderRowSetWithDriver(o))
	mDB.ExpectQuery("users\\.name, users\\.phone, users\\.vehicle_type, users\\.vehicle_plate").
		WithArgs(repoOrderID).
		WillReturnRows(orderRowSetWithDriver(o))

	repo := NewRepository(mDB)
	// Dua panggilan: expectation pgxmock bersifat ordered, satu per query.
	_, err = repo.GetOrderByID(context.Background(), repoOrderID)
	require.NoError(t, err)
	_, err = repo.GetOrderByID(context.Background(), repoOrderID)
	require.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_ListOrdersByCustomer_NoStatus(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	o := repoOrder(statusSettled, &repoDriverID)
	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoCustID, 20, 0).
		WillReturnRows(orderRowSet(o))

	repo := NewRepository(mDB)
	got, err := repo.ListOrdersByCustomer(context.Background(), repoCustID, "", 20, 0)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, repoOrderID, got[0].ID)
	assert.Equal(t, statusSettled, got[0].Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_ListOrdersByCustomer_WithStatus(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	o := repoOrder(statusCompleted, &repoDriverID)
	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoCustID, "COMPLETED", 10, 10).
		WillReturnRows(orderRowSet(o))

	repo := NewRepository(mDB)
	got, err := repo.ListOrdersByCustomer(context.Background(), repoCustID, "COMPLETED", 10, 10)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, statusCompleted, got[0].Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_CountOrdersByCustomer(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs(repoCustID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(47))

	repo := NewRepository(mDB)
	count, err := repo.CountOrdersByCustomer(context.Background(), repoCustID, "")
	assert.NoError(t, err)
	assert.Equal(t, 47, count)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_CountOrdersByCustomer_WithStatus(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs(repoCustID, "COMPLETED").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	repo := NewRepository(mDB)
	count, err := repo.CountOrdersByCustomer(context.Background(), repoCustID, "COMPLETED")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_LockOrderForUpdate(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	o := repoOrder(statusDriverAssigned, &repoDriverID)
	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnRows(orderRowSet(o))

	repo := NewRepository(mDB)
	got, err := repo.LockOrderForUpdate(context.Background(), mDB, repoOrderID)
	assert.NoError(t, err)
	assert.Equal(t, statusDriverAssigned, got.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetExpiredSearchingOrders(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(repoOrderID))

	repo := NewRepository(mDB)
	ids, err := repo.GetExpiredSearchingOrders(context.Background())
	assert.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, repoOrderID, ids[0])
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_CancelOrder_True(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(repoOrderID, statusSearchingDriver, reasonExpired, decimal.Zero).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	ok, err := repo.CancelOrder(context.Background(), mDB, repoOrderID, statusSearchingDriver, reasonExpired, decimal.Zero)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_CancelOrder_WithFee(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	fee := decimal.NewFromInt(5000)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(repoOrderID, statusDriverAssigned, reasonCustomerCancel, fee).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	ok, err := repo.CancelOrder(context.Background(), mDB, repoOrderID, statusDriverAssigned, reasonCustomerCancel, fee)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_CompleteOrder(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	fare := decimal.NewFromInt(36800)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(repoOrderID, statusTripStarted, fare, fare, fare).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	ok, err := repo.CompleteOrder(context.Background(), mDB, repoOrderID, statusTripStarted, fare, fare, fare)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_MarkSettled(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(repoOrderID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.MarkSettled(context.Background(), mDB, repoOrderID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_IncrementOverdueDebt(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	amount := decimal.NewFromInt(10000)
	mDB.ExpectExec("UPDATE users SET overdue_debt").
		WithArgs(repoCustID, amount).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.IncrementOverdueDebt(context.Background(), mDB, repoCustID, amount)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_ResetDriverIdle(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE users").
		WithArgs(repoDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.ResetDriverIdle(context.Background(), mDB, repoDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_MarkDriverSuspended(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE users").
		WithArgs(repoDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.MarkDriverSuspended(context.Background(), mDB, repoDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetDriver_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id, user_type, status, working_status, COALESCE").
		WithArgs(repoDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_type", "status", "working_status", "min_balance_threshold"}).
			AddRow(repoDriverID, userTypeDriver, "ACTIVE", workingStatusIdle, decimal.Zero))

	repo := NewRepository(mDB)
	d, err := repo.GetDriver(context.Background(), repoDriverID)
	assert.NoError(t, err)
	assert.Equal(t, userTypeDriver, d.UserType)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetDriver_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT id, user_type, status, working_status").
		WithArgs(repoDriverID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	_, err = repo.GetDriver(context.Background(), repoDriverID)
	assert.ErrorIs(t, err, ErrDriverNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetDriverBalance_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(repoDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(50000)))

	repo := NewRepository(mDB)
	b, err := repo.GetDriverBalance(context.Background(), repoDriverID)
	assert.NoError(t, err)
	assert.Equal(t, decimal.NewFromInt(50000), b)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_GetDriverBalance_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(repoDriverID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	_, err = repo.GetDriverBalance(context.Background(), repoDriverID)
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_LockOrderForAccept_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT 1 FROM ride_orders").
		WithArgs(repoOrderID).
		WillReturnRows(pgxmock.NewRows([]string{"1"}).AddRow(1))

	repo := NewRepository(mDB)
	err = repo.LockOrderForAccept(context.Background(), mDB, repoOrderID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_LockOrderForAccept_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT 1 FROM ride_orders").
		WithArgs(repoOrderID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	err = repo.LockOrderForAccept(context.Background(), mDB, repoOrderID)
	assert.ErrorIs(t, err, ErrOrderNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// LockDriverUserForAccept — guard atomik user_type='driver' AND status='ACTIVE'
// dengan SELECT FOR UPDATE NOWAIT (ROADMAP 02 §2.3: users → ride_orders).
func TestRepository_LockDriverUserForAccept_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("FOR UPDATE NOWAIT").
		WithArgs(repoDriverID).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "user_type", "status", "working_status", "min_balance_threshold",
		}).AddRow(repoDriverID, "driver", "ACTIVE", "IDLE", decimal.Zero))

	repo := NewRepository(mDB)
	d, err := repo.LockDriverUserForAccept(context.Background(), mDB, repoDriverID)
	assert.NoError(t, err)
	assert.Equal(t, repoDriverID, d.ID)
	assert.Equal(t, workingStatusIdle, d.WorkingStatus)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_LockDriverUserForAccept_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("FOR UPDATE NOWAIT").
		WithArgs(repoDriverID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	_, err = repo.LockDriverUserForAccept(context.Background(), mDB, repoDriverID)
	assert.ErrorIs(t, err, ErrDriverNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_LockDriverUserForAccept_LockTimeout(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("FOR UPDATE NOWAIT").
		WithArgs(repoDriverID).
		WillReturnError(&pgconn.PgError{Code: "55P03"})

	repo := NewRepository(mDB)
	_, err = repo.LockDriverUserForAccept(context.Background(), mDB, repoDriverID)
	var pgErr *pgconn.PgError
	assert.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "55P03", pgErr.Code)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_AssignDriver_True(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(repoOrderID, repoDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	ok, err := repo.AssignDriver(context.Background(), mDB, repoOrderID, repoDriverID)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_AssignDriver_False(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE ride_orders").
		WithArgs(repoOrderID, repoDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewRepository(mDB)
	ok, err := repo.AssignDriver(context.Background(), mDB, repoOrderID, repoDriverID)
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_MarkDriverBusy(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectExec("UPDATE users").
		WithArgs(repoDriverID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.MarkDriverBusy(context.Background(), mDB, repoDriverID)
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- scanOrderRow error branches ----

func TestScanOrderRow_QueryError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)
	mDB.ExpectQuery("SELECT ride_orders.id, ride_orders.customer_id, ride_orders.driver_id").
		WithArgs(repoOrderID).
		WillReturnError(errors.New("conn refused"))

	repo := NewRepository(mDB)
	_, err = repo.GetOrderByID(context.Background(), repoOrderID)
	assert.EqualError(t, err, "conn refused")
	assert.NoError(t, mDB.ExpectationsWereMet())
}
