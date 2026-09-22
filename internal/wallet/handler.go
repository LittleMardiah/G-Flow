// Package wallet — HTTP Handler (Gin).
//
// Handler hanya bertugas memetakan request HTTP menjadi panggilan Service
// dan men-map error ke status code + error code sesuai API_CONTRACT.
// Tidak mengandung business logic finansial (itu milik Service/Repository).
package wallet

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// WalletService adalah kontrak service yang dibutuhkan Handler.
// Dipenuhi oleh *Service; dijadikan interface agar mudah di-mock pada test.
type WalletService interface {
	TopUp(ctx context.Context, req TopUpRequest) (*TopUpResponse, error)
	Transfer(ctx context.Context, req TransferRequest) (*TransferResponse, error)
	GetBalance(ctx context.Context, userID uuid.UUID, walletID uuid.UUID) (decimal.Decimal, error)
	GetMyWallet(ctx context.Context, userID uuid.UUID, walletType string) (*MyWallet, error)
	GetWalletHistory(ctx context.Context, userID uuid.UUID, walletID uuid.UUID, referenceType string, page, pageSize int) (*WalletHistory, error)
	ProcessTopUpWebhook(ctx context.Context, txnID uuid.UUID) error
	UpdateWalletStatus(ctx context.Context, walletID uuid.UUID, adminID uuid.UUID, newStatus string, reason string) (*UpdateWalletStatusResult, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc WalletService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc WalletService) *Handler {
	return &Handler{svc: svc}
}

// request/response bodies.

type topUpRequestBody struct {
	Amount         decimal.Decimal `json:"amount"`
	IdempotencyKey string          `json:"idempotency_key"`
}

type transferRequestBody struct {
	ToWalletID     uuid.UUID       `json:"to_wallet_id"`
	Amount         decimal.Decimal `json:"amount"`
	IdempotencyKey string          `json:"idempotency_key"`
	Description    string          `json:"description"`
}

type webhookRequestBody struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Status        string    `json:"status"`
}

