package ride

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/g-flow/g-flow/internal/auth"
)

type mockRideService struct {
	mock.Mock
}

func (m *mockRideService) BookRide(ctx context.Context, req BookRideRequest) (*BookRideResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BookRideResponse), args.Error(1)
}

func (m *mockRideService) AcceptOrder(ctx context.Context, orderID, driverID uuid.UUID) (*AcceptOrderResponse, error) {
	args := m.Called(ctx, orderID, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AcceptOrderResponse), args.Error(1)
}

func (m *mockRideService) UpdateRideStatus(ctx context.Context, req UpdateRideStatusRequest) (*UpdateRideStatusResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UpdateRideStatusResponse), args.Error(1)
}

func (m *mockRideService) GetOrder(ctx context.Context, orderID uuid.UUID) (*RideOrder, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RideOrder), args.Error(1)
}

func (m *mockRideService) GetRidesHistory(ctx context.Context, customerID uuid.UUID, page, pageSize int, status string) ([]RideOrder, int, error) {
	args := m.Called(ctx, customerID, page, pageSize, status)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]RideOrder), args.Int(1), args.Error(2)
}

func setupGin() (*gin.Engine, *gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return gin.New(), c, w
}

func TestHandler_BookRide_Success(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("BookRide", mock.Anything, mock.Anything).Return(&BookRideResponse{
		OrderID:       svcOrderID,
		Status:        statusSearchingDriver,
		PickupLat:     -6.2,
		PickupLng:     106.8,
		DropoffLat:    -6.26,
		DropoffLng:    106.8,
		DistanceKm:    decimal.NewFromFloat(6.7),
		EstimatedFare: decimal.NewFromInt(36800),
		PaymentMethod: PaymentMethodWallet,
	}, nil)

	h := NewHandler(svc)
	h.BookRide(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var out struct {
		Success bool             `json:"success"`
		Data    bookRideResponse `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	svc.AssertExpectations(t)
}

func TestHandler_BookRide_BindError(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`not-json`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := NewHandler(nil)
	h.BookRide(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_BookRide_InvalidCoordinates(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-200,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := NewHandler(nil)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_BookRide_SamePickupDropoff(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.2,"dropoff_lng":106.8,"payment_method":"WALLET"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := NewHandler(nil)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_BookRide_InvalidPaymentMethod(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"QRIS"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := NewHandler(nil)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_BookRide_Unauthorized(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	// Tidak ada user_id di context.

	h := NewHandler(nil)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_BookRide_ServiceError(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("BookRide", mock.Anything, mock.Anything).Return(nil, ErrOverdueDebt)

	h := NewHandler(svc)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	svc.AssertExpectations(t)
}

// BookRide memakai voucher_code: service menerima VoucherCode ter-set,
// VoucherID nil; respons menampilkan discount_amount + voucher_code.
func TestHandler_BookRide_VoucherSuccess(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET","voucher_code":"RIDE20"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Idempotency-Key", "idem-v1")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	voucherCode := "RIDE20"
	svc.On("BookRide", mock.Anything, mock.MatchedBy(func(req BookRideRequest) bool {
		return req.VoucherCode != nil && *req.VoucherCode == voucherCode && req.VoucherID == nil
	})).Return(&BookRideResponse{
		OrderID:        svcOrderID,
		Status:         statusSearchingDriver,
		DistanceKm:     decimal.NewFromFloat(6.7),
		EstimatedFare:  decimal.NewFromInt(36800),
		PaymentMethod:  PaymentMethodWallet,
		EscrowAmount:   decimal.NewFromInt(29440),
		DiscountAmount: decimal.NewFromInt(7360),
		VoucherCode:    &voucherCode,
	}, nil)

	h := NewHandler(svc)
	h.BookRide(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var out struct {
		Success bool             `json:"success"`
		Data    bookRideResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	assert.True(t, out.Data.DiscountAmount.Equal(decimal.NewFromInt(7360)))
	require.NotNil(t, out.Data.VoucherCode)
	assert.Equal(t, voucherCode, *out.Data.VoucherCode)
	svc.AssertExpectations(t)
}

// BookRide memakai voucher_id: service menerima VoucherID ter-set.
func TestHandler_BookRide_VoucherByID(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET","voucher_id":"88888888-8888-8888-8888-888888888888"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("BookRide", mock.Anything, mock.MatchedBy(func(req BookRideRequest) bool {
		return req.VoucherID != nil && *req.VoucherID == svcVoucherID && req.VoucherCode == nil
	})).Return(&BookRideResponse{
		OrderID:        svcOrderID,
		Status:         statusSearchingDriver,
		DistanceKm:     decimal.NewFromFloat(6.7),
		EstimatedFare:  decimal.NewFromInt(36800),
		PaymentMethod:  PaymentMethodWallet,
		DiscountAmount: decimal.NewFromInt(10000),
	}, nil)

	h := NewHandler(svc)
	h.BookRide(c)
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

// voucher_id bukan UUID valid → 422 INVALID_VOUCHER_ID (tanpa panggil service).
func TestHandler_BookRide_InvalidVoucherID(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET","voucher_id":"not-a-uuid"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	h := NewHandler(nil)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_VOUCHER_ID")
}

// Voucher tidak ditemukan → 404 VOUCHER_NOT_FOUND.
func TestHandler_BookRide_VoucherNotFound(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET","voucher_code":"GAK ADA"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("BookRide", mock.Anything, mock.Anything).Return(nil, ErrVoucherNotFound)

	h := NewHandler(svc)
	h.BookRide(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "VOUCHER_NOT_FOUND")
	svc.AssertExpectations(t)
}

// Voucher expired → 422 VOUCHER_EXPIRED.
func TestHandler_BookRide_VoucherExpired(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET","voucher_code":"RIDE20"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("BookRide", mock.Anything, mock.Anything).Return(nil, ErrVoucherExpired)

	h := NewHandler(svc)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "VOUCHER_EXPIRED")
	svc.AssertExpectations(t)
}

// Voucher di bawah min_order / limit pemakaian → 422 map API.
func TestHandler_BookRide_VoucherMinOrder(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/book",
		bytes.NewBufferString(`{"pickup_lat":-6.2,"pickup_lng":106.8,"dropoff_lat":-6.26,"dropoff_lng":106.8,"payment_method":"WALLET","voucher_code":"RIDE20"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("BookRide", mock.Anything, mock.Anything).Return(nil, ErrVoucherMinOrder)

	h := NewHandler(svc)
	h.BookRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "VOUCHER_MIN_ORDER_NOT_MET")
	svc.AssertExpectations(t)
}

func TestHandler_AcceptOrder_Success(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/"+svcOrderID.String()+"/accept", nil)
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("AcceptOrder", mock.Anything, svcOrderID, svcDriverID).Return(
		&AcceptOrderResponse{OrderID: svcOrderID, Status: statusDriverAssigned, DriverID: svcDriverID}, nil)

	h := NewHandler(svc)
	h.AcceptOrder(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_AcceptOrder_InvalidOrderID(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/notauuid/accept", nil)
	c.Set("user_id", svcDriverID.String())

	h := NewHandler(nil)
	h.AcceptOrder(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_AcceptOrder_Unauthorized(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/"+svcOrderID.String()+"/accept", nil)

	h := NewHandler(nil)
	h.AcceptOrder(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_AcceptOrder_ServiceError(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/rides/"+svcOrderID.String()+"/accept", nil)
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("AcceptOrder", mock.Anything, svcOrderID, svcDriverID).Return(nil, ErrDriverBusy)

	h := NewHandler(svc)
	h.AcceptOrder(c)
	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateStatus_Success(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`{"status":"DRIVER_ARRIVED"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("UpdateRideStatus", mock.Anything, mock.Anything).Return(
		&UpdateRideStatusResponse{OrderID: svcOrderID, Status: statusDriverArrived}, nil)

	h := NewHandler(svc)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateStatus_WithActualFare(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`{"status":"COMPLETED","actual_fare":60000}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("UpdateRideStatus", mock.Anything, mock.MatchedBy(func(req UpdateRideStatusRequest) bool {
		return req.ActualFare != nil && req.ActualFare.Equal(decimal.NewFromInt(60000))
	})).Return(&UpdateRideStatusResponse{OrderID: svcOrderID, Status: statusSettled}, nil)

	h := NewHandler(svc)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateStatus_WithoutActualFare(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`{"status":"COMPLETED"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("UpdateRideStatus", mock.Anything, mock.MatchedBy(func(req UpdateRideStatusRequest) bool {
		return req.Status == "COMPLETED" && req.ActualFare == nil
	})).Return(&UpdateRideStatusResponse{OrderID: svcOrderID, Status: statusSettled}, nil)

	h := NewHandler(svc)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateStatus_ActualFareZeroIgnored(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`{"status":"COMPLETED","actual_fare":0}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("UpdateRideStatus", mock.Anything, mock.MatchedBy(func(req UpdateRideStatusRequest) bool {
		return req.ActualFare == nil
	})).Return(&UpdateRideStatusResponse{OrderID: svcOrderID, Status: statusSettled}, nil)

	h := NewHandler(svc)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateStatus_InvalidOrderID(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/xyz/status",
		bytes.NewBufferString(`{"status":"CANCELLED"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	h := NewHandler(nil)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_UpdateStatus_BindError(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`not-json`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcCustomerID.String())

	h := NewHandler(nil)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UpdateStatus_Unauthorized(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`{"status":"CANCELLED"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := NewHandler(nil)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_UpdateStatus_ServiceError(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/rides/"+svcOrderID.String()+"/status",
		bytes.NewBufferString(`{"status":"COMPLETED"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", svcDriverID.String())

	svc := new(mockRideService)
	svc.On("UpdateRideStatus", mock.Anything, mock.Anything).Return(nil, ErrInvalidTransition)

	h := NewHandler(svc)
	h.UpdateStatus(c)
	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetRide_CustomerOwner_Success(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides/"+svcOrderID.String(), nil)
	c.Set("user_id", svcCustomerID.String())

	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID)
	svc := new(mockRideService)
	svc.On("GetOrder", mock.Anything, svcOrderID).Return(order, nil)

	h := NewHandler(svc)
	h.GetRide(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Success bool               `json:"success"`
		Data    rideDetailResponse `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	assert.Equal(t, svcOrderID, out.Data.OrderID)
	assert.Equal(t, svcCustomerID, out.Data.CustomerID)
	assert.NotNil(t, out.Data.DriverID)
	assert.Equal(t, statusDriverAssigned, out.Data.Status)
	assert.Equal(t, PaymentMethodWallet, out.Data.PaymentMethod)
	svc.AssertExpectations(t)
}

func TestHandler_GetRide_DriverAssigned_Success(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides/"+svcOrderID.String(), nil)
	c.Set("user_id", svcDriverID.String())

	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID)
	svc := new(mockRideService)
	svc.On("GetOrder", mock.Anything, svcOrderID).Return(order, nil)

	h := NewHandler(svc)
	h.GetRide(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Success bool               `json:"success"`
		Data    rideDetailResponse `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	assert.Equal(t, svcOrderID, out.Data.OrderID)
	assert.Equal(t, svcCustomerID, out.Data.CustomerID)
	assert.Equal(t, statusDriverAssigned, out.Data.Status)
	svc.AssertExpectations(t)
}

func TestHandler_GetRide_Forbidden(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides/"+svcOrderID.String(), nil)
	// User lain: bukan customer pemesan, bukan driver tertunjuk.
	c.Set("user_id", "99999999-9999-9999-9999-999999999999")

	order := svcOrder(statusDriverAssigned, PaymentMethodWallet, &svcDriverID)
	svc := new(mockRideService)
	svc.On("GetOrder", mock.Anything, svcOrderID).Return(order, nil)

	h := NewHandler(svc)
	h.GetRide(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetRide_NotFound(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides/"+svcOrderID.String(), nil)
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("GetOrder", mock.Anything, svcOrderID).Return(nil, ErrOrderNotFound)

	h := NewHandler(svc)
	h.GetRide(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetRide_InvalidOrderID(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides/notauuid", nil)
	c.Set("user_id", svcCustomerID.String())

	h := NewHandler(nil)
	h.GetRide(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_GetRide_Unauthorized(t *testing.T) {
	_, c, w := setupGin()
	c.Params = gin.Params{gin.Param{Key: "order_id", Value: svcOrderID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides/"+svcOrderID.String(), nil)

	h := NewHandler(nil)
	h.GetRide(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// svcHistoryOrder membangun RideOrder dengan field yang diekspos di list
// history (GET /rides).
func svcHistoryOrder(id uuid.UUID, status, payment string) *RideOrder {
	completed := time.Now().Add(-time.Minute)
	return &RideOrder{
		ID:             id,
		CustomerID:     svcCustomerID,
		PickupAddress:  "Jl. Merdeka 1",
		DropoffAddress: "Jl. Sudirman 88",
		DistanceKm:     decimal.NewFromFloat(6.7),
		EstimatedFare:  decimal.NewFromInt(36800),
		PaymentMethod:  payment,
		Status:         status,
		CreatedAt:      time.Now().Add(-time.Hour),
		CompletedAt:    &completed,
	}
}

func TestHandler_GetRideHistory_Empty(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides", nil)
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("GetRidesHistory", mock.Anything, svcCustomerID, 1, 20, "").Return([]RideOrder{}, 0, nil)

	h := NewHandler(svc)
	h.GetRideHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []rideHistoryResponse `json:"orders"`
		} `json:"data"`
		Meta struct {
			Page       int `json:"page"`
			PageSize   int `json:"page_size"`
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"meta"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	assert.Empty(t, out.Data.Orders)
	assert.Equal(t, 0, out.Meta.Total)
	assert.Equal(t, 0, out.Meta.TotalPages)
	svc.AssertExpectations(t)
}

func TestHandler_GetRideHistory_SingleOrder(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides", nil)
	c.Set("user_id", svcCustomerID.String())

	order := svcHistoryOrder(svcOrderID, statusSettled, PaymentMethodWallet)
	svc := new(mockRideService)
	svc.On("GetRidesHistory", mock.Anything, svcCustomerID, 1, 20, "").Return([]RideOrder{*order}, 1, nil)

	h := NewHandler(svc)
	h.GetRideHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []rideHistoryResponse `json:"orders"`
		} `json:"data"`
		Meta struct {
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"meta"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	assert.Len(t, out.Data.Orders, 1)
	assert.Equal(t, svcOrderID, out.Data.Orders[0].OrderID)
	assert.Equal(t, statusSettled, out.Data.Orders[0].Status)
	assert.Equal(t, "Jl. Merdeka 1", out.Data.Orders[0].PickupAddress)
	assert.Equal(t, "Jl. Sudirman 88", out.Data.Orders[0].DropoffAddress)
	assert.Equal(t, PaymentMethodWallet, out.Data.Orders[0].PaymentMethod)
	assert.NotNil(t, out.Data.Orders[0].CompletedAt)
	assert.Equal(t, 1, out.Meta.Total)
	assert.Equal(t, 1, out.Meta.TotalPages)
	svc.AssertExpectations(t)
}

func TestHandler_GetRideHistory_Pagination(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides?page=2&page_size=10", nil)
	c.Set("user_id", svcCustomerID.String())

	orders := []RideOrder{
		*svcHistoryOrder(svcOrderID, statusSettled, PaymentMethodWallet),
		*svcHistoryOrder(uuid.MustParse("77777777-7777-7777-7777-777777777777"), statusCompleted, PaymentMethodCash),
	}
	svc := new(mockRideService)
	svc.On("GetRidesHistory", mock.Anything, svcCustomerID, 2, 10, "").Return(orders, 47, nil)

	h := NewHandler(svc)
	h.GetRideHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []rideHistoryResponse `json:"orders"`
		} `json:"data"`
		Meta struct {
			Page       int `json:"page"`
			PageSize   int `json:"page_size"`
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"meta"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.True(t, out.Success)
	assert.Len(t, out.Data.Orders, 2)
	assert.Equal(t, 2, out.Meta.Page)
	assert.Equal(t, 10, out.Meta.PageSize)
	assert.Equal(t, 47, out.Meta.Total)
	assert.Equal(t, 5, out.Meta.TotalPages)
	svc.AssertExpectations(t)
}

func TestHandler_GetRideHistory_FilterStatus(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides?status=completed", nil)
	c.Set("user_id", svcCustomerID.String())

	order := svcHistoryOrder(svcOrderID, statusCompleted, PaymentMethodWallet)
	svc := new(mockRideService)
	svc.On("GetRidesHistory", mock.Anything, svcCustomerID, 1, 20, "COMPLETED").Return([]RideOrder{*order}, 1, nil)

	h := NewHandler(svc)
	h.GetRideHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Data struct {
			Orders []rideHistoryResponse `json:"orders"`
		} `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Len(t, out.Data.Orders, 1)
	assert.Equal(t, statusCompleted, out.Data.Orders[0].Status)
	svc.AssertExpectations(t)
}

func TestHandler_GetRideHistory_NonCustomer_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/rides",
		func(c *gin.Context) {
			c.Set("user_id", svcDriverID.String())
			c.Set("user_type", "driver")
			c.Next()
		},
		auth.RBACMiddleware("customer"),
		func(c *gin.Context) { NewHandler(new(mockRideService)).GetRideHistory(c) },
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rides", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandler_GetRideHistory_Unauthorized(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides", nil)

	h := NewHandler(nil)
	h.GetRideHistory(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_GetRideHistory_InvalidPage(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides?page=abc", nil)
	c.Set("user_id", svcCustomerID.String())

	h := NewHandler(nil)
	h.GetRideHistory(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetRideHistory_InvalidPageSize(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides?page_size=abc", nil)
	c.Set("user_id", svcCustomerID.String())

	h := NewHandler(nil)
	h.GetRideHistory(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetRideHistory_ServiceError(t *testing.T) {
	_, c, w := setupGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rides?page=0", nil)
	c.Set("user_id", svcCustomerID.String())

	svc := new(mockRideService)
	svc.On("GetRidesHistory", mock.Anything, svcCustomerID, 0, 20, "").Return(nil, 0, ErrInvalidPagination)

	h := NewHandler(svc)
	h.GetRideHistory(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

// ---- helper functions coverage ----

func TestStatusForError(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, statusForError(ErrCustomerNotFound))
	assert.Equal(t, http.StatusNotFound, statusForError(ErrWalletNotFound))
	assert.Equal(t, http.StatusNotFound, statusForError(ErrOrderNotFound))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrDriverNotFound))
	assert.Equal(t, http.StatusForbidden, statusForError(ErrNotCustomer))
	assert.Equal(t, http.StatusForbidden, statusForError(ErrNotDriver))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrInvalidCoordinates))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrSamePickupDropoff))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrInvalidPaymentMethod))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrCustomerInactive))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrOverdueDebt))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrWalletInactive))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrInsufficientBalance))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrDriverInactive))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrInsufficientDriverBalance))
	assert.Equal(t, http.StatusUnprocessableEntity, statusForError(ErrIdempotencyKeyRequired))
	assert.Equal(t, http.StatusConflict, statusForError(ErrIdempotencyInProgress))
	assert.Equal(t, http.StatusConflict, statusForError(ErrDriverBusy))
	assert.Equal(t, http.StatusConflict, statusForError(ErrOrderNotSearching))
	assert.Equal(t, http.StatusConflict, statusForError(ErrLockTimeout))
	assert.Equal(t, http.StatusConflict, statusForError(ErrInvalidTransition))
	assert.Equal(t, http.StatusForbidden, statusForError(ErrNotAllowed))
	assert.Equal(t, http.StatusBadRequest, statusForError(ErrInvalidStatus))
	assert.Equal(t, http.StatusBadRequest, statusForError(ErrInvalidPagination))
	assert.Equal(t, http.StatusInternalServerError, statusForError(errors.New("other")))
}

