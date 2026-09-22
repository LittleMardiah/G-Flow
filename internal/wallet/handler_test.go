package wallet

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockWalletService adalah mock dari interface WalletService.
type mockWalletService struct {
	mock.Mock
}

func (m *mockWalletService) TopUp(ctx context.Context, req TopUpRequest) (*TopUpResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TopUpResponse), args.Error(1)
}

func (m *mockWalletService) Transfer(ctx context.Context, req TransferRequest) (*TransferResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TransferResponse), args.Error(1)
}

func (m *mockWalletService) GetBalance(ctx context.Context, userID uuid.UUID, walletID uuid.UUID) (decimal.Decimal, error) {
	args := m.Called(ctx, userID, walletID)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

func (m *mockWalletService) GetWalletHistory(ctx context.Context, userID uuid.UUID, walletID uuid.UUID, referenceType string, page, pageSize int) (*WalletHistory, error) {
	args := m.Called(ctx, userID, walletID, referenceType, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*WalletHistory), args.Error(1)
}

func (m *mockWalletService) ProcessTopUpWebhook(ctx context.Context, txnID uuid.UUID) error {
	args := m.Called(ctx, txnID)
	return args.Error(0)
}

func (m *mockWalletService) UpdateWalletStatus(ctx context.Context, walletID uuid.UUID, adminID uuid.UUID, newStatus string, reason string) (*UpdateWalletStatusResult, error) {
	args := m.Called(ctx, walletID, adminID, newStatus, reason)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UpdateWalletStatusResult), args.Error(1)
}

func init() {
	gin.SetMode(gin.TestMode)
}

// newCtx membuat context gin dengan path params yang sudah di-set.
func newCtx(t *testing.T, method, target string, params map[string]string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reader io.Reader = strings.NewReader("")
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	var ps gin.Params
	for k, v := range params {
		ps = append(ps, gin.Param{Key: k, Value: v})
	}
	c.Params = ps

	return c, w
}

var (
	testHW    = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	testHU    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testHFrom = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	testHTo   = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
)

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func errCode(t *testing.T, m map[string]interface{}) string {
	t.Helper()
	errObj, ok := m["error"].(map[string]interface{})
	if !ok {
		return ""
	}
	code, _ := errObj["code"].(string)
	return code
}

// ---- TopUp ----

func TestHandler_TopUp_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("TopUp", mock.Anything, mock.Anything).Return(&TopUpResponse{
		TransactionID: uuid.New(),
		WalletID:      testHW,
		Amount:        decimal.NewFromInt(500000),
		NewBalance:    decimal.NewFromInt(500000),
		Status:        redisCompleted,
	}, nil)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHW.String()+"/topup",
		map[string]string{"wallet_id": testHW.String()},
		`{"amount":500000,"idempotency_key":"topup-1"}`)
	c.Set("user_id", testHU.String())

	h.TopUp(c)

	assert.Equal(t, http.StatusOK, w.Code)
	m := decodeBody(t, w)
	assert.Equal(t, true, m["success"])
	data, ok := m["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "COMPLETED", data["status"])
	svc.AssertExpectations(t)
}

func TestHandler_TopUp_InvalidAmount(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHW.String()+"/topup",
		map[string]string{"wallet_id": testHW.String()},
		`{"amount":0,"idempotency_key":"topup-2"}`)

	h.TopUp(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_AMOUNT", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "TopUp", mock.Anything, mock.Anything)
}

func TestHandler_TopUp_IdempotencyInProgress(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("TopUp", mock.Anything, mock.Anything).Return(nil, ErrIdempotencyInProgress)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHW.String()+"/topup",
		map[string]string{"wallet_id": testHW.String()},
		`{"amount":100000,"idempotency_key":"topup-3"}`)
	c.Set("user_id", testHU.String())

	h.TopUp(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "IDEMPOTENCY_IN_PROGRESS", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_TopUp_Unauthorized(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHW.String()+"/topup",
		map[string]string{"wallet_id": testHW.String()},
		`{"amount":100000,"idempotency_key":"topup-4"}`)
	// user_id tidak diset di context -> harus 401, service tidak dipanggil.

	h.TopUp(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "TopUp", mock.Anything, mock.Anything)
}

