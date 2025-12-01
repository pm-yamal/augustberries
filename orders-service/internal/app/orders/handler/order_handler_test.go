package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/orders-service/internal/app/orders/entity"
	"augustberries/orders-service/internal/app/orders/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOrderService мок для OrderService в тестах handler
type MockOrderService struct {
	mock.Mock
}

func (m *MockOrderService) CreateOrder(ctx interface{}, userID uuid.UUID, req *entity.CreateOrderRequest, authToken string) (*entity.OrderWithItems, error) {
	args := m.Called(ctx, userID, req, authToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.OrderWithItems), args.Error(1)
}

func (m *MockOrderService) GetOrder(ctx interface{}, orderID uuid.UUID, userID uuid.UUID) (*entity.OrderWithItems, error) {
	args := m.Called(ctx, orderID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.OrderWithItems), args.Error(1)
}

func (m *MockOrderService) UpdateOrderStatus(ctx interface{}, orderID uuid.UUID, userID uuid.UUID, newStatus entity.OrderStatus) (*entity.Order, error) {
	args := m.Called(ctx, orderID, userID, newStatus)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Order), args.Error(1)
}

func (m *MockOrderService) DeleteOrder(ctx interface{}, orderID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, orderID, userID)
	return args.Error(0)
}

func (m *MockOrderService) GetUserOrders(ctx interface{}, userID uuid.UUID) ([]entity.Order, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Order), args.Error(1)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// ===================== CreateOrder Handler Tests =====================

func TestCreateOrderHandler_Success(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()
	productID := uuid.New()

	order := &entity.OrderWithItems{
		Order: entity.Order{
			ID:            orderID,
			UserID:        userID,
			TotalPrice:    110.0,
			DeliveryPrice: 10.0,
			Currency:      "USD",
			Status:        entity.OrderStatusPending,
			CreatedAt:     time.Now(),
		},
		Items: []entity.OrderItem{
			{ID: uuid.New(), OrderID: orderID, ProductID: productID, Quantity: 2, UnitPrice: 50.0},
		},
	}

	mockService := new(MockOrderService)
	mockService.On("CreateOrder", mock.Anything, userID, mock.AnythingOfType("*entity.CreateOrderRequest"), mock.Anything).Return(order, nil)

	router.POST("/orders", func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("auth_token", "test-token")

		var req entity.CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		result, err := mockService.CreateOrder(c.Request.Context(), userID, &req, "test-token")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, buildOrderResponse(result))
	})

	reqBody := entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 2},
		},
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response entity.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, orderID, response.ID)
	assert.Equal(t, entity.OrderStatusPending, response.Status)
}

func TestCreateOrderHandler_InvalidJSON(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()

	router.POST("/orders", func(c *gin.Context) {
		c.Set("user_id", userID)

		var req entity.CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	})

	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderHandler_Unauthorized(t *testing.T) {
	router := setupTestRouter()

	router.POST("/orders", func(c *gin.Context) {
		// user_id НЕ установлен в context
		_, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
	})

	reqBody := entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: uuid.New(), Quantity: 1},
		},
		Currency: "USD",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateOrderHandler_ProductNotFound(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("CreateOrder", mock.Anything, userID, mock.Anything, mock.Anything).Return(nil, service.ErrProductNotFound)

	router.POST("/orders", func(c *gin.Context) {
		c.Set("user_id", userID)

		var req entity.CreateOrderRequest
		c.ShouldBindJSON(&req)

		_, err := mockService.CreateOrder(c.Request.Context(), userID, &req, "")
		if err == service.ErrProductNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "One or more products not found in catalog"})
			return
		}
	})

	reqBody := entity.CreateOrderRequest{
		Items:    []entity.OrderItemRequest{{ProductID: uuid.New(), Quantity: 1}},
		Currency: "USD",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===================== GetOrder Handler Tests =====================

func TestGetOrderHandler_Success(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	order := &entity.OrderWithItems{
		Order: entity.Order{
			ID:         orderID,
			UserID:     userID,
			TotalPrice: 100.0,
			Status:     entity.OrderStatusPending,
			CreatedAt:  time.Now(),
		},
		Items: []entity.OrderItem{},
	}

	mockService := new(MockOrderService)
	mockService.On("GetOrder", mock.Anything, orderID, userID).Return(order, nil)

	router.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		result, err := mockService.GetOrder(c.Request.Context(), oid, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, buildOrderResponse(result))
	})

	req, _ := http.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.OrderResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, orderID, response.ID)
}

