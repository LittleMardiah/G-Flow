package wallet

import (
	"context"
	"encoding/json"
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
)

// ---- mocks ----

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) GetByUserIDAndType(ctx context.Context, userID uuid.UUID, walletType string) (*Wallet, error) {
	args := m.Called(ctx, userID, walletType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Wallet), args.Error(1)
}

func (m *mockRepo) GetByID(ctx context.Context, walletID uuid.UUID) (*Wallet, error) {
	args := m.Called(ctx, walletID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Wallet), args.Error(1)
}

func (m *mockRepo) GetBalance(ctx context.Context, walletID uuid.UUID) (decimal.Decimal, error) {
	args := m.Called(ctx, walletID)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

func (m *mockRepo) GetWalletOwner(ctx context.Context, walletID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, walletID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *mockRepo) ListLedgerEntries(ctx context.Context, walletID uuid.UUID, refType string, limit, offset int) ([]LedgerEntry, error) {
	args := m.Called(ctx, walletID, refType, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]LedgerEntry), args.Error(1)
}

func (m *mockRepo) CountLedgerEntries(ctx context.Context, walletID uuid.UUID, refType string) (int, error) {
	args := m.Called(ctx, walletID, refType)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) UpdateWalletStatus(ctx context.Context, walletID uuid.UUID, newStatus string) (time.Time, error) {
	args := m.Called(ctx, walletID, newStatus)
	return args.Get(0).(time.Time), args.Error(1)
}

type mockLedger struct {
	mock.Mock
}

func (m *mockLedger) CreateLedgerEntries(_ context.Context, _ pgx.Tx, entries []LedgerEntry) error {
	args := m.Called(entries)
	return args.Error(0)
}

var (
	testUserID   = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testWalletID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	testBankID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

func newCustomerWallet(balance decimal.Decimal) *Wallet {
	return &Wallet{
		ID:      testWalletID,
		UserID:  testUserID,
		Type:    WalletTypeCustomer,
		Balance: balance,
		Status:  "ACTIVE",
	}
}

// ---- Test 1: TopUp success ----

func TestTopUpSuccess(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(100000)
	idemKey := "topup-key-1"

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.Zero), nil)
	repo.On("GetBalance", mock.Anything, testWalletID).
		Return(amount, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).
		Return(nil)

	// overdue_debt: 0 (tidak ada hutang).
	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(decimal.Zero))
	// kyc_limit: UNVERIFIED (limit 2jt; amount 100rb -> OK)
	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow("UNVERIFIED"))
	// idemAcquire -> no rows -> insert PROCESSING
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, testUserID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	// tx
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemBankGateway).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testBankID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectExec("INSERT INTO topup_transactions").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectExec("UPDATE topup_transactions").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit()
	// cache: update L2 COMPLETED
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, testUserID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         amount,
		IdempotencyKey: idemKey,
	})

	assert.NoError(t, err)
	assert.Equal(t, redisCompleted, resp.Status)
	assert.Equal(t, testWalletID, resp.WalletID)
	assert.True(t, resp.NewBalance.Equal(amount))
	assert.NotEqual(t, uuid.Nil, resp.TransactionID)

	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- GetMyWallet (TD-120) ----

func TestService_GetMyWallet_Success(t *testing.T) {
	repo := new(mockRepo)
	svc := NewService(repo, nil, nil, nil)

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.NewFromInt(150000)), nil)

	got, err := svc.GetMyWallet(context.Background(), testUserID, WalletTypeCustomer)

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, testWalletID, got.WalletID)
	assert.Equal(t, WalletStatusActive, got.Status)
	assert.Equal(t, WalletTypeCustomer, got.WalletType)
	assert.Equal(t, "150000", got.Balance.String())
	repo.AssertExpectations(t)
}

func TestService_GetMyWallet_NotFound(t *testing.T) {
	repo := new(mockRepo)
	svc := NewService(repo, nil, nil, nil)

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(nil, ErrWalletNotFound)

	got, err := svc.GetMyWallet(context.Background(), testUserID, WalletTypeCustomer)

	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrWalletNotFound)
	repo.AssertExpectations(t)
}

func TestService_GetMyWallet_DriverType(t *testing.T) {
	repo := new(mockRepo)
	svc := NewService(repo, nil, nil, nil)

	driverWallet := &Wallet{
		ID:      uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		UserID:  testUserID,
		Type:    WalletTypeDriver,
		Balance: decimal.NewFromInt(900000),
		Status:  WalletStatusActive,
	}
	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeDriver).
		Return(driverWallet, nil)

	got, err := svc.GetMyWallet(context.Background(), testUserID, WalletTypeDriver)

	assert.NoError(t, err)
	assert.Equal(t, WalletTypeDriver, got.WalletType)
	assert.Equal(t, "900000", got.Balance.String())
	repo.AssertExpectations(t)
}

