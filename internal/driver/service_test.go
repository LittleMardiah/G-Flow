package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepo adalah stub Repo untuk unit test service.
type mockRepo struct {
	lat, lng       float64
	locErr         error
	activeOrders   int
	activeErr      error
	rides          []RideAvailableOrder
	ridesErr       error
	foods          []FoodAvailableOrder
	foodsErr       error
	sends          []SendAvailableOrder
	sendsErr       error
	locationCalled bool
	countCalled    bool
	rideCalled     bool
	foodCalled     bool
	sendCalled     bool
	activeRides    []ActiveRideOrder
	activeFoods    []ActiveFoodOrder
	activeSends    []ActiveSendOrder
	activeOrderErr error
	activeRideCall bool
	activeFoodCall bool
	activeSendCall bool
	profile        *DriverProfile
	profileErr     error
	profileCall    bool
}

func (m *mockRepo) GetDriverLocation(_ context.Context, _ uuid.UUID) (float64, float64, error) {
	m.locationCalled = true
	return m.lat, m.lng, m.locErr
}

func (m *mockRepo) CountActiveOrders(_ context.Context, _ uuid.UUID) (int, error) {
	m.countCalled = true
	return m.activeOrders, m.activeErr
}

func (m *mockRepo) GetAvailableRideOrders(_ context.Context, _, _, _ float64) ([]RideAvailableOrder, error) {
	m.rideCalled = true
	return m.rides, m.ridesErr
}

func (m *mockRepo) GetAvailableFoodOrders(_ context.Context, _, _, _ float64) ([]FoodAvailableOrder, error) {
	m.foodCalled = true
	return m.foods, m.foodsErr
}

func (m *mockRepo) GetAvailableSendOrders(_ context.Context, _, _, _ float64) ([]SendAvailableOrder, error) {
	m.sendCalled = true
	return m.sends, m.sendsErr
}

func (m *mockRepo) GetActiveRideOrders(_ context.Context, _ uuid.UUID) ([]ActiveRideOrder, error) {
	m.activeRideCall = true
	return m.activeRides, m.activeOrderErr
}

func (m *mockRepo) GetActiveFoodOrders(_ context.Context, _ uuid.UUID) ([]ActiveFoodOrder, error) {
	m.activeFoodCall = true
	return m.activeFoods, m.activeOrderErr
}

func (m *mockRepo) GetActiveSendOrders(_ context.Context, _ uuid.UUID) ([]ActiveSendOrder, error) {
	m.activeSendCall = true
	return m.activeSends, m.activeOrderErr
}

func (m *mockRepo) GetDriverProfile(_ context.Context, _ uuid.UUID) (*DriverProfile, error) {
	m.profileCall = true
	return m.profile, m.profileErr
}

// mockRedis adalah stub RedisClient untuk unit test service.
type mockRedis struct {
	lat, lng string
	err      error
}

func (m *mockRedis) HGet(_ context.Context, _ string, field string) *redis.StringCmd {
	if m.err != nil {
		return redis.NewStringResult("", m.err)
	}
	if field == fieldLat {
		return redis.NewStringResult(m.lat, nil)
	}
	return redis.NewStringResult(m.lng, nil)
}

func redisRepo() *mockRepo {
	return &mockRepo{
		lat: -6.2, lng: 106.8,
		activeOrders: 1,
		rides: []RideAvailableOrder{{
			ID: orderID, CustomerID: custID, PickupAddress: "Jl. Pickup",
			PickupLat: -6.2, PickupLng: 106.8, DropoffAddress: "Jl. Drop",
			EstimatedFare: decimal.NewFromInt(20000), PaymentMethod: "WALLET",
			Status: "SEARCHING_DRIVER", DistanceKm: 1.2, CreatedAt: "2026-08-31 10:00:00",
		}},
		foods: []FoodAvailableOrder{{
			ID: orderID, CustomerID: custID, MerchantID: merchID, MerchantName: "Warung",
			DeliveryAddress: "Jl. Tujuan", DeliveryLat: -6.2, DeliveryLng: 106.8,
			TotalAmount: decimal.NewFromInt(45000), PaymentMethod: "CASH",
			Status: "READY_FOR_PICKUP", DistanceKm: 0.8, CreatedAt: "2026-08-31 10:00:00",
		}},
	}
}

