package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSecurityHeadersTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestSecurityHeaders_Default(t *testing.T) {
	r := setupSecurityHeadersTestRouter()
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "0", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"))
	// HTTP tidak boleh set HSTS
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_WithHTTPS(t *testing.T) {
	r := setupSecurityHeadersTestRouter()
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_WithHTTPProto(t *testing.T) {
	r := setupSecurityHeadersTestRouter()
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "http")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}
