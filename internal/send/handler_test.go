package send

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSendService struct {
	mock.Mock
}

func (m *mockSendService) CreateSendOrder(ctx context.Context, req CreateSendOrderRequest) (*CreateSendOrderResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CreateSendOrderResponse), args.Error(1)
}

func (m *mockSendService) GetSendOrder(ctx context.Context, orderID, userID uuid.UUID) (*SendOrderDetail, error) {
	args := m.Called(ctx, orderID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendOrderDetail), args.Error(1)
}

func (m *mockSendService) UpdateSendOrderStatus(ctx context.Context, req UpdateSendOrderStatusRequest) (*UpdateSendOrderStatusResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UpdateSendOrderStatusResponse), args.Error(1)
}

func (m *mockSendService) AcceptSendOrder(ctx context.Context, req AcceptSendOrderRequest) (*AcceptSendOrderResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AcceptSendOrderResponse), args.Error(1)
}

func (m *mockSendService) UpdateSendOrderStop(ctx context.Context, req UpdateSendOrderStopRequest) (*UpdateSendOrderStopResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UpdateSendOrderStopResponse), args.Error(1)
}

func newSendHandlerCtx(t *testing.T, method, target, body string, params ...gin.Param) (*mockSendService, *gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = params
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	return new(mockSendService), c, w
}

const validCreateSendBody = `{"pickup_lat":-6.2,"pickup_lng":106.8,"pickup_address":"Jl. Pickup",` +
	`"package_weight_kg":2,"package_type":"standard","payment_method":"wallet",` +
	`"stops":[{"recipient_name":"A","recipient_phone":"0811","delivery_address":"Jl. A",` +
	`"delivery_lat":-6.25,"delivery_lng":106.8}]}`

