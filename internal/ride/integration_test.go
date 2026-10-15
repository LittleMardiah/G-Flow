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

	rdb, err := redis.ParseURL(testRedisURL())
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

// newDriver mendaftarkan driver lalu memakai wallet DRIVER yang sudah dibuat
// otomatis oleh /auth/register (createWalletsForUser di internal/auth).
// Helper ini TIDAK boleh INSERT wallet lagi — wallet_user_type_unique akan
// menolak (SQLSTATE 23505). Saldo awal di-set pada wallet yang sudah ada.
func newDriver(t *testing.T, e *testEnv, initialBalance decimal.Decimal) (token string, driverID uuid.UUID, walletID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	token, driverID = registerAndLogin(t, e.r, "driver")
	walletID = e.getWalletID(t, ctx, driverID, "DRIVER")
	e.setWalletBalance(t, ctx, walletID, initialBalance)
	return token, driverID, walletID
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
		"pickup_lat":      pLat,
		"pickup_lng":      pLng,
		"pickup_address":  "Jl. Test A",
		"dropoff_lat":     dLat,
		"dropoff_lng":     dLng,
		"dropoff_address": "Jl. Test B",
		"payment_method":  paymentMethod,
	}, token, uuid.New().String())
	require.Equal(t, http.StatusCreated, w.Code, "book: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		OrderID       uuid.UUID       `json:"order_id"`
		Status        string          `json:"status"`
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
		gin.H{"status": status, "reason": reason}, token, uuid.NewString())
}

// updateRideStatusWithFare mirror updateRideStatus tapi menambahkan
// actual_fare pada body (TD-069): dipakai driver saat COMPLETED untuk
// mengirim fare aktual.
func updateRideStatusWithFare(t *testing.T, e *testEnv, token string, orderID uuid.UUID, status string, actualFare int64) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(e.r, http.MethodPatch, "/api/v1/rides/"+orderID.String()+"/status",
		gin.H{"status": status, "actual_fare": actualFare}, token, uuid.NewString())
}

// backdateRideArrivedAt memundurkan arrived_at agar lolos rule TD-129
// (>=5 menit setelah DRIVER_ARRIVED) tanpa harus sleep di test.
func backdateRideArrivedAt(t *testing.T, e *testEnv, ctx context.Context, orderID uuid.UUID) {
	t.Helper()
	_, err := e.pool.Exec(ctx,
		`UPDATE ride_orders SET arrived_at = NOW() - INTERVAL '6 minutes' WHERE id = $1`,
		orderID)
	require.NoError(t, err)
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

// TC-INT-RD-002.B — TD-071: Driver yang SAMA accept 2 order BERBEDA secara
// paralel. Fix v2.3 (lock users FOR UPDATE NOWAIT dulu) harus mencegah double
// assign: tepat satu 200, satu 409, driver BUSY, dan hanya satu order ter-assign.
func TestIntegrationRide_SameDriverDoubleAccept(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	order1, _ := bookRide(t, e, custToken, "WALLET")
	order2, _ := bookRide(t, e, custToken, "WALLET")

	dToken, _, _ := newDriver(t, e, decimal.NewFromInt(100000))

	type result struct{ code int }
	results := make(chan result, 2)

	var wg sync.WaitGroup
	for _, oid := range []uuid.UUID{order1, order2} {
		wg.Add(1)
		go func(orderID uuid.UUID) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/rides/"+orderID.String()+"/accept", nil)
			req.Header.Set("Authorization", "Bearer "+dToken)
			rec := httptest.NewRecorder()
			e.r.ServeHTTP(rec, req)
			results <- result{code: rec.Code}
		}(oid)
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
	require.Equal(t, 1, success, "tepat satu order harus di-accept")
	require.Equal(t, 1, conflict, "yang kedua harus 409 (busy/lock)")
	require.Equal(t, 2, success+conflict, "semua request harus selesai")

	// Driver hanya ter-assign ke SATU order; order lain tetap SEARCHING_DRIVER.
	var assigned int
	require.NoError(t, e.pool.QueryRow(ctx, `
		SELECT count(*) FROM ride_orders
		WHERE id IN ($1, $2)
		  AND driver_id IS NOT NULL
		  AND status IN ('DRIVER_ASSIGNED', 'DRIVER_ARRIVED', 'TRIP_STARTED')
	`, order1, order2).Scan(&assigned))
	require.Equal(t, 1, assigned, "driver double-assign TIDAK boleh terjadi")

	for _, oid := range []uuid.UUID{order1, order2} {
		var status string
		require.NoError(t, e.pool.QueryRow(ctx,
			`SELECT status FROM ride_orders WHERE id = $1`, oid).Scan(&status))
		if status == "SEARCHING_DRIVER" {
			assertEmptyDriver(t, e, ctx, oid)
		} else {
			require.Equal(t, "DRIVER_ASSIGNED", status)
		}
	}

	var ws string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT working_status FROM users WHERE id = $1`, getOnlyDriverID(t, e, ctx, order1, order2)).Scan(&ws))
	require.Equal(t, "BUSY", ws, "driver pemenang harus BUSY")

	var events int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM ride_order_events
		 WHERE to_status = 'DRIVER_ASSIGNED' AND order_id IN ($1, $2)`,
		order1, order2).Scan(&events))
	require.Equal(t, 1, events, "tepat satu event assign")
}

