package wallet

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
)

// TestRepository_Create_Duplicate: unique constraint (user_id, wallet_type)
// dilanggar -> ErrWalletAlreadyExists.
func TestRepository_Create_Duplicate(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("INSERT INTO wallets").
		WithArgs(testUserID, WalletTypeCustomer).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	repo := NewRepository(mDB)
	w, err := repo.Create(context.Background(), testUserID, WalletTypeCustomer)

	assert.ErrorIs(t, err, ErrWalletAlreadyExists)
	assert.Nil(t, w)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_Create_Success: mock QueryRow return wallet baru.
func TestRepository_Create_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	w := &Wallet{ID: testWalletID, UserID: testUserID, Type: WalletTypeCustomer,
		Balance: decimal.Zero, Status: "ACTIVE"}

	mDB.ExpectQuery("INSERT INTO wallets").
		WithArgs(testUserID, WalletTypeCustomer).
		WillReturnRows(walletRowSet(w))

	repo := NewRepository(mDB)
	got, err := repo.Create(context.Background(), testUserID, WalletTypeCustomer)

	assert.NoError(t, err)
	assert.Equal(t, testWalletID, got.ID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_UpdateStatus_NotFound: 0 baris ter-update -> ErrWalletNotFound.
func TestRepository_UpdateStatus_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectExec("UPDATE wallets").
		WithArgs("SUSPENDED", testWalletID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewRepository(mDB)
	err = repo.UpdateStatus(context.Background(), testWalletID, "SUSPENDED")

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_UpdateStatus_Success: 1 baris ter-update -> nil.
func TestRepository_UpdateStatus_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectExec("UPDATE wallets").
		WithArgs("ACTIVE", testWalletID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.UpdateStatus(context.Background(), testWalletID, "ACTIVE")

	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_UpdateBalance_NotFound: 0 baris ter-update -> ErrWalletNotFound.
func TestRepository_UpdateBalance_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(10000)
	mDB.ExpectExec("UPDATE wallets").
		WithArgs(amount, testWalletID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewRepository(mDB)
	err = repo.UpdateBalance(context.Background(), testWalletID, amount)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// walletRowSet membuat pgxmock rows untuk kolom lengkap wallet.
func walletRowSet(w *Wallet) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}).
		AddRow(w.ID, w.UserID, w.Type, w.Balance, w.Status, w.CreatedAt, w.UpdatedAt)
}

// TestRepository_GetByID_Success: mock QueryRow return wallet,
// assert wallet.ID == expected.
func TestRepository_GetByID_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	w := &Wallet{
		ID:      testWalletID,
		UserID:  testUserID,
		Type:    WalletTypeCustomer,
		Balance: decimal.NewFromInt(50000),
		Status:  "ACTIVE",
	}

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testWalletID).
		WillReturnRows(walletRowSet(w))

	repo := NewRepository(mDB)
	got, err := repo.GetByID(context.Background(), testWalletID)

	assert.NoError(t, err)
	assert.Equal(t, testWalletID, got.ID)
	assert.Equal(t, testUserID, got.UserID)
	assert.Equal(t, w.Type, got.Type)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetByID_NotFound: mock QueryRow return pgx.ErrNoRows,
// assert ErrWalletNotFound.
func TestRepository_GetByID_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testWalletID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	got, err := repo.GetByID(context.Background(), testWalletID)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.Nil(t, got)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletsByUserID_Success: mock Query return 2 rows,
// assert len(wallets) == 2.
func TestRepository_GetWalletsByUserID_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	w1 := &Wallet{ID: testWalletID, UserID: testUserID, Type: WalletTypeCustomer,
		Balance: decimal.NewFromInt(10000), Status: "ACTIVE"}
	w2 := &Wallet{ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"), UserID: testUserID,
		Type: WalletTypeSystemPlatform, Balance: decimal.Zero, Status: "ACTIVE"}

	rows := pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}).
		AddRow(w1.ID, w1.UserID, w1.Type, w1.Balance, w1.Status, nil, nil).
		AddRow(w2.ID, w2.UserID, w2.Type, w2.Balance, w2.Status, nil, nil)

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testUserID).
		WillReturnRows(rows)

	repo := NewRepository(mDB)
	wallets, err := repo.GetWalletsByUserID(context.Background(), testUserID)

	assert.NoError(t, err)
	assert.Len(t, wallets, 2)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletsByUserID_Empty: mock Query return 0 rows,