// ---- Test 2: TopUp idempotency duplicate (L2 COMPLETED cached) ----

func TestTopUpIdempotencyDuplicate(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(100000)
	idemKey := "topup-key-dup"

	cached := TopUpResponse{
		TransactionID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		WalletID:      testWalletID,
		Amount:        amount,
		NewBalance:    amount,
		Status:        redisCompleted,
	}
	cachedJSON, _ := json.Marshal(cached)

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.Zero), nil)

	// overdue_debt: 0 (tidak ada hutang).
	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(decimal.Zero))
	// kyc limit query dijalankan sebelum idemAcquire.
	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow("VERIFIED"))
	// L2 returns COMPLETED cached -> tidak perlu tx.
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(redisCompleted, cachedJSON, nil))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         amount,
		IdempotencyKey: idemKey,
	})

	assert.NoError(t, err)
	assert.Equal(t, cached.TransactionID, resp.TransactionID)
	assert.Equal(t, cached.Status, resp.Status)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test 3: Transfer insufficient balance ----

func TestTransferInsufficientBalance(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	fromID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	toID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	fromWallet := &Wallet{
		ID:      fromID,
		UserID:  testUserID,
		Type:    WalletTypeCustomer,
		Balance: decimal.NewFromInt(100), // saldo kecil
		Status:  "ACTIVE",
	}
	toWallet := &Wallet{
		ID:      toID,
		UserID:  uuid.MustParse("99999999-9999-9999-9999-999999999999"),
		Type:    WalletTypeCustomer,
		Balance: decimal.Zero,
		Status:  "ACTIVE",
	}

	repo.On("GetByID", mock.Anything, fromID).Return(fromWallet, nil)
	repo.On("GetByID", mock.Anything, toID).Return(toWallet, nil)

	idemKey := "transfer-key-1"
	// idemAcquire -> no rows -> insert PROCESSING
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, testUserID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	// tx
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	// lock both wallets ascending
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	// reload from & to balances (urutan: dari lalu ke)
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(fromID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}).
			AddRow(fromID, testUserID, WalletTypeCustomer, fromWallet.Balance, "ACTIVE", nil, nil))
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(toID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}).
			AddRow(toID, toWallet.UserID, WalletTypeCustomer, toWallet.Balance, "ACTIVE", nil, nil))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.Transfer(context.Background(), TransferRequest{
		UserID:         testUserID, // pemilik wallet (dari JWT claim)
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         decimal.NewFromInt(500), // > saldo 100
		IdempotencyKey: idemKey,
		Description:    "p2p",
	})

	assert.ErrorIs(t, err, ErrInsufficientBalance)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test 4: TopUp dengan overdue debt (Bug #47) ----

var testPlatformID = uuid.MustParse("55555555-5555-5555-5555-555555555555")

func ledgerHasReferenceType(entries []LedgerEntry, want string) bool {
	for _, e := range entries {
		if e.ReferenceType == want {
			return true
		}
	}
	return false
}

func TestTopUpWithOverdueDebt(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(100000) // topup 100rb
	debt := decimal.NewFromInt(30000)    // hutang 30rb
	idemKey := "topup-key-debt"

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.Zero), nil)
	repo.On("GetBalance", mock.Anything, testWalletID).
		Return(decimal.NewFromInt(40000), nil)
	// 4 entry: OVERDUE_SETTLEMENT (debit customer, credit platform) + TOPUP net 70rb.
	lgr.On("CreateLedgerEntries", mock.MatchedBy(func(entries []LedgerEntry) bool {
		return len(entries) == 4 &&
			ledgerHasReferenceType(entries, referenceTypeOverdueSettlement) &&
			ledgerHasReferenceType(entries, referenceTypeTopUp)
	})).Return(nil)

	// overdue_debt = 30rb.
	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(debt))
	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow("UNVERIFIED"))
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, testUserID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	// system wallets: bank gateway lalu platform.
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemBankGateway).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testBankID))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemPlatform).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testPlatformID))
	// lock 3 wallet (customer, bank, platform).
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	// hutang lunas -> overdue_debt = 0.
	mDB.ExpectExec("UPDATE users SET overdue_debt").
		WithArgs(pgxmock.AnyArg(), testUserID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO topup_transactions").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectExec("UPDATE topup_transactions").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, testUserID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         amount,
		IdempotencyKey: idemKey,
	})

	assert.NoError(t, err)
	assert.Equal(t, redisCompleted, resp.Status)
	assert.Equal(t, amount, resp.Amount)
	assert.True(t, resp.DebtSettled.Equal(debt), "debt_settled = %v, want %v", resp.DebtSettled, debt)
	assert.True(t, resp.WalletCredited.Equal(decimal.NewFromInt(70000)), "wallet_credited = %v", resp.WalletCredited)
	assert.True(t, resp.NewBalance.Equal(decimal.NewFromInt(40000)))

	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestTopUpPartialDebtSettlement(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(20000) // topup < hutang
	debt := decimal.NewFromInt(30000)
	idemKey := "topup-key-debt-partial"

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.Zero), nil)
	repo.On("GetBalance", mock.Anything, testWalletID).
		Return(decimal.Zero, nil)
	// Hanya OVERDUE_SETTLEMENT: seluruh amount dipakai bayar hutang (net = 0).
	lgr.On("CreateLedgerEntries", mock.MatchedBy(func(entries []LedgerEntry) bool {
		return len(entries) == 2 &&
			ledgerHasReferenceType(entries, referenceTypeOverdueSettlement) &&
			!ledgerHasReferenceType(entries, referenceTypeTopUp)
	})).Return(nil)

	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(debt))
	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow("UNVERIFIED"))
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, testUserID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemBankGateway).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testBankID))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemPlatform).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testPlatformID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 3"))
	// sisa hutang setelah pembayaran 20rb -> 10rb.
	mDB.ExpectExec("UPDATE users SET overdue_debt").
		WithArgs(pgxmock.AnyArg(), testUserID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectExec("INSERT INTO topup_transactions").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectExec("UPDATE topup_transactions").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, testUserID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         amount,
		IdempotencyKey: idemKey,
	})

	assert.NoError(t, err)
	assert.True(t, resp.DebtSettled.Equal(amount), "debt_settled = %v, want %v", resp.DebtSettled, amount)
	assert.True(t, resp.WalletCredited.IsZero(), "wallet_credited = %v, want 0", resp.WalletCredited)
	assert.True(t, resp.NewBalance.IsZero())

	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test TopUp: KYC balance limit exceeded ----

