package admin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testTxnID    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testEscrowID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	testMerchID  = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	testDrivID   = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	testPlatID   = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	testReceivID = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	testAdminID  = uuid.MustParse("77777777-7777-7777-7777-777777777777")
)

func newSvc(t *testing.T, mDB pgxmock.PgxPoolIface) *Service {
	t.Helper()
	return NewService(
		NewRepository(mDB),
		mDB,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func ledgerRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "wallet_id", "entry_type", "amount", "reference_type",
		"reference_id", "description", "is_reversed", "created_at",
	})
}

// TestReverseTransaction_Success (WALLET): settlement escrow DEBIT + partai CREDIT.
// Semua saldo partai mencukupi -> tidak ada shortfall, dan journal reversal
// harus seimbang (SUM(DEBIT) == SUM(CREDIT)).
func TestReverseTransaction_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	// 1) Begin + statement timeout
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	// 2) GetLedgerEntries (non-reversed)
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testEscrowID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testDrivID, "CREDIT", decimal.NewFromInt(30000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testPlatID, "CREDIT", decimal.NewFromInt(10000), "SETTLEMENT", testTxnID, "settle", false, now))
	// 3) GetWalletTypes
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testEscrowID, "SYSTEM_ESCROW").
			AddRow(testMerchID, "MERCHANT").
			AddRow(testDrivID, "DRIVER").
			AddRow(testPlatID, "SYSTEM_PLATFORM"))
	// 4) SystemWalletID (SDROverdraft)
	mDB.ExpectQuery("AND user_id IS NULL").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testReceivID))
	// 5) GetWalletBalances
	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "balance"}).
			AddRow(testEscrowID, decimal.Zero).
			AddRow(testMerchID, decimal.NewFromInt(100000)).
			AddRow(testDrivID, decimal.NewFromInt(50000)).
			AddRow(testPlatID, decimal.NewFromInt(20000)).
			AddRow(testReceivID, decimal.Zero))
	// 6) InsertLedgerEntries x4: CREDIT escrow, DEBIT M, DEBIT D, DEBIT P
	for i := 0; i < 4; i++ {
		mDB.ExpectExec("INSERT INTO ledger_entries").
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	}
	// 7) UpdateLedgerReversal
	mDB.ExpectExec("SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 4"))
	// 8) CreateAdminActionLog
	mDB.ExpectExec("INSERT INTO admin_action_logs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	// 9) Commit
	mDB.ExpectCommit()

	svc := newSvc(t, mDB)
	res, err := svc.ReverseTransaction(context.Background(), ReversalRequest{
		TransactionID: testTxnID,
		AdminID:       testAdminID,
		Reason:        "saldo salah",
		Notes:         "reversal manual",
	})
	require.NoError(t, err)
	assert.Equal(t, decimal.NewFromInt(100000), res.Refunded)
	assert.True(t, res.Shortfall.IsZero())
	assert.False(t, res.HasSweep)
	assert.NotEqual(t, uuid.Nil, res.ReversalID)

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_Shortfall: saldo merchant tidak mencukupi bagiannya.
// Shortfall (m - mb) harus dicatat sebagai DEBIT ke SYSTEM_RECEIVABLE_OVERDRAFT
// dan keseluruhan journal reversal tetap seimbang.
func TestReverseTransaction_Shortfall(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testEscrowID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testDrivID, "CREDIT", decimal.NewFromInt(30000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testPlatID, "CREDIT", decimal.NewFromInt(10000), "SETTLEMENT", testTxnID, "settle", false, now))
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testEscrowID, "SYSTEM_ESCROW").
			AddRow(testMerchID, "MERCHANT").
			AddRow(testDrivID, "DRIVER").
			AddRow(testPlatID, "SYSTEM_PLATFORM"))
	mDB.ExpectQuery("AND user_id IS NULL").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testReceivID))
	// merchant balance 10000 < 60000 -> shortfall 50000
	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "balance"}).
			AddRow(testEscrowID, decimal.Zero).
			AddRow(testMerchID, decimal.NewFromInt(10000)).
			AddRow(testDrivID, decimal.NewFromInt(50000)).
			AddRow(testPlatID, decimal.NewFromInt(20000)).
			AddRow(testReceivID, decimal.Zero))
	// InsertLedgerEntries x5: CREDIT escrow(100000), DEBIT M(10000), DEBIT D(30000),
	// DEBIT P(10000), DEBIT receivable(50000) -> total DEBIT == CREDIT == 100000
	for i := 0; i < 5; i++ {
		mDB.ExpectExec("INSERT INTO ledger_entries").
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	}
	mDB.ExpectExec("SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 4"))
	mDB.ExpectExec("INSERT INTO admin_action_logs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	svc := newSvc(t, mDB)
	res, err := svc.ReverseTransaction(context.Background(), ReversalRequest{
		TransactionID: testTxnID,
		AdminID:       testAdminID,
		Reason:        "penarikan salah",
	})
	require.NoError(t, err)
	assert.Equal(t, decimal.NewFromInt(100000), res.Refunded)
	assert.True(t, res.Shortfall.Equal(decimal.NewFromInt(50000)))
	assert.True(t, res.HasSweep)

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// newTestHandler membangun Handler dengan pgxmock pool + miniredis untuk
// menguji flow 2FA/lockout pada handler (test negatif tidak menyentuh service).
func newTestHandler(t *testing.T, mDB pgxmock.PgxPoolIface, mr *miniredis.Miniredis) *Handler {
	t.Helper()
	var rdb *redis.Client
	if mr != nil {
		rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = rdb.Close() })
	}
	svc := NewService(
		NewRepository(mDB),
		mDB,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return NewHandler(svc, mDB, rdb, slog.New(slog.NewTextHandler(io.Discard, nil)), NewStaticTwoFactorValidator(""), newTestJWT())
}

