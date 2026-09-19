package food

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/wallet"
)

func foodBeginTx(t *testing.T, mDB pgxmock.PgxPoolIface) pgx.Tx {
	t.Helper()
	mDB.ExpectBegin()
	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	return tx
}

func foodLockExpect(mDB pgxmock.PgxPoolIface, tag string) {
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag(tag))
}

// settleFoodOrderTx — error branches

func TestSettleFoodOrderTx_NoDriver(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, nil)
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	err := svc.settleFoodOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrDriverNotFound)
}

func TestSettleFoodOrderTx_DriverWalletError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleFoodOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestSettleFoodOrderTx_MerchantWalletMissing(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	order.MerchantWalletID = nil
	repo := new(mockRepo)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleFoodOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestSettleFoodOrderTx_PlatformWalletError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo := new(mockRepo)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(uuid.Nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleFoodOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestSettleFoodOrderTx_InvalidPaymentMethod(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, "QRIS", &fDriverID)
	repo := new(mockRepo)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	err := svc.settleFoodOrderTx(context.Background(), nil, order)
	assert.ErrorIs(t, err, ErrInvalidPaymentMethod)
}

func TestSettleFoodOrderTx_CashLockError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_CashLedgerError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	foodLockExpect(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_CashBalanceError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	foodLockExpect(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_CashMarkSuspendedError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	foodLockExpect(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-60000)))
	repo.On("MarkDriverSuspended", mock.Anything, mock.Anything, fDriverID).
		Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_CashResetIdleError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	foodLockExpect(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.Zero))
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).
		Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_WalletEscrowError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(uuid.Nil, ErrWalletNotFound)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_WalletLockError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_WalletLedgerError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 4")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_WalletResetIdleError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 4")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).
		Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_MarkSettledError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	foodLockExpect(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.Zero))
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkFoodOrderSettled", mock.Anything, mock.Anything, fOrderID).
		Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_EventError(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodWallet, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 4")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkFoodOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).
		Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSettleFoodOrderTx_SuccessCash(t *testing.T) {
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &fDriverID)
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(fDriverWalletDef(), nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemPlatform).
		Return(fPlatformID, nil)
	foodLockExpect(mDB, "SELECT 3")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fDriverWID).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(-1000)))
	repo.On("ResetDriverIdle", mock.Anything, mock.Anything, fDriverID).Return(nil)
	repo.On("MarkFoodOrderSettled", mock.Anything, mock.Anything, fOrderID).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := NewService(repo, mDB, nil, lgr)
	err = svc.settleFoodOrderTx(context.Background(), tx, order)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// holdFoodEscrow — semua cabang

func Test_holdFoodEscrow_NoWallet(t *testing.T) {
	svc := NewService(new(mockRepo), nil, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	o.CustomerWalletID = nil
	err := svc.holdFoodEscrow(context.Background(), nil, o, decimal.NewFromInt(100))
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_holdFoodEscrow_SystemWalletNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(uuid.Nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err := svc.holdFoodEscrow(context.Background(), nil, o, decimal.NewFromInt(100))
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_holdFoodEscrow_LockError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err = svc.holdFoodEscrow(context.Background(), tx, o, decimal.NewFromInt(100))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdFoodEscrow_BalanceError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err = svc.holdFoodEscrow(context.Background(), tx, o, decimal.NewFromInt(100))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdFoodEscrow_InsufficientBalance(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(50)))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err = svc.holdFoodEscrow(context.Background(), tx, o, decimal.NewFromInt(100))
	assert.ErrorIs(t, err, ErrInsufficientBalance)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdFoodEscrow_LedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200)))
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err = svc.holdFoodEscrow(context.Background(), tx, o, decimal.NewFromInt(100))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_holdFoodEscrow_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200)))

	svc := NewService(repo, mDB, nil, lgr)
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)

	var got []wallet.LedgerEntry
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Return(nil).Run(func(args mock.Arguments) {
		got = args.Get(0).([]wallet.LedgerEntry)
	})
	err = svc.holdFoodEscrow(context.Background(), tx, o, decimal.NewFromInt(100))
	assert.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, wallet.EntryDebit, got[0].EntryType)
	assert.Equal(t, wallet.EntryCredit, got[1].EntryType)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// refundFoodEscrow — error branches

