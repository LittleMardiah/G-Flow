package admin

// Test untuk fitur ledger admin (STEP A3, ROADMAP 4.2.2):
//   - GET  /admin/ledger                  (ListLedger + CountLedger)
//   - GET  /admin/ledger/verify/:wallet_id (VerifyWalletLedger)
//   - POST /admin/ledger/export           (ListLedgerForExport -> CSV)
//
// Pola sama seperti test dashboard/reversal: pgxmock untuk DB, gin httptest
// untuk handler. Response mengikuti apps/admin_web/src/lib/types.ts
// (LedgerPage, BalanceVerification).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ledgerRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/ledger", h.GetLedgerList)
	r.GET("/admin/ledger/verify/:wallet_id", h.VerifyLedger)
	r.POST("/admin/ledger/export", h.ExportLedger)
	return r
}

func ledgerResultRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "wallet_id", "entry_type", "amount", "reference", "created_at", "status", "note",
	})
}

// TestGetLedger_Service: GetLedger memanggil ListLedger (pagination) lalu
// CountLedger, dan mengembalikan {ledger, total_count}.
func TestGetLedger_Service(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	id1, id2 := uuid.New(), uuid.New()
	wID := uuid.New()
	mDB.ExpectQuery("ORDER BY le.created_at DESC LIMIT").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(ledgerResultRows().
			AddRow(id1, wID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT/"+testTxnID.String(), now, "ACTIVE", "settle").
			AddRow(id2, wID, "CREDIT", decimal.NewFromInt(50000), "REVERSAL/"+testTxnID.String(), now, "REVERSED", "refund"))
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs().
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))

	svc := newSvc(t, mDB)
	list, err := svc.GetLedger(context.Background(), LedgerFilter{Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, int64(2), list.TotalCount)
	require.Len(t, list.Ledger, 2)
	assert.Equal(t, id1, list.Ledger[0].ID)
	assert.Equal(t, "ACTIVE", list.Ledger[0].Status)
	assert.Equal(t, id2, list.Ledger[1].ID)
	assert.Equal(t, "REVERSED", list.Ledger[1].Status)
	assert.True(t, decimal.Decimal(list.Ledger[0].Amount).Equal(decimal.NewFromInt(100000)))
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetLedger_FilteredService: seluruh filter opsional (tanggal, wallet_type,
// entry_type, search) diteruskan ke kedua query (placeholder dinamis).
func TestGetLedger_FilteredService(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	dateFrom := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2026, 9, 21, 23, 59, 59, 0, time.UTC)
	now := time.Now()
	id1 := uuid.New()
	wID := uuid.New()

	mDB.ExpectQuery("ORDER BY le.created_at DESC LIMIT").
		WithArgs(anyArgs(9)...).
		WillReturnRows(ledgerResultRows().
			AddRow(id1, wID, "CREDIT", decimal.NewFromInt(75000), "FOOD/"+testTxnID.String(), now, "ACTIVE", "settle"))
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs(anyArgs(7)...).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))

	svc := newSvc(t, mDB)
	list, err := svc.GetLedger(context.Background(), LedgerFilter{
		Limit:      5,
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
		WalletType: "DRIVER",
		EntryType:  "CREDIT",
		Search:     "xyz",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), list.TotalCount)
	require.Len(t, list.Ledger, 1)
	assert.Equal(t, id1, list.Ledger[0].ID)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

// TestGetLedger_Handler: GET /admin/ledger dengan filter via query param ->
// envelope {success, data: {ledger, total_count}} + field snake_case.
func TestGetLedger_Handler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	id1 := uuid.New()
	wID := uuid.New()
	mDB.ExpectQuery("ORDER BY le.created_at DESC LIMIT").
		WithArgs(anyArgs(7)...). // wallet_type + entry_type + search x3 + limit + offset
		WillReturnRows(ledgerResultRows().
			AddRow(id1, wID, "CREDIT", decimal.NewFromInt(75000), "FOOD/"+testTxnID.String(), now, "ACTIVE", "settle"))
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs(anyArgs(5)...).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/admin/ledger?limit=5&offset=0&wallet_type=DRIVER&entry_type=CREDIT&search=foo", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"success":true`)
	assert.Contains(t, body, `"ledger"`)
	assert.Contains(t, body, `"total_count":1`)
	assert.Contains(t, body, `"entry_type":"CREDIT"`)
	assert.Contains(t, body, `"reference":"FOOD/`)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetLedger_InvalidParams: limit/offset/date tidak valid -> 400
