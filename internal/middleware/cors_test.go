package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCORSRouter(origins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(origins))
	r.GET("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	return r
}

func doCORSSimpleRequest(r http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodOptions {
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, X-Admin-2FA-Token")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestCORS_Preflight_ValidOrigin: OPTIONS dari origin ter-whitelist -> 204
// dengan header ACAO di-echo, Allow-Methods, Allow-Headers, Allow-Credentials.
func TestCORS_Preflight_ValidOrigin(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000"})
	w := doCORSSimpleRequest(r, http.MethodOptions, "/api/v1/admin/login", "http://localhost:3000")

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
}

// TestCORS_GET_ValidOrigin: request aktual dari origin ter-whitelist -> handler
// dieksekusi (200) + header ACAO di-echo.
func TestCORS_GET_ValidOrigin(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000"})
	w := doCORSSimpleRequest(r, http.MethodGet, "/api/v1/auth/login", "http://localhost:3000")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// TestCORS_OriginInvalid: origin di luar whitelist -> handler tetap jalan (200)
// tapi TANPA header ACAO (browser memblokir). Preflight origin invalid -> 204
// tanpa ACAO.
func TestCORS_OriginInvalid(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000"})

	getW := doCORSSimpleRequest(r, http.MethodGet, "/api/v1/auth/login", "http://evil.example.com")
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Empty(t, getW.Header().Get("Access-Control-Allow-Origin"))

	preW := doCORSSimpleRequest(r, http.MethodOptions, "/api/v1/auth/login", "http://evil.example.com")
	require.Equal(t, http.StatusNoContent, preW.Code)
	assert.Empty(t, preW.Header().Get("Access-Control-Allow-Origin"))
}

// TestCORS_WildcardDevOnly: mode "*" (dev only) mengizinkan semua origin via
// ACAO:*, TAPI tidak men-set Allow-Credentials (invalid di browser).
func TestCORS_WildcardDevOnly(t *testing.T) {
	r := newCORSRouter([]string{"*"})
	w := doCORSSimpleRequest(r, http.MethodGet, "/api/v1/auth/login", "http://anything.example.com")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
}

// TestParseOrigins: env koma-separated dipecah & di-trim; kosong -> default.
func TestParseOrigins(t *testing.T) {
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3100", "http://localhost:8080"}, ParseOrigins(""))
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3100"}, ParseOrigins("http://localhost:3000, http://localhost:3100"))
}