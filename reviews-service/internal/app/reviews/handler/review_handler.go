package handler

import (
	"errors"
	"net/http"

	"augustberries/reviews-service/internal/app/reviews/entity"
	"augustberries/reviews-service/internal/app/reviews/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ReviewHandler обрабатывает HTTP запросы для отзывов с использованием Gin
type ReviewHandler struct {
	reviewService *service.ReviewService
	validator     *validator.Validate
}

// NewReviewHandler создает новый обработчик отзывов
func NewReviewHandler(reviewService *service.ReviewService) *ReviewHandler {
	return &ReviewHandler{
		reviewService: reviewService,
		validator:     validator.New(),
	}
}

// CreateReview обрабатывает POST /reviews/
// Создает новый отзыв и отправляет событие REVIEW_CREATED в Kafka
func (h *ReviewHandler) CreateReview(c *gin.Context) {
	// Получаем userID из контекста (установлен middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var req entity.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	// Создаем отзыв
	review, err := h.reviewService.CreateReview(c.Request.Context(), userIDStr, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create review"})
		return
	}

	c.JSON(http.StatusCreated, review)
}

// GetReviewsByProduct обрабатывает GET /reviews/{product_id}
// Получает все отзывы по товару (используется индекс product_id)
func (h *ReviewHandler) GetReviewsByProduct(c *gin.Context) {
	productID := c.Param("product_id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product ID is required"})
		return
	}

	reviews, err := h.reviewService.GetReviewsByProduct(c.Request.Context(), productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get reviews"})
		return
	}

	response := entity.ReviewListResponse{
		Reviews: reviews,
		Total:   len(reviews),
	}

	c.JSON(http.StatusOK, response)
}

// UpdateReview обрабатывает PATCH /reviews/{review_id}
// Обновляет конкретный отзыв с проверкой прав доступа
func (h *ReviewHandler) UpdateReview(c *gin.Context) {
	// Получаем userID из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Получаем review_id из параметров URL
	reviewID := c.Param("review_id")
	if reviewID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Review ID is required"})
		return
	}

	var req entity.UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Обновляем отзыв
	review, err := h.reviewService.UpdateReview(c.Request.Context(), reviewID, userIDStr, &req)
	if err != nil {
		if errors.Is(err, service.ErrReviewNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
			return
		}
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update review"})
		return
	}

	c.JSON(http.StatusOK, review)
}

// DeleteReview обрабатывает DELETE /reviews/{review_id}
// Удаляет конкретный отзыв с проверкой прав доступа
func (h *ReviewHandler) DeleteReview(c *gin.Context) {
	// Получаем userID из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Получаем review_id из параметров URL
	reviewID := c.Param("review_id")
	if reviewID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Review ID is required"})
		return
	}

	// Удаляем отзыв
	if err := h.reviewService.DeleteReview(c.Request.Context(), reviewID, userIDStr); err != nil {
		if errors.Is(err, service.ErrReviewNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
			return
		}
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete review"})
		return
	}

	c.JSON(http.StatusOK, entity.SuccessResponse{
		Message: "Review deleted successfully",
	})
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
