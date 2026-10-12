package location

// Test handler SetOnlineStatus (TD-186): PATCH /drivers/me/status.
// Sukses -> 200 {is_online}; body invalid -> 400; driver tidak ditemukan ->
// 404 DRIVER_NOT_FOUND; tanpa identitas -> 401.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockLocationService struct {
	mock.Mock
}

func (m *mockLocationService) UpdateLocation(ctx context.Context, driverID uuid.UUID, lat, lng float64) error {
	args := m.Called(ctx, driverID, lat, lng)
	return args.Error(0)
}

func (m *mockLocationService) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, error) {
	args := m.Called(ctx, lat, lng, radiusKm)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]NearbyDriver), args.Error(1)
}

func (m *mockLocationService) SetOnlineStatus(ctx context.Context, driverID uuid.UUID, isOnline bool) error {
	args := m.Called(ctx, driverID, isOnline)
	return args.Error(0)
}

func newLocationHandlerCtx(t *testing.T, method, target, body string) (*mockLocationService, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	return new(mockLocationService), c, w
}

func TestHandler_SetOnlineStatus_Success(t *testing.T) {
	driverID := uuid.New()
	svc, c, w := newLocationHandlerCtx(t, http.MethodPatch, "/api/v1/drivers/me/status", `{"is_online":true}`)
	c.Set("user_id", driverID.String())

	svc.On("SetOnlineStatus", mock.Anything, driverID, true).Return(nil)

	h := NewHandler(svc)
	h.SetOnlineStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"is_online":true`)
	svc.AssertExpectations(t)
}

func TestHandler_SetOnlineStatus_InvalidBody(t *testing.T) {
	driverID := uuid.New()
	svc, c, w := newLocationHandlerCtx(t, http.MethodPatch, "/api/v1/drivers/me/status", `{bad json`)
	c.Set("user_id", driverID.String())

	h := NewHandler(svc)
	h.SetOnlineStatus(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
}

func TestHandler_SetOnlineStatus_MissingIdentity(t *testing.T) {
	svc, c, w := newLocationHandlerCtx(t, http.MethodPatch, "/api/v1/drivers/me/status", `{"is_online":false}`)

	h := NewHandler(svc)
	h.SetOnlineStatus(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestHandler_SetOnlineStatus_NotFound(t *testing.T) {
	driverID := uuid.New()
	svc, c, w := newLocationHandlerCtx(t, http.MethodPatch, "/api/v1/drivers/me/status", `{"is_online":false}`)
	c.Set("user_id", driverID.String())

	svc.On("SetOnlineStatus", mock.Anything, driverID, false).Return(ErrDriverNotFound)

	h := NewHandler(svc)
	h.SetOnlineStatus(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "DRIVER_NOT_FOUND")
	svc.AssertExpectations(t)
}
