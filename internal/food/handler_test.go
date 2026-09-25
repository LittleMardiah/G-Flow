package food

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var fMenuID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

type mockFoodService struct {
	mock.Mock
}

func (m *mockFoodService) RegisterMerchant(ctx context.Context, req RegisterMerchantRequest) (*RegisterMerchantResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RegisterMerchantResponse), args.Error(1)
}

func (m *mockFoodService) GetMerchant(ctx context.Context, merchantID uuid.UUID) (*Merchant, error) {
	args := m.Called(ctx, merchantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Merchant), args.Error(1)
}

func (m *mockFoodService) GetMerchantForUser(ctx context.Context, userID uuid.UUID) (*Merchant, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Merchant), args.Error(1)
}

func (m *mockFoodService) UpdateMerchant(ctx context.Context, req UpdateMerchantRequest) (*Merchant, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Merchant), args.Error(1)
}

func (m *mockFoodService) CreateMenu(ctx context.Context, req CreateMenuRequest) (*MenuResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MenuResponse), args.Error(1)
}

func (m *mockFoodService) UpdateMenu(ctx context.Context, req UpdateMenuRequest) (*MenuResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MenuResponse), args.Error(1)
}

func (m *mockFoodService) DeleteMenu(ctx context.Context, merchantID, menuID, userID uuid.UUID) error {
	args := m.Called(ctx, merchantID, menuID, userID)
	return args.Error(0)
}

func (m *mockFoodService) GetMenus(ctx context.Context, merchantID, userID uuid.UUID) ([]*MenuResponse, error) {
	args := m.Called(ctx, merchantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*MenuResponse), args.Error(1)
}

func (m *mockFoodService) CreateItem(ctx context.Context, req CreateItemRequest) (*ItemResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ItemResponse), args.Error(1)
}

func (m *mockFoodService) UpdateItem(ctx context.Context, req UpdateItemRequest) (*ItemResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ItemResponse), args.Error(1)
}

func (m *mockFoodService) DeleteItem(ctx context.Context, merchantID, itemID, userID uuid.UUID) error {
	args := m.Called(ctx, merchantID, itemID, userID)
	return args.Error(0)
}

func (m *mockFoodService) GetItems(ctx context.Context, merchantID, userID uuid.UUID) ([]*ItemResponse, error) {
	args := m.Called(ctx, merchantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ItemResponse), args.Error(1)
}

func (m *mockFoodService) SearchMerchants(ctx context.Context, category, search string, lat, lng float64) ([]*MerchantSearchResult, error) {
	args := m.Called(ctx, category, search, lat, lng)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*MerchantSearchResult), args.Error(1)
}

func (m *mockFoodService) GetMerchantItems(ctx context.Context, merchantID, userID uuid.UUID) ([]*ItemResponse, error) {
	args := m.Called(ctx, merchantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ItemResponse), args.Error(1)
}

func (m *mockFoodService) CreateFoodOrder(ctx context.Context, req CreateFoodOrderRequest) (*CreateFoodOrderResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CreateFoodOrderResponse), args.Error(1)
}

func (m *mockFoodService) UpdateFoodOrderStatus(ctx context.Context, req UpdateFoodOrderStatusRequest) (*UpdateFoodOrderStatusResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UpdateFoodOrderStatusResponse), args.Error(1)
}

func (m *mockFoodService) GetFoodOrder(ctx context.Context, orderID, userID uuid.UUID) (*FoodOrder, error) {
	args := m.Called(ctx, orderID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FoodOrder), args.Error(1)
}

func (m *mockFoodService) GetFoodOrderItems(ctx context.Context, orderID uuid.UUID) ([]*FoodOrderItem, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FoodOrderItem), args.Error(1)
}

func (m *mockFoodService) GetFoodOrderHistory(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*FoodOrder, int, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*FoodOrder), args.Int(1), args.Error(2)
}

func (m *mockFoodService) GetMerchantOrders(ctx context.Context, userID, merchantID uuid.UUID, status string, page, pageSize int) ([]*FoodOrder, int, error) {
	args := m.Called(ctx, userID, merchantID, status, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*FoodOrder), args.Int(1), args.Error(2)
}

