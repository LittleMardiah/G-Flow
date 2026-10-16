package location

// Test repository SetDriverOnlineStatus (TD-186): update users.is_online
// hanya untuk driver ACTIVE; RowsAffected==0 -> pgx.ErrNoRows (404 di HTTP).

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_SetDriverOnlineStatus_True(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	driverID := uuid.New()
	mDB.ExpectExec("UPDATE users").
		WithArgs(driverID, true).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	require.NoError(t, repo.SetDriverOnlineStatus(context.Background(), driverID, true))
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_SetDriverOnlineStatus_False(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	driverID := uuid.New()
	mDB.ExpectExec("UPDATE users").
		WithArgs(driverID, false).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	require.NoError(t, repo.SetDriverOnlineStatus(context.Background(), driverID, false))
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestRepository_SetDriverOnlineStatus_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	driverID := uuid.New()
	mDB.ExpectExec("UPDATE users").
		WithArgs(driverID, true).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewRepository(mDB)
	err = repo.SetDriverOnlineStatus(context.Background(), driverID, true)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestGetNearbyDrivers_FiltersStale(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mDB.Close()

	lat, lng := -6.1751, 106.8650
	radiusKm := 5.0

	// Stale row should NOT be returned due to updated_at filter
	// But our test mocks the Query result - we just need to ensure SQL contains the filter.
	// For a real behavioral check, we would test with integration. Here just verify query.
	_ = lat
	_ = lng
	_ = radiusKm
	// We'll add a minimal assert by expecting the query with updated_at condition
	// However pgxmock ExpectQuery checks full SQL or substring; easier to match pattern
	// Just mock empty results since we only care filter exists in implementation
	mDB.ExpectQuery("updated_at >=").
		WithArgs(lat, lng, radiusKm).
		WillReturnRows(pgxmock.NewRows([]string{"driver_id", "current_lat", "current_lng", "distance_km"}))

	repo := NewRepository(mDB)
	drivers, err := repo.GetNearbyDrivers(context.Background(), lat, lng, radiusKm)
	require.NoError(t, err)
	assert.Empty(t, drivers)
	assert.NoError(t, mDB.ExpectationsWereMet())
}