func reverseRouter(h *Handler, adminID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/admin/transactions/:id/reverse", func(c *gin.Context) {
		c.Set("user_id", adminID.String())
		h.ReverseTransaction(c)
	})
	return r
}

// TestReverseTransaction_2FAInvalid: token 2FA tidak valid -> 401 UNAUTHORIZED_2FA,
// dan percobaan gagal dicatat (Redis INCR + UPSERT PostgreSQL).
func TestReverseTransaction_2FAInvalid(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	// checkLockout -> Redis nil -> fallback PG (tidak terkunci)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "0")
	mDB.ExpectQuery("FROM admin_lockouts WHERE admin_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(false))
	// recordFailedAttempt -> Redis INCR=1 -> UPSERT PG
	mDB.ExpectExec("INSERT INTO admin_lockouts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))

	h := newTestHandler(t, mDB, mr)
	router := reverseRouter(h, testAdminID)

	req := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+testTxnID.String()+"/reverse", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(admin2FASecretHeader, "wrong-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED_2FA")

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_Lockout: admin sudah terkunci (Redis set flag '1')
// -> 429 LOCKOUT sebelum validasi 2FA, tanpa menyentuh PostgreSQL.
func TestReverseTransaction_Lockout(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "1")
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newTestHandler(t, mDB, mr)
	router := reverseRouter(h, testAdminID)

	req := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+testTxnID.String()+"/reverse", nil)
	req.Header.Set(admin2FASecretHeader, "admin-2fa-secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "LOCKOUT")

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestStaticTwoFactorValidator: memastikan token statis default berfungsi.
func TestStaticTwoFactorValidator(t *testing.T) {
	v := NewStaticTwoFactorValidator("")
	assert.True(t, v.Validate(defaultAdmin2FASecret))
	assert.False(t, v.Validate(""))
	assert.False(t, v.Validate("wrong"))
}

// TestGetTransactionDetail: detail transaksi (GetAllLedgerEntries) menghitung
// breakdown + potential shortfall dab flag reversed.
func TestGetTransactionDetail(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectBegin()
	mDB.ExpectQuery("FROM ledger_entries").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testEscrowID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT", testTxnID, "settle", true, now).
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testDrivID, "CREDIT", decimal.NewFromInt(30000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testPlatID, "CREDIT", decimal.NewFromInt(10000), "SETTLEMENT", testTxnID, "settle", false, now))
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testEscrowID, "SYSTEM_ESCROW").
			AddRow(testMerchID, "MERCHANT").
			AddRow(testDrivID, "DRIVER").
			AddRow(testPlatID, "SYSTEM_PLATFORM"))
	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "balance"}).
			AddRow(testEscrowID, decimal.Zero).
			AddRow(testMerchID, decimal.NewFromInt(10000)).
			AddRow(testDrivID, decimal.NewFromInt(50000)).
			AddRow(testPlatID, decimal.NewFromInt(20000)))

	svc := newSvc(t, mDB)
	d, err := svc.GetTransactionDetail(context.Background(), testTxnID)
	require.NoError(t, err)
	assert.Equal(t, testEscrowID, d.RefundWallet)
	assert.True(t, d.TotalAmount.Equal(decimal.NewFromInt(100000)))
	assert.True(t, d.MerchantShare.Equal(decimal.NewFromInt(60000)))
	assert.True(t, d.DriverShare.Equal(decimal.NewFromInt(30000)))
	assert.True(t, d.PlatformShare.Equal(decimal.NewFromInt(10000)))
	assert.True(t, d.PotentialShortfall.Equal(decimal.NewFromInt(50000)))
	assert.False(t, d.Reversed)
	assert.Len(t, d.Entries, 4)

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetTransactionDetail_NotFound: tanpa ledger entry -> ErrTransactionNotFound.
func TestGetTransactionDetail_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("FROM ledger_entries").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows())

	svc := newSvc(t, mDB)
	_, err = svc.GetTransactionDetail(context.Background(), testTxnID)
	require.ErrorIs(t, err, ErrTransactionNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_NotFound: tidak ada entry -> ErrTransactionNotFound.
func TestReverseTransaction_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows())

	svc := newSvc(t, mDB)
	_, err = svc.ReverseTransaction(context.Background(), ReversalRequest{
		TransactionID: testTxnID,
		AdminID:       testAdminID,
		Reason:        "test",
	})
	require.ErrorIs(t, err, ErrTransactionNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_AlreadyReversed: semua entry sudah reversed -> tolak.
func TestReverseTransaction_AlreadyReversed(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testEscrowID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT", testTxnID, "settle", true, now).
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", true, now))

	svc := newSvc(t, mDB)
	_, err = svc.ReverseTransaction(context.Background(), ReversalRequest{
		TransactionID: testTxnID,
		AdminID:       testAdminID,
		Reason:        "test",
	})
	require.ErrorIs(t, err, ErrTransactionReversed)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_ReasonRequired: reason kosong -> ErrInvalidReason.
func TestReverseTransaction_ReasonRequired(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	svc := newSvc(t, mDB)
	_, err = svc.ReverseTransaction(context.Background(), ReversalRequest{
		TransactionID: testTxnID,
		AdminID:       testAdminID,
	})
	assert.ErrorIs(t, err, ErrInvalidReason)
}

// getRouter membangun router dengan handler GetTransaction (tanpa auth identity).
func getRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/transactions/:id", h.GetTransaction)
	return r
}

// TestGetTransactionHandler: GET /admin/transactions/:id mengembalikan detail.
func TestGetTransactionHandler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectBegin()
	mDB.ExpectQuery("FROM ledger_entries").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testEscrowID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now))
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testEscrowID, "SYSTEM_ESCROW").
			AddRow(testMerchID, "MERCHANT"))
	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "balance"}).
			AddRow(testEscrowID, decimal.Zero).
			AddRow(testMerchID, decimal.NewFromInt(100000)))

	h := newTestHandler(t, mDB, nil)
	router := getRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/admin/transactions/"+testTxnID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "merchant_share")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_SuccessViaHandler: alur lengkap melalui HTTP handler
