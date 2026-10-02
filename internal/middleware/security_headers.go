package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware menambahkan security headers standar OWASP pada semua
// response API. Header yang diterapkan:
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - Strict-Transport-Security: max-age=31536000; includeSubDomains
//     (hanya jika request via HTTPS; dicek via c.Request.TLS != nil atau X-Forwarded-Proto)
//   - Referrer-Policy: no-referrer
//   - X-XSS-Protection: 0
//   - Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Referrer-Policy", "no-referrer")
		c.Writer.Header().Set("X-XSS-Protection", "0")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// HSTS hanya saat HTTPS (dev localhost HTTP → skip HSTS)
		if isHTTPS(c) {
			c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

func isHTTPS(c *gin.Context) bool {
	if c.Request != nil && c.Request.TLS != nil {
		return true
	}
	proto := strings.ToLower(c.GetHeader("X-Forwarded-Proto"))
	return proto == "https"
}