func TestTopUp_KycLimitExceeded(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	// UNVERIFIED limit = 2jt. Saldo wallet sudah 1.5jt, topup 1jt
	// => net 2.5jt melebihi 2jt -> ditolak sebelum idemAcquire/begintx.
	amount := decimal.NewFromInt(1000000)
	idemKey := "topup-key-kyc-limit"

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.NewFromInt(1500000)), nil)

	// overdue_debt: 0 -> netTopUp = full amount.
	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(decimal.Zero))
	// kyc_status: UNVERIFIED -> limit 2jt.
	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow(kycUnverified))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         amount,
		IdempotencyKey: idemKey,
	})

	assert.ErrorIs(t, err, ErrKycLimitExceeded)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test TopUp: idempotency still processing (fresh PROCESSING) ----

// func TestTopUp_IdempotencyProcessing(t *testing.T) {
// 	repo := new(mockRepo)
// 	lgr := new(mockLedger)
// 	mDB, err := pgxmock.NewPool()
// 	assert.NoError(t, err)
//
// 	amount := decimal.NewFromInt(100000)
// 	idemKey := "topup-key-processing"
//
// 	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
// 		Return(newCustomerWallet(decimal.Zero), nil)
//
// 	// overdue_debt: 0.
// 	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
// 		WithArgs(testUserID).
// 		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(decimal.Zero))
// 	// kyc limit: VERIFIED (20jt, tidak kena limit).
// 	mDB.ExpectQuery("kyc_status").
// 		WithArgs(testUserID).
// 		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow(kycVerified))
// 	// L2 returns PROCESSING fresh (debounce_at di masa depan) -> in progress.
// 	future := time.Now().Add(5 * time.Minute)
// 	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
// 		WithArgs(idemKey, testUserID).
// 		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
// 			AddRow(pgProcessing, "{}", future))
//
// 	svc := NewService(repo, lgr, nil, mDB)
// 	_, err = svc.TopUp(context.Background(), TopUpRequest{
// 		UserID:         testUserID,
// 		Amount:         amount,
// 		IdempotencyKey: idemKey,
// 	})
//
// 	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
// 	repo.AssertExpectations(t)
// 	assert.NoError(t, mDB.ExpectationsWereMet())
// }

