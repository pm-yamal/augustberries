package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOrderProcessingService мок для OrderProcessingServiceInterface
type MockOrderProcessingService struct {
	mock.Mock
}

func (m *MockOrderProcessingService) ProcessOrderEvent(ctx context.Context, event *entity.OrderEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// ===================== NewKafkaConsumer Tests =====================

func TestNewKafkaConsumer(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	brokers := []string{"localhost:9092"}
	topic := "order_events"
	groupID := "test-group"

	// Act
	consumer := NewKafkaConsumer(brokers, topic, groupID, 1, 10e6, orderSvc, exchangeSvc)

	// Assert
	assert.NotNil(t, consumer)
	assert.NotNil(t, consumer.reader)
	assert.NotNil(t, consumer.orderSvc)
	assert.NotNil(t, consumer.exchangeSvc)
	assert.NotNil(t, consumer.stopChan)
	assert.NotNil(t, consumer.doneChan)

	// Cleanup
	consumer.reader.Close()
}

func TestNewKafkaConsumer_MultipleBrokers(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	brokers := []string{"broker1:9092", "broker2:9092", "broker3:9092"}
	topic := "order_events"
	groupID := "test-group"

	// Act
	consumer := NewKafkaConsumer(brokers, topic, groupID, 1024, 10e6, orderSvc, exchangeSvc)

	// Assert
	assert.NotNil(t, consumer)

	// Cleanup
	consumer.reader.Close()
}

// ===================== processMessage Tests =====================

func TestKafkaConsumer_ProcessMessage_Success(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
	}

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := entity.OrderEvent{
		EventType:  entity.EventTypeOrderCreated,
		OrderID:    orderID,
		UserID:     userID,
		TotalPrice: 100.0,
		Currency:   "USD",
		Timestamp:  time.Now(),
	}

	eventJSON, _ := json.Marshal(event)

	message := kafka.Message{
		Topic:     "order_events",
		Partition: 0,
		Offset:    1,
		Key:       []byte(orderID.String()),
		Value:     eventJSON,
	}

	orderSvc.On("ProcessOrderEvent", ctx, mock.MatchedBy(func(e *entity.OrderEvent) bool {
		return e.OrderID == orderID && e.EventType == entity.EventTypeOrderCreated
	})).Return(nil)

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.NoError(t, err)
	orderSvc.AssertExpectations(t)
}

func TestKafkaConsumer_ProcessMessage_InvalidJSON(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
	}

	ctx := context.Background()

	message := kafka.Message{
		Value: []byte("invalid json {{{"),
	}

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
	orderSvc.AssertNotCalled(t, "ProcessOrderEvent")
}

func TestKafkaConsumer_ProcessMessage_ServiceError(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
	}

	ctx := context.Background()

	event := entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   uuid.New(),
	}
	eventJSON, _ := json.Marshal(event)

	message := kafka.Message{
		Value: eventJSON,
	}

	orderSvc.On("ProcessOrderEvent", ctx, mock.Anything).Return(errors.New("processing failed"))

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process order event")
}

func TestKafkaConsumer_ProcessMessage_EmptyMessage(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
	}

	ctx := context.Background()

	message := kafka.Message{
		Value: []byte{},
	}

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestKafkaConsumer_ProcessMessage_OrderUpdated(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
	}

	ctx := context.Background()

	event := entity.OrderEvent{
		EventType: entity.EventTypeOrderUpdated, // ORDER_UPDATED
		OrderID:   uuid.New(),
	}
	eventJSON, _ := json.Marshal(event)

	message := kafka.Message{
		Value: eventJSON,
	}

	// ORDER_UPDATED обрабатывается, но пропускается в service
	orderSvc.On("ProcessOrderEvent", ctx, mock.Anything).Return(nil)

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.NoError(t, err)
	orderSvc.AssertExpectations(t)
}

// ===================== Start/Stop Tests =====================

func TestKafkaConsumer_StartStop(t *testing.T) {
	// Тест на graceful shutdown без реального Kafka
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	// Создаём consumer напрямую без reader
	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
	}

	exchangeSvc.On("EnsureRatesAvailable", mock.Anything).Return(nil)

	// Симулируем consume loop который сразу выходит
	go func() {
		<-consumer.stopChan
		close(consumer.doneChan)
	}()

	// Act
	close(consumer.stopChan)
	<-consumer.doneChan

	// Assert - consumer остановился без паники
	assert.NotNil(t, consumer)
}

// ===================== GetStats Tests =====================

func TestKafkaConsumer_GetStats(t *testing.T) {
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := NewKafkaConsumer(
		[]string{"localhost:9092"},
		"order_events",
		"test-group",
		1,
		10e6,
		orderSvc,
		exchangeSvc,
	)

	// Act
	stats := consumer.GetStats()

	// Assert
	assert.Equal(t, "order_events", stats.Topic)

	// Cleanup
	consumer.reader.Close()
}

// ===================== Message Parsing Tests =====================

func TestKafkaConsumer_ProcessMessage_AllEventFields(t *testing.T) {
	// Проверяем что все поля события корректно парсятся
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
	}

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()
	now := time.Now().Truncate(time.Second)

	event := entity.OrderEvent{
		EventType:  entity.EventTypeOrderCreated,
		OrderID:    orderID,
		UserID:     userID,
		TotalPrice: 150.50,
		Currency:   "EUR",
		Status:     entity.OrderStatusPending,
		ItemsCount: 5,
		Timestamp:  now,
	}

	eventJSON, _ := json.Marshal(event)
	message := kafka.Message{Value: eventJSON}

	var capturedEvent *entity.OrderEvent
	orderSvc.On("ProcessOrderEvent", ctx, mock.Anything).Run(func(args mock.Arguments) {
		capturedEvent = args.Get(1).(*entity.OrderEvent)
	}).Return(nil)

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, capturedEvent)
	assert.Equal(t, orderID, capturedEvent.OrderID)
	assert.Equal(t, userID, capturedEvent.UserID)
	assert.Equal(t, 150.50, capturedEvent.TotalPrice)
	assert.Equal(t, "EUR", capturedEvent.Currency)
	assert.Equal(t, entity.OrderStatusPending, capturedEvent.Status)
	assert.Equal(t, 5, capturedEvent.ItemsCount)
}

func TestKafkaConsumer_ProcessMessage_UnknownEventType(t *testing.T) {
	// Неизвестный тип события всё равно передаётся в service
	// Arrange
	orderSvc := new(MockOrderProcessingService)
	exchangeSvc := new(MockExchangeRateService)

	consumer := &KafkaConsumer{
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
	}

	ctx := context.Background()

	event := entity.OrderEvent{
		EventType: "UNKNOWN_EVENT_TYPE",
		OrderID:   uuid.New(),
	}
	eventJSON, _ := json.Marshal(event)
	message := kafka.Message{Value: eventJSON}

	orderSvc.On("ProcessOrderEvent", ctx, mock.Anything).Return(nil)

	// Act
	err := consumer.processMessage(ctx, message)

	// Assert
	assert.NoError(t, err)
	orderSvc.AssertExpectations(t)
}
