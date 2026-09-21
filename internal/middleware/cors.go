// Package middleware — CORS (cross-origin resource sharing).
//
// Middleware menangani permintaan lintas-asal dari Admin Web (localhost:3000)
// ke backend (localhost:8080). Tanpa ini, browser memblokir request via
// kebijakan same-origin dan preflight OPTIONS berakhir 404 karena tidak ada
// route yang mendaftar method OPTIONS.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultCORSOrigins adalah daftar origin bawaan bila env CORS_ORIGINS kosong.
// Mencakup Admin Web (3000), landing/app lain (3100), dan backend itu sendiri
// (8080) agar antar-layer bisa saling memanggil.
const DefaultCORSOrigins = "http://localhost:3000,http://localhost:3100,http://localhost:8080"

// CORS mengembalikan gin.HandlerFunc yang menerapkan whitelist origin:
//   - Preflight OPTIONS → 204 langsung dengan header CORS (tanpa masuk route,
//     jadi tidak kena AuthMiddleware).
//   - Actual request dengan Origin ter-whitelist → header AC-Allow-Origin di-echo
//     + Allow-Credentials true (browser butuh ini untuk header Authorization &
//     X-Admin-2FA-Token dari Admin Web).
//   - Origin tidak dikenal → request tetap diteruskan tapi TANPA header ACAO
//     (browser memblokir response); preflight di-stop 204.
//
// Mode dev "*": jika daftar berisi "*", semua origin diizinkan via ACAO:*.
// Allow-Credentials sengaja TIDAK di-set (kombinasi * + credentials dilarang
// browser). Gunakan "*" hanya untuk development lokal, bukan production.
func CORS(origins []string) gin.HandlerFunc {
	allowAll := false
	allow := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
			continue
		}
		allow[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Header("Vary", "Origin")

		var allowOrigin string
		switch {
		case allowAll:
			allowOrigin = "*"
		case origin != "":
			if _, ok := allow[origin]; ok {
				allowOrigin = origin
			}
		}

		if allowOrigin == "" {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)
		if !allowAll && origin != "" {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			reqHeaders := c.GetHeader("Access-Control-Request-Headers")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if reqHeaders != "" {
				c.Header("Access-Control-Allow-Headers", reqHeaders)
			} else {
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-2FA-Token, X-Request-ID")
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// ParseOrigins memecah env CORS_ORIGINS (koma-separated, whitespace di-trim)
// menjadi slice. Jika env kosong, mengembalikan DefaultCORSOrigins.
func ParseOrigins(env string) []string {
	if strings.TrimSpace(env) == "" {
		env = DefaultCORSOrigins
	}
	var out []string
	for _, p := range strings.Split(env, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}