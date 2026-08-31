//go:build integration

// Package food_test — integration test end-to-end (HTTP + PostgreSQL + Redis)
// untuk G-Food (Task 3.2 s/d 3.4 + auto-cancel Task 3.7): merchant onboarding,
// katalog menu & item, order WALLET & CASH, settlement 4-way, refund cancel,
// emergency cancel driver, dan auto-cancel worker.
//
// Menyalin setup router dari cmd/api/main.go dan memakai PostgreSQL + Redis
// dari docker-compose. File ini TIDAK ikut kompilasi pada `go test ./...`
// biasa (build tag `integration`); jalankan dengan:
//
//	env DATABASE_URL=... go test -tags integration ./internal/food/ -run TC_FOOD -v
//
// Prasyarat: PostgreSQL ter-migrate (docker-compose) + Redis:6380 up.
package food_test

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
	"github.com/g-flow/g-flow/internal/food"
	"github.com/g-flow/g-flow/internal/middleware"
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

// Coordinat default replicating ride integration test (jarak ~0 untuk
// determinisme; fare G-Food tidak bergantung jarak).
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

	// Reset saldo wallet sistem ke 0 agar test idempotent (system wallet
	// dipakai escrow/settlement & hanya bertambah lintas run).
	if _, err := pool.Exec(context.Background(),
		`UPDATE wallets SET balance = 0 WHERE user_id IS NULL`); err != nil {
		t.Fatalf("gagal reset saldo wallet sistem: %v", err)
	}
	return pool
}

// newTestEnv membangun router Gin yang menyalin setup cmd/api/main.go
// (subset route yang dibutuhkan food flow).
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	pool := setupPool(t)
	gin.SetMode(gin.TestMode)

	rdb, err := redis.ParseURL("redis://localhost:6380/0")
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	jwtSvc := auth.NewJWTService("food-integration-test-secret")
	blacklist := auth.NewBlacklistService(rd)
	authHandler := auth.NewHandler(jwtSvc, blacklist, pool)

	walletLedger := wallet.NewLedgerService(pool)
	walletHandler := wallet.NewHandler(
		wallet.NewService(wallet.NewRepository(pool), walletLedger, rd, pool))

	foodHandler := food.NewHandler(food.NewService(
		food.NewRepository(pool), pool, rd, walletLedger))

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	api := r.Group("/api/v1", middleware.AuthMiddleware(jwtSvc, blacklist))
	{
		api.POST("/wallets/:wallet_id/topup", walletHandler.TopUp)
		api.GET("/wallets/:wallet_id/balance", walletHandler.GetBalance)
	}

	catalog := r.Group("/api/v1", middleware.OptionalAuthMiddleware(jwtSvc, blacklist))
	{
		catalog.GET("/merchants", foodHandler.GetMerchants)
		catalog.GET("/merchants/:id/items", foodHandler.GetMerchantItems)
	}

	merchants := r.Group("/api/v1/merchants", middleware.AuthMiddleware(jwtSvc, blacklist), auth.RBACMiddleware("merchant"))
	{
		merchants.POST("/register", foodHandler.RegisterMerchant)
		merchants.GET("/:id", foodHandler.GetMerchant)
		merchants.PATCH("/:id", foodHandler.UpdateMerchant)
		merchants.POST("/:id/menus", foodHandler.CreateMenu)
		merchants.GET("/:id/menus", foodHandler.GetMenus)
		merchants.POST("/:id/items", foodHandler.CreateItem)
	}

	foodOrders := r.Group("/api/v1/food-orders", middleware.AuthMiddleware(jwtSvc, blacklist))
	{
		foodOrders.POST("", auth.RBACMiddleware("customer"), foodHandler.CreateFoodOrder)
		foodOrders.GET("/:id", foodHandler.GetFoodOrder)
		foodOrders.PATCH("/:id", foodHandler.UpdateFoodOrderStatus)
	}

	return &testEnv{pool: pool, r: r, ledger: walletLedger}
}

