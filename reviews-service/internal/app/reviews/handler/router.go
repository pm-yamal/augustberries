package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"augustberries/pkg/metrics"
)

// SetupRoutes настраивает все маршруты Reviews Service с использованием Gin
// Применяет Auth middleware для защиты эндпоинтов
func SetupRoutes(reviewHandler *ReviewHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.Default()

	// Prometheus metrics middleware
	router.Use(metrics.GinPrometheusMiddleware("reviews-service"))

	// Health check endpoint - публичный, без аутентификации
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "reviews-service",
		})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Reviews endpoints - все требуют аутентификации
	reviews := router.Group("/reviews")
	reviews.Use(authMiddleware.Authenticate()) // Все маршруты требуют JWT токен
	{
		// Базовые операции с отзывами
		reviews.POST("/", reviewHandler.CreateReview)                          // Создать отзыв
		reviews.GET("/product/:product_id", reviewHandler.GetReviewsByProduct) // Получить все отзывы по товару (используется индекс)
		reviews.PATCH("/:review_id", reviewHandler.UpdateReview)               // Обновить конкретный отзыв
		reviews.DELETE("/:review_id", reviewHandler.DeleteReview)              // Удалить конкретный отзыв
	}

	return router
}