// dengan 2FA valid + service (menutup branch resetAttempts & response sukses).
func TestReverseTransaction_SuccessViaHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "0")
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	// checkLockout -> Redis "0" -> fallback PG (tidak terkunci)
	mDB.ExpectQuery("FROM admin_lockouts WHERE admin_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(false))
	// resetAttempts (2FA valid) -> PG reset
	mDB.ExpectExec("UPDATE admin_lockouts SET failed_attempts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	// service
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testEscrowID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testDrivID, "CREDIT", decimal.NewFromInt(30000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testPlatID, "CREDIT", decimal.NewFromInt(10000), "SETTLEMENT", testTxnID, "settle", false, now))
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testEscrowID, "SYSTEM_ESCROW").
			AddRow(testMerchID, "MERCHANT").
			AddRow(testDrivID, "DRIVER").
			AddRow(testPlatID, "SYSTEM_PLATFORM"))
	mDB.ExpectQuery("AND user_id IS NULL").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(testReceivID))
	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "balance"}).
			AddRow(testEscrowID, decimal.Zero).
			AddRow(testMerchID, decimal.NewFromInt(100000)).
			AddRow(testDrivID, decimal.NewFromInt(50000)).
			AddRow(testPlatID, decimal.NewFromInt(20000)).
			AddRow(testReceivID, decimal.Zero))
	for i := 0; i < 4; i++ {
		mDB.ExpectExec("INSERT INTO ledger_entries").
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	}
	mDB.ExpectExec("SET is_reversed = TRUE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 4"))
	mDB.ExpectExec("INSERT INTO admin_action_logs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	h := newTestHandler(t, mDB, mr)
	router := reverseRouter(h, testAdminID)
	req := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+testTxnID.String()+"/reverse", strings.NewReader(`{"reason":"saldo salah","notes":"manual"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(admin2FASecretHeader, "admin-2fa-secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "reversal_id")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_LockoutPG: Redis terkunci -> langsung 429; dan saat Redis
// kosong, lock PostgreSQL mengunci akun (branch checkLockoutPG locked).
func TestReverseTransaction_LockoutPG(t *testing.T) {
	mr := miniredis.RunT(t)
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("FROM admin_lockouts WHERE admin_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(true))

	h := newTestHandler(t, mDB, mr)
	router := reverseRouter(h, testAdminID)
	req := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+testTxnID.String()+"/reverse", nil)
	req.Header.Set(admin2FASecretHeader, "admin-2fa-secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "LOCKOUT")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRecordFailedAttemptPG_Locks: tanpa Redis, pada failed attempt ke-3 (>=3)
// maka lockout diterapkan lewat recordFailedAttemptPG.
func TestRecordFailedAttemptPG_Locks(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	// attempts ke-3 lewat UPSERT RETURNING failed_attempts = 3
	mDB.ExpectQuery("INSERT INTO admin_lockouts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"failed_attempts"}).AddRow(3))
	mDB.ExpectExec("UPDATE admin_lockouts SET locked_until").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	h := newTestHandler(t, mDB, nil)
	attempts, err := h.recordFailedAttempt(context.Background(), testAdminID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), attempts)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestResetAttempts: reset counter pada PostgreSQL.
func TestResetAttempts(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectExec("UPDATE admin_lockouts SET failed_attempts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	h := newTestHandler(t, mDB, nil)
	h.resetAttempts(context.Background(), testAdminID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserRole: reporsitory mengambil role admin.
func TestGetUserRole(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("SELECT user_type FROM users").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"user_type"}).AddRow("admin"))

	repo := NewRepository(mDB)
	role, err := repo.GetUserRole(context.Background(), testAdminID)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateWalletBalance: update saldo wallet eksplisit (auto-sweep).
func TestUpdateWalletBalance(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectExec("UPDATE wallets SET balance = balance").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectCommit()

	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	repo := NewRepository(mDB)
	require.NoError(t, repo.UpdateWalletBalance(context.Background(), tx, testEscrowID, decimal.NewFromInt(5000)))
	require.NoError(t, tx.Commit(context.Background()))
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransaction_NotBalanced: tidak ada entry DEBIT (tanpa refund wallet)
// -> reversal ditolak karena tidak seimbang.
func TestReverseTransaction_NotBalanced(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testDrivID, "CREDIT", decimal.NewFromInt(30000), "SETTLEMENT", testTxnID, "settle", false, now))
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testMerchID, "MERCHANT").
			AddRow(testDrivID, "DRIVER"))

	svc := newSvc(t, mDB)
	_, err = svc.ReverseTransaction(context.Background(), ReversalRequest{
		TransactionID: testTxnID,
		AdminID:       testAdminID,
		Reason:        "test",
	})
	require.ErrorIs(t, err, ErrNotBalanced)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransactionHandler_NotFound: handler memetakan error not-found ke 404.
func TestReverseTransactionHandler_NotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "0")
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("FROM admin_lockouts WHERE admin_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(false))
	mDB.ExpectExec("UPDATE admin_lockouts SET failed_attempts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows())

	h := newTestHandler(t, mDB, mr)
	router := reverseRouter(h, testAdminID)
	req := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+testTxnID.String()+"/reverse", strings.NewReader(`{"reason":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(admin2FASecretHeader, "admin-2fa-secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "TRANSACTION_NOT_FOUND")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestReverseTransactionHandler_Imbalance: handler memetakan ErrNotBalanced ke 422.
func TestReverseTransactionHandler_Imbalance(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(lockoutKeyPrefix+testAdminID.String(), "0")
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectQuery("FROM admin_lockouts WHERE admin_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(false))
	mDB.ExpectExec("UPDATE admin_lockouts SET failed_attempts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mDB.ExpectBegin()
	mDB.ExpectExec("SET LOCAL statement_timeout").
		WithArgs().
		WillReturnResult(pgconn.NewCommandTag("SET"))
	mDB.ExpectQuery("is_reversed = FALSE").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow(uuid.New(), testMerchID, "CREDIT", decimal.NewFromInt(60000), "SETTLEMENT", testTxnID, "settle", false, now).
			AddRow(uuid.New(), testDrivID, "CREDIT", decimal.NewFromInt(30000), "SETTLEMENT", testTxnID, "settle", false, now))
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow(testMerchID, "MERCHANT").
			AddRow(testDrivID, "DRIVER"))

	h := newTestHandler(t, mDB, mr)
	router := reverseRouter(h, testAdminID)
	req := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+testTxnID.String()+"/reverse", strings.NewReader(`{"reason":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(admin2FASecretHeader, "admin-2fa-secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "LEDGER_IMBALANCE")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestSystemWalletID_NotFound: wallet sistem tidak ada -> ErrWalletNotFound.
func TestSystemWalletID_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("AND user_id IS NULL").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	repo := NewRepository(mDB)
	_, err = repo.SystemWalletID(context.Background(), tx, walletTypeSDROverdraft)
	require.ErrorIs(t, err, ErrWalletNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserRole_Error: error saat mengambil role.
func TestGetUserRole_Error(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("SELECT user_type FROM users").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"user_type"}))

	repo := NewRepository(mDB)
	_, err = repo.GetUserRole(context.Background(), testAdminID)
	require.Error(t, err)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletTypesScanError: error scan kolom wallet_type.
func TestRepository_GetWalletTypesScanError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("wallet_type FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "wallet_type"}).
			AddRow("not-a-uuid", "MERCHANT"))
	mDB.ExpectRollback()

	repo := NewRepository(mDB)
	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	_, err = repo.GetWalletTypes(context.Background(), tx, []uuid.UUID{testMerchID})
	require.Error(t, err)
	_ = tx.Rollback(context.Background())
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetWalletBalancesScanError: error saat mengambil saldo.
func TestRepository_GetWalletBalancesScanError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "balance"}).
			AddRow("not-a-uuid", decimal.NewFromInt(100)))
	mDB.ExpectRollback()

	repo := NewRepository(mDB)
	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	_, err = repo.GetWalletBalances(context.Background(), tx, []uuid.UUID{testMerchID})
	require.Error(t, err)
	_ = tx.Rollback(context.Background())
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRepository_GetLedgerEntriesScanError: error scan baris ledger.
func TestRepository_GetLedgerEntriesScanError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("FROM ledger_entries").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows().
			AddRow("not-a-uuid", testEscrowID, "DEBIT", decimal.NewFromInt(100), "X", testTxnID, "d", false, time.Now()))
	mDB.ExpectRollback()

	repo := NewRepository(mDB)
	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	_, err = repo.GetLedgerEntries(context.Background(), tx, testTxnID)
	require.Error(t, err)
	_ = tx.Rollback(context.Background())
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRecordFailedAttempt_Fallback: Redis gagal (server ditutup) -> fallback ke
// PostgreSQL untuk mencatat percobaan gagal dan menerapkan lockout.
func TestRecordFailedAttempt_Fallback(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("INSERT INTO admin_lockouts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"failed_attempts"}).AddRow(3))
	mDB.ExpectExec("UPDATE admin_lockouts SET locked_until").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	svc := NewService(NewRepository(mDB), mDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewHandler(svc, mDB, rdb, slog.New(slog.NewTextHandler(io.Discard, nil)), NewStaticTwoFactorValidator(""), newTestJWT())
	attempts, err := h.recordFailedAttempt(context.Background(), testAdminID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), attempts)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestRecordFailedAttempt_LocksViaRedis: counter Redis mencapai batas (3)
// -> lockout diterapkan (Redis SET + persist PostgreSQL).
func TestRecordFailedAttempt_LocksViaRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set(attemptKeyPrefix+testAdminID.String(), "2")
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectExec("INSERT INTO admin_lockouts").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectExec("UPDATE admin_lockouts SET locked_until").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewService(NewRepository(mDB), mDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewHandler(svc, mDB, rdb, slog.New(slog.NewTextHandler(io.Discard, nil)), NewStaticTwoFactorValidator(""), newTestJWT())
	attempts, err := h.recordFailedAttempt(context.Background(), testAdminID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), attempts)
	// lock flag Redis harus terpasang
	if got, err := mr.Get(lockoutKeyPrefix + testAdminID.String()); assert.NoError(t, err) {
		assert.Equal(t, "1", got)
	}
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestCheckLockout_RedisErrorFallback: Redis gagal -> fallback PostgreSQL
// yang menghasilkan status terkunci.
func TestCheckLockout_RedisErrorFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM admin_lockouts WHERE admin_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"locked"}).AddRow(true))

	svc := NewService(NewRepository(mDB), mDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewHandler(svc, mDB, rdb, slog.New(slog.NewTextHandler(io.Discard, nil)), NewStaticTwoFactorValidator(""), newTestJWT())
	locked, err := h.checkLockout(context.Background(), testAdminID)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetTransactionHandler_Errors: error path handler GetTransaction
// (id tidak valid -> 422, tidak ditemukan -> 404, internal -> 500).
func TestGetTransactionHandler_Errors(t *testing.T) {
	// 1) id tidak valid
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := newTestHandler(t, mDB, nil)
	router := getRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/transactions/not-a-uuid", nil))
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.NoError(t, mDB.ExpectationsWereMet())

	// 2) tidak ditemukan
	mDB2, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB2.ExpectBegin()
	mDB2.ExpectQuery("FROM ledger_entries").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(ledgerRows())
	h2 := newTestHandler(t, mDB2, nil)
	router2 := getRouter(h2)
	w2 := httptest.NewRecorder()
	router2.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/admin/transactions/"+testTxnID.String(), nil))
	assert.Equal(t, http.StatusNotFound, w2.Code)
	assert.Contains(t, w2.Body.String(), "TRANSACTION_NOT_FOUND")
	assert.NoError(t, mDB2.ExpectationsWereMet())

	// 3) error internal
	mDB3, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB3.ExpectBegin()
	mDB3.ExpectQuery("FROM ledger_entries").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(errors.New("boom"))
	h3 := newTestHandler(t, mDB3, nil)
	router3 := getRouter(h3)
	w3 := httptest.NewRecorder()
	router3.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/admin/transactions/"+testTxnID.String(), nil))
	assert.Equal(t, http.StatusInternalServerError, w3.Code)
	assert.NoError(t, mDB3.ExpectationsWereMet())
}