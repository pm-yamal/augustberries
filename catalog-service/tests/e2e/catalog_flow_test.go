//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// BaseURL - адрес запущенного catalog-service
	// Для E2E тестов сервис должен быть запущен через docker-compose
	BaseURL = "http://localhost:8081"
)

// TestFullCatalogFlow тестирует полный цикл работы с каталогом:
// 1. Создание категории
// 2. Получение всех категорий (проверка кеша)
// 3. Создание товара в категории
// 4. Получение товара
// 5. Обновление товара (смена цены - событие в Kafka)
// 6. Удаление товара
// 7. Удаление категории
func TestFullCatalogFlow(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// ==================== Step 1: Create Category ====================
	t.Log("Step 1: Creating category")

	categoryName := fmt.Sprintf("Test Category %d", time.Now().UnixNano())
	createCategoryReq := entity.CreateCategoryRequest{Name: categoryName}
	categoryBody, _ := json.Marshal(createCategoryReq)

	resp, err := client.Post(
		BaseURL+"/categories",
		"application/json",
		bytes.NewBuffer(categoryBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Category creation should succeed")

	var category entity.Category
	err = json.NewDecoder(resp.Body).Decode(&category)
	require.NoError(t, err)
	assert.Equal(t, categoryName, category.Name)
	assert.NotEqual(t, uuid.Nil, category.ID)

	categoryID := category.ID
	t.Logf("Created category: %s (ID: %s)", category.Name, categoryID)

	// ==================== Step 2: Get All Categories ====================
	t.Log("Step 2: Getting all categories")

	resp, err = client.Get(BaseURL + "/categories")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var categoriesResponse entity.CategoryListResponse
	err = json.NewDecoder(resp.Body).Decode(&categoriesResponse)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, categoriesResponse.Total, 1)

	t.Logf("Total categories: %d", categoriesResponse.Total)

	// ==================== Step 3: Create Product ====================
	t.Log("Step 3: Creating product")

	productName := fmt.Sprintf("Test Product %d", time.Now().UnixNano())
	createProductReq := entity.CreateProductRequest{
		Name:        productName,
		Description: "This is a test product created by E2E tests",
		Price:       99.99,
		CategoryID:  categoryID,
	}
	productBody, _ := json.Marshal(createProductReq)

	resp, err = client.Post(
		BaseURL+"/products",
		"application/json",
		bytes.NewBuffer(productBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Product creation should succeed")

	var product entity.Product
	err = json.NewDecoder(resp.Body).Decode(&product)
	require.NoError(t, err)
	assert.Equal(t, productName, product.Name)
	assert.Equal(t, 99.99, product.Price)
	assert.Equal(t, categoryID, product.CategoryID)

	productID := product.ID
	t.Logf("Created product: %s (ID: %s, Price: %.2f)", product.Name, productID, product.Price)

	// ==================== Step 4: Get Product with Category ====================
	t.Log("Step 4: Getting product with category info")

	resp, err = client.Get(BaseURL + "/products/" + productID.String())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var productWithCategory entity.ProductWithCategory
	err = json.NewDecoder(resp.Body).Decode(&productWithCategory)
	require.NoError(t, err)
	assert.Equal(t, productID, productWithCategory.ID)
	assert.Equal(t, categoryName, productWithCategory.Category.Name)

	t.Logf("Product category: %s", productWithCategory.Category.Name)

	// ==================== Step 5: Update Product Price ====================
	t.Log("Step 5: Updating product price (triggers Kafka event)")

	newPrice := 149.99
	updateProductReq := entity.UpdateProductRequest{
		Price: newPrice,
	}
	updateBody, _ := json.Marshal(updateProductReq)

	req, _ := http.NewRequest(http.MethodPut, BaseURL+"/products/"+productID.String(), bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Product update should succeed")

	var updatedProduct entity.Product
	err = json.NewDecoder(resp.Body).Decode(&updatedProduct)
	require.NoError(t, err)
	assert.Equal(t, newPrice, updatedProduct.Price)

	t.Logf("Updated product price: %.2f -> %.2f", 99.99, newPrice)

	// ==================== Step 6: Delete Product ====================
	t.Log("Step 6: Deleting product")

	req, _ = http.NewRequest(http.MethodDelete, BaseURL+"/products/"+productID.String(), nil)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Product deletion should succeed")

	// Verify product is deleted
	resp, err = client.Get(BaseURL + "/products/" + productID.String())
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Product should not be found after deletion")

	t.Log("Product deleted successfully")

	// ==================== Step 7: Delete Category ====================
	t.Log("Step 7: Deleting category")

	req, _ = http.NewRequest(http.MethodDelete, BaseURL+"/categories/"+categoryID.String(), nil)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Category deletion should succeed")

	// Verify category is deleted
	resp, err = client.Get(BaseURL + "/categories/" + categoryID.String())
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Category should not be found after deletion")

	t.Log("Category deleted successfully")
	t.Log("Full catalog flow completed successfully!")
}

// TestCategoryValidation тестирует валидацию при создании категории
func TestCategoryValidation(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name           string
		request        entity.CreateCategoryRequest
		expectedStatus int
	}{
		{
			name:           "Empty name",
			request:        entity.CreateCategoryRequest{Name: ""},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Name too short",
			request:        entity.CreateCategoryRequest{Name: "A"},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)

			resp, err := client.Post(
				BaseURL+"/categories",
				"application/json",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// TestProductValidation тестирует валидацию при создании товара
func TestProductValidation(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name           string
		request        entity.CreateProductRequest
		expectedStatus int
	}{
		{
			name: "Empty name",
			request: entity.CreateProductRequest{
				Name:        "",
				Description: "Valid description here",
				Price:       99.99,
				CategoryID:  uuid.New(),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Zero price",
			request: entity.CreateProductRequest{
				Name:        "Valid Name",
				Description: "Valid description here",
				Price:       0,
				CategoryID:  uuid.New(),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Negative price",
			request: entity.CreateProductRequest{
				Name:        "Valid Name",
				Description: "Valid description here",
				Price:       -10.0,
				CategoryID:  uuid.New(),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Non-existent category",
			request: entity.CreateProductRequest{
				Name:        "Valid Name",
				Description: "Valid description here",
				Price:       99.99,
				CategoryID:  uuid.New(),
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)

			resp, err := client.Post(
				BaseURL+"/products",
				"application/json",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// TestInvalidUUID тестирует обработку невалидных UUID
func TestInvalidUUID(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/categories/invalid-uuid"},
		{http.MethodPut, "/categories/invalid-uuid"},
		{http.MethodDelete, "/categories/invalid-uuid"},
		{http.MethodGet, "/products/invalid-uuid"},
		{http.MethodPut, "/products/invalid-uuid"},
		{http.MethodDelete, "/products/invalid-uuid"},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			var req *http.Request
			if endpoint.method == http.MethodPut {
				body := []byte(`{"name": "test"}`)
				req, _ = http.NewRequest(endpoint.method, BaseURL+endpoint.path, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(endpoint.method, BaseURL+endpoint.path, nil)
			}

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 for invalid UUID")
		})
	}
}

// TestCategoryCaching проверяет работу кеширования категорий
func TestCategoryCaching(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Создаём категорию
	categoryName := fmt.Sprintf("Cache Test %d", time.Now().UnixNano())
	createReq := entity.CreateCategoryRequest{Name: categoryName}
	body, _ := json.Marshal(createReq)

	resp, err := client.Post(BaseURL+"/categories", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)

	var createdCategory entity.Category
	err = json.NewDecoder(resp.Body).Decode(&createdCategory)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	categoryID := createdCategory.ID

	// Cleanup: удаляем категорию после теста
	defer func() {
		req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/categories/"+categoryID.String(), nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	// Первый запрос - заполняет кеш
	resp, err = client.Get(BaseURL + "/categories")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Второй запрос - должен вернуться из кеша (но мы не можем это проверить напрямую)
	resp, err = client.Get(BaseURL + "/categories")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var categoriesResponse entity.CategoryListResponse
	err = json.NewDecoder(resp.Body).Decode(&categoriesResponse)
	require.NoError(t, err)

	// Проверяем что наша категория есть в списке
	found := false
	for _, cat := range categoriesResponse.Categories {
		if cat.Name == categoryName {
			found = true
			break
		}
	}
	assert.True(t, found, "Created category should be in the list")
}

// TestHealthCheck проверяет что сервис отвечает
func TestHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(BaseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