// assertEmptyDriver memastikan order tidak memiliki driver.
func assertEmptyDriver(t *testing.T, e *testEnv, ctx context.Context, orderID uuid.UUID) {
	t.Helper()
	var driverID *uuid.UUID
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT driver_id FROM ride_orders WHERE id = $1`, orderID).Scan(&driverID))
	require.Nil(t, driverID)
}

// getOnlyDriverID mengambil driver_id dari order yang ter-assign.
func getOnlyDriverID(t *testing.T, e *testEnv, ctx context.Context, ids ...uuid.UUID) uuid.UUID {
	t.Helper()
	for _, id := range ids {
		var driverID *uuid.UUID
		require.NoError(t, e.pool.QueryRow(ctx,
			`SELECT driver_id FROM ride_orders WHERE id = $1`, id).Scan(&driverID))
		if driverID != nil {
			return *driverID
		}
	}
	t.Fatal("tidak ada order yang ter-assign ke driver")
	return uuid.Nil
}
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

	// Cancel SEBELUM driver ditunjuk → full refund, TANPA fee (LOGIC_FLOW 2.1).
	require.NotNil(t, order.CancellationFee)
	require.True(t, order.CancellationFee.IsZero(), "cancel sebelum assign harus 0 fee, got %v", order.CancellationFee)

	// Refund penuh: saldo customer kembali 100.000 & escrow kembali ke
	// nilai sebelum booking (0 setelah reset run).
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000)))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBeforeBook))

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "CUSTOMER_CANCEL")
}

// TC-INT-RD-004 — Cancel setalah DRIVER_ASSIGNED: fee 5.000 dari customer
// → credit driver; refund (estimated - 5.000) ke customer; escrow kembali 0.
func TestIntegrationRide_CustomerCancelAfterAssigned(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	dToken, _, driverWallet := newDriver(t, e, decimal.NewFromInt(100000))

	escrowBeforeBook := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	orderID, estimated := bookRide(t, e, custToken, "WALLET")

	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

	w = updateRideStatus(t, e, custToken, orderID, "CANCELLED", "CUSTOMER_CANCEL")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "CANCELLED", updatedStatus(t, w))

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "CANCELLED", order.Status)
	require.NotNil(t, order.CancellationReason)
	require.Equal(t, "CUSTOMER_CANCEL", *order.CancellationReason)
	require.NotNil(t, order.CancellationFee)
	require.True(t, order.CancellationFee.Equal(decimal.NewFromInt(5000)), "fee harus 5.000, got %v", order.CancellationFee)

	// Customer: 100.000 - fee 5.000 = 95.000. Driver: 100.000 + 5.000.
	// Escrow kembali ke nilai sebelum booking.
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(95000)),
		"customer balance: %v", e.getBalance(t, ctx, custWallet))
	require.True(t, e.getBalance(t, ctx, driverWallet).Equal(decimal.NewFromInt(105000)),
		"driver balance: %v", e.getBalance(t, ctx, driverWallet))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBeforeBook))
	require.True(t, estimated.Sub(decimal.NewFromInt(5000)).IsPositive(), "estimasi harus > fee")

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "CUSTOMER_CANCEL")
}

// Cancel setelah DRIVER_ARRIVED: fee 10.000 dari customer → credit driver;
// refund (estimated - 10.000) ke customer.
func TestIntegrationRide_CustomerCancelAfterArrived(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	dToken, _, driverWallet := newDriver(t, e, decimal.NewFromInt(100000))

	escrowBeforeBook := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	orderID, estimated := bookRide(t, e, custToken, "WALLET")

	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())
	w = updateRideStatus(t, e, dToken, orderID, "DRIVER_ARRIVED", "")
	require.Equal(t, http.StatusOK, w.Code, "arrived: %s", w.Body.String())
	require.Equal(t, "DRIVER_ARRIVED", updatedStatus(t, w))

	w = updateRideStatus(t, e, custToken, orderID, "CANCELLED", "CUSTOMER_CANCEL")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "CANCELLED", order.Status)
	require.NotNil(t, order.CancellationReason)
	require.Equal(t, "CUSTOMER_CANCEL", *order.CancellationReason)
	require.NotNil(t, order.CancellationFee)
	require.True(t, order.CancellationFee.Equal(decimal.NewFromInt(10000)), "fee harus 10.000, got %v", order.CancellationFee)

	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(90000)),
		"customer balance: %v", e.getBalance(t, ctx, custWallet))
	require.True(t, e.getBalance(t, ctx, driverWallet).Equal(decimal.NewFromInt(110000)),
		"driver balance: %v", e.getBalance(t, ctx, driverWallet))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBeforeBook))
	require.True(t, estimated.Sub(decimal.NewFromInt(10000)).IsPositive(), "estimasi harus > fee")

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "CUSTOMER_CANCEL")
}

// TC-INT-RD-005 — No-Show: driver tiba (DRIVER_ARRIVED), customer tidak
// muncul, driver cancel reason NO_SHOW → fee 10.000 dari customer → driver.
func TestIntegrationRide_NoShowCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	dToken, dID, driverWallet := newDriver(t, e, decimal.NewFromInt(100000))

	escrowBeforeBook := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	orderID, estimated := bookRide(t, e, custToken, "WALLET")

	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())
	w = updateRideStatus(t, e, dToken, orderID, "DRIVER_ARRIVED", "")
	require.Equal(t, http.StatusOK, w.Code, "arrived: %s", w.Body.String())

	backdateRideArrivedAt(t, e, ctx, orderID)

	// Driver membatalkan karena customer no-show (sudah menunggu > 5 menit).
	w = updateRideStatus(t, e, dToken, orderID, "CANCELLED", "NO_SHOW")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "CANCELLED", order.Status)
	require.NotNil(t, order.CancellationReason)
	require.Equal(t, "NO_SHOW", *order.CancellationReason)
	require.NotNil(t, order.CancellationFee)
	require.True(t, order.CancellationFee.Equal(decimal.NewFromInt(10000)), "no-show fee harus 10.000, got %v", order.CancellationFee)

	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(90000)),
		"customer balance: %v", e.getBalance(t, ctx, custWallet))
	require.True(t, e.getBalance(t, ctx, driverWallet).Equal(decimal.NewFromInt(110000)),
		"driver balance: %v", e.getBalance(t, ctx, driverWallet))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBeforeBook))
	require.True(t, estimated.Sub(decimal.NewFromInt(10000)).IsPositive(), "estimasi harus > fee")

	// Driver kembali IDLE setelah cancel.
	var ws string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT working_status FROM users WHERE id = $1`, dID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
	assertCancelEvent(t, e, ctx, orderID, "NO_SHOW")
}

