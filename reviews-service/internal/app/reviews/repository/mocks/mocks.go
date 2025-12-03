package mocks

import (
	"context"

	"augustberries/reviews-service/internal/app/reviews/entity"

	"github.com/stretchr/testify/mock"
)

// MockReviewRepository мок для ReviewRepository
type MockReviewRepository struct {
	mock.Mock
}

func (m *MockReviewRepository) Create(ctx context.Context, review *entity.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewRepository) GetByProductID(ctx context.Context, productID string) ([]entity.Review, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Review), args.Error(1)
}

func (m *MockReviewRepository) GetByID(ctx context.Context, id string) (*entity.Review, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Review), args.Error(1)
}

func (m *MockReviewRepository) Update(ctx context.Context, review *entity.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewRepository) GetByUserID(ctx context.Context, userID string) ([]entity.Review, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Review), args.Error(1)
}

// MockMessagePublisher мок для Kafka MessagePublisher
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
