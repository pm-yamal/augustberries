package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
)

// ExchangeRateService управляет получением и хранением курсов валют
type ExchangeRateService struct {
	rateRepo   repository.ExchangeRateRepository
	apiURL     string
	apiTimeout time.Duration
	httpClient *http.Client
}

// NewExchangeRateService создает новый сервис курсов валют
func NewExchangeRateService(
	rateRepo repository.ExchangeRateRepository,
	apiURL string,
	apiTimeoutSec int,
) *ExchangeRateService {
	return &ExchangeRateService{
		rateRepo:   rateRepo,
		apiURL:     apiURL,
		apiTimeout: time.Duration(apiTimeoutSec) * time.Second,
		httpClient: &http.Client{
			Timeout: time.Duration(apiTimeoutSec) * time.Second,
		},
	}
}

// FetchAndStoreRates получает курсы валют из внешнего API и сохраняет в Redis
// Вызывается по cron расписанию
func (s *ExchangeRateService) FetchAndStoreRates(ctx context.Context) error {
	log.Println("Starting to fetch exchange rates from API...")

	// Получаем курсы из внешнего API
	rates, err := s.fetchRatesFromAPI(ctx)
	if err != nil {
		log.Printf("WARNING: Failed to fetch rates from API: %v", err)
		log.Println("Worker will continue using cached rates if available")

		// Не возвращаем ошибку, чтобы worker продолжал работать с кэшированными курсами
		// Это fallback механизм
		return nil
	}

	// Преобразуем в entity.ExchangeRate
	exchangeRates := make([]*entity.ExchangeRate, 0, len(rates))
	now := time.Now()

	for currency, rate := range rates {
		exchangeRates = append(exchangeRates, &entity.ExchangeRate{
			Currency:  currency,
			Rate:      rate,
			UpdatedAt: now,
		})
	}

	// Сохраняем все курсы в Redis батчем
	if err := s.rateRepo.SetMultiple(ctx, exchangeRates); err != nil {
		return fmt.Errorf("failed to store rates in redis: %w", err)
	}

	log.Printf("Successfully fetched and stored %d exchange rates", len(exchangeRates))
	return nil
}

// fetchRatesFromAPI получает курсы валют из внешнего API
func (s *ExchangeRateService) fetchRatesFromAPI(ctx context.Context) (map[string]float64, error) {
	// Создаем HTTP запрос с контекстом
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Выполняем запрос
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус код
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Парсим JSON ответ
	var apiResponse entity.ExchangeRatesResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// Возвращаем только курсы валют
	return apiResponse.Rates, nil
}

// GetRate получает курс валюты из Redis
// Если курса нет или он устарел, возвращает ошибку
func (s *ExchangeRateService) GetRate(ctx context.Context, currency string) (*entity.ExchangeRate, error) {
	rate, err := s.rateRepo.Get(ctx, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate for %s: %w", currency, err)
	}

	// Проверяем возраст курса и выводим предупреждение если курс старый
	// (более 2 часов согласно TTL 30 минут + запас)
	age := time.Since(rate.UpdatedAt)
	if age > 2*time.Hour {
		log.Printf("WARNING: Using outdated exchange rate for %s (age: %v). API may be unavailable.", currency, age)
	}

	return rate, nil
}

// GetRates получает курсы нескольких валют из Redis
func (s *ExchangeRateService) GetRates(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error) {
	rates, err := s.rateRepo.GetMultiple(ctx, currencies)
	if err != nil {
		return nil, fmt.Errorf("failed to get rates: %w", err)
	}

	return rates, nil
}

// ConvertCurrency конвертирует сумму из одной валюты в другую
// Использует курсы из Redis
func (s *ExchangeRateService) ConvertCurrency(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, float64, error) {
	// Если валюты одинаковые, возвращаем исходную сумму
	if fromCurrency == toCurrency {
		return amount, 1.0, nil
	}

	// Получаем курсы обеих валют
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

	// Конвертация: amount * (toRate / fromRate)
	// Например: 100 RUB в USD = 100 * (1/91.23) / (1/1) = 100 / 91.23 = 1.096 USD
	exchangeRate := toRate.Rate / fromRate.Rate
	convertedAmount := amount * exchangeRate

	return convertedAmount, exchangeRate, nil
}

// EnsureRatesAvailable проверяет наличие курсов в Redis
// Если курсов нет, запрашивает их из API
func (s *ExchangeRateService) EnsureRatesAvailable(ctx context.Context) error {
	// Проверяем наличие основных валют
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
