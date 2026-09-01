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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"log/slog"
)

// Handler menerima request HTTP terkait administrasi.
type Handler struct {
	svc    *Service
	db     DB
	redis  *redis.Client
	logger *slog.Logger
	twoFA  TwoFactorValidator
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc *Service, db DB, rdb *redis.Client, logger *slog.Logger, twoFA TwoFactorValidator) *Handler {
	return &Handler{svc: svc, db: db, redis: rdb, logger: logger, twoFA: twoFA}
}

// reverseRequestBody adalah body request POST .../reverse.
type reverseRequestBody struct {
	Reason string `json:"reason"`
	Notes  string `json:"notes"`
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
