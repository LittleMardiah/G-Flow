package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSvc adalah stub DriverService untuk unit test handler.
type mockSvc struct {
	res    *AvailableOrdersResult
	err    error
	gotID  uuid.UUID
	called bool
}

func (m *mockSvc) GetAvailableOrders(_ context.Context, driverID uuid.UUID) (*AvailableOrdersResult, error) {
	m.called = true
	m.gotID = driverID
	if m.err != nil {
		return nil, m.err
	}
	return m.res, nil
}

func setupHandler(svc DriverService, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/api/v1/drivers/available-orders", func(c *gin.Context) {
		// Simulasikan claim JWT yang diset AuthMiddleware.
		if userID != "" {
			c.Set("user_id", userID)
		}
		h.GetAvailableOrders(c)
	})
	return r
}

func doRequest(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers/available-orders", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_GetAvailableOrders_Success(t *testing.T) {
	m := &mockSvc{res: &AvailableOrdersResult{
		CapacityAvailable: true,
		ActiveOrders:      1,
		MaxActiveOrders:   3,
		Orders: []AvailableOrder{{
			ID: orderID, Type: OrderTypeRide, Status: "SEARCHING_DRIVER",
			PickupAddress: "Jl. Pickup", DistanceKm: 1.2,
			Earning: decimal.NewFromInt(16000), Fare: decimal.NewFromInt(20000),
			PaymentMethod: "WALLET", CreatedAt: "2026-08-31 10:00:00",
		}},
	}}
	r := setupHandler(m, driverID.String())

	w := doRequest(r)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Orders            []AvailableOrder `json:"orders"`
			CapacityAvailable bool             `json:"capacity_available"`
			ActiveOrders      int              `json:"active_orders"`
			MaxActiveOrders   int              `json:"max_active_orders"`
			RadiusKm          float64          `json:"radius_km"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.True(t, body.Data.CapacityAvailable)
	assert.Equal(t, 1, body.Data.ActiveOrders)
	require.Len(t, body.Data.Orders, 1)
	assert.Equal(t, orderID, body.Data.Orders[0].ID)
	assert.Equal(t, OrderTypeRide, body.Data.Orders[0].Type)
}

func TestHandler_GetAvailableOrders_Unauthorized(t *testing.T) {
	m := &mockSvc{res: &AvailableOrdersResult{Orders: []AvailableOrder{}}}
	r := setupHandler(m, "") // tanpa user_id

	w := doRequest(r) // tanpa user_id
	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, m.called)
}

func TestHandler_GetAvailableOrders_LocationUnavailable(t *testing.T) {
	m := &mockSvc{err: ErrDriverLocationUnavailable}
	r := setupHandler(m, driverID.String())

	w := doRequest(r)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "DRIVER_LOCATION_UNAVAILABLE", body.Error.Code)
}

func TestHandler_GetAvailableOrders_InternalError(t *testing.T) {
	m := &mockSvc{err: errors.New("boom")}
	r := setupHandler(m, driverID.String())

	w := doRequest(r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_GetAvailableOrders_InvalidCoords(t *testing.T) {
	m := &mockSvc{err: ErrInvalidCoordinates}
	r := setupHandler(m, driverID.String())

	w := doRequest(r)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
