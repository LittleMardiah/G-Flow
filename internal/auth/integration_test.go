//go:build integration

// Package auth_test berisi integration test berbasis HTTP (httptest + Gin)
// terhadap database nyata PostgreSQL. File ini TIDAK ikut kompilasi pada
// `go test ./...` biasa; jalankan dengan:
//
//	go test -tags integration ./internal/auth/ -run Integration -v
//
// Prasyarat: docker-compose (PostgreSQL:15432, Redis:6380) sudah up dan
// migrations sudah dijalankan.
package auth_test

import (
	"bytes"
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

	"github.com/g-flow/g-flow/internal/auth"
	"github.com/g-flow/g-flow/internal/db"
	"github.com/g-flow/g-flow/internal/middleware"
	"github.com/g-flow/g-flow/internal/wallet"
)

// testConfig menyimpan dependensi yang dipakai untuk membangun router.
type testConfig struct {
	pool      *pgxpool.Pool
	jwt       *auth.JWTService
	blacklist *auth.BlacklistService
}

func testDBURL(t *testing.T) string {
	t.Helper()
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://postgres:password@localhost:15432/g_flow_dev"
}

func setupTestConfig(t *testing.T) *testConfig {
	t.Helper()
	pool, err := db.NewDB(db.Config{DatabaseURL: testDBURL(t)})
	if err != nil {
		t.Fatalf("gagal konek database: %v", err)
	}
	t.Cleanup(func() { db.Close(pool) })

	rdb, err := redis.ParseURL("redis://localhost:6380/0")
	if err != nil {
		t.Fatalf("redis url parse: %v", err)
	}
	rd := redis.NewClient(rdb)
	t.Cleanup(func() { _ = rd.Close() })

	return &testConfig{
		pool:      pool,
		jwt:       auth.NewJWTService("integration-test-secret"),
		blacklist: auth.NewBlacklistService(rd),
	}
}

// newRouter membangun router Gin yang menyalin setup dari cmd/api/main.go
// (endpoint auth publik + API v1 yang dilindungi auth JWT).
func (tc *testConfig) newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	authHandler := auth.NewHandler(tc.jwt, tc.blacklist, tc.pool)

	repo := wallet.NewRepository(tc.pool)
	ledger := wallet.NewLedgerService(tc.pool)
	svc := wallet.NewService(repo, ledger, nil, tc.pool)
	walletHandler := wallet.NewHandler(svc)

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	api := r.Group("/api/v1", middleware.AuthMiddleware(tc.jwt, tc.blacklist))
	{
		api.POST("/auth/logout", authHandler.Logout)
		api.POST("/wallets/:wallet_id/topup", walletHandler.TopUp)
		api.POST("/wallets/:wallet_id/transfer", walletHandler.Transfer)
		api.GET("/wallets/:wallet_id/balance", walletHandler.GetBalance)

		// TD-120: GET /wallets/me — auto-resolve wallet milik user (mirror
		// cmd/api/main.go wiring; dipakai test wallet provisioning TD-128).
		api.GET("/wallets/me", auth.RBACMiddleware("customer", "driver", "merchant"), walletHandler.GetMyWallet)

		// Endpoint khusus test untuk menutup branch coverage auth/middleware.
		api.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
	}

	// Route dengan RBAC (hanya admin) untuk menutup RBACMiddleware.
	admin := api.Group("", auth.RBACMiddleware("admin"))
	admin.GET("/admin/panel", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": "admin"})
	})

	return r
}

type apiResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		UserID       uuid.UUID `json:"user_id"`
		UserType     string    `json:"user_type"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any, token string) (*httptest.ResponseRecorder, apiResponse) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var out apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

// randomEmail membuat email unik untuk test.
func randomEmail() string {
	return "it" + strings.ReplaceAll(uuid.New().String(), "-", "") + "@test.com"
}

// registerLogin mendaftarkan user baru lalu login, mengembalikan token akses.
func registerLogin(t *testing.T, r *gin.Engine) string {
	t.Helper()
	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Integration User",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register gagal: status=%d body=%s", w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	if wLogin.Code != http.StatusOK || resp.Data.AccessToken == "" {
		t.Fatalf("login gagal: status=%d body=%s", wLogin.Code, wLogin.Body.String())
	}
	return resp.Data.AccessToken
}

func TestAuthLogin_Success(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Login User",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register gagal: %d %s", w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login sukses seharusnya 200, got %d (%s)", wLogin.Code, wLogin.Body.String())
	}
	if resp.Data.AccessToken == "" {
		t.Fatalf("access_token kosong")
	}
	if resp.Data.RefreshToken == "" {
		t.Fatalf("refresh_token kosong")
	}
	if resp.Data.UserType != "customer" {
		t.Fatalf("user_type = %q, want customer", resp.Data.UserType)
	}
	if resp.Data.UserID == uuid.Nil {
		t.Fatalf("user_id kosong")
	}
}

func TestAuthLogin_InvalidCredentials(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Invalid Cred User",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register gagal: %d %s", w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "wrong-password",
	}, "")
	if wLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", wLogin.Code, wLogin.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("expected INVALID_CREDENTIALS, got %+v", resp.Error)
	}
}

func TestAuthRegister_DuplicateEmail(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	body := gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Dup User",
		"user_type": "customer",
	}

	w1, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", body, "")
	if w1.Code != http.StatusCreated {
		t.Fatalf("register pertama gagal: %d %s", w1.Code, w1.Body.String())
	}

	w2, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", body, "")
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (%s)", w2.Code, w2.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "EMAIL_ALREADY_REGISTERED" {
		t.Fatalf("expected EMAIL_ALREADY_REGISTERED, got %+v", resp.Error)
	}
}

// doRawJSON mengirim request dengan body string mentah (untuk menguji badan
// JSON yang tidak valid).
func doRawJSON(t *testing.T, r *gin.Engine, method, path, rawBody, token string) (*httptest.ResponseRecorder, apiResponse) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var out apiResponse
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

// registerAndLoginRole mendaftar user dengan role tertentu lalu login.
func registerAndLoginRole(t *testing.T, r *gin.Engine, role string) string {
	t.Helper()
	email := randomEmail()
	body := gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Role " + role,
		"user_type": role,
	}
	if role == "driver" {
		body["vehicle_plate"] = "B 1234 XY"
		body["license_number"] = "SIM-12345"
	}

	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", body, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register role %s gagal: %d %s", role, w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	if wLogin.Code != http.StatusOK || resp.Data.AccessToken == "" {
		t.Fatalf("login role %s gagal: %d %s", role, wLogin.Code, wLogin.Body.String())
	}
	return resp.Data.AccessToken
}

// TestAuthAuthenticatedRoute_MissingToken memastikan endpoint yang dilindungi
// menolak request tanpa token (menutup branch AuthMiddleware header kosong).
func TestAuthAuthenticatedRoute_MissingToken(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, _ := doJSON(t, r, http.MethodGet, "/api/v1/me", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestAuthProtectedRoute_Authorized memvalidasi token via middleware dan
// menutup ValidateToken + IsBlacklisted pada path sukses.
func TestAuthProtectedRoute_Authorized(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	token := registerAndLoginRole(t, r, "customer")
	w, _ := doJSON(t, r, http.MethodGet, "/api/v1/me", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestAuthLogout_RevokesToken menutup Logout handler + blacklist.Add, lalu
// memastikan token yang sudah di-blacklist ditolak (IsBlacklisted).
func TestAuthLogout_RevokesToken(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	token := registerAndLoginRole(t, r, "customer")

	// Token valid sebelumnya.
	w0, _ := doJSON(t, r, http.MethodGet, "/api/v1/me", nil, token)
	if w0.Code != http.StatusOK {
		t.Fatalf("token harus valid sebelum logout: %d", w0.Code)
	}

	// Logout.
	wLogout, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/logout", gin.H{}, token)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("logout gagal: %d %s", wLogout.Code, wLogout.Body.String())
	}

	// Token yang sama harus ditolak setelah logout (revoked via blacklist).
	w1, _ := doJSON(t, r, http.MethodGet, "/api/v1/me", nil, token)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 setelah logout, got %d (%s)", w1.Code, w1.Body.String())
	}
}

// TestAuthRBAC_Forbidden: role customer tidak boleh akses endpoint admin.
func TestAuthRBAC_Forbidden(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	token := registerAndLoginRole(t, r, "customer")
	w, _ := doJSON(t, r, http.MethodGet, "/api/v1/admin/panel", nil, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestAuthRBAC_Allowed: role admin boleh akses endpoint admin.
func TestAuthRBAC_Allowed(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	token := registerAndLoginRole(t, r, "admin")
	w, _ := doJSON(t, r, http.MethodGet, "/api/v1/admin/panel", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestAuthLogin_AccountInactive: akun non-ACTIVE harus 403 ACCOUNT_INACTIVE.
func TestAuthLogin_AccountInactive(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Inactive User",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register gagal: %d %s", w.Code, w.Body.String())
	}

	// Paksa status USER menjadi SUSPENDED.
	if _, err := tc.pool.Exec(t.Context(),
		`UPDATE users SET status = 'SUSPENDED' WHERE email = $1`, email); err != nil {
		t.Fatalf("update status: %v", err)
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	if wLogin.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", wLogin.Code, wLogin.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "ACCOUNT_INACTIVE" {
		t.Fatalf("expected ACCOUNT_INACTIVE, got %+v", resp.Error)
	}
}

// TestAuthLogin_InvalidEmailFormat: email tidak valid -> 422 INVALID_EMAIL.
func TestAuthLogin_InvalidEmailFormat(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     "bukan-email",
		"password":  "password123",
		"name":      "Bad Email",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_EMAIL" {
		t.Fatalf("expected INVALID_EMAIL, got %+v", resp.Error)
	}
}

// TestAuthRegister_ShortPassword: password < 8 karakter -> 422 INVALID_PASSWORD.
func TestAuthRegister_ShortPassword(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     randomEmail(),
		"password":  "short",
		"name":      "Short Pass",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_PASSWORD" {
		t.Fatalf("expected INVALID_PASSWORD, got %+v", resp.Error)
	}
}

// TestAuthRegister_InvalidUserType: user_type tidak dikenal -> 422.
func TestAuthRegister_InvalidUserType(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     randomEmail(),
		"password":  "password123",
		"name":      "Bad Type",
		"user_type": "superuser",
	}, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_USER_TYPE" {
		t.Fatalf("expected INVALID_USER_TYPE, got %+v", resp.Error)
	}
}

// TestAuthLogin_WrongPassword: password salah -> 401 INVALID_CREDENTIALS.
func TestAuthLogin_WrongPassword(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Wrong Pass",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register gagal: %d %s", w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "definitely-wrong",
	}, "")
	if wLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", wLogin.Code, wLogin.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("expected INVALID_CREDENTIALS, got %+v", resp.Error)
	}
}

// TestAuthRegister_MissingName: name kosong -> 422 INVALID_NAME.
func TestAuthRegister_MissingName(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     randomEmail(),
		"password":  "password123",
		"name":      "",
		"user_type": "customer",
	}, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_NAME" {
		t.Fatalf("expected INVALID_NAME, got %+v", resp.Error)
	}
}

// TestAuthRegister_DriverSuccess: registrasi driver lengkap (dengan
// vehicle_type, plate, license, phone) menutup branch assignment data driver
// dan branch phone non-kosong pada handler Register.
func TestAuthRegister_DriverSuccess(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":          randomEmail(),
		"password":       "password123",
		"name":           "Full Driver",
		"user_type":      "driver",
		"phone":          "081234567890",
		"vehicle_type":   "motorcycle",
		"vehicle_plate":  "B 1234 XY",
		"license_number": "SIM-12345",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("registrasi driver seharusnya sukses, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Data.UserType != "driver" {
		t.Fatalf("user_type = %q, want driver", resp.Data.UserType)
	}
}

// TestAuthRegister_CustomerWithPhone: registrasi customer dengan phone
// menutup branch phone non-kosong.
func TestAuthRegister_CustomerWithPhone(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     randomEmail(),
		"password":  "password123",
		"name":      "Customer Phone",
		"user_type": "customer",
		"phone":     "+6281234567890",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("registrasi customer gagal, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestAuthLogin_EmptyCredentials: email/password kosong -> 400 INVALID_REQUEST.
func TestAuthLogin_EmptyCredentials(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    "",
		"password": "",
	}, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %+v", resp.Error)
	}
}

// TestAuthLogin_MalformedJSON: badan JSON rusak -> 400 INVALID_REQUEST.
func TestAuthLogin_MalformedJSON(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doRawJSON(t, r, http.MethodPost, "/api/v1/auth/login", "{not-valid-json", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %+v", resp.Error)
	}
}

// TestAuthRegister_MalformedJSON: badan JSON rusak -> 400 INVALID_REQUEST.
func TestAuthRegister_MalformedJSON(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doRawJSON(t, r, http.MethodPost, "/api/v1/auth/register", "{oops", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %+v", resp.Error)
	}
}

// TestAuthRegister_DriverMissingInfo: driver tanpa plate/license -> 422.
func TestAuthRegister_DriverMissingInfo(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	w, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     randomEmail(),
		"password":  "password123",
		"name":      "Driver No Info",
		"user_type": "driver",
	}, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != "INVALID_DRIVER_INFO" {
		t.Fatalf("expected INVALID_DRIVER_INFO, got %+v", resp.Error)
	}
}

// TestAuthRegister_DriverHasWallets_Integration (TD-128): register driver
// membentuk user + 2 wallet (CUSTOMER + DRIVER) dalam 1 transaksi.
// Memverifikasi GET /wallets/me?type=CUSTOMER & ?type=DRIVER → 200.
func TestAuthRegister_DriverHasWallets_Integration(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":          email,
		"password":       "password123",
		"name":           "Driver Wallet",
		"user_type":      "driver",
		"vehicle_type":   "motorcycle",
		"vehicle_plate":  "B 1234 XY",
		"license_number": "SIM-12345",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("registrasi driver gagal: %d (%s)", w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	if wLogin.Code != http.StatusOK || resp.Data.AccessToken == "" {
		t.Fatalf("login driver gagal: %d (%s)", wLogin.Code, wLogin.Body.String())
	}

	for _, wt := range []string{"CUSTOMER", "DRIVER"} {
		wWallet, _ := doJSON(t, r, http.MethodGet, "/api/v1/wallets/me?type="+wt, nil, resp.Data.AccessToken)
		if wWallet.Code != http.StatusOK {
			t.Fatalf("GET /wallets/me?type=%s harus 200, got %d (%s)", wt, wWallet.Code, wWallet.Body.String())
		}
	}
}

// TestAuthRegister_MerchantHasWallet_Integration (TD-128 scope expansion):
// register merchant membentuk wallet CUSTOMER + MERCHANT. GET
// /wallets/me?type=MERCHANT → 200.
func TestAuthRegister_MerchantHasWallet_Integration(t *testing.T) {
	tc := setupTestConfig(t)
	r := tc.newRouter()

	email := randomEmail()
	w, _ := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", gin.H{
		"email":     email,
		"password":  "password123",
		"name":      "Merchant Wallet",
		"user_type": "merchant",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("registrasi merchant gagal: %d (%s)", w.Code, w.Body.String())
	}

	wLogin, resp := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", gin.H{
		"email":    email,
		"password": "password123",
	}, "")
	if wLogin.Code != http.StatusOK || resp.Data.AccessToken == "" {
		t.Fatalf("login merchant gagal: %d (%s)", wLogin.Code, wLogin.Body.String())
	}

	for _, wt := range []string{"CUSTOMER", "MERCHANT"} {
		wWallet, _ := doJSON(t, r, http.MethodGet, "/api/v1/wallets/me?type="+wt, nil, resp.Data.AccessToken)
		if wWallet.Code != http.StatusOK {
			t.Fatalf("GET /wallets/me?type=%s harus 200, got %d (%s)", wt, wWallet.Code, wWallet.Body.String())
		}
	}
}
