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

func (m *mockWalletService) GetBalance(ctx context.Context, walletID uuid.UUID) (decimal.Decimal, error) {
	args := m.Called(ctx, walletID)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

func (m *mockWalletService) ProcessTopUpWebhook(ctx context.Context, txnID uuid.UUID) error {
	args := m.Called(ctx, txnID)
	return args.Error(0)
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

// ---- GetBalance ----

func TestHandler_GetBalance_Success(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetBalance", mock.Anything, testHW).Return(decimal.NewFromInt(500000), nil)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")

	h.GetBalance(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decodeBody(t, w)["success"])
	svc.AssertExpectations(t)
}

func TestHandler_GetBalance_NotFound(t *testing.T) {
	svc := new(mockWalletService)
	h := NewHandler(svc)

	svc.On("GetBalance", mock.Anything, testHW).Return(decimal.Zero, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")

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

	svc.On("GetBalance", mock.Anything, testHW).Return(decimal.Zero, ErrWalletNotFound)

	c, w := newCtx(t, http.MethodGet, "/wallets/"+testHW.String()+"/balance",
		map[string]string{"wallet_id": testHW.String()}, "")

	h.GetBalance(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "WALLET_NOT_FOUND", errCode(t, decodeBody(t, w)))
	svc.AssertExpectations(t)
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

// ---- TestHandler_codeForError: mapping error -> error code (API_CONTRACT) ----

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
		{name: "unknown", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForError(tt.err))
		})
	}
}
