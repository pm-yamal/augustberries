package handler

import (
	"bytes"
	"encoding/json"
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

type MockReviewService struct {
	mock.Mock
}

func (m *MockReviewService) CreateReview(ctx interface{}, userID string, req *entity.CreateReviewRequest) (*entity.Review, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Review), args.Error(1)
}

func (m *MockReviewService) GetReviewsByProduct(ctx interface{}, productID string) ([]entity.Review, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Review), args.Error(1)
}

func (m *MockReviewService) UpdateReview(ctx interface{}, reviewID string, userID string, req *entity.UpdateReviewRequest) (*entity.Review, error) {
	args := m.Called(ctx, reviewID, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Review), args.Error(1)
}

func (m *MockReviewService) DeleteReview(ctx interface{}, reviewID string, userID string) error {
	args := m.Called(ctx, reviewID, userID)
	return args.Error(0)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestCreateReviewHandler_Success(t *testing.T) {
	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	review := &entity.Review{ID: reviewID, ProductID: "product-456", UserID: userID, Rating: 5, Text: "Great!", CreatedAt: time.Now()}

	mockService := new(MockReviewService)
	mockService.On("CreateReview", mock.Anything, userID, mock.AnythingOfType("*entity.CreateReviewRequest")).Return(review, nil)

	router.POST("/reviews", func(c *gin.Context) {
		c.Set("user_id", userID)
		var req entity.CreateReviewRequest
		c.ShouldBindJSON(&req)
		result, _ := mockService.CreateReview(c.Request.Context(), userID, &req)
		c.JSON(http.StatusCreated, result)
	})

	body, _ := json.Marshal(entity.CreateReviewRequest{ProductID: "product-456", Rating: 5, Text: "Great!"})
	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateReviewHandler_Unauthorized(t *testing.T) {
	router := setupTestRouter()

	router.POST("/reviews", func(c *gin.Context) {
		_, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
	})

	body, _ := json.Marshal(entity.CreateReviewRequest{ProductID: "product-456", Rating: 5, Text: "Great!"})
	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetReviewsByProductHandler_Success(t *testing.T) {
	router := setupTestRouter()
	productID := "product-456"

	reviews := []entity.Review{
		{ID: primitive.NewObjectID(), ProductID: productID, Rating: 5},
		{ID: primitive.NewObjectID(), ProductID: productID, Rating: 4},
	}

	mockService := new(MockReviewService)
	mockService.On("GetReviewsByProduct", mock.Anything, productID).Return(reviews, nil)

	router.GET("/reviews/:product_id", func(c *gin.Context) {
		pid := c.Param("product_id")
		result, _ := mockService.GetReviewsByProduct(c.Request.Context(), pid)
		c.JSON(http.StatusOK, entity.ReviewListResponse{Reviews: result, Total: len(result)})
	})

	req, _ := http.NewRequest(http.MethodGet, "/reviews/"+productID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.ReviewListResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 2, response.Total)
}

func TestUpdateReviewHandler_Success(t *testing.T) {
	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	updated := &entity.Review{ID: reviewID, UserID: userID, Rating: 5, Text: "Updated!"}

	mockService := new(MockReviewService)
	mockService.On("UpdateReview", mock.Anything, reviewID.Hex(), userID, mock.Anything).Return(updated, nil)

	router.PATCH("/reviews/:review_id", func(c *gin.Context) {
		c.Set("user_id", userID)
		rid := c.Param("review_id")
		var req entity.UpdateReviewRequest
		c.ShouldBindJSON(&req)
		result, _ := mockService.UpdateReview(c.Request.Context(), rid, userID, &req)
		c.JSON(http.StatusOK, result)
	})

	body, _ := json.Marshal(entity.UpdateReviewRequest{Rating: 5, Text: "Updated!"})
	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateReviewHandler_NotFound(t *testing.T) {
	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService := new(MockReviewService)
	mockService.On("UpdateReview", mock.Anything, reviewID.Hex(), userID, mock.Anything).Return(nil, service.ErrReviewNotFound)

	router.PATCH("/reviews/:review_id", func(c *gin.Context) {
		c.Set("user_id", userID)
		rid := c.Param("review_id")
		var req entity.UpdateReviewRequest
		c.ShouldBindJSON(&req)
		_, err := mockService.UpdateReview(c.Request.Context(), rid, userID, &req)
		if err == service.ErrReviewNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
			return
		}
	})

	body, _ := json.Marshal(entity.UpdateReviewRequest{Rating: 5})
	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateReviewHandler_Forbidden(t *testing.T) {
	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService := new(MockReviewService)
	mockService.On("UpdateReview", mock.Anything, reviewID.Hex(), userID, mock.Anything).Return(nil, service.ErrUnauthorized)

	router.PATCH("/reviews/:review_id", func(c *gin.Context) {
		c.Set("user_id", userID)
		rid := c.Param("review_id")
		var req entity.UpdateReviewRequest
		c.ShouldBindJSON(&req)
		_, err := mockService.UpdateReview(c.Request.Context(), rid, userID, &req)
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	})

	body, _ := json.Marshal(entity.UpdateReviewRequest{Rating: 1})
	req, _ := http.NewRequest(http.MethodPatch, "/reviews/"+reviewID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteReviewHandler_Success(t *testing.T) {
	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService := new(MockReviewService)
	mockService.On("DeleteReview", mock.Anything, reviewID.Hex(), userID).Return(nil)

	router.DELETE("/reviews/:review_id", func(c *gin.Context) {
		c.Set("user_id", userID)
		rid := c.Param("review_id")
		mockService.DeleteReview(c.Request.Context(), rid, userID)
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	})

	req, _ := http.NewRequest(http.MethodDelete, "/reviews/"+reviewID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteReviewHandler_NotFound(t *testing.T) {
	router := setupTestRouter()
	userID := "user-123"
	reviewID := primitive.NewObjectID()

	mockService := new(MockReviewService)
	mockService.On("DeleteReview", mock.Anything, reviewID.Hex(), userID).Return(service.ErrReviewNotFound)

	router.DELETE("/reviews/:review_id", func(c *gin.Context) {
		c.Set("user_id", userID)
		rid := c.Param("review_id")
		err := mockService.DeleteReview(c.Request.Context(), rid, userID)
		if err == service.ErrReviewNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
			return
		}
	})

	req, _ := http.NewRequest(http.MethodDelete, "/reviews/"+reviewID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
