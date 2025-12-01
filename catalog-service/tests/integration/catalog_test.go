//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"
	"augustberries/catalog-service/internal/app/catalog/handler"
	"augustberries/catalog-service/internal/app/catalog/repository"
	"augustberries/catalog-service/internal/app/catalog/service"
	"augustberries/catalog-service/internal/app/catalog/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// CatalogIntegrationTestSuite содержит интеграционные тесты для catalog-service
// Требует запущенные PostgreSQL и Redis
type CatalogIntegrationTestSuite struct {
	suite.Suite
	db          *gorm.DB
	redisClient *util.RedisClient
	router      *gin.Engine
}

// SetupSuite выполняется один раз перед всеми тестами
func (s *CatalogIntegrationTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Подключение к PostgreSQL (тестовая БД)
	dsn := "host=localhost port=5433 user=postgres password=postgres dbname=catalog_service_test sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(s.T(), err, "Failed to connect to PostgreSQL")
	s.db = db

	// Подключение к Redis
	s.redisClient = util.NewRedisClient("localhost:6380", "redis_password", 15)

	// Применяем миграции
	s.setupDatabase()

	// Инициализируем репозитории
	categoryRepo := repository.NewCategoryRepository(s.db)
	productRepo := repository.NewProductRepository(s.db)

	// Создаем mock Kafka producer для тестов (не отправляет реальные сообщения)
	kafkaProducer := &mockKafkaProducer{}

	// Инициализируем сервис
	catalogService := service.NewCatalogService(categoryRepo, productRepo, s.redisClient, kafkaProducer)

	// Инициализируем handler
	catalogHandler := handler.NewCatalogHandler(catalogService)

	// Настраиваем router
	s.router = gin.New()
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "catalog-service"})
	})

	// Categories routes
	categories := s.router.Group("/categories")
	{
		categories.POST("", catalogHandler.CreateCategory)
		categories.GET("", catalogHandler.GetAllCategories)
		categories.GET("/:id", catalogHandler.GetCategory)
		categories.PUT("/:id", catalogHandler.UpdateCategory)
		categories.DELETE("/:id", catalogHandler.DeleteCategory)
	}

	// Products routes
	products := s.router.Group("/products")
	{
		products.POST("", catalogHandler.CreateProduct)
		products.GET("", catalogHandler.GetAllProducts)
		products.GET("/:id", catalogHandler.GetProduct)
		products.PUT("/:id", catalogHandler.UpdateProduct)
		products.DELETE("/:id", catalogHandler.DeleteProduct)
	}
}

// TearDownSuite выполняется один раз после всех тестов
func (s *CatalogIntegrationTestSuite) TearDownSuite() {
	s.cleanupDatabase()
	if s.redisClient != nil {
		s.redisClient.Close()
	}
}

// SetupTest выполняется перед каждым тестом
func (s *CatalogIntegrationTestSuite) SetupTest() {
	// Очищаем данные перед каждым тестом
	s.db.Exec("DELETE FROM products")
	s.db.Exec("DELETE FROM categories")
	s.redisClient.DeleteCategories(context.Background())
}

func (s *CatalogIntegrationTestSuite) setupDatabase() {
	// Автоматическая миграция
	err := s.db.AutoMigrate(&entity.Category{}, &entity.Product{})
	require.NoError(s.T(), err)
}

func (s *CatalogIntegrationTestSuite) cleanupDatabase() {
	s.db.Exec("DROP TABLE IF EXISTS products")
	s.db.Exec("DROP TABLE IF EXISTS categories")
}

// mockKafkaProducer - мок для Kafka в интеграционных тестах
type mockKafkaProducer struct{}

func (m *mockKafkaProducer) PublishMessage(ctx context.Context, key string, value []byte) error {
	return nil
}

func (m *mockKafkaProducer) Close() error {
	return nil
}

// ==================== Category Tests ====================

func (s *CatalogIntegrationTestSuite) TestCreateCategory_Success() {
	// Arrange
	reqBody := entity.CreateCategoryRequest{Name: "Electronics"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusCreated, rec.Code)

	var response entity.Category
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Electronics", response.Name)
	assert.NotEqual(s.T(), uuid.Nil, response.ID)
}

func (s *CatalogIntegrationTestSuite) TestGetAllCategories_Success() {
	// Arrange - создаём категории
	categories := []string{"Electronics", "Books", "Clothing"}
	for _, name := range categories {
		s.db.Create(&entity.Category{ID: uuid.New(), Name: name, CreatedAt: time.Now()})
	}

	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.CategoryListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, response.Total)
}

