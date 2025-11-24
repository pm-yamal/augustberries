package service

import (
	"context"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
)

// ExchangeRateServiceInterface определяет интерфейс для работы с курсами валют
type ExchangeRateServiceInterface interface {
	// FetchAndStoreRates получает курсы валют из внешнего API и сохраняет в Redis
	FetchAndStoreRates(ctx context.Context) error
	// GetRate получает курс валюты из Redis
	GetRate(ctx context.Context, currency string) (*entity.ExchangeRate, error)
	// GetRates получает курсы нескольких валют из Redis
	GetRates(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error)
	// ConvertCurrency конвертирует сумму из одной валюты в другую
	ConvertCurrency(ctx context.Context, amount float64, from, to string) (float64, float64, error)
	// EnsureRatesAvailable проверяет наличие курсов в Redis
	EnsureRatesAvailable(ctx context.Context) error
}

// OrderProcessingServiceInterface определяет интерфейс для обработки заказов
type OrderProcessingServiceInterface interface {
	// ProcessOrderEvent обрабатывает событие заказа из Kafka
	ProcessOrderEvent(ctx context.Context, event *entity.OrderEvent) error
}

// ExchangeRateAPIClient определяет интерфейс для взаимодействия с внешним API курсов валют
type ExchangeRateAPIClient interface {
	// FetchRates получает курсы валют из внешнего API
	FetchRates(ctx context.Context) (map[string]float64, error)
}
