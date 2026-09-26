//go:build integration

// Package send_test — integration test end-to-end (HTTP + PostgreSQL + Redis)
// untuk G-Send (Task 3.5 s/d 3.6 + auto-cancel Task 3.7): create order WALLET
// single & multi-stop, accept driver + capacity guard, emergency cancel driver,
// settlement 3-way, dan auto-cancel worker.
//
// Menyalin setup router dari cmd/api/main.go dan memakai PostgreSQL + Redis
// dari docker-compose. File ini TIDAK ikut kompilasi pada `go test ./...`
// biasa (build tag `integration`); jalankan dengan:
//
//	env DATABASE_URL=... go test -tags integration ./internal/send/ -run TC_SEND -v
//
// Prasyarat: PostgreSQL ter-migrate (docker-compose) + Redis:6380 up.
package send_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/auth"
	"github.com/g-flow/g-flow/internal/db"
	"github.com/g-flow/g-flow/internal/middleware"
	"github.com/g-flow/g-flow/internal/send"
	"github.com/g-flow/g-flow/internal/wallet"
	"github.com/g-flow/g-flow/internal/worker"
)

// testEnv menyimpan dependensi yang dipakai membangun router + akses DB test.
type testEnv struct {
	pool   *pgxpool.Pool
	r      *gin.Engine
	ledger *wallet.LedgerService
}

// apiResp adalah envelope respons standar seluruh API G-Flow.
type apiResp struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Koordinat default (jarak pickup→stop = 0 → degenerate allocation; total
// fare deterministik = base_fare 15.000).
const (
	testLat = -6.200000
	testLng = 106.816666
)

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset, melewati integration test")
	}
	pool, err := db.NewDB(db.Config{DatabaseURL: url})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(pool) })

	if _, err := pool.Exec(context.Background(),
		`UPDATE wallets SET balance = 0 WHERE user_id IS NULL`); err != nil {
		t.Fatalf("gagal reset saldo wallet sistem: %v", err)
	}
	return pool
}

// newTestEnv membangun router Gin yang menyalin setup cmd/api/main.go
// (subset route yang dibutuhkan send flow).
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	pool := setupPool(t)
	gin.SetMode(gin.TestMode)

	rdb, err := redis.ParseURL("redis://localhost:6380/0")
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	jwtSvc := auth.NewJWTService("send-integration-test-secret")
	blacklist := auth.NewBlacklistService(rd)
	authHandler := auth.NewHandler(jwtSvc, blacklist, pool)

	walletLedger := wallet.NewLedgerService(pool)
	walletHandler := wallet.NewHandler(
		wallet.NewService(wallet.NewRepository(pool), walletLedger, rd, pool))

	sendHandler := send.NewHandler(send.NewService(
		send.NewRepository(pool), pool, rd, walletLedger))

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	api := r.Group("/api/v1", middleware.AuthMiddleware(jwtSvc, blacklist))
	{
		api.POST("/wallets/:wallet_id/topup", walletHandler.TopUp)
		api.GET("/wallets/:wallet_id/balance", walletHandler.GetBalance)
	}

	sendOrders := r.Group("/api/v1/send-orders", middleware.AuthMiddleware(jwtSvc, blacklist))
	{
		sendOrders.POST("", auth.RBACMiddleware("customer"), sendHandler.CreateSendOrder)
		sendOrders.GET("/:id", sendHandler.GetSendOrder)
		sendOrders.PATCH("/:id", sendHandler.UpdateSendOrderStatus)
		sendOrders.POST("/:id/accept", auth.RBACMiddleware("driver"), sendHandler.AcceptSendOrder)
		sendOrders.PATCH("/:id/stops/:stop_id", auth.RBACMiddleware("driver"), sendHandler.UpdateSendOrderStop)
	}

	return &testEnv{pool: pool, r: r, ledger: walletLedger}
}

