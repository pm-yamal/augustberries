package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetupRoutes настраивает все маршруты Orders Service с использованием Gin
// Применяет Auth middleware для защиты эндпоинтов
func SetupRoutes(orderHandler *OrderHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.Default()

	// Health check endpoint - публичный, без аутентификации
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "orders-service",
		})
	})

	// Orders endpoints - все требуют аутентификации
	orders := router.Group("/orders")
	orders.Use(authMiddleware.Authenticate()) // Все маршруты требуют JWT токен
	{
		// Базовые операции с заказами
		orders.POST("/", orderHandler.CreateOrder)         // Создать заказ
		orders.GET("/", orderHandler.GetUserOrders)        // Получить все заказы пользователя
		orders.GET("/:id", orderHandler.GetOrder)          // Получить заказ по ID
		orders.PATCH("/:id", orderHandler.UpdateOrderStatus) // Обновить статус заказа
		orders.DELETE("/:id", orderHandler.DeleteOrder)    // Удалить заказ
	}

	return router
}
