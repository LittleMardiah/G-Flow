package food

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandler_UpdateMerchant_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/bad", `{}`, gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.UpdateMerchant(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String(), `{bad`, gin.Param{Key: "id", Value: fMerchID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.UpdateMerchant(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String(), `{"merchant_name":"X"}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c3.Set("user_id", fCustID.String())
	svc3.On("UpdateMerchant", mock.Anything, mock.Anything).Return(nil, ErrMerchantNotFound)
	h3 := NewHandler(svc3)
	h3.UpdateMerchant(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_CreateMenu_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/bad/menus", `{}`, gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.CreateMenu(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/menus", `{bad`, gin.Param{Key: "id", Value: fMerchID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.CreateMenu(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/menus", `{"name":"M"}`, gin.Param{Key: "id", Value: fMerchID.String()})
	c3.Set("user_id", fCustID.String())
	svc3.On("CreateMenu", mock.Anything, mock.Anything).Return(nil, ErrMerchantNotFound)
	h3 := NewHandler(svc3)
	h3.CreateMenu(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_UpdateMenu_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/bad/menus/"+fMenuID.String(), `{}`,
		gin.Param{Key: "id", Value: "bad"}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	h := NewHandler(new(mockFoodService))
	h.UpdateMenu(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String()+"/menus/bad", `{}`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: "bad"})
	h2 := NewHandler(new(mockFoodService))
	h2.UpdateMenu(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	_, c3, w3 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String()+"/menus/"+fMenuID.String(), `{bad`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	h3 := NewHandler(new(mockFoodService))
	h3.UpdateMenu(c3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)

	svc4, c4, w4 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String()+"/menus/"+fMenuID.String(), `{"name":"M"}`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	c4.Set("user_id", fCustID.String())
	svc4.On("UpdateMenu", mock.Anything, mock.Anything).Return(nil, ErrMenuNotFound)
	h4 := NewHandler(svc4)
	h4.UpdateMenu(c4)
	assert.Equal(t, http.StatusNotFound, w4.Code)
}

func TestHandler_DeleteMenu_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodDelete, "/api/v1/merchants/bad/menus/"+fMenuID.String(), "",
		gin.Param{Key: "id", Value: "bad"}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	h := NewHandler(new(mockFoodService))
	h.DeleteMenu(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodDelete, "/api/v1/merchants/"+fMerchID.String()+"/menus/bad", "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: "bad"})
	h2 := NewHandler(new(mockFoodService))
	h2.DeleteMenu(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodDelete, "/api/v1/merchants/"+fMerchID.String()+"/menus/"+fMenuID.String(), "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "menu_id", Value: fMenuID.String()})
	c3.Set("user_id", fCustID.String())
	svc3.On("DeleteMenu", mock.Anything, fMerchID, fMenuID, fCustID).Return(ErrMenuNotFound)
	h3 := NewHandler(svc3)
	h3.DeleteMenu(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_GetMenus_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/bad/menus", "", gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.GetMenus(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/menus", "", gin.Param{Key: "id", Value: fMerchID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.GetMenus(c2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/menus", "", gin.Param{Key: "id", Value: fMerchID.String()})
	c3.Set("user_id", fCustID.String())
	svc3.On("GetMenus", mock.Anything, fMerchID, fCustID).Return(nil, ErrMerchantNotFound)
	h3 := NewHandler(svc3)
	h3.GetMenus(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_CreateItem_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/bad/items", `{}`, gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.CreateItem(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/items", `{bad`, gin.Param{Key: "id", Value: fMerchID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.CreateItem(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	_, c3, w3 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/items",
		`{"menu_id":"bad","name":"N"}`, gin.Param{Key: "id", Value: fMerchID.String()})
	h3 := NewHandler(new(mockFoodService))
	h3.CreateItem(c3)
	assert.Equal(t, http.StatusUnprocessableEntity, w3.Code)

	svc4, c4, w4 := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/merchants/"+fMerchID.String()+"/items",
		`{"menu_id":"`+fMenuID.String()+`","name":"N","price":10000}`,
		gin.Param{Key: "id", Value: fMerchID.String()})
	c4.Set("user_id", fCustID.String())
	svc4.On("CreateItem", mock.Anything, mock.Anything).Return(nil, ErrMenuNotFound)
	h4 := NewHandler(svc4)
	h4.CreateItem(c4)
	assert.Equal(t, http.StatusNotFound, w4.Code)
}

func TestHandler_UpdateItem_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/bad/items/"+fItemID.String(), `{}`,
		gin.Param{Key: "id", Value: "bad"}, gin.Param{Key: "item_id", Value: fItemID.String()})
	h := NewHandler(new(mockFoodService))
	h.UpdateItem(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String()+"/items/bad", `{}`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: "bad"})
	h2 := NewHandler(new(mockFoodService))
	h2.UpdateItem(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	_, c3, w3 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String()+"/items/"+fItemID.String(), `{bad`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: fItemID.String()})
	h3 := NewHandler(new(mockFoodService))
	h3.UpdateItem(c3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)

	svc4, c4, w4 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/merchants/"+fMerchID.String()+"/items/"+fItemID.String(), `{"name":"N"}`,
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: fItemID.String()})
	c4.Set("user_id", fCustID.String())
	svc4.On("UpdateItem", mock.Anything, mock.Anything).Return(nil, ErrItemNotFound)
	h4 := NewHandler(svc4)
	h4.UpdateItem(c4)
	assert.Equal(t, http.StatusNotFound, w4.Code)
}

func TestHandler_DeleteItem_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodDelete, "/api/v1/merchants/bad/items/"+fItemID.String(), "",
		gin.Param{Key: "id", Value: "bad"}, gin.Param{Key: "item_id", Value: fItemID.String()})
	h := NewHandler(new(mockFoodService))
	h.DeleteItem(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodDelete, "/api/v1/merchants/"+fMerchID.String()+"/items/bad", "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: "bad"})
	h2 := NewHandler(new(mockFoodService))
	h2.DeleteItem(c2)
	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	_, c3, w3 := newFoodHandlerCtx(t, http.MethodDelete, "/api/v1/merchants/"+fMerchID.String()+"/items/"+fItemID.String(), "",
		gin.Param{Key: "id", Value: fMerchID.String()}, gin.Param{Key: "item_id", Value: fItemID.String()})
	h3 := NewHandler(new(mockFoodService))
	h3.DeleteItem(c3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)
}

func TestHandler_GetItems_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/bad/items", "", gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.GetItems(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/items", "", gin.Param{Key: "id", Value: fMerchID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.GetItems(c2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/items", "", gin.Param{Key: "id", Value: fMerchID.String()})
	c3.Set("user_id", fCustID.String())
	svc3.On("GetItems", mock.Anything, fMerchID, fCustID).Return(nil, ErrMerchantNotFound)
	h3 := NewHandler(svc3)
	h3.GetItems(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestHandler_GetMerchantItems_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/bad/items", "", gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.GetMerchantItems(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/items", "", gin.Param{Key: "id", Value: fMerchID.String()})
	svc2.On("GetMerchantItems", mock.Anything, fMerchID, uuid.Nil).Return(nil, ErrMerchantNotFound)
	h2 := NewHandler(svc2)
	h2.GetMerchantItems(c2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestHandler_GetFoodOrder_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders/bad", "", gin.Param{Key: "id", Value: "bad"})
	h := NewHandler(new(mockFoodService))
	h.GetFoodOrder(c)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.GetFoodOrder(c2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	svc3, c3, w3 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	c3.Set("user_id", fCustID.String())
	svc3.On("GetFoodOrder", mock.Anything, fOrderID, fCustID).Return(nil, ErrFoodOrderNotFound)
	h3 := NewHandler(svc3)
	h3.GetFoodOrder(c3)
	assert.Equal(t, http.StatusNotFound, w3.Code)

	svc4, c4, w4 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders/"+fOrderID.String(), "", gin.Param{Key: "id", Value: fOrderID.String()})
	c4.Set("user_id", fCustID.String())
	svc4.On("GetFoodOrder", mock.Anything, fOrderID, fCustID).Return(&FoodOrder{ID: fOrderID}, nil)
	svc4.On("GetFoodOrderItems", mock.Anything, fOrderID).Return(nil, errors.New("internal"))
	h4 := NewHandler(svc4)
	h4.GetFoodOrder(c4)
	assert.Equal(t, http.StatusInternalServerError, w4.Code)
}

func TestHandler_GetFoodOrderHistory_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders?page=1&page_size=10", "")
	h := NewHandler(new(mockFoodService))
	h.GetFoodOrderHistory(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	svc2, c2, w2 := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/food-orders?page=1&page_size=10", "")
	c2.Set("user_id", fCustID.String())
	svc2.On("GetFoodOrderHistory", mock.Anything, fCustID, 1, 10).Return(nil, 0, ErrFoodOrderNotFound)
	h2 := NewHandler(svc2)
	h2.GetFoodOrderHistory(c2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestHandler_UpdateFoodOrderStatus_Errors(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{bad`,
		gin.Param{Key: "id", Value: fOrderID.String()})
	h := NewHandler(new(mockFoodService))
	h.UpdateFoodOrderStatus(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	_, c2, w2 := newFoodHandlerCtx(t, http.MethodPatch, "/api/v1/food-orders/"+fOrderID.String(), `{"status":"confirmed"}`,
		gin.Param{Key: "id", Value: fOrderID.String()})
	h2 := NewHandler(new(mockFoodService))
	h2.UpdateFoodOrderStatus(c2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestHandler_CreateFoodOrder_Unauthorized(t *testing.T) {
	_, c, w := newFoodHandlerCtx(t, http.MethodPost, "/api/v1/food-orders",
		`{"merchant_id":"`+fMerchID.String()+`","payment_method":"wallet","delivery_address":"Jl. B","items":[{"item_id":"`+fItemID.String()+`","quantity":2}]}`)
	c.Request.Header.Set("X-Idempotency-Key", "idem-1")
	h := NewHandler(new(mockFoodService))
	h.CreateFoodOrder(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_UserIDNonString(t *testing.T) {
	svc, c, w := newFoodHandlerCtx(t, http.MethodGet, "/api/v1/merchants/"+fMerchID.String()+"/menus", "", gin.Param{Key: "id", Value: fMerchID.String()})
	c.Set("user_id", 12345)
	h := NewHandler(svc)
	h.GetMenus(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}