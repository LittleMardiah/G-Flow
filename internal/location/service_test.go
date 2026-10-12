package location

// Test service SetOnlineStatus (TD-186): update is_online di repo; ketika
// offline, key Redis driver:location:{id} harus terhapus (instant offline);
// ketika online, key tidak disentuh (driver kirim lokasi lagi via UpdateLocation).

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) UpsertLocation(ctx context.Context, driverID uuid.UUID, lat, lng float64) error {
	args := m.Called(ctx, driverID, lat, lng)
	return args.Error(0)
}

func (m *mockRepo) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, error) {
	args := m.Called(ctx, lat, lng, radiusKm)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]NearbyDriver), args.Error(1)
}

func (m *mockRepo) SetDriverOnlineStatus(ctx context.Context, driverID uuid.UUID, isOnline bool) error {
	args := m.Called(ctx, driverID, isOnline)
	return args.Error(0)
}

// sebRedis membuat *redis.Client berbasis miniredis agar Service memakai
// Redis nyata (DEL benar-benar dieksekusi) pada test.
func sebRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mini := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mini, rdb
}

func TestService_SetOnlineStatus_OfflineDeletesRedis(t *testing.T) {
	mr := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	_, rdb := sebRedis(t)
	driverID := uuid.New()
	key := redisKey(driverID)
	require.NoError(t, rdb.HSet(context.Background(), key, "lat", "-6.2", "lng", "106.8").Err())

	mr.On("SetDriverOnlineStatus", mock.Anything, driverID, false).Return(nil)

	svc := NewService(mr, rdb, mDB)
	require.NoError(t, svc.SetOnlineStatus(context.Background(), driverID, false))

	mr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
	assert.Equal(t, int64(0), rdb.Exists(context.Background(), key).Val(), "key offline harus terhapus")
}

func TestService_SetOnlineStatus_OnlineNoRedisCleanup(t *testing.T) {
	mr := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	_, rdb := sebRedis(t)
	driverID := uuid.New()
	key := redisKey(driverID)
	require.NoError(t, rdb.HSet(context.Background(), key, "lat", "-6.2", "lng", "106.8").Err())

	mr.On("SetDriverOnlineStatus", mock.Anything, driverID, true).Return(nil)

	svc := NewService(mr, rdb, mDB)
	require.NoError(t, svc.SetOnlineStatus(context.Background(), driverID, true))

	mr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
	assert.Equal(t, int64(1), rdb.Exists(context.Background(), key).Val(), "key online harus tetap ada")
}

func TestService_SetOnlineStatus_NoRowsMapsToErrDriverNotFound(t *testing.T) {
	mr := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	driverID := uuid.New()
	mr.On("SetDriverOnlineStatus", mock.Anything, driverID, true).Return(pgx.ErrNoRows)

	svc := NewService(mr, nil, mDB)
	err = svc.SetOnlineStatus(context.Background(), driverID, true)
	require.ErrorIs(t, err, ErrDriverNotFound)
	mr.AssertExpectations(t)
}

func TestService_SetOnlineStatus_RepoErrorPassthrough(t *testing.T) {
	mr := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	driverID := uuid.New()
	sentinel := errors.New("db unreachable")
	mr.On("SetDriverOnlineStatus", mock.Anything, driverID, true).Return(sentinel)

	svc := NewService(mr, nil, mDB)
	err = svc.SetOnlineStatus(context.Background(), driverID, true)
	require.ErrorIs(t, err, sentinel)
	mr.AssertExpectations(t)
}