func (s *CatalogIntegrationTestSuite) TestGetCategory_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	req := httptest.NewRequest(http.MethodGet, "/categories/"+category.ID.String(), nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.Category
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), category.ID, response.ID)
	assert.Equal(s.T(), "Electronics", response.Name)
}

func (s *CatalogIntegrationTestSuite) TestGetCategory_NotFound() {
	// Arrange
	nonExistentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/categories/"+nonExistentID.String(), nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusNotFound, rec.Code)
}

func (s *CatalogIntegrationTestSuite) TestUpdateCategory_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	reqBody := entity.UpdateCategoryRequest{Name: "Updated Electronics"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/categories/"+category.ID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.Category
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Electronics", response.Name)
}

func (s *CatalogIntegrationTestSuite) TestDeleteCategory_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "ToDelete", CreatedAt: time.Now()}
	s.db.Create(category)

	req := httptest.NewRequest(http.MethodDelete, "/categories/"+category.ID.String(), nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	// Проверяем что категория удалена
	var count int64
	s.db.Model(&entity.Category{}).Where("id = ?", category.ID).Count(&count)
	assert.Equal(s.T(), int64(0), count)
}

// ==================== Product Tests ====================

func (s *CatalogIntegrationTestSuite) TestCreateProduct_Success() {
	// Arrange - создаём категорию
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	reqBody := entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "High-performance laptop for developers",
		Price:       1299.99,
		CategoryID:  category.ID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusCreated, rec.Code)

	var response entity.Product
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Laptop", response.Name)
	assert.Equal(s.T(), 1299.99, response.Price)
	assert.Equal(s.T(), category.ID, response.CategoryID)
}

func (s *CatalogIntegrationTestSuite) TestCreateProduct_CategoryNotFound() {
	// Arrange
	reqBody := entity.CreateProductRequest{
		Name:        "Laptop",
		Description: "Description here for validation",
		Price:       999.99,
		CategoryID:  uuid.New(), // Несуществующая категория
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusBadRequest, rec.Code)
}

func (s *CatalogIntegrationTestSuite) TestGetAllProducts_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	products := []entity.Product{
		{ID: uuid.New(), Name: "Laptop", Description: "Desc1", Price: 1299.99, CategoryID: category.ID, CreatedAt: time.Now()},
		{ID: uuid.New(), Name: "Phone", Description: "Desc2", Price: 999.99, CategoryID: category.ID, CreatedAt: time.Now()},
	}
	for _, p := range products {
		s.db.Create(&p)
	}

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.ProductListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, response.Total)
}

func (s *CatalogIntegrationTestSuite) TestGetProduct_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	product := &entity.Product{
		ID:          uuid.New(),
		Name:        "Laptop",
		Description: "Description",
		Price:       1299.99,
		CategoryID:  category.ID,
		CreatedAt:   time.Now(),
	}
	s.db.Create(product)

	req := httptest.NewRequest(http.MethodGet, "/products/"+product.ID.String(), nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.ProductWithCategory
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), product.ID, response.ID)
	assert.Equal(s.T(), "Electronics", response.Category.Name)
}

func (s *CatalogIntegrationTestSuite) TestUpdateProduct_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	product := &entity.Product{
		ID:          uuid.New(),
		Name:        "Laptop",
		Description: "Description",
		Price:       1299.99,
		CategoryID:  category.ID,
		CreatedAt:   time.Now(),
	}
	s.db.Create(product)

	reqBody := entity.UpdateProductRequest{
		Name:  "Updated Laptop",
		Price: 1399.99,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/products/"+product.ID.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.Product
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Laptop", response.Name)
	assert.Equal(s.T(), 1399.99, response.Price)
}

func (s *CatalogIntegrationTestSuite) TestDeleteProduct_Success() {
	// Arrange
	category := &entity.Category{ID: uuid.New(), Name: "Electronics", CreatedAt: time.Now()}
	s.db.Create(category)

	product := &entity.Product{
		ID:          uuid.New(),
		Name:        "ToDelete",
		Description: "Description",
		Price:       99.99,
		CategoryID:  category.ID,
		CreatedAt:   time.Now(),
	}
	s.db.Create(product)

	req := httptest.NewRequest(http.MethodDelete, "/products/"+product.ID.String(), nil)
	rec := httptest.NewRecorder()

	// Act
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	// Проверяем что товар удалён
	var count int64
	s.db.Model(&entity.Product{}).Where("id = ?", product.ID).Count(&count)
	assert.Equal(s.T(), int64(0), count)
}

func (s *CatalogIntegrationTestSuite) TestHealthCheck() {
	// Act
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)
}

// Запуск test suite
func TestCatalogIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogIntegrationTestSuite))
}
