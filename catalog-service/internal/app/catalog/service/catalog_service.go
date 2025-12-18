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
	"augustberries/pkg/metrics"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrProductNotFound  = errors.New("product not found")
	ErrValidation       = errors.New("validation error")
)

type CatalogService struct {
	categoryRepo  repository.CategoryRepository
	productRepo   repository.ProductRepository
	redisClient   util.RedisCache
	kafkaProducer util.MessagePublisher
	validator     *validator.Validate
}

func NewCatalogService(
	categoryRepo repository.CategoryRepository,
	productRepo repository.ProductRepository,
	redisClient util.RedisCache,
	kafkaProducer util.MessagePublisher,
) *CatalogService {
	return &CatalogService{
		categoryRepo:  categoryRepo,
		productRepo:   productRepo,
		redisClient:   redisClient,
		kafkaProducer: kafkaProducer,
		validator:     validator.New(),
	}
}

func (s *CatalogService) CreateCategory(ctx context.Context, req *entity.CreateCategoryRequest) (*entity.Category, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, formatValidationError(err))
	}

	category := &entity.Category{
		ID:        uuid.New(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := s.categoryRepo.Create(ctx, category); err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	if err := s.redisClient.DeleteCategories(ctx); err != nil {
		fmt.Printf("failed to invalidate categories cache: %v\n", err)
	}

	return category, nil
}

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

func (s *CatalogService) GetAllCategories(ctx context.Context) ([]entity.Category, error) {
	categories, err := s.redisClient.GetCategories(ctx)
	if err == nil && len(categories) > 0 {
		metrics.RecordCacheHit("catalog-service", "categories")
		return categories, nil
	}

	metrics.RecordCacheMiss("catalog-service", "categories")

	categories, err = s.categoryRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	if err := s.redisClient.SetCategories(ctx, categories, time.Hour); err != nil {
		fmt.Printf("failed to cache categories: %v\n", err)
	}

	return categories, nil
}

func (s *CatalogService) UpdateCategory(ctx context.Context, id uuid.UUID, req *entity.UpdateCategoryRequest) (*entity.Category, error) {
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	category.Name = req.Name

	if err := s.categoryRepo.Update(ctx, category); err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	if err := s.redisClient.DeleteCategories(ctx); err != nil {
		fmt.Printf("failed to invalidate categories cache: %v\n", err)
	}

	return category, nil
}

func (s *CatalogService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return ErrCategoryNotFound
		}
		return fmt.Errorf("failed to delete category: %w", err)
	}

	if err := s.redisClient.DeleteCategories(ctx); err != nil {
		fmt.Printf("failed to invalidate categories cache: %v\n", err)
	}

	return nil
}

func (s *CatalogService) CreateProduct(ctx context.Context, req *entity.CreateProductRequest) (*entity.Product, error) {
	if _, err := s.categoryRepo.GetByID(ctx, req.CategoryID); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to verify category: %w", err)
	}

	product := &entity.Product{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		CategoryID:  req.CategoryID,
		CreatedAt:   time.Now(),
	}

	if err := s.productRepo.Create(ctx, product); err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return product, nil
}

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

func (s *CatalogService) GetAllProducts(ctx context.Context) ([]entity.ProductWithCategory, error) {
	products, err := s.productRepo.GetAllWithCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}
	return products, nil
}

func (s *CatalogService) UpdateProduct(ctx context.Context, id uuid.UUID, req *entity.UpdateProductRequest) (*entity.Product, error) {
	product, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	oldPrice := product.Price

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
		if _, err := s.categoryRepo.GetByID(ctx, req.CategoryID); err != nil {
			if errors.Is(err, repository.ErrCategoryNotFound) {
				return nil, ErrCategoryNotFound
			}
			return nil, fmt.Errorf("failed to verify category: %w", err)
		}
		product.CategoryID = req.CategoryID
	}

	if err := s.productRepo.Update(ctx, product); err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

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
			fmt.Printf("failed to publish product updated event: %v\n", err)
		}
	}

	return product, nil
}

func (s *CatalogService) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	_, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return ErrProductNotFound
		}
		return fmt.Errorf("failed to get product: %w", err)
	}

	if err := s.productRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}

func (s *CatalogService) publishProductEvent(ctx context.Context, event entity.ProductEvent) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal product event: %w", err)
	}

	if err := s.kafkaProducer.PublishMessage(ctx, event.ProductID.String(), eventData); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	return nil
}

func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			switch e.Tag() {
			case "required":
				return e.Field() + " is required"
			case "min":
				return e.Field() + " must be at least " + e.Param() + " characters"
			case "gt":
				return e.Field() + " must be greater than " + e.Param()
			}
		}
	}
	return err.Error()
}