// doJSON mengirim request HTTP. Goroutine-safe (TIDAK menyentuh *testing.T).
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

// registerAndLogin mendaftar user (customer/driver/merchant) lalu login;
// mengembalikan access token + user id.
func registerAndLogin(t *testing.T, r *gin.Engine, userType string) (string, uuid.UUID) {
	t.Helper()
	email := "fit" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"
	body := gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Food IT " + userType,
		"user_type": userType,
	}
	if userType == "driver" {
		body["vehicle_type"] = "motorcycle"
		body["vehicle_plate"] = "B 9876 XY"
		body["license_number"] = "SIM-FOOD-42"
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

func topupCustomer(t *testing.T, e *testEnv, token string, walletID uuid.UUID, amount string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/wallets/"+walletID.String()+"/topup",
		gin.H{"amount": amount, "idempotency_key": uuid.New().String()}, token, "")
	require.Equal(t, http.StatusOK, w.Code, "topup: %s", w.Body.String())
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

// --- helpers alur bisnis G-Food ---

// registerMerchant mendaftar merchant (status PENDING_VERIFICATION) dan
// mengembalikan merchant id.
func registerMerchant(t *testing.T, e *testEnv, token string) uuid.UUID {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/merchants/register", gin.H{
		"merchant_name": "Warung IT " + uuid.New().String(),
		"description":   "integration test merchant",
		"category":      "makanan",
		"address":       "Jl. Test Merchat",
		"latitude":      testLat,
		"longitude":     testLng,
		"phone":         "081234567890",
	}, token, "")
	require.Equal(t, http.StatusCreated, w.Code, "register merchant: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	require.Equal(t, "PENDING_VERIFICATION", data.Status)
	require.NotEqual(t, uuid.Nil, data.ID)
	return data.ID
}

// activateMerchant menandai merchant ACTIVE (simulasi verifikasi admin pada
// flow onboarding; tidak ada endpoint verifikasi di scope Task 3.2).
func (e *testEnv) activateMerchant(t *testing.T, ctx context.Context, merchantID uuid.UUID) {
	t.Helper()
	_, err := e.pool.Exec(ctx,
		`UPDATE food_merchants SET status = 'ACTIVE', verified_at = NOW() WHERE id = $1`, merchantID)
	require.NoError(t, err)
}

func createMenu(t *testing.T, e *testEnv, token string, merchantID uuid.UUID) uuid.UUID {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/merchants/"+merchantID.String()+"/menus",
		gin.H{"name": "Menu IT " + uuid.New().String()}, token, "")
	require.Equal(t, http.StatusCreated, w.Code, "create menu: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	return data.ID
}

func createItem(t *testing.T, e *testEnv, token string, merchantID, menuID uuid.UUID, price int) uuid.UUID {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/merchants/"+merchantID.String()+"/items", gin.H{
		"menu_id":      menuID.String(),
		"name":         "Item IT " + uuid.New().String(),
		"price":        price,
		"stock":        100,
		"is_available": true,
	}, token, "")
	require.Equal(t, http.StatusCreated, w.Code, "create item: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	return data.ID
}

// createFoodOrder membuat food order (WALLET → CONFIRMED + escrow; CASH →
// CREATED tanpa escrow). Mengembalikan order id + status awal.
func createFoodOrder(t *testing.T, e *testEnv, token string, merchantID, itemID uuid.UUID, paymentMethod string) (uuid.UUID, string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPost, "/api/v1/food-orders", gin.H{
		"merchant_id":      merchantID.String(),
		"delivery_address": "Jl. Test Food",
		"delivery_lat":     testLat,
		"delivery_lng":     testLng,
		"payment_method":   paymentMethod,
		"items": []gin.H{
			{"item_id": itemID.String(), "quantity": 1},
		},
	}, token, uuid.New().String())
	require.Equal(t, http.StatusCreated, w.Code, "create food order: %s", w.Body.String())

	out := mustResp(t, w)
	var data struct {
		ID           uuid.UUID       `json:"id"`
		Status       string          `json:"status"`
		ItemSubtotal decimal.Decimal `json:"item_subtotal"`
		TotalAmount  decimal.Decimal `json:"total_amount"`
	}
	require.NoError(t, json.Unmarshal(out.Data, &data))
	return data.ID, data.Status
}

