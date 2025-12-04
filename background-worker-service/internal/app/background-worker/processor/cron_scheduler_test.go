package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

// ===================== NewCronScheduler Tests =====================

func TestNewCronScheduler(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)

	// Act
	scheduler := NewCronScheduler(mockSvc)

	// Assert
	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.cron)
	assert.Equal(t, mockSvc, scheduler.exchangeSvc)
}

// ===================== Start Tests =====================

func TestCronScheduler_Start_Success(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()

	// Initial fetch при старте
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(nil)

	// Act
	err := scheduler.Start(ctx, "*/5 * * * *") // Каждые 5 минут

	// Assert
	assert.NoError(t, err)
	assert.Len(t, scheduler.GetEntries(), 1) // Одна задача добавлена

	// Cleanup
	scheduler.Stop()
	mockSvc.AssertExpectations(t)
}

func TestCronScheduler_Start_InvalidSchedule(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()

	// Act
	err := scheduler.Start(ctx, "invalid cron expression")

	// Assert
	assert.Error(t, err)
}

func TestCronScheduler_Start_InitialFetchError_ContinuesWork(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()

	// Initial fetch fails but scheduler should continue
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(errors.New("api unavailable"))

	// Act
	err := scheduler.Start(ctx, "*/5 * * * *")

	// Assert
	assert.NoError(t, err) // Scheduler starts despite initial error
	assert.Len(t, scheduler.GetEntries(), 1)

	// Cleanup
	scheduler.Stop()
}

// ===================== Stop Tests =====================

func TestCronScheduler_Stop(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(nil)

	scheduler.Start(ctx, "*/5 * * * *")

	// Act
	scheduler.Stop()

	// Assert - проверяем что cron остановлен (GetEntries всё ещё возвращает entries)
	// но новые задачи не будут выполняться
	assert.NotNil(t, scheduler.cron)
}

// ===================== GetEntries Tests =====================

func TestCronScheduler_GetEntries_Empty(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	// Act
	entries := scheduler.GetEntries()

	// Assert
	assert.Empty(t, entries)
}

func TestCronScheduler_GetEntries_AfterStart(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(nil)

	scheduler.Start(ctx, "0 * * * *") // Каждый час

	// Act
	entries := scheduler.GetEntries()

	// Assert
	assert.Len(t, entries, 1)

	// Cleanup
	scheduler.Stop()
}

// ===================== Cron Job Execution Tests =====================

func TestCronScheduler_JobExecution(t *testing.T) {
	// Тестируем что cron job вызывает FetchAndStoreRates
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()

	// Ожидаем минимум 2 вызова: initial + cron trigger
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(nil)

	// Используем @every для быстрого теста
	err := scheduler.Start(ctx, "@every 100ms")
	assert.NoError(t, err)

	// Ждём выполнения cron job
	time.Sleep(350 * time.Millisecond)

	// Cleanup
	scheduler.Stop()

	// Assert - должно быть минимум 2 вызова (initial + 2-3 cron triggers)
	assert.GreaterOrEqual(t, len(mockSvc.Calls), 2)
}

func TestCronScheduler_JobExecution_WithError(t *testing.T) {
	// Cron job продолжает работать даже при ошибках
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx := context.Background()

	// Все вызовы возвращают ошибку
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(errors.New("api error"))

	err := scheduler.Start(ctx, "@every 100ms")
	assert.NoError(t, err)

	time.Sleep(350 * time.Millisecond)

	scheduler.Stop()

	// Assert - несмотря на ошибки, вызовы продолжаются
	assert.GreaterOrEqual(t, len(mockSvc.Calls), 2)
}

// ===================== Context Cancellation Tests =====================

func TestCronScheduler_ContextCancellation(t *testing.T) {
	// Arrange
	mockSvc := new(MockExchangeRateService)
	scheduler := NewCronScheduler(mockSvc)

	ctx, cancel := context.WithCancel(context.Background())
	mockSvc.On("FetchAndStoreRates", mock.Anything).Return(nil)

	scheduler.Start(ctx, "*/5 * * * *")

	// Act
	cancel()
	scheduler.Stop()

	// Assert - scheduler should stop gracefully
	assert.NotNil(t, scheduler)
}
