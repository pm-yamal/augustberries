package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"
	"augustberries/catalog-service/internal/app/catalog/repository"
	"augustberries/catalog-service/internal/app/catalog/repository/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Хелперы для создания тестовых данных

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
		Description: "High-performance laptop for developers",
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

// ==================== Category Tests ====================

func TestCatalogService_CreateCategory_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryRepo.On("Create", ctx, mock.AnythingOfType("*entity.Category")).Return(nil)
	redisCache.On("DeleteCategories", ctx).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.CreateCategoryRequest{
		Name: "Electronics",
	}

	// Act
	category, err := service.CreateCategory(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, category)
	assert.Equal(t, "Electronics", category.Name)
	assert.NotEqual(t, uuid.Nil, category.ID)

	categoryRepo.AssertExpectations(t)
	redisCache.AssertExpectations(t)
}

func TestCatalogService_CreateCategory_RepoError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryRepo.On("Create", ctx, mock.AnythingOfType("*entity.Category")).Return(errors.New("db error"))

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.CreateCategoryRequest{Name: "Electronics"}

	// Act
	category, err := service.CreateCategory(ctx, req)

	// Assert
	assert.Nil(t, category)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create category")
}

func TestCatalogService_CreateCategory_CacheErrorIgnored(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryRepo.On("Create", ctx, mock.AnythingOfType("*entity.Category")).Return(nil)
	redisCache.On("DeleteCategories", ctx).Return(errors.New("redis error"))

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.CreateCategoryRequest{Name: "Electronics"}

	// Act
	category, err := service.CreateCategory(ctx, req)

	// Assert - ошибка кеша не должна прерывать выполнение
	require.NoError(t, err)
	assert.NotNil(t, category)
}

func TestCatalogService_GetCategory_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	expectedCategory := newTestCategory()
	categoryRepo.On("GetByID", ctx, expectedCategory.ID).Return(expectedCategory, nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	category, err := service.GetCategory(ctx, expectedCategory.ID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedCategory.ID, category.ID)
	assert.Equal(t, expectedCategory.Name, category.Name)
}

