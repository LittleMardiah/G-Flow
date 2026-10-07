//go:build integration

// Package wallet_test — integration test GET /api/v1/wallets/me (TD-121).
//
// Endpoint ini di-cover unit test (internal/wallet/handler_test.go, 4 case)
// tapi belum punya integration test terhadap DB nyata. Test di sini
// menutup jalur HTTP lengkap: AuthMiddleware -> RBACMiddleware ->
// handler.GetMyWallet -> service.GetMyWallet -> Repository.GetByUserIDAndType
// (query ke tabel `wallets` nyata).
//
// Wiring router disalin dari cmd/api/main.go:199 (RBAC customer/driver/
// merchant). Jalankan dengan:
//
//	go test -tags integration ./internal/wallet/ -run TestIntegrationWallet -v
//
// Prasyarat: docker-compose (PostgreSQL:15432, Redis:6380) up + migrations.
package wallet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/auth"
	"github.com/g-flow/g-flow/internal/middleware"
	"github.com/g-flow/g-flow/internal/wallet"
)

// setupMeRouter membangun router yang hanya menyalin wiring endpoint
// GET /wallets/me dari cmd/api/main.go (auth publik + api v1 + RBAC).
func setupMeRouter(t *testing.T) *gin.Engine {
	t.Helper()
	pool := setupPool(t)

	rdb, err := redis.ParseURL(testRedisURL())
	require.NoError(t, err)
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	jwt := auth.NewJWTService("integration-test-secret")
	blacklist := auth.NewBlacklistService(rd)

	authHandler := auth.NewHandler(jwt, blacklist, pool)
	walletHandler := wallet.NewHandler(
		wallet.NewService(wallet.NewRepository(pool), wallet.NewLedgerService(pool), nil, pool),
	)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	api := r.Group("/api/v1", middleware.AuthMiddleware(jwt, blacklist))
	// TD-120: static route /wallets/me didaftarkan sebelum /wallets/:wallet_id.
	api.GET("/wallets/me", auth.RBACMiddleware("customer", "driver", "merchant"), walletHandler.GetMyWallet)

	return r
}

// meResp adalah bentuk response sukses GET /wallets/me sesuai DESIGN TD-120.
type meResp struct {
	Success bool `json:"success"`
	Data    struct {
		WalletID   uuid.UUID       `json:"wallet_id"`
		Balance    decimal.Decimal `json:"balance"`
		Status     string          `json:"status"`
		WalletType string          `json:"wallet_type"`
	} `json:"data"`
}

// meErrorResp adalah bentuk response error (API_CONTRACT 3.2).
type meErrorResp struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// meRegisterAndLogin mendaftar user dengan user_type tertentu lalu login,
// mengembalikan token akses + user id. Driver wajib mengisi plate + SIM
// (auth.NewHandler.Register menolak tanpa itu).
func meRegisterAndLogin(t *testing.T, r *gin.Engine, userType string) (token string, userID uuid.UUID) {
	t.Helper()
	email := "me" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"

	body := gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Me Test " + userType,
		"user_type": userType,
	}
	if userType == "driver" {
		body["vehicle_type"] = "motorcycle"
		body["vehicle_plate"] = "B 9999 ZZ"
		body["license_number"] = "SIM-" + uuid.New().String()[:8]
	}

	w := wDoJSON(r, http.MethodPost, "/api/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code, "register: %s", w.Body.String())

	w = wDoJSON(r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login: %s", w.Body.String())

	var ar authResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ar))
	require.NotEmpty(t, ar.Data.AccessToken)
	return ar.Data.AccessToken, ar.Data.UserID
}

