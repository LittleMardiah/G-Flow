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

	"github.com/g-flow/g-flow/internal/auth"
)

// mockSvc adalah stub DriverService untuk unit test handler.
type mockSvc struct {
	res           *AvailableOrdersResult
	err           error
	activeRes     *ActiveOrdersResult
	activeErr     error
	profileRes    *DriverProfile
	profileErr    error
	gotID         uuid.UUID
	called        bool
	profileCalled bool
}

func (m *mockSvc) GetAvailableOrders(_ context.Context, driverID uuid.UUID) (*AvailableOrdersResult, error) {
	m.called = true
	m.gotID = driverID
	if m.err != nil {
		return nil, m.err
	}
	return m.res, nil
}

func (m *mockSvc) GetActiveOrders(_ context.Context, driverID uuid.UUID) (*ActiveOrdersResult, error) {
	m.called = true
	m.gotID = driverID
	if m.activeErr != nil {
		return nil, m.activeErr
	}
	return m.activeRes, nil
}

func (m *mockSvc) GetDriverProfile(_ context.Context, driverID uuid.UUID) (*DriverProfile, error) {
	m.profileCalled = true
	m.gotID = driverID
	if m.profileErr != nil {
		return nil, m.profileErr
	}
	return m.profileRes, nil
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

// setupActiveOrdersRouter memuat route GET /drivers/orders dengan chain yang
// sama seperti produksi: fake AuthMiddleware (set user_id+user_type) dulu,
// lalu RBAC driver, baru handler. withRBAC=false melewati RBAC agar bisa
// menguji perilaku handler saat claim user_id tidak ada (401).
func setupActiveOrdersRouter(svc DriverService, userID, userType string, withRBAC bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(svc)
	r := gin.New()

	handlers := []gin.HandlerFunc{
		func(c *gin.Context) {
			if userID != "" {
				c.Set("user_id", userID)
			}
			if userType != "" {
				c.Set("user_type", userType)
			}
			c.Next()
		},
	}
	if withRBAC {
		handlers = append(handlers, auth.RBACMiddleware("driver"))
	}
	handlers = append(handlers, h.GetDriverOrders)
	r.GET("/api/v1/drivers/orders", handlers...)
	return r
}

func doRequest(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers/available-orders", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doActiveRequest(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers/orders", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setupProfileRouter(svc DriverService, userID, userType string, withRBAC bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(svc)
	r := gin.New()
	handlers := []gin.HandlerFunc{
		func(c *gin.Context) {
			if userID != "" {
				c.Set("user_id", userID)
			}
			if userType != "" {
				c.Set("user_type", userType)
			}
			c.Next()
		},
	}
	if withRBAC {
		handlers = append(handlers, auth.RBACMiddleware("driver"))
	}
	handlers = append(handlers, h.GetDriverMe)
	r.GET("/api/v1/drivers/me", handlers...)
	return r
}

func doProfileRequest(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drivers/me", bytes.NewReader(nil))
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

// ---- TD-077 A2: GET /drivers/orders (active orders list) ----

// TestHandler_GetDriverOrders_Empty: driver tanpa order aktif → 200 + orders=[].
func TestHandler_GetDriverOrders_Empty(t *testing.T) {
	m := &mockSvc{activeRes: &ActiveOrdersResult{Orders: []DriverActiveOrder{}}}
	r := setupActiveOrdersRouter(m, driverID.String(), "driver", true)

	w := doActiveRequest(r)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []json.RawMessage `json:"orders"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Empty(t, body.Data.Orders)
	assert.True(t, m.called)
	assert.Equal(t, driverID, m.gotID)
}

// TestHandler_GetDriverOrders_OneRide: 1 ride aktif → 200 + 1 item type=ride.
func TestHandler_GetDriverOrders_OneRide(t *testing.T) {
	m := &mockSvc{activeRes: &ActiveOrdersResult{Orders: []DriverActiveOrder{{
		Type: OrderTypeRide, OrderID: orderID, Status: "TRIP_STARTED", DriverID: driverID,
		PickupAddress: "Jl. Pickup", DropoffAddress: "Jl. Drop",
		PickupLat: -6.2, PickupLng: 106.8, DropoffLat: -6.3, DropoffLng: 106.9,
		DistanceKm: 3.5, EstimatedFare: decimal.NewFromInt(24000), PaymentMethod: "WALLET",
		CreatedAt: "2026-08-31 10:00:00",
	}}}}
	r := setupActiveOrdersRouter(m, driverID.String(), "driver", true)

	w := doActiveRequest(r)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []DriverActiveOrder `json:"orders"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Orders, 1)
	o := body.Data.Orders[0]
	assert.Equal(t, "ride", o.Type)
	assert.Equal(t, orderID, o.OrderID)
	assert.Equal(t, "TRIP_STARTED", o.Status)
	assert.Equal(t, "Jl. Pickup", o.PickupAddress)
	assert.Equal(t, decimal.NewFromInt(24000), o.EstimatedFare)
}

// TestHandler_GetDriverOrders_Mixed: 1 food + 1 send aktf → 200 + 2 item.
func TestHandler_GetDriverOrders_Mixed(t *testing.T) {
	foodID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	sendID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	m := &mockSvc{activeRes: &ActiveOrdersResult{Orders: []DriverActiveOrder{
		{
			Type: OrderTypeFood, OrderID: foodID, Status: "PICKED_UP", DriverID: driverID,
			MerchantName: "Warung", DeliveryAddress: "Jl. Tujuan",
			TotalAmount: decimal.NewFromInt(45000), DeliveryFee: decimal.NewFromInt(20000),
			PaymentMethod: "CASH", CreatedAt: "2026-08-31 10:00:00",
		},
		{
			Type: OrderTypeSend, OrderID: sendID, Status: "IN_TRANSIT", DriverID: driverID,
			PickupAddress: "Jl. Kirim", TotalFare: decimal.NewFromInt(30000),
			FirstStopAddress: "Jl. Stop 1", PaymentMethod: "WALLET", CreatedAt: "2026-08-31 10:00:00",
		},
	}}}
	r := setupActiveOrdersRouter(m, driverID.String(), "driver", true)

	w := doActiveRequest(r)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []DriverActiveOrder `json:"orders"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Orders, 2)
	assert.Equal(t, "food", body.Data.Orders[0].Type)
	assert.Equal(t, "send", body.Data.Orders[1].Type)
	assert.Equal(t, foodID, body.Data.Orders[0].OrderID)
	assert.Equal(t, sendID, body.Data.Orders[1].OrderID)
	assert.Equal(t, "Jl. Stop 1", body.Data.Orders[1].FirstStopAddress)
}

// TestHandler_GetDriverOrders_Forbidden: user_type customer → 403 (RBAC).
func TestHandler_GetDriverOrders_Forbidden(t *testing.T) {
	m := &mockSvc{}
	// user_type=customer → RBACMiddleware("driver") blokir sebelum handler.
	r := setupActiveOrdersRouter(m, "99999999-9999-9999-9999-999999999999", "customer", true)

	w := doActiveRequest(r)
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, m.called)
}

// TestHandler_GetDriverOrders_Unauthorized: JWT claim user_id hilang → 401.
func TestHandler_GetDriverOrders_Unauthorized(t *testing.T) {
	m := &mockSvc{}
	r := setupActiveOrdersRouter(m, "", "", false) // tanpa RBAC agar handler terpanggil

	w := doActiveRequest(r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, m.called)
}

// TestHandler_GetDriverOrders_CompletedNotIncluded: service HANYA mengembalikan
// order aktif (SQL memfilter status COMPLETED/CANCELLED/SETTLED di repo);
// handler meneruskan list tersebut → 200 & status non-aktif tidak muncul.
func TestHandler_GetDriverOrders_CompletedNotIncluded(t *testing.T) {
	m := &mockSvc{activeRes: &ActiveOrdersResult{Orders: []DriverActiveOrder{{
		Type: OrderTypeRide, OrderID: orderID, Status: "DRIVER_ASSIGNED",
		DriverID: driverID, PickupAddress: "Jl. Pickup", EstimatedFare: decimal.NewFromInt(20000),
		PaymentMethod: "WALLET", CreatedAt: "2026-08-31 10:00:00",
	}}}}
	r := setupActiveOrdersRouter(m, driverID.String(), "driver", true)

	w := doActiveRequest(r)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []DriverActiveOrder `json:"orders"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Orders, 1)
	assert.NotEqual(t, "COMPLETED", body.Data.Orders[0].Status)
	assert.Equal(t, "DRIVER_ASSIGNED", body.Data.Orders[0].Status)
}

// TestHandler_GetDriverOrders_ServiceError: error repo/service → 500.
func TestHandler_GetDriverOrders_ServiceError(t *testing.T) {
	m := &mockSvc{activeErr: errors.New("boom")}
	r := setupActiveOrdersRouter(m, driverID.String(), "driver", true)

	w := doActiveRequest(r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_GetDriverMe_Success(t *testing.T) {
	m := &mockSvc{profileRes: &DriverProfile{
		DriverID:          driverID,
		Name:              "Budi Santoso",
		Email:             "budi@example.com",
		Phone:             ptrString("081234567890"),
		VehicleType:       ptrString("motor"),
		VehiclePlate:      ptrString("B 1234 ABC"),
		LicenseNumber:     ptrString("SIM-123"),
		LicenseExpiry:     ptrString("2027-12-31"),
		Status:            "ACTIVE",
		RatingAvg:         4.75,
		TotalRides:        12,
		BankName:          ptrString("Bank Central Asia"),
		BankAccountNumber: ptrString("9876543210"),
	}}
	r := setupProfileRouter(m, driverID.String(), "driver", true)

	w := doProfileRequest(r)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool                  `json:"success"`
		Data    DriverProfileResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, driverID, body.Data.DriverID)
	assert.Equal(t, "Budi Santoso", body.Data.Name)
	assert.Equal(t, "****3210", *body.Data.BankAccountMasked)
	assert.Equal(t, 4.75, body.Data.RatingAvg)
	assert.Equal(t, int64(12), body.Data.TotalRides)
	assert.NotContains(t, w.Body.String(), "9876543210")
	assert.True(t, m.profileCalled)
	assert.Equal(t, driverID, m.gotID)
}

func TestHandler_GetDriverMe_Unauthorized(t *testing.T) {
	m := &mockSvc{}
	r := setupProfileRouter(m, "", "", false)

	w := doProfileRequest(r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, m.profileCalled)
}

func TestHandler_GetDriverMe_Forbidden(t *testing.T) {
	m := &mockSvc{}
	r := setupProfileRouter(m, custID.String(), "customer", true)

	w := doProfileRequest(r)
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, m.profileCalled)
}

func TestHandler_GetDriverMe_NotFound(t *testing.T) {
	m := &mockSvc{profileErr: ErrDriverNotFound}
	r := setupProfileRouter(m, driverID.String(), "driver", true)

	w := doProfileRequest(r)
	require.Equal(t, http.StatusNotFound, w.Code)

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "DRIVER_NOT_FOUND", body.Error.Code)
}