func Test_refundFoodEscrow_SystemWalletNotFound(t *testing.T) {
	repo := new(mockRepo)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(uuid.Nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err := svc.refundFoodEscrow(context.Background(), nil, o)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func Test_refundFoodEscrow_LockError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err = svc.refundFoodEscrow(context.Background(), tx, o)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func Test_refundFoodEscrow_LedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	tx := foodBeginTx(t, mDB)

	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	o := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	err = svc.refundFoodEscrow(context.Background(), tx, o)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// CreateFoodOrder — error branches

func TestCreateFoodOrder_GetCustomerError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(nil, pgx.ErrNoRows)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestCreateFoodOrder_CustomerWalletError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(nil, ErrWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestCreateFoodOrder_MerchantError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(nil, ErrMerchantNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrMerchantNotFound)
}

func TestCreateFoodOrder_MerchantWalletError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(nil, ErrMerchantWalletNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrMerchantWalletNotFound)
}

func TestCreateFoodOrder_ItemsError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return(nil, pgx.ErrNoRows)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestCreateFoodOrder_InsertOrderError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "ec-io-err"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	setupFoodDB(mDB, idemKey, fCustID)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_InsertOrderItemError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "ec-ioi-err"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	setupFoodDB(mDB, idemKey, fCustID)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_HoldEscrowSystemWalletError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "ec-esc-err"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(uuid.Nil, ErrWalletNotFound)

	setupFoodDB(mDB, idemKey, fCustID)
	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_InsertEventError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "ec-ev-err"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows)

	setupFoodDB(mDB, idemKey, fCustID)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200000)))

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_CommitError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "ec-commit-err"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(200000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupFoodDB(mDB, idemKey, fCustID)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200000)))
	mDB.ExpectCommit().WillReturnError(pgx.ErrTxClosed)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_CacheError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	idemKey := "ec-cache-err"
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(200000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)
	repo.On("InsertFoodOrder", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertFoodOrderItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	setupFoodDB(mDB, idemKey, fCustID)
	foodLockExpect(mDB, "SELECT 2")
	mDB.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(fCustWallet).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(200000)))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, fCustID, pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, idemKey))
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_IdempotencyCompleted(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	body := []byte(`{"id":"` + fOrderID.String() + `","status":"CONFIRMED","item_subtotal":100000}`)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(redisCompleted, body, nil))

	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	resp, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.NoError(t, err)
	assert.Equal(t, foodStatusConfirmed, resp.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_IdempotencyInProgress(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	future := time.Now().Add(5 * time.Minute)
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs("k", fCustID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", &future))

	repo := new(mockRepo)
	repo.On("GetCustomer", mock.Anything, fCustID).Return(fCust(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, walletTypeCustomer).
		Return(fCustWalletSvc(decimal.NewFromInt(100000)), nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fCustID, WalletTypeMerchant).
		Return(fMerchantWalletDef(), nil)
	repo.On("GetItemsByIDs", mock.Anything, mock.Anything).Return([]*Item{fItem()}, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestCreateFoodOrder_RedisInvalidCached(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	redisVal, _ := json.Marshal(redisCache{State: redisCompleted, Response: json.RawMessage(`"bad"`)})
	require.NoError(t, mr.Set(redisKey(fCustID, "k"), string(redisVal)))

	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	_, err := svc.CreateFoodOrder(context.Background(), fValidOrderReq(PaymentMethodWallet, "k"))
	assert.ErrorIs(t, err, ErrInvalidCachedResponse)
}

// UpdateFoodOrderStatus — error branches

func TestUpdateFoodOrderStatus_GetOrderError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(nil, ErrFoodOrderNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrFoodOrderNotFound)
}

func TestUpdateFoodOrderStatus_SettledOrder(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusSettled, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestUpdateFoodOrderStatus_GetMerchantError(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(nil, ErrMerchantNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrMerchantNotFound)
}

func TestUpdateFoodOrderStatus_BeginError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin().WillReturnError(pgx.ErrTxClosed)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_SetTimeoutError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_LockPlainError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(nil, errors.New("boom"))

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.Error(t, err, "boom")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_LockedStatusChanged(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	locked := fFoodOrder(foodStatusInTransit, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(locked, nil)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_UpdateRepoError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID,
		foodStatusCreated, foodStatusCancelled, mock.Anything, mock.Anything).
		Return(false, pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_EventError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID,
		foodStatusCreated, foodStatusCancelled, mock.Anything, mock.Anything).
		Return(true, nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).
		Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_CancelRefundLedgerError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID,
		foodStatusConfirmed, foodStatusCancelled, mock.Anything, mock.Anything).
		Return(true, nil)
	repo.On("SystemWalletID", mock.Anything, mock.Anything, walletTypeSystemEscrow).
		Return(fEscrowID, nil)
	foodLockExpect(mDB, "SELECT 2")
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(pgx.ErrNoRows)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_DeliverSettleError(t *testing.T) {
	repo := new(mockRepo)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	d := fDriverID
	order := fFoodOrder(foodStatusInTransit, PaymentMethodCash, &d)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID,
		foodStatusInTransit, foodStatusDelivered, mock.Anything, mock.Anything).
		Return(true, nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("GetWalletByUserAndType", mock.Anything, fDriverID, walletTypeDriver).
		Return(nil, ErrWalletNotFound)

	svc := NewService(repo, mDB, nil, new(mockLedger))
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fDriverID, Status: foodStatusDelivered,
	})
	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestUpdateFoodOrderStatus_CommitError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	order := fFoodOrder(foodStatusCreated, PaymentMethodCash, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(fMerchant(), nil)
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	repo.On("LockFoodOrder", mock.Anything, mock.Anything, fOrderID).Return(order, nil)
	repo.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything, fOrderID,
		foodStatusCreated, foodStatusCancelled, mock.Anything, mock.Anything).
		Return(true, nil)
	repo.On("InsertFoodOrderEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mDB.ExpectCommit().WillReturnError(pgx.ErrTxClosed)

	svc := NewService(repo, mDB, nil, lgr)
	_, err = svc.UpdateFoodOrderStatus(context.Background(), UpdateFoodOrderStatusRequest{
		OrderID: fOrderID, UserID: fCustID, Status: foodStatusCancelled,
	})
	assert.ErrorIs(t, err, pgx.ErrTxClosed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// GetFoodOrder / History error branches

func TestGetFoodOrder_GetOrderError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(nil, ErrFoodOrderNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.GetFoodOrder(context.Background(), fOrderID, fCustID)
	assert.ErrorIs(t, err, ErrFoodOrderNotFound)
}

func TestGetFoodOrder_GetMerchantError(t *testing.T) {
	repo := new(mockRepo)
	order := fFoodOrder(foodStatusConfirmed, PaymentMethodWallet, nil)
	repo.On("GetFoodOrderByID", mock.Anything, fOrderID).Return(order, nil)
	repo.On("GetMerchantByID", mock.Anything, fMerchID).Return(nil, ErrMerchantNotFound)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, err := svc.GetFoodOrder(context.Background(), fOrderID, fCustID)
	assert.ErrorIs(t, err, ErrMerchantNotFound)
}

func TestGetFoodOrderHistory_CountError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("CountFoodOrdersByCustomer", mock.Anything, fCustID).Return(0, pgx.ErrNoRows)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, _, err := svc.GetFoodOrderHistory(context.Background(), fCustID, 1, 20)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestGetFoodOrderHistory_ListError(t *testing.T) {
	repo := new(mockRepo)
	repo.On("CountFoodOrdersByCustomer", mock.Anything, fCustID).Return(1, nil)
	repo.On("GetFoodOrdersByCustomer", mock.Anything, fCustID, 20, 0).
		Return(nil, pgx.ErrNoRows)
	svc := NewService(repo, nil, nil, new(mockLedger))
	_, _, err := svc.GetFoodOrderHistory(context.Background(), fCustID, 1, 20)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// redisSet + cacheResponse + misc coverage

func Test_redisSet_Writes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	svc := NewService(new(mockRepo), nil, rdb, new(mockLedger))
	svc.redisSet(context.Background(), fCustID, "w", redisCompleted, json.RawMessage(`{"a":1}`))
	resp, ok := svc.redisGetCachedResp(context.Background(), fCustID, "w")
	assert.True(t, ok)
	assert.JSONEq(t, `{"a":1}`, string(resp))
}

func Test_cacheResponse_MarshalSetup(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs("k", fCustID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	svc := NewService(new(mockRepo), mDB, nil, new(mockLedger))
	err = svc.cacheResponse(context.Background(), fCustID, "k", map[string]string{"x": "1"})
	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestHandler_CodeForError_Full(t *testing.T) {
	cases := map[error]string{
		ErrMerchantNotFound:      "MERCHANT_NOT_FOUND",
		ErrMenuNotFound:          "MENU_NOT_FOUND",
		ErrItemNotFound:          "ITEM_NOT_FOUND",
		ErrFoodOrderNotFound:     "FOOD_ORDER_NOT_FOUND",
		ErrDriverNotFound:        "DRIVER_NOT_FOUND",
		ErrWalletNotFound:        "WALLET_NOT_FOUND",
		ErrNotMerchant:           "NOT_MERCHANT",
		ErrNotMerchantOwner:      "FORBIDDEN",
		ErrNotAllowed:            "FORBIDDEN",
		ErrNotCustomer:           "NOT_CUSTOMER",
		ErrMerchantInactive:      "MERCHANT_INACTIVE",
		ErrMerchantAlreadyExists: "MERCHANT_ALREADY_EXISTS",
		ErrInvalidCoordinates:    "INVALID_COORDINATES",
		ErrInvalidMenu:           "INVALID_MENU",
		ErrInvalidPrice:          "INVALID_PRICE",
		ErrEmptyName:             "INVALID_REQUEST",
		ErrCustomerInactive:      "CUSTOMER_INACTIVE",
		ErrOverdueDebt:           "OVERDUE_DEBT",
		ErrWalletInactive:        "WALLET_INACTIVE",
		ErrMerchantWalletNotFound: "MERCHANT_WALLET_NOT_FOUND",
		ErrInsufficientBalance:    "INSUFFICIENT_BALANCE",
		ErrInvalidPaymentMethod:   "INVALID_PAYMENT_METHOD",
		ErrIdempotencyKeyRequired: "IDEMPOTENCY_KEY_REQUIRED",
		ErrIdempotencyInProgress:  "IDEMPOTENCY_IN_PROGRESS",
		ErrInvalidCachedResponse:  "IDEMPOTENCY_INVALID_CACHE",
		ErrInvalidItem:            "INVALID_ITEM",
		ErrInsufficientStock:      "INSUFFICIENT_STOCK",
		ErrEmptyItems:             "EMPTY_ITEMS",
		ErrInvalidDeliveryAddress: "INVALID_DELIVERY_ADDRESS",
		ErrInvalidStatus:          "INVALID_STATUS",
		ErrInvalidTransition:      "INVALID_TRANSITION",
		ErrLockTimeout:            "LOCK_TIMEOUT",
	}
	for e, want := range cases {
		assert.Equal(t, want, codeForError(e))
	}
	assert.Equal(t, "INTERNAL_SERVER_ERROR", codeForError(errors.New("other")))
}