// ---- Transfer ----

func TestHandler_Transfer_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("Transfer", mock.Anything, mock.Anything).Return(&TransferResponse{
		TransferID:  uuid.New(),
		FromBalance: decimal.NewFromInt(40000),
		ToBalance:   decimal.NewFromInt(60000),
	}, nil)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":50000,"idempotency_key":"tr-1","description":"p2p"}`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["success"])
	svc.AssertExpectations(t)
}

func TestHandler_Transfer_InsufficientBalance(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("Transfer", mock.Anything, mock.Anything).Return(nil, ErrInsufficientBalance)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":999999,"idempotency_key":"tr-2"}`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INSUFFICIENT_BALANCE", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_Transfer_Unauthorized(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":50000,"idempotency_key":"tr-3"}`)
	// user_id tidak diset -> 401, service tidak dipanggil.

	h.Transfer(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "Transfer", mock.Anything, mock.Anything)
}

func TestHandler_Transfer_InvalidBody(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{invalid json`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "Transfer", mock.Anything, mock.Anything)
}

func TestHandler_Transfer_NonPositiveAmount(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":0,"idempotency_key":"tr-4"}`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_AMOUNT", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "Transfer", mock.Anything, mock.Anything)
}

func TestHandler_Transfer_MissingIdempotency(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":50000}`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_IDEMPOTENCY_KEY", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "Transfer", mock.Anything, mock.Anything)
}

func TestHandler_Transfer_ServiceError(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("Transfer", mock.Anything, mock.Anything).Return(nil, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHFrom.String()+"/transfer",
		map[string]string{"wallet_id": testHFrom.String()},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":50000,"idempotency_key":"tr-5"}`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

// ---- GetBalance ----

func TestHandler_GetBalance_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetBalance", mock.Anything, testHU, testHW).Return(decimal.NewFromInt(500000), nil)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetBalance(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["success"])
	svc.AssertExpectations(t)
}

func TestHandler_GetBalance_NotFound(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetBalance", mock.Anything, testHU, testHW).Return(decimal.Zero, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetBalance(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

// ---- Webhook ----

func TestHandler_Webhook_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	txnID := uuid.New()
	svc.On("ProcessTopUpWebhook", mock.Anything, txnID).Return(nil)

	c, w := newCtx(t, http.MethodPost, "/webhooks/topup", nil,
		`{"transaction_id":"`+txnID.String()+`","status":"COMPLETED"}`)

	h.ProcessTopUpWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["success"])
	svc.AssertExpectations(t)
}

// ---- Tambahan coverage (refactor) ----

func TestTopUp_InvalidWalletID(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/not-a-uuid/topup",
		map[string]string{"wallet_id": "not-a-uuid"},
		`{"amount":100000,"idempotency_key":"topup-x"}`)
	c.Set("user_id", testHU.String())

	h.TopUp(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_WALLET_ID", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "TopUp", mock.Anything, mock.Anything)
}

func TestTopUp_MissingIdempotency(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/"+testHW.String()+"/topup",
		map[string]string{"wallet_id": testHW.String()},
		`{"amount":100000}`)
	c.Set("user_id", testHU.String())

	h.TopUp(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "INVALID_IDEMPOTENCY_KEY", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "TopUp", mock.Anything, mock.Anything)
}

func TestTransfer_InvalidWalletID(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/wallets/not-a-uuid/transfer",
		map[string]string{"wallet_id": "not-a-uuid"},
		`{"to_wallet_id":"`+testHTo.String()+`","amount":50000,"idempotency_key":"tr-x"}`)
	c.Set("user_id", testHU.String())

	h.Transfer(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_WALLET_ID", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "Transfer", mock.Anything, mock.Anything)
}

