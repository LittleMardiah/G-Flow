package admin

// Test untuk fitur dashboard admin (STEP A1 v2):
//   - GET /admin/dashboard/kpis
//   - GET /admin/dashboard/transactions
//
// Pola sama seperti test reversal/login: pgxmock untuk DB, gin httptest untuk
// handler. Semua expectation DB dimock berurutan mengikuti eksekusi service.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kpiExpectations mendaftarkan kelima query repo KPI pada mock secara
// berurutan sesuai urutan pemanggilannya di Service.GetDashboardKPIs.
func kpiExpectations(mDB pgxmock.PgxPoolIface) {
	mDB.ExpectQuery("SELECT 1 FROM ride_orders").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(3)))
	mDB.ExpectQuery("INTERVAL '24 hours'").
		WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(decimal.NewFromInt(250000)))
	mDB.ExpectQuery("actual_fare, estimated_fare").
		WillReturnRows(pgxmock.NewRows([]string{"avg"}).AddRow(decimal.NewFromInt(50000)))
	mDB.ExpectQuery("settled_at::date = CURRENT_DATE").
		WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(decimal.NewFromInt(120000)))
	mDB.ExpectQuery("GROUP BY status::text").
		WillReturnRows(pgxmock.NewRows([]string{"status", "count"}).
			AddRow("CREATED", int64(2)).
			AddRow("SETTLED", int64(1)))
}

func dashboardRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/dashboard/kpis", h.GetDashboardKPIs)
	r.GET("/admin/dashboard/transactions", h.GetDashboardTransactions)
	return r
}

// TestDashboardKPIs_Service: GetDashboardKPIs merangkum 5 query KPI menjadi
// payload snake_case yang sesuai apps/admin_web/src/lib/types.ts.
func TestDashboardKPIs_Service(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	kpiExpectations(mDB)

	svc := newSvc(t, mDB)
	k, err := svc.GetDashboardKPIs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(3), k.ActiveOrders)
	assert.True(t, decimal.Decimal(k.TotalTransactionVolume).Equal(decimal.NewFromInt(250000)))
	assert.True(t, decimal.Decimal(k.AvgFare).Equal(decimal.NewFromInt(50000)))
	assert.True(t, decimal.Decimal(k.RevenueToday).Equal(decimal.NewFromInt(120000)))
	assert.Equal(t, float64(0), k.ErrorRate)
	require.Len(t, k.OrderStatusBreakdown, 2)
	assert.Equal(t, "CREATED", k.OrderStatusBreakdown[0].Status)
	assert.Equal(t, int64(2), k.OrderStatusBreakdown[0].Count)
	assert.Equal(t, "SETTLED", k.OrderStatusBreakdown[1].Status)

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestDashboardTransactions_Service: GetDashboardTransactions mengembalikan
// total_count + daftar transaksi dengan status turunan is_reversed.
func TestDashboardTransactions_Service(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	row1 := uuid.New()
	row2 := uuid.New()
	wID := uuid.New()
	mDB.ExpectQuery("FROM ledger_entries").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
	mDB.ExpectQuery("ORDER BY created_at DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "wallet_id", "entry_type", "amount", "reference", "created_at", "status", "note",
		}).
			AddRow(row1, wID, "DEBIT", decimal.NewFromInt(100000), "SETTLEMENT/"+testTxnID.String(), now, "ACTIVE", "settle").
			AddRow(row2, wID, "CREDIT", decimal.NewFromInt(50000), "REVERSAL/"+testTxnID.String(), now, "REVERSED", "refund"))

	svc := newSvc(t, mDB)
	list, err := svc.GetDashboardTransactions(context.Background(), 10, 0)
	require.NoError(t, err)

	assert.Equal(t, int64(2), list.TotalCount)
	require.Len(t, list.Transactions, 2)
	assert.Equal(t, row1, list.Transactions[0].ID)
	assert.Equal(t, "DEBIT", list.Transactions[0].EntryType)
	assert.Equal(t, "ACTIVE", list.Transactions[0].Status)
	assert.Equal(t, row2, list.Transactions[1].ID)
	assert.Equal(t, "REVERSED", list.Transactions[1].Status)
	assert.True(t, decimal.Decimal(list.Transactions[0].Amount).Equal(decimal.NewFromInt(100000)))

	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestDashboardKPIs_Handler: seluruh alur via HTTP — envelope {success, data}
// dengan field KPI snake_case.
func TestDashboardKPIs_Handler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	kpiExpectations(mDB)

	h := newTestHandler(t, mDB, nil)
	router := dashboardRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/dashboard/kpis", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"success":true`)
	assert.Contains(t, body, `"active_orders":3`)
	assert.Contains(t, body, `"total_transaction_volume":250000`)
	assert.Contains(t, body, `"avg_fare":50000`)
	assert.Contains(t, body, `"revenue_today":120000`)
	assert.Contains(t, body, `"error_rate":0`)
	assert.Contains(t, body, `"order_status_breakdown"`)
	assert.Contains(t, body, `"status":"CREATED"`)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestDashboardTransactions_Handler: via HTTP dengan query limit/offset.
func TestDashboardTransactions_Handler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	row1 := uuid.New()
	wID := uuid.New()
	mDB.ExpectQuery("FROM ledger_entries").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mDB.ExpectQuery("ORDER BY created_at DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "wallet_id", "entry_type", "amount", "reference", "created_at", "status", "note",
		}).
			AddRow(row1, wID, "CREDIT", decimal.NewFromInt(75000), "FOOD/"+testTxnID.String(), now, "ACTIVE", "settle"))

	h := newTestHandler(t, mDB, nil)
	router := dashboardRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/dashboard/transactions?limit=5&offset=0", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"success":true`)
	assert.Contains(t, body, `"total_count":1`)
	assert.Contains(t, body, `"entry_type":"CREDIT"`)
	assert.Contains(t, body, `"reference":"FOOD/`)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestDashboardTransactions_InvalidParams: limit/offset invalid -> 400
// INVALID_REQUEST tanpa menyentuh database.
func TestDashboardTransactions_InvalidParams(t *testing.T) {
	tests := []string{
		"/admin/dashboard/transactions?limit=abc",
		"/admin/dashboard/transactions?limit=0",
		"/admin/dashboard/transactions?limit=-2",
		"/admin/dashboard/transactions?offset=-1",
		"/admin/dashboard/transactions?offset=xyz",
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			mDB, err := pgxmock.NewPool()
			require.NoError(t, err)
			h := newTestHandler(t, mDB, nil)
			router := dashboardRouter(h)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
			assert.NoError(t, mDB.ExpectationsWereMet())
		})
	}
}

// TestDashboardKPIs_Handler_RepoError: error query -> 500 INTERNAL_SERVER_ERROR.
func TestDashboardKPIs_Handler_RepoError(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("SELECT 1 FROM ride_orders").
		WillReturnError(errors.New("boom"))

	h := newTestHandler(t, mDB, nil)
	router := dashboardRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/dashboard/kpis", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_SERVER_ERROR")
	assert.NoError(t, mDB.ExpectationsWereMet())
}