func TestCatalogService_GetCategory_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryID := uuid.New()
	categoryRepo.On("GetByID", ctx, categoryID).Return(nil, repository.ErrCategoryNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	category, err := service.GetCategory(ctx, categoryID)

	// Assert
	assert.Nil(t, category)
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestCatalogService_GetAllCategories_CacheHit(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	cachedCategories := []entity.Category{
		{ID: uuid.New(), Name: "Electronics"},
		{ID: uuid.New(), Name: "Books"},
	}
	redisCache.On("GetCategories", ctx).Return(cachedCategories, nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	categories, err := service.GetAllCategories(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, categories, 2)
	// Репозиторий НЕ должен вызываться при cache hit
	categoryRepo.AssertNotCalled(t, "GetAll")
}

func TestCatalogService_GetAllCategories_CacheMiss(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	dbCategories := []entity.Category{
		{ID: uuid.New(), Name: "Electronics"},
		{ID: uuid.New(), Name: "Books"},
	}
	redisCache.On("GetCategories", ctx).Return(nil, errors.New("cache miss"))
	categoryRepo.On("GetAll", ctx).Return(dbCategories, nil)
	redisCache.On("SetCategories", ctx, dbCategories, time.Hour).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	categories, err := service.GetAllCategories(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, categories, 2)
	categoryRepo.AssertCalled(t, "GetAll", ctx)
	redisCache.AssertCalled(t, "SetCategories", ctx, dbCategories, time.Hour)
}

func TestCatalogService_UpdateCategory_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	existingCategory := newTestCategory()
	categoryRepo.On("GetByID", ctx, existingCategory.ID).Return(existingCategory, nil)
	categoryRepo.On("Update", ctx, existingCategory).Return(nil)
	redisCache.On("DeleteCategories", ctx).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.UpdateCategoryRequest{Name: "Updated Electronics"}

	// Act
	category, err := service.UpdateCategory(ctx, existingCategory.ID, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Updated Electronics", category.Name)
}

func TestCatalogService_UpdateCategory_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryID := uuid.New()
	categoryRepo.On("GetByID", ctx, categoryID).Return(nil, repository.ErrCategoryNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.UpdateCategoryRequest{Name: "Updated"}

	// Act
	category, err := service.UpdateCategory(ctx, categoryID, req)

	// Assert
	assert.Nil(t, category)
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestCatalogService_DeleteCategory_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryID := uuid.New()
	categoryRepo.On("Delete", ctx, categoryID).Return(nil)
	redisCache.On("DeleteCategories", ctx).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	err := service.DeleteCategory(ctx, categoryID)

	// Assert
	require.NoError(t, err)
	categoryRepo.AssertExpectations(t)
	redisCache.AssertExpectations(t)
}

func TestCatalogService_DeleteCategory_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryID := uuid.New()
	categoryRepo.On("Delete", ctx, categoryID).Return(repository.ErrCategoryNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	err := service.DeleteCategory(ctx, categoryID)

	// Assert
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

// ==================== Product Tests ====================

func TestCatalogService_CreateProduct_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	category := newTestCategory()
	categoryRepo.On("GetByID", ctx, category.ID).Return(category, nil)
	productRepo.On("Create", ctx, mock.AnythingOfType("*entity.Product")).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "High-performance laptop for developers",
		Price:       1299.99,
		CategoryID:  category.ID,
	}

	// Act
	product, err := service.CreateProduct(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, product)
	assert.Equal(t, "Laptop", product.Name)
	assert.Equal(t, 1299.99, product.Price)
	assert.Equal(t, category.ID, product.CategoryID)
}

func TestCatalogService_CreateProduct_CategoryNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	categoryID := uuid.New()
	categoryRepo.On("GetByID", ctx, categoryID).Return(nil, repository.ErrCategoryNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "Description here",
		Price:       999.99,
		CategoryID:  categoryID,
	}

	// Act
	product, err := service.CreateProduct(ctx, req)

	// Assert
	assert.Nil(t, product)
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestCatalogService_GetProduct_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	expectedProduct := newTestProductWithCategory()
	productRepo.On("GetWithCategory", ctx, expectedProduct.ID).Return(expectedProduct, nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	product, err := service.GetProduct(ctx, expectedProduct.ID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedProduct.ID, product.ID)
	assert.Equal(t, expectedProduct.Name, product.Name)
	assert.NotEmpty(t, product.Category.Name)
}

func TestCatalogService_GetProduct_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	productID := uuid.New()
	productRepo.On("GetWithCategory", ctx, productID).Return(nil, repository.ErrProductNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	product, err := service.GetProduct(ctx, productID)

	// Assert
	assert.Nil(t, product)
	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestCatalogService_GetAllProducts_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	products := []entity.ProductWithCategory{
		*newTestProductWithCategory(),
		*newTestProductWithCategory(),
	}
	productRepo.On("GetAllWithCategories", ctx).Return(products, nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	result, err := service.GetAllProducts(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestCatalogService_UpdateProduct_Success_NoPriceChange(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	category := newTestCategory()
	existingProduct := newTestProduct(category.ID)

	productRepo.On("GetByID", ctx, existingProduct.ID).Return(existingProduct, nil)
	productRepo.On("Update", ctx, existingProduct).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.UpdateProductRequest{
		Name: "Updated Laptop",
	}

	// Act
	product, err := service.UpdateProduct(ctx, existingProduct.ID, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Updated Laptop", product.Name)
	// Kafka НЕ должен вызываться, т.к. цена не изменилась
	kafkaProducer.AssertNotCalled(t, "PublishMessage")
}

func TestCatalogService_UpdateProduct_Success_PriceChanged(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	category := newTestCategory()
	existingProduct := newTestProduct(category.ID)
	oldPrice := existingProduct.Price

	productRepo.On("GetByID", ctx, existingProduct.ID).Return(existingProduct, nil)
	productRepo.On("Update", ctx, existingProduct).Return(nil)
	kafkaProducer.On("PublishMessage", ctx, existingProduct.ID.String(), mock.AnythingOfType("[]uint8")).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	newPrice := oldPrice + 100.0
	req := &entity.UpdateProductRequest{
		Price: newPrice,
	}

	// Act
	product, err := service.UpdateProduct(ctx, existingProduct.ID, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, newPrice, product.Price)
	// Kafka ДОЛЖЕН вызываться, т.к. цена изменилась
	kafkaProducer.AssertCalled(t, "PublishMessage", ctx, existingProduct.ID.String(), mock.AnythingOfType("[]uint8"))
}

func TestCatalogService_UpdateProduct_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	productID := uuid.New()
	productRepo.On("GetByID", ctx, productID).Return(nil, repository.ErrProductNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.UpdateProductRequest{Name: "Updated"}

	// Act
	product, err := service.UpdateProduct(ctx, productID, req)

	// Assert
	assert.Nil(t, product)
	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestCatalogService_UpdateProduct_CategoryNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	existingProduct := newTestProduct(uuid.New())
	newCategoryID := uuid.New()

	productRepo.On("GetByID", ctx, existingProduct.ID).Return(existingProduct, nil)
	categoryRepo.On("GetByID", ctx, newCategoryID).Return(nil, repository.ErrCategoryNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.UpdateProductRequest{
		CategoryID: newCategoryID,
	}

	// Act
	product, err := service.UpdateProduct(ctx, existingProduct.ID, req)

	// Assert
	assert.Nil(t, product)
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestCatalogService_DeleteProduct_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	existingProduct := newTestProduct(uuid.New())

	productRepo.On("GetByID", ctx, existingProduct.ID).Return(existingProduct, nil)
	productRepo.On("Delete", ctx, existingProduct.ID).Return(nil)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	err := service.DeleteProduct(ctx, existingProduct.ID)

	// Assert
	require.NoError(t, err)
	productRepo.AssertExpectations(t)
}

func TestCatalogService_DeleteProduct_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	productID := uuid.New()
	productRepo.On("GetByID", ctx, productID).Return(nil, repository.ErrProductNotFound)

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	// Act
	err := service.DeleteProduct(ctx, productID)

	// Assert
	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestCatalogService_UpdateProduct_KafkaErrorIgnored(t *testing.T) {
	// Arrange
	ctx := context.Background()
	categoryRepo := new(mocks.MockCategoryRepository)
	productRepo := new(mocks.MockProductRepository)
	redisCache := new(mocks.MockRedisCache)
	kafkaProducer := new(mocks.MockMessagePublisher)

	existingProduct := newTestProduct(uuid.New())
	oldPrice := existingProduct.Price

	productRepo.On("GetByID", ctx, existingProduct.ID).Return(existingProduct, nil)
	productRepo.On("Update", ctx, existingProduct).Return(nil)
	kafkaProducer.On("PublishMessage", ctx, existingProduct.ID.String(), mock.AnythingOfType("[]uint8")).Return(errors.New("kafka error"))

	service := NewCatalogService(categoryRepo, productRepo, redisCache, kafkaProducer)

	req := &entity.UpdateProductRequest{
		Price: oldPrice + 50.0,
	}

	// Act
	product, err := service.UpdateProduct(ctx, existingProduct.ID, req)

	// Assert - ошибка Kafka не должна прерывать выполнение
	require.NoError(t, err)
	assert.NotNil(t, product)
}
