//go:build integration

// Package ride_test — integration test end-to-end (HTTP + database nyata)
// untuk fase lanjutan G-Ride: F005 Dispatch (Accept), Cancel, Auto-Cancel
// (expired order), Settlement WALLET & CASH.
//
// Menyalin setup router dari cmd/api/main.go dan memakai PostgreSQL + Redis
// dari docker-compose. File ini TIDAK ikut kompilasi pada `go test ./...`
// biasa (build tag `integration`); jalankan dengan:
//
//	env DATABASE_URL=... go test -tags integration ./internal/ride/ -run Integration -v
//
// Prasyarat: PostgreSQL ter-migrate (docker-compose) + Redis:6380 up.
package ride_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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
	"github.com/g-flow/g-flow/internal/ride"
	"github.com/g-flow/g-flow/internal/wallet"
)

// testEnv menyimpan dependensi yang dipakai untuk membangun router test.
type testEnv struct {
	pool *pgxpool.Pool
	r    *gin.Engine
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

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset, melewati integration test")
	}
	pool, err := db.NewDB(db.Config{DatabaseURL: url})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(pool) })

	// Reset saldo wallet sistem ke 0 agar test idempotent (system wallet
	// dipakai escrow/settlement & hanya bisa bertambah lintas run).
	if _, err := pool.Exec(context.Background(),
		`UPDATE wallets SET balance = 0 WHERE user_id IS NULL`); err != nil {
		t.Fatalf("gagal reset saldo wallet sistem: %v", err)
	}
	return pool
}

// newTestEnv membangun router Gin yang menyalin setup cmd/api/main.go.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	pool := setupPool(t)
	gin.SetMode(gin.TestMode)

	rdb, err := redis.ParseURL("redis://localhost:6380/0")
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	jwtSvc := auth.NewJWTService("ride-integration-test-secret")
	blacklist := auth.NewBlacklistService(rd)
	authHandler := auth.NewHandler(jwtSvc, blacklist, pool)

	walletLedger := wallet.NewLedgerService(pool)
	walletHandler := wallet.NewHandler(
		wallet.NewService(wallet.NewRepository(pool), walletLedger, rd, pool))

	rideHandler := ride.NewHandler(ride.NewService(
		ride.NewRepository(pool), walletLedger, rd, pool))

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	api := r.Group("/api/v1", middleware.AuthMiddleware(jwtSvc, blacklist))
	{
		api.POST("/rides/book", rideHandler.BookRide)
		api.POST("/rides/:order_id/accept", rideHandler.AcceptOrder)
		api.PATCH("/rides/:order_id/status", rideHandler.UpdateStatus)
		api.POST("/wallets/:wallet_id/topup", walletHandler.TopUp)
		api.GET("/wallets/:wallet_id/balance", walletHandler.GetBalance)
	}

	return &testEnv{pool: pool, r: r}
}

// doJSON mengirim request HTTP. Goroutine-safe (TIDAK menyentuh *testing.T)
// sehingga bisa dipakai bersamaan pada test konkurrensi accept.
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

