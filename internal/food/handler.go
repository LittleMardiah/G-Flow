// Package food — HTTP Handler (Gin) untuk modul G-Food (Phase 3, Task 3.2:
// Merchant Onboarding & Catalog Management).
//
// Handler hanya memetakan request HTTP ke panggilan Service dan men-map error
// ke status code + error code sesuai API_CONTRACT 7.1. Tidak mengandung
// business logic (itu milik Service/Repository). Resource merchant hanya bisa
// dikelola oleh user dengan role merchant (RBAC di main.go).
package food

import (
	"context"
	"errors"
	"net/http"

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
	ID            uuid.UUID       `json:"id"`
	UserID        uuid.UUID       `json:"user_id"`
	MerchantName  string          `json:"merchant_name"`
	Category      string          `json:"category"`
	Address       string          `json:"address"`
	Latitude      decimal.Decimal `json:"latitude"`
	Longitude     decimal.Decimal `json:"longitude"`
	Phone         *string         `json:"phone"`
	OpeningTime   *string         `json:"opening_time"`
	ClosingTime   *string         `json:"closing_time"`
	IsOpen        bool            `json:"is_open"`
	Status        string          `json:"status"`
	LogoURL       *string         `json:"logo_url"`
	AvgRating     decimal.Decimal `json:"avg_rating"`
	TotalReviews  int             `json:"total_reviews"`
	TotalOrders   int             `json:"total_orders"`
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
		errors.Is(err, ErrItemNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrNotMerchant),
		errors.Is(err, ErrNotMerchantOwner):
		return http.StatusForbidden
	case errors.Is(err, ErrMerchantInactive),
		errors.Is(err, ErrInvalidCoordinates),
		errors.Is(err, ErrInvalidMenu),
		errors.Is(err, ErrInvalidPrice),
		errors.Is(err, ErrEmptyName):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrMerchantAlreadyExists):
		return http.StatusConflict
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
	case errors.Is(err, ErrNotMerchant):
		return "NOT_MERCHANT"
	case errors.Is(err, ErrNotMerchantOwner):
		return "FORBIDDEN"
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