func TestService_GetAvailableOrders_FromRedis(t *testing.T) {
	repo := redisRepo()
	svc := NewService(repo, &mockRedis{lat: "-6.2", lng: "106.8"})

	res, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.NoError(t, err)
	require.False(t, repo.locationCalled) // lokasi dibaca dari Redis, bukan DB
	require.True(t, repo.rideCalled)
	require.True(t, repo.foodCalled)
	require.True(t, repo.sendCalled)

	assert.True(t, res.CapacityAvailable)
	assert.Equal(t, 1, res.ActiveOrders)
	assert.Equal(t, maxActiveOrders, res.MaxActiveOrders)

	require.Len(t, res.Orders, 2)
	assert.Equal(t, OrderTypeRide, res.Orders[0].Type)
	assert.True(t, res.Orders[0].Earning.Equal(decimal.NewFromFloat(16000))) // 20000*0.8
	assert.Equal(t, OrderTypeFood, res.Orders[1].Type)
	assert.Equal(t, "Warung", res.Orders[1].MerchantName)
}

func TestService_GetAvailableOrders_FallbackRepo_WhenNoRedis(t *testing.T) {
	repo := redisRepo()
	svc := NewService(repo, nil)

	res, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.NoError(t, err)
	assert.True(t, repo.locationCalled) // fallback ke repo.GetDriverLocation
	require.Len(t, res.Orders, 2)
}

func TestService_GetAvailableOrders_CapacityFull(t *testing.T) {
	repo := redisRepo()
	repo.activeOrders = 3
	svc := NewService(repo, &mockRedis{lat: "-6.2", lng: "106.8"})

	res, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.NoError(t, err)
	assert.False(t, res.CapacityAvailable)
	assert.Equal(t, 3, res.ActiveOrders)
	// Order tetap dikembalikan walau kapasitas penuh (daftar tampil, accept diblokir).
	require.Len(t, res.Orders, 2)
}

func TestService_GetAvailableOrders_NoLocation(t *testing.T) {
	repo := &mockRepo{locErr: ErrDriverLocationNotFound}
	svc := NewService(repo, nil)

	_, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.ErrorIs(t, err, ErrDriverLocationUnavailable)
}

func TestService_GetAvailableOrders_InvalidCoordinates(t *testing.T) {
	repo := &mockRepo{lat: 999, lng: 999}
	svc := NewService(repo, nil)

	_, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.ErrorIs(t, err, ErrInvalidCoordinates)
}

func TestService_GetAvailableOrders_CountError(t *testing.T) {
	repo := redisRepo()
	repo.activeErr = errors.New("db down")
	repo.activeOrders = 0
	svc := NewService(repo, &mockRedis{lat: "-6.2", lng: "106.8"})

	_, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.Error(t, err)
}

func TestService_GetAvailableOrders_OrderError(t *testing.T) {
	repo := redisRepo()
	repo.ridesErr = errors.New("db down")
	svc := NewService(repo, &mockRedis{lat: "-6.2", lng: "106.8"})

	_, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.Error(t, err)
}

func TestService_GetAvailableOrders_SendEarning(t *testing.T) {
	repo := redisRepo()
	repo.rides = nil
	repo.foods = nil
	repo.sends = []SendAvailableOrder{{
		ID: orderID, SenderID: sendrID, PickupAddress: "Jl. Kirim",
		FirstStopAddress: "Jl. Stop 1", TotalFare: decimal.NewFromInt(30000),
		PaymentMethod: "WALLET", Status: "SEARCHING_DRIVER", DistanceKm: 2.0,
		CreatedAt: "2026-08-31 10:00:00",
	}}
	svc := NewService(repo, &mockRedis{lat: "-6.2", lng: "106.8"})

	res, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.NoError(t, err)
	require.Len(t, res.Orders, 1)
	assert.Equal(t, OrderTypeSend, res.Orders[0].Type)
	assert.Equal(t, "Jl. Stop 1", res.Orders[0].StopAddress)
	assert.True(t, res.Orders[0].Earning.Equal(decimal.NewFromFloat(27000))) // 30000*0.9
}

func TestService_GetAvailableOrders_Empty(t *testing.T) {
	repo := redisRepo()
	repo.rides = nil
	repo.foods = nil
	repo.sends = nil
	svc := NewService(repo, &mockRedis{lat: "-6.2", lng: "106.8"})

	res, err := svc.GetAvailableOrders(context.Background(), driverID)
	require.NoError(t, err)
	assert.NotNil(t, res.Orders)
	assert.Len(t, res.Orders, 0) // slice kosong (bukan nil) agar JSON []
}

// ---- TD-077 A2: GetActiveOrders (service) ----

// TestService_GetActiveOrders_Empty: tanpa order aktif → [] (slice kosong,
// bukan nil) tanpa error; ketiga repo di-query.
func TestService_GetActiveOrders_Empty(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo, nil)

	res, err := svc.GetActiveOrders(context.Background(), driverID)
	require.NoError(t, err)
	require.True(t, repo.activeRideCall)
	require.True(t, repo.activeFoodCall)
	require.True(t, repo.activeSendCall)
	assert.NotNil(t, res.Orders)
	assert.Len(t, res.Orders, 0)
}

