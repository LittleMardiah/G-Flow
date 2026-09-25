package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDriverRepo(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewRepository(mDB), mDB
}

var (
	driverID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orderID  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	custID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	merchID  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	sendrID  = uuid.MustParse("55555555-5555-5555-5555-555555555555")
)

func TestRepo_GetDriverLocation(t *testing.T) {
	r, mDB := newDriverRepo(t)
	mDB.ExpectQuery("SELECT current_lat, current_lng").
		WithArgs(driverID).
		WillReturnRows(pgxmock.NewRows([]string{"current_lat", "current_lng"}).
			AddRow(-6.2, 106.8))

	lat, lng, err := r.GetDriverLocation(context.Background(), driverID)
	require.NoError(t, err)
	assert.Equal(t, -6.2, lat)
	assert.Equal(t, 106.8, lng)
}

func TestRepo_GetDriverLocation_NotFound(t *testing.T) {
	r, mDB := newDriverRepo(t)
	mDB.ExpectQuery("SELECT current_lat, current_lng").
		WithArgs(driverID).
		WillReturnError(pgx.ErrNoRows)

	_, _, err := r.GetDriverLocation(context.Background(), driverID)
	require.ErrorIs(t, err, ErrDriverLocationNotFound)
}

func TestRepo_GetDriverProfile(t *testing.T) {
	r, mDB := newDriverRepo(t)
	cols := []string{
		"driver_id", "name", "email", "phone", "vehicle_type", "vehicle_plate",
		"license_number", "license_expiry", "status", "bank_name", "bank_account_number",
		"rating_avg", "total_rides",
	}
	mDB.ExpectQuery("SELECT u.id, u.name").
		WithArgs(driverID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(
			driverID, "Budi Santoso", "budi@example.com", "081234567890", "motor", "B 1234 ABC",
			"SIM-123", "2027-12-31", "ACTIVE", "Bank Central Asia", "1234567890", 4.75, 12,
		))

	profile, err := r.GetDriverProfile(context.Background(), driverID)
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, driverID, profile.DriverID)
	assert.Equal(t, "Budi Santoso", profile.Name)
	maskedAccount := maskBankAccount(profile.BankAccountNumber)
	require.NotNil(t, maskedAccount)
	assert.Equal(t, "****7890", *maskedAccount)
	assert.Equal(t, 4.75, profile.RatingAvg)
	assert.Equal(t, int64(12), profile.TotalRides)
}

func TestRepo_GetDriverProfile_NotFound(t *testing.T) {
	r, mDB := newDriverRepo(t)
	mDB.ExpectQuery("SELECT u.id, u.name").
		WithArgs(driverID).
		WillReturnError(pgx.ErrNoRows)

	_, err := r.GetDriverProfile(context.Background(), driverID)
	require.ErrorIs(t, err, ErrDriverNotFound)
}

func TestRepo_GetDriverProfile_RatingsTableUnavailable(t *testing.T) {
	r, mDB := newDriverRepo(t)
	mDB.ExpectQuery("SELECT u.id, u.name").
		WithArgs(driverID).
		WillReturnError(&pgconn.PgError{Code: "42P01", Message: "relation ratings_reviews does not exist"})
	cols := []string{
		"driver_id", "name", "email", "phone", "vehicle_type", "vehicle_plate",
		"license_number", "license_expiry", "status", "bank_name", "bank_account_number",
		"rating_avg", "total_rides",
	}
	mDB.ExpectQuery("SELECT u.id, u.name").
		WithArgs(driverID).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(
			driverID, "Budi Santoso", "budi@example.com", "081234567890", "motor", "B 1234 ABC",
			"SIM-123", "2027-12-31", "ACTIVE", "Bank Central Asia", "1234567890", 0, 0,
		))

	profile, err := r.GetDriverProfile(context.Background(), driverID)
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Zero(t, profile.RatingAvg)
	assert.Zero(t, profile.TotalRides)
}

func TestRepo_CountActiveOrders(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{"zero", 0},
		{"three", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mDB := newDriverRepo(t)
			mDB.ExpectQuery("SELECT").
				WithArgs(driverID).
				WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(tt.count))

			got, err := r.CountActiveOrders(context.Background(), driverID)
			require.NoError(t, err)
			assert.Equal(t, tt.count, got)
		})
	}
}