func (m *mockFoodService) AcceptFoodOrder(ctx context.Context, req AcceptFoodOrderRequest) (*AcceptFoodOrderResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AcceptFoodOrderResponse), args.Error(1)
}

func newFoodHandlerCtx(t *testing.T, method, target, body string, params ...gin.Param) (*mockFoodService, *gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = params
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	return new(mockFoodService), c, w
}

func TestHandler_RegisterMerchant(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/register",
		`{"merchant_name":"Warung","category":"food","address":"Jl. B"}`)
	c.Set("user_id", fCustID.String())

	svc.On("RegisterMerchant", mock.Anything, mock.MatchedBy(func(r RegisterMerchantRequest) bool {
		return r.UserID == fCustID && r.MerchantName == "Warung"
	})).Return(&RegisterMerchantResponse{ID: fMerchID, MerchantName: "Warung", Status: merchantStatusWaiting, WalletID: fMerchWallet}, nil)

	h := NewHandler(svc)
	h.RegisterMerchant(c)
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/register", `{"merchant_name":"Warung"}`)
	c2.Set("user_id", fCustID.String())
	svc2.On("RegisterMerchant", mock.Anything, mock.Anything).Return(nil, ErrMerchantAlreadyExists)
	h2 := NewHandler(svc2)
	h2.RegisterMerchant(c2)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestHandler_RegisterMerchant_Errors(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/register",
		`{"merchant_name":"x"}`)
	h := NewHandler(svc)
	h.RegisterMerchant(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/register", `{bad json`)
	h2 := NewHandler(new(mockFoodService))
	h2.RegisterMerchant(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestHandler_GetMerchant(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String(), "", gin.Param{Key: "id", Value: fMerchID.String()})
	m := &Merchant{ID: fMerchID, UserID: fCustID, Name: "Warung", Category: "food",
		Latitude: decimal.NewFromFloat(-6.2), Longitude: decimal.NewFromFloat(106.8), Status: merchantStatusWaiting}
	svc.On("GetMerchant", mock.Anything, fMerchID).Return(m, nil)
	h := NewHandler(svc)
	h.GetMerchant(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/notauuid", "", gin.Param{Key: "id", Value: "notauuid"})
	h2 := NewHandler(new(mockFoodService))
	h2.GetMerchant(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String(), "", gin.Param{Key: "id", Value: fMerchID.String()})
	svc3.On("GetMerchant", mock.Anything, fMerchID).Return(nil, ErrMerchantNotFound)
	h3 := NewHandler(svc3)
	h3.GetMerchant(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_GetMerchantMe(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/me", "")
	c.Set("user_id", fCustID.String())
	m := &Merchant{ID: fMerchID, UserID: fCustID, Name: "Warung", Category: "food",
		Latitude: decimal.NewFromFloat(-6.2), Longitude: decimal.NewFromFloat(106.8), Status: merchantStatusWaiting}
	svc.On("GetMerchantForUser", mock.Anything, fCustID).Return(m, nil)
	h := NewHandler(svc)
	h.GetMerchantMe(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/me", "")
	h2 := NewHandler(new(mockFoodService))
	h2.GetMerchantMe(c2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/me", "")
	c3.Set("user_id", fCustID.String())
	svc3.On("GetMerchantForUser", mock.Anything, fCustID).Return(nil, ErrMerchantNotFound)
	h3 := NewHandler(svc3)
	h3.GetMerchantMe(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_UpdateMerchant(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String(),
		`{"merchant_name":"Warung Baru","is_open":true}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", fCustID.String())
	open := true
	m := &Merchant{ID: fMerchID, UserID: fCustID, Name: "Warung Baru", Category: "food", Status: merchantStatusWaiting, IsOpen: open}
	svc.On("UpdateMerchant", mock.Anything, mock.MatchedBy(func(r UpdateMerchantRequest) bool {
		return r.MerchantID == fMerchID && r.UserID == fCustID && r.MerchantName != nil && *r.MerchantName == "Warung Baru"
	})).Return(m, nil)
	h := NewHandler(svc)
	h.UpdateMerchant(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String(), `{"merchant_name":"x"}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c2.Set("user_id", fCustID.String())
	svc2.On("UpdateMerchant", mock.Anything, mock.Anything).Return(nil, ErrNotMerchantOwner)
	h2 := NewHandler(svc2)
	h2.UpdateMerchant(c2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}

func TestHandler_Menus(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/menus",
		`{"name":"Menu A","description":"desc"}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", fCustID.String())
	desc := "desc"
	svc.On("CreateMenu", mock.Anything, mock.MatchedBy(func(r CreateMenuRequest) bool {
		return r.MerchantID == fMerchID && r.Name == "Menu A"
	})).Return(&MenuResponse{ID: fMenuID, MerchantID: fMerchID, Name: "Menu A", Description: &desc, IsActive: true}, nil)
	h := NewHandler(svc)
	h.CreateMenu(c)
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/menus", `{}`, gin.Param{Key: "id", Value: fMerchID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.CreateMenu(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestHandler_UpdateMenu(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPatch,
		"/api/v1/merchants/"+fMerchID.String()+"/menus/"+fMenuID.String(), `{"name":"Menu B"}`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("UpdateMenu", mock.Anything, mock.MatchedBy(func(r UpdateMenuRequest) bool {
		return r.MerchantID == fMerchID && r.MenuID == fMenuID && r.Name != nil && *r.Name == "Menu B"
	})).Return(&MenuResponse{ID: fMenuID, MerchantID: fMerchID, Name: "Menu B", IsActive: true}, nil)
	h := NewHandler(svc)
	h.UpdateMenu(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_DeleteMenu(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodDelete,
		"/api/v1/merchants/"+fMerchID.String()+"/menus/"+fMenuID.String(), "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("DeleteMenu", mock.Anything, fMerchID, fMenuID, fCustID).Return(nil)
	h := NewHandler(svc)
	h.DeleteMenu(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetMenus(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/menus", "", gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("GetMenus", mock.Anything, fMerchID, fCustID).Return([]*MenuResponse{{ID: fMenuID, MerchantID: fMerchID, Name: "Menu A", IsActive: true}}, nil)
	h := NewHandler(svc)
	h.GetMenus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_Items(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/items",
		`{"menu_id":"`+fMenuID.String()+`","name":"Nasi","price":50000,"stock":10}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("CreateItem", mock.Anything, mock.MatchedBy(func(r CreateItemRequest) bool {
		return r.MerchantID == fMerchID && r.MenuID == fMenuID && r.Name == "Nasi"
	})).Return(&ItemResponse{ID: fItemID, MenuID: fMenuID, MerchantID: fMerchID, Name: "Nasi", Price: decimal.NewFromInt(50000), Stock: 10, IsAvailable: true}, nil)
	h := NewHandler(svc)
	h.CreateItem(c)
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/items",
		`{"menu_id":"bad","name":"x"}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c2.Set("user_id", fCustID.String())
	svc2.On("CreateItem", mock.Anything, mock.Anything).Return(nil, ErrInvalidItem)
	h2 := NewHandler(svc2)
	h2.CreateItem(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)
}

func TestHandler_UpdateItem(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPatch,
		"/api/v1/merchants/"+fMerchID.String()+"/items/"+fItemID.String(), `{"name":"Nasi Goreng","price":55000}`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: fItemID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("UpdateItem", mock.Anything, mock.MatchedBy(func(r UpdateItemRequest) bool {
		return r.MerchantID == fMerchID && r.ItemID == fItemID && r.Name != nil && r.Price != nil
	})).Return(&ItemResponse{ID: fItemID, MerchantID: fMerchID, Name: "Nasi Goreng", Price: decimal.NewFromInt(55000), IsAvailable: true}, nil)
	h := NewHandler(svc)
	h.UpdateItem(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_DeleteItem(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodDelete,
		"/api/v1/merchants/"+fMerchID.String()+"/items/"+fItemID.String(), "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: fItemID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("DeleteItem", mock.Anything, fMerchID, fItemID, fCustID).Return(nil)
	h := NewHandler(svc)
	h.DeleteItem(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodDelete,
		"/api/v1/merchants/"+fMerchID.String()+"/items/"+fItemID.String(), "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: fItemID.String()})
	svc2.On("DeleteItem", mock.Anything, fMerchID, fItemID, fCustID).Return(ErrItemNotFound)
	c2.Set("user_id", fCustID.String())
	h2 := NewHandler(svc2)
	h2.DeleteItem(c2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestHandler_GetItems(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/items", "", gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("GetItems", mock.Anything, fMerchID, fCustID).Return([]*ItemResponse{{ID: fItemID, MerchantID: fMerchID, Name: "Nasi", IsAvailable: true}}, nil)
	h := NewHandler(svc)
	h.GetItems(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetMerchants_Discovery(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants?category=food&lat=-6.2&lng=106.8", "")
	dist := decimal.NewFromFloat(1.5)
	svc.On("SearchMerchants", mock.Anything, "food", "", -6.2, 106.8).Return([]*MerchantSearchResult{
		{Merchant: &Merchant{ID: fMerchID, Name: "Warung", Category: "food", Status: merchantStatusConfirmed}, DistanceKm: &dist},
	}, nil)
	h := NewHandler(svc)
	h.GetMerchants(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants", "")
	svc2.On("SearchMerchants", mock.Anything, "", "", 0.0, 0.0).Return(nil, errors.New("internal"))
	h2 := NewHandler(svc2)
	h2.GetMerchants(c2)
	assert.Equal(t, http.StatusInternalServerError, w2.Code)
}

func TestHandler_GetMerchantItems(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/items", "", gin.Param{Key: "id", Value: fMerchID.String()})
	svc.On("GetMerchantItems", mock.Anything, fMerchID, uuid.Nil).Return([]*ItemResponse{{ID: fItemID, MerchantID: fMerchID, Name: "Nasi", IsAvailable: true}}, nil)
	h := NewHandler(svc)
	h.GetMerchantItems(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_CreateFoodOrder(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders",
		`{"merchant_id":"`+fMerchID.String()+`","payment_method":"wallet","delivery_address":"Jl. B","items":[{"item_id":"`+fItemID.String()+`","quantity":2}]}`)
	c.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c.Set("user_id", fCustID.String())
	svc.On("CreateFoodOrder", mock.Anything, mock.MatchedBy(func(r CreateFoodOrderRequest) bool {
		return r.MerchantID == fMerchID && r.UserID == fCustID && r.IdempotencyKey == "idem-1" && len(r.Items) == 1
	})).Return(&CreateFoodOrderResponse{
		ID: fOrderID, Status: foodStatusCreated, ItemSubtotal: decimal.NewFromInt(100000),
		DeliveryFee: decimal.NewFromInt(20000), TotalAmount: decimal.NewFromInt(120000),
	}, nil)
	h := NewHandler(svc)
	h.CreateFoodOrder(c)
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_CreateFoodOrder_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders", `{}`)
	h := NewHandler(new(mockFoodService))
	h.CreateFoodOrder(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders",
		`{"merchant_id":"`+fMerchID.String()+`","payment_method":"wallet","items":[]}`)
	c2.Request.Header.Set("X-Idempotency-Key", "idem-1")
	h2 := NewHandler(new(mockFoodService))
	h2.CreateFoodOrder(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders",
		`{"merchant_id":"`+fMerchID.String()+`","payment_method":"wallet","items":[{"item_id":"`+fItemID.String()+`"}]}`)
	c3.Request.Header.Set("X-Idempotency-Key", "idem-1")
	c3.Set("user_id", fCustID.String())
	svc3.On("CreateFoodOrder", mock.Anything, mock.Anything).Return(nil, ErrInsufficientStock)
	h3 := NewHandler(svc3)
	h3.CreateFoodOrder(c3)
	assert.Equal(t, http.StatusUnprocessableEntity, w3.Code)
}

func TestHandler_UpdateFoodOrderStatus(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{"status":"confirmed"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c.Set("user_id", fMerchID.String())
	svc.On("UpdateFoodOrderStatus", mock.Anything, mock.MatchedBy(func(r UpdateFoodOrderStatusRequest) bool {
		return r.OrderID == fOrderID && r.Status == "CONFIRMED"
	})).Return(&UpdateFoodOrderStatusResponse{OrderID: fOrderID, Status: foodStatusConfirmed}, nil)
	h := NewHandler(svc)
	h.UpdateFoodOrderStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/bad", `{"status":"x"}`, gin.Param{Key: "id", Value: "bad"})
	c2.Set("user_id", fMerchID.String())
	h2 := NewHandler(new(mockFoodService))
	h2.UpdateFoodOrderStatus(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{"status":"bad"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c3.Set("user_id", fMerchID.String())
	svc3.On("UpdateFoodOrderStatus", mock.Anything, mock.Anything).Return(nil, ErrInvalidTransition)
	h3 := NewHandler(svc3)
	h3.UpdateFoodOrderStatus(c3)
	assert.Equal(t, http.StatusConflict, w3.Code)
}

func TestHandler_UpdateFoodOrderStatus_MerchantWallet(t *testing.T) {
	// Merchant confirm order WALLET (born CONFIRMED/WAITING) → 200.
	svc, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{"status":"confirmed"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c.Set("user_id", fMerchID.String())
	svc.On("UpdateFoodOrderStatus", mock.Anything, mock.MatchedBy(func(r UpdateFoodOrderStatusRequest) bool {
		return r.OrderID == fOrderID && r.Status == "CONFIRMED"
	})).Return(&UpdateFoodOrderStatusResponse{OrderID: fOrderID, Status: foodStatusConfirmed}, nil)
	h := NewHandler(svc)
	h.UpdateFoodOrderStatus(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	// Merchant reject order WALLET → CANCELLED 200.
	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{"status":"cancelled"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c2.Set("user_id", fMerchID.String())
	svc2.On("UpdateFoodOrderStatus", mock.Anything, mock.MatchedBy(func(r UpdateFoodOrderStatusRequest) bool {
		return r.OrderID == fOrderID && r.Status == "CANCELLED"
	})).Return(&UpdateFoodOrderStatusResponse{OrderID: fOrderID, Status: foodStatusCancelled}, nil)
	h2 := NewHandler(svc2)
	h2.UpdateFoodOrderStatus(c2)
	assert.Equal(t, http.StatusOK, w2.Code)
	svc2.AssertExpectations(t)

	// Merchant skip confirm (PREPARING langsung saat merchant_status WAITING) → 409.
	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{"status":"preparing"}`, gin.Param{Key: "id", Value: fOrderID.String()})
	c3.Set("user_id", fMerchID.String())
	svc3.On("UpdateFoodOrderStatus", mock.Anything, mock.MatchedBy(func(r UpdateFoodOrderStatusRequest) bool {
		return r.OrderID == fOrderID && r.Status == "PREPARING"
	})).Return(nil, ErrInvalidTransition)
	h3 := NewHandler(svc3)
	h3.UpdateFoodOrderStatus(c3)
	assert.Equal(t, http.StatusConflict, w3.Code)
	svc3.AssertExpectations(t)
}

func TestHandler_GetFoodOrder(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("GetFoodOrder", mock.Anything, fOrderID, fCustID).Return(&FoodOrder{ID: fOrderID, CustomerID: fCustID, MerchantID: fMerchID, Status: foodStatusCreated}, nil)
	svc.On("GetFoodOrderItems", mock.Anything, fOrderID).Return([]*FoodOrderItem{{OrderID: fOrderID, ItemID: fItemID, ItemName: "Nasi"}}, nil)
	h := NewHandler(svc)
	h.GetFoodOrder(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetFoodOrderHistory(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders?page=1&page_size=10", "")
	c.Set("user_id", fCustID.String())
	svc.On("GetFoodOrderHistory", mock.Anything, fCustID, 1, 10).Return([]*FoodOrder{{ID: fOrderID, Status: foodStatusCreated}}, 1, nil)
	h := NewHandler(svc)
	h.GetFoodOrderHistory(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders?page=1&page_size=abc", "")
	c2.Set("user_id", fCustID.String())
	h2 := NewHandler(svc2)
	h2.GetFoodOrderHistory(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)
}

func TestHandler_GetMerchantOrders(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders?page=1&limit=10&status=waiting", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", fCustID.String())
	svc.On("GetMerchantOrders", mock.Anything, fCustID, fMerchID, "WAITING", 1, 10).
		Return([]*FoodOrder{{ID: fOrderID, MerchantID: fMerchID, Status: foodStatusCreated, Items: []FoodOrderItem{{ID: fItemID, OrderID: fOrderID, ItemID: fItemID, ItemName: "Nasi"}}}}, 1, nil)
	h := NewHandler(svc)
	h.GetMerchantOrders(c)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"items\"")
	svc.AssertExpectations(t)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	c2.Set("user_id", fCustID.String())
	svc2.On("GetMerchantOrders", mock.Anything, fCustID, fMerchID, "", 1, 20).
		Return([]*FoodOrder{}, 0, nil)
	h2 := NewHandler(svc2)
	h2.GetMerchantOrders(c2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestHandler_GetMerchantOrders_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	h := NewHandler(new(mockFoodService))
	h.GetMerchantOrders(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders?page=abc", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	c2.Set("user_id", fCustID.String())
	h2 := NewHandler(svc2)
	h2.GetMerchantOrders(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders?page_size=abc", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	c3.Set("user_id", fCustID.String())
	h3 := NewHandler(svc3)
	h3.GetMerchantOrders(c3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)

	svc4, c4, w4 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	c4.Set("user_id", fCustID.String())
	svc4.On("GetMerchantOrders", mock.Anything, fCustID, fMerchID, "", 1, 20).Return(nil, 0, ErrNotMerchantOwner)
	h4 := NewHandler(svc4)
	h4.GetMerchantOrders(c4)
	assert.Equal(t, http.StatusForbidden, w4.Code)

	svc5, c5, w5 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/orders", "",
		gin.Param{Key: "id", Value: fMerchID.String()})
	c5.Set("user_id", fCustID.String())
	svc5.On("GetMerchantOrders", mock.Anything, fCustID, fMerchID, "", 1, 20).Return(nil, 0, ErrMerchantNotFound)
	h5 := NewHandler(svc5)
	h5.GetMerchantOrders(c5)
	assert.Equal(t, http.StatusNotFound, w5.Code)
}

func TestHandler_StatusForError(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{ErrMerchantNotFound, http.StatusNotFound},
		{ErrMenuNotFound, http.StatusNotFound},
		{ErrItemNotFound, http.StatusNotFound},
		{ErrFoodOrderNotFound, http.StatusNotFound},
		{ErrWalletNotFound, http.StatusNotFound},
		{ErrNotMerchant, http.StatusForbidden},
		{ErrNotCustomer, http.StatusForbidden},
		{ErrMerchantAlreadyExists, http.StatusConflict},
		{ErrIdempotencyInProgress, http.StatusConflict},
		{ErrInvalidTransition, http.StatusConflict},
		{ErrLockTimeout, http.StatusConflict},
		{ErrInvalidStatus, http.StatusBadRequest},
		{ErrEmptyName, http.StatusUnprocessableEntity},
		{ErrInvalidPrice, http.StatusUnprocessableEntity},
		{ErrInsufficientBalance, http.StatusUnprocessableEntity},
		{ErrInvalidDeliveryAddress, http.StatusUnprocessableEntity},
		{ErrEmptyItems, http.StatusUnprocessableEntity},
		{errors.New("other"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.code, statusForError(tc.err))
	}
}

func TestHandler_CodeForError(t *testing.T) {
	assert.Equal(t, "MERCHANT_NOT_FOUND", codeForError(ErrMerchantNotFound))
	assert.Equal(t, "FOOD_ORDER_NOT_FOUND", codeForError(ErrFoodOrderNotFound))
	assert.Equal(t, "NOT_CUSTOMER", codeForError(ErrNotCustomer))
	assert.Equal(t, "INVALID_TRANSITION", codeForError(ErrInvalidTransition))
	assert.Equal(t, "INSUFFICIENT_STOCK", codeForError(ErrInsufficientStock))
	assert.Equal(t, "IDEMPOTENCY_KEY_REQUIRED", codeForError(ErrIdempotencyKeyRequired))
	assert.Equal(t, "LOCK_TIMEOUT", codeForError(ErrLockTimeout))
	assert.Equal(t, "INTERNAL_SERVER_ERROR", codeForError(errors.New("other")))
}

// ---- AcceptFoodOrder (TD-078) ----

func acceptOrderCtx(t *testing.T, orderID string) (*mockFoodService, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders/"+orderID+"/accept", "")
	c.Request.Header.Set("X-Idempotency-Key", "k")
	c.Params = gin.Params{{Key: "food_order_id", Value: orderID}}
	return svc, c, w
}

func TestHandler_AcceptFoodOrder(t *testing.T) {
	svc, c, w := acceptOrderCtx(t, fOrderID.String())
	c.Set("user_id", fDriverID.String())
	svc.On("AcceptFoodOrder", mock.Anything, mock.MatchedBy(func(r AcceptFoodOrderRequest) bool {
		return r.OrderID == fOrderID && r.DriverID == fDriverID && r.IdempotencyKey == "k"
	})).Return(&AcceptFoodOrderResponse{ID: fOrderID, DriverID: fDriverID, Status: foodStatusReadyForPickup}, nil)

	h := NewHandler(svc)
	h.AcceptFoodOrder(c)
	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_AcceptFoodOrder_MissingKey(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders/"+fOrderID.String()+"/accept", "")
	c.Set("user_id", fDriverID.String())
	c.Params = gin.Params{{Key: "food_order_id", Value: fOrderID.String()}}

	h := NewHandler(svc)
	h.AcceptFoodOrder(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_AcceptFoodOrder_InvalidOrderID(t *testing.T) {
	svc, c, w := acceptOrderCtx(t, "not-a-uuid")
	c.Set("user_id", fDriverID.String())

	h := NewHandler(svc)
	h.AcceptFoodOrder(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_AcceptFoodOrder_Unauthorized(t *testing.T) {
	svc, c, w := acceptOrderCtx(t, fOrderID.String())

	h := NewHandler(svc)
	h.AcceptFoodOrder(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "AcceptFoodOrder", mock.Anything, mock.Anything)
}

func TestHandler_AcceptFoodOrder_Errors(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{ErrNotDriver, http.StatusForbidden},
		{ErrDriverInactive, http.StatusForbidden},
		{ErrDriverBusy, http.StatusConflict},
		{ErrOrderNotReadyForAccept, http.StatusConflict},
		{ErrDriverCapacityExceeded, http.StatusUnprocessableEntity},
		{ErrUserNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		svc, c, w := acceptOrderCtx(t, fOrderID.String())
		c.Set("user_id", fDriverID.String())
		svc.On("AcceptFoodOrder", mock.Anything, mock.Anything).Return(nil, tc.err)

		h := NewHandler(svc)
		h.AcceptFoodOrder(c)
		assert.Equal(t, tc.code, w.Code, tc.err)
	}
}

func TestHandler_AcceptFoodOrder_StatusForError(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{ErrNotDriver, http.StatusForbidden},
		{ErrDriverInactive, http.StatusForbidden},
		{ErrDriverBusy, http.StatusConflict},
		{ErrOrderNotReadyForAccept, http.StatusConflict},
		{ErrDriverCapacityExceeded, http.StatusUnprocessableEntity},
		{ErrUserNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.code, statusForError(tc.err))
	}
}

func TestHandler_AcceptFoodOrder_CodeForError(t *testing.T) {
	assert.Equal(t, "NOT_DRIVER", codeForError(ErrNotDriver))
	assert.Equal(t, "DRIVER_INACTIVE", codeForError(ErrDriverInactive))
	assert.Equal(t, "DRIVER_BUSY", codeForError(ErrDriverBusy))
	assert.Equal(t, "DRIVER_CAPACITY_EXCEEDED", codeForError(ErrDriverCapacityExceeded))
	assert.Equal(t, "ORDER_NOT_READY_FOR_ACCEPT", codeForError(ErrOrderNotReadyForAccept))
	assert.Equal(t, "USER_NOT_FOUND", codeForError(ErrUserNotFound))
}



