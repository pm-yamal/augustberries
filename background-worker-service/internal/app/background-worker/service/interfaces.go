package service

import (
	"context"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
)

type ExchangeRateServiceInterface interface {
	FetchAndStoreRates(ctx context.Context) error
	GetRate(ctx context.Context, currency string) (*entity.ExchangeRate, error)
	GetRates(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error)
	ConvertCurrency(ctx context.Context, amount float64, from, to string) (float64, float64, error)
	EnsureRatesAvailable(ctx context.Context) error
}

type OrderProcessingServiceInterface interface {
	ProcessOrderEvent(ctx context.Context, event *entity.OrderEvent) error
}

type ExchangeRateAPIClient interface {
	FetchRates(ctx context.Context) (map[string]float64, error)
}
