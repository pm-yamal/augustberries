package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"augustberries/orders-service/internal/app/orders/entity"
	"augustberries/orders-service/internal/app/orders/repository"
	"augustberries/orders-service/internal/app/orders/repository/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===================== CreateOrder Tests =====================

func TestCreateOrder_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	productID := uuid.New()
	authToken := "test-token"

	req := &entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 2},
		},
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}

	// Mock Catalog Service
	products := map[uuid.UUID]*entity.ProductWithCategory{
		productID: {
			Product: entity.Product{
				ID:    productID,
				Name:  "Test Product",
				Price: 50.0,
			},
		},
	}
	catalogClient.On("GetProducts", ctx, []uuid.UUID{productID}).Return(products, nil)

	// Mock repository
	orderRepo.On("Create", ctx, mock.AnythingOfType("*entity.Order")).Return(nil)
	orderItemRepo.On("Create", ctx, mock.AnythingOfType("*entity.OrderItem")).Return(nil)
	kafkaProducer.On("PublishMessage", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	// Act
	result, err := service.CreateOrder(ctx, userID, req, authToken)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, entity.OrderStatusPending, result.Status)
	assert.Equal(t, "USD", result.Currency)
	// TotalPrice = (50.0 * 2) + 10.0 = 110.0
	assert.Equal(t, 110.0, result.TotalPrice)
	assert.Len(t, result.Items, 1)

	orderRepo.AssertExpectations(t)
	orderItemRepo.AssertExpectations(t)
	catalogClient.AssertExpectations(t)
}

func TestCreateOrder_ProductNotFound(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	productID := uuid.New()
	authToken := "test-token"

	req := &entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 1},
		},
		DeliveryPrice: 5.0,
		Currency:      "USD",
	}

	// Mock: товар не найден в Catalog Service
	catalogClient.On("GetProducts", ctx, []uuid.UUID{productID}).Return(map[uuid.UUID]*entity.ProductWithCategory{}, nil)

	// Act
	result, err := service.CreateOrder(ctx, userID, req, authToken)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestCreateOrder_CatalogServiceError(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	productID := uuid.New()

	req := &entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 1},
		},
		DeliveryPrice: 5.0,
		Currency:      "USD",
	}

	catalogClient.On("GetProducts", ctx, []uuid.UUID{productID}).Return(nil, errors.New("catalog service unavailable"))

	// Act
	result, err := service.CreateOrder(ctx, userID, req, "token")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get products from catalog")
}

func TestCreateOrder_OrderRepoError(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	productID := uuid.New()

	req := &entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 1},
		},
		DeliveryPrice: 5.0,
		Currency:      "USD",
	}

	products := map[uuid.UUID]*entity.ProductWithCategory{
		productID: {Product: entity.Product{ID: productID, Price: 100.0}},
	}
	catalogClient.On("GetProducts", ctx, []uuid.UUID{productID}).Return(products, nil)
	orderRepo.On("Create", ctx, mock.Anything).Return(errors.New("db error"))

	// Act
	result, err := service.CreateOrder(ctx, userID, req, "token")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create order")
}

func TestCreateOrder_KafkaErrorIgnored(t *testing.T) {
	// Ошибка Kafka не должна прерывать создание заказа
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	productID := uuid.New()

	req := &entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 1},
		},
		DeliveryPrice: 5.0,
		Currency:      "RUB",
	}

	products := map[uuid.UUID]*entity.ProductWithCategory{
		productID: {Product: entity.Product{ID: productID, Price: 1000.0}},
	}
	catalogClient.On("GetProducts", ctx, []uuid.UUID{productID}).Return(products, nil)
	orderRepo.On("Create", ctx, mock.Anything).Return(nil)
	orderItemRepo.On("Create", ctx, mock.Anything).Return(nil)
	kafkaProducer.On("PublishMessage", ctx, mock.Anything, mock.Anything).Return(errors.New("kafka error"))

	// Act
	result, err := service.CreateOrder(ctx, userID, req, "token")

	// Assert
	assert.NoError(t, err) // Заказ создан несмотря на ошибку Kafka
	assert.NotNil(t, result)
}