// registerAndLogin mendaftar user (customer/driver) lalu login; mengembalikan
// access token + user id.
func registerAndLogin(t *testing.T, r *gin.Engine, userType string) (string, uuid.UUID) {
	t.Helper()
	email := "it" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"
	body := gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Ride IT " + userType,
		"user_type": userType,
	}
	if userType == "driver" {
		body["vehicle_type"] = "motorcycle"
		body["vehicle_plate"] = "B 9876 XY"
		body["license_number"] = "SIM-RIDE-42"
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

// newDriver mendaftarkan driver + membuat wallet DRIVER dengan saldo awal.
func newDriver(t *testing.T, e *testEnv, initialBalance decimal.Decimal) (token string, driverID uuid.UUID, walletID uuid.UUID) {
	t.Helper()
	token, driverID = registerAndLogin(t, e.r, "driver")
	walletID = e.createWallet(t, context.Background(), driverID, "DRIVER", initialBalance)
	return token, driverID, walletID
}

// --- helpers akses DB ---

func (e *testEnv) createWallet(t *testing.T, ctx context.Context, userID uuid.UUID, walletType string, balance decimal.Decimal) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.pool.QueryRow(ctx, `
		INSERT INTO wallets (user_id, wallet_type, balance, status)
		VALUES ($1, $2, $3, 'ACTIVE')
		RETURNING id`, userID, walletType, balance).Scan(&id)
	require.NoError(t, err)
	return id
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

func getOrder(t *testing.T, e *testEnv, ctx context.Context, orderID uuid.UUID) *ride.RideOrder {
	t.Helper()
	o, err := ride.NewRepository(e.pool).GetOrderByID(ctx, orderID)
	require.NoError(t, err)
	return o
}

// --- helpers alur bisnis ---

func topupCustomer(t *testing.T, e *testEnv, token string, walletID uuid.UUID, amount string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup",
		gin.H{"amount": amount, "idempotency_key": uuid.New().String()}, token, "")
	require.Equal(t, http.StatusOK, w.Code, "topup: %s", w.Body.String())
}

// Koordinat default (jarak pendek ~6.7km → fare kecil).
const (
	defaultPickupLat  = -6.200000
	defaultPickupLng  = 106.816666
	defaultDropoffLat = -6.260000
	defaultDropoffLng = 106.816666
)

func bookRide(t *testing.T, e *testEnv, token, paymentMethod string) (uuid.UUID, decimal.Decimal) {
	t.Helper()
	return bookRideCoords(t, e, token, paymentMethod,
		defaultPickupLat, defaultPickupLng, defaultDropoffLat, defaultDropoffLng)
}

