// Package ride — HTTP Handler (Gin).
//
// Handler hanya bertugas memetakan request HTTP menjadi panggilan Service
// dan men-map error ke status code + error code sesuai API_CONTRACT 7.1.
// Tidak mengandung business logic (itu milik Service/Repository).
package ride

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// RideService adalah kontrak service yang dibutuhkan Handler.
// Dipenuhi oleh *Service; dijadikan interface agar mudah di-mock pada test.
type RideService interface {
	BookRide(ctx context.Context, req BookRideRequest) (*BookRideResponse, error)
	AcceptOrder(ctx context.Context, orderID uuid.UUID, driverID uuid.UUID) (*AcceptOrderResponse, error)
	UpdateRideStatus(ctx context.Context, req UpdateRideStatusRequest) (*UpdateRideStatusResponse, error)
	GetOrder(ctx context.Context, orderID uuid.UUID) (*RideOrder, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc RideService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc RideService) *Handler {
	return &Handler{svc: svc}
}

// bookRideRequestBody input JSON dari POST /rides/book (API_CONTRACT 7.1).
type bookRideRequestBody struct {
	PickupLat     float64 `json:"pickup_lat"`
	PickupLng     float64 `json:"pickup_lng"`
	DropoffLat    float64 `json:"dropoff_lat"`
	DropoffLng    float64 `json:"dropoff_lng"`
	PaymentMethod string  `json:"payment_method"`
}

// bookRideResponse memotong BookRideResponse service ke field yang
// dijanjikan API_CONTRACT 7.1 untuk respons booking.
type bookRideResponse struct {
	OrderID       uuid.UUID       `json:"order_id"`
	Status        string          `json:"status"`
	DistanceKm    decimal.Decimal `json:"distance_km"`
	EstimatedFare decimal.Decimal `json:"estimated_fare"`
	PaymentMethod string          `json:"payment_method"`
	ExpiresAt     time.Time       `json:"expires_at"`
}

// BookRide POST /api/v1/rides/book
func (h *Handler) BookRide(c *gin.Context) {
	var body bookRideRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if !validLatLng(body.PickupLat, body.PickupLng) ||
		!validLatLng(body.DropoffLat, body.DropoffLng) {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_COORDINATES", ErrInvalidCoordinates.Error())
		return
	}
	if body.PickupLat == body.DropoffLat && body.PickupLng == body.DropoffLng {
		writeError(c, http.StatusUnprocessableEntity, "SAME_PICKUP_DROPOFF", ErrSamePickupDropoff.Error())
		return
	}
	if body.PaymentMethod != PaymentMethodWallet && body.PaymentMethod != PaymentMethodCash {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PAYMENT_METHOD", ErrInvalidPaymentMethod.Error())
		return
	}

	// userID diambil dari JWT claim (diset oleh AuthMiddleware), bukan body.
	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.BookRide(c.Request.Context(), BookRideRequest{
		UserID:         userID,
		PickupLat:      body.PickupLat,
		PickupLng:      body.PickupLng,
		DropoffLat:     body.DropoffLat,
		DropoffLng:     body.DropoffLng,
		PaymentMethod:  body.PaymentMethod,
		IdempotencyKey: c.GetHeader("X-Idempotency-Key"),
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": bookRideResponse{
			OrderID:       resp.OrderID,
			Status:        resp.Status,
			DistanceKm:    resp.DistanceKm,
			EstimatedFare: resp.EstimatedFare,
			PaymentMethod: resp.PaymentMethod,
			ExpiresAt:     resp.ExpiresAt,
		},
	})
}

// AcceptOrder POST /api/v1/rides/:order_id/accept
// Auth: driver (JWT). Mengambil order_id dari path & driver_id dari JWT claim
// `user_id`, lalu menetapkan driver ke order.
func (h *Handler) AcceptOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("order_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order_id")
		return
	}

	// driverID diambil dari JWT claim (diset oleh AuthMiddleware). Handler
	// hanya mengizinkan driver; role 'driver' divalidasi di service.
	driverID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.AcceptOrder(c.Request.Context(), orderID, driverID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": AcceptOrderResponse{
			OrderID:    resp.OrderID,
			Status:     resp.Status,
			DriverID:   resp.DriverID,
			AssignedAt: resp.AssignedAt,
		},
	})
}