// ---- Test Transfer: self transfer ditolak ----

func TestTransfer_SelfTransfer(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.Transfer(context.Background(), TransferRequest{
		UserID:         testUserID,
		FromWalletID:   testWalletID,
		ToWalletID:     testWalletID,
		Amount:         decimal.NewFromInt(50000),
		IdempotencyKey: "self-transfer",
	})

	assert.ErrorIs(t, err, ErrSameWalletTransfer)
	repo.AssertNotCalled(t, "GetByID")
}

// ---- Test 5: Transfer dari wallet milik user lain ditolak ----

func TestTransferWalletNotOwned(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	fromID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	toID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	otherOwner := uuid.MustParse("99999999-9999-9999-9999-999999999999")

	fromWallet := &Wallet{
		ID:      fromID,
		UserID:  otherOwner, // wallet milik user lain
		Type:    WalletTypeCustomer,
		Balance: decimal.NewFromInt(100000),
		Status:  "ACTIVE",
	}
	toWallet := &Wallet{
		ID:      toID,
		UserID:  uuid.MustParse("88888888-8888-8888-8888-888888888888"),
		Type:    WalletTypeCustomer,
		Balance: decimal.Zero,
		Status:  "ACTIVE",
	}

	repo.On("GetByID", mock.Anything, fromID).Return(fromWallet, nil)
	repo.On("GetByID", mock.Anything, toID).Return(toWallet, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.Transfer(context.Background(), TransferRequest{
		UserID:         testUserID, // bukan pemilik fromID
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         decimal.NewFromInt(50000),
		IdempotencyKey: "transfer-unauthorized",
	})

	assert.ErrorIs(t, err, ErrWalletNotOwned)
	repo.AssertExpectations(t)
}

// ---- Test Service: ProcessTopUpWebhook ----

// TestService_ProcessTopUpWebhook_Success: panggil ProcessTopUpWebhook dengan
// txnID random. Tidak ada baris PENDING -> markTopUpCompleted log warning dan
// return nil (idempotent).
func TestService_ProcessTopUpWebhook_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)

	txnID := uuid.New()
	mDB.ExpectExec("UPDATE topup_transactions").
		WithArgs(txnID).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 0")) // 0 rows -> idempotent

	err = svc.ProcessTopUpWebhook(context.Background(), txnID)

	assert.NoError(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
	repo.AssertNotCalled(t, "GetBalance")
}

// ---- Test Service: GetBalance ----

// TestService_GetBalance_Success: wallet milik user, mock owner & GetBalance
// return balance, assert balance > 0.
func TestService_GetBalance_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	balance := decimal.NewFromInt(120000)
	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(testUserID, nil)
	repo.On("GetBalance", mock.Anything, testWalletID).Return(balance, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetBalance(context.Background(), testUserID, testWalletID)

	assert.NoError(t, err)
	assert.True(t, got.GreaterThan(decimal.Zero), "balance harus > 0, got %v", got)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetBalance_NotOwned: wallet milik user lain -> ErrWalletNotOwned (403).
func TestService_GetBalance_NotOwned(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	otherUser := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(otherUser, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetBalance(context.Background(), testUserID, testWalletID)

	assert.ErrorIs(t, err, ErrWalletNotOwned)
	assert.True(t, got.IsZero())
	repo.AssertNotCalled(t, "GetBalance", mock.Anything, mock.Anything)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_overdueDebtFor_UserNotFound: user tidak ada (pgx.ErrNoRows) ->
// ErrWalletNotFound.
func TestService_overdueDebtFor_UserNotFound(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, lgr, nil, mDB)
	debt, err := svc.overdueDebtFor(context.Background(), testUserID)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.True(t, debt.IsZero())
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_kycLimitFor_UserNotFound: user tidak ada (pgx.ErrNoRows) ->
// ErrWalletNotFound.
func TestService_kycLimitFor_UserNotFound(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	svc := NewService(repo, lgr, nil, mDB)
	limit, err := svc.kycLimitFor(context.Background(), testUserID)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.True(t, limit.IsZero())
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test Service: redisKey helper ----

// TestService_redisKey: test helper redisKey(userID, key) return string format.
func TestService_redisKey(t *testing.T) {
	got := redisKey(testUserID, "my-key")
	assert.Equal(t, redisKeyPrefix+testUserID.String()+":my-key", got)
}

// ---- Test Service: redisGetCachedResp ----

// TestService_redisGetCachedResp_Success: dengan Redis (miniredis) return
// cached response ketika state COMPLETED + response non-kosong.
func TestService_redisGetCachedResp_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	raw := json.RawMessage(`{"transaction_id":"abc","status":"COMPLETED"}`)
	cached := redisCache{State: redisCompleted, Response: raw}
	b, err := json.Marshal(cached)
	assert.NoError(t, err)

	assert.NoError(t, mr.Set(redisKey(testUserID, "k1"), string(b)))

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), testUserID, "k1")

	assert.True(t, ok)
	assert.Equal(t, raw, resp)
}

// TestService_redisGetCachedResp_NotFound: dengan Redis kosong (kunci tidak ada)
// return nil / ok=false.
func TestService_redisGetCachedResp_NotFound(t *testing.T) {
	mr := miniredis.RunT(t)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), testUserID, "missing")

	assert.False(t, ok)
	assert.Nil(t, resp)
}

