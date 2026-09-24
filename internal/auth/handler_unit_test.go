package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/g-flow/g-flow/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHandler_Register_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	cfg := &config.Config{
		App: config.AppConfig{
			JWTSecret: "test-secret",
		},
	}
	jwtSvc := NewJWTService(cfg.App.JWTSecret)
	blacklist := NewBlacklistService(nil)
	svc := NewHandler(jwtSvc, blacklist, mockDB)

	email := "test@example.com"
	name := "Test User"
	phone := "081234567890"
	userType := "customer"
	userID := uuid.New()

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	mockDB.ExpectBegin()

	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, phone, name, userType, pgxmock.AnyArg(), nil, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(userID))

	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(userID, "CUSTOMER").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mockDB.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	body := `{"email":"test@example.com","password":"Test123!","name":"Test User","phone":"081234567890","user_type":"customer"}`
	c.Request.Body = io.NopCloser(strings.NewReader(body))

	svc.Register(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, userID.String(), resp["data"].(map[string]interface{})["user_id"])

	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_DuplicateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	cfg := &config.Config{
		App: config.AppConfig{
			JWTSecret: "test-secret",
		},
	}
	jwtSvc := NewJWTService(cfg.App.JWTSecret)
	blacklist := NewBlacklistService(nil)
	svc := NewHandler(jwtSvc, blacklist, mockDB)

	email := "test@example.com"

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	body := `{"email":"test@example.com","password":"Test123!","name":"Test User","phone":"081234567890","user_type":"customer"}`
	c.Request.Body = io.NopCloser(strings.NewReader(body))

	svc.Register(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "EMAIL_ALREADY_REGISTERED", resp["error"].(map[string]interface{})["code"])

	require.NoError(t, mockDB.ExpectationsWereMet())
}

// --- Login ---

// runLogin mengeksekusi Handler.Login dalam context test dengan body JSON.
func runLogin(t *testing.T, mockDB pgxmock.PgxPoolIface, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{App: config.AppConfig{JWTSecret: "test-secret"}}
	svc := NewHandler(NewJWTService(cfg.App.JWTSecret), NewBlacklistService(nil), mockDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Body = io.NopCloser(strings.NewReader(body))
	svc.Login(c)
	return w
}

// hashPassword menghasilkan bcrypt hash untuk password tertentu.
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(hash)
}

// decodeErrorCode mengekstrak error.code dari response JSON.
func decodeErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp["error"].(map[string]interface{})["code"].(string)
}

