package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetupRoutes настраивает все маршруты Reviews Service с использованием Gin
// Применяет Auth middleware для защиты эндпоинтов
func SetupRoutes(reviewHandler *ReviewHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.Default()

	// Health check endpoint - публичный, без аутентификации
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "reviews-service",
		})
	})

	// Reviews endpoints - все требуют аутентификации
	reviews := router.Group("/reviews")
	reviews.Use(authMiddleware.Authenticate()) // Все маршруты требуют JWT токен
	{
		// Базовые операции с отзывами
		reviews.POST("/", reviewHandler.CreateReview)                 // Создать отзыв
		reviews.GET("/:review_id", reviewHandler.GetReviewsByProduct) // Получить отзывы по товару (используется индекс)
		reviews.PATCH("/:review_id", reviewHandler.UpdateReview)      // Обновить отзыв
		reviews.DELETE("/:review_id", reviewHandler.DeleteReview)     // Удалить отзыв
	}

	return router
}
