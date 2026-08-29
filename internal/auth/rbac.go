// Package auth — Role-Based Access Control (RBAC) middleware.
//
// RBACMiddleware membatasi akses endpoint hanya untuk role tertentu yang
// diizinkan (user_type dari claims JWT yang sudah di-set oleh AuthMiddleware).
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RBACMiddleware mengembalikan gin.HandlerFunc yang membolehkan request hanya
// jika user_type pada context termasuk dalam allowedRoles.
//
//	Dipakai setelah AuthMiddleware, contoh:
//
//	router.GET("/wallets/:id/balance",
//	    middleware.AuthMiddleware(jwtService, blacklistService),
//	    auth.RBACMiddleware("customer", "driver", "merchant"),
//	    walletHandler.GetBalance,
//	)
func RBACMiddleware(allowedRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		userType := c.GetString("user_type")
		if userType == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "FORBIDDEN", "message": "user_type tidak ditemukan"})
			return
		}

		if _, ok := allowed[userType]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "FORBIDDEN", "message": "role tidak diizinkan mengakses resource ini"})
			return
		}

		c.Next()
	}
}
