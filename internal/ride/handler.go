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
	"strconv"
	"strings"
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
	GetRidesHistory(ctx context.Context, customerID uuid.UUID, page, pageSize int, status string) ([]RideOrder, int, error)
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
// VoucherCode/VoucherID opsional (TD-070): salah satu, tidak keduanya.
type bookRideRequestBody struct {
	PickupLat     float64 `json:"pickup_lat"`
	PickupLng     float64 `json:"pickup_lng"`
	DropoffLat    float64 `json:"dropoff_lat"`
	DropoffLng    float64 `json:"dropoff_lng"`
	PaymentMethod string  `json:"payment_method"`
	VoucherCode   string  `json:"voucher_code"`
	VoucherID     string  `json:"voucher_id"`
}

// bookRideResponse memotong BookRideResponse service ke field yang
// dijanjikan API_CONTRACT 7.1 untuk respons booking. DiscountAmount &
// VoucherCode hanya muncul saat booking memakai voucher (TD-070).
type bookRideResponse struct {
	OrderID       uuid.UUID       `json:"order_id"`
	Status        string          `json:"status"`
	DistanceKm    decimal.Decimal `json:"distance_km"`
	EstimatedFare decimal.Decimal `json:"estimated_fare"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	VoucherCode   *string         `json:"voucher_code"`
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

	// Voucher (TD-070): diizinkan salah satu dari voucher_code / voucher_id.
	// Keduanya diisi → service mengembalikan VOUCHER_INVALID. voucher_id yang
	// bukan UUID valid → 422 langsung dari handler.
	var voucherID *uuid.UUID
	if body.VoucherID != "" {
		id, err := uuid.Parse(body.VoucherID)
		if err != nil {
			writeError(c, http.StatusUnprocessableEntity, "INVALID_VOUCHER_ID", "invalid voucher_id")
			return
		}
		voucherID = &id
	}
	req := BookRideRequest{
		UserID:         userID,
		PickupLat:      body.PickupLat,
		PickupLng:      body.PickupLng,
		DropoffLat:     body.DropoffLat,
		DropoffLng:     body.DropoffLng,
		PaymentMethod:  body.PaymentMethod,
		IdempotencyKey: c.GetHeader("X-Idempotency-Key"),
		VoucherID:      voucherID,
	}
	if body.VoucherCode != "" {
		req.VoucherCode = &body.VoucherCode
	}

	resp, err := h.svc.BookRide(c.Request.Context(), req)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": bookRideResponse{
			OrderID:        resp.OrderID,
			Status:         resp.Status,
			DistanceKm:     resp.DistanceKm,
			EstimatedFare:  resp.EstimatedFare,
			DiscountAmount: resp.DiscountAmount,
			VoucherCode:    resp.VoucherCode,
			PaymentMethod:  resp.PaymentMethod,
			ExpiresAt:      resp.ExpiresAt,
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
// ActualFare opsional (TD-069): fare aktual dari driver saat COMPLETED
// (dalam rupiah bilangan bulat); nil/no-request → pakai estimated_fare.
type updateStatusRequestBody struct {
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	ActualFare *int64 `json:"actual_fare"`
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

	// ActualFare dimasukkan ke service sebagai *decimal (nil → estimated).
	var actualFare *decimal.Decimal
	if body.ActualFare != nil && *body.ActualFare > 0 {
		v := decimal.NewFromInt(*body.ActualFare)
		actualFare = &v
	}

	resp, err := h.svc.UpdateRideStatus(c.Request.Context(), UpdateRideStatusRequest{
		OrderID:    orderID,
		UserID:     userID,
		Status:     body.Status,
		Reason:     body.Reason,
		ActualFare: actualFare,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	data := gin.H{
		"order_id":          resp.OrderID,
		"status":            resp.Status,
		"status_updated_at": time.Now(),
	}
	if resp.CancellationFee != nil {
		data["cancellation_fee"] = resp.CancellationFee
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
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
	CancellationFee    *decimal.Decimal `json:"cancellation_fee"`
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
			CancellationFee:    order.CancellationFee,
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

// rideHistoryResponse memotong RideOrder ke field yang diekspos pada
// GET /rides list history (API_CONTRACT 7.4). Kolom internal (wallet_id,
// voucher_id, surge, base_fare, per_km_rate) tidak ditampilkan di list.
type rideHistoryResponse struct {
	OrderID        uuid.UUID        `json:"order_id"`
	Status         string           `json:"status"`
	PickupAddress  string           `json:"pickup_address"`
	DropoffAddress string           `json:"dropoff_address"`
	DistanceKm     decimal.Decimal  `json:"distance_km"`
	EstimatedFare  decimal.Decimal  `json:"estimated_fare"`
	ActualFare     *decimal.Decimal `json:"actual_fare"`
	PaymentMethod  string           `json:"payment_method"`
	CreatedAt      time.Time        `json:"created_at"`
	CompletedAt    *time.Time       `json:"completed_at"`
	SettledAt      *time.Time       `json:"settled_at"`
}

// GetRideHistory GET /api/v1/rides
// Auth: customer (RBAC di route). Riwayat ride customer dengan pagination
// (default page=1, page_size=20, maks 50), urut created_at DESC. Status
// opsional untuk filter (exact match, case-insensitive).
func (h *Handler) GetRideHistory(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid page")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid page_size")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	status := strings.ToUpper(c.Query("status"))

	orders, total, err := h.svc.GetRidesHistory(c.Request.Context(), userID, page, pageSize, status)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	list := make([]rideHistoryResponse, 0, len(orders))
	for _, o := range orders {
		list = append(list, rideHistoryResponse{
			OrderID:        o.ID,
			Status:         o.Status,
			PickupAddress:  o.PickupAddress,
			DropoffAddress: o.DropoffAddress,
			DistanceKm:     o.DistanceKm,
			EstimatedFare:  o.EstimatedFare,
			ActualFare:     o.ActualFare,
			PaymentMethod:  o.PaymentMethod,
			CreatedAt:      o.CreatedAt,
			CompletedAt:    o.CompletedAt,
			SettledAt:      o.SettledAt,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"orders": list,
		},
		"meta": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
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
		errors.Is(err, ErrOrderNotFound),
		errors.Is(err, ErrVoucherNotFound):
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
		errors.Is(err, ErrIdempotencyKeyRequired),
		errors.Is(err, ErrVoucherInvalid),
		errors.Is(err, ErrVoucherExpired),
		errors.Is(err, ErrVoucherMinOrder),
		errors.Is(err, ErrVoucherPerUserLimit):
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
	case errors.Is(err, ErrInvalidStatus),
		errors.Is(err, ErrInvalidPagination):
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
	case errors.Is(err, ErrVoucherNotFound):
		return "VOUCHER_NOT_FOUND"
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
	case errors.Is(err, ErrInvalidPagination):
		return "INVALID_REQUEST"
	case errors.Is(err, ErrInvalidTransition):
		return "INVALID_STATUS_TRANSITION"
	case errors.Is(err, ErrNotAllowed):
		return "FORBIDDEN"
	case errors.Is(err, ErrIdempotencyKeyRequired):
		return "INVALID_IDEMPOTENCY_KEY"
	case errors.Is(err, ErrVoucherInvalid):
		return "VOUCHER_INVALID"
	case errors.Is(err, ErrVoucherExpired):
		return "VOUCHER_EXPIRED"
	case errors.Is(err, ErrVoucherMinOrder):
		return "VOUCHER_MIN_ORDER_NOT_MET"
	case errors.Is(err, ErrVoucherPerUserLimit):
		return "VOUCHER_PER_USER_LIMIT"
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
