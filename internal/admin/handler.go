// Package admin — HTTP Handler (Gin) untuk Transaction Reversal (Task 4.2.4).
//
// Handler memetakan request HTTP POST /api/v1/admin/transactions/{id}/reverse
// menjadi panggilan Service.ReverseTransaction dengan penerapan keamanan:
//
//	1. Autentikasi (AuthMiddleware) + RBAC role admin — dipasang di main.go.
//	2. Validasi 2FA header X-Admin-2FA-Token (simulasi MVP).
//	3. Lockout akun setelah 3x kegagalan 2FA (15 menit).
//
// Error code:
//
//	401 UNAUTHORIZED_2FA  — token 2FA tidak valid / tidak ada
//	403 FORBIDDEN         — role bukan admin (ditangani RBAC middleware)
//	422 INVALID_REQUEST   — body/reason tidak valid
//	429 LOCKOUT           — akun terkunci
package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/g-flow/g-flow/internal/auth"
)

// Handler menerima request HTTP terkait administrasi.
type Handler struct {
	svc    *Service
	db     DB
	redis  *redis.Client
	logger *slog.Logger
	twoFA  TwoFactorValidator
	jwt    *auth.JWTService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc *Service, db DB, rdb *redis.Client, logger *slog.Logger, twoFA TwoFactorValidator, jwtService *auth.JWTService) *Handler {
	return &Handler{svc: svc, db: db, redis: rdb, logger: logger, twoFA: twoFA, jwt: jwtService}
}

// reverseRequestBody adalah body request POST .../reverse.
type reverseRequestBody struct {
	Reason string `json:"reason"`
	Notes  string `json:"notes"`
}

// adminLoginRequestBody adalah body request POST /admin/login.
type adminLoginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ReverseTransaction POST /admin/transactions/:id/reverse
func (h *Handler) ReverseTransaction(c *gin.Context) {
	idParam := c.Param("id")
	transactionID, err := uuid.Parse(idParam)
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_TRANSACTION_ID", "id transaksi tidak valid")
		return
	}

	// Admin identity dari AuthMiddleware (claims.sub).
	adminIDStr := c.GetString("user_id")
	adminID, err := uuid.Parse(adminIDStr)
	if err != nil || adminIDStr == "" {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "tidak dapat mengidentifikasi admin")
		return
	}

	// Lockout check sebelum validasi 2FA.
	locked, err := h.checkLockout(c.Request.Context(), adminID)
	if err != nil {
		h.logger.Warn("lockout check failed", "admin_id", adminID, "error", err)
	}
	if locked {
		writeError(c, http.StatusTooManyRequests, "LOCKOUT", "akun terkunci karena terlalu banyak percobaan 2FA; coba lagi 15 menit lagi")
		return
	}

	// Validasi 2FA.
	twoFAToken := c.GetHeader(admin2FASecretHeader)
	if twoFAToken == "" {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED_2FA", "header X-Admin-2FA-Token wajib diisi")
		return
	}
	if !h.twoFA.Validate(twoFAToken) {
		attempts, _ := h.recordFailedAttempt(c.Request.Context(), adminID)
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED_2FA", "token 2FA tidak valid")
		_ = attempts
		return
	}
	h.resetAttempts(c.Request.Context(), adminID)

	// Bind body.
	var body reverseRequestBody
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "format body tidak valid")
			return
		}
	}
	if body.Reason == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_REQUEST", "reason wajib diisi")
		return
	}

	result, err := h.svc.ReverseTransaction(c.Request.Context(), ReversalRequest{
		TransactionID: transactionID,
		AdminID:       adminID,
		Reason:        body.Reason,
		Notes:         body.Notes,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTransactionNotFound):
			writeError(c, http.StatusNotFound, "TRANSACTION_NOT_FOUND", "transaksi tidak ditemukan")
		case errors.Is(err, ErrTransactionReversed):
			writeError(c, http.StatusConflict, "ALREADY_REVERSED", "transaksi sudah pernah di-reverse")
		case errors.Is(err, ErrNotBalanced):
			writeError(c, http.StatusUnprocessableEntity, "LEDGER_IMBALANCE", "ledger tidak seimbang untuk reversal")
		case errors.Is(err, ErrInvalidReason):
			writeError(c, http.StatusUnprocessableEntity, "INVALID_REQUEST", "reason wajib diisi")
		default:
			h.logger.Error("reversal failed", "transaction_id", transactionID, "admin_id", adminID, "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal membalikkan transaksi")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"reversal_id": result.ReversalID,
			"transaction_id": transactionID,
			"refunded":       result.Refunded,
			"shortfall":      result.Shortfall,
			"has_sweep":      result.HasSweep,
		},
	})
}

