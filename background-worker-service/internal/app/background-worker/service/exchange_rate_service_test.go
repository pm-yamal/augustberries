package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===================== FetchAndStoreRates Tests =====================

func TestFetchAndStoreRates_Success(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	apiRates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.93,
		"RUB": 91.23,
	}

	apiClient.On("FetchRates", ctx).Return(apiRates, nil)
	rateRepo.On("SetMultiple", ctx, mock.AnythingOfType("[]*entity.ExchangeRate")).Return(nil)

	// Act
	err := service.FetchAndStoreRates(ctx)

	// Assert
	assert.NoError(t, err)
	apiClient.AssertExpectations(t)
	rateRepo.AssertExpectations(t)
}

func TestFetchAndStoreRates_APIError_ContinuesWithCache(t *testing.T) {
	// API недоступен - worker продолжает работу с кэшированными курсами
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	apiClient.On("FetchRates", ctx).Return(nil, errors.New("api unavailable"))

	// Act
	err := service.FetchAndStoreRates(ctx)

	// Assert
	assert.NoError(t, err) // Не возвращает ошибку, fallback на кэш
	apiClient.AssertExpectations(t)
	rateRepo.AssertNotCalled(t, "SetMultiple") // Репозиторий не вызывается
}

func TestFetchAndStoreRates_RedisError(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	apiRates := map[string]float64{"USD": 1.0}

	apiClient.On("FetchRates", ctx).Return(apiRates, nil)
	rateRepo.On("SetMultiple", ctx, mock.Anything).Return(errors.New("redis error"))

	// Act
	err := service.FetchAndStoreRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store rates")
}

// ===================== GetRate Tests =====================

func TestGetRate_Success(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	expectedRate := &entity.ExchangeRate{
		Currency:  "USD",
		Rate:      1.0,
		UpdatedAt: time.Now(),
	}

	rateRepo.On("Get", ctx, "USD").Return(expectedRate, nil)

	// Act
	rate, err := service.GetRate(ctx, "USD")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedRate.Currency, rate.Currency)
	assert.Equal(t, expectedRate.Rate, rate.Rate)
}

func TestGetRate_NotFound(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	rateRepo.On("Get", ctx, "XYZ").Return(nil, errors.New("rate not found"))

	// Act
	rate, err := service.GetRate(ctx, "XYZ")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rate)
}

// ===================== ConvertCurrency Tests =====================

func TestConvertCurrency_Success(t *testing.T) {
	// Конвертация 100 USD -> RUB при курсе USD=1, RUB=91.23
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	rates := map[string]*entity.ExchangeRate{
		"USD": {Currency: "USD", Rate: 1.0, UpdatedAt: time.Now()},
		"RUB": {Currency: "RUB", Rate: 91.23, UpdatedAt: time.Now()},
	}

	rateRepo.On("GetMultiple", ctx, []string{"USD", "RUB"}).Return(rates, nil)

	// Act
	converted, exchangeRate, err := service.ConvertCurrency(ctx, 100.0, "USD", "RUB")

	// Assert
	assert.NoError(t, err)
	assert.InDelta(t, 9123.0, converted, 0.01) // 100 * 91.23 = 9123
	assert.InDelta(t, 91.23, exchangeRate, 0.01)
}

func TestConvertCurrency_SameCurrency(t *testing.T) {
	// Конвертация USD -> USD должна вернуть ту же сумму
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	// Act
	converted, exchangeRate, err := service.ConvertCurrency(ctx, 100.0, "USD", "USD")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 100.0, converted)
	assert.Equal(t, 1.0, exchangeRate)
	rateRepo.AssertNotCalled(t, "GetMultiple") // Репозиторий не должен вызываться
}

func TestConvertCurrency_EURtoRUB(t *testing.T) {
	// Конвертация 100 EUR -> RUB при курсах EUR=0.93, RUB=91.23
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	rates := map[string]*entity.ExchangeRate{
		"EUR": {Currency: "EUR", Rate: 0.93, UpdatedAt: time.Now()},
		"RUB": {Currency: "RUB", Rate: 91.23, UpdatedAt: time.Now()},
	}

	rateRepo.On("GetMultiple", ctx, []string{"EUR", "RUB"}).Return(rates, nil)

	// Act
	converted, exchangeRate, err := service.ConvertCurrency(ctx, 100.0, "EUR", "RUB")

	// Assert
	assert.NoError(t, err)
	// 100 EUR * (91.23 / 0.93) = 100 * 98.096... = 9809.67...
	expectedRate := 91.23 / 0.93
	assert.InDelta(t, 100.0*expectedRate, converted, 0.01)
	assert.InDelta(t, expectedRate, exchangeRate, 0.01)
}

func TestConvertCurrency_FromCurrencyNotFound(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	rates := map[string]*entity.ExchangeRate{
		"RUB": {Currency: "RUB", Rate: 91.23, UpdatedAt: time.Now()},
		// USD отсутствует
	}

	rateRepo.On("GetMultiple", ctx, []string{"USD", "RUB"}).Return(rates, nil)

	// Act
	_, _, err := service.ConvertCurrency(ctx, 100.0, "USD", "RUB")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate for USD not found")
}

func TestConvertCurrency_ToCurrencyNotFound(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	rates := map[string]*entity.ExchangeRate{
		"USD": {Currency: "USD", Rate: 1.0, UpdatedAt: time.Now()},
		// RUB отсутствует
	}

	rateRepo.On("GetMultiple", ctx, []string{"USD", "RUB"}).Return(rates, nil)

	// Act
	_, _, err := service.ConvertCurrency(ctx, 100.0, "USD", "RUB")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate for RUB not found")
}

func TestConvertCurrency_RepoError(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	rateRepo.On("GetMultiple", ctx, []string{"USD", "RUB"}).Return(nil, errors.New("redis error"))

	// Act
	_, _, err := service.ConvertCurrency(ctx, 100.0, "USD", "RUB")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get rates")
}

// ===================== EnsureRatesAvailable Tests =====================

func TestEnsureRatesAvailable_AllExist(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	// Все валюты существуют
	for _, currency := range entity.SupportedCurrencies {
		rateRepo.On("Exists", ctx, currency).Return(true, nil)
	}

	// Act
	err := service.EnsureRatesAvailable(ctx)

	// Assert
	assert.NoError(t, err)
	apiClient.AssertNotCalled(t, "FetchRates") // API не вызывается
}

func TestEnsureRatesAvailable_MissingRate_FetchesFromAPI(t *testing.T) {
	// Arrange
	rateRepo := new(mocks.MockExchangeRateRepository)
	apiClient := new(mocks.MockExchangeRateAPIClient)

	service := NewExchangeRateService(rateRepo, apiClient)

	ctx := context.Background()

	// USD отсутствует
	rateRepo.On("Exists", ctx, "USD").Return(false, nil)

	apiRates := map[string]float64{"USD": 1.0, "RUB": 91.23}
	apiClient.On("FetchRates", ctx).Return(apiRates, nil)
	rateRepo.On("SetMultiple", ctx, mock.Anything).Return(nil)

	// Act
	err := service.EnsureRatesAvailable(ctx)

	// Assert
	assert.NoError(t, err)
	apiClient.AssertExpectations(t)
}