// TestService_redisGetCachedResp_NotCompleted: state bukan COMPLETED ->
// return nil / ok=false (menutup cabang state check).
func TestService_redisGetCachedResp_NotCompleted(t *testing.T) {
	mr := miniredis.RunT(t)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cached := redisCache{State: "PROCESSING", Response: json.RawMessage(`{}`)}
	b, _ := json.Marshal(cached)
	assert.NoError(t, mr.Set(redisKey(testUserID, "k2"), string(b)))

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), testUserID, "k2")

	assert.False(t, ok)
	assert.Nil(t, resp)
}

// TestService_redisGetCachedResp_NilRedis: redis tidak dikonfigurasi ->
// langsung return nil / ok=false.
func TestService_redisGetCachedResp_NilRedis(t *testing.T) {
	svc := NewService(new(mockRepo), new(mockLedger), nil, nil)
	resp, ok := svc.redisGetCachedResp(context.Background(), testUserID, "k3")

	assert.False(t, ok)
	assert.Nil(t, resp)
}

// ---- Test Service: Transfer success ----

func TestService_TransferSuccess(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	fromID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	toID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	fromWallet := &Wallet{
		ID: fromID, UserID: testUserID, Type: WalletTypeCustomer,
		Balance: decimal.NewFromInt(100000), Status: "ACTIVE",
	}
	toWallet := &Wallet{
		ID: toID, UserID: uuid.MustParse("99999999-9999-9999-9999-999999999999"),
		Type: WalletTypeCustomer, Balance: decimal.NewFromInt(50000), Status: "ACTIVE",
	}

	repo.On("GetByID", mock.Anything, fromID).Return(fromWallet, nil)
	repo.On("GetByID", mock.Anything, toID).Return(toWallet, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)

	idemKey := "transfer-key-success"
	relFrom := decimal.NewFromInt(60000) // balance setelah lock
	relTo := decimal.NewFromInt(90000)

	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, testUserID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(fromID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}).
			AddRow(fromID, testUserID, WalletTypeCustomer, relFrom, "ACTIVE", nil, nil))
	mDB.ExpectQuery("SELECT id, user_id, wallet_type, balance, status, created_at, updated_at").
		WithArgs(toID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "wallet_type", "balance", "status", "created_at", "updated_at"}).
			AddRow(toID, toWallet.UserID, WalletTypeCustomer, relTo, "ACTIVE", nil, nil))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, testUserID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.Transfer(context.Background(), TransferRequest{
		UserID:         testUserID,
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         decimal.NewFromInt(40000),
		IdempotencyKey: idemKey,
		Description:    "p2p transfer",
	})

	assert.NoError(t, err)
	assert.Equal(t, decimal.NewFromInt(20000), resp.FromBalance)
	assert.Equal(t, decimal.NewFromInt(130000), resp.ToBalance)
	repo.AssertExpectations(t)
	lgr.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test Service: TopUp invalid amount ----

func TestService_TopUp_InvalidAmount(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         decimal.NewFromInt(100), // < 1000
		IdempotencyKey: "invalid-amount",
	})

	assert.ErrorIs(t, err, ErrInvalidAmount)
	repo.AssertNotCalled(t, "GetByUserIDAndType")
}

func TestService_TopUp_MissingIdempotency(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.TopUp(context.Background(), TopUpRequest{
		UserID: testUserID,
		Amount: decimal.NewFromInt(10000),
	})

	assert.EqualError(t, err, "idempotency key is required")
	repo.AssertNotCalled(t, "GetByUserIDAndType")
}

