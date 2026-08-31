// Package driver — HTTP Handler (Gin). TD-009: Available Orders endpoint for
// driver.
//
// Handler hanya memetakan request HTTP menjadi panggilan Service dan men-map
// error ke status code + error code (pola sama dgn internal/location &
// internal/food). Tidak mengandung business logic.
package driver

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DriverService adalah kontrak service yang dibutuhkan Handler. Dipenuhi oleh
// *Service; dijadikan interface agar mudah di-mock pada test.
type DriverService interface {
	GetAvailableOrders(ctx context.Context, driverID uuid.UUID) (*AvailableOrdersResult, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc DriverService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc DriverService) *Handler {
	return &Handler{svc: svc}
}

// GetAvailableOrders GET /api/v1/drivers/available-orders
// Auth: driver (JWT + RBAC). Mengambil driver_id dari JWT claim `user_id`,
// lalu mengembalikan daftar order tersedia (ride/food/send) dalam radius 5 km
// beserta status kapasitas driver.
func (h *Handler) GetAvailableOrders(c *gin.Context) {
	driverIDStr := c.GetString("user_id")
	if driverIDStr == "" {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}
	driverID, err := uuid.Parse(driverIDStr)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	res, err := h.svc.GetAvailableOrders(c.Request.Context(), driverID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	orders := make([]AvailableOrder, 0, len(res.Orders))
	for _, o := range res.Orders {
		orders = append(orders, o)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"orders":             orders,
			"capacity_available": res.CapacityAvailable,
			"active_orders":      res.ActiveOrders,
			"max_active_orders":  res.MaxActiveOrders,
			"radius_km":          availableOrdersRadiusKm,
		},
	})
}

// statusForError memetakan error service ke status code HTTP.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrDriverLocationUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrNoLocation), errors.Is(err, ErrInvalidCoordinates):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// codeForError memetakan error service ke error code.
func codeForError(err error) string {
	switch {
	case errors.Is(err, ErrDriverLocationUnavailable):
		return "DRIVER_LOCATION_UNAVAILABLE"
	case errors.Is(err, ErrNoLocation):
		return "DRIVER_LOCATION_NOT_FOUND"
	case errors.Is(err, ErrInvalidCoordinates):
		return "INVALID_COORDINATES"
	default:
		return "INTERNAL_SERVER_ERROR"
	}
}

// writeError menulis error response dalam format
// { "success": false, "error": { "code": "...", "message": "..." } }.
func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
