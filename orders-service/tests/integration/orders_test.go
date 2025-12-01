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

	"augustberries/orders-service/internal/app/orders/entity"
	"augustberries/orders-service/internal/app/orders/handler"
	"augustberries/orders-service/internal/app/orders/repository"
	"augustberries/orders-service/internal/app/orders/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// MockCatalogClient мок для CatalogServiceClient в integration тестах
type MockCatalogClient struct {
	mock.Mock
	AuthToken string
}

func (m *MockCatalogClient) SetAuthToken(token string) {
	m.AuthToken = token
}

func (m *MockCatalogClient) GetProduct(ctx context.Context, productID uuid.UUID) (*entity.ProductWithCategory, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ProductWithCategory), args.Error(1)
}

func (m *MockCatalogClient) GetProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]*entity.ProductWithCategory, error) {
	args := m.Called(ctx, productIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]*entity.ProductWithCategory), args.Error(1)
}

// MockKafkaProducer мок для Kafka в integration тестах
type MockKafkaProducer struct {
	mock.Mock
	Messages [][]byte
}

func (m *MockKafkaProducer) PublishMessage(ctx context.Context, key string, value []byte) error {
	m.Messages = append(m.Messages, value)
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockKafkaProducer) Close() error {
	return nil
}

// OrdersIntegrationTestSuite тестовый suite для integration тестов
type OrdersIntegrationTestSuite struct {
	suite.Suite
	db            *gorm.DB
	router        *gin.Engine
	orderService  *service.OrderService
	catalogClient *MockCatalogClient
	kafkaProducer *MockKafkaProducer
	testUserID    uuid.UUID
	testProductID uuid.UUID
}

func TestOrdersIntegrationSuite(t *testing.T) {
	suite.Run(t, new(OrdersIntegrationTestSuite))
}

func (s *OrdersIntegrationTestSuite) SetupSuite() {
	// Получаем параметры подключения из окружения или используем defaults
	dsn := getEnv("TEST_DATABASE_URL", "postgres://orders_test:orders_test_password@localhost:5434/orders_test_db?sslmode=disable")

	var err error
	s.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(s.T(), err, "Failed to connect to database")

	// Автомиграция
	err = s.db.AutoMigrate(&entity.Order{}, &entity.OrderItem{})
	require.NoError(s.T(), err, "Failed to migrate database")

	// Инициализация компонентов
	orderRepo := repository.NewOrderRepository(s.db)
	orderItemRepo := repository.NewOrderItemRepository(s.db)

	s.catalogClient = &MockCatalogClient{}
	s.kafkaProducer = &MockKafkaProducer{Messages: make([][]byte, 0)}

	s.orderService = service.NewOrderService(orderRepo, orderItemRepo, s.catalogClient, s.kafkaProducer)

	// Тестовые данные
	s.testUserID = uuid.New()
	s.testProductID = uuid.New()

	// Настройка router
	gin.SetMode(gin.TestMode)
	s.router = gin.New()

	orderHandler := handler.NewOrderHandler(s.orderService)

	// Middleware для установки user_id и auth_token
	authMiddleware := func(c *gin.Context) {
		c.Set("user_id", s.testUserID)
		c.Set("auth_token", "test-token")
		c.Next()
	}

	orders := s.router.Group("/orders")
	orders.Use(authMiddleware)
	{
		orders.POST("", orderHandler.CreateOrder)
		orders.GET("", orderHandler.GetUserOrders)
		orders.GET("/:id", orderHandler.GetOrder)
		orders.PATCH("/:id", orderHandler.UpdateOrderStatus)
		orders.DELETE("/:id", orderHandler.DeleteOrder)
	}

	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
}

func (s *OrdersIntegrationTestSuite) SetupTest() {
	// Очистка таблиц перед каждым тестом
	s.db.Exec("DELETE FROM order_items")
	s.db.Exec("DELETE FROM orders")

	// Сброс моков
	s.catalogClient.ExpectedCalls = nil
	s.catalogClient.Calls = nil
	s.kafkaProducer.Messages = make([][]byte, 0)
	s.kafkaProducer.ExpectedCalls = nil
	s.kafkaProducer.Calls = nil
}

