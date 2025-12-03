//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/processor"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// MockAPIClient для E2E тестов
type MockAPIClient struct {
	Rates map[string]float64
}

func (m *MockAPIClient) FetchRates(ctx context.Context) (map[string]float64, error) {
	return m.Rates, nil
}

// BackgroundWorkerE2ETestSuite E2E тестовый suite
type BackgroundWorkerE2ETestSuite struct {
	suite.Suite
	db                     *gorm.DB
	redisClient            *redis.Client
	kafkaWriter            *kafka.Writer
	kafkaReader            *kafka.Reader
	orderRepo              repository.OrderRepository
	rateRepo               repository.ExchangeRateRepository
	exchangeService        *service.ExchangeRateService
	orderProcessingService *service.OrderProcessingService
	kafkaConsumer          *processor.KafkaConsumer
	mockAPIClient          *MockAPIClient
	ctx                    context.Context
	cancel                 context.CancelFunc
}

func TestBackgroundWorkerE2ESuite(t *testing.T) {
	suite.Run(t, new(BackgroundWorkerE2ETestSuite))
}

func (s *BackgroundWorkerE2ETestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// PostgreSQL
	dsn := getEnv("TEST_DATABASE_URL", "postgres://worker_test:worker_test_password@localhost:5435/worker_test_db?sslmode=disable")

	var err error
	s.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(s.T(), err, "Failed to connect to PostgreSQL")

	err = s.db.AutoMigrate(&entity.Order{})
	require.NoError(s.T(), err, "Failed to migrate Order")

	// Redis
	redisAddr := getEnv("TEST_REDIS_ADDR", "localhost:6380")
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err = s.redisClient.Ping(s.ctx).Result()
	require.NoError(s.T(), err, "Failed to connect to Redis")

	// Kafka
	kafkaBroker := getEnv("TEST_KAFKA_BROKER", "localhost:9096")
	kafkaTopic := getEnv("TEST_KAFKA_TOPIC", "order_events_test")

	// Создаём топик если не существует
	s.createKafkaTopic(kafkaBroker, kafkaTopic)

	// Kafka Writer для отправки событий
	s.kafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}

	// Repositories
	s.orderRepo = repository.NewOrderRepository(s.db)
	s.rateRepo = repository.NewExchangeRateRepository(s.redisClient, 30*time.Minute)

	// Mock API Client
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

	// Kafka Consumer
	s.kafkaConsumer = processor.NewKafkaConsumer(
		[]string{kafkaBroker},
		kafkaTopic,
		"e2e-test-group-"+uuid.New().String(), // Уникальный group ID для каждого запуска
		1,      // minBytes
		10e6,   // maxBytes (10MB)
		s.orderProcessingService,
		s.exchangeService,
	)
}

func (s *BackgroundWorkerE2ETestSuite) createKafkaTopic(broker, topic string) {
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		s.T().Logf("Warning: Failed to connect to Kafka for topic creation: %v", err)
		return
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		s.T().Logf("Warning: Failed to get Kafka controller: %v", err)
		return
	}

	controllerConn, err := kafka.Dial("tcp", controller.Host+":"+string(rune(controller.Port)))
	if err != nil {
		// Fallback: используем исходное соединение
		controllerConn = conn
	} else {
		defer controllerConn.Close()
	}

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		s.T().Logf("Topic creation (may already exist): %v", err)
	}
}

func (s *BackgroundWorkerE2ETestSuite) SetupTest() {
	// Очистка PostgreSQL
	s.db.Exec("DELETE FROM orders")

	// Очистка Redis
	s.redisClient.FlushDB(s.ctx)

	// Загрузка курсов валют
	err := s.exchangeService.FetchAndStoreRates(s.ctx)
	require.NoError(s.T(), err)
}

func (s *BackgroundWorkerE2ETestSuite) TearDownSuite() {
	s.cancel()

	if s.kafkaWriter != nil {
		s.kafkaWriter.Close()
	}
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		sqlDB.Close()
	}
}

// ===================== E2E Tests =====================