func TestRepo_GetAvailableRideOrders(t *testing.T) {
	r, mDB := newDriverRepo(t)
	cols := []string{
		"id", "customer_id", "pickup_address", "pickup_lat", "pickup_lng",
		"dropoff_address", "dropoff_lat", "dropoff_lng",
		"estimated_fare", "payment_method", "status", "distance_km", "created_at",
	}
	mDB.ExpectQuery("SELECT id, customer_id, pickup_address").
		WithArgs(-6.2, 106.8, 5.0).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(orderID, custID, "Jl. Pickup", -6.2, 106.8,
				"Jl. Drop", -6.3, 106.9,
				decimal.NewFromInt(20000), "WALLET", "SEARCHING_DRIVER", 1.2, "2026-08-31 10:00:00"))

	orders, err := r.GetAvailableRideOrders(context.Background(), -6.2, 106.8, 5.0)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, orderID, orders[0].ID)
	assert.Equal(t, "SEARCHING_DRIVER", orders[0].Status)
	assert.Equal(t, decimal.NewFromInt(20000), orders[0].EstimatedFare)
	assert.Equal(t, 1.2, orders[0].DistanceKm)
}

func TestRepo_GetAvailableRideOrders_Error(t *testing.T) {
	r, mDB := newDriverRepo(t)
	mDB.ExpectQuery("SELECT id, customer_id, pickup_address").
		WillReturnError(errors.New("db down"))

	_, err := r.GetAvailableRideOrders(context.Background(), -6.2, 106.8, 5.0)
	require.Error(t, err)
}

func TestRepo_GetAvailableFoodOrders(t *testing.T) {
	r, mDB := newDriverRepo(t)
	cols := []string{
		"id", "customer_id", "merchant_id", "merchant_name",
		"delivery_address", "delivery_lat", "delivery_lng",
		"payment_method", "total_amount", "status", "distance_km", "created_at",
	}
	mDB.ExpectQuery("SELECT fo.id, fo.customer_id").
		WithArgs(-6.2, 106.8, 5.0).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(orderID, custID, merchID, "Warung Nasi",
				"Jl. Tujuan", -6.2, 106.8,
				"CASH", decimal.NewFromInt(45000), "READY_FOR_PICKUP", 0.8, "2026-08-31 10:00:00"))

	orders, err := r.GetAvailableFoodOrders(context.Background(), -6.2, 106.8, 5.0)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, merchID, orders[0].MerchantID)
	assert.Equal(t, "Warung Nasi", orders[0].MerchantName)
	assert.Equal(t, decimal.NewFromInt(45000), orders[0].TotalAmount)
}

func TestRepo_GetAvailableSendOrders(t *testing.T) {
	r, mDB := newDriverRepo(t)
	cols := []string{
		"id", "sender_id", "pickup_address", "pickup_lat", "pickup_lng",
		"first_stop_address",
		"payment_method", "total_fare", "status", "distance_km", "created_at",
	}
	mDB.ExpectQuery("SELECT so.id, so.sender_id").
		WithArgs(-6.2, 106.8, 5.0).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(orderID, sendrID, "Jl. Kirim", -6.2, 106.8,
				"Jl. Stop 1",
				"WALLET", decimal.NewFromInt(30000), "SEARCHING_DRIVER", 2.0, "2026-08-31 10:00:00"))

	orders, err := r.GetAvailableSendOrders(context.Background(), -6.2, 106.8, 5.0)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, orderID, orders[0].ID)
	assert.Equal(t, "Jl. Stop 1", orders[0].FirstStopAddress)
	assert.Equal(t, decimal.NewFromInt(30000), orders[0].TotalFare)
}

func TestRepo_GetAvailableSendOrders_Error(t *testing.T) {
	r, mDB := newDriverRepo(t)
	mDB.ExpectQuery("SELECT so.id, so.sender_id").
		WillReturnError(errors.New("db down"))

	_, err := r.GetAvailableSendOrders(context.Background(), -6.2, 106.8, 5.0)
	require.Error(t, err)
}
