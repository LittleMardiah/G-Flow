// Package middleware — request logging (F014).
//
// Logger mencatat setiap request HTTP dalam format JSON terstruktur via
// log/slog (stdlib Go 1.21+). Setiap request mendapat X-Request-ID (dipakai
// header client jika ada, jika tidak digenerate UUID) agar mudah ditelusuri
// di agregasi log. Panic di-handler dicatat dengan stack trace.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestIDContextKey adalah key tempat X-Request-ID disimpan di Gin context.
const requestIDContextKey = "gflow.request_id"

// RequestID mengambil request_id dari context untuk penelusuran log.
func RequestID(c *gin.Context) string {
	return c.GetString(requestIDContextKey)
}

// Logger adalah middleware yang mencatat setiap request: method, path,
// status, duration_ms, user_id (dari JWT claim bila ada), dan request_id.
// Output JSON ke default slog logger (di-set ke JSON di cmd/api/main.go).
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// X-Request-ID: pertahankan header client, jika kosong generate UUID.
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Writer.Header().Set("X-Request-ID", reqID)
		c.Set(requestIDContextKey, reqID)

		c.Next()

		attrs := []slog.Attr{
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000.0),
		}
		if c.ClientIP() != "" {
			attrs = append(attrs, slog.String("client_ip", c.ClientIP()))
		}
		if userID := c.GetString("user_id"); userID != "" {
			attrs = append(attrs, slog.String("user_id", userID))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		slog.LogAttrs(c.Request.Context(), levelForStatus(c.Writer.Status()), "http_request", attrs...)
	}
}

// Recovery menangkap panic pada handler, mencatatnya sebagai error dengan
// stack trace (runtime/debug.Stack), lalu merespons 500 INTERNAL_SERVER_ERROR
// sesuai format API_CONTRACT.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("http_panic",
					slog.Any("panic", r),
					slog.String("request_id", c.GetString(requestIDContextKey)),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "INTERNAL_SERVER_ERROR",
						"message": "internal server error",
					},
				})
			}
		}()
		c.Next()
	}
}

// levelForStatus memetakan status HTTP ke level log: 5xx -> ERROR, 4xx -> WARN,
// selainnya -> INFO.
func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
