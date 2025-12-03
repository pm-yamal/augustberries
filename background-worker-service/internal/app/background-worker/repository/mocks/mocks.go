package mocks

import (
	"context"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockOrderRepository мок для OrderRepository
type MockOrderRepository struct {
	mock.Mock
}

func (m *MockOrderRepository) GetByID(ctx context.Context, orderID uuid.UUID) (*entity.Order, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Order), args.Error(1)
}

func (m *MockOrderRepository) Update(ctx context.Context, order *entity.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockOrderRepository) UpdateDeliveryAndTotal(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64) error {
	args := m.Called(ctx, orderID, deliveryPrice, totalPrice)
	return args.Error(0)
}

func (m *MockOrderRepository) UpdateOrderWithCurrency(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64, currency string) error {
	args := m.Called(ctx, orderID, deliveryPrice, totalPrice, currency)
	return args.Error(0)
}

// MockExchangeRateRepository мок для ExchangeRateRepository
type MockExchangeRateRepository struct {
	mock.Mock
}

func (m *MockExchangeRateRepository) Get(ctx context.Context, currency string) (*entity.ExchangeRate, error) {
	args := m.Called(ctx, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ExchangeRate), args.Error(1)
}

func (m *MockExchangeRateRepository) Set(ctx context.Context, rate *entity.ExchangeRate) error {
	args := m.Called(ctx, rate)
	return args.Error(0)
}

func (m *MockExchangeRateRepository) SetMultiple(ctx context.Context, rates []*entity.ExchangeRate) error {
	args := m.Called(ctx, rates)
	return args.Error(0)
}

func (m *MockExchangeRateRepository) GetMultiple(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error) {
	args := m.Called(ctx, currencies)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*entity.ExchangeRate), args.Error(1)
}

func (m *MockExchangeRateRepository) Exists(ctx context.Context, currency string) (bool, error) {
	args := m.Called(ctx, currency)
	return args.Bool(0), args.Error(1)
}

// MockExchangeRateAPIClient мок для ExchangeRateAPIClient
type MockExchangeRateAPIClient struct {
	mock.Mock
}

func (m *MockExchangeRateAPIClient) FetchRates(ctx context.Context) (map[string]float64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]float64), args.Error(1)
}

// MockExchangeRateService мок для ExchangeRateServiceInterface
type MockExchangeRateService struct {
	mock.Mock
}

func (m *MockExchangeRateService) FetchAndStoreRates(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockExchangeRateService) GetRate(ctx context.Context, currency string) (*entity.ExchangeRate, error) {
	args := m.Called(ctx, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ExchangeRate), args.Error(1)
}

func (m *MockExchangeRateService) GetRates(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error) {
	args := m.Called(ctx, currencies)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*entity.ExchangeRate), args.Error(1)
}

func (m *MockExchangeRateService) ConvertCurrency(ctx context.Context, amount float64, from, to string) (float64, float64, error) {
	args := m.Called(ctx, amount, from, to)
	return args.Get(0).(float64), args.Get(1).(float64), args.Error(2)
}

func (m *MockExchangeRateService) EnsureRatesAvailable(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
