package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
)

// ExchangeRateAPIClientImpl реализует интерфейс ExchangeRateAPIClient
// Отвечает только за HTTP запросы к внешнему API
type ExchangeRateAPIClientImpl struct {
	apiURL     string
	httpClient *http.Client
}

// NewExchangeRateAPIClient создает новый HTTP клиент для API курсов валют
func NewExchangeRateAPIClient(apiURL string, timeoutSec int) *ExchangeRateAPIClientImpl {
	return &ExchangeRateAPIClientImpl{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// FetchRates получает курсы валют из внешнего API
func (c *ExchangeRateAPIClientImpl) FetchRates(ctx context.Context) (map[string]float64, error) {
	// Создаем HTTP запрос с контекстом
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Выполняем запрос
	resp, err := c.httpClient.Do(req)
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