// doJSON mengirim request HTTP. Goroutine-safe.
func doJSON(r *gin.Engine, method, path string, body any, token, idemKey string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idemKey != "" {
		req.Header.Set("X-Idempotency-Key", idemKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func mustResp(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var out apiResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

// registerAndLogin mendaftar user (customer/driver) lalu login.
func registerAndLogin(t *testing.T, r *gin.Engine, userType string) (string, uuid.UUID) {
	t.Helper()
	email := "sit" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"
	body := gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Send IT " + userType,
		"user_type": userType,
	}
	if userType == "driver" {
		body["vehicle_type"] = "motorcycle"
		body["vehicle_plate"] = "B 9876 XY"
		body["license_number"] = "SIM-SEND-42"
	}

	w := doJSON(r, http.MethodPost, "/api/v1/auth/register", body, "", "")
	require.Equal(t, http.StatusCreated, w.Code, "register: %s", w.Body.String())

	w = doJSON(r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "", "")
	require.Equal(t, http.StatusOK, w.Code, "login: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		AccessToken string    `json:"access_token"`
		UserID      uuid.UUID `json:"user_id"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	require.NotEmpty(t, data.AccessToken)
	return data.AccessToken, data.UserID
}

// --- helpers akses DB ---

// setWalletBalance menimpa saldo wallet yang sudah ada (dipakai untuk
// memberi saldo awal pada wallet hasil auto-create /auth/register).
func (e *testEnv) setWalletBalance(t *testing.T, ctx context.Context, walletID uuid.UUID, balance decimal.Decimal) {
	t.Helper()
	_, err := e.pool.Exec(ctx,
		`UPDATE wallets SET balance = $2, updated_at = NOW() WHERE id = $1`, walletID, balance)
	require.NoError(t, err)
}

func (e *testEnv) getWalletID(t *testing.T, ctx context.Context, userID uuid.UUID, walletType string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.pool.QueryRow(ctx,
		`SELECT id FROM wallets WHERE user_id = $1 AND wallet_type = $2`, userID, walletType).Scan(&id)
	require.NoError(t, err)
	return id
}

func (e *testEnv) getBalance(t *testing.T, ctx context.Context, walletID uuid.UUID) decimal.Decimal {
	t.Helper()
	var bal decimal.Decimal
	err := e.pool.QueryRow(ctx, `SELECT balance FROM wallets WHERE id = $1`, walletID).Scan(&bal)
	require.NoError(t, err)
	return bal
}

func (e *testEnv) systemWalletBalance(t *testing.T, ctx context.Context, walletType string) decimal.Decimal {
	t.Helper()
	var bal decimal.Decimal
	err := e.pool.QueryRow(ctx,
		`SELECT balance FROM wallets WHERE user_id IS NULL AND wallet_type = $1`, walletType).Scan(&bal)
	require.NoError(t, err)
	return bal
}

func topupCustomer(t *testing.T, e *testEnv, token string, walletID uuid.UUID, amount string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup",
		gin.H{"amount": amount, "idempotency_key": uuid.New().String()}, token, "")
	require.Equal(t, http.StatusOK, w.Code, "topup: %s", w.Body.String())
}

func assertLedgerBalanced(t *testing.T, e *testEnv, ctx context.Context, referenceID uuid.UUID) {
	t.Helper()
	rows, err := e.pool.Query(ctx, `
		SELECT entry_type, amount FROM ledger_entries
		WHERE reference_id = $1 AND is_reversed = FALSE`, referenceID)
	require.NoError(t, err)
	defer rows.Close()

	debit, credit := decimal.Zero, decimal.Zero
	for rows.Next() {
		var typ string
		var amt decimal.Decimal
		require.NoError(t, rows.Scan(&typ, &amt))
		switch typ {
		case "DEBIT":
			debit = debit.Add(amt)
		case "CREDIT":
			credit = credit.Add(amt)
		default:
			t.Fatalf("entry_type tak dikenal: %q", typ)
		}
	}
	require.NoError(t, rows.Err())
	require.True(t, debit.IsPositive(), "tidak ada entry ledger untuk reference_id %v", referenceID)
	require.True(t, debit.Equal(credit),
		"ledger tidak seimbang untuk %v: DEBIT=%v CREDIT=%v", referenceID, debit, credit)
}

// --- helpers alur bisnis G-Send ---

// stopTemplate membangun daftar stop request (jarak nol terhadap pickup).
func stopTemplate(stops int) []gin.H {
	out := make([]gin.H, 0, stops)
	for i := 0; i < stops; i++ {
		out = append(out, gin.H{
			"recipient_name":   "Penerima " + uuid.New().String(),
			"recipient_phone":  "081200000" + string(rune('0'+i)),
			"delivery_address": "Jl. Test Send " + string(rune('0'+i)),
			"delivery_lat":     testLat,
			"delivery_lng":     testLng,
		})
	}
	return out
}

// createSendOrder membuat send order WALLET. Mengembalikan order id, status,
// dan total fare (deterministik: base_fare 15.000 karena jarak 0).
func createSendOrder(t *testing.T, e *testEnv, token string, stops int) (uuid.UUID, string, decimal.Decimal) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/send-orders", gin.H{
		"pickup_lat":        testLat,
		"pickup_lng":        testLng,
		"pickup_address":    "Jl. Test Pickup",
		"package_weight_kg": 1,
		"package_type":      "STANDARD",
		"declared_value":    0,
		"payment_method":    "WALLET",
		"stops":             stopTemplate(stops),
	}, token, uuid.New().String())
	require.Equal(t, http.StatusCreated, w.Code, "create send order: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		ID           uuid.UUID       `json:"id"`
		Status       string          `json:"status"`
		TotalFare    decimal.Decimal `json:"total_fare"`
		EscrowAmount decimal.Decimal `json:"escrow_amount"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	require.Equal(t, "SEARCHING_DRIVER", data.Status)
	require.True(t, data.EscrowAmount.Equal(data.TotalFare), "escrow harus = total fare")
	return data.ID, data.Status, data.TotalFare
}

// acceptSendOrder mengirim POST /send-orders/:id/accept. Mengembalikan kode
// status HTTP + status response.
func acceptSendOrder(t *testing.T, e *testEnv, driverToken string, orderID uuid.UUID) (int, string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/send-orders/"+orderID.String()+"/accept",
		nil, driverToken, uuid.New().String())
	out := mustResp(t, w)
	status := ""
	if w.Code == http.StatusOK {
		var data struct {
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal(out.Data, &data))
		status = data.Status
	}
	return w.Code, status
}

// updateSendStatus mengirim PATCH /send-orders/:id/status.
func updateSendStatus(t *testing.T, e *testEnv, token string, orderID uuid.UUID, status, reason string) (int, string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPatch, "/api/v1/send-orders/"+orderID.String(),
		gin.H{"status": status, "reason": reason}, token, "")
	if w.Code != http.StatusOK {
		t.Logf("PATCH send-orders/%s status=%s -> %d: %s", orderID, status, w.Code, w.Body.String())
	}
	out := mustResp(t, w)
	var data struct {
		Status string `json:"status"`
	}
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(out.Data, &data))
	}
	return w.Code, data.Status
}