func TestService_Transfer_MissingIdempotency(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.Transfer(context.Background(), TransferRequest{
		UserID:       testUserID,
		FromWalletID: testWalletID,
		ToWalletID:   uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Amount:       decimal.NewFromInt(5000),
	})

	assert.EqualError(t, err, "idempotency key is required")
}

func TestService_Transfer_NotPositiveAmount(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.Transfer(context.Background(), TransferRequest{
		UserID:         testUserID,
		FromWalletID:   testWalletID,
		ToWalletID:     uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Amount:         decimal.Zero,
		IdempotencyKey: "k",
	})

	assert.ErrorIs(t, err, ErrInvalidAmount)
}

// ---- Test Service: TopUp wallet inactive ----

func TestService_TopUp_WalletNotFound(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(nil, ErrWalletNotFound)

	// Redis nil -> skip L1; setelah GetByUserIDAndType error, flow berhenti
	// sebelum query overdue_debt/kyc/db apapun.

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         decimal.NewFromInt(10000),
		IdempotencyKey: "topup-nf",
	})

	assert.ErrorIs(t, err, ErrWalletNotFound)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestService_Transfer_InactiveWallet(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	fromID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	toID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	repo.On("GetByID", mock.Anything, fromID).Return(&Wallet{
		ID: fromID, UserID: testUserID, Type: WalletTypeCustomer,
		Balance: decimal.NewFromInt(10000), Status: "SUSPENDED",
	}, nil)
	repo.On("GetByID", mock.Anything, toID).Return(&Wallet{
		ID: toID, UserID: uuid.MustParse("99999999-9999-9999-9999-999999999999"),
		Type: WalletTypeCustomer, Balance: decimal.Zero, Status: "ACTIVE",
	}, nil)

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.Transfer(context.Background(), TransferRequest{
		UserID:         testUserID,
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         decimal.NewFromInt(5000),
		IdempotencyKey: "k",
	})

	assert.ErrorIs(t, err, ErrWalletInactive)
	repo.AssertExpectations(t)
}

// ---- Test Service: KYC VERIFIED limit path (TopUp) ----

