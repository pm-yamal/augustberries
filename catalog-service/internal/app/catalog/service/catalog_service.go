package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"
	"augustberries/catalog-service/internal/app/catalog/repository"
	"augustberries/catalog-service/internal/app/catalog/util"

	"github.com/google/uuid"
)

var (
	// Ошибки бизнес-логики для обработки в handlers
	ErrCategoryNotFound = errors.New("category not found")
	ErrProductNotFound  = errors.New("product not found")
)

// CatalogService обрабатывает бизнес-логику каталога товаров
// Координирует работу репозиториев, Redis кеша и Kafka producer
type CatalogService struct {
	categoryRepo  repository.CategoryRepository // Репозиторий для работы с категориями в PostgreSQL
	productRepo   repository.ProductRepository  // Репозиторий для работы с товарами в PostgreSQL
	redisClient   *util.RedisClient             // Клиент для кеширования категорий
	kafkaProducer *util.KafkaProducer           // Producer для отправки событий о товарах
}

// NewCatalogService создает новый сервис каталога с внедрением зависимостей
func NewCatalogService(
	categoryRepo repository.CategoryRepository,
	productRepo repository.ProductRepository,
	redisClient *util.RedisClient,
	kafkaProducer *util.KafkaProducer,
) *CatalogService {
	return &CatalogService{
		categoryRepo:  categoryRepo,
		productRepo:   productRepo,
		redisClient:   redisClient,
		kafkaProducer: kafkaProducer,
	}
}

// === CATEGORIES ===

// CreateCategory создает новую категорию и инвалидирует кеш
// После создания категории нужно обновить кеш для актуальности данных
func (s *CatalogService) CreateCategory(ctx context.Context, req *entity.CreateCategoryRequest) (*entity.Category, error) {
	// Создаем новую категорию с уникальным ID
	category := &entity.Category{
		ID:        uuid.New(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	// Сохраняем в PostgreSQL
	if err := s.categoryRepo.Create(ctx, category); err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	// Инвалидируем кеш категорий чтобы при следующем запросе загрузить свежие данные
	if err := s.redisClient.DeleteCategories(ctx); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		// Категория уже создана, проблемы с кешем не критичны
		fmt.Printf("failed to invalidate categories cache: %v\n", err)
	}

	return category, nil
}

// GetCategory получает категорию по ID из PostgreSQL
// Не использует кеш, так как запрашивается конкретная категория
func (s *CatalogService) GetCategory(ctx context.Context, id uuid.UUID) (*entity.Category, error) {
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	return category, nil
}

// GetAllCategories получает все категории с кешированием в Redis
// Сначала проверяет кеш, если нет - загружает из БД и кеширует
func (s *CatalogService) GetAllCategories(ctx context.Context) ([]entity.Category, error) {
	// Пытаемся получить из кеша Redis
	categories, err := s.redisClient.GetCategories(ctx)
	if err == nil && len(categories) > 0 {
		// Cache hit - возвращаем данные из кеша
		return categories, nil
	}

	// Cache miss - загружаем из PostgreSQL
	categories, err = s.categoryRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	// Сохраняем в кеш на 1 час для последующих запросов
	if err := s.redisClient.SetCategories(ctx, categories, time.Hour); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		// Данные получены из БД, проблемы с кешем не критичны
		fmt.Printf("failed to cache categories: %v\n", err)
	}

	return categories, nil
}

// UpdateCategory обновляет категорию и инвалидирует кеш
// Проверяет существование категории перед обновлением
func (s *CatalogService) UpdateCategory(ctx context.Context, id uuid.UUID, req *entity.UpdateCategoryRequest) (*entity.Category, error) {
	// Проверяем существование категории
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	// Обновляем поля категории
	category.Name = req.Name

	// Сохраняем изменения в PostgreSQL
	if err := s.categoryRepo.Update(ctx, category); err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	// Инвалидируем кеш для обновления данных
	if err := s.redisClient.DeleteCategories(ctx); err != nil {
		fmt.Printf("failed to invalidate categories cache: %v\n", err)
	}

	return category, nil
}