func TestGetBalance_NotFound(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetBalance", mock.Anything, testHU, testHW).Return(decimal.Zero, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetBalance(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestGetBalance_InvalidWalletID(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodGet, "/wallets/not-a-uuid/balance",
		map[string]string{"wallet_id": "not-a-uuid"}, "")
	c.Set("user_id", testHU.String())

	h.GetBalance(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_WALLET_ID", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "GetBalance", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetBalance_Unauthorized(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")

	h.GetBalance(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "GetBalance", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessTopUpWebhook_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	txnID := uuid.New()
	svc.On("ProcessTopUpWebhook", mock.Anything, txnID).Return(nil)

	c, w := newCtx(t, http.MethodPost, "/webhooks/topup", nil,
		`{"transaction_id":"`+txnID.String()+`","status":"COMPLETED"}`)

	h.ProcessTopUpWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["success"])
	svc.AssertExpectations(t)
}

func TestHandler_Webhook_InvalidBody(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodPost, "/webhooks/topup", nil, `{invalid json`)

	h.ProcessTopUpWebhook(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "ProcessTopUpWebhook", mock.Anything, mock.Anything)
}

func TestHandler_Webhook_Error(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	txnID := uuid.New()
	svc.On("ProcessTopUpWebhook", mock.Anything, txnID).Return(ErrWalletNotFound)

	c, w := newCtx(t, http.MethodPost, "/webhooks/topup", nil,
		`{"transaction_id":"`+txnID.String()+`","status":"COMPLETED"}`)

	h.ProcessTopUpWebhook(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

// ---- UpdateWalletStatus ----

func TestHandler_UpdateWalletStatus_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	updatedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.On("UpdateWalletStatus", mock.Anything, testHW, testHU, "SUSPENDED", "fraud investigation").
		Return(&UpdateWalletStatusResult{
			WalletID:  testHW,
			Status:    "SUSPENDED",
			UpdatedAt: updatedAt,
		}, nil)

	c, w := newCtx(t, http.MethodPatch, "/wallets/"+testHW.String()+"/status",
		map[string]string{"wallet_id": testHW.String()},
		`{"status":"SUSPENDED","reason":"fraud investigation"}`)
	c.Set("user_id", testHU.String())

	h.UpdateWalletStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)
	m := decodeBody(t, w)
	assert.Equal(t, true, m["success"])
	data, ok := m["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, testHW.String(), data["wallet_id"])
	assert.Equal(t, "SUSPENDED", data["status"])
	assert.Equal(t, updatedAt.Format(time.RFC3339Nano), data["updated_at"])
	svc.AssertExpectations(t)
}

func TestHandler_UpdateWalletStatus_InvalidStatus(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("UpdateWalletStatus", mock.Anything, testHW, testHU, "BLOCKED", "x").
		Return(nil, ErrInvalidStatus)

	c, w := newCtx(t, http.MethodPatch, "/wallets/"+testHW.String()+"/status",
		map[string]string{"wallet_id": testHW.String()},
		`{"status":"BLOCKED","reason":"x"}`)
	c.Set("user_id", testHU.String())

	h.UpdateWalletStatus(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_UpdateWalletStatus_ReasonRequired(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("UpdateWalletStatus", mock.Anything, testHW, testHU, "FROZEN", "").
		Return(nil, ErrReasonRequired)

	c, w := newCtx(t, http.MethodPatch, "/wallets/"+testHW.String()+"/status",
		map[string]string{"wallet_id": testHW.String()},
		`{"status":"FROZEN"}`)
	c.Set("user_id", testHU.String())

	h.UpdateWalletStatus(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_UpdateWalletStatus_NotFound(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("UpdateWalletStatus", mock.Anything, testHW, testHU, "SUSPENDED", "wallet tidak ada").
		Return(nil, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodPatch, "/wallets/"+testHW.String()+"/status",
		map[string]string{"wallet_id": testHW.String()},
		`{"status":"SUSPENDED","reason":"wallet tidak ada"}`)
	c.Set("user_id", testHU.String())

	h.UpdateWalletStatus(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_codeForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "wallet not found", err: ErrWalletNotFound, want: "WALLET_NOT_FOUND"},
		{name: "invalid amount", err: ErrInvalidAmount, want: "INVALID_AMOUNT"},
		{name: "kyc limit exceeded", err: ErrKycLimitExceeded, want: "KYC_LIMIT_EXCEEDED"},
		{name: "insufficient balance", err: ErrInsufficientBalance, want: "INSUFFICIENT_BALANCE"},
		{name: "self transfer", err: ErrSameWalletTransfer, want: "SELF_TRANSFER_NOT_ALLOWED"},
		{name: "idempotency in progress", err: ErrIdempotencyInProgress, want: "IDEMPOTENCY_IN_PROGRESS"},
		{name: "wallet inactive", err: ErrWalletInactive, want: "WALLET_INACTIVE"},
		{name: "wallet not owned", err: ErrWalletNotOwned, want: "WALLET_NOT_OWNED"},
		{name: "invalid pagination", err: ErrInvalidPagination, want: "INVALID_REQUEST"},
		{name: "invalid status", err: ErrInvalidStatus, want: "INVALID_REQUEST"},
		{name: "reason required", err: ErrReasonRequired, want: "INVALID_REQUEST"},
		{name: "invalid cached response", err: ErrInvalidCachedResponse, want: "INTERNAL_ERROR"},
		{name: "unknown error", err: errors.New("boom"), want: "INTERNAL_SERVER_ERROR"},
		{name: "nil error", err: nil, want: "INTERNAL_SERVER_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, codeForError(tt.err))
		})
	}
}

// TestHandler_statusForError: mapping error -> HTTP status untuk menutup
// cabang statusForError yang belum ter-cover.
func TestHandler_statusForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "wallet not found", err: ErrWalletNotFound, want: http.StatusNotFound},
		{name: "invalid amount", err: ErrInvalidAmount, want: http.StatusUnprocessableEntity},
		{name: "kyc limit", err: ErrKycLimitExceeded, want: http.StatusUnprocessableEntity},
		{name: "insufficient", err: ErrInsufficientBalance, want: http.StatusUnprocessableEntity},
		{name: "self transfer", err: ErrSameWalletTransfer, want: http.StatusUnprocessableEntity},
		{name: "idempotency", err: ErrIdempotencyInProgress, want: http.StatusConflict},
		{name: "wallet inactive", err: ErrWalletInactive, want: http.StatusForbidden},
		{name: "wallet not owned", err: ErrWalletNotOwned, want: http.StatusForbidden},
		{name: "invalid pagination", err: ErrInvalidPagination, want: http.StatusBadRequest},
		{name: "invalid status", err: ErrInvalidStatus, want: http.StatusBadRequest},
		{name: "reason required", err: ErrReasonRequired, want: http.StatusBadRequest},
		{name: "unknown", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForError(tt.err))
		})
	}
}