type updateWalletStatusRequestBody struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// ledgerEntryResponse adalah bentuk JSON satu entry sesuai DESIGN TD-058
// (align field API CONTRACT /ledger/audit, tanpa wallet_id karena konteksnya
// sudah scoped ke satu wallet).
type ledgerEntryResponse struct {
	LedgerID      uuid.UUID       `json:"ledger_id"`
	EntryType     string          `json:"entry_type"`
	Amount        decimal.Decimal `json:"amount"`
	BalanceAfter  decimal.Decimal `json:"balance_after"`
	ReferenceType string          `json:"reference_type"`
	ReferenceID   string          `json:"reference_id"`
	Description   string          `json:"description"`
	IsReversed    bool            `json:"is_reversed"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ledgerEntryToJSON memetakan LedgerEntry domain ke bentuk response.
// reference_id yang nil (uuid.Nil setelah COALESCE di repository) diekspos
// sebagai string kosong agar field tetap ada dan JSON-nya tidak bergantung
// pada nilai kebetulan.
func ledgerEntryToJSON(e LedgerEntry) ledgerEntryResponse {
	refID := ""
	if e.ReferenceID != uuid.Nil {
		refID = e.ReferenceID.String()
	}
	return ledgerEntryResponse{
		LedgerID:      e.ID,
		EntryType:     e.EntryType,
		Amount:        e.Amount,
		BalanceAfter:  e.BalanceAfter,
		ReferenceType: e.ReferenceType,
		ReferenceID:   refID,
		Description:   e.Description,
		IsReversed:    e.IsReversed,
		CreatedAt:     e.CreatedAt,
	}
}

// --- endpoints ---

// TopUp POST /api/v1/wallets/:wallet_id/topup
func (h *Handler) TopUp(c *gin.Context) {
	if _, err := uuid.Parse(c.Param("wallet_id")); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_WALLET_ID", "invalid wallet_id")
		return
	}

	var body topUpRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if !body.Amount.IsPositive() {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_AMOUNT", ErrInvalidAmount.Error())
		return
	}
	if body.IdempotencyKey == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_IDEMPOTENCY_KEY", "idempotency key is required")
		return
	}

	// userID diambil dari JWT claim (diset oleh AuthMiddleware), bukan
	// di-resolve dari wallet_id (auth tidak lagi stub — Phase 1.2).
	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.TopUp(c.Request.Context(), TopUpRequest{
		UserID:         userID,
		Amount:         body.Amount,
		IdempotencyKey: body.IdempotencyKey,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// Transfer POST /api/v1/wallets/:wallet_id/transfer
func (h *Handler) Transfer(c *gin.Context) {
	fromWalletID, err := uuid.Parse(c.Param("wallet_id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_WALLET_ID", "invalid wallet_id")
		return
	}

	var body transferRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if !body.Amount.IsPositive() {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_AMOUNT", ErrInvalidAmount.Error())
		return
	}
	if body.IdempotencyKey == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_IDEMPOTENCY_KEY", "idempotency key is required")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.Transfer(c.Request.Context(), TransferRequest{
		UserID:         userID,
		FromWalletID:   fromWalletID,
		ToWalletID:     body.ToWalletID,
		Amount:         body.Amount,
		IdempotencyKey: body.IdempotencyKey,
		Description:    body.Description,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// GetBalance GET /api/v1/wallets/:wallet_id/balance
func (h *Handler) GetBalance(c *gin.Context) {
	walletID, err := uuid.Parse(c.Param("wallet_id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_WALLET_ID", "invalid wallet_id")
		return
	}

	// userID dari JWT claim (AuthMiddleware). tanpa identitas valid -> 401.
	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	balance, err := h.svc.GetBalance(c.Request.Context(), userID, walletID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"wallet_id": walletID,
			"balance":   balance,
		},
	})
}

// GetMyWallet GET /api/v1/wallets/me (TD-120)
// Query: ?type=CUSTOMER (default CUSTOMER). Auto-resolve wallet milik user
// terautentikasi (JWT claim) berdasarkan user_id + wallet_type, jadi mobile
// tidak perlu tahu wallet_id. 200 {success, data:{wallet_id, balance, status,
// wallet_type}}; 404 (WALLET_NOT_FOUND) jika user tidak punya wallet tipe tsb.
func (h *Handler) GetMyWallet(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	walletType := strings.TrimSpace(c.Query("type"))
	if walletType == "" {
		walletType = WalletTypeCustomer
	}

	resp, err := h.svc.GetMyWallet(c.Request.Context(), userID, walletType)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// GetWalletHistory GET /api/v1/wallets/:wallet_id/history
// Query: ?reference_type=&page=1&page_size=20 (default page=1, page_size=20,
// max 50). Sort created_at DESC. Pemilik wallet (JWT claim) wajib cocok.
func (h *Handler) GetWalletHistory(c *gin.Context) {
	walletID, err := uuid.Parse(c.Param("wallet_id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_WALLET_ID", "invalid wallet_id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	page, pageSize := 1, 20
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "page harus bilangan bulat >= 1")
			return
		}
		page = n
	}
	if v := c.Query("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "page_size harus bilangan bulat 1..50")
			return
		}
		pageSize = n
	}

	resp, err := h.svc.GetWalletHistory(
		c.Request.Context(), userID, walletID,
		strings.TrimSpace(c.Query("reference_type")),
		page, pageSize,
	)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	entries := make([]ledgerEntryResponse, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entries = append(entries, ledgerEntryToJSON(e))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"entries": entries,
		},
		"meta": gin.H{
			"page":        resp.Page,
			"page_size":   resp.PageSize,
			"total":       resp.Total,
			"total_pages": resp.TotalPages,
		},
	})
}

// UpdateWalletStatus PATCH /api/v1/wallets/:wallet_id/status
// Khusus role admin (RBAC). Body: {status, reason}. Status non-ACTIVE wajib
// reason. admin_id dari JWT claim.
func (h *Handler) UpdateWalletStatus(c *gin.Context) {
	walletID, err := uuid.Parse(c.Param("wallet_id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_WALLET_ID", "invalid wallet_id")
		return
	}

	adminID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	var body updateWalletStatusRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	resp, err := h.svc.UpdateWalletStatus(
		c.Request.Context(), walletID, adminID,
		strings.TrimSpace(body.Status), strings.TrimSpace(body.Reason),
	)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"wallet_id":  resp.WalletID,
			"status":     resp.Status,
			"updated_at": resp.UpdatedAt,
		},
	})
}

// ProcessTopUpWebhook POST /webhooks/topup
func (h *Handler) ProcessTopUpWebhook(c *gin.Context) {
	var body webhookRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if err := h.svc.ProcessTopUpWebhook(c.Request.Context(), body.TransactionID); err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// --- helpers ---

// userIDFromContext mengambil user_id dari Gin context (diset oleh
// AuthMiddleware dari JWT claim `sub`). Mengembalikan false jika konteks
// tidak berisi identitas user yang valid.
func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return uuid.Nil, false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return userID, true
}

// statusForError memetakan error service ke status code HTTP.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrWalletNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrKycLimitExceeded),
		errors.Is(err, ErrInsufficientBalance),
		errors.Is(err, ErrSameWalletTransfer):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrIdempotencyInProgress):
		return http.StatusConflict
	case errors.Is(err, ErrWalletInactive):
		return http.StatusForbidden
	case errors.Is(err, ErrWalletNotOwned):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidPagination),
		errors.Is(err, ErrInvalidStatus),
		errors.Is(err, ErrReasonRequired):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// codeForError memetakan error service ke error code (API_CONTRACT 3.2).
func codeForError(err error) string {
	switch {
	case errors.Is(err, ErrWalletNotFound):
		return "WALLET_NOT_FOUND"
	case errors.Is(err, ErrInvalidAmount):
		return "INVALID_AMOUNT"
	case errors.Is(err, ErrKycLimitExceeded):
		return "KYC_LIMIT_EXCEEDED"
	case errors.Is(err, ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE"
	case errors.Is(err, ErrSameWalletTransfer):
		return "SELF_TRANSFER_NOT_ALLOWED"
	case errors.Is(err, ErrIdempotencyInProgress):
		return "IDEMPOTENCY_IN_PROGRESS"
	case errors.Is(err, ErrWalletInactive):
		return "WALLET_INACTIVE"
	case errors.Is(err, ErrWalletNotOwned):
		return "WALLET_NOT_OWNED"
	case errors.Is(err, ErrInvalidPagination),
		errors.Is(err, ErrInvalidStatus),
		errors.Is(err, ErrReasonRequired):
		return "INVALID_REQUEST"
	case errors.Is(err, ErrInvalidCachedResponse):
		return "INTERNAL_ERROR"
	default:
		return "INTERNAL_SERVER_ERROR"
	}
}

// writeError menulis error response sesuai format API_CONTRACT:
// { "success": false, "error": { "code": "...", "message": "..." } }
func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
