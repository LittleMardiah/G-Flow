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