func TestCodeForError(t *testing.T) {
	assert.Equal(t, "CUSTOMER_NOT_FOUND", codeForError(ErrCustomerNotFound))
	assert.Equal(t, "WALLET_NOT_FOUND", codeForError(ErrWalletNotFound))
	assert.Equal(t, "ORDER_NOT_FOUND", codeForError(ErrOrderNotFound))
	assert.Equal(t, "DRIVER_NOT_FOUND", codeForError(ErrDriverNotFound))
	assert.Equal(t, "NOT_CUSTOMER", codeForError(ErrNotCustomer))
	assert.Equal(t, "NOT_DRIVER", codeForError(ErrNotDriver))
	assert.Equal(t, "INVALID_COORDINATES", codeForError(ErrInvalidCoordinates))
	assert.Equal(t, "SAME_PICKUP_DROPOFF", codeForError(ErrSamePickupDropoff))
	assert.Equal(t, "INVALID_PAYMENT_METHOD", codeForError(ErrInvalidPaymentMethod))
	assert.Equal(t, "CUSTOMER_INACTIVE", codeForError(ErrCustomerInactive))
	assert.Equal(t, "OVERDUE_DEBT", codeForError(ErrOverdueDebt))
	assert.Equal(t, "WALLET_INACTIVE", codeForError(ErrWalletInactive))
	assert.Equal(t, "INSUFFICIENT_BALANCE", codeForError(ErrInsufficientBalance))
	assert.Equal(t, "DRIVER_INACTIVE", codeForError(ErrDriverInactive))
	assert.Equal(t, "DRIVER_BUSY", codeForError(ErrDriverBusy))
	assert.Equal(t, "ORDER_NOT_SEARCHING", codeForError(ErrOrderNotSearching))
	assert.Equal(t, "INSUFFICIENT_DRIVER_BALANCE", codeForError(ErrInsufficientDriverBalance))
	assert.Equal(t, "LOCK_TIMEOUT", codeForError(ErrLockTimeout))
	assert.Equal(t, "INVALID_REQUEST", codeForError(ErrInvalidStatus))
	assert.Equal(t, "INVALID_REQUEST", codeForError(ErrInvalidPagination))
	assert.Equal(t, "INVALID_STATUS_TRANSITION", codeForError(ErrInvalidTransition))
	assert.Equal(t, "FORBIDDEN", codeForError(ErrNotAllowed))
	assert.Equal(t, "INVALID_IDEMPOTENCY_KEY", codeForError(ErrIdempotencyKeyRequired))
	assert.Equal(t, "IDEMPOTENCY_IN_PROGRESS", codeForError(ErrIdempotencyInProgress))
	assert.Equal(t, "INTERNAL_ERROR", codeForError(ErrInvalidCachedResponse))
	assert.Equal(t, "INTERNAL_SERVER_ERROR", codeForError(errors.New("other")))
}

func TestUserIDFromContext(t *testing.T) {
	_, c, _ := setupGin()
	_, ok := userIDFromContext(c)
	assert.False(t, ok)

	c.Set("user_id", "not-a-uuid")
	_, ok = userIDFromContext(c)
	assert.False(t, ok)

	c.Set("user_id", 123)
	_, ok = userIDFromContext(c)
	assert.False(t, ok)

	c.Set("user_id", svcCustomerID.String())
	id, ok := userIDFromContext(c)
	assert.True(t, ok)
	assert.Equal(t, svcCustomerID, id)
}