// updateStatusRequestBody input JSON dari PATCH /rides/:order_id/status
// (API_CONTRACT 7.3): status target + reason (wajib untuk CANCELLED).
type updateStatusRequestBody struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// UpdateStatus PATCH /api/v1/rides/:order_id/status
// Auth: customer (pemilik order) atau driver tertunjuk. Mengambil order_id
// dari path dan user_id dari JWT claim `user_id`, lalu meneruskan ke service
// yang memvalidasi transisi + otorisasi. Endpoint ini menangani transisi
// DRIVER_ARRIVED → TRIP_STARTED → COMPLETED (settlement otomatis) dan
// CANCELLED (customer / driver).
func (h *Handler) UpdateStatus(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("order_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order_id")
		return
	}

	var body updateStatusRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.UpdateRideStatus(c.Request.Context(), UpdateRideStatusRequest{
		OrderID: orderID,
		UserID:  userID,
		Status:  body.Status,
		Reason:  body.Reason,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"order_id":          resp.OrderID,
			"status":            resp.Status,
			"status_updated_at": time.Now(),
		},
	})
}

// rideDetailResponse memotong RideOrder ke field yang dijanjikan
// API_CONTRACT 7.4 (GET /rides/{order_id}). Kolom internal (wallet_id,
// voucher_id, surge, settlement_notes) tidak diekspos ke konsumen.
type rideDetailResponse struct {
	OrderID            uuid.UUID        `json:"order_id"`
	CustomerID         uuid.UUID        `json:"customer_id"`
	DriverID           *uuid.UUID       `json:"driver_id"`
	Status             string           `json:"status"`
	PickupLat          float64          `json:"pickup_lat"`
	PickupLng          float64          `json:"pickup_lng"`
	PickupAddress      string           `json:"pickup_address"`
	DropoffLat         float64          `json:"dropoff_lat"`
	DropoffLng         float64          `json:"dropoff_lng"`
	DropoffAddress     string           `json:"dropoff_address"`
	DistanceKm         decimal.Decimal  `json:"distance_km"`
	BaseFare           decimal.Decimal  `json:"base_fare"`
	PerKmRate          decimal.Decimal  `json:"per_km_rate"`
	EstimatedFare      decimal.Decimal  `json:"estimated_fare"`
	ActualFare         *decimal.Decimal `json:"actual_fare"`
	DriverEarning      *decimal.Decimal `json:"driver_earning"`
	DiscountAmount     *decimal.Decimal `json:"discount_amount"`
	PaymentMethod      string           `json:"payment_method"`
	CancellationReason *string          `json:"cancellation_reason"`
	CreatedAt          time.Time        `json:"created_at"`
	ExpiresAt          *time.Time       `json:"expires_at"`
	AssignedAt         *time.Time       `json:"assigned_at"`
	PickupAt           *time.Time       `json:"pickup_at"`
	CompletedAt        *time.Time       `json:"completed_at"`
	SettledAt          *time.Time       `json:"settled_at"`
	IsSettled          bool             `json:"is_settled"`
}

// GetRide GET /api/v1/rides/:order_id
// Auth: JWT. Detail order hanya untuk pemiliknya — customer pemesan atau
// driver tertunjuk. Ownership divalidasi terhadap user_id dari JWT claim;
// selain itu → 403 FORBIDDEN (mirror ErrNotAllowed di UpdateStatus).
func (h *Handler) GetRide(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("order_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order_id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	order, err := h.svc.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	if order.CustomerID != userID && (order.DriverID == nil || *order.DriverID != userID) {
		writeError(c, statusForError(ErrNotAllowed), codeForError(ErrNotAllowed), ErrNotAllowed.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": rideDetailResponse{
			OrderID:            order.ID,
			CustomerID:         order.CustomerID,
			DriverID:           order.DriverID,
			Status:             order.Status,
			PickupLat:          order.PickupLat,
			PickupLng:          order.PickupLng,
			PickupAddress:      order.PickupAddress,
			DropoffLat:         order.DropoffLat,
			DropoffLng:         order.DropoffLng,
			DropoffAddress:     order.DropoffAddress,
			DistanceKm:         order.DistanceKm,
			BaseFare:           order.BaseFare,
			PerKmRate:          order.PerKmRate,
			EstimatedFare:      order.EstimatedFare,
			ActualFare:         order.ActualFare,
			DriverEarning:      order.DriverEarning,
			DiscountAmount:     order.DiscountAmount,
			PaymentMethod:      order.PaymentMethod,
			CancellationReason: order.CancellationReason,
			CreatedAt:          order.CreatedAt,
			ExpiresAt:          order.ExpiresAt,
			AssignedAt:         order.AssignedAt,
			PickupAt:           order.PickupAt,
			CompletedAt:        order.CompletedAt,
			SettledAt:          order.SettledAt,
			IsSettled:          order.IsSettled,
		},
	})
}