// Anti-bypass (security): customer TIDAK bisa memicu NO_SHOW (fee) — hanya
// driver tertunjuk yang boleh, walau status sudah DRIVER_ARRIVED.
func TestIntegrationRide_CustomerNoShowForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")
	dToken, _, _ := newDriver(t, e, decimal.NewFromInt(100000))

	orderID, _ := bookRide(t, e, custToken, "WALLET")
	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())
	w = updateRideStatus(t, e, dToken, orderID, "DRIVER_ARRIVED", "")
	require.Equal(t, http.StatusOK, w.Code, "arrived: %s", w.Body.String())

	w = updateRideStatus(t, e, custToken, orderID, "CANCELLED", "NO_SHOW")
	require.Equal(t, http.StatusForbidden, w.Code, "customer tidak boleh NO_SHOW: %s", w.Body.String())

	// Order tetap DRIVER_ARRIVED — transaksi gagal dan di-rollback.
	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "DRIVER_ARRIVED", order.Status)
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

// TC-INT-RD-009 — TD-069 SURPLUS: actual_fare < estimated_fare. Customer
// di-refund delta (surplus), escrow dilepas sebesar actual_fare, settlement
// memakai actual (driver 80%, platform 20%). Double-entry seimbang.
func TestIntegrationRide_DeltaFareSurplus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	escrowBefore := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")

	orderID, estimated := bookRide(t, e, custToken, "WALLET")
	const actualFare = 30000
	require.True(t, estimated.GreaterThan(decimal.NewFromInt(actualFare)),
		"estimasi %v harus > actual 30000 agar terjadi surplus", estimated)

	dToken, _, drvWallet := newDriver(t, e, decimal.Zero)

	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

	for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
		w = updateRideStatus(t, e, dToken, orderID, st, "")
		require.Equal(t, http.StatusOK, w.Code, "status %s: %s", st, w.Body.String())
	}

	w = updateRideStatusWithFare(t, e, dToken, orderID, "COMPLETED", actualFare)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "SETTLED", updatedStatus(t, w))

	surplus := estimated.Sub(decimal.NewFromInt(actualFare)).Round(2)
	commission := decimal.NewFromInt(actualFare).Mul(decimal.RequireFromString("0.20")).Round(2)
	earning := decimal.NewFromInt(actualFare).Sub(commission)

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "SETTLED", order.Status)
	require.NotNil(t, order.ActualFare)
	require.True(t, order.ActualFare.Equal(decimal.NewFromInt(actualFare)), "actual=%v", order.ActualFare)
	require.NotNil(t, order.DriverEarning)
	require.True(t, order.DriverEarning.Equal(earning), "earning=%v want %v", order.DriverEarning, earning)
	require.NotNil(t, order.PlatformCommission)
	require.True(t, order.PlatformCommission.Equal(commission), "commission=%v want %v", order.PlatformCommission, commission)

	// Surplus di-refund → customer net hanya membayar actual (100.000 - 30.000)
	// bukan estimated; escrow dilepas sebesar surplus + actual = estimated.
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000-actualFare)),
		"customer balance=%v want %v", e.getBalance(t, ctx, custWallet), decimal.NewFromInt(100000-actualFare))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBefore),
		"escrow=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"), escrowBefore)
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore.Add(commission)),
		"platform=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM"), platformBefore.Add(commission))
	require.True(t, e.getBalance(t, ctx, drvWallet).Equal(earning), "driver=%v want %v", e.getBalance(t, ctx, drvWallet), earning)
	require.True(t, surplus.IsPositive(), "surplus harus positif, got %v", surplus)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// TC-INT-RD-010 — TD-069 SHORTFALL: actual_fare > estimated_fare.
