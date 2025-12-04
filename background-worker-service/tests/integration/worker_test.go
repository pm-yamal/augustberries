//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// MockAPIClient для integration тестов
type MockAPIClient struct {
	Rates map[string]float64
	Error error
}

func (m *MockAPIClient) FetchRates(ctx context.Context) (map[string]float64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Rates, nil
}

// BackgroundWorkerIntegrationTestSuite тестовый suite
type BackgroundWorkerIntegrationTestSuite struct {
	suite.Suite
	db                     *gorm.DB
	redisClient            *redis.Client
	orderRepo              repository.OrderRepository
	rateRepo               repository.ExchangeRateRepository
	exchangeService        *service.ExchangeRateService
	orderProcessingService *service.OrderProcessingService
	mockAPIClient          *MockAPIClient
}

func TestBackgroundWorkerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(BackgroundWorkerIntegrationTestSuite))
}

func (s *BackgroundWorkerIntegrationTestSuite) SetupSuite() {
	// PostgreSQL
	dsn := getEnv("TEST_DATABASE_URL", "postgres://worker_test:worker_test_password@localhost:5435/worker_test_db?sslmode=disable")

	var err error
	s.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(s.T(), err, "Failed to connect to PostgreSQL")

	// AutoMigrate для таблицы orders
	err = s.db.AutoMigrate(&entity.Order{})
	require.NoError(s.T(), err, "Failed to migrate Order")

	// Redis
	redisAddr := getEnv("TEST_REDIS_ADDR", "localhost:6380")
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err = s.redisClient.Ping(context.Background()).Result()
	require.NoError(s.T(), err, "Failed to connect to Redis")

	// Repositories
	s.orderRepo = repository.NewOrderRepository(s.db)
	s.rateRepo = repository.NewExchangeRateRepository(s.redisClient, 30*time.Minute)

	// Mock API Client с тестовыми курсами
	s.mockAPIClient = &MockAPIClient{
		Rates: map[string]float64{
			"USD": 1.0,
			"EUR": 0.93,
			"RUB": 91.23,
			"GBP": 0.79,
		},
	}

	// Services
	s.exchangeService = service.NewExchangeRateService(s.rateRepo, s.mockAPIClient)
	s.orderProcessingService = service.NewOrderProcessingService(s.orderRepo, s.exchangeService)
}

func (s *BackgroundWorkerIntegrationTestSuite) SetupTest() {
	ctx := context.Background()

	// Очистка PostgreSQL
	s.db.Exec("DELETE FROM orders")

	// Очистка Redis
	s.redisClient.FlushDB(ctx)

	// Загружаем курсы валют
	err := s.exchangeService.FetchAndStoreRates(ctx)
	require.NoError(s.T(), err)
}

func (s *BackgroundWorkerIntegrationTestSuite) TearDownSuite() {
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		sqlDB.Close()
	}
}

// ===================== Integration Tests =====================