// walletIDOf membaca wallet bertipe tertentu milik user langsung dari DB —
// dipakai sebagai ground truth yang independen dari response endpoint.
func walletIDOf(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, walletType string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM wallets WHERE user_id = $1 AND wallet_type = $2`, userID, walletType,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestIntegrationWalletMe_Customer: register customer (auth auto-create wallet
// CUSTOMER) lalu GET /wallets/me?type=CUSTOMER -> 200 dengan keempat field
// DESIGN (wallet_id, balance, status, wallet_type) dan wallet_id cocok dengan
// baris DB.
func TestIntegrationWalletMe_Customer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := setupPool(t)
	r := setupMeRouter(t)

	token, userID := meRegisterAndLogin(t, r, "customer")

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=CUSTOMER", nil, token)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var body meResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, wallet.WalletTypeCustomer, body.Data.WalletType)
	require.Equal(t, wallet.WalletStatusActive, body.Data.Status)
	require.True(t, body.Data.Balance.IsZero(), "balance awal = %v, want 0", body.Data.Balance)
	require.Equal(t, walletIDOf(ctx, t, pool, userID, wallet.WalletTypeCustomer), body.Data.WalletID)
}

// TestIntegrationWalletMe_DefaultType: tanpa query ?type= -> handler
// default ke CUSTOMER (handler.GetMyWallet). Hasil harus identik dengan
// ?type=CUSTOMER eksplisit.
func TestIntegrationWalletMe_DefaultType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := setupPool(t)
	r := setupMeRouter(t)

	token, userID := meRegisterAndLogin(t, r, "customer")

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me", nil, token)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var body meResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, wallet.WalletTypeCustomer, body.Data.WalletType)
	require.Equal(t, walletIDOf(ctx, t, pool, userID, wallet.WalletTypeCustomer), body.Data.WalletID)
}

// TestIntegrationWalletMe_Driver: register driver (auth auto-create wallet
// CUSTOMER + DRIVER, auth/handler.go walletsForUserType) lalu
// GET /wallets/me?type=DRIVER -> 200 dengan wallet_type DRIVER.
func TestIntegrationWalletMe_Driver(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := setupPool(t)
	r := setupMeRouter(t)

	token, userID := meRegisterAndLogin(t, r, "driver")

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=DRIVER", nil, token)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var body meResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, wallet.WalletTypeDriver, body.Data.WalletType)
	require.Equal(t, wallet.WalletStatusActive, body.Data.Status)
	require.Equal(t, walletIDOf(ctx, t, pool, userID, wallet.WalletTypeDriver), body.Data.WalletID)
}

// TestIntegrationWalletMe_Merchant: register merchant (auto-create wallet
// CUSTOMER) lalu GET /wallets/me?type=MERCHANT -> 404 WALLET_NOT_FOUND
// karena register merchant hanya provisioned 1 wallet (CUSTOMER).
// Mengunci perilaku aktual sistem, bukan kebetulan.
func TestIntegrationWalletMe_Merchant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	r := setupMeRouter(t)

	token, _ := meRegisterAndLogin(t, r, "merchant")

	// Wallet CUSTOMER milik merchant -> 200.
	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=CUSTOMER", nil, token)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Wallet MERCHANT tidak di-provision saat register -> 404.
	w = wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=MERCHANT", nil, token)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	var e meErrorResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	require.Equal(t, "WALLET_NOT_FOUND", e.Error.Code)
}

// TestIntegrationWalletMe_NoWalletOfType: customer (hanya punya wallet
// CUSTOMER) meminta ?type=DRIVER -> 404 WALLET_NOT_FOUND. RBAC middleware
// mengizinkan customer, jadi 404 datang dari repository, bukan middleware.
func TestIntegrationWalletMe_NoWalletOfType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	r := setupMeRouter(t)

	token, _ := meRegisterAndLogin(t, r, "customer")

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=DRIVER", nil, token)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	var e meErrorResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	require.Equal(t, "WALLET_NOT_FOUND", e.Error.Code)
}

// TestIntegrationWalletMe_InvalidType: ?type=INVALID bukan label member
// wallet_type_enum PostgreSQL. SEBELUM TD-164 nilai ini diteruskan mentah ke
// query, cast enum gagal (SQLSTATE 22P02) dan handler membalas
// 500 INTERNAL_SERVER_ERROR dengan pesan PostgreSQL mentah ("invalid input
// value for enum wallet_type_enum") — membocorkan detail skema DB ke client.
// Sesudah TD-164 handler memvalidasi `type` terhadap whitelist
// (handler.go validWalletTypes) SEBELUM memanggil service, jadi sekarang
// 400 INVALID_REQUEST dan query tidak pernah dieksekusi.
//
// Test ini sengaja ditulis ulang (bukan dihapus) supaya gap TD-164 tidak bisa
// muncul lagi diam-diam: kalau whitelist dihapus tanpa mapping 22P02, test ini
// kembali MERAH dengan 500.
func TestIntegrationWalletMe_InvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	r := setupMeRouter(t)

	token, _ := meRegisterAndLogin(t, r, "customer")

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=INVALID", nil, token)
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())

	var e meErrorResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	require.Equal(t, "INVALID_REQUEST", e.Error.Code)
	// Nama enum/kolom PostgreSQL tidak boleh bocor ke client.
	require.NotContains(t, w.Body.String(), "wallet_type_enum")
	require.NotContains(t, w.Body.String(), "22P02")
}

// TestIntegrationWalletMe_TypeCaseInsensitive: ?type=customer (lowercase) ->
// handler normalisasi (ToUpper) lalu lolos whitelist -> 200 CUSTOMER. Mobile
// tidak perlu tahu label enum PostgreSQL wajib uppercase.
func TestIntegrationWalletMe_TypeCaseInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := setupPool(t)
	r := setupMeRouter(t)

	token, userID := meRegisterAndLogin(t, r, "customer")

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=customer", nil, token)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var body meResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, wallet.WalletTypeCustomer, body.Data.WalletType)
	require.Equal(t, walletIDOf(ctx, t, pool, userID, wallet.WalletTypeCustomer), body.Data.WalletID)
}

// TestIntegrationWalletMe_Unauthorized: tanpa header Authorization ->
// AuthMiddleware menolak sebelum handler, 401 UNAUTHORIZED.
func TestIntegrationWalletMe_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	r := setupMeRouter(t)

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=CUSTOMER", nil, "")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body: %s", w.Body.String())
}

// TestIntegrationWalletMe_InvalidToken: header Authorization dengan token
// sampah -> 401 UNAUTHORIZED dari AuthMiddleware (bukan handler).
func TestIntegrationWalletMe_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	r := setupMeRouter(t)

	w := wDoJSON(r, http.MethodGet, "/api/v1/wallets/me?type=CUSTOMER", nil, "not-a-real-token")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body: %s", w.Body.String())
}
