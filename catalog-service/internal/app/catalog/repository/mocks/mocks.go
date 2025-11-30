package mocks

import (
	"context"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockCategoryRepository мок для CategoryRepository
type MockCategoryRepository struct {
	mock.Mock
}

func (m *MockCategoryRepository) Create(ctx context.Context, category *entity.Category) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Category, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Category), args.Error(1)
}

func (m *MockCategoryRepository) GetAll(ctx context.Context) ([]entity.Category, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Category), args.Error(1)
}

func (m *MockCategoryRepository) Update(ctx context.Context, category *entity.Category) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockCategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockProductRepository мок для ProductRepository
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) Create(ctx context.Context, product *entity.Product) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Product), args.Error(1)
}

func (m *MockProductRepository) GetAll(ctx context.Context) ([]entity.Product, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Product), args.Error(1)
}

func (m *MockProductRepository) GetWithCategory(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ProductWithCategory), args.Error(1)
}

func (m *MockProductRepository) GetAllWithCategories(ctx context.Context) ([]entity.ProductWithCategory, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.ProductWithCategory), args.Error(1)
}

func (m *MockProductRepository) Update(ctx context.Context, product *entity.Product) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockRedisCache мок для RedisCache
type MockRedisCache struct {
	mock.Mock
}

func (m *MockRedisCache) SetCategories(ctx context.Context, categories []entity.Category, ttl time.Duration) error {
	args := m.Called(ctx, categories, ttl)
	return args.Error(0)
}

func (m *MockRedisCache) GetCategories(ctx context.Context) ([]entity.Category, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Category), args.Error(1)
}

func (m *MockRedisCache) DeleteCategories(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRedisCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockMessagePublisher мок для MessagePublisher (Kafka)
type MockMessagePublisher struct {
	mock.Mock
}

func (m *MockMessagePublisher) PublishMessage(ctx context.Context, key string, value []byte) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockMessagePublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}