func TestGetOrderHandler_InvalidID(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()

	router.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		_, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}
	})

	req, _ := http.NewRequest(http.MethodGet, "/orders/invalid-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetOrderHandler_NotFound(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("GetOrder", mock.Anything, orderID, userID).Return(nil, service.ErrOrderNotFound)

	router.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		_, err := mockService.GetOrder(c.Request.Context(), oid, userID)
		if err == service.ErrOrderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
	})

	req, _ := http.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetOrderHandler_Forbidden(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("GetOrder", mock.Anything, orderID, userID).Return(nil, service.ErrUnauthorized)

	router.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		_, err := mockService.GetOrder(c.Request.Context(), oid, userID)
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	})

	req, _ := http.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ===================== UpdateOrderStatus Handler Tests =====================

func TestUpdateOrderStatusHandler_Success(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	updatedOrder := &entity.Order{
		ID:     orderID,
		UserID: userID,
		Status: entity.OrderStatusConfirmed,
	}

	mockService := new(MockOrderService)
	mockService.On("UpdateOrderStatus", mock.Anything, orderID, userID, entity.OrderStatusConfirmed).Return(updatedOrder, nil)

	router.PATCH("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		var req entity.UpdateOrderStatusRequest
		c.ShouldBindJSON(&req)

		result, err := mockService.UpdateOrderStatus(c.Request.Context(), oid, userID, req.Status)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": result.ID, "status": result.Status})
	})

	reqBody := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusConfirmed}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateOrderStatusHandler_InvalidTransition(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("UpdateOrderStatus", mock.Anything, orderID, userID, mock.Anything).Return(nil, service.ErrInvalidOrderStatus)

	router.PATCH("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		var req entity.UpdateOrderStatusRequest
		c.ShouldBindJSON(&req)

		_, err := mockService.UpdateOrderStatus(c.Request.Context(), oid, userID, req.Status)
		if err == service.ErrInvalidOrderStatus {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status transition"})
			return
		}
	})

	reqBody := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusShipped}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===================== DeleteOrder Handler Tests =====================

func TestDeleteOrderHandler_Success(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("DeleteOrder", mock.Anything, orderID, userID).Return(nil)

	router.DELETE("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		if err := mockService.DeleteOrder(c.Request.Context(), oid, userID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, entity.SuccessResponse{Message: "Order deleted successfully"})
	})

	req, _ := http.NewRequest(http.MethodDelete, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteOrderHandler_NotFound(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	orderID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("DeleteOrder", mock.Anything, orderID, userID).Return(service.ErrOrderNotFound)

	router.DELETE("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)

		oid, _ := uuid.Parse(c.Param("id"))

		if err := mockService.DeleteOrder(c.Request.Context(), oid, userID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
	})

	req, _ := http.NewRequest(http.MethodDelete, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ===================== GetUserOrders Handler Tests =====================

func TestGetUserOrdersHandler_Success(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()

	orders := []entity.Order{
		{ID: uuid.New(), UserID: userID, TotalPrice: 100.0, Status: entity.OrderStatusPending},
		{ID: uuid.New(), UserID: userID, TotalPrice: 200.0, Status: entity.OrderStatusDelivered},
	}

	mockService := new(MockOrderService)
	mockService.On("GetUserOrders", mock.Anything, userID).Return(orders, nil)

	router.GET("/orders", func(c *gin.Context) {
		c.Set("user_id", userID)

		result, _ := mockService.GetUserOrders(c.Request.Context(), userID)

		c.JSON(http.StatusOK, gin.H{"orders": result, "total": len(result)})
	})

	req, _ := http.NewRequest(http.MethodGet, "/orders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(2), response["total"])
}

func TestGetUserOrdersHandler_Empty(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()

	mockService := new(MockOrderService)
	mockService.On("GetUserOrders", mock.Anything, userID).Return([]entity.Order{}, nil)

	router.GET("/orders", func(c *gin.Context) {
		c.Set("user_id", userID)

		result, _ := mockService.GetUserOrders(c.Request.Context(), userID)

		c.JSON(http.StatusOK, gin.H{"orders": result, "total": len(result)})
	})

	req, _ := http.NewRequest(http.MethodGet, "/orders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(0), response["total"])
}