func (s *BackgroundWorkerE2ETestSuite) TestE2E_OrderCreated_FullFlow() {
	// Полный E2E тест:
	// 1. Создаём заказ в PostgreSQL
	// 2. Отправляем ORDER_CREATED в Kafka
	// 3. Worker обрабатывает событие
	// 4. Проверяем что заказ обновлён с конвертированной валютой

	// Arrange
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
	s.Require().NoError(err)

	event := &entity.OrderEvent{
		EventType:  entity.EventTypeOrderCreated,
		OrderID:    orderID,
		UserID:     userID,
		TotalPrice: 110.0,
		Currency:   "USD",
		Status:     entity.OrderStatusPending,
		ItemsCount: 2,
		Timestamp:  time.Now(),
	}

	// Запускаем consumer
	s.kafkaConsumer.Start(s.ctx)
	defer s.kafkaConsumer.Stop()

	// Даём consumer время запуститься
	time.Sleep(500 * time.Millisecond)

	// Act - отправляем событие в Kafka
	eventJSON, _ := json.Marshal(event)
	err = s.kafkaWriter.WriteMessages(s.ctx, kafka.Message{
		Key:   []byte(orderID.String()),
		Value: eventJSON,
	})
	s.Require().NoError(err)

	// Ждём обработки (с таймаутом)
	s.waitForOrderUpdate(orderID, "RUB", 10*time.Second)

	// Assert
	var updatedOrder entity.Order
	err = s.db.First(&updatedOrder, "id = ?", orderID).Error
	s.Require().NoError(err)

	// Проверяем конвертацию
	// DeliveryPrice: 10 USD * 91.23 = 912.3 RUB
	// TotalPrice: 100 * 91.23 + 912.3 = 10035.3 RUB
	s.Equal("RUB", updatedOrder.Currency)
	s.InDelta(912.3, updatedOrder.DeliveryPrice, 0.1)
	s.InDelta(10035.3, updatedOrder.TotalPrice, 0.1)
}

func (s *BackgroundWorkerE2ETestSuite) TestE2E_OrderCreated_EURtoRUB() {
	// E2E тест конвертации EUR -> RUB

	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    220.0, // 200 товары + 20 доставка
		DeliveryPrice: 20.0,
		Currency:      "EUR",
		Status:        entity.OrderStatusPending,
		CreatedAt:     time.Now(),
	}

	err := s.db.Create(order).Error
	s.Require().NoError(err)

	event := &entity.OrderEvent{
		EventType:  entity.EventTypeOrderCreated,
		OrderID:    orderID,
		UserID:     userID,
		TotalPrice: 220.0,
		Currency:   "EUR",
		Timestamp:  time.Now(),
	}

	s.kafkaConsumer.Start(s.ctx)
	defer s.kafkaConsumer.Stop()
	time.Sleep(500 * time.Millisecond)

	eventJSON, _ := json.Marshal(event)
	err = s.kafkaWriter.WriteMessages(s.ctx, kafka.Message{
		Key:   []byte(orderID.String()),
		Value: eventJSON,
	})
	s.Require().NoError(err)

	s.waitForOrderUpdate(orderID, "RUB", 10*time.Second)

	var updatedOrder entity.Order
	s.db.First(&updatedOrder, "id = ?", orderID)

	// EUR -> RUB: rate = 91.23 / 0.93 = 98.096
	expectedRate := 91.23 / 0.93
	expectedDelivery := 20.0 * expectedRate
	expectedTotal := 200.0*expectedRate + expectedDelivery

	s.Equal("RUB", updatedOrder.Currency)
	s.InDelta(expectedDelivery, updatedOrder.DeliveryPrice, 0.5)
	s.InDelta(expectedTotal, updatedOrder.TotalPrice, 0.5)
}

