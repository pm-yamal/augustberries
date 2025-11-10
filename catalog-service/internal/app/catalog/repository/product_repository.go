package repository

import (
	"context"
	"errors"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrProductNotFound = errors.New("product not found")
)

type productRepository struct {
	db *gorm.DB
}

// NewProductRepository создает новый репозиторий товаров
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

// Create создает новый товар
func (r *productRepository) Create(ctx context.Context, product *entity.Product) error {
	result := r.db.WithContext(ctx).Create(product)
	return result.Error
}

// GetByID получает товар по ID
func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Product, error) {
	var product entity.Product
	result := r.db.WithContext(ctx).First(&product, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, result.Error
	}

	return &product, nil
}

// GetAll получает все товары
func (r *productRepository) GetAll(ctx context.Context) ([]entity.Product, error) {
	var products []entity.Product
	result := r.db.WithContext(ctx).Order("created_at DESC").Find(&products)

	if result.Error != nil {
		return nil, result.Error
	}

	return products, nil
}

// GetWithCategory получает товар с информацией о категории
func (r *productRepository) GetWithCategory(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error) {
	var product entity.Product
	result := r.db.WithContext(ctx).Preload("Category").First(&product, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, result.Error
	}

	// Создаем ProductWithCategory из product
	pwc := &entity.ProductWithCategory{
		Product: product,
	}
	if product.Category != nil {
		pwc.Category = *product.Category
	}

	return pwc, nil
}

// GetAllWithCategories получает все товары с информацией о категориях
func (r *productRepository) GetAllWithCategories(ctx context.Context) ([]entity.ProductWithCategory, error) {
	var products []entity.Product
	result := r.db.WithContext(ctx).Preload("Category").Order("created_at DESC").Find(&products)

	if result.Error != nil {
		return nil, result.Error
	}

	// Преобразуем в ProductWithCategory
	var productsWithCat []entity.ProductWithCategory
	for _, p := range products {
		pwc := entity.ProductWithCategory{
			Product: p,
		}
		if p.Category != nil {
			pwc.Category = *p.Category
		}
		productsWithCat = append(productsWithCat, pwc)
	}

	return productsWithCat, nil
}

// Update обновляет товар
func (r *productRepository) Update(ctx context.Context, product *entity.Product) error {
	result := r.db.WithContext(ctx).Model(product).Where("id = ?", product.ID).Updates(map[string]interface{}{
		"name":        product.Name,
		"description": product.Description,
		"price":       product.Price,
		"category_id": product.CategoryID,
	})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrProductNotFound
	}

	return nil
}

// Delete удаляет товар
func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&entity.Product{}, "id = ?", id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrProductNotFound
	}

	return nil
}