func TestCreateOrder_MultipleItems(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	productID1 := uuid.New()
	productID2 := uuid.New()

	req := &entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID1, Quantity: 2},
			{ProductID: productID2, Quantity: 3},
		},
		DeliveryPrice: 15.0,
		Currency:      "USD",
	}

	products := map[uuid.UUID]*entity.ProductWithCategory{
		productID1: {Product: entity.Product{ID: productID1, Price: 100.0}},
		productID2: {Product: entity.Product{ID: productID2, Price: 50.0}},
	}
	catalogClient.On("GetProducts", ctx, mock.Anything).Return(products, nil)
	orderRepo.On("Create", ctx, mock.Anything).Return(nil)
	orderItemRepo.On("Create", ctx, mock.Anything).Return(nil).Times(2)
	kafkaProducer.On("PublishMessage", ctx, mock.Anything, mock.Anything).Return(nil)

	// Act
	result, err := service.CreateOrder(ctx, userID, req, "token")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result.Items, 2)
	// TotalPrice = (100*2) + (50*3) + 15 = 200 + 150 + 15 = 365
	assert.Equal(t, 365.0, result.TotalPrice)
}

// ===================== GetOrder Tests =====================

func TestGetOrder_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	order := &entity.OrderWithItems{
		Order: entity.Order{
			ID:         orderID,
			UserID:     userID,
			TotalPrice: 100.0,
			Status:     entity.OrderStatusPending,
		},
		Items: []entity.OrderItem{
			{ID: uuid.New(), OrderID: orderID, ProductID: uuid.New(), Quantity: 1, UnitPrice: 100.0},
		},
	}

	orderRepo.On("GetWithItems", ctx, orderID).Return(order, nil)

	// Act
	result, err := service.GetOrder(ctx, orderID, userID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, orderID, result.ID)
	assert.Len(t, result.Items, 1)
}