// Subtest A: customer saldo CUKUP → delta di-debit penuh dari customer,
// tanpa subsidi & tanpa overdue_debt.
// Subtest B: customer saldo KURANG → kekurangan ditutup subsidi
// SYSTEM_PLATFORM_SUBSIDY (wallet sistem dedicated, TD-132) + dicatat
// overdue_debt.
func TestIntegrationRide_DeltaFareShortfall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("customer_balance_covers_delta", func(t *testing.T) {
		ctx := context.Background()
		e := newTestEnv(t)

		custToken, custID := registerAndLogin(t, e.r, "customer")
		custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
		topupCustomer(t, e, custToken, custWallet, "500000")

		escrowBefore := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
		platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")

		orderID, estimated := bookRide(t, e, custToken, "WALLET")
		const actualFare = 60000
		require.True(t, estimated.LessThan(decimal.NewFromInt(actualFare)),
			"estimasi %v harus < actual 60000 agar terjadi shortfall", estimated)

		dToken, _, drvWallet := newDriver(t, e, decimal.Zero)

		w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
		require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

		for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
			w = updateRideStatus(t, e, dToken, orderID, st, "")
			require.Equal(t, http.StatusOK, w.Code, "status %s: %s", st, w.Body.String())
		}

		w = updateRideStatusWithFare(t, e, dToken, orderID, "COMPLETED", actualFare)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "SETTLED", updatedStatus(t, w))

		delta := decimal.NewFromInt(actualFare).Sub(estimated).Round(2)
		commission := decimal.NewFromInt(actualFare).Mul(decimal.RequireFromString("0.20")).Round(2)
		earning := decimal.NewFromInt(actualFare).Sub(commission)

		order := getOrder(t, e, ctx, orderID)
		require.Equal(t, "SETTLED", order.Status)
		require.NotNil(t, order.ActualFare)
		require.True(t, order.ActualFare.Equal(decimal.NewFromInt(actualFare)), "actual=%v", order.ActualFare)
		require.True(t, order.DriverEarning.Equal(earning), "earning=%v want %v", order.DriverEarning, earning)
		require.True(t, order.PlatformCommission.Equal(commission), "commission=%v want %v", order.PlatformCommission, commission)

		// Saldo cukup → delta penuh di-debit customer: net 500.000 - 60.000.
		require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(500000-actualFare)),
			"customer balance=%v want %v", e.getBalance(t, ctx, custWallet), decimal.NewFromInt(500000-actualFare))
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBefore),
			"escrow=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"), escrowBefore)
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore.Add(commission)),
			"platform=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM"), platformBefore.Add(commission))
		require.True(t, e.getBalance(t, ctx, drvWallet).Equal(earning), "driver=%v want %v", e.getBalance(t, ctx, drvWallet), earning)
		require.True(t, delta.IsPositive(), "delta harus positif, got %v", delta)

		// Tanpa subsidi → overdue_debt tetap 0.
		var debt decimal.Decimal
		require.NoError(t, e.pool.QueryRow(ctx,
			`SELECT COALESCE(overdue_debt, 0) FROM users WHERE id = $1`, custID).Scan(&debt))
		require.True(t, debt.IsZero(), "overdue_debt harus 0, got %v", debt)

		assertLedgerBalanced(t, e, ctx, orderID)
	})

	t.Run("customer_balance_insufficient_subsidy", func(t *testing.T) {
		ctx := context.Background()
		e := newTestEnv(t)

		custToken, custID := registerAndLogin(t, e.r, "customer")
		custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
		// Top-up minimal: setelah escrow dipegang, saldo tak cukup menutup
		// delta → sisa shortfall ditutup subsidi + dicatat overdue_debt.
		topupCustomer(t, e, custToken, custWallet, "40000")

		escrowBefore := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
		platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")
		subsidyBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM_SUBSIDY")

		orderID, estimated := bookRide(t, e, custToken, "WALLET")
		const actualFare = 60000
		require.True(t, estimated.LessThan(decimal.NewFromInt(actualFare)),
			"estimasi %v harus < actual 60000 agar terjadi shortfall", estimated)

		dToken, _, drvWallet := newDriver(t, e, decimal.Zero)

		w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
		require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())

		for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
			w = updateRideStatus(t, e, dToken, orderID, st, "")
			require.Equal(t, http.StatusOK, w.Code, "status %s: %s", st, w.Body.String())
		}

		balanceAfterEscrow := e.getBalance(t, ctx, custWallet)

		w = updateRideStatusWithFare(t, e, dToken, orderID, "COMPLETED", actualFare)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "SETTLED", updatedStatus(t, w))

		delta := decimal.NewFromInt(actualFare).Sub(estimated).Round(2)
		require.True(t, delta.IsPositive(), "delta harus positif, got %v", delta)
		require.True(t, balanceAfterEscrow.LessThan(delta),
			"saldo customer %v harus < delta %v agar terjadi shortfall", balanceAfterEscrow, delta)
		shortfall := delta.Sub(balanceAfterEscrow).Round(2)

		commission := decimal.NewFromInt(actualFare).Mul(decimal.RequireFromString("0.20")).Round(2)
		earning := decimal.NewFromInt(actualFare).Sub(commission)

		order := getOrder(t, e, ctx, orderID)
		require.Equal(t, "SETTLED", order.Status)
		require.NotNil(t, order.ActualFare)
		require.True(t, order.ActualFare.Equal(decimal.NewFromInt(actualFare)), "actual=%v", order.ActualFare)
		require.True(t, order.DriverEarning.Equal(earning), "earning=%v want %v", order.DriverEarning, earning)
		require.True(t, order.PlatformCommission.Equal(commission), "commission=%v want %v", order.PlatformCommission, commission)

		// Saldo customer habis dipakai menutup delta; kekurangan ditutup subsidi.
		require.True(t, e.getBalance(t, ctx, custWallet).IsZero(),
			"customer balance=%v want 0", e.getBalance(t, ctx, custWallet))
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowBefore),
			"escrow=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"), escrowBefore)
		// TD-132: subsidi shortfall TIDAK lagi tercampur ke SYSTEM_PLATFORM —
		// platform hanya mendapat komisi, shortfall diserap wallet subsidi.
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore.Add(commission)),
			"platform=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM"), platformBefore.Add(commission))
		require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM_SUBSIDY").Equal(subsidyBefore.Sub(shortfall)),
			"subsidy=%v want %v", e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM_SUBSIDY"), subsidyBefore.Sub(shortfall))
		require.True(t, e.getBalance(t, ctx, drvWallet).Equal(earning), "driver=%v want %v", e.getBalance(t, ctx, drvWallet), earning)

		// Kekurangan yang ditutup subsidi dicatat sebagai overdue_debt customer.
		var debt decimal.Decimal
		require.NoError(t, e.pool.QueryRow(ctx,
			`SELECT COALESCE(overdue_debt, 0) FROM users WHERE id = $1`, custID).Scan(&debt))
		require.True(t, debt.Equal(shortfall), "overdue_debt=%v want %v", debt, shortfall)

		assertLedgerBalanced(t, e, ctx, orderID)
	})
}