// updateFoodStatus mengirim PATCH /food-orders/:id. Mengembalikan status dari
// respons data (status akhir setelah auto-settlement bila DELIVERED).
func updateFoodStatus(t *testing.T, e *testEnv, token string, orderID uuid.UUID, status, reason string) (int, string) {
	t.Helper()
	w := doJSON(e.r, http.MethodPatch, "/api/v1/food-orders/"+orderID.String(),
		gin.H{"status": status, "reason": reason}, token, "")
	if w.Code != http.StatusOK {
		t.Logf("PATCH food-orders/%s status=%s -> %d: %s", orderID, status, w.Code, w.Body.String())
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

// assignFoodDriver mensimulasikan hasil driver matching engine (belum di
// implementasi sebagai endpoint; Task 3.3 menunjuk driver lewat proses async):
// menunjuk driver sebagai driver_id/driver_wallet_id food order, dan bila
// busy=true menandai working_status driver BUSY (efek dari matching engine).
func (e *testEnv) assignFoodDriver(t *testing.T, ctx context.Context, orderID, driverID, driverWalletID uuid.UUID, busy bool) {
	t.Helper()
	_, err := e.pool.Exec(ctx,
		`UPDATE food_orders SET driver_id = $1, driver_wallet_id = $2 WHERE id = $3`,
		driverID, driverWalletID, orderID)
	require.NoError(t, err)
	if busy {
		_, err := e.pool.Exec(ctx,
			`UPDATE users SET working_status = 'BUSY' WHERE id = $1`, driverID)
		require.NoError(t, err)
	}
}

// newDriver mendaftarkan driver + wallet DRIVER dengan saldo awal.
func newDriver(t *testing.T, e *testEnv, initialBalance int) (string, uuid.UUID, uuid.UUID) {
	t.Helper()
	token, driverID := registerAndLogin(t, e.r, "driver")
	walletID := e.createWallet(t, context.Background(), driverID, "DRIVER", decimal.NewFromInt(int64(initialBalance)))
	return token, driverID, walletID
}

// getFoodOrderStatus membaca status food_orders dari DB.
func (e *testEnv) getFoodOrderStatus(t *testing.T, ctx context.Context, orderID uuid.UUID) string {
	t.Helper()
	var status string
	err := e.pool.QueryRow(ctx, `SELECT status FROM food_orders WHERE id = $1`, orderID).Scan(&status)
	require.NoError(t, err)
	return status
}

// ============================================================================
// TC-FOOD-001 — E2E WALLET: merchant register → order → merchant confirm →
// driver pickup → delivery → settlement 4-way. Escrow dilepas penuh, merchant
// 85%, driver 90% delivery fee, platform 15% item + 10% delivery fee, ledger
// double-entry seimbang.
// ============================================================================
func TestIntegrationFood_TC_FOOD_001_E2EWalletSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWalletID := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWalletID, "100000")
	const (
		itemPrice       = 25000
		deliveryFee     = 20000
		commissionPct   = "0.15"
		deliveryCommPct = "0.10"
	)
	subtotal := decimal.NewFromInt(itemPrice)
	commission := subtotal.Mul(decimal.RequireFromString(commissionPct))
	merchantEarning := subtotal.Sub(commission)
	driverEarning := decimal.NewFromInt(deliveryFee).Mul(decimal.RequireFromString("0.90"))
	deliveryCommission := decimal.NewFromInt(deliveryFee).Mul(decimal.RequireFromString(deliveryCommPct))
	total := subtotal.Add(decimal.NewFromInt(deliveryFee))

	// Setup merchant + katalog.
	merchToken, merchID := registerAndLogin(t, e.r, "merchant")
	merchantID := registerMerchant(t, e, merchToken)
	e.activateMerchant(t, ctx, merchantID)
	menuID := createMenu(t, e, merchToken, merchantID)
	itemID := createItem(t, e, merchToken, merchantID, menuID, itemPrice)

	// Driver + wallet DRIVER.
	driverToken, driverID, driverWalletID := newDriver(t, e, 0)

	// Order WALLET → CONFIRMED, escrow dipegang.
	orderID, status := createFoodOrder(t, e, custToken, merchantID, itemID, "WALLET")
	require.Equal(t, "CONFIRMED", status)
	escrowBefore := e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW")
	require.True(t, escrowBefore.Equal(total), "escrow sebelum delivery = %v, want %v", escrowBefore, total)
	require.True(t, e.getBalance(t, ctx, custWalletID).Equal(decimal.NewFromInt(100000).Sub(total)))

	// Driver matching (simulasi) → merchant siapkan → driver antar.
	e.assignFoodDriver(t, ctx, orderID, driverID, driverWalletID, true)

	// Merchant: PREPARING → READY_FOR_PICKUP (order WALLET lahir CONFIRMED).
	code, got := updateFoodStatus(t, e, merchToken, orderID, "PREPARING", "")
	require.Equal(t, http.StatusOK, code, "merchant PREPARING: %s")
	require.Equal(t, "PREPARING", got)
	code, got = updateFoodStatus(t, e, merchToken, orderID, "READY_FOR_PICKUP", "")
	require.Equal(t, http.StatusOK, code, "merchant READY_FOR_PICKUP: %s")
	require.Equal(t, "READY_FOR_PICKUP", got)

	// Driver: PICKED_UP → IN_TRANSIT → DELIVERED (auto → SETTLED).
	code, got = updateFoodStatus(t, e, driverToken, orderID, "PICKED_UP", "")
	require.Equal(t, http.StatusOK, code, "driver PICKED_UP: %d")
	require.Equal(t, "PICKED_UP", got)
	code, got = updateFoodStatus(t, e, driverToken, orderID, "IN_TRANSIT", "")
	require.Equal(t, http.StatusOK, code, "driver IN_TRANSIT")
	require.Equal(t, "IN_TRANSIT", got)
	code, got = updateFoodStatus(t, e, driverToken, orderID, "DELIVERED", "")
	require.Equal(t, http.StatusOK, code, "driver DELIVERED")
	require.Equal(t, "SETTLED", got, "order harus langsung SETTLED")

	// Settlement 4-way.
	require.Equal(t, "SETTLED", e.getFoodOrderStatus(t, ctx, orderID))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus 0 setelah settlement")
	require.True(t, e.getBalance(t, ctx, custWalletID).Equal(decimal.NewFromInt(100000).Sub(total)),
		"balance customer harus tetap 100000 - total")
	merchantWalletID := e.getWalletID(t, ctx, merchID, "MERCHANT")
	require.True(t, e.getBalance(t, ctx, merchantWalletID).Equal(merchantEarning.Round(2)),
		"merchant earning = %v, got %v", merchantEarning.Round(2), e.getBalance(t, ctx, merchantWalletID))
	require.True(t, e.getBalance(t, ctx, driverWalletID).Equal(driverEarning.Round(2)),
		"driver earning = %v, got %v", driverEarning.Round(2), e.getBalance(t, ctx, driverWalletID))
	platform := commission.Add(deliveryCommission).Round(2)
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(platform),
		"platform = %v, got %v", platform, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM"))

	// Driver kembali IDLE setelah settlement.
	var ws string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT working_status FROM users WHERE id = $1`, driverID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// ============================================================================
// TC-FOOD-002 — E2E CASH: settlement 3-entry dari wallet driver (merchant
// share + komisi item + komisi delivery). Saldo driver menembus ceiling
// -50.000 → driver di-SUSPENDED.
// ============================================================================
func TestIntegrationFood_TC_FOOD_002_CashDriverSuspended(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	// Item 60.000: merchant share 51.000, komisi 9.000, delivery komisi 2.000.
	// Driver balance 0 - (51.000 + 9.000 + 2.000) = -62.000 < -50.000 → SUSPENDED.
	const itemPrice = 60000
	subtotal := decimal.NewFromInt(itemPrice)
	commission := subtotal.Mul(decimal.RequireFromString("0.15")).Round(2) // 9000
	merchantEarning := subtotal.Sub(commission)                            // 51000
	const deliveryFee = 20000
	deliveryCommission := decimal.NewFromInt(deliveryFee).Mul(decimal.RequireFromString("0.10")) // 2000
	require.True(t, merchantEarning.Equal(decimal.NewFromInt(51000)))
	require.True(t, commission.Equal(decimal.NewFromInt(9000)))

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWalletID := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWalletID, "100000")

	merchToken, merchID := registerAndLogin(t, e.r, "merchant")
	merchantID := registerMerchant(t, e, merchToken)
	e.activateMerchant(t, ctx, merchantID)
	menuID := createMenu(t, e, merchToken, merchantID)
	itemID := createItem(t, e, merchToken, merchantID, menuID, itemPrice)

	driverToken, driverID, driverWalletID := newDriver(t, e, 0)

	// CASH order → CREATED (tanpa escrow).
	orderID, status := createFoodOrder(t, e, custToken, merchantID, itemID, "CASH")
	require.Equal(t, "CREATED", status)
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero())

	e.assignFoodDriver(t, ctx, orderID, driverID, driverWalletID, true)

	code, got := updateFoodStatus(t, e, merchToken, orderID, "CONFIRMED", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "CONFIRMED", got)
	code, got = updateFoodStatus(t, e, merchToken, orderID, "PREPARING", "")
	require.Equal(t, http.StatusOK, code)
	code, got = updateFoodStatus(t, e, merchToken, orderID, "READY_FOR_PICKUP", "")
	require.Equal(t, http.StatusOK, code)
	code, got = updateFoodStatus(t, e, driverToken, orderID, "PICKED_UP", "")
	require.Equal(t, http.StatusOK, code)
	code, got = updateFoodStatus(t, e, driverToken, orderID, "IN_TRANSIT", "")
	require.Equal(t, http.StatusOK, code)
	code, got = updateFoodStatus(t, e, driverToken, orderID, "DELIVERED", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "SETTLED", got)

	// Settlement CASH: driver debit merchant share + kedua komisi.
	expectedDriver := decimal.NewFromInt(0).
		Sub(merchantEarning).Sub(commission).Sub(deliveryCommission).Round(2) // -62000
	require.True(t, e.getBalance(t, ctx, driverWalletID).Equal(expectedDriver),
		"driver balance = %v, want %v", e.getBalance(t, ctx, driverWalletID), expectedDriver)

	merchantWalletID := e.getWalletID(t, ctx, merchID, "MERCHANT")
	require.True(t, e.getBalance(t, ctx, merchantWalletID).Equal(merchantEarning.Round(2)),
		"merchant share = %v, got %v", merchantEarning.Round(2), e.getBalance(t, ctx, merchantWalletID))
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_PLATFORM").Equal(commission.Add(deliveryCommission).Round(2)))

	// Driver SUSPENDED + working_status IDLE.
	var uStatus, ws string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT status, working_status FROM users WHERE id = $1`, driverID).Scan(&uStatus, &ws))
	require.Equal(t, "SUSPENDED", uStatus)
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// ============================================================================
// TC-FOOD-003 — Customer cancel sebelum merchant mengonfirmasi → full refund
// escrow (customer balance pulih, SYSTEM_ESCROW kembali 0, is_refunded TRUE).
// ============================================================================
func TestIntegrationFood_TC_FOOD_003_CustomerCancelFullRefund(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWalletID := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWalletID, "100000")

	merchToken, _ := registerAndLogin(t, e.r, "merchant")
	merchantID := registerMerchant(t, e, merchToken)
	e.activateMerchant(t, ctx, merchantID)
	menuID := createMenu(t, e, merchToken, merchantID)
	itemID := createItem(t, e, merchToken, merchantID, menuID, 25000)

	orderID, _ := createFoodOrder(t, e, custToken, merchantID, itemID, "WALLET")
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsPositive())

	code, got := updateFoodStatus(t, e, custToken, orderID, "CANCELLED", "batal oleh customer")
	require.Equal(t, http.StatusOK, code, "cancel food order: %d")
	require.Equal(t, "CANCELLED", got)

	// Refund penuh.
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus kembali 0 setelah refund, got %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"))
	require.True(t, e.getBalance(t, ctx, custWalletID).Equal(decimal.NewFromInt(100000)),
		"customer balance harus pulih 100000, got %v", e.getBalance(t, ctx, custWalletID))

	var isRefunded bool
	var cancelledStatus string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT is_refunded, status FROM food_orders WHERE id = $1`, orderID).Scan(&isRefunded, &cancelledStatus))
	require.True(t, isRefunded)
	require.Equal(t, "CANCELLED", cancelledStatus)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// ============================================================================
// TC-FOOD-004 — Driver emergency cancel (setelah PICKED_UP): order CANCELLED,
// escrow WALLET di-refund penuh, dan working_status driver kembali IDLE.
// ============================================================================
func TestIntegrationFood_TC_FOOD_004_DriverEmergencyCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWalletID := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWalletID, "100000")

	merchToken, _ := registerAndLogin(t, e.r, "merchant")
	merchantID := registerMerchant(t, e, merchToken)
	e.activateMerchant(t, ctx, merchantID)
	menuID := createMenu(t, e, merchToken, merchantID)
	itemID := createItem(t, e, merchToken, merchantID, menuID, 25000)

	driverToken, driverID, driverWalletID := newDriver(t, e, 0)

	orderID, _ := createFoodOrder(t, e, custToken, merchantID, itemID, "WALLET")

	// Matching engine menunjuk driver + BUSY.
	e.assignFoodDriver(t, ctx, orderID, driverID, driverWalletID, true)
	code, got := updateFoodStatus(t, e, merchToken, orderID, "PREPARING", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "PREPARING", got)
	code, got = updateFoodStatus(t, e, merchToken, orderID, "READY_FOR_PICKUP", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "READY_FOR_PICKUP", got)
	code, got = updateFoodStatus(t, e, driverToken, orderID, "PICKED_UP", "")
	require.Equal(t, http.StatusOK, code)

	// Driver emergency cancel dari PICKED_UP.
	code, got = updateFoodStatus(t, e, driverToken, orderID, "CANCELLED", "driver darurat")
	require.Equal(t, http.StatusOK, code, "driver emergency cancel: %d")
	require.Equal(t, "CANCELLED", got)

	// Refund penuh escrow WALLET.
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus kembali 0, got %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"))
	require.True(t, e.getBalance(t, ctx, custWalletID).Equal(decimal.NewFromInt(100000)),
		"customer balance harus pulih, got %v", e.getBalance(t, ctx, custWalletID))

	// Driver harus kembali IDLE setelah emergency cancel.
	var ws string
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT working_status FROM users WHERE id = $1`, driverID).Scan(&ws))
	require.Equal(t, "IDLE", ws)

	assertLedgerBalanced(t, e, ctx, orderID)
}