func TestGetOrder_NotFound(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	orderRepo.On("GetWithItems", ctx, orderID).Return(nil, repository.ErrOrderNotFound)

	// Act
	result, err := service.GetOrder(ctx, orderID, userID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestGetOrder_Unauthorized(t *testing.T) {
	// Попытка получить чужой заказ
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	ownerID := uuid.New()
	anotherUserID := uuid.New()
	orderID := uuid.New()

	order := &entity.OrderWithItems{
		Order: entity.Order{
			ID:     orderID,
			UserID: ownerID, // Владелец - другой пользователь
		},
	}

	orderRepo.On("GetWithItems", ctx, orderID).Return(order, nil)

	// Act
	result, err := service.GetOrder(ctx, orderID, anotherUserID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

// ===================== UpdateOrderStatus Tests =====================

func TestUpdateOrderStatus_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	order := &entity.Order{
		ID:         orderID,
		UserID:     userID,
		Status:     entity.OrderStatusPending,
		TotalPrice: 100.0,
		Currency:   "USD",
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)
	orderRepo.On("Update", ctx, mock.AnythingOfType("*entity.Order")).Return(nil)
	orderItemRepo.On("GetByOrderID", ctx, orderID).Return([]entity.OrderItem{}, nil)
	kafkaProducer.On("PublishMessage", ctx, mock.Anything, mock.Anything).Return(nil)

	// Act
	result, err := service.UpdateOrderStatus(ctx, orderID, userID, entity.OrderStatusConfirmed)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, entity.OrderStatusConfirmed, result.Status)
}

func TestUpdateOrderStatus_NotFound(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	orderRepo.On("GetByID", ctx, orderID).Return(nil, repository.ErrOrderNotFound)

	// Act
	result, err := service.UpdateOrderStatus(ctx, orderID, userID, entity.OrderStatusConfirmed)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestUpdateOrderStatus_Unauthorized(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	ownerID := uuid.New()
	anotherUserID := uuid.New()
	orderID := uuid.New()

	order := &entity.Order{
		ID:     orderID,
		UserID: ownerID,
		Status: entity.OrderStatusPending,
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	result, err := service.UpdateOrderStatus(ctx, orderID, anotherUserID, entity.OrderStatusConfirmed)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestUpdateOrderStatus_InvalidTransition(t *testing.T) {
	// Попытка невалидного перехода: delivered -> pending
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	order := &entity.Order{
		ID:     orderID,
		UserID: userID,
		Status: entity.OrderStatusDelivered, // Финальный статус
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	result, err := service.UpdateOrderStatus(ctx, orderID, userID, entity.OrderStatusPending)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrInvalidOrderStatus)
}

// ===================== Status Transitions Tests =====================

func TestStatusTransitions(t *testing.T) {
	testCases := []struct {
		name     string
		from     entity.OrderStatus
		to       entity.OrderStatus
		expected bool
	}{
		{"pending -> confirmed", entity.OrderStatusPending, entity.OrderStatusConfirmed, true},
		{"pending -> cancelled", entity.OrderStatusPending, entity.OrderStatusCancelled, true},
		{"pending -> shipped", entity.OrderStatusPending, entity.OrderStatusShipped, false},
		{"pending -> delivered", entity.OrderStatusPending, entity.OrderStatusDelivered, false},
		{"confirmed -> shipped", entity.OrderStatusConfirmed, entity.OrderStatusShipped, true},
		{"confirmed -> cancelled", entity.OrderStatusConfirmed, entity.OrderStatusCancelled, true},
		{"confirmed -> pending", entity.OrderStatusConfirmed, entity.OrderStatusPending, false},
		{"shipped -> delivered", entity.OrderStatusShipped, entity.OrderStatusDelivered, true},
		{"shipped -> cancelled", entity.OrderStatusShipped, entity.OrderStatusCancelled, false},
		{"delivered -> any", entity.OrderStatusDelivered, entity.OrderStatusPending, false},
		{"cancelled -> any", entity.OrderStatusCancelled, entity.OrderStatusPending, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidStatusTransition(tc.from, tc.to)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ===================== DeleteOrder Tests =====================

func TestDeleteOrder_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	order := &entity.Order{
		ID:     orderID,
		UserID: userID,
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)
	orderRepo.On("Delete", ctx, orderID).Return(nil)

	// Act
	err := service.DeleteOrder(ctx, orderID, userID)

	// Assert
	assert.NoError(t, err)
	orderRepo.AssertExpectations(t)
}

func TestDeleteOrder_NotFound(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()
	orderID := uuid.New()

	orderRepo.On("GetByID", ctx, orderID).Return(nil, repository.ErrOrderNotFound)

	// Act
	err := service.DeleteOrder(ctx, orderID, userID)

	// Assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestDeleteOrder_Unauthorized(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	ownerID := uuid.New()
	anotherUserID := uuid.New()
	orderID := uuid.New()

	order := &entity.Order{
		ID:     orderID,
		UserID: ownerID,
	}

	orderRepo.On("GetByID", ctx, orderID).Return(order, nil)

	// Act
	err := service.DeleteOrder(ctx, orderID, anotherUserID)

	// Assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

// ===================== GetUserOrders Tests =====================

func TestGetUserOrders_Success(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()

	orders := []entity.Order{
		{ID: uuid.New(), UserID: userID, TotalPrice: 100.0, Status: entity.OrderStatusPending, CreatedAt: time.Now()},
		{ID: uuid.New(), UserID: userID, TotalPrice: 200.0, Status: entity.OrderStatusDelivered, CreatedAt: time.Now()},
	}

	orderRepo.On("GetByUserID", ctx, userID).Return(orders, nil)

	// Act
	result, err := service.GetUserOrders(ctx, userID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetUserOrders_Empty(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()

	orderRepo.On("GetByUserID", ctx, userID).Return([]entity.Order{}, nil)

	// Act
	result, err := service.GetUserOrders(ctx, userID)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetUserOrders_RepoError(t *testing.T) {
	// Arrange
	orderRepo := new(mocks.MockOrderRepository)
	orderItemRepo := new(mocks.MockOrderItemRepository)
	catalogClient := new(mocks.MockCatalogServiceClient)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}

	service := NewOrderService(orderRepo, orderItemRepo, catalogClient, kafkaProducer)

	ctx := context.Background()
	userID := uuid.New()

	orderRepo.On("GetByUserID", ctx, userID).Return(nil, errors.New("db error"))

	// Act
	result, err := service.GetUserOrders(ctx, userID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}