// --- Voucher discount (TD-070) ---

// createVoucher membuat voucher ACTIVE baru langsung di DB (belum ada
// endpoint admin voucher). Per hit call memakai kode unik.
func (e *testEnv) createVoucher(t *testing.T, ctx context.Context, discountType string, value decimal.Decimal, perUserLimit int) (uuid.UUID, string) {
	t.Helper()
	code := "ITV" + strings.ToUpper(strings.ReplaceAll(uuid.New().String(), "-", ""))[:12]
	var id uuid.UUID
	err := e.pool.QueryRow(ctx, `
		INSERT INTO vouchers (
			code, name, discount_type, discount_value, applicable_services,
			per_user_limit, valid_from, valid_to, status
		) VALUES ($1, 'IT Voucher', $2, $3, ARRAY['RIDE'::order_type_enum], $4,
			NOW() - INTERVAL '1 hour', NOW() + INTERVAL '24 hours', 'ACTIVE')
		RETURNING id`,
		code, discountType, value, perUserLimit).Scan(&id)
	require.NoError(t, err)
	return id, code
}

type voucherBookData struct {
	OrderID        uuid.UUID       `json:"order_id"`
	Status         string          `json:"status"`
	EstimatedFare  decimal.Decimal `json:"estimated_fare"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	VoucherCode    *string         `json:"voucher_code"`
}

// bookRideWithVoucher melakukan booking WALLET memakai voucher_code dan
// mengembalikan data respons booking (termasuk diskon).
func bookRideWithVoucher(t *testing.T, e *testEnv, token, code string) voucherBookData {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/book", gin.H{
		"pickup_lat":      defaultPickupLat,
		"pickup_lng":      defaultPickupLng,
		"pickup_address":  "Jl. Test A",
		"dropoff_lat":     defaultDropoffLat,
		"dropoff_lng":     defaultDropoffLng,
		"dropoff_address": "Jl. Test B",
		"payment_method":  "WALLET",
		"voucher_code":    code,
	}, token, uuid.New().String())
	require.Equal(t, http.StatusCreated, w.Code, "book: %s", w.Body.String())

	var data voucherBookData
	require.NoError(t, json.Unmarshal(mustResp(t, w).Data, &data))
	require.Equal(t, "SEARCHING_DRIVER", data.Status)
	require.NotEqual(t, uuid.Nil, data.OrderID)
	return data
}

// Book + settle end-to-end memakai voucher PERCENTAGE: escrow post-diskon,
// user_vouchers APPLIED, settlement berbasis fareBasis (estimated − diskon).
func TestIntegrationRide_VoucherBookingSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "500000")

	voucherID, code := e.createVoucher(t, ctx, "PERCENTAGE", decimal.NewFromInt(20), 5)
	_ = e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")

	data := bookRideWithVoucher(t, e, custToken, code)
	discount := data.EstimatedFare.Mul(decimal.RequireFromString("0.20")).Round(2)
	require.True(t, data.DiscountAmount.Equal(discount), "discount=%v want %v", data.DiscountAmount, discount)
	require.NotNil(t, data.VoucherCode)
	require.Equal(t, code, *data.VoucherCode)

	// Escrow yang ditahan = estimated − diskon (bukan estimated penuh).
	finalFare := data.EstimatedFare.Sub(discount)
	escrowHeld := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	require.True(t, escrowHeld.Equal(finalFare), "escrow held=%v want %v", escrowHeld, finalFare)
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(500000).Sub(finalFare)),
		"balance=%v want %v", e.getBalance(t, ctx, custWallet), decimal.NewFromInt(500000).Sub(finalFare))

	// Trigger: used_count bertambah & user_vouchers tercatat APPLIED.
	var usedCount int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT used_count FROM vouchers WHERE id = $1`, voucherID).Scan(&usedCount))
	require.Equal(t, 1, usedCount)
	var uvStatus string
	var uvDiscount decimal.Decimal
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT status, discount_amount_applied FROM user_vouchers WHERE voucher_id = $1`, voucherID).
		Scan(&uvStatus, &uvDiscount))
	require.Equal(t, "APPLIED", uvStatus)
	require.True(t, uvDiscount.Equal(discount), "discount_applied=%v want %v", uvDiscount, discount)

	// Order menyimpan voucher_id + discount_amount.
	order := getOrder(t, e, ctx, data.OrderID)
	require.NotNil(t, order.VoucherID)
	require.Equal(t, voucherID, *order.VoucherID)
	require.NotNil(t, order.DiscountAmount)
	require.True(t, order.DiscountAmount.Equal(discount), "order discount=%v want %v", *order.DiscountAmount, discount)

	// Settlement memakai fareBasis (post-diskon).
	dToken, _, drvWallet := newDriver(t, e, decimal.Zero)
	require.Equal(t, http.StatusOK,
		doJSON(e.r, http.MethodPost, "/api/v1/rides/"+data.OrderID.String()+"/accept", nil, dToken, "").Code,
		"accept harus 200")
	for _, st := range []string{"DRIVER_ARRIVED", "TRIP_STARTED"} {
		require.Equal(t, http.StatusOK, updateRideStatus(t, e, dToken, data.OrderID, st, "").Code, "status %s", st)
	}
	w := updateRideStatus(t, e, dToken, data.OrderID, "COMPLETED", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "SETTLED", updatedStatus(t, w))

	commission := finalFare.Mul(decimal.RequireFromString("0.20")).Round(2)
	earning := finalFare.Sub(commission)
	order = getOrder(t, e, ctx, data.OrderID)
	require.Equal(t, "SETTLED", order.Status)
	require.NotNil(t, order.DriverEarning)
	require.True(t, order.DriverEarning.Equal(earning), "earning=%v want %v", order.DriverEarning, earning)
	require.NotNil(t, order.PlatformCommission)
	require.True(t, order.PlatformCommission.Equal(commission), "commission=%v want %v", order.PlatformCommission, commission)
	require.True(t, e.getBalance(t, ctx, drvWallet).Equal(earning), "driver=%v want %v", e.getBalance(t, ctx, drvWallet), earning)
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(500000).Sub(finalFare)))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").Equal(escrowHeld.Sub(finalFare)))

	// Voucher tetap APPLIED setelah settlement (bukan cancel).
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT status FROM user_vouchers WHERE voucher_id = $1`, voucherID).Scan(&uvStatus))
	require.Equal(t, "APPLIED", uvStatus)

	assertLedgerBalanced(t, e, ctx, data.OrderID)
}

