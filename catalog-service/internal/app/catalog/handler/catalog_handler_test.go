package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"
	"augustberries/catalog-service/internal/app/catalog/repository/mocks"
	"augustberries/catalog-service/internal/app/catalog/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Хелперы для создания тестового окружения

func setupTestHandler() (*CatalogHandler, *mocks.MockCategoryRepository, *mocks.MockProductRepository, *mocks.MockRedisCache, *mocks.MockMessagePublisher) {
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	catalogService := service.NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)
	handler := NewCatalogHandler(catalogService)

	return handler, categoryRepo, productRepo, redisCache, kafkaProducer
}

func newTestCategory() *entity.Category {
	return &entity.Category{
		ID:        uuid.New(),
		Name:      "Electronics",
		CreatedAt: time.Now(),
	}
}

func newTestProduct(categoryID uuid.UUID) *entity.Product {
	return &entity.Product{
		ID:          uuid.New(),
		Name:        "Laptop",
		Description: "High-performance laptop",
		Price:       1299.99,
		CategoryID:  categoryID,
		CreatedAt:   time.Now(),
	}
}

func newTestProductWithCategory() *entity.ProductWithCategory {
	category := newTestCategory()
	product := newTestProduct(category.ID)
	return &entity.ProductWithCategory{
		Product:  *product,
		Category: *category,
	}
}

// ==================== Category Handler Tests ====================