// ---- GetWalletHistory ----

// sampleHistory membuat WalletHistory contoh dengan 2 entries.
func sampleHistory() *WalletHistory {
	return &WalletHistory{
		Entries: []LedgerEntry{
			{
				ID:            uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				WalletID:      testHW,
				EntryType:     EntryDebit,
				Amount:        decimal.NewFromInt(50000),
				BalanceAfter:  decimal.NewFromInt(150000),
				ReferenceID:   uuid.MustParse("55555555-5555-5555-5555-555555555555"),
				ReferenceType: "TRANSFER",
				Description:   "TRANSFER - from wallet",
				CreatedAt:     time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC),
			},
			{
				ID:            uuid.MustParse("66666666-6666-6666-6666-666666666666"),
				WalletID:      testHW,
				EntryType:     EntryCredit,
				Amount:        decimal.NewFromInt(100000),
				BalanceAfter:  decimal.NewFromInt(200000),
				ReferenceID:   uuid.Nil,
				ReferenceType: "TOPUP",
				Description:   "TOPUP - CREDIT CUSTOMER wallet",
				CreatedAt:     time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC),
			},
		},
		Page:       1,
		PageSize:   20,
		Total:      2,
		TotalPages: 1,
	}
}

func TestHandler_GetWalletHistory_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetWalletHistory", mock.Anything, testHU, testHW, "TRANSFER", 1, 20).Return(sampleHistory(), nil)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/history?reference_type=TRANSFER&page=1&page_size=20",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetWalletHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)
	m := decodeBody(t, w)
	assert.Equal(t, true, m["success"])
	meta, ok := m["meta"].(map[string]interface{})
	assert.True(t, ok)
	assert.EqualValues(t, 1, meta["page"])
	assert.EqualValues(t, 20, meta["page_size"])
	assert.EqualValues(t, 2, meta["total"])
	assert.EqualValues(t, 1, meta["total_pages"])

	data, ok := m["data"].(map[string]interface{})
	assert.True(t, ok)
	entries, ok := data["entries"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, entries, 2)

	first, ok := entries[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "44444444-4444-4444-4444-444444444444", first["ledger_id"])
	assert.Equal(t, "DEBIT", first["entry_type"])
	assert.Equal(t, "TRANSFER", first["reference_type"])
	assert.Equal(t, "55555555-5555-5555-5555-555555555555", first["reference_id"])
	assert.Equal(t, false, first["is_reversed"])

	// reference_id nil -> string kosong, field tetap ada.
	second, ok := entries[1].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "", second["reference_id"])
	svc.AssertExpectations(t)
}

