// Package food — HTTP Handler (Gin) untuk modul G-Food (Phase 3, Task 3.2:
// Merchant Onboarding & Catalog Management; Task 3.3: Food Order Creation & Cart).
//
// Handler hanya memetakan request HTTP ke panggilan Service dan men-map error
// ke status code + error code sesuai API_CONTRACT 7.1. Tidak mengandung
// business logic (itu milik Service/Repository).
//   - Resource merchant (Task 3.2): RBAC role 'merchant' di main.go.
//   - Catalog discovery (Task 3.3): auth OPSIONAL — pemilik merchant melihat
//     seluruh item, anonymous hanya item yang tersedia.
//   - Food orders (Task 3.3): auth wajib; RBAC per-route (customer untuk
//     create/history; customer/merchant/driver untuk status & detail —
//     ownership di validasi di Service).
package food

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

// FoodService adalah kontrak service yang dibutuhkan Handler.
// Dipenuhi oleh *Service; dijadikan interface agar mudah di-mock pada test.
type FoodService interface {
	RegisterMerchant(ctx context.Context, req RegisterMerchantRequest) (*RegisterMerchantResponse, error)
	GetMerchant(ctx context.Context, merchantID uuid.UUID) (*Merchant, error)
	UpdateMerchant(ctx context.Context, req UpdateMerchantRequest) (*Merchant, error)

	CreateMenu(ctx context.Context, req CreateMenuRequest) (*MenuResponse, error)
	UpdateMenu(ctx context.Context, req UpdateMenuRequest) (*MenuResponse, error)
	DeleteMenu(ctx context.Context, merchantID, menuID, userID uuid.UUID) error
	GetMenus(ctx context.Context, merchantID, userID uuid.UUID) ([]*MenuResponse, error)

	CreateItem(ctx context.Context, req CreateItemRequest) (*ItemResponse, error)
	UpdateItem(ctx context.Context, req UpdateItemRequest) (*ItemResponse, error)
	DeleteItem(ctx context.Context, merchantID, itemID, userID uuid.UUID) error
	GetItems(ctx context.Context, merchantID, userID uuid.UUID) ([]*ItemResponse, error)

	// Task 3.3 — Catalog Discovery (3.3.1/3.3.2).
	SearchMerchants(ctx context.Context, category, search string, lat, lng float64) ([]*MerchantSearchResult, error)
	GetMerchantItems(ctx context.Context, merchantID, userID uuid.UUID) ([]*ItemResponse, error)

	// Task 3.3 — Food Order Creation, Status Update, Retrieval & History.
	CreateFoodOrder(ctx context.Context, req CreateFoodOrderRequest) (*CreateFoodOrderResponse, error)
	UpdateFoodOrderStatus(ctx context.Context, req UpdateFoodOrderStatusRequest) (*UpdateFoodOrderStatusResponse, error)
	GetFoodOrder(ctx context.Context, orderID, userID uuid.UUID) (*FoodOrder, error)
	GetFoodOrderItems(ctx context.Context, orderID uuid.UUID) ([]*FoodOrderItem, error)
	GetFoodOrderHistory(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*FoodOrder, int, error)
	GetMerchantOrders(ctx context.Context, userID, merchantID uuid.UUID, status string, page, pageSize int) ([]*FoodOrder, int, error)
}