func TestHandler_CreateSendOrder(t *testing.T) {
	svc, c, w := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders", validCreateSendBody)
	c.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c.Set("user_id", fCustID.String())
	svc.On("CreateSendOrder", mock.Anything, mock.MatchedBy(func(r CreateSendOrderRequest) bool {
		return r.UserID == fCustID && r.PaymentMethod == PaymentMethodWallet &&
			r.PackageType == "STANDARD" && r.IdempotencyKey == "idem-1" && len(r.Stops) == 1
	})).Return(&CreateSendOrderResponse{
		ID: fOrderID, Status: sendStatusSearchingDriver, PackageType: "STANDARD",
		PaymentMethod: PaymentMethodWallet, TotalFare: decimal.NewFromInt(48336),
		EscrowAmount: decimal.NewFromInt(48336), StopsCount: 1,
		EstimatedPickupTime: sendEstimatedPickup, FareBreakdownPerStop: []SendStopResponse{},
	}, nil)
	h := NewHandler(svc)
	h.CreateSendOrder(c)
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_CreateSendOrder_Errors(t *testing.T) {
	_, c, w := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders", validCreateSendBody)
	h := NewHandler(new(mockSendService))
	h.CreateSendOrder(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders", `{bad json`)
	c2.Request.Header.Set("X-Idempotency-Key", "idem-1")
	h2 := NewHandler(new(mockSendService))
	h2.CreateSendOrder(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	_, c3, w3 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders", validCreateSendBody)
	c3.Request.Header.Set("X-Idempotency-Key", "idem-1")
	h3 := NewHandler(new(mockSendService))
	h3.CreateSendOrder(c3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)

	svc4, c4, w4 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders", validCreateSendBody)
	c4.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c4.Set("user_id", fCustID.String())
	svc4.On("CreateSendOrder", mock.Anything, mock.Anything).Return(nil, ErrInsufficientBalance)
	h4 := NewHandler(svc4)
	h4.CreateSendOrder(c4)
	assert.Equal(t, http.StatusUnprocessableEntity, w4.Code)

	svc5, c5, w5 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders", validCreateSendBody)
	c5.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c5.Set("user_id", fCustID.String())
	svc5.On("CreateSendOrder", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	h5 := NewHandler(svc5)
	h5.CreateSendOrder(c5)
	assert.Equal(t, http.StatusInternalServerError, w5.Code)
}

func TestHandler_GetSendOrder(t *testing.T) {
	svc, c, w := newSendHandlerCtx(t, http.MethodGet, "/api/v1/send-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("GetSendOrder", mock.Anything, fOrderID, fCustID).Return(&SendOrderDetail{
		Order: fSendOrder(sendStatusSearchingDriver, PaymentMethodWallet, nil),
		Stops: []*SendOrderStop{fSendStop(fOrderID, fStop1ID, 1, stopStatusPending)},
	}, nil)
	h := NewHandler(svc)
	h.GetSendOrder(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newSendHandlerCtx(t, http.MethodGet, "/api/v1/send-orders/notauuid", "", gin.Param{Key: "id", Value: "notauuid"})
	h2 := NewHandler(new(mockSendService))
	h2.GetSendOrder(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	svc3, c3, w3 := newSendHandlerCtx(t, http.MethodGet, "/api/v1/send-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	h3 := NewHandler(svc3)
	h3.GetSendOrder(c3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)

	svc4, c4, w4 := newSendHandlerCtx(t, http.MethodGet, "/api/v1/send-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	c4.Set("user_id", fDriverID.String())
	svc4.On("GetSendOrder", mock.Anything, fOrderID, fDriverID).Return(nil, ErrNotAllowed)
	h4 := NewHandler(svc4)
	h4.GetSendOrder(c4)
	assert.Equal(t, http.StatusForbidden, w4.Code)

	svc5, c5, w5 := newSendHandlerCtx(t, http.MethodGet, "/api/v1/send-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	c5.Set("user_id", fCustID.String())
	svc5.On("GetSendOrder", mock.Anything, fOrderID, fCustID).Return(nil, ErrSendOrderNotFound)
	h5 := NewHandler(svc5)
	h5.GetSendOrder(c5)
	assert.Equal(t, http.StatusNotFound, w5.Code)
}

func TestHandler_UpdateSendOrderStatus(t *testing.T) {
	svc, c, w := newSendHandlerCtx(t, http.MethodPatch, "/api/v1/send-orders/"+fOrderID.String(),
		`{"status":"cancelled","reason":"customer request"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("UpdateSendOrderStatus", mock.Anything, mock.MatchedBy(func(r UpdateSendOrderStatusRequest) bool {
		return r.OrderID == fOrderID && r.UserID == fCustID && r.Status == sendStatusCancelled && r.Reason == "customer request"
	})).Return(&UpdateSendOrderStatusResponse{OrderID: fOrderID, Status: sendStatusCancelled}, nil)
	h := NewHandler(svc)
	h.UpdateSendOrderStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newSendHandlerCtx(t, http.MethodPatch, "/api/v1/send-orders/bad", `{"status":"cancelled"}`, gin.Param{Key: "id", Value: "bad"})
	c2.Set("user_id", fCustID.String())
	h2 := NewHandler(new(mockSendService))
	h2.UpdateSendOrderStatus(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	_, c3, w3 := newSendHandlerCtx(t, http.MethodPatch, "/api/v1/send-orders/"+fOrderID.String(), `{bad json`, gin.Param{Key: "id", Value: fOrderID.String()})
	c3.Set("user_id", fCustID.String())
	h3 := NewHandler(new(mockSendService))
	h3.UpdateSendOrderStatus(c3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)

	svc4, c4, w4 := newSendHandlerCtx(t, http.MethodPatch, "/api/v1/send-orders/"+fOrderID.String(), `{"status":"cancelled"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	h4 := NewHandler(svc4)
	h4.UpdateSendOrderStatus(c4)
	assert.Equal(t, http.StatusUnauthorized, w4.Code)

	svc5, c5, w5 := newSendHandlerCtx(t, http.MethodPatch, "/api/v1/send-orders/"+fOrderID.String(),
		`{"status":"delivered"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c5.Set("user_id", fDriverID.String())
	svc5.On("UpdateSendOrderStatus", mock.Anything, mock.Anything).Return(nil, ErrInvalidTransition)
	h5 := NewHandler(svc5)
	h5.UpdateSendOrderStatus(c5)
	assert.Equal(t, http.StatusConflict, w5.Code)
}

func TestHandler_AcceptSendOrder(t *testing.T) {
	svc, c, w := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders/"+fOrderID.String()+"/accept", "", gin.Param{Key: "id", Value: fOrderID.String()})
	c.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c.Set("user_id", fDriverID.String())
	svc.On("AcceptSendOrder", mock.Anything, mock.MatchedBy(func(r AcceptSendOrderRequest) bool {
		return r.OrderID == fOrderID && r.DriverID == fDriverID && r.IdempotencyKey == "idem-1"
	})).Return(&AcceptSendOrderResponse{ID: fOrderID, Status: sendStatusDriverAssigned, Stops: []AcceptSendOrderStop{}}, nil)
	h := NewHandler(svc)
	h.AcceptSendOrder(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders/"+fOrderID.String()+"/accept", "", gin.Param{Key: "id", Value: fOrderID.String()})
	c2.Set("user_id", fDriverID.String())
	h2 := NewHandler(new(mockSendService))
	h2.AcceptSendOrder(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	_, c3, w3 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders/bad/accept", "", gin.Param{Key: "id", Value: "bad"})
	c3.Request.Header.Set("X-Idempotency-Key", "idem-1")
	h3 := NewHandler(new(mockSendService))
	h3.AcceptSendOrder(c3)
	assert.Equal(t, http.StatusUnprocessableEntity, w3.Code)

	svc4, c4, w4 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders/"+fOrderID.String()+"/accept", "", gin.Param{Key: "id", Value: fOrderID.String()})
	c4.Request.Header.Set("X-Idempotency-Key", "idem-1")
	h4 := NewHandler(svc4)
	h4.AcceptSendOrder(c4)
	assert.Equal(t, http.StatusUnauthorized, w4.Code)

	svc5, c5, w5 := newSendHandlerCtx(t, http.MethodPost, "/api/v1/send-orders/"+fOrderID.String()+"/accept", "", gin.Param{Key: "id", Value: fOrderID.String()})
	c5.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c5.Set("user_id", fDriverID.String())
	svc5.On("AcceptSendOrder", mock.Anything, mock.Anything).Return(nil, ErrDriverBusy)
	h5 := NewHandler(svc5)
	h5.AcceptSendOrder(c5)
	assert.Equal(t, http.StatusConflict, w5.Code)
}

func TestHandler_UpdateSendOrderStop(t *testing.T) {
	svc, c, w := newSendHandlerCtx(t, http.MethodPatch,
		"/api/v1/send-orders/"+fOrderID.String()+"/stops/"+fStop1ID.String(),
		`{"status":"completed","delivery_photo_url":"https://img/1.jpg"}`,
		gin.Param{Key: "id", Value: fOrderID.String()}, gin.Param{Key: "stop_id", Value: fStop1ID.String()})
	c.Set("user_id", fDriverID.String())
	svc.On("UpdateSendOrderStop", mock.Anything, mock.MatchedBy(func(r UpdateSendOrderStopRequest) bool {
		return r.OrderID == fOrderID && r.StopID == fStop1ID && r.UserID == fDriverID &&
			r.Status == "COMPLETED" && r.DeliveryPhotoURL == "https://img/1.jpg"
	})).Return(&UpdateSendOrderStopResponse{
		OrderID: fOrderID, StopID: fStop1ID, StopStatus: stopStatusDelivered, OrderStatus: sendStatusInTransit,
	}, nil)
	h := NewHandler(svc)
	h.UpdateSendOrderStop(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newSendHandlerCtx(t, http.MethodPatch,
		"/api/v1/send-orders/bad/stops/"+fStop1ID.String(), `{"status":"completed"}`,
		gin.Param{Key: "id", Value: "bad"}, gin.Param{Key: "stop_id", Value: fStop1ID.String()})
	c2.Set("user_id", fDriverID.String())
	h2 := NewHandler(new(mockSendService))
	h2.UpdateSendOrderStop(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	_, c3, w3 := newSendHandlerCtx(t, http.MethodPatch,
		"/api/v1/send-orders/"+fOrderID.String()+"/stops/bad", `{"status":"completed"}`,
		gin.Param{Key: "id", Value: fOrderID.String()}, gin.Param{Key: "stop_id", Value: "bad"})
	c3.Set("user_id", fDriverID.String())
	h3 := NewHandler(new(mockSendService))
	h3.UpdateSendOrderStop(c3)
	assert.Equal(t, http.StatusUnprocessableEntity, w3.Code)

	_, c4, w4 := newSendHandlerCtx(t, http.MethodPatch,
		"/api/v1/send-orders/"+fOrderID.String()+"/stops/"+fStop1ID.String(), `{bad json`,
		gin.Param{Key: "id", Value: fOrderID.String()}, gin.Param{Key: "stop_id", Value: fStop1ID.String()})
	c4.Set("user_id", fDriverID.String())
	h4 := NewHandler(new(mockSendService))
	h4.UpdateSendOrderStop(c4)
	assert.Equal(t, http.StatusBadRequest, w4.Code)

	svc5, c5, w5 := newSendHandlerCtx(t, http.MethodPatch,
		"/api/v1/send-orders/"+fOrderID.String()+"/stops/"+fStop1ID.String(), `{"status":"completed"}`,
		gin.Param{Key: "id", Value: fOrderID.String()}, gin.Param{Key: "stop_id", Value: fStop1ID.String()})
	h5 := NewHandler(svc5)
	h5.UpdateSendOrderStop(c5)
	assert.Equal(t, http.StatusUnauthorized, w5.Code)

	svc6, c6, w6 := newSendHandlerCtx(t, http.MethodPatch,
		"/api/v1/send-orders/"+fOrderID.String()+"/stops/"+fStop1ID.String(), `{"status":"wat"}`,
		gin.Param{Key: "id", Value: fOrderID.String()}, gin.Param{Key: "stop_id", Value: fStop1ID.String()})
	c6.Set("user_id", fDriverID.String())
	svc6.On("UpdateSendOrderStop", mock.Anything, mock.Anything).Return(nil, ErrInvalidStopStatus)
	h6 := NewHandler(svc6)
	h6.UpdateSendOrderStop(c6)
	assert.Equal(t, http.StatusBadRequest, w6.Code)
}

func TestHandler_StatusForError(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{ErrSendOrderNotFound, http.StatusNotFound},
		{ErrWalletNotFound, http.StatusNotFound},
		{ErrUserNotFound, http.StatusNotFound},
		{ErrDriverNotFound, http.StatusNotFound},
		{ErrStopNotFound, http.StatusNotFound},
		{ErrNotAllowed, http.StatusForbidden},
		{ErrNotCustomer, http.StatusForbidden},
		{ErrNotDriver, http.StatusForbidden},
		{ErrIdempotencyInProgress, http.StatusConflict},
		{ErrInvalidCachedResponse, http.StatusConflict},
		{ErrInvalidTransition, http.StatusConflict},
		{ErrLockTimeout, http.StatusConflict},
		{ErrDriverBusy, http.StatusConflict},
		{ErrOrderNotSearching, http.StatusConflict},
		{ErrStopsNotDelivered, http.StatusConflict},
		{ErrInvalidStatus, http.StatusBadRequest},
		{ErrInvalidStopStatus, http.StatusBadRequest},
		{ErrCustomerInactive, http.StatusUnprocessableEntity},
		{ErrOverdueDebt, http.StatusUnprocessableEntity},
		{ErrWalletInactive, http.StatusUnprocessableEntity},
		{ErrInsufficientBalance, http.StatusUnprocessableEntity},
		{ErrInvalidPaymentMethod, http.StatusUnprocessableEntity},
		{ErrInvalidPackageType, http.StatusUnprocessableEntity},
		{ErrInvalidWeight, http.StatusUnprocessableEntity},
		{ErrInvalidDeclaredValue, http.StatusUnprocessableEntity},
		{ErrInvalidStopsCount, http.StatusUnprocessableEntity},
		{ErrInvalidRecipient, http.StatusUnprocessableEntity},
		{ErrInvalidCoordinates, http.StatusUnprocessableEntity},
		{ErrIdempotencyKeyRequired, http.StatusUnprocessableEntity},
		{ErrDriverInactive, http.StatusUnprocessableEntity},
		{ErrInsufficientDriverBalance, http.StatusUnprocessableEntity},
		{ErrDriverCapacityExceeded, http.StatusUnprocessableEntity},
		{ErrStopAlreadyDelivered, http.StatusInternalServerError},
		{errors.New("other"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.code, statusForError(tc.err), "err=%v", tc.err)
	}
}

func TestHandler_CodeForError(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{ErrSendOrderNotFound, "SEND_ORDER_NOT_FOUND"},
		{ErrWalletNotFound, "WALLET_NOT_FOUND"},
		{ErrUserNotFound, "USER_NOT_FOUND"},
		{ErrDriverNotFound, "DRIVER_NOT_FOUND"},
		{ErrStopNotFound, "STOP_NOT_FOUND"},
		{ErrNotAllowed, "FORBIDDEN"},
		{ErrNotCustomer, "NOT_CUSTOMER"},
		{ErrNotDriver, "NOT_DRIVER"},
		{ErrCustomerInactive, "CUSTOMER_INACTIVE"},
		{ErrOverdueDebt, "OVERDUE_DEBT"},
		{ErrWalletInactive, "WALLET_INACTIVE"},
		{ErrInsufficientBalance, "INSUFFICIENT_BALANCE"},
		{ErrInvalidPaymentMethod, "INVALID_PAYMENT_METHOD"},
		{ErrInvalidPackageType, "INVALID_PACKAGE_TYPE"},
		{ErrInvalidWeight, "INVALID_WEIGHT"},
		{ErrInvalidDeclaredValue, "INVALID_DECLARED_VALUE"},
		{ErrInvalidStopsCount, "INVALID_STOPS_COUNT"},
		{ErrInvalidRecipient, "INVALID_RECIPIENT"},
		{ErrInvalidCoordinates, "INVALID_COORDINATES"},
		{ErrIdempotencyKeyRequired, "IDEMPOTENCY_KEY_REQUIRED"},
		{ErrIdempotencyInProgress, "IDEMPOTENCY_IN_PROGRESS"},
		{ErrInvalidCachedResponse, "IDEMPOTENCY_INVALID_CACHE"},
		{ErrInvalidStatus, "INVALID_STATUS"},
		{ErrInvalidStopStatus, "INVALID_STOP_STATUS"},
		{ErrInvalidTransition, "INVALID_TRANSITION"},
		{ErrLockTimeout, "LOCK_TIMEOUT"},
		{ErrDriverBusy, "DRIVER_BUSY"},
		{ErrDriverInactive, "DRIVER_INACTIVE"},
		{ErrInsufficientDriverBalance, "INSUFFICIENT_DRIVER_BALANCE"},
		{ErrDriverCapacityExceeded, "DRIVER_CAPACITY_EXCEEDED"},
		{ErrOrderNotSearching, "ORDER_NOT_SEARCHING_DRIVER"},
		{ErrStopsNotDelivered, "STOPS_NOT_DELIVERED"},
		{errors.New("other"), "INTERNAL_SERVER_ERROR"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.code, codeForError(tc.err), "err=%v", tc.err)
	}
}

func TestHandler_UserIDFromContext(t *testing.T) {
	_, c, _ := newSendHandlerCtx(t, http.MethodGet, "/", "")
	_, ok := userIDFromContext(c)
	assert.False(t, ok)

	c2 := &gin.Context{}
	c2.Set("user_id", fCustID.String())
	id, ok := userIDFromContext(c2)
	assert.True(t, ok)
	assert.Equal(t, fCustID, id)

	c3 := &gin.Context{}
	c3.Set("user_id", 12345)
	_, ok = userIDFromContext(c3)
	assert.False(t, ok)

	c4 := &gin.Context{}
	c4.Set("user_id", "")
	_, ok = userIDFromContext(c4)
	assert.False(t, ok)

	c5 := &gin.Context{}
	c5.Set("user_id", "not-a-uuid")
	_, ok = userIDFromContext(c5)
	assert.False(t, ok)
}