func bookRideCoords(t *testing.T, e *testEnv, token, paymentMethod string, pLat, pLng, dLat, dLng float64) (uuid.UUID, decimal.Decimal) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/book", gin.H{
		"pickup_lat":     pLat,
		"pickup_lng":     pLng,
		"pickup_address": "Jl. Test A",
		"dropoff_lat":    dLat,
		"dropoff_lng":    dLng,
		"dropoff_address": "Jl. Test B",
		"payment_method":  paymentMethod,
	}, token, uuid.New().String())
	require.Equal(t, http.StatusCreated, w.Code, "book: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		OrderID       uuid.UUID `json:"order_id"`
		Status        string    `json:"status"`
		EstimatedFare decimal.Decimal `json:"estimated_fare"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	require.Equal(t, "SEARCHING_DRIVER", data.Status)
	require.NotEqual(t, uuid.Nil, data.OrderID)
	return data.OrderID, data.EstimatedFare
}

func updateRideStatus(t *testing.T, e *testEnv, token string, orderID uuid.UUID, status, reason string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(e.r, http.MethodPatch, "/api/v1/rides/"+orderID.String()+"/status",
		gin.H{"status": status, "reason": reason}, token, "")
}

// updatedStatus membaca field data.status dari respons PATCH /status.
func updatedStatus(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	out := mustResp(t, w)
	var data struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	return data.Status
}

// assertLedgerBalanced memastikan double-entry ledger untuk satu reference_id
// seimbang (SUM DEBIT == SUM CREDIT), sesuai trigger validate_ledger_balance.
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

// assertCancelEvent memastikan event CANCELLED terakhir punya reason yang
// diharapkan dan metadata audit berisi reason tersebut.
func assertCancelEvent(t *testing.T, e *testEnv, ctx context.Context, orderID uuid.UUID, reason string) {
	t.Helper()
	var gotReason *string
	var metadata []byte
	err := e.pool.QueryRow(ctx, `
		SELECT reason, metadata FROM ride_order_events
		WHERE order_id = $1 AND to_status = 'CANCELLED'
		ORDER BY created_at DESC LIMIT 1`, orderID).Scan(&gotReason, &metadata)
	require.NoError(t, err)
	require.NotNil(t, gotReason)
	require.Equal(t, reason, *gotReason)
	require.Contains(t, string(metadata), reason)
}

// TC-INT-RD-001 — E2E WALLET: booking → accept → arrival → trip → completed
// (settlement otomatis 80/20). Escrow dilepas penuh, driver digaji 80%,
// platform 20%, ledger double-entry seimbang.
func TestIntegrationRide_E2EWalletSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "500000")
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(500000)))

	orderID, fare := bookRide(t, e, custToken, "WALLET")

	// Escrow tertahan: saldo customer berkurang sesuai fare.
	wantAfterEscrow := decimal.NewFromInt(500000).Sub(fare)
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(wantAfterEscrow),
		"customer balance = %v, want %v", e.getBalance(t, ctx, custWallet), wantAfterEscrow)

	escrowBefore := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")

	dToken, _, drvWallet := newDriver(t, e, decimal.Zero)

	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

	// Driver sibuk → tidak bisa menerima (order) kedua kalinya.
	w = doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusConflict, w.Code, "accept kedua seharusnya 409: %s", w.Body.String())

	for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
		w = updateRideStatus(t, e, dToken, orderID, st, "")
		require.Equal(t, http.StatusOK, w.Code, "status %s: %s", st, w.Body.String())
	}

	// COMPLETED otomatis memicu settlement → respons status SETTLED.
	w = updateRideStatus(t, e, dToken, orderID, "COMPLETED", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "SETTLED", updatedStatus(t, w))

	commission := fare.Mul(decimal.RequireFromString("0.20")).Round(2)
	earning := fare.Sub(commission)

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "SETTLED", order.Status)
	require.True(t, order.IsSettled)
	require.NotNil(t, order.DriverEarning)
	require.True(t, order.DriverEarning.Equal(earning), "earning=%v want %v", order.DriverEarning, earning)
	require.NotNil(t, order.PlatformCommission)
	require.True(t, order.PlatformCommission.Equal(commission), "commission=%v want %v", order.PlatformCommission, commission)

	// Escrow dilepas penuh (kembali ke saldo sebelum booking); platform menambah komisi.
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBefore.Sub(fare)))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore.Add(commission)))
	// Customer tidak berubah pasca-settlement; driver 0 + earning.
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(wantAfterEscrow))
	require.True(t, e.getBalance(t, ctx, drvWallet).Equal(earning), "driver wallet=%v want %v", e.getBalance(t, ctx, drvWallet), earning)

	// Driver kembali IDLE setelah selesai bertugas.
	require.NotNil(t, order.DriverID)
	var ws string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT working_status FROM users WHERE id = $1`, *order.DriverID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// TC-INT-RD-002 — Konkurrensi Accept: 10 driver berlomba menerima satu order.
// Tepat satu 200, sembilan 409; order di-assign satu driver & satu event.
func TestIntegrationRide_ConcurrentAccept(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")
	orderID, _ := bookRide(t, e, custToken, "WALLET")

	const n = 10
	drvTokens := make([]string, n)
	for i := 0; i < n; i++ {
		tok, _, _ := newDriver(t, e, decimal.NewFromInt(100000))
		drvTokens[i] = tok
	}

	type result struct{ code int }
	results := make(chan result, n)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/rides/"+orderID.String()+"/accept", nil)
			req.Header.Set("Authorization", "Bearer "+drvTokens[idx])
			rec := httptest.NewRecorder()
			e.r.ServeHTTP(rec, req)
			results <- result{code: rec.Code}
		}(i)
	}
	wg.Wait()
	close(results)

	success, conflict := 0, 0
	for r := range results {
		switch r.code {
		case http.StatusOK:
			success++
		case http.StatusConflict:
			conflict++
		}
	}
	require.Equal(t, 1, success, "tepat satu driver harus menang")
	require.Equal(t, n-1, conflict, "sisanya harus 409")
	require.Equal(t, n, success+conflict, "semua request harus selesai")

	// Hanya satu driver yang ter-assign dan satu event transisi assign.
	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "DRIVER_ASSIGNED", order.Status)
	var events int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM ride_order_events WHERE order_id = $1 AND to_status = 'DRIVER_ASSIGNED'`,
		orderID).Scan(&events))
	require.Equal(t, 1, events)

	var ws string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT working_status FROM users WHERE id = $1`, *order.DriverID).Scan(&ws))
	require.Equal(t, "BUSY", ws, "driver pemenang harus BUSY")
}