// Handler menerima request HTTP dan memanggil Service.
type Handler struct {
	svc FoodService
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(svc FoodService) *Handler {
	return &Handler{svc: svc}
}

// merchantResponse memotong Merchant service ke field yang relevan untuk
// respons GET/PATCH merchant.
type merchantResponse struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"user_id"`
	MerchantName string          `json:"merchant_name"`
	Category     string          `json:"category"`
	Address      string          `json:"address"`
	Latitude     decimal.Decimal `json:"latitude"`
	Longitude    decimal.Decimal `json:"longitude"`
	Phone        *string         `json:"phone"`
	OpeningTime  *string         `json:"opening_time"`
	ClosingTime  *string         `json:"closing_time"`
	IsOpen       bool            `json:"is_open"`
	Status       string          `json:"status"`
	LogoURL      *string         `json:"logo_url"`
	AvgRating    decimal.Decimal `json:"avg_rating"`
	TotalReviews int             `json:"total_reviews"`
	TotalOrders  int             `json:"total_orders"`
}

func toMerchantResponse(m *Merchant) merchantResponse {
	return merchantResponse{
		ID:           m.ID,
		UserID:       m.UserID,
		MerchantName: m.Name,
		Category:     m.Category,
		Address:      m.Address,
		Latitude:     m.Latitude,
		Longitude:    m.Longitude,
		Phone:        m.Phone,
		OpeningTime:  m.OpeningTime,
		ClosingTime:  m.ClosingTime,
		IsOpen:       m.IsOpen,
		Status:       m.Status,
		LogoURL:      m.LogoURL,
		AvgRating:    m.AvgRating,
		TotalReviews: m.TotalReviews,
		TotalOrders:  m.TotalOrders,
	}
}

// registerMerchantRequestBody input JSON dari POST /merchants/register
// (ROADMAP 3.2.1). Kolom polos dari JWT (`user_id` tidak ada di body).
type registerMerchantRequestBody struct {
	MerchantName string  `json:"merchant_name" binding:"required"`
	Description  string  `json:"description"`
	Category     string  `json:"category"`
	Address      string  `json:"address"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Phone        string  `json:"phone"`
	OpeningTime  string  `json:"opening_time"`
	ClosingTime  string  `json:"closing_time"`
	LogoURL      string  `json:"logo_url"`
}

// RegisterMerchant POST /api/v1/merchants/register
// Auth: merchant (JWT). Membuat merchant profile + wallet MERCHANT dalam satu
// transaksi. user_id diambil dari JWT claim (diset oleh AuthMiddleware).
func (h *Handler) RegisterMerchant(c *gin.Context) {
	var body registerMerchantRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.RegisterMerchant(c.Request.Context(), RegisterMerchantRequest{
		UserID:              userID,
		MerchantName:        body.MerchantName,
		MerchantDescription: body.Description,
		Category:            body.Category,
		Address:             body.Address,
		Latitude:            body.Latitude,
		Longitude:           body.Longitude,
		Phone:               body.Phone,
		OpeningTime:         body.OpeningTime,
		ClosingTime:         body.ClosingTime,
		LogoURL:             body.LogoURL,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    resp,
	})
}

// GetMerchant GET /api/v1/merchants/:id
// Auth: merchant. Mengambil profil merchant publik oleh id.
func (h *Handler) GetMerchant(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	merchant, err := h.svc.GetMerchant(c.Request.Context(), merchantID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    toMerchantResponse(merchant),
	})
}

// updateMerchantRequestBody input JSON dari PATCH /merchants/{id}
// (partial update; hanya field yang dikirim yang diubah).
type updateMerchantRequestBody struct {
	MerchantName *string `json:"merchant_name"`
	Category     *string `json:"category"`
	OpeningTime  *string `json:"opening_time"`
	ClosingTime  *string `json:"closing_time"`
	IsOpen       *bool   `json:"is_open"`
	LogoURL      *string `json:"logo_url"`
}

// UpdateMerchant PATCH /api/v1/merchants/:id
// Auth: merchant & pemilik. Memperbarui info profil merchant.
func (h *Handler) UpdateMerchant(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	var body updateMerchantRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	merchant, err := h.svc.UpdateMerchant(c.Request.Context(), UpdateMerchantRequest{
		UserID:       userID,
		MerchantID:   merchantID,
		MerchantName: body.MerchantName,
		Category:     body.Category,
		OpeningTime:  body.OpeningTime,
		ClosingTime:  body.ClosingTime,
		IsOpen:       body.IsOpen,
		LogoURL:      body.LogoURL,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    toMerchantResponse(merchant),
	})
}

// createMenuRequestBody input JSON dari POST /merchants/{id}/menus.
type createMenuRequestBody struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	SequenceOrder int    `json:"sequence_order"`
}