// GetTransaction GET /admin/transactions/:id
// Menampilkan detail transaksi + breakdown proporsional merchant/driver/platform
// agar halaman reversal bisa menampilkan rincian sebelum konfirmasi.
func (h *Handler) GetTransaction(c *gin.Context) {
	idParam := c.Param("id")
	transactionID, err := uuid.Parse(idParam)
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_TRANSACTION_ID", "id transaksi tidak valid")
		return
	}

	detail, err := h.svc.GetTransactionDetail(c.Request.Context(), transactionID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTransactionNotFound):
			writeError(c, http.StatusNotFound, "TRANSACTION_NOT_FOUND", "transaksi tidak ditemukan")
		default:
			h.logger.Error("get transaction detail failed", "transaction_id", transactionID, "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengambil detail transaksi")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

// GetDashboardKPIs GET /admin/dashboard/kpis
// Mengembalikan ringkasan KPI dashboard admin (envelope {success, data}).
// Error internal dipetakan ke 500 INTERNAL_SERVER_ERROR.
func (h *Handler) GetDashboardKPIs(c *gin.Context) {
	kpis, err := h.svc.GetDashboardKPIs(c.Request.Context())
	if err != nil {
		h.logger.Error("get dashboard kpis failed", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengambil KPI dashboard")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": kpis})
}

// GetDashboardTransactions GET /admin/dashboard/transactions?limit=&offset=
// Mengembalikan daftar transaksi ledger terbaru (pagination, default limit=10
// offset=0, limit dibatasi maksimal 100).
func (h *Handler) GetDashboardTransactions(c *gin.Context) {
	limit, offset := 10, 0
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "limit harus bilangan bulat positif")
			return
		}
		limit = min(n, 100)
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "offset harus bilangan bulat non-negatif")
			return
		}
		offset = n
	}

	list, err := h.svc.GetDashboardTransactions(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("get dashboard transactions failed", "limit", limit, "offset", offset, "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengambil daftar transaksi")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// AdminLogin POST /admin/login (publik, tanpa auth middleware)
// Memvalidasi kredensial admin lalu menerbitkan access + refresh token JWT.
func (h *Handler) AdminLogin(c *gin.Context) {
	var body adminLoginRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "format body tidak valid")
		return
	}
	if strings.TrimSpace(body.Email) == "" || body.Password == "" {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "email dan password wajib diisi")
		return
	}

	result, err := h.svc.Login(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email atau password salah")
		case errors.Is(err, ErrNotAdmin):
			writeError(c, http.StatusForbidden, "FORBIDDEN", "akun bukan admin")
		case errors.Is(err, ErrAccountInactive):
			writeError(c, http.StatusForbidden, "ACCOUNT_INACTIVE", "akun tidak aktif (suspended/frozen)")
		default:
			h.logger.Error("admin login failed", "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal memproses login")
		}
		return
	}

	accessToken, refreshToken, err := h.jwt.GenerateToken(result.UserID, result.Email, result.UserType)
	if err != nil {
		h.logger.Error("admin token generation failed", "user_id", result.UserID, "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal membuat token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"user_id":       result.UserID,
			"user_type":     result.UserType,
		},
	})
}

// writeError menulis error response sesuai format API_CONTRACT.
func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