func (s *OrdersIntegrationTestSuite) TearDownSuite() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		sqlDB.Close()
	}
}

// ===================== Integration Tests =====================

func (s *OrdersIntegrationTestSuite) TestCreateOrder_Success() {
	// Настраиваем mock Catalog Service
	products := map[uuid.UUID]*entity.ProductWithCategory{
		s.testProductID: {
			Product: entity.Product{
				ID:    s.testProductID,
				Name:  "Test Product",
				Price: 99.99,
			},
		},
	}
	s.catalogClient.On("GetProducts", mock.Anything, mock.Anything).Return(products, nil)
	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reqBody := entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: s.testProductID, Quantity: 2},
		},
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusCreated, w.Code)

	var response entity.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)

	s.Equal(s.testUserID, response.UserID)
	s.Equal(entity.OrderStatusPending, response.Status)
	s.Equal("USD", response.Currency)
	// TotalPrice = (99.99 * 2) + 10.0 = 209.98
	s.Equal(209.98, response.TotalPrice)

	// Проверяем что заказ сохранён в БД
	var dbOrder entity.Order
	s.db.First(&dbOrder, "id = ?", response.ID)
	s.Equal(response.ID, dbOrder.ID)

	// Проверяем Kafka событие
	s.Len(s.kafkaProducer.Messages, 1)
}

func (s *OrdersIntegrationTestSuite) TestGetOrder_Success() {
	// Создаём заказ напрямую в БД
	orderID := uuid.New()
	order := entity.Order{
		ID:            orderID,
		UserID:        s.testUserID,
		TotalPrice:    150.0,
		DeliveryPrice: 10.0,
		Currency:      "USD",
		Status:        entity.OrderStatusPending,
		CreatedAt:     time.Now(),
	}
	s.db.Create(&order)

	item := entity.OrderItem{
		ID:        uuid.New(),
		OrderID:   orderID,
		ProductID: s.testProductID,
		Quantity:  1,
		UnitPrice: 140.0,
	}
	s.db.Create(&item)

	// Получаем заказ через API
	req, _ := http.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	var response entity.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)

	s.Equal(orderID, response.ID)
	s.Len(response.Items, 1)
}

func (s *OrdersIntegrationTestSuite) TestGetOrder_NotFound() {
	nonExistentID := uuid.New()

	req, _ := http.NewRequest(http.MethodGet, "/orders/"+nonExistentID.String(), nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

func (s *OrdersIntegrationTestSuite) TestGetOrder_AccessDenied() {
	// Создаём заказ от другого пользователя
	anotherUserID := uuid.New()
	orderID := uuid.New()
	order := entity.Order{
		ID:         orderID,
		UserID:     anotherUserID, // Другой пользователь
		TotalPrice: 100.0,
		Status:     entity.OrderStatusPending,
		CreatedAt:  time.Now(),
	}
	s.db.Create(&order)

	// Пытаемся получить чужой заказ
	req, _ := http.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusForbidden, w.Code)
}

func (s *OrdersIntegrationTestSuite) TestUpdateOrderStatus_Success() {
	// Создаём заказ
	orderID := uuid.New()
	order := entity.Order{
		ID:         orderID,
		UserID:     s.testUserID,
		TotalPrice: 100.0,
		Currency:   "USD",
		Status:     entity.OrderStatusPending,
		CreatedAt:  time.Now(),
	}
	s.db.Create(&order)

	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Обновляем статус
	updateReq := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusConfirmed}
	body, _ := json.Marshal(updateReq)

	req, _ := http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	// Проверяем в БД
	var dbOrder entity.Order
	s.db.First(&dbOrder, "id = ?", orderID)
	s.Equal(entity.OrderStatusConfirmed, dbOrder.Status)
}

