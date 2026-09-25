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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DriverService adalah kontrak service yang dibutuhkan Handler. Dipenuhi oleh
// *Service; dijadikan interface agar mudah di-mock pada test.
type DriverService interface {
	GetAvailableOrders(ctx context.Context, driverID uuid.UUID) (*AvailableOrdersResult, error)
	GetActiveOrders(ctx context.Context, driverID uuid.UUID) (*ActiveOrdersResult, error)
	GetDriverProfile(ctx context.Context, driverID uuid.UUID) (*DriverProfile, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc DriverService
}

type DriverProfileResponse struct {
	DriverID          uuid.UUID `json:"driver_id"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	Phone             *string   `json:"phone"`
	VehicleType       *string   `json:"vehicle_type"`
	VehiclePlate      *string   `json:"vehicle_plate"`
	LicenseNumber     *string   `json:"license_number"`
	LicenseExpiry     *string   `json:"license_expiry"`
	Status            string    `json:"status"`
	RatingAvg         float64   `json:"rating_avg"`
	TotalRides        int64     `json:"total_rides"`
	BankName          *string   `json:"bank_name"`
	BankAccountMasked *string   `json:"bank_account_masked"`
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

// GetDriverOrders GET /api/v1/drivers/orders
// Auth: driver (JWT + RBAC). Mengambil driver_id dari JWT claim `user_id`,
// lalu mengembalikan daftar order aktif milik driver (ride + food + send)
// yang sedang dikerjakan — TD-077 A2. Response flat list, tanpa meta (sama
// dgn GetAvailableOrders).
func (h *Handler) GetDriverOrders(c *gin.Context) {
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

	res, err := h.svc.GetActiveOrders(c.Request.Context(), driverID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	orders := make([]DriverActiveOrder, 0, len(res.Orders))
	for _, o := range res.Orders {
		orders = append(orders, o)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"orders": orders,
		},
	})
}

func (h *Handler) GetDriverMe(c *gin.Context) {
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

	profile, err := h.svc.GetDriverProfile(c.Request.Context(), driverID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}
	if profile == nil {
		writeError(c, http.StatusNotFound, "DRIVER_NOT_FOUND", ErrDriverNotFound.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    toDriverProfileResponse(profile),
	})
}

func toDriverProfileResponse(profile *DriverProfile) DriverProfileResponse {
	return DriverProfileResponse{
		DriverID:          profile.DriverID,
		Name:              profile.Name,
		Email:             profile.Email,
		Phone:             profile.Phone,
		VehicleType:       profile.VehicleType,
		VehiclePlate:      profile.VehiclePlate,
		LicenseNumber:     profile.LicenseNumber,
		LicenseExpiry:     profile.LicenseExpiry,
		Status:            profile.Status,
		RatingAvg:         profile.RatingAvg,
		TotalRides:        profile.TotalRides,
		BankName:          profile.BankName,
		BankAccountMasked: maskBankAccount(profile.BankAccountNumber),
	}
}

func maskBankAccount(account *string) *string {
	if account == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*account)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) <= 4 {
		masked := "****"
		return &masked
	}
	masked := "****" + trimmed[len(trimmed)-4:]
	return &masked
}

// statusForError memetakan error service ke status code HTTP.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrDriverNotFound):
		return http.StatusNotFound
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
	case errors.Is(err, ErrDriverNotFound):
		return "DRIVER_NOT_FOUND"
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
