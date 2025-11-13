package handler

import (
	"errors"
	"net/http"

	"augustberries/orders-service/internal/app/orders/entity"
	"augustberries/orders-service/internal/app/orders/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// OrderHandler обрабатывает HTTP запросы для заказов с использованием Gin
type OrderHandler struct {
	orderService *service.OrderService
	validator    *validator.Validate
}

// NewOrderHandler создает новый обработчик заказов
func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		validator:    validator.New(),
	}
}

// CreateOrder обрабатывает POST /orders/
// Создает новый заказ с проверкой цен из Catalog Service
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	// Получаем userID из контекста (установлен middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Получаем auth токен для запросов к Catalog Service
	authToken, _ := c.Get("auth_token")
	authTokenStr, _ := authToken.(string)

	var req entity.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	// Создаем заказ
	order, err := h.orderService.CreateOrder(c.Request.Context(), userUUID, &req, authTokenStr)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "One or more products not found in catalog"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	// Формируем ответ
	response := buildOrderResponse(order)
	c.JSON(http.StatusCreated, response)
}

// GetOrder обрабатывает GET /orders/{id}
// Получает заказ по ID с проверкой прав доступа
func (h *OrderHandler) GetOrder(c *gin.Context) {
	// Получаем userID из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Парсим orderID из URL
	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Получаем заказ
	order, err := h.orderService.GetOrder(c.Request.Context(), orderID, userUUID)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order"})
		return
	}

	// Формируем ответ
	response := buildOrderResponse(order)
	c.JSON(http.StatusOK, response)
}

// UpdateOrderStatus обрабатывает PATCH /orders/{id}
// Обновляет статус заказа (shipped, delivered)
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	// Получаем userID из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Парсим orderID из URL
	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req entity.UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	// Обновляем статус
	order, err := h.orderService.UpdateOrderStatus(c.Request.Context(), orderID, userUUID, req.Status)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		if errors.Is(err, service.ErrInvalidOrderStatus) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status transition"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":     order.ID,
		"status": order.Status,
	})
}

// DeleteOrder обрабатывает DELETE /orders/{id}
// Удаляет заказ с проверкой прав доступа
func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	// Получаем userID из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Парсим orderID из URL
	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Удаляем заказ
	if err := h.orderService.DeleteOrder(c.Request.Context(), orderID, userUUID); err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete order"})
		return
	}

	c.JSON(http.StatusOK, entity.SuccessResponse{
		Message: "Order deleted successfully",
	})
}

// GetUserOrders обрабатывает GET /orders/
// Получает все заказы текущего пользователя
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	// Получаем userID из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Получаем заказы пользователя
	orders, err := h.orderService.GetUserOrders(c.Request.Context(), userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"total":  len(orders),
	})
}

// buildOrderResponse формирует ответ с информацией о заказе
func buildOrderResponse(order *entity.OrderWithItems) entity.OrderResponse {
	items := make([]entity.ItemResponse, len(order.Items))
	for i, item := range order.Items {
		items[i] = entity.ItemResponse{
			ID:         item.ID,
			ProductID:  item.ProductID,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: item.UnitPrice * float64(item.Quantity),
		}
	}

	return entity.OrderResponse{
		ID:            order.ID,
		UserID:        order.UserID,
		TotalPrice:    order.TotalPrice,
		DeliveryPrice: order.DeliveryPrice,
		Currency:      order.Currency,
		Status:        order.Status,
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Items:         items,
	}
}

// formatValidationError форматирует ошибки валидации
func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			return fieldError.Field() + " is " + fieldError.Tag()
		}
	}
	return "Validation failed"
}
