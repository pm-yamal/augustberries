package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/reviews-service/internal/app/reviews/entity"
	"augustberries/reviews-service/internal/app/reviews/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockReviewService реализует ReviewServiceInterface для тестирования
type MockReviewService struct {
	mock.Mock
}

func (m *MockReviewService) CreateReview(ctx context.Context, userID string, req *entity.CreateReviewRequest) (*entity.Review, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Review), args.Error(1)
}

func (m *MockReviewService) GetReviewsByProduct(ctx context.Context, productID string) ([]entity.Review, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Review), args.Error(1)
}

func (m *MockReviewService) GetReview(ctx context.Context, reviewID string) (*entity.Review, error) {
	args := m.Called(ctx, reviewID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Review), args.Error(1)
}

func (m *MockReviewService) UpdateReview(ctx context.Context, reviewID string, userID string, req *entity.UpdateReviewRequest) (*entity.Review, error) {
	args := m.Called(ctx, reviewID, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Review), args.Error(1)
}

func (m *MockReviewService) DeleteReview(ctx context.Context, reviewID string, userID string) error {
	args := m.Called(ctx, reviewID, userID)
	return args.Error(0)
}

func (m *MockReviewService) GetUserReviews(ctx context.Context, userID string) ([]entity.Review, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Review), args.Error(1)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// authMiddleware устанавливает user_id в контекст
func authMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}
}

// ===================== CreateReview Tests =====================

func TestCreateReviewHandler_Success(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	expectedReview := &entity.Review{
		ID:        reviewID,
		ProductID: "product-456",
		UserID:    userID,
		Rating:    5,
		Text:      "Отличный товар! Рекомендую всем покупать.",
		CreatedAt: time.Now(),
	}

	mockService.On("CreateReview", mock.Anything, userID, mock.AnythingOfType("*entity.CreateReviewRequest")).Return(expectedReview, nil)

	router.POST("/reviews", authMiddleware(userID), handler.CreateReview)

	// Act
	reqBody := entity.CreateReviewRequest{
		ProductID: "product-456",
		Rating:    5,
		Text:      "Отличный товар! Рекомендую всем покупать.",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var response entity.Review
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, reviewID, response.ID)
	assert.Equal(t, userID, response.UserID)

	mockService.AssertExpectations(t)
}