func (s *BackgroundWorkerIntegrationTestSuite) TestExchangeRates_FetchAndStore() {
	ctx := context.Background()

	// Проверяем что курсы сохранены в Redis
	rate, err := s.exchangeService.GetRate(ctx, "USD")
	s.NoError(err)
	s.Equal(1.0, rate.Rate)

	rate, err = s.exchangeService.GetRate(ctx, "RUB")
	s.NoError(err)
	s.Equal(91.23, rate.Rate)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestExchangeRates_Convert_USDtoRUB() {
	ctx := context.Background()

	// Конвертация 100 USD -> RUB
	converted, exchangeRate, err := s.exchangeService.ConvertCurrency(ctx, 100.0, "USD", "RUB")

	s.NoError(err)
	s.InDelta(9123.0, converted, 0.01)
	s.InDelta(91.23, exchangeRate, 0.01)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestExchangeRates_Convert_EURtoRUB() {
	ctx := context.Background()

	// Конвертация 100 EUR -> RUB
	// EUR=0.93, RUB=91.23, rate = 91.23/0.93 = 98.096
	converted, exchangeRate, err := s.exchangeService.ConvertCurrency(ctx, 100.0, "EUR", "RUB")

	s.NoError(err)
	expectedRate := 91.23 / 0.93
	s.InDelta(100.0*expectedRate, converted, 0.01)
	s.InDelta(expectedRate, exchangeRate, 0.01)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestOrderProcessing_FullCycle() {
	ctx := context.Background()

	// 1. Создаём заказ в БД
	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0, // 100 товары + 10 доставка
		DeliveryPrice: 10.0,
		Currency:      "USD",
		Status:        entity.OrderStatusPending,
		CreatedAt:     time.Now(),
	}

	err := s.db.Create(order).Error
	s.NoError(err)

	// 2. Создаём событие ORDER_CREATED
	event := &entity.OrderEvent{
		EventType:  entity.EventTypeOrderCreated,
		OrderID:    orderID,
		UserID:     userID,
		TotalPrice: 110.0,
		Currency:   "USD",
	}

	// 3. Обрабатываем событие
	err = s.orderProcessingService.ProcessOrderEvent(ctx, event)
	s.NoError(err)

	// 4. Проверяем что заказ обновлён
	var updatedOrder entity.Order
	err = s.db.First(&updatedOrder, "id = ?", orderID).Error
	s.NoError(err)

	// Ожидаемые значения:
	// DeliveryPrice: 10 USD * 91.23 = 912.3 RUB
	// TotalPrice: 100 * 91.23 + 912.3 = 9123 + 912.3 = 10035.3 RUB
	s.Equal("RUB", updatedOrder.Currency)
	s.InDelta(912.3, updatedOrder.DeliveryPrice, 0.01)
	s.InDelta(10035.3, updatedOrder.TotalPrice, 0.01)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestOrderProcessing_ZeroDelivery_Skipped() {
	ctx := context.Background()

	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    100.0,
		DeliveryPrice: 0.0, // Нулевая доставка
		Currency:      "USD",
		Status:        entity.OrderStatusPending,
	}

	err := s.db.Create(order).Error
	s.NoError(err)

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	err = s.orderProcessingService.ProcessOrderEvent(ctx, event)
	s.NoError(err)

	// Заказ не должен измениться
	var updatedOrder entity.Order
	s.db.First(&updatedOrder, "id = ?", orderID)

	s.Equal("USD", updatedOrder.Currency) // Валюта не изменилась
	s.Equal(100.0, updatedOrder.TotalPrice)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestOrderProcessing_EUROrder() {
	ctx := context.Background()

	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "EUR", // Заказ в EUR
		Status:        entity.OrderStatusPending,
	}

	err := s.db.Create(order).Error
	s.NoError(err)

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	err = s.orderProcessingService.ProcessOrderEvent(ctx, event)
	s.NoError(err)

	var updatedOrder entity.Order
	s.db.First(&updatedOrder, "id = ?", orderID)

	// EUR -> RUB: rate = 91.23 / 0.93 = 98.096
	expectedRate := 91.23 / 0.93
	expectedDelivery := 10.0 * expectedRate
	expectedTotal := 100.0*expectedRate + expectedDelivery

	s.Equal("RUB", updatedOrder.Currency)
	s.InDelta(expectedDelivery, updatedOrder.DeliveryPrice, 0.1)
	s.InDelta(expectedTotal, updatedOrder.TotalPrice, 0.1)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestExchangeRates_EnsureAvailable() {
	ctx := context.Background()

	// Очищаем Redis
	s.redisClient.FlushDB(ctx)

	// EnsureRatesAvailable должен загрузить курсы
	err := s.exchangeService.EnsureRatesAvailable(ctx)
	s.NoError(err)

	// Проверяем что курсы доступны
	rate, err := s.exchangeService.GetRate(ctx, "USD")
	s.NoError(err)
	s.NotNil(rate)
}

func (s *BackgroundWorkerIntegrationTestSuite) TestExchangeRates_SameCurrency() {
	ctx := context.Background()

	// USD -> USD должно вернуть ту же сумму
	converted, rate, err := s.exchangeService.ConvertCurrency(ctx, 100.0, "USD", "USD")

	s.NoError(err)
	s.Equal(100.0, converted)
	s.Equal(1.0, rate)
}

// Helper function
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