func TestHandler_Login_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	password := "Password123!"
	userID := uuid.New()

	mockDB.ExpectQuery(`SELECT id, email, user_type, status, password_hash FROM users`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "user_type", "status", "password_hash"}).
			AddRow(userID, email, "customer", "ACTIVE", hashPassword(t, password)))

	w := runLogin(t, mockDB, `{"email":"test@example.com","password":"Password123!"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["access_token"])
	assert.NotEmpty(t, data["refresh_token"])
	assert.Equal(t, userID.String(), data["user_id"])
	assert.Equal(t, "customer", data["user_type"])

	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Login_InvalidPassword(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	userID := uuid.New()

	mockDB.ExpectQuery(`SELECT id, email, user_type, status, password_hash FROM users`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "user_type", "status", "password_hash"}).
			AddRow(userID, email, "customer", "ACTIVE", hashPassword(t, "DifferentPass!")))

	w := runLogin(t, mockDB, `{"email":"test@example.com","password":"Password123!"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "INVALID_CREDENTIALS", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Login_UserNotFound(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockDB.ExpectQuery(`SELECT id, email, user_type, status, password_hash FROM users`).
		WithArgs("test@example.com").
		WillReturnError(pgx.ErrNoRows)

	w := runLogin(t, mockDB, `{"email":"test@example.com","password":"Password123!"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "INVALID_CREDENTIALS", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Login_AccountInactive(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	userID := uuid.New()

	mockDB.ExpectQuery(`SELECT id, email, user_type, status, password_hash FROM users`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "user_type", "status", "password_hash"}).
			AddRow(userID, email, "customer", "SUSPENDED", hashPassword(t, "Password123!")))

	w := runLogin(t, mockDB, `{"email":"test@example.com","password":"Password123!"}`)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "ACCOUNT_INACTIVE", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Login_EmptyCredentials(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runLogin(t, mockDB, `{"email":"","password":""}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Login_QueryError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockDB.ExpectQuery(`SELECT id, email, user_type, status, password_hash FROM users`).
		WithArgs("test@example.com").
		WillReturnError(errors.New("connection refused"))

	w := runLogin(t, mockDB, `{"email":"test@example.com","password":"Password123!"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

// --- Register: validation & error paths ---

// runRegister mengeksekusi Handler.Register dalam context test dengan body JSON.
func runRegister(t *testing.T, mockDB pgxmock.PgxPoolIface, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{App: config.AppConfig{JWTSecret: "test-secret"}}
	svc := NewHandler(NewJWTService(cfg.App.JWTSecret), NewBlacklistService(nil), mockDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Body = io.NopCloser(strings.NewReader(body))
	svc.Register(c)
	return w
}

func TestHandler_Register_InvalidBody(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{invalid-json`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_InvalidEmail(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{"email":"not-an-email","password":"Password123!","name":"T","user_type":"customer"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_EMAIL", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_ShortPassword(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{"email":"a@b.com","password":"short","name":"T","user_type":"customer"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_PASSWORD", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_EmptyName(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{"email":"a@b.com","password":"Password123!","name":"","user_type":"customer"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_NAME", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_InvalidUserType(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{"email":"a@b.com","password":"Password123!","name":"T","user_type":"hacker"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_USER_TYPE", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

// TestHandler_Register_AdminUserTypeRejected: user_type "admin" TIDAK boleh
// mendaftar via public register -> 422 INVALID_USER_TYPE, dan DB TIDAK dipanggil
// (tidak ada expectation, jadi ExpectationsWereMet membuktikan tidak ada query).
func TestHandler_Register_AdminUserTypeRejected(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{"email":"admin@test.com","password":"Password123!","name":"Admin","user_type":"admin"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_USER_TYPE", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_DriverMissingInfo(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	w := runRegister(t, mockDB, `{"email":"a@b.com","password":"Password123!","name":"T","user_type":"driver"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_DRIVER_INFO", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_ExistsQueryError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs("test@example.com").
		WillReturnError(errors.New("db down"))

	w := runRegister(t, mockDB, `{"email":"test@example.com","password":"Password123!","name":"T","user_type":"customer"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_BeginError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs("test@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin().WillReturnError(errors.New("no conn"))

	w := runRegister(t, mockDB, `{"email":"test@example.com","password":"Password123!","name":"T","user_type":"customer"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_InsertOnConflict(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, nil, "Test User", "customer", pgxmock.AnyArg(), nil, nil, nil).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	w := runRegister(t, mockDB, `{"email":"test@example.com","password":"Password123!","name":"Test User","user_type":"customer"}`)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "EMAIL_ALREADY_REGISTERED", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_InsertError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, nil, "Test User", "customer", pgxmock.AnyArg(), nil, nil, nil).
		WillReturnError(errors.New("constraint failure"))

	w := runRegister(t, mockDB, `{"email":"test@example.com","password":"Password123!","name":"Test User","user_type":"customer"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_WalletError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, nil, "Test User", "customer", pgxmock.AnyArg(), nil, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(pgxmock.AnyArg(), "CUSTOMER").
		WillReturnError(errors.New("wallet failed"))

	w := runRegister(t, mockDB, `{"email":"test@example.com","password":"Password123!","name":"Test User","user_type":"customer"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_CommitError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "test@example.com"
	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, nil, "Test User", "customer", pgxmock.AnyArg(), nil, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(pgxmock.AnyArg(), "CUSTOMER").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectCommit().WillReturnError(errors.New("commit failed"))

	w := runRegister(t, mockDB, `{"email":"test@example.com","password":"Password123!","name":"Test User","user_type":"customer"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestHandler_Register_DriverSuccess(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "driver@example.com"
	userID := uuid.New()

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, "081234567891", "Driver One", "driver", pgxmock.AnyArg(), "car", "B 1234 CD", "LIC-001").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(userID))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(userID, "CUSTOMER").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(userID, "DRIVER").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectCommit()

	w := runRegister(t, mockDB, `{"email":"DRIVER@Example.COM","password":"Password123!","name":"Driver One","phone":"081234567891","user_type":"driver","vehicle_type":"car","vehicle_plate":"B 1234 CD","license_number":"LIC-001"}`)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, userID.String(), resp["data"].(map[string]interface{})["user_id"])
	assert.Equal(t, "driver", resp["data"].(map[string]interface{})["user_type"])

	require.NoError(t, mockDB.ExpectationsWereMet())
}

// TestHandler_Register_MerchantSuccess: registrasi merchant membuat 2 wallet
// (CUSTOMER + MERCHANT) dalam satu transaksi (TD-128 scope expansion).
func TestHandler_Register_MerchantSuccess(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "merchant@example.com"
	userID := uuid.New()

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, nil, "Merchant One", "merchant", pgxmock.AnyArg(), nil, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(userID))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(userID, "CUSTOMER").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(userID, "MERCHANT").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectCommit()

	w := runRegister(t, mockDB, `{"email":"merchant@example.com","password":"Password123!","name":"Merchant One","user_type":"merchant"}`)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, userID.String(), resp["data"].(map[string]interface{})["user_id"])
	assert.Equal(t, "merchant", resp["data"].(map[string]interface{})["user_type"])

	require.NoError(t, mockDB.ExpectationsWereMet())
}

// TestHandler_Register_DriverSecondWalletError: wallet DRIVER gagal insert →
// 500 (rollback transaksi; user dan wallet CUSTOMER tidak jadi tersimpan).
func TestHandler_Register_DriverSecondWalletError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	email := "driver2@example.com"

	mockDB.ExpectQuery(`SELECT EXISTS`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mockDB.ExpectBegin()
	mockDB.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, nil, "Driver Two", "driver", pgxmock.AnyArg(), "motorcycle", "B 5678 EF", "LIC-002").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(pgxmock.AnyArg(), "CUSTOMER").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectExec(`INSERT INTO wallets`).
		WithArgs(pgxmock.AnyArg(), "DRIVER").
		WillReturnError(errors.New("wallet driver failed"))

	w := runRegister(t, mockDB, `{"email":"driver2@example.com","password":"Password123!","name":"Driver Two","user_type":"driver","vehicle_type":"motorcycle","vehicle_plate":"B 5678 EF","license_number":"LIC-002"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeErrorCode(t, w.Body.Bytes()))
	require.NoError(t, mockDB.ExpectationsWereMet())
}
