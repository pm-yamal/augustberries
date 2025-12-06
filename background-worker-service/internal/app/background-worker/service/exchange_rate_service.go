package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
	"augustberries/pkg/metrics"
)

type ExchangeRateService struct {
	rateRepo  repository.ExchangeRateRepository
	apiClient ExchangeRateAPIClient
}

func NewExchangeRateService(
	rateRepo repository.ExchangeRateRepository,
	apiClient ExchangeRateAPIClient,
) *ExchangeRateService {
	return &ExchangeRateService{
		rateRepo:  rateRepo,
		apiClient: apiClient,
	}
}

func (s *ExchangeRateService) FetchAndStoreRates(ctx context.Context) error {
	log.Println("Fetching exchange rates from API...")

	rates, err := s.apiClient.FetchRates(ctx)
	if err != nil {
		log.Printf("WARNING: Failed to fetch rates from API: %v", err)
		metrics.WorkerExchangeRateUpdates.WithLabelValues("failed").Inc()
		return nil
	}

	exchangeRates := make([]*entity.ExchangeRate, 0, len(rates))
	now := time.Now()

	for currency, rate := range rates {
		exchangeRates = append(exchangeRates, &entity.ExchangeRate{
			Currency:  currency,
			Rate:      rate,
			UpdatedAt: now,
		})
	}

	if err := s.rateRepo.SetMultiple(ctx, exchangeRates); err != nil {
		metrics.WorkerExchangeRateUpdates.WithLabelValues("failed").Inc()
		return fmt.Errorf("failed to store rates in redis: %w", err)
	}

	metrics.WorkerExchangeRateUpdates.WithLabelValues("success").Inc()
	log.Printf("Successfully stored %d exchange rates", len(exchangeRates))
	return nil
}

func (s *ExchangeRateService) GetRate(ctx context.Context, currency string) (*entity.ExchangeRate, error) {
	rate, err := s.rateRepo.Get(ctx, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate for %s: %w", currency, err)
	}

	age := time.Since(rate.UpdatedAt)
	if age > 2*time.Hour {
		log.Printf("WARNING: Using outdated exchange rate for %s (age: %v)", currency, age)
	}

	return rate, nil
}

func (s *ExchangeRateService) GetRates(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error) {
	rates, err := s.rateRepo.GetMultiple(ctx, currencies)
	if err != nil {
		return nil, fmt.Errorf("failed to get rates: %w", err)
	}
	return rates, nil
}

func (s *ExchangeRateService) ConvertCurrency(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, float64, error) {
	if fromCurrency == toCurrency {
		return amount, 1.0, nil
	}

	rates, err := s.GetRates(ctx, []string{fromCurrency, toCurrency})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get rates for conversion: %w", err)
	}

	fromRate, ok := rates[fromCurrency]
	if !ok {
		return 0, 0, fmt.Errorf("rate for %s not found", fromCurrency)
	}

	toRate, ok := rates[toCurrency]
	if !ok {
		return 0, 0, fmt.Errorf("rate for %s not found", toCurrency)
	}

	exchangeRate := toRate.Rate / fromRate.Rate
	convertedAmount := amount * exchangeRate

	return convertedAmount, exchangeRate, nil
}

func (s *ExchangeRateService) EnsureRatesAvailable(ctx context.Context) error {
	for _, currency := range entity.SupportedCurrencies {
		exists, err := s.rateRepo.Exists(ctx, currency)
		if err != nil {
			return fmt.Errorf("failed to check rate existence: %w", err)
		}

		if !exists {
			log.Printf("Rate for %s not found, fetching from API...", currency)
			return s.FetchAndStoreRates(ctx)
		}
	}
	return nil
}
