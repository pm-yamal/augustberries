package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/stretchr/testify/assert"
)

// ===================== ExchangeRateAPIClient Tests =====================

func TestFetchRates_Success(t *testing.T) {
	// Arrange
	expectedRates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.93,
		"RUB": 91.23,
		"GBP": 0.79,
	}

	apiResponse := entity.ExchangeRatesResponse{
		Base:  "USD",
		Date:  "2024-01-15",
		Rates: expectedRates,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse)
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedRates, rates)
	assert.Equal(t, 1.0, rates["USD"])
	assert.Equal(t, 0.93, rates["EUR"])
	assert.Equal(t, 91.23, rates["RUB"])
}

func TestFetchRates_HTTPError_500(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "API returned status 500")
}

func TestFetchRates_HTTPError_404(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "API returned status 404")
}

func TestFetchRates_HTTPError_401(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("API key invalid"))
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "API returned status 401")
}

func TestFetchRates_InvalidJSON(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json {{{"))
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestFetchRates_EmptyResponse(t *testing.T) {
	// Arrange
	apiResponse := entity.ExchangeRatesResponse{
		Base:  "USD",
		Date:  "2024-01-15",
		Rates: map[string]float64{}, // Пустой map
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse)
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, rates)
	assert.Empty(t, rates)
}

func TestFetchRates_ContextCanceled(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Долгий ответ
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменяем сразу

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
}

func TestFetchRates_Timeout(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // Долгий ответ
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Таймаут 1 секунда
	client := NewExchangeRateAPIClient(server.URL, 1)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestFetchRates_ConnectionRefused(t *testing.T) {
	// Arrange
	// Используем несуществующий адрес
	client := NewExchangeRateAPIClient("http://localhost:59999/rates", 1)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestFetchRates_PartialRates(t *testing.T) {
	// API возвращает только часть валют
	// Arrange
	expectedRates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.93,
		// RUB отсутствует
	}

	apiResponse := entity.ExchangeRatesResponse{
		Base:  "USD",
		Date:  "2024-01-15",
		Rates: expectedRates,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse)
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rates))
	assert.Equal(t, 1.0, rates["USD"])
	assert.Equal(t, 0.93, rates["EUR"])
	_, hasRUB := rates["RUB"]
	assert.False(t, hasRUB)
}

func TestFetchRates_LargeRates(t *testing.T) {
	// Большие значения курсов (например, для JPY или криптовалют)
	// Arrange
	expectedRates := map[string]float64{
		"USD": 1.0,
		"JPY": 149.50,
		"KRW": 1320.45,
		"VND": 24315.0,
	}

	apiResponse := entity.ExchangeRatesResponse{
		Base:  "USD",
		Date:  "2024-01-15",
		Rates: expectedRates,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse)
	}))
	defer server.Close()

	client := NewExchangeRateAPIClient(server.URL, 10)
	ctx := context.Background()

	// Act
	rates, err := client.FetchRates(ctx)

	// Assert
	assert.NoError(t, err)
	assert.InDelta(t, 149.50, rates["JPY"], 0.01)
	assert.InDelta(t, 1320.45, rates["KRW"], 0.01)
	assert.InDelta(t, 24315.0, rates["VND"], 0.01)
}

func TestNewExchangeRateAPIClient(t *testing.T) {
	// Проверяем создание клиента
	// Arrange & Act
	client := NewExchangeRateAPIClient("https://api.example.com/rates", 30)

	// Assert
	assert.NotNil(t, client)
	assert.Equal(t, "https://api.example.com/rates", client.apiURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}