// assert len(wallets) == 0.
func TestRepository_GetWalletsByUserID_Empty(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}))

	repo := NewRepository(mDB)
	wallets, err := repo.GetWalletsByUserID(context.Background(), testUserID)

	assert.NoError(t, err)
	assert.Len(t, wallets, 0)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_UpdateBalance_Success: 1 baris ter-update -> nil.
func TestRepository_UpdateBalance_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(10000)
	mDB.ExpectExec("UPDATE wallets").
		WithArgs(amount, testWalletID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	repo := NewRepository(mDB)
	err = repo.UpdateBalance(context.Background(), testWalletID, amount)

	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletsByUserID_QueryError: Query gagal -> error mentah.
func TestRepository_GetWalletsByUserID_QueryError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	wallets, err := repo.GetWalletsByUserID(context.Background(), testUserID)

	assert.Nil(t, wallets)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetByUserIDAndType_Success: mock QueryRow return wallet,
// assert wallet.Type == expected.
func TestRepository_GetByUserIDAndType_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	w := &Wallet{ID: testWalletID, UserID: testUserID, Type: WalletTypeCustomer,
		Balance: decimal.NewFromInt(50000), Status: "ACTIVE"}

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testUserID, WalletTypeCustomer).
		WillReturnRows(walletRowSet(w))

	repo := NewRepository(mDB)
	got, err := repo.GetByUserIDAndType(context.Background(), testUserID, WalletTypeCustomer)

	assert.NoError(t, err)
	assert.Equal(t, WalletTypeCustomer, got.Type)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetByUserIDAndType_NotFound: mock QueryRow return
// pgx.ErrNoRows, assert ErrWalletNotFound.
func TestRepository_GetByUserIDAndType_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(testUserID, WalletTypeCustomer).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	got, err := repo.GetByUserIDAndType(context.Background(), testUserID, WalletTypeCustomer)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.Nil(t, got)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetBalance_Success: mock QueryRow return balance,
// assert balance > 0.
func TestRepository_GetBalance_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	balance := decimal.NewFromInt(75000)
	mDB.ExpectQuery("SELECT balance").
		WithArgs(testWalletID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(balance))

	repo := NewRepository(mDB)
	got, err := repo.GetBalance(context.Background(), testWalletID)

	assert.NoError(t, err)
	assert.True(t, got.GreaterThan(decimal.Zero), "balance harus > 0, got %v", got)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetBalance_NotFound: mock QueryRow return pgx.ErrNoRows,
// assert ErrWalletNotFound.
func TestRepository_GetBalance_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT balance").
		WithArgs(testWalletID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	got, err := repo.GetBalance(context.Background(), testWalletID)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.True(t, got.IsZero())
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletOwner_Success: mock QueryRow return user_id,
// assert owner == expected.
func TestRepository_GetWalletOwner_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT user_id").
		WithArgs(testWalletID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(testUserID))

	repo := NewRepository(mDB)
	owner, err := repo.GetWalletOwner(context.Background(), testWalletID)

	assert.NoError(t, err)
	assert.Equal(t, testUserID, owner)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletOwner_NotFound: mock QueryRow return pgx.ErrNoRows,
// assert ErrWalletNotFound.
func TestRepository_GetWalletOwner_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT user_id").
		WithArgs(testWalletID).
		WillReturnError(pgx.ErrNoRows)

	repo := NewRepository(mDB)
	owner, err := repo.GetWalletOwner(context.Background(), testWalletID)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.True(t, owner == uuid.Nil)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletOwner_QueryError: error lain -> error mentah.
func TestRepository_GetWalletOwner_QueryError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT user_id").
		WithArgs(testWalletID).
		WillReturnError(errors.New("conn refused"))

	repo := NewRepository(mDB)
	owner, err := repo.GetWalletOwner(context.Background(), testWalletID)

	assert.Equal(t, errors.New("conn refused").Error(), err.Error())
	assert.True(t, owner == uuid.Nil)
	assert.NoError(t, mDB.ExpectationsWereMet())
}
