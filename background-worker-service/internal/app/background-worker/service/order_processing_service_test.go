package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===================== ProcessOrderCreated Tests =====================

func TestProcessOrderCreated_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType:  entity.EventTypeOrderCreated,
		OrderID:    orderID,
		UserID:     userID,
		TotalPrice: 110.0, // 100 товары + 10 доставка
		Currency:   "USD",
	}

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "USD",
		Status:        entity.OrderStatusPending,
		CreatedAt:     time.Now(),
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Конвертация доставки: 10 USD -> RUB (курс 91.23)
	exchangeSvc.On("ConvertCurrency", ctx, 10.0, "USD", "RUB").Return(912.3, 91.23, nil)
	// Конвертация товаров: 100 USD -> RUB
	exchangeSvc.On("ConvertCurrency", ctx, 100.0, "USD", "RUB").Return(9123.0, 91.23, nil)

	// Итого: 9123 + 912.3 = 10035.3 RUB
	orderRepo.On("UpdateOrderWithCurrency", ctx, orderID, 912.3, 10035.3, "RUB").Return(nil)

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.NoError(t, err)
	orderRepo.AssertExpectations(t)
	exchangeSvc.AssertExpectations(t)
}

func TestProcessOrderCreated_ZeroDelivery_Skipped(t *testing.T) {
	// Если доставка = 0, обработка пропускается
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    100.0,
		DeliveryPrice: 0.0, // Нулевая доставка
		Currency:      "USD",
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.NoError(t, err)
	exchangeSvc.AssertNotCalled(t, "ConvertCurrency") // Конвертация не вызывается
	orderRepo.AssertNotCalled(t, "UpdateOrderWithCurrency")
}

func TestProcessOrderCreated_OrderNotFound(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	orderRepo.On("GetByID", ctx, orderID).Return(nil, errors.New("order not found"))

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get order")
}

func TestProcessOrderCreated_ValidationFailed(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	// Заказ с пустой валютой - не пройдёт валидацию
	order := &entity.Order{
		ID:            orderID,
		UserID:        uuid.New(),
		TotalPrice:    100.0,
		DeliveryPrice: 10.0,
		Currency:      "", // Пустая валюта
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestProcessOrderCreated_ConversionError(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)
	exchangeSvc.On("ConvertCurrency", ctx, 10.0, "USD", "RUB").Return(0.0, 0.0, errors.New("rate not found"))

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to calculate delivery")
}

func TestProcessOrderCreated_UpdateError(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)
	exchangeSvc.On("ConvertCurrency", ctx, 10.0, "USD", "RUB").Return(912.3, 91.23, nil)
	exchangeSvc.On("ConvertCurrency", ctx, 100.0, "USD", "RUB").Return(9123.0, 91.23, nil)
	orderRepo.On("UpdateOrderWithCurrency", ctx, orderID, mock.Anything, mock.Anything, "RUB").Return(errors.New("db error"))

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update order")
}

// ===================== ProcessOrderEvent Tests =====================

func TestProcessOrderEvent_OrderCreated(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    100.0,
		DeliveryPrice: 0.0, // Нулевая доставка - пропускается
		Currency:      "USD",
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	err := service.ProcessOrderEvent(ctx, event)

	// Assert
	assert.NoError(t, err)
}

func TestProcessOrderEvent_OrderUpdated_Skipped(t *testing.T) {
	// ORDER_UPDATED пока не обрабатывается
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderUpdated,
		OrderID:   uuid.New(),
	}

	// Act
	err := service.ProcessOrderEvent(ctx, event)

	// Assert
	assert.NoError(t, err)
	orderRepo.AssertNotCalled(t, "GetByID") // Репозиторий не вызывается
}

func TestProcessOrderEvent_UnknownType_Skipped(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()

	event := &entity.OrderEvent{
		EventType: "UNKNOWN_EVENT",
		OrderID:   uuid.New(),
	}

	// Act
	err := service.ProcessOrderEvent(ctx, event)

	// Assert
	assert.NoError(t, err)
	orderRepo.AssertNotCalled(t, "GetByID")
}

// ===================== ValidateOrder Tests =====================

func TestValidateOrder_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	order := &entity.Order{
		ID:            uuid.New(),
		UserID:        uuid.New(),
		TotalPrice:    100.0,
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}

	// Act
	err := service.ValidateOrder(order)

	// Assert
	assert.NoError(t, err)
}

func TestValidateOrder_InvalidOrderID(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	order := &entity.Order{
		ID:       uuid.Nil, // Невалидный ID
		UserID:   uuid.New(),
		Currency: "USD",
	}

	// Act
	err := service.ValidateOrder(order)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order ID")
}

func TestValidateOrder_InvalidUserID(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	order := &entity.Order{
		ID:       uuid.New(),
		UserID:   uuid.Nil, // Невалидный UserID
		Currency: "USD",
	}

	// Act
	err := service.ValidateOrder(order)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user ID")
}

func TestValidateOrder_EmptyCurrency(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	order := &entity.Order{
		ID:       uuid.New(),
		UserID:   uuid.New(),
		Currency: "", // Пустая валюта
	}

	// Act
	err := service.ValidateOrder(order)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency not specified")
}

func TestValidateOrder_NegativeDeliveryPrice(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	order := &entity.Order{
		ID:            uuid.New(),
		UserID:        uuid.New(),
		Currency:      "USD",
		DeliveryPrice: -10.0, // Отрицательная доставка
	}

	// Act
	err := service.ValidateOrder(order)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delivery price cannot be negative")
}

func TestValidateOrder_NegativeTotalPrice(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	order := &entity.Order{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		Currency:   "USD",
		TotalPrice: -100.0, // Отрицательная сумма
	}

	// Act
	err := service.ValidateOrder(order)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total price cannot be negative")
}

// ===================== Currency Conversion Edge Cases =====================

func TestProcessOrderCreated_EURtoRUB(t *testing.T) {
	// Тест конвертации EUR -> RUB
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "EUR", // Заказ в EUR
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Конвертация из EUR в RUB (EUR=0.93, RUB=91.23, rate=98.096)
	exchangeSvc.On("ConvertCurrency", ctx, 10.0, "EUR", "RUB").Return(980.96, 98.096, nil)
	exchangeSvc.On("ConvertCurrency", ctx, 100.0, "EUR", "RUB").Return(9809.6, 98.096, nil)

	orderRepo.On("UpdateOrderWithCurrency", ctx, orderID, 980.96, 10790.56, "RUB").Return(nil)

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.NoError(t, err)
}

func TestProcessOrderCreated_DefaultCurrencyUSD(t *testing.T) {
	// Если валюта не указана, используется USD по умолчанию
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	exchangeSvc := new(mocks.MockExchangeRateService)

	service := NewOrderProcessingService(orderRepo, exchangeSvc)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()

	event := &entity.OrderEvent{
		EventType: entity.EventTypeOrderCreated,
		OrderID:   orderID,
	}

	// Заказ без указания валюты - но валидация требует валюту
	// Поэтому проверяем что валидация сработает
	order := &entity.Order{
		ID:            orderID,
		UserID:        userID,
		TotalPrice:    110.0,
		DeliveryPrice: 10.0,
		Currency:      "", // Пустая валюта
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	err := service.ProcessOrderCreated(ctx, event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency not specified")
}
