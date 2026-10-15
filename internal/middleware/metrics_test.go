package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestErrorRate_EmptyWindow: tanpa request -> 0.
func TestErrorRate_EmptyWindow(t *testing.T) {
	resetErrorRate()
	assert.Equal(t, float64(0), GetErrorRate())
}

// TestErrorRate_Ratio: 5xx/total = 2/3.
func TestErrorRate_Ratio(t *testing.T) {
	resetErrorRate()
	observeStatus(http.StatusOK)
	observeStatus(http.StatusInternalServerError)
	observeStatus(http.StatusBadGateway)
	assert.InDelta(t, 2.0/3.0, GetErrorRate(), 1e-9)
}

// TestErrorRate_WindowExpired: window >= 24 jam -> anggap kosong (0),
// observe berikutnya reset window dengan counter fresh.
func TestErrorRate_WindowExpired(t *testing.T) {
	resetErrorRate()
	observeStatus(http.StatusInternalServerError)
	errorRateState.mu.Lock()
	errorRateState.windowStart = time.Now().Add(-errorRateWindow - time.Second)
	errorRateState.mu.Unlock()
	assert.Equal(t, float64(0), GetErrorRate())
	observeStatus(http.StatusOK)
	assert.Equal(t, float64(0), GetErrorRate())
}

// TestErrorRate_LoggerHook: Logger middleware memanggil observeStatus
// dengan status final handler (alur end-to-end).
func TestErrorRate_LoggerHook(t *testing.T) {
	resetErrorRate()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger())
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, float64(1), GetErrorRate())
}
