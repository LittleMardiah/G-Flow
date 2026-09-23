// Package send — HTTP Handler (Gin) untuk modul G-Send (Phase 3, Task 3.5:
// Order Creation & Pricing).
//
// Handler hanya memetakan request HTTP ke panggilan Service dan men-map error
// ke status code + error code sesuai API_CONTRACT 9 / 7.1. Tidak mengandung
// business logic (itu milik Service/Repository).
//   - POST /api/v1/send-orders: RBAC role 'customer' di main.go; X-Idempotency-Key wajib.
//   - GET /api/v1/send-orders/:id & PATCH /api/v1/send-orders/:id: auth wajib;
//     ownership (sender / driver tertunjuk) divalidasi di Service.
package send

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SendService adalah kontrak service yang dibutuhkan Handler.
// Dipenuhi oleh *Service; dijadikan interface agar mudah di-mock pada test.
type SendService interface {
	CreateSendOrder(ctx context.Context, req CreateSendOrderRequest) (*CreateSendOrderResponse, error)
	GetSendOrder(ctx context.Context, orderID, userID uuid.UUID) (*SendOrderDetail, error)
	UpdateSendOrderStatus(ctx context.Context, req UpdateSendOrderStatusRequest) (*UpdateSendOrderStatusResponse, error)
	AcceptSendOrder(ctx context.Context, req AcceptSendOrderRequest) (*AcceptSendOrderResponse, error)
	UpdateSendOrderStop(ctx context.Context, req UpdateSendOrderStopRequest) (*UpdateSendOrderStopResponse, error)
	GetSendOrderHistory(ctx context.Context, senderID uuid.UUID, page, pageSize int) ([]*SendOrder, int, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc SendService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc SendService) *Handler {
	return &Handler{svc: svc}
}

// createSendOrderRequestBody input JSON dari POST /send-orders (ROADMAP 3.5.1).
type createSendOrderRequestBody struct {
	PickupLat           float64 `json:"pickup_lat"`
	PickupLng           float64 `json:"pickup_lng"`
	PickupAddress       string  `json:"pickup_address" binding:"required"`
	PackageWeightKg     float64 `json:"package_weight_kg"`
	PackageDimensionsCm string  `json:"package_dimensions_cm"`
	PackageDescription  string  `json:"package_description"`
	PackageType         string  `json:"package_type"`
	DeclaredValue       float64 `json:"declared_value"`
	PaymentMethod       string  `json:"payment_method" binding:"required"`
	Stops               []struct {
		RecipientName   string  `json:"recipient_name"`
		RecipientPhone  string  `json:"recipient_phone"`
		DeliveryAddress string  `json:"delivery_address" binding:"required"`
		DeliveryLat     float64 `json:"delivery_lat"`
		DeliveryLng     float64 `json:"delivery_lng"`
	} `json:"stops" binding:"required,min=1,max=3,dive"`
}

// CreateSendOrder POST /api/v1/send-orders
// Auth: customer. Idempotency key wajib di header X-Idempotency-Key; request
// dengan key yang sama mengembalikan response pertama (L1 Redis / L2 DB).
func (h *Handler) CreateSendOrder(c *gin.Context) {
	key := c.GetHeader("X-Idempotency-Key")
	if key == "" {
		writeError(c, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REQUIRED", "X-Idempotency-Key header is required")
		return
	}

	var body createSendOrderRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	req := CreateSendOrderRequest{
		UserID:              userID,
		PickupLat:           body.PickupLat,
		PickupLng:           body.PickupLng,
		PickupAddress:       body.PickupAddress,
		PackageWeightKg:     body.PackageWeightKg,
		PackageDimensionsCm: body.PackageDimensionsCm,
		PackageDescription:  body.PackageDescription,
		PackageType:         strings.ToUpper(body.PackageType),
		DeclaredValue:       decimal.NewFromFloat(body.DeclaredValue),
		PaymentMethod:       strings.ToUpper(body.PaymentMethod),
		IdempotencyKey:      key,
	}
	for _, st := range body.Stops {
		req.Stops = append(req.Stops, SendStopRequest{
			RecipientName:   st.RecipientName,
			RecipientPhone:  st.RecipientPhone,
			DeliveryAddress: st.DeliveryAddress,
			DeliveryLat:     st.DeliveryLat,
			DeliveryLng:     st.DeliveryLng,
		})
	}

	resp, err := h.svc.CreateSendOrder(c.Request.Context(), req)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

// GetSendOrder GET /api/v1/send-orders/:id
// Auth: sender (pemilik) atau driver tertunjuk. Mengembalikan detail order +
// stops.
func (h *Handler) GetSendOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	detail, err := h.svc.GetSendOrder(c.Request.Context(), orderID, userID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"order": detail.Order,
			"stops": detail.Stops,
		},
	})
}