func (s *BackgroundWorkerE2ETestSuite) TestE2E_MultipleOrders_Sequential() {
	// Обработка нескольких заказов последовательно

	orders := []struct {
		id       uuid.UUID
		total    float64
		delivery float64
		currency string
	}{
		{uuid.New(), 110.0, 10.0, "USD"},
		{uuid.New(), 220.0, 20.0, "EUR"},
		{uuid.New(), 330.0, 30.0, "USD"},
	}

	// Создаём заказы в БД
	for _, o := range orders {
		order := &entity.Order{
			ID:            o.id,
			UserID:        uuid.New(),
			TotalPrice:    o.total,
			DeliveryPrice: o.delivery,
			Currency:      o.currency,
			Status:        entity.OrderStatusPending,
			CreatedAt:     time.Now(),
		}
		err := s.db.Create(order).Error
		s.Require().NoError(err)
	}

	s.kafkaConsumer.Start(s.ctx)
	defer s.kafkaConsumer.Stop()
	time.Sleep(500 * time.Millisecond)

	// Отправляем события в Kafka
	for _, o := range orders {
		event := &entity.OrderEvent{
			EventType:  entity.EventTypeOrderCreated,
			OrderID:    o.id,
			TotalPrice: o.total,
			Currency:   o.currency,
			Timestamp:  time.Now(),
		}

		eventJSON, _ := json.Marshal(event)
		err := s.kafkaWriter.WriteMessages(s.ctx, kafka.Message{
			Key:   []byte(o.id.String()),
			Value: eventJSON,
		})
		s.Require().NoError(err)
	}

	// Ждём обработки всех заказов
	for _, o := range orders {
		s.waitForOrderUpdate(o.id, "RUB", 15*time.Second)
	}

	// Проверяем что все заказы обновлены
	for _, o := range orders {
		var updatedOrder entity.Order
		err := s.db.First(&updatedOrder, "id = ?", o.id).Error
		s.Require().NoError(err)
		s.Equal("RUB", updatedOrder.Currency, "Order %s should be in RUB", o.id)
	}
}

func (s *BackgroundWorkerE2ETestSuite) TestE2E_OrderUpdated_Ignored() {
	// ORDER_UPDATED должен игнорироваться

	orderID := uuid.New()
	userID := uuid.New()

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "USD", // Останется USD
		Status:        entity.OrderStatusPending,
	}

	err := s.db.Create(order).Error
	s.Require().NoError(err)

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderUpdated, // Не ORDER_CREATED
		OrderID:   orderID,
		Timestamp: time.Now(),
	}

	s.kafkaConsumer.Start(s.ctx)
	defer s.kafkaConsumer.Stop()
	time.Sleep(500 * time.Millisecond)

	eventJSON, _ := json.Marshal(event)
	err = s.kafkaWriter.WriteMessages(s.ctx, kafka.Message{
		Key:   []byte(orderID.String()),
		Value: eventJSON,
	})
	s.Require().NoError(err)

	// Ждём немного
	time.Sleep(2 * time.Second)

	// Проверяем что заказ НЕ изменился
	var updatedOrder entity.Order
	s.db.First(&updatedOrder, "id = ?", orderID)

	s.Equal("USD", updatedOrder.Currency) // Валюта не изменилась
	s.Equal(110.0, updatedOrder.TotalPrice)
	s.Equal(10.0, updatedOrder.DeliveryPrice)
}

func (s *BackgroundWorkerE2ETestSuite) TestE2E_ZeroDelivery_Skipped() {
	// Заказ с нулевой доставкой не обрабатывается

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
	s.Require().NoError(err)

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
		Timestamp: time.Now(),
	}

	s.kafkaConsumer.Start(s.ctx)
	defer s.kafkaConsumer.Stop()
	time.Sleep(500 * time.Millisecond)

	eventJSON, _ := json.Marshal(event)
	err = s.kafkaWriter.WriteMessages(s.ctx, kafka.Message{
		Key:   []byte(orderID.String()),
		Value: eventJSON,
	})
	s.Require().NoError(err)

	time.Sleep(2 * time.Second)

	var updatedOrder entity.Order
	s.db.First(&updatedOrder, "id = ?", orderID)

	// Заказ не изменился
	s.Equal("USD", updatedOrder.Currency)
	s.Equal(100.0, updatedOrder.TotalPrice)
	s.Equal(0.0, updatedOrder.DeliveryPrice)
}

// ===================== Helper Methods =====================

func (s *BackgroundWorkerE2ETestSuite) waitForOrderUpdate(orderID uuid.UUID, expectedCurrency string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		var order entity.Order
		if err := s.db.First(&order, "id = ?", orderID).Error; err == nil {
			if order.Currency == expectedCurrency {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	s.T().Logf("Timeout waiting for order %s to update to currency %s", orderID, expectedCurrency)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