// Cancel order ber-voucher: trigger rollback_voucher_soft_delete menandai
// user_vouchers CANCELLED + used_count kembali; refund escrow post-diskon.
func TestIntegrationRide_VoucherCancelRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	voucherID, code := e.createVoucher(t, ctx, "PERCENTAGE", decimal.NewFromInt(20), 5)
	data := bookRideWithVoucher(t, e, custToken, code)
	finalFare := data.EstimatedFare.Sub(data.DiscountAmount)

	// Escrow post-diskon: saldo customer berkurang finalFare (bukan estimated).
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000).Sub(finalFare)),
		"balance=%v want %v", e.getBalance(t, ctx, custWallet), decimal.NewFromInt(100000).Sub(finalFare))

	var uvStatus string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT status FROM user_vouchers WHERE voucher_id = $1`, voucherID).Scan(&uvStatus))
	require.Equal(t, "APPLIED", uvStatus)

	w := updateRideStatus(t, e, custToken, data.OrderID, "CANCELLED", "CUSTOMER_CANCEL")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "CANCELLED", updatedStatus(t, w))

	// Trigram rollback: user_vouchers → CANCELLED, used_count → 0.
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT status FROM user_vouchers WHERE voucher_id = $1`, voucherID).Scan(&uvStatus))
	require.Equal(t, "CANCELLED", uvStatus, "voucher harus di-rollback ke CANCELLED saat order cancel")
	var usedCount int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT used_count FROM vouchers WHERE id = $1`, voucherID).Scan(&usedCount))
	require.Equal(t, 0, usedCount, "used_count harus kembali 0 setelah rollback")

	// Full refund escrow post-diskon → saldo customer kembali penuh.
	require.True(t, e.getBalance(t, ctx, custWallet).Equal(decimal.NewFromInt(100000)),
		"balance=%v want 100000", e.getBalance(t, ctx, custWallet))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero())
	require.Equal(t, "CANCELLED", getOrder(t, e, ctx, data.OrderID).Status)

	assertLedgerBalanced(t, e, ctx, data.OrderID)
}

// --- TD-176: idempotency UpdateRideStatus (PostgreSQL L2, DB nyata) ---
//
// Helper + integration test untuk membuktikan binding pgx v5 terhadap
// idempotency_cache di PostgreSQL nyata (SELECT state/response_body/debounce_at
// + INSERT PROCESSING + UPDATE COMPLETED), preseden TD-181 (pgxmock bisa
// lolos, real DB fail). Semua helper di bawah LOKAL di file ini.

// updateStatusKey PATCH /rides/:order_id/status dengan X-Idempotency-Key
// eksplisit. updateRideStatus (helper existing) selalu mengirim key kosong;
// sejak TD-176 handler mewajibkan header, transisi status wajib pakai helper
// ini.
func updateStatusKey(t *testing.T, e *testEnv, token string, orderID uuid.UUID, status, key string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(e.r, http.MethodPatch, "/api/v1/rides/"+orderID.String()+"/status",
		gin.H{"status": status}, token, key)
}

// seedAssignedRide menyiapkan order yang sudah di-assign untuk test transisi
// status: register customer + topup + book + register driver + accept.
// Mengembalikan token customer, token + ID driver (aktor transisi status),
// orderID, dan wallet driver.
func seedAssignedRide(t *testing.T, e *testEnv) (custToken, dToken string, driverID, orderID, drvWallet uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWallet := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWallet, "100000")

	orderID, _ = bookRide(t, e, custToken, "WALLET")

	dToken, driverID, drvWallet = newDriver(t, e, decimal.NewFromInt(100000))
	w := doJSON(e.r, http.MethodPost, "/api/v1/rides/"+orderID.String()+"/accept", nil, dToken, "")
	require.Equal(t, http.StatusOK, w.Code, "accept: %s", w.Body.String())
	return custToken, dToken, driverID, orderID, drvWallet
}

// resetRideIdempotency mem-flush side effect idempotency milik order/aktor
// test supaya run berulang idempoten: semua event ride order + semua baris
// idempotency_cache milik aktor (driver pelaku PATCH status).
func resetRideIdempotency(t *testing.T, e *testEnv, ctx context.Context, orderID uuid.UUID, actorID uuid.UUID) {
	t.Helper()
	_, err := e.pool.Exec(ctx,
		`DELETE FROM ride_order_events WHERE order_id = $1`, orderID)
	require.NoError(t, err)
	_, err = e.pool.Exec(ctx,
		`DELETE FROM idempotency_cache WHERE user_id = $1`, actorID)
	require.NoError(t, err)
}

// delRedisIdemKey menghapus key L1 Redis milik satu user. Dipakai untuk
// memaksa replay key yang sama jatuh ke L2 PostgreSQL sehingga test benar-benar
// membuktikan binding pgx v5 SELECT state/response_body/debounce_at (bukan
// hanya hit Redis L1). Prefix Redis = "idempotency:" (service.go, scope per
// user).
func delRedisIdemKey(t *testing.T, userID uuid.UUID, key string) {
	t.Helper()
	rdb, err := redis.ParseURL(testRedisURL())
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	defer func() { _ = rd.Close() }()
	require.NoError(t, rd.Del(context.Background(),
		"idempotency:"+userID.String()+":"+key).Err())
}

// Test 1 (TD-176): key yang SAMA dipakai dua kali → respons identik, tepat
// satu event DRIVER_ARRIVED, dan hanya satu baris idempotency_cache. Redis L1
// di-flush di antara dua PATCH supaya replay kedua memotong binding pgx v5
// SELECT ... COMPLETED di PostgreSQL nyata (bukan hit cache Redis).
func TestIntegrationRide_UpdateStatus_IdempotentSameKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	_, dToken, driverID, orderID, _ := seedAssignedRide(t, e)
	resetRideIdempotency(t, e, ctx, orderID, driverID)

	key := "K1-" + uuid.NewString()

	w1 := updateStatusKey(t, e, dToken, orderID, "DRIVER_ARRIVED", key)
	require.Equal(t, http.StatusOK, w1.Code, "PATCH#1: %s", w1.Body.String())
	require.Equal(t, "DRIVER_ARRIVED", updatedStatus(t, w1))

	// Bypass L1 Redis agar replay kedua memotong L2 PostgreSQL.
	delRedisIdemKey(t, driverID, key)

	w2 := updateStatusKey(t, e, dToken, orderID, "DRIVER_ARRIVED", key)
	require.Equal(t, http.StatusOK, w2.Code, "PATCH#2: %s", w2.Body.String())
	require.Equal(t, "DRIVER_ARRIVED", updatedStatus(t, w2))
	// Respons dianggap identik pada field semantik (order_id + status) yang
	// disimpan di cache; status_updated_at dari handler selalu NOW().
	require.Equal(t, updatedStatus(t, w1), updatedStatus(t, w2),
		"replay key sama harus mengembalikan respons yang identik")

	var events int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM ride_order_events
		 WHERE order_id = $1 AND to_status = 'DRIVER_ARRIVED'`, orderID).Scan(&events))
	require.Equal(t, 1, events, "replay tidak boleh menambah event DRIVER_ARRIVED")

	var cacheRows int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM idempotency_cache
		 WHERE key = $1 AND user_id = $2`, key, driverID).Scan(&cacheRows))
	require.Equal(t, 1, cacheRows, "tepat satu baris idempotency_cache untuk K1")
}

// Test 2 (TD-176): replay dengan key BARU (bukan cache) saat status target
// sudah tercapai → 200 + respons status DRIVER_ARRIVED via guard #4a di
// pre-read updateRideStatusTx; TIDAK ada event DRIVER_ARRIVED baru.
func TestIntegrationRide_UpdateStatus_ReplaySameStatus_4a(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	_, dToken, driverID, orderID, _ := seedAssignedRide(t, e)
	resetRideIdempotency(t, e, ctx, orderID, driverID)

	key1 := "K1-" + uuid.NewString()
	w := updateStatusKey(t, e, dToken, orderID, "DRIVER_ARRIVED", key1)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "DRIVER_ARRIVED", updatedStatus(t, w))

	// Key BARU → tidak ada cache COMPLETED (L1 maupun L2) sehingga eksekusi
	// masuk ke updateRideStatusTx; pre-read melihat order.Status ==
	// req.Status == DRIVER_ARRIVED → return via #4a tanpa transisi/event baru.
	key2 := "K2-" + uuid.NewString()
	w = updateStatusKey(t, e, dToken, orderID, "DRIVER_ARRIVED", key2)
	require.Equal(t, http.StatusOK, w.Code, "replay key baru: %s", w.Body.String())
	require.Equal(t, "DRIVER_ARRIVED", updatedStatus(t, w))

	var events int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM ride_order_events
		 WHERE order_id = $1 AND to_status = 'DRIVER_ARRIVED'`, orderID).Scan(&events))
	require.Equal(t, 1, events, "PATCH key baru boleh 200 tapi tidak menambah event")
}

