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
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// ledgerExportRequest adalah body POST /admin/ledger/export (filter sama E1).
type ledgerExportRequest struct {
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	DateFrom   string `json:"date_from"`
	DateTo     string `json:"date_to"`
	WalletType string `json:"wallet_type"`
	EntryType  string `json:"entry_type"`
	Search     string `json:"search"`
}

// GetLedgerList GET /admin/ledger
// Mengembalikan daftar ledger entries dengan filter opsional (offset, limit,
// date_from, date_to, wallet_type, entry_type, search) + total_count.
// Envelope {success, data: {ledger, total_count}} sesuai types.ts -> LedgerPage.
func (h *Handler) GetLedgerList(c *gin.Context) {
	f := LedgerFilter{Limit: 10, Offset: 0}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "limit harus bilangan bulat positif")
			return
		}
		f.Limit = min(n, 100)
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "offset harus bilangan bulat non-negatif")
			return
		}
		f.Offset = n
	}
	if v := c.Query("date_from"); v != "" {
		d, err := parseLedgerDate(v)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "date_from tidak valid (format RFC3339 atau YYYY-MM-DD)")
			return
		}
		f.DateFrom = &d
	}
	if v := c.Query("date_to"); v != "" {
		d, err := parseLedgerDate(v)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "date_to tidak valid (format RFC3339 atau YYYY-MM-DD)")
			return
		}
		f.DateTo = &d
	}
	f.WalletType = strings.TrimSpace(c.Query("wallet_type"))
	f.EntryType = strings.TrimSpace(c.Query("entry_type"))
	f.Search = strings.TrimSpace(c.Query("search"))

	list, err := h.svc.GetLedger(c.Request.Context(), f)
	if err != nil {
		h.logger.Error("get ledger failed", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengambil ledger")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// VerifyLedger GET /admin/ledger/verify/:wallet_id
// Memverifikasi saldo wallet terhadap agregat ledger entries (double-entry).
// Response {total_debit, total_credit, discrepancy, status} dengan status
// "BALANCED" | "MISMATCH" (types.ts -> BalanceVerification).
func (h *Handler) VerifyLedger(c *gin.Context) {
	walletID, err := uuid.Parse(c.Param("wallet_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_WALLET_ID", "wallet id tidak valid")
		return
	}

	result, err := h.svc.VerifyLedger(c.Request.Context(), walletID)
	if err != nil {
		switch {
		case errors.Is(err, ErrWalletNotFound):
			writeError(c, http.StatusNotFound, "WALLET_NOT_FOUND", "wallet tidak ditemukan")
		default:
			h.logger.Error("verify ledger failed", "wallet_id", walletID, "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal memverifikasi ledger")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ExportLedger POST /admin/ledger/export
// Menerima filter JSON (sama seperti E1) dan mengembalikan file CSV
// (Content-Type: text/csv, filename ledger-export-<timestamp>.csv).
func (h *Handler) ExportLedger(c *gin.Context) {
	var body ledgerExportRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "format body tidak valid")
		return
	}

	f := LedgerFilter{
		Offset:     body.Offset,
		WalletType: strings.TrimSpace(body.WalletType),
		EntryType:  strings.TrimSpace(body.EntryType),
		Search:     strings.TrimSpace(body.Search),
	}
	if body.Limit > 0 {
		f.Limit = body.Limit
	}
	if body.DateFrom != "" {
		d, err := parseLedgerDate(body.DateFrom)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "date_from tidak valid (format RFC3339 atau YYYY-MM-DD)")
			return
		}
		f.DateFrom = &d
	}
	if body.DateTo != "" {
		d, err := parseLedgerDate(body.DateTo)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "date_to tidak valid (format RFC3339 atau YYYY-MM-DD)")
			return
		}
		f.DateTo = &d
	}

	data, err := h.svc.ExportLedger(c.Request.Context(), f)
	if err != nil {
		h.logger.Error("export ledger failed", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengekspor ledger")
		return
	}

	filename := fmt.Sprintf("ledger-export-%d.csv", time.Now().Unix())
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv", data)
}

// userStatusRequestBody adalah body request PATCH /admin/users/:id/:action.
// reason bersifat opsional (frontend tidak mengirim body saat ini).
type userStatusRequestBody struct {
	Reason string `json:"reason"`
}

// GetUsers GET /admin/users
// Mengembalikan daftar user dengan filter opsional (offset, limit, role,
// status, search) + total_count. Envelope {success, data: {users, total_count}}
// sesuai apps/admin_web/src/lib/types.ts -> UserListResponse.
func (h *Handler) GetUsers(c *gin.Context) {
	f := UserFilter{Limit: 50, Offset: 0}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "limit harus bilangan bulat positif")
			return
		}
		f.Limit = min(n, 100)
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "offset harus bilangan bulat non-negatif")
			return
		}
		f.Offset = n
	}
	if role := strings.ToUpper(strings.TrimSpace(c.Query("role"))); role != "" {
		switch role {
		case "CUSTOMER", "DRIVER", "MERCHANT", "ADMIN", "SYSTEM":
			f.Role = role
		default:
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "role tidak valid")
			return
		}
	}
	f.Status = strings.ToUpper(strings.TrimSpace(c.Query("status")))
	f.Search = strings.TrimSpace(c.Query("search"))

	list, err := h.svc.GetUsers(c.Request.Context(), f)
	if err != nil {
		h.logger.Error("get users failed", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengambil daftar user")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// GetUserDetail GET /admin/users/:id
// Mengembalikan detail satu user (AdminUser). Envelope {success, data: {..}}.
func (h *Handler) GetUserDetail(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_USER_ID", "id user tidak valid")
		return
	}

	user, err := h.svc.GetUserDetail(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			writeError(c, http.StatusNotFound, "USER_NOT_FOUND", "user tidak ditemukan")
		default:
			h.logger.Error("get user detail failed", "user_id", id, "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mengambil detail user")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": user})
}

// UpdateUserStatus PATCH /admin/users/:id/:action (freeze|suspend|ban|unfreeze)
// Memetakan aksi ke status users: freeze->FROZEN, suspend->SUSPENDED,
// ban->DELETED (enum DB tidak punya BANNED — TD-052), unfreeze->ACTIVE.
// Menulis audit log admin_action_logs di dalam transaksi yang sama.
func (h *Handler) UpdateUserStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_USER_ID", "id user tidak valid")
		return
	}

	adminIDStr := c.GetString("user_id")
	adminID, err := uuid.Parse(adminIDStr)
	if err != nil || adminIDStr == "" {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "tidak dapat mengidentifikasi admin")
		return
	}

	action := c.Param("action")
	var body userStatusRequestBody
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "format body tidak valid")
			return
		}
	}

	result, err := h.svc.UpdateUserStatus(c.Request.Context(), UserStatusRequest{
		UserID:  id,
		AdminID: adminID,
		Action:  action,
		Reason:  body.Reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAction):
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "aksi tidak valid (freeze|suspend|ban|unfreeze)")
		case errors.Is(err, ErrUserNotFound):
			writeError(c, http.StatusNotFound, "USER_NOT_FOUND", "user tidak ditemukan")
		default:
			h.logger.Error("update user status failed", "user_id", id, "action", action, "admin_id", adminID, "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal memperbarui status user")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// parseLedgerDate mem-parse tanggal filter ledger (RFC3339 atau YYYY-MM-DD
// dari <input type="date"> frontend admin_web).
func parseLedgerDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
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