// TestService_GetActiveOrders_Mixed: union 1 ride + 1 food + 1 send → 3 item,
// urutan ride→food→send, tiap item memetakan field ke struktur DTO benar.
func TestService_GetActiveOrders_Mixed(t *testing.T) {
	foodID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	sendID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	repo := &mockRepo{
		activeRides: []ActiveRideOrder{{
			ID: orderID, DriverID: driverID, PickupAddress: "Jl. Pickup",
			PickupLat: -6.2, PickupLng: 106.8, DropoffAddress: "Jl. Drop",
			DropoffLat: -6.3, DropoffLng: 106.9, DistanceKm: 3.5,
			EstimatedFare: decimal.NewFromInt(24000), PaymentMethod: "WALLET",
			Status: "TRIP_STARTED", CreatedAt: "2026-08-31 10:00:00",
		}},
		activeFoods: []ActiveFoodOrder{{
			ID: foodID, DriverID: driverID, MerchantName: "Warung",
			MerchantAddress: "Jl. Warung", DeliveryAddress: "Jl. Tujuan",
			DeliveryFee: decimal.NewFromInt(20000), TotalAmount: decimal.NewFromInt(45000),
			PaymentMethod: "CASH", Status: "PICKED_UP", CreatedAt: "2026-08-31 10:00:00",
		}},
		activeSends: []ActiveSendOrder{{
			ID: sendID, DriverID: driverID, PickupAddress: "Jl. Kirim",
			DistanceKm: 2.0, TotalFare: decimal.NewFromInt(30000),
			PaymentMethod: "WALLET", Status: "IN_TRANSIT", CreatedAt: "2026-08-31 10:00:00",
			FirstStopAddress: ptrString("Jl. Stop 1"),
		}},
	}
	svc := NewService(repo, nil)

	res, err := svc.GetActiveOrders(context.Background(), driverID)
	require.NoError(t, err)
	require.Len(t, res.Orders, 3)

	ride := res.Orders[0]
	assert.Equal(t, OrderTypeRide, ride.Type)
	assert.Equal(t, orderID, ride.OrderID)
	assert.Equal(t, "TRIP_STARTED", ride.Status)
	assert.Equal(t, "Jl. Drop", ride.DropoffAddress)
	assert.Equal(t, decimal.NewFromInt(24000), ride.EstimatedFare)
	// Field milik tipe lain tidak terisi.
	assert.Empty(t, ride.MerchantName)

	food := res.Orders[1]
	assert.Equal(t, OrderTypeFood, food.Type)
	assert.Equal(t, foodID, food.OrderID)
	assert.Equal(t, "Warung", food.MerchantName)
	assert.Equal(t, decimal.NewFromInt(45000), food.TotalAmount)
	assert.Empty(t, food.EstimatedFare)
	assert.Empty(t, food.TotalFare)

	send := res.Orders[2]
	assert.Equal(t, OrderTypeSend, send.Type)
	assert.Equal(t, sendID, send.OrderID)
	assert.Equal(t, "Jl. Stop 1", send.FirstStopAddress)
	assert.Equal(t, decimal.NewFromInt(30000), send.TotalFare)
	assert.Empty(t, send.EstimatedFare)
}

// TestService_GetActiveOrders_SendNoStop: send tanpa stop tersimpan → alamat
// stop pertama string kosong (bukan crash / mengisi field lain).
func TestService_GetActiveOrders_SendNoStop(t *testing.T) {
	repo := &mockRepo{
		activeSends: []ActiveSendOrder{{
			ID: orderID, DriverID: driverID, PickupAddress: "Jl. Kirim",
			TotalFare: decimal.NewFromInt(30000), PaymentMethod: "WALLET",
			Status: "DRIVER_ASSIGNED", CreatedAt: "2026-08-31 10:00:00",
		}},
	}
	svc := NewService(repo, nil)

	res, err := svc.GetActiveOrders(context.Background(), driverID)
	require.NoError(t, err)
	require.Len(t, res.Orders, 1)
	assert.Equal(t, "", res.Orders[0].FirstStopAddress)
}

// TestService_GetActiveOrders_Error: error salah satu repo → di-propagate.
func TestService_GetActiveOrders_Error(t *testing.T) {
	repo := &mockRepo{activeOrderErr: errors.New("db down")}
	svc := NewService(repo, nil)

	_, err := svc.GetActiveOrders(context.Background(), driverID)
	require.Error(t, err)
}

func TestService_GetDriverProfile(t *testing.T) {
	want := &DriverProfile{DriverID: driverID, Name: "Budi"}
	repo := &mockRepo{profile: want}
	svc := NewService(repo, nil)

	got, err := svc.GetDriverProfile(context.Background(), driverID)
	require.NoError(t, err)
	assert.Same(t, want, got)
	assert.True(t, repo.profileCall)
}

func ptrString(s string) *string { return &s }
