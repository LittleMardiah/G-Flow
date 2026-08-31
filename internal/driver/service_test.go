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
