package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
)

// CatalogClient клиент для взаимодействия с Catalog Service
// Используется для проверки цен товаров при создании заказа
type CatalogClient struct {
	baseURL    string
	httpClient *http.Client
	authToken  string // JWT токен для аутентификации в Catalog Service
}

// NewCatalogClient создает новый клиент для Catalog Service
func NewCatalogClient(baseURL string) *CatalogClient {
	return &CatalogClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Таймаут для HTTP запросов
		},
	}
}

// SetAuthToken устанавливает JWT токен для аутентификации
func (c *CatalogClient) SetAuthToken(token string) {
	c.authToken = token
}

// GetProduct получает информацию о товаре из Catalog Service
// Используется для проверки актуальности цены при создании заказа
func (c *CatalogClient) GetProduct(ctx context.Context, productID uuid.UUID) (*entity.ProductWithCategory, error) {
	url := fmt.Sprintf("%s/products/%s", c.baseURL, productID.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем JWT токен для аутентификации
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("product not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var product entity.ProductWithCategory
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &product, nil
}

// GetProducts получает информацию о нескольких товарах
// Можно использовать для оптимизации запросов при создании заказа с несколькими позициями
func (c *CatalogClient) GetProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]*entity.ProductWithCategory, error) {
	products := make(map[uuid.UUID]*entity.ProductWithCategory)

	// В реальном проекте здесь можно сделать batch запрос к Catalog Service
	// Но так как в API нет batch endpoint, делаем последовательные запросы
	for _, productID := range productIDs {
		product, err := c.GetProduct(ctx, productID)
		if err != nil {
			return nil, fmt.Errorf("failed to get product %s: %w", productID, err)
		}
		products[productID] = product
	}

	return products, nil
}