// CreateMenu POST /api/v1/merchants/:id/menus
// Auth: merchant & pemilik.
func (h *Handler) CreateMenu(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	var body createMenuRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	menu, err := h.svc.CreateMenu(c.Request.Context(), CreateMenuRequest{
		UserID:        userID,
		MerchantID:    merchantID,
		Name:          body.Name,
		Description:   body.Description,
		SequenceOrder: body.SequenceOrder,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": menu})
}

// UpdateMenu PATCH /api/v1/merchants/:id/menus/:menu_id
// Auth: merchant & pemilik.
func (h *Handler) UpdateMenu(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}
	menuID, err := uuid.Parse(c.Param("menu_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MENU_ID", "invalid menu_id")
		return
	}

	var body struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		SequenceOrder *int    `json:"sequence_order"`
		IsActive      *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	menu, err := h.svc.UpdateMenu(c.Request.Context(), UpdateMenuRequest{
		UserID:        userID,
		MerchantID:    merchantID,
		MenuID:        menuID,
		Name:          body.Name,
		Description:   body.Description,
		SequenceOrder: body.SequenceOrder,
		IsActive:      body.IsActive,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": menu})
}

// DeleteMenu DELETE /api/v1/merchants/:id/menus/:menu_id
// Auth: merchant & pemilik.
func (h *Handler) DeleteMenu(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}
	menuID, err := uuid.Parse(c.Param("menu_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MENU_ID", "invalid menu_id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	if err := h.svc.DeleteMenu(c.Request.Context(), merchantID, menuID, userID); err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"menu_id": menuID, "deleted": true}})
}

// GetMenus GET /api/v1/merchants/:id/menus
// Auth: merchant & pemilik.
func (h *Handler) GetMenus(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	menus, err := h.svc.GetMenus(c.Request.Context(), merchantID, userID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": menus})
}

// createItemRequestBody input JSON dari POST /merchants/{id}/items
// (ROADMAP 3.2.3 POST /merchants/{id}/items).
type createItemRequestBody struct {
	MenuID      string  `json:"menu_id" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
	Stock       int     `json:"stock"`
	IsAvailable bool    `json:"is_available"`
}

// CreateItem POST /api/v1/merchants/:id/items
// Auth: merchant & pemilik.
func (h *Handler) CreateItem(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	var body createItemRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	menuID, err := uuid.Parse(body.MenuID)
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MENU_ID", "invalid menu_id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	item, err := h.svc.CreateItem(c.Request.Context(), CreateItemRequest{
		UserID:      userID,
		MerchantID:  merchantID,
		MenuID:      menuID,
		Name:        body.Name,
		Description: body.Description,
		Price:       decimal.NewFromFloat(body.Price),
		ImageURL:    body.ImageURL,
		Stock:       body.Stock,
		IsAvailable: body.IsAvailable,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
}

// UpdateItem PATCH /api/v1/merchants/:id/items/:item_id
// Auth: merchant & pemilik.
func (h *Handler) UpdateItem(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}
	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ITEM_ID", "invalid item_id")
		return
	}

	var body struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Price       *float64 `json:"price"`
		ImageURL    *string  `json:"image_url"`
		Stock       *int     `json:"stock"`
		IsAvailable *bool    `json:"is_available"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	var price *decimal.Decimal
	if body.Price != nil {
		p := decimal.NewFromFloat(*body.Price)
		price = &p
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	item, err := h.svc.UpdateItem(c.Request.Context(), UpdateItemRequest{
		UserID:      userID,
		MerchantID:  merchantID,
		ItemID:      itemID,
		Name:        body.Name,
		Description: body.Description,
		Price:       price,
		ImageURL:    body.ImageURL,
		Stock:       body.Stock,
		IsAvailable: body.IsAvailable,
	})
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// DeleteItem DELETE /api/v1/merchants/:id/items/:item_id
// Auth: merchant & pemilik.
func (h *Handler) DeleteItem(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}
	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ITEM_ID", "invalid item_id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	if err := h.svc.DeleteItem(c.Request.Context(), merchantID, itemID, userID); err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"item_id": itemID, "deleted": true}})
}

// GetItems GET /api/v1/merchants/:id/items
// Auth: merchant & pemilik.
func (h *Handler) GetItems(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	items, err := h.svc.GetItems(c.Request.Context(), merchantID, userID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// ---- Task 3.3: Catalog Discovery & Food Orders ----

// GetMerchants GET /api/v1/merchants
// Auth: optional. Catalog discovery (ROADMAP 3.3.1): filter category/search
// query param, dan sortir asc by distance bila lat/lng diberikan. Hanya
// merchant ACTIVE yang dikembalikan.
func (h *Handler) GetMerchants(c *gin.Context) {
	category := c.Query("category")
	search := c.Query("search")

	lat, _ := strconv.ParseFloat(c.Query("lat"), 64)
	lng, _ := strconv.ParseFloat(c.Query("lng"), 64)

	results, err := h.svc.SearchMerchants(c.Request.Context(), category, search, lat, lng)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	type searchEntry struct {
		merchantResponse
		DistanceKM *decimal.Decimal `json:"distance_km,omitempty"`
	}
	out := make([]searchEntry, 0, len(results))
	for _, res := range results {
		e := searchEntry{merchantResponse: toMerchantResponse(res.Merchant)}
		if res.DistanceKm != nil {
			e.DistanceKM = res.DistanceKm
		}
		out = append(out, e)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

// GetMerchantItems GET /api/v1/merchants/:id/items
// Auth: optional. Catalog retrieval (ROADMAP 3.3.2). Pemilik merchant (token
// valid) melihat seluruh item termasuk yang tidak available; selainnya hanya
// item is_available=TRUE dari merchant ACTIVE.
func (h *Handler) GetMerchantItems(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

	userID, _ := userIDFromContext(c)

	items, err := h.svc.GetMerchantItems(c.Request.Context(), merchantID, userID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// createFoodOrderRequestBody input JSON dari POST /food-orders (ROADMAP 3.3.3).
// Keranjang (items) wajib minimal satu item; opsi item disimpan apa adanya.
type createFoodOrderRequestBody struct {
	MerchantID      uuid.UUID  `json:"merchant_id" binding:"required"`
	DeliveryAddress string     `json:"delivery_address"`
	DeliveryLat     float64    `json:"delivery_lat"`
	DeliveryLng     float64    `json:"delivery_lng"`
	PaymentMethod   string     `json:"payment_method" binding:"required"`
	VoucherID       *uuid.UUID `json:"voucher_id"`
	Items           []struct {
		ItemID              uuid.UUID         `json:"item_id" binding:"required"`
		Quantity            int               `json:"quantity"`
		Options             map[string]string `json:"options"`
		SpecialInstructions string            `json:"special_instructions"`
	} `json:"items" binding:"required,min=1,dive"`
}

// CreateFoodOrder POST /api/v1/food-orders
// Auth: customer. Idempotency key wajib di header X-Idempotency-Key; request
// dengan key yang sama mengembalikan response pertama (L1 Redis / L2 DB).
func (h *Handler) CreateFoodOrder(c *gin.Context) {
	key := c.GetHeader("X-Idempotency-Key")
	if key == "" {
		writeError(c, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REQUIRED", "X-Idempotency-Key header is required")
		return
	}

	var body createFoodOrderRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	req := CreateFoodOrderRequest{
		UserID:          userID,
		MerchantID:      body.MerchantID,
		DeliveryAddress: body.DeliveryAddress,
		DeliveryLat:     body.DeliveryLat,
		DeliveryLng:     body.DeliveryLng,
		PaymentMethod:   strings.ToUpper(body.PaymentMethod),
		VoucherID:       body.VoucherID,
		IdempotencyKey:  key,
	}
	for _, it := range body.Items {
		req.Items = append(req.Items, FoodOrderItemRequest{
			ItemID:              it.ItemID,
			Quantity:            it.Quantity,
			Options:             it.Options,
			SpecialInstructions: it.SpecialInstructions,
		})
	}

	resp, err := h.svc.CreateFoodOrder(c.Request.Context(), req)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

// updateFoodOrderStatusRequestBody input JSON dari PATCH /food-orders/{id}
// (ROADMAP 3.3.4).
type updateFoodOrderStatusRequestBody struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

// UpdateFoodOrderStatus PATCH /api/v1/food-orders/:id
// Auth: customer (pemilik), merchant (owner), driver (tertunjuk).
// Transisi status menggunakan FOR UPDATE NOWAIT + audit trail.
func (h *Handler) UpdateFoodOrderStatus(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	var body updateFoodOrderStatusRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	resp, err := h.svc.UpdateFoodOrderStatus(c.Request.Context(), UpdateFoodOrderStatusRequest{
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

// GetFoodOrder GET /api/v1/food-orders/:id
// Auth: customer (pemilik), merchant (owner), driver (tertunjuk). Mengembalikan
// detail order + item-nya.
func (h *Handler) GetFoodOrder(c *gin.Context) {
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

	order, err := h.svc.GetFoodOrder(c.Request.Context(), orderID, userID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	items, err := h.svc.GetFoodOrderItems(c.Request.Context(), orderID)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"order": order,
			"items": items,
		},
	})
}

// GetFoodOrderHistory GET /api/v1/food-orders?page=&page_size=
// Auth: customer. Riwayat food order customer dengan pagination (urutan
// terbaru; default page=1, page_size=20, maks 50).
func (h *Handler) GetFoodOrderHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PAGE_SIZE", "invalid page_size")
		return
	}

	userID, ok := userIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user identity")
		return
	}

	orders, total, err := h.svc.GetFoodOrderHistory(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		writeError(c, statusForError(err), codeForError(err), err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": orders,
			"pagination": gin.H{
				"page":      page,
				"page_size": pageSize,
				"total":     total,
			},
		},
	})
}

// GetMerchantOrders GET /api/v1/merchants/:id/orders
// Auth: merchant (RBAC di route). Daftar food order milik merchant dengan
// pagination (default page=1, page_size=20, maks 50; `limit` alias dari
// `page_size`), urut created_at DESC. Filter status opsional (merchant_status
// ATAU status order). Hanya merchant pemilik yang bisa mengakses — ownership
// divalidasi di Service (GetMerchantByUserID + cocokkan ID merch).
// Error: 400 invalid page/page_size, 403 bukan pemilik, 404 merchant tidak ada.
func (h *Handler) GetMerchantOrders(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_MERCHANT_ID", "invalid merchant id")
		return
	}

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

	status := strings.ToUpper(c.Query("status"))

	orders, total, err := h.svc.GetMerchantOrders(c.Request.Context(), userID, merchantID, status, page, pageSize)
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
	case errors.Is(err, ErrMerchantNotFound),
		errors.Is(err, ErrMenuNotFound),
		errors.Is(err, ErrItemNotFound),
		errors.Is(err, ErrFoodOrderNotFound),
		errors.Is(err, ErrDriverNotFound),
		errors.Is(err, ErrWalletNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrNotMerchant),
		errors.Is(err, ErrNotMerchantOwner),
		errors.Is(err, ErrNotAllowed),
		errors.Is(err, ErrNotCustomer):
		return http.StatusForbidden
	case errors.Is(err, ErrMerchantAlreadyExists),
		errors.Is(err, ErrIdempotencyInProgress),
		errors.Is(err, ErrInvalidCachedResponse),
		errors.Is(err, ErrInvalidTransition),
		errors.Is(err, ErrLockTimeout):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidStatus):
		return http.StatusBadRequest
	case errors.Is(err, ErrMerchantInactive),
		errors.Is(err, ErrInvalidCoordinates),
		errors.Is(err, ErrInvalidMenu),
		errors.Is(err, ErrInvalidPrice),
		errors.Is(err, ErrEmptyName),
		errors.Is(err, ErrCustomerInactive),
		errors.Is(err, ErrOverdueDebt),
		errors.Is(err, ErrWalletInactive),
		errors.Is(err, ErrMerchantWalletNotFound),
		errors.Is(err, ErrInsufficientBalance),
		errors.Is(err, ErrInvalidPaymentMethod),
		errors.Is(err, ErrIdempotencyKeyRequired),
		errors.Is(err, ErrInvalidItem),
		errors.Is(err, ErrInsufficientStock),
		errors.Is(err, ErrEmptyItems),
		errors.Is(err, ErrInvalidDeliveryAddress):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// codeForError memetakan error service ke error code (API_CONTRACT 7.1).
func codeForError(err error) string {
	switch {
	case errors.Is(err, ErrMerchantNotFound):
		return "MERCHANT_NOT_FOUND"
	case errors.Is(err, ErrMenuNotFound):
		return "MENU_NOT_FOUND"
	case errors.Is(err, ErrItemNotFound):
		return "ITEM_NOT_FOUND"
	case errors.Is(err, ErrFoodOrderNotFound):
		return "FOOD_ORDER_NOT_FOUND"
	case errors.Is(err, ErrDriverNotFound):
		return "DRIVER_NOT_FOUND"
	case errors.Is(err, ErrWalletNotFound):
		return "WALLET_NOT_FOUND"
	case errors.Is(err, ErrNotMerchant):
		return "NOT_MERCHANT"
	case errors.Is(err, ErrNotMerchantOwner):
		return "FORBIDDEN"
	case errors.Is(err, ErrNotAllowed):
		return "FORBIDDEN"
	case errors.Is(err, ErrNotCustomer):
		return "NOT_CUSTOMER"
	case errors.Is(err, ErrMerchantInactive):
		return "MERCHANT_INACTIVE"
	case errors.Is(err, ErrMerchantAlreadyExists):
		return "MERCHANT_ALREADY_EXISTS"
	case errors.Is(err, ErrInvalidCoordinates):
		return "INVALID_COORDINATES"
	case errors.Is(err, ErrInvalidMenu):
		return "INVALID_MENU"
	case errors.Is(err, ErrInvalidPrice):
		return "INVALID_PRICE"
	case errors.Is(err, ErrEmptyName):
		return "INVALID_REQUEST"
	case errors.Is(err, ErrCustomerInactive):
		return "CUSTOMER_INACTIVE"
	case errors.Is(err, ErrOverdueDebt):
		return "OVERDUE_DEBT"
	case errors.Is(err, ErrWalletInactive):
		return "WALLET_INACTIVE"
	case errors.Is(err, ErrMerchantWalletNotFound):
		return "MERCHANT_WALLET_NOT_FOUND"
	case errors.Is(err, ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE"
	case errors.Is(err, ErrInvalidPaymentMethod):
		return "INVALID_PAYMENT_METHOD"
	case errors.Is(err, ErrIdempotencyKeyRequired):
		return "IDEMPOTENCY_KEY_REQUIRED"
	case errors.Is(err, ErrIdempotencyInProgress):
		return "IDEMPOTENCY_IN_PROGRESS"
	case errors.Is(err, ErrInvalidCachedResponse):
		return "IDEMPOTENCY_INVALID_CACHE"
	case errors.Is(err, ErrInvalidItem):
		return "INVALID_ITEM"
	case errors.Is(err, ErrInsufficientStock):
		return "INSUFFICIENT_STOCK"
	case errors.Is(err, ErrEmptyItems):
		return "EMPTY_ITEMS"
	case errors.Is(err, ErrInvalidDeliveryAddress):
		return "INVALID_DELIVERY_ADDRESS"
	case errors.Is(err, ErrInvalidStatus):
		return "INVALID_STATUS"
	case errors.Is(err, ErrInvalidTransition):
		return "INVALID_TRANSITION"
	case errors.Is(err, ErrLockTimeout):
		return "LOCK_TIMEOUT"
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