// GetSendOrderHistory GET /api/v1/send-orders?page=&page_size=
// Auth: customer (RBAC di route). Riwayat send order sender dengan pagination
// (default page=1, page_size=20, maks 50; `limit` alias dari `page_size`),
// urut created_at DESC. Query invalid → 400 INVALID_REQUEST.
func (h *Handler) GetSendOrderHistory(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid page")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid page_size")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	orders, total, err := h.svc.GetSendOrderHistory(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"orders": orders,
			"meta": gin.H{
				"page":        page,
				"page_size":   pageSize,
				"total":       total,
				"total_pages": totalPages,
			},
		},
	})
}

// updateSendOrderStatusRequestBody input JSON dari PATCH /send-orders/{id}.
type updateSendOrderStatusRequestBody struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

// UpdateSendOrderStatus PATCH /api/v1/send-orders/:id
// Auth: sender (pemilik) atau driver tertunjuk.
// Transisi status menggunakan FOR UPDATE NOWAIT + audit trail.
func (h *Handler) UpdateSendOrderStatus(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	var body updateSendOrderStatusRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.UpdateSendOrderStatus(c.Request.Context(), UpdateSendOrderStatusRequest{
		OrderID: orderID,
		UserID:  userID,
		Status:  strings.ToUpper(body.Status),
		Reason:  body.Reason,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// AcceptSendOrder POST /api/v1/send-orders/:id/accept
// Auth: driver. Idempotency key wajib di header X-Idempotency-Key.
// Capacity check + atomic assign (SEARCHING_DRIVER → DRIVER_ASSIGNED).
func (h *Handler) AcceptSendOrder(c *gin.Context) {
	key := c.GetHeader("X-Idempotency-Key")
	if key == "" {
		writeError(c, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REQUIRED", "X-Idempotency-Key header is required")
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.AcceptSendOrder(c.Request.Context(), AcceptSendOrderRequest{
		OrderID:        orderID,
		DriverID:       userID,
		IdempotencyKey: key,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// updateSendOrderStopRequestBody input JSON dari PATCH /send-orders/{id}/stops/{stop_id}.
type updateSendOrderStopRequestBody struct {
	Status           string `json:"status" binding:"required"`
	DeliveryPhotoURL string `json:"delivery_photo_url"`
}

// UpdateSendOrderStop PATCH /api/v1/send-orders/:id/stops/:stop_id
// Auth: driver tertunjuk. Menandai stop COMPLETED + proof photo; ketika semua
// stop selesai, order otomatis DELIVERED → SETTLED.
func (h *Handler) UpdateSendOrderStop(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	stopID, err := uuid.Parse(c.Param("stop_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_STOP_ID", "invalid stop id")
		return
	}

	var body updateSendOrderStopRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.UpdateSendOrderStop(c.Request.Context(), UpdateSendOrderStopRequest{
		OrderID:          orderID,
		StopID:           stopID,
		UserID:           userID,
		Status:           strings.ToUpper(body.Status),
		DeliveryPhotoURL: body.DeliveryPhotoURL,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
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
	case errors.Is(err, ErrSendOrderNotFound),
		errors.Is(err, ErrWalletNotFound),
		errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrDriverNotFound),
		errors.Is(err, ErrStopNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrNotAllowed),
		errors.Is(err, ErrNotCustomer),
		errors.Is(err, ErrNotDriver):
		return http.StatusForbidden
	case errors.Is(err, ErrIdempotencyInProgress),
		errors.Is(err, ErrInvalidCachedResponse),
		errors.Is(err, ErrInvalidTransition),
		errors.Is(err, ErrLockTimeout),
		errors.Is(err, ErrDriverBusy),
		errors.Is(err, ErrOrderNotSearching),
		errors.Is(err, ErrStopsNotDelivered):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidStatus),
		errors.Is(err, ErrInvalidStopStatus):
		return http.StatusBadRequest
	case errors.Is(err, ErrCustomerInactive),
		errors.Is(err, ErrOverdueDebt),
		errors.Is(err, ErrWalletInactive),
		errors.Is(err, ErrInsufficientBalance),
		errors.Is(err, ErrInvalidPaymentMethod),
		errors.Is(err, ErrInvalidPackageType),
		errors.Is(err, ErrInvalidWeight),
		errors.Is(err, ErrInvalidDeclaredValue),
		errors.Is(err, ErrInvalidStopsCount),
		errors.Is(err, ErrInvalidRecipient),
		errors.Is(err, ErrInvalidCoordinates),
		errors.Is(err, ErrIdempotencyKeyRequired),
		errors.Is(err, ErrDriverInactive),
		errors.Is(err, ErrInsufficientDriverBalance),
		errors.Is(err, ErrDriverCapacityExceeded):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// codeForError memetakan error service ke error code (API_CONTRACT 7.1).
func codeForError(err error) string {
	switch {
	case errors.Is(err, ErrSendOrderNotFound):
		return "SEND_ORDER_NOT_FOUND"
	case errors.Is(err, ErrWalletNotFound):
		return "WALLET_NOT_FOUND"
	case errors.Is(err, ErrUserNotFound):
		return "USER_NOT_FOUND"
	case errors.Is(err, ErrDriverNotFound):
		return "DRIVER_NOT_FOUND"
	case errors.Is(err, ErrStopNotFound):
		return "STOP_NOT_FOUND"
	case errors.Is(err, ErrNotAllowed):
		return "FORBIDDEN"
	case errors.Is(err, ErrNotCustomer):
		return "NOT_CUSTOMER"
	case errors.Is(err, ErrNotDriver):
		return "NOT_DRIVER"
	case errors.Is(err, ErrCustomerInactive):
		return "CUSTOMER_INACTIVE"
	case errors.Is(err, ErrOverdueDebt):
		return "OVERDUE_DEBT"
	case errors.Is(err, ErrWalletInactive):
		return "WALLET_INACTIVE"
	case errors.Is(err, ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE"
	case errors.Is(err, ErrInvalidPaymentMethod):
		return "INVALID_PAYMENT_METHOD"
	case errors.Is(err, ErrInvalidPackageType):
		return "INVALID_PACKAGE_TYPE"
	case errors.Is(err, ErrInvalidWeight):
		return "INVALID_WEIGHT"
	case errors.Is(err, ErrInvalidDeclaredValue):
		return "INVALID_DECLARED_VALUE"
	case errors.Is(err, ErrInvalidStopsCount):
		return "INVALID_STOPS_COUNT"
	case errors.Is(err, ErrInvalidRecipient):
		return "INVALID_RECIPIENT"
	case errors.Is(err, ErrInvalidCoordinates):
		return "INVALID_COORDINATES"
	case errors.Is(err, ErrIdempotencyKeyRequired):
		return "IDEMPOTENCY_KEY_REQUIRED"
	case errors.Is(err, ErrIdempotencyInProgress):
		return "IDEMPOTENCY_IN_PROGRESS"
	case errors.Is(err, ErrInvalidCachedResponse):
		return "IDEMPOTENCY_INVALID_CACHE"
	case errors.Is(err, ErrInvalidStatus):
		return "INVALID_STATUS"
	case errors.Is(err, ErrInvalidStopStatus):
		return "INVALID_STOP_STATUS"
	case errors.Is(err, ErrInvalidTransition):
		return "INVALID_TRANSITION"
	case errors.Is(err, ErrLockTimeout):
		return "LOCK_TIMEOUT"
	case errors.Is(err, ErrDriverBusy):
		return "DRIVER_BUSY"
	case errors.Is(err, ErrDriverInactive):
		return "DRIVER_INACTIVE"
	case errors.Is(err, ErrInsufficientDriverBalance):
		return "INSUFFICIENT_DRIVER_BALANCE"
	case errors.Is(err, ErrDriverCapacityExceeded):
		return "DRIVER_CAPACITY_EXCEEDED"
	case errors.Is(err, ErrOrderNotSearching):
		return "ORDER_NOT_SEARCHING_DRIVER"
	case errors.Is(err, ErrStopsNotDelivered):
		return "STOPS_NOT_DELIVERED"
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