// Test 3 (TD-176): replay COMPLETED memakai key BARU setelah order
// SETTLED (settlement COMPLETED→SETTLED sudah commit) → 200 + respons status
// COMPLETED via guard #4b di pre-read updateRideStatusTx. Settlement TIDAK
// dieksekusi dua kali: ledger_entries tidak bertambah, saldo driver & platform
// tidak berubah.
func TestIntegrationRide_UpdateStatus_ReplayAfterCommit_Completed_4b(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	_, dToken, driverID, orderID, drvWallet := seedAssignedRide(t, e)
	resetRideIdempotency(t, e, ctx, orderID, driverID)

	// Tiap transisi memakai key berbeda (satu operasi = satu key; key yang
	// sama untuk dua transisi akan mengembalikan cache transisi pertama).
	// COMPLETED (transisi yang memicu settlement) memakai K1.
	keyArrive := "K-arrive-" + uuid.NewString()
	keyStart := "K-start-" + uuid.NewString()
	keyComplete := "K1-" + uuid.NewString()

	for _, step := range []struct {
		status string
		key    string
	}{
		{"DRIVER_ARRIVED", keyArrive},
		{"TRIP_STARTED", keyStart},
	} {
		w := updateStatusKey(t, e, dToken, orderID, step.status, step.key)
		require.Equal(t, http.StatusOK, w.Code, "status %s: %s", step.status, w.Body.String())
	}

	w := updateStatusKey(t, e, dToken, orderID, "COMPLETED", keyComplete)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "SETTLED", updatedStatus(t, w))

	order := getOrder(t, e, ctx, orderID)
	require.Equal(t, "SETTLED", order.Status, "COMPLETED harus memicu settlement")

	var ledgerBefore int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM ledger_entries WHERE reference_id = $1`, orderID).Scan(&ledgerBefore))
	driverBalBefore := e.getBalance(t, ctx, drvWallet)
	platformBefore := e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM")

	// Replay COMPLETED dengan key BARU (K3) → pre-read #4b: order sudah
	// SETTLED + target COMPLETED → respons COMPLETED tanpa settlement baru.
	keyReplay := "K3-" + uuid.NewString()
	w = updateStatusKey(t, e, dToken, orderID, "COMPLETED", keyReplay)
	require.Equal(t, http.StatusOK, w.Code, "replay COMPLETED: %s", w.Body.String())
	require.Equal(t, "COMPLETED", updatedStatus(t, w))

	var ledgerAfter int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM ledger_entries WHERE reference_id = $1`, orderID).Scan(&ledgerAfter))
	require.Equal(t, ledgerBefore, ledgerAfter,
		"replay COMPLETED tidak boleh menambah ledger_entries (%d → %d)", ledgerBefore, ledgerAfter)
	require.True(t, e.getBalance(t, ctx, drvWallet).Equal(driverBalBefore),
		"driver wallet berubah: %v → %v", driverBalBefore, e.getBalance(t, ctx, drvWallet))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platformBefore),
		"platform wallet berubah: %v → %v", platformBefore, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM"))
}

// testRedisURL mengembalikan REDIS_URL dari env (untuk CI), fallback ke
// port dev lokal 6380. TD-167: env-driven supaya CI (Redis:6379) dan
// dev lokal (Redis:6380) sama-sama jalan.
func testRedisURL() string {
	if url := os.Getenv("REDIS_URL"); url != "" {
		return url
	}
	return "redis://localhost:6380/0"
}