func (s *OrdersIntegrationTestSuite) TestUpdateOrderStatus_InvalidTransition() {
	// Создаём заказ со статусом delivered (финальный)
	orderID := uuid.New()
	order := entity.Order{
		ID:         orderID,
		UserID:     s.testUserID,
		TotalPrice: 100.0,
		Status:     entity.OrderStatusDelivered,
		CreatedAt:  time.Now(),
	}
	s.db.Create(&order)

	// Пытаемся изменить статус
	updateReq := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusPending}
	body, _ := json.Marshal(updateReq)

	req, _ := http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *OrdersIntegrationTestSuite) TestDeleteOrder_Success() {
	// Создаём заказ с позициями
	orderID := uuid.New()
	order := entity.Order{
		ID:         orderID,
		UserID:     s.testUserID,
		TotalPrice: 100.0,
		Status:     entity.OrderStatusPending,
		CreatedAt:  time.Now(),
	}
	s.db.Create(&order)

	item := entity.OrderItem{
		ID:        uuid.New(),
		OrderID:   orderID,
		ProductID: s.testProductID,
		Quantity:  1,
		UnitPrice: 100.0,
	}
	s.db.Create(&item)

	// Удаляем заказ
	req, _ := http.NewRequest(http.MethodDelete, "/orders/"+orderID.String(), nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	// Проверяем что заказ удалён
	var count int64
	s.db.Model(&entity.Order{}).Where("id = ?", orderID).Count(&count)
	s.Equal(int64(0), count)

	// Проверяем что позиции тоже удалены (CASCADE)
	s.db.Model(&entity.OrderItem{}).Where("order_id = ?", orderID).Count(&count)
	s.Equal(int64(0), count)
}

func (s *OrdersIntegrationTestSuite) TestGetUserOrders_Success() {
	// Создаём несколько заказов
	for i := 0; i < 3; i++ {
		order := entity.Order{
			ID:         uuid.New(),
			UserID:     s.testUserID,
			TotalPrice: float64(100 * (i + 1)),
			Status:     entity.OrderStatusPending,
			CreatedAt:  time.Now(),
		}
		s.db.Create(&order)
	}

	req, _ := http.NewRequest(http.MethodGet, "/orders", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	s.Equal(float64(3), response["total"])
}

func (s *OrdersIntegrationTestSuite) TestGetUserOrders_Empty() {
	req, _ := http.NewRequest(http.MethodGet, "/orders", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	s.Equal(float64(0), response["total"])
}

func (s *OrdersIntegrationTestSuite) TestHealthCheck() {
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *OrdersIntegrationTestSuite) TestOrderWorkflow_FullCycle() {
	// Настраиваем моки
	products := map[uuid.UUID]*entity.ProductWithCategory{
		s.testProductID: {Product: entity.Product{ID: s.testProductID, Price: 100.0}},
	}
	s.catalogClient.On("GetProducts", mock.Anything, mock.Anything).Return(products, nil)
	s.kafkaProducer.On("PublishMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// 1. Создаём заказ
	createReq := entity.CreateOrderRequest{
		Items:         []entity.OrderItemRequest{{ProductID: s.testProductID, Quantity: 1}},
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusCreated, w.Code)

	var createdOrder entity.OrderResponse
	json.Unmarshal(w.Body.Bytes(), &createdOrder)
	orderID := createdOrder.ID

	// 2. Подтверждаем (pending -> confirmed)
	updateReq := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusConfirmed}
	body, _ = json.Marshal(updateReq)
	req, _ = http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	// 3. Отправляем (confirmed -> shipped)
	updateReq.Status = entity.OrderStatusShipped
	body, _ = json.Marshal(updateReq)
	req, _ = http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	// 4. Доставляем (shipped -> delivered)
	updateReq.Status = entity.OrderStatusDelivered
	body, _ = json.Marshal(updateReq)
	req, _ = http.NewRequest(http.MethodPatch, "/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	// 5. Проверяем финальный статус
	var dbOrder entity.Order
	s.db.First(&dbOrder, "id = ?", orderID)
	assert.Equal(s.T(), entity.OrderStatusDelivered, dbOrder.Status)
}

// Helper function
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
