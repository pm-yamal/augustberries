//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"augustberries/reviews-service/internal/app/reviews/entity"
	"augustberries/reviews-service/internal/app/reviews/handler"
	"augustberries/reviews-service/internal/app/reviews/repository"
	"augustberries/reviews-service/internal/app/reviews/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MockKafkaProducer struct {
	mock.Mock
	Messages [][]byte
}

func (m *MockKafkaProducer) PublishMessage(ctx context.Context, key string, value []byte) error {
	m.Messages = append(m.Messages, value)
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockKafkaProducer) Close() error { return nil }

type ReviewsIntegrationTestSuite struct {
	suite.Suite
	client        *mongo.Client
	db            *mongo.Database
	router        *gin.Engine
	reviewService *service.ReviewService
	kafkaProducer *MockKafkaProducer
	testUserID    string
	testProductID string
}

func TestReviewsIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ReviewsIntegrationTestSuite))
}

func (s *ReviewsIntegrationTestSuite) SetupSuite() {
	mongoURI := getEnv("TEST_MONGODB_URI", "mongodb://localhost:27018")
	dbName := getEnv("TEST_MONGODB_DATABASE", "reviews_test_db")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	s.client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	s.Require().NoError(err)

	s.db = s.client.Database(dbName)

	reviewRepo := repository.NewReviewRepository(s.db)
	s.kafkaProducer = &MockKafkaProducer{Messages: make([][]byte, 0)}
	s.reviewService = service.NewReviewService(reviewRepo, s.kafkaProducer)

	s.testUserID = "test-user-" + primitive.NewObjectID().Hex()
	s.testProductID = "test-product-" + primitive.NewObjectID().Hex()

	gin.SetMode(gin.TestMode)
	s.router = gin.New()

	reviewHandler := handler.NewReviewHandler(s.reviewService)

	authMiddleware := func(c *gin.Context) {
		c.Set("user_id", s.testUserID)
		c.Next()
	}

	reviews := s.router.Group("/reviews")
	reviews.POST("", authMiddleware, reviewHandler.CreateReview)
	reviews.GET("/:product_id", reviewHandler.GetReviewsByProduct)
	reviews.PATCH("/:review_id", authMiddleware, reviewHandler.UpdateReview)
	reviews.DELETE("/:review_id", authMiddleware, reviewHandler.DeleteReview)

	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
}

func (s *ReviewsIntegrationTestSuite) SetupTest() {
	ctx := context.Background()
	s.db.Collection("reviews").Drop(ctx)
	s.kafkaProducer.Messages = make([][]byte, 0)
	s.kafkaProducer.ExpectedCalls = nil
	s.kafkaProducer.Calls = nil
}

func (s *ReviewsIntegrationTestSuite) TearDownSuite() {
	if s.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.client.Disconnect(ctx)
	}
}

func (s *ReviewsIntegrationTestSuite) TestCreateReview_Success() {
	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reqBody := entity.CreateReviewRequest{ProductID: s.testProductID, Rating: 5, Text: "Excellent product!"}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusCreated, w.Code)

	var response entity.Review
	json.Unmarshal(w.Body.Bytes(), &response)
	s.Equal(s.testUserID, response.UserID)
	s.Equal(5, response.Rating)
}

func (s *ReviewsIntegrationTestSuite) TestGetReviewsByProduct_Success() {
	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	for i := 1; i <= 3; i++ {
		reqBody := entity.CreateReviewRequest{ProductID: s.testProductID, Rating: i + 2, Text: "Test review text here."}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
	}

	req, _ := http.NewRequest(http.MethodGet, "/reviews/"+s.testProductID, nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	var response entity.ReviewListResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	s.Equal(3, response.Total)
}

func (s *ReviewsIntegrationTestSuite) TestUpdateReview_Success() {
	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	createReq := entity.CreateReviewRequest{ProductID: s.testProductID, Rating: 3, Text: "Average product here."}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	var created entity.Review
	json.Unmarshal(w.Body.Bytes(), &created)

	updateReq := entity.UpdateReviewRequest{Rating: 5, Text: "Updated: great product!"}
	body, _ = json.Marshal(updateReq)
	req, _ = http.NewRequest(http.MethodPatch, "/reviews/"+created.ID.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	var updated entity.Review
	json.Unmarshal(w.Body.Bytes(), &updated)
	s.Equal(5, updated.Rating)
}

func (s *ReviewsIntegrationTestSuite) TestDeleteReview_Success() {
	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	createReq := entity.CreateReviewRequest{ProductID: s.testProductID, Rating: 4, Text: "Good product here."}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest(http.MethodPost, "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	var created entity.Review
	json.Unmarshal(w.Body.Bytes(), &created)

	req, _ = http.NewRequest(http.MethodDelete, "/reviews/"+created.ID.Hex(), nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *ReviewsIntegrationTestSuite) TestHealthCheck() {
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	s.Equal(http.StatusOK, w.Code)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