// getSendStopIDs mengembalikan ID stop terurut stop_number.
func (e *testEnv) getSendStopIDs(t *testing.T, ctx context.Context, orderID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := e.pool.Query(ctx,
		`SELECT id FROM send_order_stops WHERE order_id = $1 ORDER BY stop_number`, orderID)
	require.NoError(t, err)
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	return ids
}

// completeSendStop mengirim PATCH /send-orders/:id/stops/:stop_id COMPLETED.
// Mengembalikan order_status + settled dari respons.
func completeSendStop(t *testing.T, e *testEnv, driverToken string, orderID, stopID uuid.UUID) (string, bool) {
	t.Helper()
	w := doJSON(e.r, http.MethodPatch, "/api/v1/send-orders/"+orderID.String()+"/stops/"+stopID.String(),
		gin.H{"status": "COMPLETED", "delivery_photo_url": "https://example.test/photo.jpg"}, driverToken, "")
	require.Equal(t, http.StatusOK, w.Code, "complete stop: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		OrderStatus string `json:"order_status"`
		Settled     bool   `json:"settled"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	return data.OrderStatus, data.Settled
}

// newDriver mendaftarkan driver lalu memakai wallet DRIVER yang sudah dibuat
// otomatis oleh /auth/register (createWalletsForUser di internal/auth).
// Helper ini TIDAK boleh INSERT wallet lagi — wallet_user_type_unique akan
// menolak (SQLSTATE 23505). Saldo awal di-set pada wallet yang sudah ada.
func newDriver(t *testing.T, e *testEnv, initialBalance int) (string, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	token, driverID := registerAndLogin(t, e.r, "driver")
	walletID := e.getWalletID(t, ctx, driverID, "DRIVER")
	e.setWalletBalance(t, ctx, walletID, decimal.NewFromInt(int64(initialBalance)))
	return token, driverID, walletID
}

// setDriverIdle (re-)menandai working_status driver IDLE. Dipakai untuk
// mensimulasikan ketersediaan driver antar order pada skenario capacity.
func (e *testEnv) setDriverIdle(t *testing.T, ctx context.Context, driverID uuid.UUID) {
	t.Helper()
	_, err := e.pool.Exec(ctx, `UPDATE users SET working_status = 'IDLE' WHERE id = $1`, driverID)
	require.NoError(t, err)
}

// getSendOrderStatus membaca status send_orders dari DB.
func (e *testEnv) getSendOrderStatus(t *testing.T, ctx context.Context, orderID uuid.UUID) string {
	t.Helper()
	var status string
	err := e.pool.QueryRow(ctx, `SELECT status FROM send_orders WHERE id = $1`, orderID).Scan(&status)
	require.NoError(t, err)
	return status
}

// TC-SEND-001 — E2E WALLET single stop: create → driver accept → pickup →
// stop COMPLETED → auto DELIVERED/3-way settlement. Escrow dilepas 90/10,
// driver kembali IDLE, ledger seimbang.
func TestIntegrationSend_TC_SEND_001_E2EWalletSingleStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	senderToken, senderID := registerAndLogin(t, e.r, "customer")
	senderWalletID := e.getWalletID(t, ctx, senderID, "CUSTOMER")
	topupCustomer(t, e, senderToken, senderWalletID, "100000")

	driverToken, driverID, driverWalletID := newDriver(t, e, 0)

	orderID, _, totalFare := createSendOrder(t, e, senderToken, 1)
	commission := totalFare.Mul(decimal.RequireFromString("0.10")).Round(2) // 1500
	earning := totalFare.Sub(commission).Round(2)                           // 13500

	code, got := acceptSendOrder(t, e, driverToken, orderID)
	require.Equal(t, http.StatusOK, code, "accept send order: %d")
	require.Equal(t, "DRIVER_ASSIGNED", got)

	code, got = updateSendStatus(t, e, driverToken, orderID, "PICKED_UP", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "PICKED_UP", got)
	code, got = updateSendStatus(t, e, driverToken, orderID, "IN_TRANSIT", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "IN_TRANSIT", got)

	stops := e.getSendStopIDs(t, ctx, orderID)
	require.Len(t, stops, 1)

	orderStatus, settled := completeSendStop(t, e, driverToken, orderID, stops[0])
	require.Equal(t, "SETTLED", orderStatus)
	require.True(t, settled)

	require.Equal(t, "SETTLED", e.getSendOrderStatus(t, ctx, orderID))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus 0 setelah settlement")
	require.True(t, e.getBalance(t, ctx, driverWalletID).Equal(earning),
		"driver earning = %v, got %v", earning, e.getBalance(t, ctx, driverWalletID))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(commission),
		"platform = %v, got %v", commission, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM"))

	var ws string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT working_status FROM users WHERE id = $1`, driverID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// TC-SEND-002 — E2E WALLET multi-stop (2 stops): stop pertama COMPLETED tidak
// memicu settlement; setelah seluruh stop COMPLETED → auto DELIVERED + 3-way
// settlement. Allocated fare terverifikasi (sum == total_fare).
func TestIntegrationSend_TC_SEND_002_MultiStopSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	senderToken, senderID := registerAndLogin(t, e.r, "customer")
	senderWalletID := e.getWalletID(t, ctx, senderID, "CUSTOMER")
	topupCustomer(t, e, senderToken, senderWalletID, "100000")

	driverToken, driverID, driverWalletID := newDriver(t, e, 0)

	orderID, _, totalFare := createSendOrder(t, e, senderToken, 2)

	code, got := acceptSendOrder(t, e, driverToken, orderID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "DRIVER_ASSIGNED", got)
	code, got = updateSendStatus(t, e, driverToken, orderID, "PICKED_UP", "")
	require.Equal(t, http.StatusOK, code)
	code, got = updateSendStatus(t, e, driverToken, orderID, "IN_TRANSIT", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "IN_TRANSIT", got)

	stops := e.getSendStopIDs(t, ctx, orderID)
	require.Len(t, stops, 2)

	// Stop 1 COMPLETED → order belum settled.
	orderStatus, settled := completeSendStop(t, e, driverToken, orderID, stops[0])
	require.Equal(t, "IN_TRANSIT", orderStatus)
	require.False(t, settled)
	require.Equal(t, "IN_TRANSIT", e.getSendOrderStatus(t, ctx, orderID))

	// Stop 2 COMPLETED → seluruh stop selesai → auto DELIVERED → SETTLED.
	orderStatus, settled = completeSendStop(t, e, driverToken, orderID, stops[1])
	require.Equal(t, "SETTLED", orderStatus)
	require.True(t, settled)

	// Verifikasi allocated fare: SUM == total fare; jarak nol → stop terakhir
	// (stop 2) mengambil seluruh fare.
	var allocSum decimal.Decimal
	allocRows, err := e.pool.Query(ctx,
		`SELECT allocated_fare FROM send_order_stops WHERE order_id = $1 ORDER BY stop_number`, orderID)
	require.NoError(t, err)
	var allocs []decimal.Decimal
	for allocRows.Next() {
		var a decimal.Decimal
		require.NoError(t, allocRows.Scan(&a))
		allocs = append(allocs, a)
		allocSum = allocSum.Add(a)
	}
	require.NoError(t, allocRows.Err())
	require.Len(t, allocs, 2)
	require.True(t, allocSum.Equal(totalFare), "SUM allocated_fare = %v, want %v", allocSum, totalFare)
	require.True(t, allocs[0].IsZero(), "jarak nol: stop pertama mendapat 0, got %v", allocs[0])
	require.True(t, allocs[1].Equal(totalFare), "stop terakhir mengambil sisa, got %v", allocs[1])

	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero())
	require.True(t, e.getBalance(t, ctx, driverWalletID).Equal(totalFare.Mul(decimal.RequireFromString("0.90")).Round(2)))
	var ws string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT working_status FROM users WHERE id = $1`, driverID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// TC-SEND-003 — Capacity driver max 3: sekaligus menguji dua guard nyata:
// (1) driver BUSY → accept order lain ditolak 409 DRIVER_BUSY; (2) kapasitas
// 3 order aktif → order ke-4 ditolak 422 DRIVER_CAPACITY_EXCEEDED.
func TestIntegrationSend_TC_SEND_003_DriverCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	senderToken, senderID := registerAndLogin(t, e.r, "customer")
	senderWalletID := e.getWalletID(t, ctx, senderID, "CUSTOMER")
	topupCustomer(t, e, senderToken, senderWalletID, "200000")

	driverToken, driverID, _ := newDriver(t, e, 0)

	// (1) Guard BUSY: driver yang punya 1 order aktif tidak bisa accept lagi.
	order1, _, _ := createSendOrder(t, e, senderToken, 1)
	code, got := acceptSendOrder(t, e, driverToken, order1)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "DRIVER_ASSIGNED", got)

	order2, _, _ := createSendOrder(t, e, senderToken, 1)
	code, _ = acceptSendOrder(t, e, driverToken, order2)
	require.Equal(t, http.StatusConflict, code, "accept saat BUSY harus 409")

	// (2) Capacity max 3: antar-order, driver kembali tersedia (IDLE) → accept
	// order berikutnya. Setelah 3 order aktif + driver IDLE, order ke-4 → 422.
	order3, _, _ := createSendOrder(t, e, senderToken, 1)
	e.setDriverIdle(t, ctx, driverID)
	code, got = acceptSendOrder(t, e, driverToken, order2)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "DRIVER_ASSIGNED", got)

	e.setDriverIdle(t, ctx, driverID)
	code, got = acceptSendOrder(t, e, driverToken, order3)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "DRIVER_ASSIGNED", got)
	require.Equal(t, 3, e.countDriverActiveOrders(t, ctx, driverID))

	// Driver tersedia lagi → 3 aktif, sehingga guard kapasitas tersulut.
	e.setDriverIdle(t, ctx, driverID)
	order4, _, _ := createSendOrder(t, e, senderToken, 1)
	code, _ = acceptSendOrder(t, e, driverToken, order4)
	require.Equal(t, http.StatusUnprocessableEntity, code, "order ke-4 harus ditolak 422")
	require.Equal(t, "DRIVER_CAPACITY_EXCEEDED", e.lastErrorCode(t, order4, driverToken))
}

// countDriverActiveOrders menghitung send order aktif (DRIVER_ASSIGNED/
// PICKED_UP/IN_TRANSIT) milik driver.
func (e *testEnv) countDriverActiveOrders(t *testing.T, ctx context.Context, driverID uuid.UUID) int {
	t.Helper()
	var n int
	err := e.pool.QueryRow(ctx, `
		SELECT count(*) FROM send_orders
		WHERE driver_id = $1 AND status IN ('DRIVER_ASSIGNED', 'PICKED_UP', 'IN_TRANSIT')`,
		driverID).Scan(&n)
	require.NoError(t, err)
	return n
}

// lastErrorCode memanggil accept ulang dengan idempotency baru pada order yang
// sama untuk membaca kode error terakhir dari apiResp.Error.Code.
func (e *testEnv) lastErrorCode(t *testing.T, orderID uuid.UUID, driverToken string) string {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/send-orders/"+orderID.String()+"/accept",
		nil, driverToken, uuid.New().String())
	out := mustResp(t, w)
	require.NotNil(t, out.Error)
	return out.Error.Code
}

// TC-SEND-004 — Driver emergency cancel (setelah PICKED_UP): order CANCELLED,
// escrow WALLET di-refund penuh, working_status driver kembali IDLE.
func TestIntegrationSend_TC_SEND_004_DriverEmergencyCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	senderToken, senderID := registerAndLogin(t, e.r, "customer")
	senderWalletID := e.getWalletID(t, ctx, senderID, "CUSTOMER")
	topupCustomer(t, e, senderToken, senderWalletID, "100000")

	driverToken, driverID, _ := newDriver(t, e, 0)

	orderID, _, totalFare := createSendOrder(t, e, senderToken, 1)
	code, got := acceptSendOrder(t, e, driverToken, orderID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "DRIVER_ASSIGNED", got)
	code, got = updateSendStatus(t, e, driverToken, orderID, "PICKED_UP", "")
	require.Equal(t, http.StatusOK, code)

	// Driver emergency cancel dari PICKED_UP.
	code, got = updateSendStatus(t, e, driverToken, orderID, "CANCELLED", "driver darurat")
	require.Equal(t, http.StatusOK, code, "driver emergency cancel: %d")
	require.Equal(t, "CANCELLED", got)

	// Refund penuh escrow WALLET.
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus kembali 0, got %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"))
	require.True(t, e.getBalance(t, ctx, senderWalletID).Equal(decimal.NewFromInt(100000)),
		"sender balance harus pulih 100000, got %v", e.getBalance(t, ctx, senderWalletID))
	require.False(t, e.getBalance(t, ctx, senderWalletID).Equal(decimal.NewFromInt(100000).Sub(totalFare)))

	var ws string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT working_status FROM users WHERE id = $1`, driverID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// TC-SEND-005 — Auto-cancel worker (Task 3.7): send order SEARCHING_DRIVER
// yang menunggu driver > 10 menit dibatalkan otomatis. WALLET → escrow
// di-refund penuh; CASH → cancel tanpa escrow.
func TestIntegrationSend_TC_SEND_005_AutoCancelExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	_, err := e.pool.Exec(ctx, `
		TRUNCATE send_order_events, send_order_stops, send_orders,
		         food_order_events, food_order_items, food_orders CASCADE`)
	require.NoError(t, err)

	acw := worker.NewWorker(worker.NewRepository(e.pool), e.pool, nil, e.ledger)

	senderToken, senderID := registerAndLogin(t, e.r, "customer")
	senderWalletID := e.getWalletID(t, ctx, senderID, "CUSTOMER")
	topupCustomer(t, e, senderToken, senderWalletID, "100000")

	walletOrderID, _, totalFare := createSendOrder(t, e, senderToken, 1)

	// CASH send order: SEARCHING_DRIVER tanpa escrow.
	cashOrderID, _, _ := createSendOrderCash(t, e, senderToken)
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(totalFare),
		"escrow sebelum auto-cancel = %v, want %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"), totalFare)

	_, err = e.pool.Exec(ctx,
		`UPDATE send_orders SET created_at = NOW() - INTERVAL '11 minutes' WHERE id = ANY($1)`,
		[]uuid.UUID{walletOrderID, cashOrderID})
	require.NoError(t, err)

	cancelled := acw.CancelSendOrders(ctx)
	require.Equal(t, 2, cancelled, "dua send order harus di-auto-cancel")

	// WALLET: refund penuh.
	require.Equal(t, "CANCELLED", e.getSendOrderStatus(t, ctx, walletOrderID))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus 0 setelah refund, got %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"))
	require.True(t, e.getBalance(t, ctx, senderWalletID).Equal(decimal.NewFromInt(100000)),
		"sender balance harus pulih 100000, got %v", e.getBalance(t, ctx, senderWalletID))

	// CASH: CANCELLED tanpa pergerakan uang.
	require.Equal(t, "CANCELLED", e.getSendOrderStatus(t, ctx, cashOrderID))

	assertLedgerBalanced(t, e, ctx, walletOrderID)
}

// createSendOrderCash membuat send order CASH (tanpa escrow).
func createSendOrderCash(t *testing.T, e *testEnv, token string) (uuid.UUID, string, decimal.Decimal) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/send-orders", gin.H{
		"pickup_lat":        testLat,
		"pickup_lng":        testLng,
		"pickup_address":    "Jl. Test Pickup",
		"package_weight_kg": 1,
		"package_type":      "STANDARD",
		"declared_value":    0,
		"payment_method":    "CASH",
		"stops":             stopTemplate(1),
	}, token, uuid.New().String())
	require.Equal(t, http.StatusCreated, w.Code, "create send order: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		ID           uuid.UUID       `json:"id"`
		Status       string          `json:"status"`
		TotalFare    decimal.Decimal `json:"total_fare"`
		EscrowAmount decimal.Decimal `json:"escrow_amount"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	require.Equal(t, "SEARCHING_DRIVER", data.Status)
	require.True(t, data.EscrowAmount.IsZero(), "CASH tidak boleh ada escrow")
	return data.ID, data.Status, data.TotalFare
}
