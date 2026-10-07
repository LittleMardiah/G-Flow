package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/auth"
)

// setupAuthRevokeTest menyiapkan JWTService nyata + BlacklistService di atas
// miniredis (pola internal/auth/blacklist_unit_test.go) dan menghasilkan satu
// token asli sehingga claims (Sub/Iat) valid untuk middleware.
func setupAuthRevokeTest(t *testing.T) (*auth.JWTService, *auth.BlacklistService, uuid.UUID, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	jwtService := auth.NewJWTService("test-secret-td174")
	blacklist := auth.NewBlacklistService(rdb)

	userID := uuid.New()
	token, _, err := jwtService.GenerateToken(userID, "user@example.com", "customer")
	require.NoError(t, err)

	return jwtService, blacklist, userID, token
}

// testRouter membuat gin.Engine dengan route GET /test yang echo
// c.GetString("user_id") — dipakai untuk memverifikasi context.
func testRouter(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.GET("/test", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})
	return r
}

// doAuthRequest menjalankan GET /test dengan token opsional ("" = tanpa header).
func doAuthRequest(r *gin.Engine, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAuthMiddleware_NoToken_401: tanpa header Authorization -> 401.
func TestAuthMiddleware_NoToken_401(t *testing.T) {
	jwtService, blacklist, _, _ := setupAuthRevokeTest(t)
	r := testRouter(AuthMiddleware(jwtService, blacklist))

	w := doAuthRequest(r, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAuthMiddleware_ValidToken_NoRevoke_Next: token valid tanpa key revoke
// -> handler dipanggil (200 OK) dan user_id di-set dari claims.Sub.
func TestAuthMiddleware_ValidToken_NoRevoke_Next(t *testing.T) {
	jwtService, blacklist, userID, token := setupAuthRevokeTest(t)
	r := testRouter(AuthMiddleware(jwtService, blacklist))

	w := doAuthRequest(r, token)
	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		UserID string `json:"user_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, userID.String(), body.UserID)
}

// TestAuthMiddleware_ValidToken_Revoked_401: RevokeUserBefore(userID, now+1h)
// membuat token (iat=now < cutoff) -> 401 dengan code UNAUTHORIZED.
func TestAuthMiddleware_ValidToken_Revoked_401(t *testing.T) {
	jwtService, blacklist, userID, token := setupAuthRevokeTest(t)

	cutoff := time.Now().Add(time.Hour).Unix()
	require.NoError(t, blacklist.RevokeUserBefore(t.Context(), userID.String(), cutoff))

	r := testRouter(AuthMiddleware(jwtService, blacklist))
	w := doAuthRequest(r, token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
	assert.NotEmpty(t, body.Error.Message)
}

// TestAuthMiddleware_ValidToken_RevokeCutoffBeforeIat_Next: cutoff = now-1h
// sedangkan iat token = now (iat > cutoff) -> tidak dianggap revoked
// (skenario user di-unsuspend / re-enabled), handler tetap dipanggil 200.
func TestAuthMiddleware_ValidToken_RevokeCutoffBeforeIat_Next(t *testing.T) {
	jwtService, blacklist, userID, token := setupAuthRevokeTest(t)

	cutoff := time.Now().Add(-time.Hour).Unix()
	require.NoError(t, blacklist.RevokeUserBefore(t.Context(), userID.String(), cutoff))

	r := testRouter(AuthMiddleware(jwtService, blacklist))
	w := doAuthRequest(r, token)
	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		UserID string `json:"user_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, userID.String(), body.UserID)
}

// TestOptionalAuthMiddleware_Revoked_Anonymous: user revoked -> handler
// tetap dipanggil (200) tapi user_id TIDAK di-set (perilaku anonim,
// konsisten dengan behavior blacklist existing).
func TestOptionalAuthMiddleware_Revoked_Anonymous(t *testing.T) {
	jwtService, blacklist, userID, token := setupAuthRevokeTest(t)

	cutoff := time.Now().Add(time.Hour).Unix()
	require.NoError(t, blacklist.RevokeUserBefore(t.Context(), userID.String(), cutoff))

	r := testRouter(OptionalAuthMiddleware(jwtService, blacklist))
	w := doAuthRequest(r, token)
	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		UserID string `json:"user_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Empty(t, body.UserID, "user revoked harus diperlakukan anonim")
}

// TestOptionalAuthMiddleware_Valid_UserIDSet: token valid tanpa revoke ->
// user_id di-set di context dan di-echo handler.
func TestOptionalAuthMiddleware_Valid_UserIDSet(t *testing.T) {
	jwtService, blacklist, userID, token := setupAuthRevokeTest(t)
	r := testRouter(OptionalAuthMiddleware(jwtService, blacklist))

	w := doAuthRequest(r, token)
	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		UserID string `json:"user_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, userID.String(), body.UserID)
}