func TestTopUp_KycVerified(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	amount := decimal.NewFromInt(100000)
	idemKey := "topup-kyc-verified"

	repo.On("GetByUserIDAndType", mock.Anything, testUserID, WalletTypeCustomer).
		Return(newCustomerWallet(decimal.Zero), nil)
	repo.On("GetBalance", mock.Anything, testWalletID).Return(amount, nil)
	lgr.On("CreateLedgerEntries", mock.AnythingOfType("[]wallet.LedgerEntry")).Return(nil)

	mDB.ExpectQuery("SELECT COALESCE\\(overdue_debt").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"overdue_debt"}).AddRow(decimal.Zero))
	mDB.ExpectQuery("kyc_status").
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"kyc_status"}).AddRow(kycVerified))
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(idemKey, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(idemKey, testUserID).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("SELECT id FROM wallets WHERE wallet_type").
		WithArgs(WalletTypeSystemBankGateway).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testBankID))
	mDB.ExpectExec("SELECT id FROM wallets WHERE id = ANY").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("SELECT 2"))
	mDB.ExpectExec("INSERT INTO topup_transactions").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectExec("UPDATE topup_transactions").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit()
	mDB.ExpectExec("UPDATE idempotency_cache").
		WithArgs(idemKey, testUserID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(repo, lgr, nil, mDB)
	resp, err := svc.TopUp(context.Background(), TopUpRequest{
		UserID:         testUserID,
		Amount:         amount,
		IdempotencyKey: idemKey,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test Service: idemAcquire error branches ----

func TestService_idemAcquire_ProcessingError(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	key := "process-key"
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(key, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"state", "response_body", "debounce_at"}).
			AddRow(pgProcessing, "{}", nil))

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.idemAcquire(context.Background(), testUserID, key)

	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestService_idemAcquire_InsertConflict(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	key := "conflict-key"
	mDB.ExpectQuery("SELECT state, response_body, debounce_at").
		WithArgs(key, testUserID).
		WillReturnError(pgx.ErrNoRows)
	mDB.ExpectExec("INSERT INTO idempotency_cache").
		WithArgs(key, testUserID).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	svc := NewService(repo, lgr, nil, mDB)
	_, err = svc.idemAcquire(context.Background(), testUserID, key)

	assert.ErrorIs(t, err, ErrIdempotencyInProgress)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test Service: redisSet dengan Redis aktif ----

func TestService_redisSet(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	svc := NewService(new(mockRepo), new(mockLedger), rdb, nil)
	svc.redisSet(context.Background(), testUserID, "setkey", redisCompleted, json.RawMessage(`{"ok":true}`))

	got, err := mr.Get(redisKey(testUserID, "setkey"))
	assert.NoError(t, err)
	cached := redisCache{}
	assert.NoError(t, json.Unmarshal([]byte(got), &cached))
	assert.Equal(t, redisCompleted, cached.State)
	assert.JSONEq(t, `{"ok":true}`, string(cached.Response))
}

// ---- Test Service: UpdateWalletStatus ----

var testAdminID = uuid.MustParse("00000000-0000-0000-0000-0000000000aa")

// TestService_UpdateWalletStatus_Success: reponse berisi wallet_id, status,
// updated_at dari repo; audit log admin_action_logs tercatat (best-effort).
func TestService_UpdateWalletStatus_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	now := time.Now()
	repo.On("UpdateWalletStatus", mock.Anything, testWalletID, WalletStatusSuspended).
		Return(now, nil)
	mDB.ExpectExec("INSERT INTO admin_action_logs").
		WithArgs(testAdminID, "wallet_status_change", "wallet", testWalletID, pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))

	svc := NewService(repo, lgr, nil, mDB)
	res, err := svc.UpdateWalletStatus(context.Background(), testWalletID, testAdminID, WalletStatusSuspended, "fraud investigation")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, testWalletID, res.WalletID)
	assert.Equal(t, WalletStatusSuspended, res.Status)
	assert.Equal(t, now, res.UpdatedAt)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_UpdateWalletStatus_InvalidStatus: status di luar whitelist ->
// ErrInvalidStatus, repo tidak dipanggil.
func TestService_UpdateWalletStatus_InvalidStatus(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	res, err := svc.UpdateWalletStatus(context.Background(), testWalletID, testAdminID, "BLOCKED", "x")

	assert.ErrorIs(t, err, ErrInvalidStatus)
	assert.Nil(t, res)
	repo.AssertNotCalled(t, "UpdateWalletStatus", mock.Anything, mock.Anything, mock.Anything)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_UpdateWalletStatus_ReasonRequired: status non-ACTIVE tanpa
// reason -> ErrReasonRequired, repo tidak dipanggil.
func TestService_UpdateWalletStatus_ReasonRequired(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)
	res, err := svc.UpdateWalletStatus(context.Background(), testWalletID, testAdminID, WalletStatusFrozen, "")

	assert.ErrorIs(t, err, ErrReasonRequired)
	assert.Nil(t, res)
	repo.AssertNotCalled(t, "UpdateWalletStatus", mock.Anything, mock.Anything, mock.Anything)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_UpdateWalletStatus_NotFound: repo mengembalikan
// ErrWalletNotFound -> error diteruskan; audit log tidak di-insert.
func TestService_UpdateWalletStatus_NotFound(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("UpdateWalletStatus", mock.Anything, testWalletID, WalletStatusSuspended).
		Return(time.Time{}, ErrWalletNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	res, err := svc.UpdateWalletStatus(context.Background(), testWalletID, testAdminID, WalletStatusSuspended, "wallet tidak ada")

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.Nil(t, res)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// ---- Test Service: GetWalletHistory ----

// sampleLedgerEntries membuat 2 entry contoh milik wallet test.
func sampleLedgerEntries() []LedgerEntry {
	return []LedgerEntry{
		{
			ID:            uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			WalletID:      testWalletID,
			EntryType:     EntryDebit,
			Amount:        decimal.NewFromInt(50000),
			BalanceAfter:  decimal.NewFromInt(150000),
			ReferenceID:   uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			ReferenceType: "TRANSFER",
			Description:   "TRANSFER - from wallet",
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			WalletID:      testWalletID,
			EntryType:     EntryCredit,
			Amount:        decimal.NewFromInt(100000),
			BalanceAfter:  decimal.NewFromInt(200000),
			ReferenceID:   uuid.MustParse("77777777-7777-7777-7777-777777777777"),
			ReferenceType: "TOPUP",
			Description:   "TOPUP - CREDIT CUSTOMER wallet",
			CreatedAt:     time.Now(),
		},
	}
}

// TestService_GetWalletHistory_Success: owner cocok, count + list terpanggil,
// total_pages dihitung benar (2 entries, page_size 20 -> 1 page).
func TestService_GetWalletHistory_Success(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	entries := sampleLedgerEntries()
	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(testUserID, nil)
	repo.On("CountLedgerEntries", mock.Anything, testWalletID, "").Return(2, nil)
	repo.On("ListLedgerEntries", mock.Anything, testWalletID, "", 20, 0).Return(entries, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 1, 20)

	assert.NoError(t, err)
	assert.Len(t, got.Entries, 2)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 20, got.PageSize)
	assert.Equal(t, 2, got.Total)
	assert.Equal(t, 1, got.TotalPages)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetWalletHistory_NotOwned: wallet milik user lain -> 403.
func TestService_GetWalletHistory_NotOwned(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	otherUser := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(otherUser, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 1, 20)

	assert.ErrorIs(t, err, ErrWalletNotOwned)
	assert.Nil(t, got)
	repo.AssertNotCalled(t, "CountLedgerEntries", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "ListLedgerEntries", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetWalletHistory_NotFound: wallet tidak ada -> ErrWalletNotFound.
func TestService_GetWalletHistory_NotFound(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(uuid.Nil, ErrWalletNotFound)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 1, 20)

	assert.ErrorIs(t, err, ErrWalletNotFound)
	assert.Nil(t, got)
	repo.AssertNotCalled(t, "CountLedgerEntries", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "ListLedgerEntries", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetWalletHistory_Empty: wallet sah tapi belum ada transaksi.
func TestService_GetWalletHistory_Empty(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(testUserID, nil)
	repo.On("CountLedgerEntries", mock.Anything, testWalletID, "").Return(0, nil)
	repo.On("ListLedgerEntries", mock.Anything, testWalletID, "", 20, 0).Return([]LedgerEntry{}, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 1, 20)

	assert.NoError(t, err)
	assert.Empty(t, got.Entries)
	assert.Equal(t, 0, got.Total)
	assert.Equal(t, 0, got.TotalPages)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetWalletHistory_FilterRefType: filter reference_type diteruskan
// ke count & list.
func TestService_GetWalletHistory_FilterRefType(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	entries := sampleLedgerEntries()
	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(testUserID, nil)
	repo.On("CountLedgerEntries", mock.Anything, testWalletID, "TOPUP").Return(1, nil)
	repo.On("ListLedgerEntries", mock.Anything, testWalletID, "TOPUP", 10, 0).Return(entries[:1], nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "TOPUP", 1, 10)

	assert.NoError(t, err)
	assert.Len(t, got.Entries, 1)
	assert.Equal(t, 1, got.Total)
	assert.Equal(t, 1, got.TotalPages)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetWalletHistory_Pagination: page 2, page_size 5, total 12 ->
// offset 5, total_pages 3.
func TestService_GetWalletHistory_Pagination(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	entries := sampleLedgerEntries()
	repo.On("GetWalletOwner", mock.Anything, testWalletID).Return(testUserID, nil)
	repo.On("CountLedgerEntries", mock.Anything, testWalletID, "").Return(12, nil)
	repo.On("ListLedgerEntries", mock.Anything, testWalletID, "", 5, 5).Return(entries, nil)

	svc := NewService(repo, lgr, nil, mDB)
	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 2, 5)

	assert.NoError(t, err)
	assert.Len(t, got.Entries, 2)
	assert.Equal(t, 2, got.Page)
	assert.Equal(t, 5, got.PageSize)
	assert.Equal(t, 12, got.Total)
	assert.Equal(t, 3, got.TotalPages)
	repo.AssertExpectations(t)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestService_GetWalletHistory_InvalidPagination: page 0 / page_size 51 ->
// ErrInvalidPagination tanpa menyentuh repo.
func TestService_GetWalletHistory_InvalidPagination(t *testing.T) {
	repo := new(mockRepo)
	lgr := new(mockLedger)
	mDB, err := pgxmock.NewPool()
	assert.NoError(t, err)

	svc := NewService(repo, lgr, nil, mDB)

	got, err := svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 0, 20)
	assert.ErrorIs(t, err, ErrInvalidPagination)
	assert.Nil(t, got)

	got, err = svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 1, 51)
	assert.ErrorIs(t, err, ErrInvalidPagination)
	assert.Nil(t, got)

	got, err = svc.GetWalletHistory(context.Background(), testUserID, testWalletID, "", 1, 0)
	assert.ErrorIs(t, err, ErrInvalidPagination)
	assert.Nil(t, got)

	repo.AssertNotCalled(t, "GetWalletOwner", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CountLedgerEntries", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "ListLedgerEntries", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.NoError(t, mDB.ExpectationsWereMet())
}