// ============================================================================
// TC-FOOD-005 — Auto-cancel worker (Task 3.7): food order berstatus CREATED
// yang menunggu konfirmasi merchant > 15 menit dibatalkan otomatis. Untuk
// WALLET, escrow memegang dana customer → semua dana di-refund penuh.
// ============================================================================
func TestIntegrationFood_TC_FOOD_005_AutoCancelExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	e := newTestEnv(t)

	// Bersihkan order food/send sisa dari run sebelumnya agar deterministik.
	_, err := e.pool.Exec(ctx, `
		TRUNCATE food_order_events, food_order_items, food_orders,
		         send_order_events, send_order_stops, send_orders CASCADE`)
	require.NoError(t, err)

	// Worker auto-cancel tanpa Redis (nil) → lock distributed dianggap dimiliki.
	acw := worker.NewWorker(worker.NewRepository(e.pool), e.pool, nil, e.ledger)

	// (a) CASH order CREATED → auto-cancel, tanpa escrow (tidak ada refund).
	custToken, custID := registerAndLogin(t, e.r, "customer")
	custWalletID := e.getWalletID(t, ctx, custID, "CUSTOMER")
	topupCustomer(t, e, custToken, custWalletID, "100000")

	merchToken, _ := registerAndLogin(t, e.r, "merchant")
	merchantID := registerMerchant(t, e, merchToken)
	e.activateMerchant(t, ctx, merchantID)
	menuID := createMenu(t, e, merchToken, merchantID)
	itemID := createItem(t, e, merchToken, merchantID, menuID, 25000)

	cashOrderID, status := createFoodOrder(t, e, custToken, merchantID, itemID, "CASH")
	require.Equal(t, "CREATED", status)

	// (b) WALLET order → simulasikan escrow-secured-belum-confirm: order WALLET
	// lahir CONFIRMED, namun worker cancel menargetkan status CREATED. Set
	// status menjadi CREATED (escrow tetap dipegang) untuk menguji path refund
	// worker secara nyata (status yang tidak bisa dicapai via API normal).
	walletOrderID, _ := createFoodOrder(t, e, custToken, merchantID, itemID, "WALLET")
	_, err = e.pool.Exec(ctx, `UPDATE food_orders SET status = 'CREATED' WHERE id = $1`, walletOrderID)
	require.NoError(t, err)

	// Backdate kedua order melewati ambang 15 menit.
	_, err = e.pool.Exec(ctx, `UPDATE food_orders SET created_at = NOW() - INTERVAL '16 minutes' WHERE id = ANY($1)`,
		[]uuid.UUID{cashOrderID, walletOrderID})
	require.NoError(t, err)

	cancelled := acw.CancelFoodOrders(ctx)
	require.Equal(t, 2, cancelled, "dua food order harus di-auto-cancel")

	// (a) CASH: CANCELLED + reason EXPIRED, is_refunded FALSE, tanpa escrow.
	require.Equal(t, "CANCELLED", e.getFoodOrderStatus(t, ctx, cashOrderID))
	var cashReason string
	var cashRefunded bool
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT cancellation_reason, is_refunded FROM food_orders WHERE id = $1`, cashOrderID).Scan(&cashReason, &cashRefunded))
	require.Equal(t, "EXPIRED", cashReason)
	require.False(t, cashRefunded)
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero())

	// (b) WALLET: CANCELLED + refund penuh + customer balance pulih.
	require.Equal(t, "CANCELLED", e.getFoodOrderStatus(t, ctx, walletOrderID))
	var wReason string
	var wRefunded bool
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT cancellation_reason, is_refunded FROM food_orders WHERE id = $1`, walletOrderID).Scan(&wReason, &wRefunded))
	require.Equal(t, "EXPIRED", wReason)
	require.True(t, wRefunded)
	require.True(t, e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW").IsZero(),
		"escrow harus 0 setelah refund auto-cancel, got %v", e.systemWalletBalance(t, ctx, "SYSTEM_ESCROW"))
	require.True(t, e.getBalance(t, ctx, custWalletID).Equal(decimal.NewFromInt(100000)),
		"customer balance harus pulih 100000, got %v", e.getBalance(t, ctx, custWalletID))

	assertLedgerBalanced(t, e, ctx, walletOrderID)
}