// TC-INT-RD-003 — Customer cancel sebelum assign → refund penuh tanpa penalti.
func TestIntegrationRide_CustomerCancelFullRefund(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	escrowBeforeBook := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	orderID, _ := bookRide(t, e, custToken, "WALLET")

	w := updateRideStatus(t, e, custToken, orderID, "CANCELLED", "CUSTOMER_CANCEL")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "CANCELLED", updatedStatus(t, w))

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "CANCELLED", order.Status)
	require.NotNil(t, order.CancellationReason)
	require.Equal(t, "CUSTOMER_CANCEL", *order.CancellationReason)

	// Refund penuh: saldo customer kembali 100.000 & escrow kembali ke
	// nilai sebelum booking (0 setelah reset run).
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000)))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBeforeBook))

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "CUSTOMER_CANCEL")
}

// TC-INT-RD-006 — Driver Emergency Cancel (setelah assign) → refund penuh,
// driver kembali IDLE, metadata audit {"reason":"DRIVER_EMERGENCY"}.
func TestIntegrationRide_DriverEmergencyCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")
	orderID, _ := bookRide(t, e, custToken, "WALLET")

	dToken, dID, _ := newDriver(t, e, decimal.NewFromInt(100000))

	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

	w = updateRideStatus(t, e, dToken, orderID, "CANCELLED", "DRIVER_EMERGENCY")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "CANCELLED", updatedStatus(t, w))

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "CANCELLED", order.Status)
	require.NotNil(t, order.CancellationReason)
	require.Equal(t, "DRIVER_EMERGENCY", *order.CancellationReason)

	// Customer di-refund penuh; driver dilepas → IDLE.
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000)))
	var ws string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT working_status FROM users WHERE id = $1`, dID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "DRIVER_EMERGENCY")
}

// TC-INT-RD-007 — Auto-Cancel Worker: order SEARCHING_DRIVER yang melewati
// expires_at (TTL 15 menit) dibatalkan dengan reason EXPIRED + refund.
func TestIntegrationRide_AutoCancelExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")
	orderID, _ := bookRide(t, e, custToken, "WALLET")

	// Paksa order kedaluwarsa (simulasi TTL 15 menit sudah lewat).
	_, err := e.pool.Exec(ctx,
		`UPDATE ride_orders SET expires_at = NOW() - INTERVAL '15 minutes' WHERE id = $1`, orderID)
	require.NoError(t, err)

	// Auto-cancel worker (seperti scheduler) — dipanggil manual di test.
	svc := ride.NewService(ride.NewRepository(e.pool), wallet.NewLedgerService(e.pool), nil, e.pool)
	cancelled, err := svc.AutoCancelExpiredOrders(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, cancelled, 1)

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "CANCELLED", order.Status)
	require.NotNil(t, order.CancellationReason)
	require.Equal(t, "EXPIRED", *order.CancellationReason)

	// Refund penuh escrow ke customer.
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000)))

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "EXPIRED")
}

// TC-INT-RD-008 — Settlement CASH (COD): komisi platform 20% di-debit dari
// wallet driver. Positif: balance masih ≥ -50.000 → driver tetap ACTIVE.
// Negatif: balance menembus ceiling -50.000 → driver SUSPENDED.
func TestIntegrationRide_CashSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("commission_debited_above_ceiling", func(t *testing.T) {
		ctx := context.Background()
		e := newTestEnv(t)

		custToken, _ := registerAndLogin(t, e.r, "customer")
		dToken, dID, drvWallet := newDriver(t, e, decimal.NewFromInt(100000))

		orderID, fare := bookRide(t, e, custToken, "CASH")
		platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")

		w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
		require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

		for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
			w = updateRideStatus(t, e, dToken, orderID, st, "")
			require.Equal(t, http.StatusOK, w.Code, "status %s: %s", st, w.Body.String())
		}
		w = updateRideStatus(t, e, dToken, orderID, "COMPLETED", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "SETTLED", updatedStatus(t, w))

		commission := fare.Mul(decimal.RequireFromString("0.20")).Round(2)

		// Customer bayar tunai ke driver → tidak ada debit escrow; komisi
		// 20% di-debit dari wallet driver menuju platform.
		wantDriver := decimal.NewFromInt(100000).Sub(commission)
		require.True(t, e.getBalance(t, ctx, drvWallet).Equal(wantDriver),
			"driver wallet=%v want %v", e.getBalance(t, ctx, drvWallet), wantDriver)
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore.Add(commission)))

		// Balance masih ≥ ceiling -50.000 → driver TIDAK di-suspend.
		var status, ws string
		require.NoError(t, e.pool.QueryRow(ctx,
			`SELECT status, working_status FROM users WHERE id = $1`, dID).Scan(&status, &ws))
		require.Equal(t, "ACTIVE", status)
		require.Equal(t, "IDLE", ws)

		order := getOrder(t, e, ctx, orderID)
		require.Equal(t, "SETTLED", order.Status)
		assertLedgerBalanced(t, e, ctx, orderID)
	})

	t.Run("below_negative_ceiling_suspended", func(t *testing.T) {
		ctx := context.Background()
		e := newTestEnv(t)

		// Jarak jauh agar komisi > 10.000 sehingga -40.000 - komisi menembus
		// ceiling -50.000 (LOGIC_FLOW 5.5).
		const (
			farLat = -6.340000
			farLng = 106.816666
		)

		custToken, _ := registerAndLogin(t, e.r, "customer")
		dToken, dID, drvWallet := newDriver(t, e, decimal.NewFromInt(-40000))

		orderID, fare := bookRideCoords(t, e, custToken, "CASH",
			defaultPickupLat, defaultPickupLng, farLat, farLng)
		platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")

		// Saldo -40.000 < min_balance_threshold (0) ditolak endpoint accept;
		// assign driver langsung lewat SQL untuk menguji jalur settlement.
		tag, err := e.pool.Exec(ctx, `
			UPDATE ride_orders SET driver_id = $2, status = 'DRIVER_ASSIGNED', assigned_at = NOW()
			WHERE id = $1 AND status = 'SEARCHING_DRIVER'`, orderID, dID)
		require.NoError(t, err)
		require.Equal(t, int64(1), tag.RowsAffected())

		for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
			w := updateRideStatus(t, e, dToken, orderID, st, "")
			require.Equal(t, http.StatusOK, w.Code, "status %s: %s", st, w.Body.String())
		}
		w := updateRideStatus(t, e, dToken, orderID, "COMPLETED", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "SETTLED", updatedStatus(t, w))

		commission := fare.Mul(decimal.RequireFromString("0.20")).Round(2)
		require.True(t, commission.GreaterThan(decimal.NewFromInt(10000)),
			"komisi harus >10.000 agar menembus ceiling, dapat %v", commission)

		wantDriver := decimal.NewFromInt(-40000).Sub(commission)
		gotDriver := e.getBalance(t, ctx, drvWallet)
		require.True(t, gotDriver.Equal(wantDriver),
			"driver wallet=%v want %v", gotDriver, wantDriver)
		require.True(t, gotDriver.LessThan(decimal.NewFromInt(-50000)),
			"balance %v harus di bawah ceiling -50.000", gotDriver)
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore.Add(commission)))

		// Mehlewati ceiling → driver di-SUSPENDED (working_status IDLE).
		var status, ws string
		require.NoError(t, e.pool.QueryRow(ctx,
			`SELECT status, working_status FROM users WHERE id = $1`, dID).Scan(&status, &ws))
		require.Equal(t, "SUSPENDED", status)
		require.Equal(t, "IDLE", ws)

		assertLedgerBalanced(t, e, ctx, orderID)
	})
}