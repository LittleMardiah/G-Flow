package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/g-flow/g-flow/internal/auth"
)

// AuthMiddleware adalah middleware JWT yang memvalidasi token Bearer dari
// header Authorization, memeriksa blacklist Redis, lalu meng-set user_id dan
// user_type ke context Gin. Mengembalikan 401 UNAUTHORIZED jika gagal.
func AuthMiddleware(jwtService *auth.JWTService, blacklist *auth.BlacklistService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "invalid authorization header"})
			return
		}

		claims, err := jwtService.ValidateToken(parts[1])
		if err != nil || claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "invalid or expired token"})
			return
		}

		if blacklisted, _ := blacklist.IsBlacklisted(c.Request.Context(), claims.Jti); blacklisted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "token has been revoked"})
			return
		}

		c.Set("user_id", claims.Sub)
		c.Set("user_type", claims.UserType)
		c.Set("claims", claims)
		c.Next()
	}
}
