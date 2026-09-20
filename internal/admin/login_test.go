package admin

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/g-flow/g-flow/internal/auth"
)

// testPassword dipakai sebagai password bersama pada test login admin
// (cocok dengan seed admin: AdminP@ssw0rd!2026).
const testPassword = "AdminP@ssw0rd!2026"

// newTestJWT membuat JWTService dummy untuk handler."
func newTestJWT() *auth.JWTService {
	return auth.NewJWTService("test-jwt-secret")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func loginRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/admin/login", h.AdminLogin)
	return r
}

func mockLoginUserRow(id uuid.UUID, email, userType, status, hash string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "email", "user_type", "status", "password_hash"}).
		AddRow(id, email, userType, status, hash)
}

func doLogin(router http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func newLoginHandler(t *testing.T, mDB pgxmock.PgxPoolIface) *Handler {
	t.Helper()
	svc := NewService(NewRepository(mDB), mDB, testLogger())
	return NewHandler(svc, mDB, nil, testLogger(), NewStaticTwoFactorValidator(""), newTestJWT())
}

// TestAdminLogin_Success: kredensial valid -> 200 + access_token + profile.
func TestAdminLogin_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, "admin@g-flow.local", "admin", "ACTIVE", mustHash(t, testPassword)))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{"email":"Admin@g-Flow.local","password":"`+testPassword+`"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.Contains(t, w.Body.String(), "refresh_token")
	assert.Contains(t, w.Body.String(), testAdminID.String())
	assert.Contains(t, w.Body.String(), `"user_type":"admin"`)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_WrongPassword: password salah -> 401 INVALID_CREDENTIALS.
func TestAdminLogin_WrongPassword(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, "admin@g-flow.local", "admin", "ACTIVE", mustHash(t, testPassword)))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{"email":"admin@g-flow.local","password":"wrong-password"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_NotAdmin: akun customer login via /admin/login -> 403 FORBIDDEN.
func TestAdminLogin_NotAdmin(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(uuid.New(), "customer@g-flow.local", "customer", "ACTIVE", mustHash(t, testPassword)))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{"email":"customer@g-flow.local","password":"`+testPassword+`"}`)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "FORBIDDEN")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_Inactive: status != ACTIVE -> 403 ACCOUNT_INACTIVE
// (error code konsisten dengan /auth/login).
func TestAdminLogin_Inactive(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, "admin@g-flow.local", "admin", "SUSPENDED", mustHash(t, testPassword)))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{"email":"admin@g-flow.local","password":"`+testPassword+`"}`)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_INACTIVE")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_NotAdminWrongPassword: akun non-admin + password salah harus
// 401 INVALID_CREDENTIALS (bcrypt dicek DULU sebelum role/status). Ini regresi
// FIX 1 — tanpa swapping order, user customer bisa di-enumerasi.
func TestAdminLogin_NotAdminWrongPassword(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(uuid.New(), "customer@g-flow.local", "customer", "ACTIVE", mustHash(t, testPassword)))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{"email":"customer@g-flow.local","password":"wrong-password"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_EmailNotExist_NoTimingLeak: email yang tidak terdaftar harus
// memakan waktu sebanding dengan kasus "password salah" (keduanya menjalankan
// bcrypt.CompareHashAndPassword dengan cost yang sama), supaya attacker tidak
// bisa membedakan email terdaftar vs tidak lewat timing response.
func TestAdminLogin_EmailNotExist_NoTimingLeak(t *testing.T) {
	// Baseline: admin valid + password salah -> jalankan bcrypt (cost 12).
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	hash12, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcryptCost)
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mockLoginUserRow(testAdminID, "admin@g-flow.local", "admin", "ACTIVE", string(hash12)))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	start := time.Now()
	w := doLogin(router, `{"email":"admin@g-flow.local","password":"wrong-password"}`)
	baseline := time.Since(start)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NoError(t, mDB.ExpectationsWereMet())

	// Kasus: email tidak terdaftar -> dummy bcrypt (cost 12).
	mDB2, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB2.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "user_type", "status", "password_hash"}))

	h2 := newLoginHandler(t, mDB2)
	router2 := loginRouter(h2)
	start = time.Now()
	w2 := doLogin(router2, `{"email":"unknown@g-flow.local","password":"wrong-password"}`)
	elapsed := time.Since(start)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
	assert.Contains(t, w2.Body.String(), "INVALID_CREDENTIALS")
	assert.NoError(t, mDB2.ExpectationsWereMet())

	t.Logf("elapsed not-found=%s, wrong-password=%s", elapsed, baseline)
	// Email tak terdaftar boleh tidak identik, tapi TIDAK boleh signifikan
	// lebih cepat (jika dummy bcrypt tidak dijalankan, response ~0.1ms).
	if elapsed < baseline/5 {
		t.Fatalf("email tak terdaftar terlalu cepat (%s vs %s): diduga kebocoran timing", elapsed, baseline)
	}
}

// TestAdminLogin_MissingField: body tanpa email/password -> 400 INVALID_REQUEST.
func TestAdminLogin_MissingField(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestAdminLogin_EmailNotFound: email tidak terdaftar -> 401 INVALID_CREDENTIALS.
func TestAdminLogin_EmailNotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	mDB.ExpectQuery("FROM users WHERE email").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "user_type", "status", "password_hash"}))

	h := newLoginHandler(t, mDB)
	router := loginRouter(h)
	w := doLogin(router, `{"email":"unknown@g-flow.local","password":"`+testPassword+`"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestServiceLogin_InvalidEmail: format email invalid -> ErrInvalidCredentials
// tanpa menyentuh database.
func TestServiceLogin_InvalidEmail(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	svc := NewService(NewRepository(mDB), mDB, testLogger())
	_, err = svc.Login(context.Background(), "bukan-email", "password")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	assert.NoError(t, mDB.ExpectationsWereMet())
}