// --- helpers ---

// userIDFromContext mengambil user_id dari Gin context (diset oleh
// AuthMiddleware dari JWT claim `sub`). Mengembalikan false jika konteks
// tidak berisi identitas user yang valid.
func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return uuid.Nil, false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return userID, true
}

// statusForError memetakan error service ke status code HTTP (API_CONTRACT 7.1).
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrCustomerNotFound),
		errors.Is(err, ErrWalletNotFound),
		errors.Is(err, ErrOrderNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrDriverNotFound):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrNotCustomer),
		errors.Is(err, ErrNotDriver):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidCoordinates),
		errors.Is(err, ErrSamePickupDropoff),
		errors.Is(err, ErrInvalidPaymentMethod),
		errors.Is(err, ErrCustomerInactive),
		errors.Is(err, ErrOverdueDebt),
		errors.Is(err, ErrWalletInactive),
		errors.Is(err, ErrInsufficientBalance),
		errors.Is(err, ErrDriverInactive),
		errors.Is(err, ErrInsufficientDriverBalance),
		errors.Is(err, ErrIdempotencyKeyRequired):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrIdempotencyInProgress),
		errors.Is(err, ErrDriverBusy),
		errors.Is(err, ErrOrderNotSearching),
		errors.Is(err, ErrLockTimeout):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidTransition):
		return http.StatusConflict
	case errors.Is(err, ErrNotAllowed):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidStatus):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// codeForError memetakan error service ke error code (API_CONTRACT 7.1).
func codeForError(err error) string {
	switch {
	case errors.Is(err, ErrCustomerNotFound):
		return "CUSTOMER_NOT_FOUND"
	case errors.Is(err, ErrWalletNotFound):
		return "WALLET_NOT_FOUND"
	case errors.Is(err, ErrOrderNotFound):
		return "ORDER_NOT_FOUND"
	case errors.Is(err, ErrDriverNotFound):
		return "DRIVER_NOT_FOUND"
	case errors.Is(err, ErrNotCustomer):
		return "NOT_CUSTOMER"
	case errors.Is(err, ErrNotDriver):
		return "NOT_DRIVER"
	case errors.Is(err, ErrInvalidCoordinates):
		return "INVALID_COORDINATES"
	case errors.Is(err, ErrSamePickupDropoff):
		return "SAME_PICKUP_DROPOFF"
	case errors.Is(err, ErrInvalidPaymentMethod):
		return "INVALID_PAYMENT_METHOD"
	case errors.Is(err, ErrCustomerInactive):
		return "CUSTOMER_INACTIVE"
	case errors.Is(err, ErrOverdueDebt):
		return "OVERDUE_DEBT"
	case errors.Is(err, ErrWalletInactive):
		return "WALLET_INACTIVE"
	case errors.Is(err, ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE"
	case errors.Is(err, ErrDriverInactive):
		return "DRIVER_INACTIVE"
	case errors.Is(err, ErrDriverBusy):
		return "DRIVER_BUSY"
	case errors.Is(err, ErrOrderNotSearching):
		return "ORDER_NOT_SEARCHING"
	case errors.Is(err, ErrInsufficientDriverBalance):
		return "INSUFFICIENT_DRIVER_BALANCE"
	case errors.Is(err, ErrLockTimeout):
		return "LOCK_TIMEOUT"
	case errors.Is(err, ErrInvalidStatus):
		return "INVALID_REQUEST"
	case errors.Is(err, ErrInvalidTransition):
		return "INVALID_STATUS_TRANSITION"
	case errors.Is(err, ErrNotAllowed):
		return "FORBIDDEN"
	case errors.Is(err, ErrIdempotencyKeyRequired):
		return "INVALID_IDEMPOTENCY_KEY"
	case errors.Is(err, ErrIdempotencyInProgress):
		return "IDEMPOTENCY_IN_PROGRESS"
	case errors.Is(err, ErrInvalidCachedResponse):
		return "INTERNAL_ERROR"
	default:
		return "INTERNAL_SERVER_ERROR"
	}
}

// writeError menulis error response sesuai format API_CONTRACT:
// { "success": false, "error": { "code": "...", "message": "..." } }
func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
