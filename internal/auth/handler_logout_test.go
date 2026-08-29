//go:build integration

// Package auth — unit test murni (tanpa mock/DB) untuk Handler.Logout dan
// BlacklistService, menutup branch yang tidak terjangkau HTTP integration
// karena middleware selalu mengisi claims sebelumnya.
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// newLogoutHandler membangun Handler dengan DB nil & blacklist nil (tidak
// dipakai pada branch yang diuji).
func newLogoutHandler() *Handler {
	return &Handler{
		jwtService: NewJWTService("secret"),
		blacklist:  NewBlacklistService(nil),
		db:         nil,
	}
}

// withRequest melengkapi context test dengan sebuah request agar
// c.Request.Context() tidak nil.
func withRequest(c *gin.Context) {
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
}

// TestLogout_NoClaims: claims tidak ada -> 401 UNAUTHORIZED.
func TestLogout_NoClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newLogoutHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	withRequest(c)
	h.Logout(c)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestLogout_EmptyJti: claims ada tapi jti kosong -> 401 UNAUTHORIZED.
func TestLogout_EmptyJti(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newLogoutHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	withRequest(c)
	c.Set("claims", &Claims{Sub: uuid.New().String()}) // Jti kosong
	h.Logout(c)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestLogout_SuccessCoverage: blacklist Add dipanggil (nil Redis -> nil).
func TestLogout_SuccessCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newLogoutHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	withRequest(c)
	c.Set("claims", &Claims{
		Sub: uuid.New().String(),
		Jti: uuid.New().String(),
		Exp: time.Now().Add(time.Hour).Unix(),
	})
	h.Logout(c)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestBlacklistAdd_ExpiredJti menutup branch TTL negatif (return nil dini).
func TestBlacklistAdd_ExpiredJti(t *testing.T) {
	s := NewBlacklistService(nil)
	if err := s.Add(context.Background(), "some-jti", time.Now().Add(-time.Hour).Unix()); err != nil {
		t.Fatalf("expected nil untuk jti sudah lewat, got %v", err)
	}
}