func TestHandler_GetWalletHistory_NotFound(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetWalletHistory", mock.Anything, testHU, testHW, "", 1, 20).Return(nil, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/history",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetWalletHistory(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_GetWalletHistory_NotOwned(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetWalletHistory", mock.Anything, testHU, testHW, "", 1, 20).Return(nil, ErrWalletNotOwned)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/history",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetWalletHistory(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "WALLET_NOT_OWNED", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_GetWalletHistory_InvalidPaginationFromService(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetWalletHistory", mock.Anything, testHU, testHW, "", 1, 20).Return(nil, ErrInvalidPagination)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/history",
		map[string]string{"wallet_id": testHW.String()}, "")
	c.Set("user_id", testHU.String())

	h.GetWalletHistory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_REQUEST", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
}

func TestHandler_GetWalletHistory_InvalidWalletID(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodGet, "/wallets/not-a-uuid/history",
		map[string]string{"wallet_id": "not-a-uuid"}, "")
	c.Set("user_id", testHU.String())

	h.GetWalletHistory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_WALLET_ID", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "GetWalletHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_GetWalletHistory_Unauthorized(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/history",
		map[string]string{"wallet_id": testHW.String()}, "")
	// user_id tidak diset -> 401, service tidak dipanggil.

	h.GetWalletHistory(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, decodeBody(t, w)))
	svc.AssertNotCalled(t, "GetWalletHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_GetWalletHistory_BadQuery(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	tests := []string{
		"/wallets/" + testHW.String() + "/history?page=abc",
		"/wallets/" + testHW.String() + "/history?page_size=0",
		"/wallets/" + testHW.String() + "/history?page_size=51",
	}
	for _, target := range tests {
		c, w := newCtx(t, http.MethodGet, target, map[string]string{"wallet_id": testHW.String()}, "")
		c.Set("user_id", testHU.String())
		h.GetWalletHistory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "INVALID_REQUEST", errCode(t, decodeBody(t, w)))
	}
	svc.AssertNotCalled(t, "GetWalletHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
