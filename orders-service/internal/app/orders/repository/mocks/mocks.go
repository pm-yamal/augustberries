package mocks

import (
	"context"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockOrderRepository мок для OrderRepository
type MockOrderRepository struct {
	mock.Mock
}

func (m *MockOrderRepository) Create(ctx context.Context, order *entity.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Order), args.Error(1)
}

func (m *MockOrderRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Order, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Order), args.Error(1)
}

func (m *MockOrderRepository) Update(ctx context.Context, order *entity.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockOrderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOrderRepository) GetWithItems(ctx context.Context, id uuid.UUID) (*entity.OrderWithItems, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.OrderWithItems), args.Error(1)
}

// MockOrderItemRepository мок для OrderItemRepository
type MockOrderItemRepository struct {
	mock.Mock
}

func (m *MockOrderItemRepository) Create(ctx context.Context, item *entity.OrderItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockOrderItemRepository) GetByOrderID(ctx context.Context, orderID uuid.UUID) ([]entity.OrderItem, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.OrderItem), args.Error(1)
}

func (m *MockOrderItemRepository) DeleteByOrderID(ctx context.Context, orderID uuid.UUID) error {
	args := m.Called(ctx, orderID)
	return args.Error(0)
}

// MockCatalogServiceClient мок для CatalogServiceClient
type MockCatalogServiceClient struct {
	mock.Mock
	AuthToken string
}

func (m *MockCatalogServiceClient) SetAuthToken(token string) {
	m.AuthToken = token
}

func (m *MockCatalogServiceClient) GetProduct(ctx context.Context, productID uuid.UUID) (*entity.ProductWithCategory, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ProductWithCategory), args.Error(1)
}

func (m *MockCatalogServiceClient) GetProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]*entity.ProductWithCategory, error) {
	args := m.Called(ctx, productIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]*entity.ProductWithCategory), args.Error(1)
}

// MockMessagePublisher мок для MessagePublisher (Kafka)
type MockMessagePublisher struct {
	mock.Mock
	Messages [][]byte
}

func (m *MockMessagePublisher) PublishMessage(ctx context.Context, key string, value []byte) error {
	m.Messages = append(m.Messages, value)
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockMessagePublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}