// DeleteCategory удаляет категорию и инвалидирует кеш
func (s *CatalogService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	// Удаляем категорию из PostgreSQL
	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return ErrCategoryNotFound
		}
		return fmt.Errorf("failed to delete category: %w", err)
	}

	// Инвалидируем кеш
	if err := s.redisClient.DeleteCategories(ctx); err != nil {
		fmt.Printf("failed to invalidate categories cache: %v\n", err)
	}

	return nil
}

// === PRODUCTS ===

// CreateProduct создает новый товар
// Проверяет существование категории перед созданием
func (s *CatalogService) CreateProduct(ctx context.Context, req *entity.CreateProductRequest) (*entity.Product, error) {
	// Проверяем существование категории
	if _, err := s.categoryRepo.GetByID(ctx, req.CategoryID); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to verify category: %w", err)
	}

	// Создаем новый товар с уникальным ID
	product := &entity.Product{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price, // Цена в базовой валюте (USD)
		CategoryID:  req.CategoryID,
		CreatedAt:   time.Now(),
	}

	// Сохраняем в PostgreSQL
	// При создании товара событие не отправляется
	if err := s.productRepo.Create(ctx, product); err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return product, nil
}

// GetProduct получает товар по ID с информацией о категории
// Использует JOIN для получения связанной категории за один запрос
func (s *CatalogService) GetProduct(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error) {
	product, err := s.productRepo.GetWithCategory(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return product, nil
}

// GetAllProducts получает все товары с информацией о категориях
// Использует JOIN для эффективной загрузки связанных данных
func (s *CatalogService) GetAllProducts(ctx context.Context) ([]entity.ProductWithCategory, error) {
	products, err := s.productRepo.GetAllWithCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}

	return products, nil
}

// UpdateProduct обновляет товар и отправляет событие PRODUCT_UPDATED при изменении цены
// Это ключевая функция - при смене цены отправляется событие в Kafka
func (s *CatalogService) UpdateProduct(ctx context.Context, id uuid.UUID, req *entity.UpdateProductRequest) (*entity.Product, error) {
	// Получаем текущий товар из БД
	product, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	// Запоминаем старую цену для проверки изменений
	oldPrice := product.Price

	// Обновляем только переданные поля (частичное обновление)
	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}
	if req.Price > 0 {
		product.Price = req.Price
	}
	if req.CategoryID != uuid.Nil {
		// Проверяем существование новой категории
		if _, err := s.categoryRepo.GetByID(ctx, req.CategoryID); err != nil {
			if errors.Is(err, repository.ErrCategoryNotFound) {
				return nil, ErrCategoryNotFound
			}
			return nil, fmt.Errorf("failed to verify category: %w", err)
		}
		product.CategoryID = req.CategoryID
	}

	// Сохраняем изменения в PostgreSQL
	if err := s.productRepo.Update(ctx, product); err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	// ВАЖНО: Если цена изменилась, отправляем событие PRODUCT_UPDATED в Kafka
	// Событие отправляется только при смене цены
	if product.Price != oldPrice {
		event := entity.ProductEvent{
			EventType:  "PRODUCT_UPDATED",
			ProductID:  product.ID,
			Name:       product.Name,
			Price:      product.Price,
			CategoryID: product.CategoryID,
			Timestamp:  time.Now(),
		}
		if err := s.publishProductEvent(ctx, event); err != nil {
			// Логируем ошибку, но не прерываем выполнение
			// Товар уже обновлен, проблемы с Kafka не критичны для основной операции
			fmt.Printf("failed to publish product updated event: %v\n", err)
		}
	}

	return product, nil
}

// DeleteProduct удаляет товар
// Проверяет существование товара перед удалением
func (s *CatalogService) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	// Проверяем существование товара
	_, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return ErrProductNotFound
		}
		return fmt.Errorf("failed to get product: %w", err)
	}

	// Удаляем товар из PostgreSQL
	if err := s.productRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}

// publishProductEvent отправляет событие о товаре в Kafka
// Используется для уведомления других сервисов об изменении цены
// Key - это ProductID для правильного партиционирования в Kafka
func (s *CatalogService) publishProductEvent(ctx context.Context, event entity.ProductEvent) error {
	// Сериализуем событие в JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal product event: %w", err)
	}

	// Отправляем в Kafka с ключом = ProductID для партиционирования
	if err := s.kafkaProducer.PublishMessage(ctx, event.ProductID.String(), eventData); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	return nil
}