// INVALID_REQUEST tanpa menyentuh database.
func TestGetLedger_InvalidParams(t *testing.T) {
	tests := []string{
		"/admin/ledger?limit=abc",
		"/admin/ledger?limit=0",
		"/admin/ledger?limit=-2",
		"/admin/ledger?offset=-1",
		"/admin/ledger?offset=xyz",
		"/admin/ledger?date_from=not-a-date",
		"/admin/ledger?date_to=2026-99-99",
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			mDB, err := pgxmock.NewPool()
			require.NoError(t, err)
			h := newTestHandler(t, mDB, nil)
			router := ledgerRouter(h)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
			assert.NoError(t, mDB.ExpectationsWereMet())
		})
	}
}

// TestVerifyLedger_Handler_Balanced: balance == (credit - debit) -> status
// "BALANCED" dengan discrepancy 0.
func TestVerifyLedger_Handler_Balanced(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(100000)))
	mDB.ExpectQuery(`SUM\(amount\) FILTER`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"debit", "credit"}).
			AddRow(decimal.NewFromInt(40000), decimal.NewFromInt(140000)))

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ledger/verify/"+testEscrowID.String(), nil))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"success":true`)
	assert.Contains(t, body, `"total_debit":40000`)
	assert.Contains(t, body, `"total_credit":140000`)
	assert.Contains(t, body, `"discrepancy":0`)
	assert.Contains(t, body, `"status":"BALANCED"`)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestVerifyLedger_Handler_Mismatch: saldo menyimpang dari agregat ledger ->
// status "MISMATCH" (types.ts) dengan discrepancy non-nol.
func TestVerifyLedger_Handler_Mismatch(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(decimal.NewFromInt(90000)))
	mDB.ExpectQuery(`SUM\(amount\) FILTER`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"debit", "credit"}).
			AddRow(decimal.NewFromInt(40000), decimal.NewFromInt(140000)))

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ledger/verify/"+testEscrowID.String(), nil))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"discrepancy":-10000`)
	assert.Contains(t, body, `"status":"MISMATCH"`)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestVerifyLedger_Handler_NotFound: wallet tidak ada -> 404 WALLET_NOT_FOUND.
func TestVerifyLedger_Handler_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("balance FROM wallets").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"balance"}))

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ledger/verify/"+testEscrowID.String(), nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "WALLET_NOT_FOUND")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestVerifyLedger_InvalidWalletID: wallet_id bukan UUID -> 422.
func TestVerifyLedger_InvalidWalletID(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/ledger/verify/not-a-uuid", nil))

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_WALLET_ID")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestExportLedger_Handler: POST /admin/ledger/export dengan filter -> CSV
// (Content-Type: text/csv, filename ledger-export-*.csv, isi baris + header).
func TestExportLedger_Handler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	id1, id2 := uuid.New(), uuid.New()
	wID := uuid.New()
	mDB.ExpectQuery("ORDER BY le.created_at DESC").
		WithArgs(anyArgs(5)...). // wallet_type + entry_type + search x3 (tanpa pagination)
		WillReturnRows(ledgerResultRows().
			AddRow(id1, wID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT/"+testTxnID.String(), now, "ACTIVE", "settle").
			AddRow(id2, wID, "CREDIT", decimal.NewFromInt(50000), "REVERSAL/"+testTxnID.String(), now, "REVERSED", "refund"))

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/ledger/export",
		strings.NewReader(`{"wallet_type":"DRIVER","entry_type":"CREDIT","search":"ref"}`)))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "ledger-export-")
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".csv")
	body := w.Body.String()
	assert.Contains(t, body, "id,wallet_id,entry_type,amount,reference,created_at,status")
	assert.Contains(t, body, id1.String())
	assert.Contains(t, body, id2.String())
	assert.Contains(t, body, "100000,SETTLEMENT/")
	assert.Contains(t, body, ",DEBIT,")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestExportLedger_InvalidBody: body bukan JSON -> 400 INVALID_REQUEST.
func TestExportLedger_InvalidBody(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newTestHandler(t, mDB, nil)
	router := ledgerRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/ledger/export",
		strings.NewReader(`{"oops":`)))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
	assert.NoError(t, mDB.ExpectationsWereMet())
}