func TestCatalogHandler_CreateCategory_Success(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, redisCache, _ := setupTestHandler()

	categoryRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Category")).Return(nil)
	redisCache.On("DeleteCategories", mock.Anything).Return(nil)

	reqBody := entity.CreateCategoryRequest{Name: "Electronics"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateCategory(c)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var response entity.Category
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Electronics", response.Name)
}

func TestCatalogHandler_CreateCategory_InvalidJSON(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := setupTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateCategory(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_CreateCategory_ValidationError(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := setupTestHandler()

	// Name слишком короткий (меньше 2 символов)
	reqBody := entity.CreateCategoryRequest{Name: "A"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateCategory(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_GetCategory_Success(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, _, _ := setupTestHandler()

	category := newTestCategory()
	categoryRepo.On("GetByID", mock.Anything, category.ID).Return(category, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories/"+category.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: category.ID.String()}}

	// Act
	handler.GetCategory(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.Category
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, category.ID, response.ID)
}

func TestCatalogHandler_GetCategory_InvalidID(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := setupTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories/invalid-uuid", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid-uuid"}}

	// Act
	handler.GetCategory(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_GetCategory_NotFound(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, _, _ := setupTestHandler()

	categoryID := uuid.New()
	categoryRepo.On("GetByID", mock.Anything, categoryID).Return(nil, service.ErrCategoryNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories/"+categoryID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: categoryID.String()}}

	// Act
	handler.GetCategory(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCatalogHandler_GetAllCategories_Success(t *testing.T) {
	// Arrange
	handler, _, _, redisCache, _ := setupTestHandler()

	categories := []entity.Category{
		{ID: uuid.New(), Name: "Electronics"},
		{ID: uuid.New(), Name: "Books"},
	}
	redisCache.On("GetCategories", mock.Anything).Return(categories, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories", nil)

	// Act
	handler.GetAllCategories(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.CategoryListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 2, response.Total)
	assert.Len(t, response.Categories, 2)
}

func TestCatalogHandler_UpdateCategory_Success(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, redisCache, _ := setupTestHandler()

	category := newTestCategory()
	categoryRepo.On("GetByID", mock.Anything, category.ID).Return(category, nil)
	categoryRepo.On("Update", mock.Anything, category).Return(nil)
	redisCache.On("DeleteCategories", mock.Anything).Return(nil)

	reqBody := entity.UpdateCategoryRequest{Name: "Updated Electronics"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/categories/"+category.ID.String(), bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: category.ID.String()}}

	// Act
	handler.UpdateCategory(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCatalogHandler_UpdateCategory_NotFound(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, _, _ := setupTestHandler()

	categoryID := uuid.New()
	categoryRepo.On("GetByID", mock.Anything, categoryID).Return(nil, service.ErrCategoryNotFound)

	reqBody := entity.UpdateCategoryRequest{Name: "Updated"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/categories/"+categoryID.String(), bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: categoryID.String()}}

	// Act
	handler.UpdateCategory(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCatalogHandler_DeleteCategory_Success(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, redisCache, _ := setupTestHandler()

	categoryID := uuid.New()
	categoryRepo.On("Delete", mock.Anything, categoryID).Return(nil)
	redisCache.On("DeleteCategories", mock.Anything).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/categories/"+categoryID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: categoryID.String()}}

	// Act
	handler.DeleteCategory(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCatalogHandler_DeleteCategory_NotFound(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, _, _ := setupTestHandler()

	categoryID := uuid.New()
	categoryRepo.On("Delete", mock.Anything, categoryID).Return(service.ErrCategoryNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/categories/"+categoryID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: categoryID.String()}}

	// Act
	handler.DeleteCategory(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ==================== Product Handler Tests ====================

func TestCatalogHandler_CreateProduct_Success(t *testing.T) {
	// Arrange
	handler, categoryRepo, productRepo, _, _ := setupTestHandler()

	category := newTestCategory()
	categoryRepo.On("GetByID", mock.Anything, category.ID).Return(category, nil)
	productRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Product")).Return(nil)

	reqBody := entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "High-performance laptop for developers",
		Price:       1299.99,
		CategoryID:  category.ID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateProduct(c)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var response entity.Product
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Laptop", response.Name)
	assert.Equal(t, 1299.99, response.Price)
}

func TestCatalogHandler_CreateProduct_ValidationError(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := setupTestHandler()

	// Цена должна быть > 0
	reqBody := entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "Description here",
		Price:       0,
		CategoryID:  uuid.New(),
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateProduct(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_CreateProduct_CategoryNotFound(t *testing.T) {
	// Arrange
	handler, categoryRepo, _, _, _ := setupTestHandler()

	categoryID := uuid.New()
	categoryRepo.On("GetByID", mock.Anything, categoryID).Return(nil, service.ErrCategoryNotFound)

	reqBody := entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "High-performance laptop for developers",
		Price:       1299.99,
		CategoryID:  categoryID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateProduct(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_GetProduct_Success(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	product := newTestProductWithCategory()
	productRepo.On("GetWithCategory", mock.Anything, product.ID).Return(product, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/products/"+product.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: product.ID.String()}}

	// Act
	handler.GetProduct(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.ProductWithCategory
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, product.ID, response.ID)
}

func TestCatalogHandler_GetProduct_NotFound(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	productID := uuid.New()
	productRepo.On("GetWithCategory", mock.Anything, productID).Return(nil, service.ErrProductNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/products/"+productID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: productID.String()}}

	// Act
	handler.GetProduct(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCatalogHandler_GetAllProducts_Success(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	products := []entity.ProductWithCategory{
		*newTestProductWithCategory(),
		*newTestProductWithCategory(),
	}
	productRepo.On("GetAllWithCategories", mock.Anything).Return(products, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/products", nil)

	// Act
	handler.GetAllProducts(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response entity.ProductListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 2, response.Total)
}

func TestCatalogHandler_UpdateProduct_Success(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	category := newTestCategory()
	product := newTestProduct(category.ID)

	productRepo.On("GetByID", mock.Anything, product.ID).Return(product, nil)
	productRepo.On("Update", mock.Anything, product).Return(nil)

	reqBody := entity.UpdateProductRequest{Name: "Updated Laptop"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/products/"+product.ID.String(), bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: product.ID.String()}}

	// Act
	handler.UpdateProduct(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCatalogHandler_UpdateProduct_NotFound(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	productID := uuid.New()
	productRepo.On("GetByID", mock.Anything, productID).Return(nil, service.ErrProductNotFound)

	reqBody := entity.UpdateProductRequest{Name: "Updated"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/products/"+productID.String(), bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: productID.String()}}

	// Act
	handler.UpdateProduct(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCatalogHandler_DeleteProduct_Success(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	product := newTestProduct(uuid.New())
	productRepo.On("GetByID", mock.Anything, product.ID).Return(product, nil)
	productRepo.On("Delete", mock.Anything, product.ID).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/products/"+product.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: product.ID.String()}}

	// Act
	handler.DeleteProduct(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCatalogHandler_DeleteProduct_NotFound(t *testing.T) {
	// Arrange
	handler, _, productRepo, _, _ := setupTestHandler()

	productID := uuid.New()
	productRepo.On("GetByID", mock.Anything, productID).Return(nil, service.ErrProductNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/products/"+productID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: productID.String()}}

	// Act
	handler.DeleteProduct(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
}
