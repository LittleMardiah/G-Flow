// Package location — HTTP Handler (Gin).
//
// Handler hanya memetakan request HTTP menjadi panggilan Service dan men-map
// error ke status code + error code. Tidak mengandung business logic.
package location

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LocationService adalah kontrak service yang dibutuhkan Handler.
// Dipenuhi oleh *Service; dijadikan interface agar mudah di-mock pada test.
type LocationService interface {
	UpdateLocation(ctx context.Context, driverID uuid.UUID, lat, lng float64) error
	GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64) ([]NearbyDriver, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc LocationService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc LocationService) *Handler {
	return &Handler{svc: svc}
}

// updateLocationRequestBody input JSON dari POST /drivers/location.
type updateLocationRequestBody struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// UpdateLocation POST /api/v1/drivers/location
// Auth: driver (JWT). Mengambil driver_id dari JWT claim `user_id` dan
// koordinat dari body, lalu menyimpan lokasi terbaru driver.
func (h *Handler) UpdateLocation(c *gin.Context) {
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

	var body updateLocationRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if !validLatLng(body.Latitude, body.Longitude) {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_COORDINATES", ErrInvalidCoordinates.Error())
		return
	}

	if err := h.svc.UpdateLocation(c.Request.Context(), driverID, body.Latitude, body.Longitude); err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"driver_id": driverID,
			"latitude":  body.Latitude,
			"longitude": body.Longitude,
		},
	})
}

// nearbyDriverResponse memotong NearbyDriver ke bentuk respons API.
type nearbyDriverResponse struct {
	DriverID   uuid.UUID `json:"driver_id"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	DistanceKm float64   `json:"distance_km"`
}

// GetNearbyDrivers GET /api/v1/drivers/nearby
// Auth: optional (public/customer/driver). Membaca query lat, lng,
// radius_km (default 5) lalu mencari driver aktif terdekat.
func (h *Handler) GetNearbyDrivers(c *gin.Context) {
	lat, lng, ok := parseCoordinates(c)
	if !ok {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_COORDINATES", "lat/lng wajib dan harus valid")
		return
	}

	radiusKm := 5.0
	if v := c.Query("radius_km"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil || parsed <= 0 {
			writeError(c, http.StatusUnprocessableEntity, "INVALID_RADIUS", ErrInvalidRadius.Error())
			return
		}
		radiusKm = parsed
	}

	drivers, err := h.svc.GetNearbyDrivers(c.Request.Context(), lat, lng, radiusKm)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	resp := make([]nearbyDriverResponse, 0, len(drivers))
	for _, d := range drivers {
		resp = append(resp, nearbyDriverResponse{
			DriverID:   d.DriverID,
			Latitude:   d.Lat,
			Longitude:  d.Lng,
			DistanceKm: d.DistanceKm,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"drivers": resp,
	})
}

// parseCoordinates membaca & memvalidasi query lat/lng.
func parseCoordinates(c *gin.Context) (float64, float64, bool) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	if latStr == "" || lngStr == "" {
		return 0, 0, false
	}
	lat, errLat := strconv.ParseFloat(latStr, 64)
	lng, errLng := strconv.ParseFloat(lngStr, 64)
	if errLat != nil || errLng != nil || !validLatLng(lat, lng) {
		return 0, 0, false
	}
	return lat, lng, true
}

// statusForError memetakan error service ke status code HTTP.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrInvalidCoordinates),
		errors.Is(err, ErrInvalidRadius):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// codeForError memetakan error service ke error code.
func codeForError(err error) string {
	switch {
	case errors.Is(err, ErrInvalidCoordinates):
		return "INVALID_COORDINATES"
	case errors.Is(err, ErrInvalidRadius):
		return "INVALID_RADIUS"
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
