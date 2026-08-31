package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func doRBAC(t *testing.T, userType string, allowed ...string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/test",
		func(c *gin.Context) {
			if userType != "" {
				c.Set("user_type", userType)
			}
			c.Next()
		},
		RBACMiddleware(allowed...),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRBACMiddleware_Allowed(t *testing.T) {
	code := doRBAC(t, "admin", "admin", "customer")
	assert.Equal(t, http.StatusOK, code)
}

func TestRBACMiddleware_Forbidden(t *testing.T) {
	code := doRBAC(t, "customer", "admin")
	assert.Equal(t, http.StatusForbidden, code)
}

func TestRBACMiddleware_MissingUserType(t *testing.T) {
	code := doRBAC(t, "", "admin")
	assert.Equal(t, http.StatusForbidden, code)
}