func TestCreateReviewHandler_Unauthorized(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	// НЕ добавляем authMiddleware - user_id не будет в контексте
	router.POST("/reviews", handler.CreateReview)

	// Act
	reqBody := entity.CreateReviewRequest{
		ProductID: "product-456",
		Rating:    5,
		Text:      "Отличный товар!",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateReviewHandler_InvalidJSON(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	router.POST("/reviews", authMiddleware(userID), handler.CreateReview)

	// Act
	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateReviewHandler_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	router.POST("/reviews", authMiddleware(userID), handler.CreateReview)

	// Act - Rating вне диапазона 1-5
	reqBody := entity.CreateReviewRequest{
		ProductID: "product-456",
		Rating:    10, // Invalid
		Text:      "Текст отзыва достаточной длины.",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateReviewHandler_ServiceError(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"

	mockService.On("CreateReview", mock.Anything, userID, mock.Anything).Return(nil, errors.New("service error"))

	router.POST("/reviews", authMiddleware(userID), handler.CreateReview)

	// Act
	reqBody := entity.CreateReviewRequest{
		ProductID: "product-456",
		Rating:    5,
		Text:      "Отличный товар! Рекомендую.",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ===================== GetReviewsByProduct Tests =====================

func TestGetReviewsByProductHandler_Success(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	productID := "product-456"

	reviews := []entity.Review{
		{ID: primitive.NewObjectID(), ProductID: productID, Rating: 5, Text: "Отлично!"},
		{ID: primitive.NewObjectID(), ProductID: productID, Rating: 4, Text: "Хорошо!"},
	}

	mockService.On("GetReviewsByProduct", mock.Anything, productID).Return(reviews, nil)

	router.GET("/reviews/product/:product_id", handler.GetReviewsByProduct)

	// Act
	req, _ := http.NewRequest(http.MethodGet, "/reviews/product/"+productID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.ReviewListResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 2, response.Total)
	assert.Len(t, response.Reviews, 2)

	mockService.AssertExpectations(t)
}

func TestGetReviewsByProductHandler_Empty(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	productID := "product-no-reviews"

	mockService.On("GetReviewsByProduct", mock.Anything, productID).Return([]entity.Review{}, nil)

	router.GET("/reviews/product/:product_id", handler.GetReviewsByProduct)

	// Act
	req, _ := http.NewRequest(http.MethodGet, "/reviews/product/"+productID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.ReviewListResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 0, response.Total)
}

func TestGetReviewsByProductHandler_ServiceError(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	productID := "product-456"

	mockService.On("GetReviewsByProduct", mock.Anything, productID).Return(nil, errors.New("db error"))

	router.GET("/reviews/product/:product_id", handler.GetReviewsByProduct)

	// Act
	req, _ := http.NewRequest(http.MethodGet, "/reviews/product/"+productID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ===================== UpdateReview Tests =====================

func TestUpdateReviewHandler_Success(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	updatedReview := &entity.Review{
		ID:     reviewID,
		UserID: userID,
		Rating: 5,
		Text:   "Обновлённый отзыв!",
	}

	mockService.On("UpdateReview", mock.Anything, reviewID.Hex(), userID, mock.AnythingOfType("*entity.UpdateReviewRequest")).Return(updatedReview, nil)

	router.PATCH("/reviews/:review_id", authMiddleware(userID), handler.UpdateReview)

	// Act
	reqBody := entity.UpdateReviewRequest{Rating: 5, Text: "Обновлённый отзыв!"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.Review
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 5, response.Rating)

	mockService.AssertExpectations(t)
}

func TestUpdateReviewHandler_NotFound(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService.On("UpdateReview", mock.Anything, reviewID.Hex(), userID, mock.Anything).Return(nil, service.ErrReviewNotFound)

	router.PATCH("/reviews/:review_id", authMiddleware(userID), handler.UpdateReview)

	// Act
	reqBody := entity.UpdateReviewRequest{Rating: 5}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateReviewHandler_Forbidden(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService.On("UpdateReview", mock.Anything, reviewID.Hex(), userID, mock.Anything).Return(nil, service.ErrUnauthorized)

	router.PATCH("/reviews/:review_id", authMiddleware(userID), handler.UpdateReview)

	// Act
	reqBody := entity.UpdateReviewRequest{Rating: 1}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateReviewHandler_Unauthorized(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	reviewID := primitive.NewObjectID()

	// НЕ добавляем authMiddleware
	router.PATCH("/reviews/:review_id", handler.UpdateReview)

	// Act
	reqBody := entity.UpdateReviewRequest{Rating: 5}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ===================== DeleteReview Tests =====================

func TestDeleteReviewHandler_Success(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService.On("DeleteReview", mock.Anything, reviewID.Hex(), userID).Return(nil)

	router.DELETE("/reviews/:review_id", authMiddleware(userID), handler.DeleteReview)

	// Act
	req, _ := http.NewRequest(http.MethodDelete, "/reviews/"+reviewID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestDeleteReviewHandler_NotFound(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService.On("DeleteReview", mock.Anything, reviewID.Hex(), userID).Return(service.ErrReviewNotFound)

	router.DELETE("/reviews/:review_id", authMiddleware(userID), handler.DeleteReview)

	// Act
	req, _ := http.NewRequest(http.MethodDelete, "/reviews/"+reviewID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteReviewHandler_Forbidden(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService.On("DeleteReview", mock.Anything, reviewID.Hex(), userID).Return(service.ErrUnauthorized)

	router.DELETE("/reviews/:review_id", authMiddleware(userID), handler.DeleteReview)

	// Act
	req, _ := http.NewRequest(http.MethodDelete, "/reviews/"+reviewID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteReviewHandler_Unauthorized(t *testing.T) {
	// Arrange
	mockService := new(MockReviewService)
	handler := NewReviewHandler(mockService)

	router := setupTestRouter()
	reviewID := primitive.NewObjectID()

	// НЕ добавляем authMiddleware
	router.DELETE("/reviews/:review_id", handler.DeleteReview)

	// Act
	req, _ := http.NewRequest(http.MethodDelete, "/reviews/"+reviewID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
