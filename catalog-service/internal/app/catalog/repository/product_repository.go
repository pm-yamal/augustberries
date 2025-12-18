package repository

import (
	"context"
	"errors"
	"strings"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(ctx context.Context, product *entity.Product) error {
	result := r.db.WithContext(ctx).Create(product)
	return wrapGormError(result.Error)
}

func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Product, error) {
	var product entity.Product
	result := r.db.WithContext(ctx).First(&product, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, wrapGormError(result.Error)
	}

	return &product, nil
}

func (r *productRepository) GetAll(ctx context.Context) ([]entity.Product, error) {
	var products []entity.Product
	result := r.db.WithContext(ctx).Order("created_at DESC").Find(&products)

	if result.Error != nil {
		return nil, wrapGormError(result.Error)
	}

	return products, nil
}

func (r *productRepository) GetWithCategory(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error) {
	var product entity.Product
	result := r.db.WithContext(ctx).Preload("Category").First(&product, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, wrapGormError(result.Error)
	}

	pwc := &entity.ProductWithCategory{
		Product: product,
	}
	if product.Category != nil {
		pwc.Category = *product.Category
	}

	return pwc, nil
}

func (r *productRepository) GetAllWithCategories(ctx context.Context) ([]entity.ProductWithCategory, error) {
	var products []entity.Product
	result := r.db.WithContext(ctx).Preload("Category").Order("created_at DESC").Find(&products)

	if result.Error != nil {
		return nil, wrapGormError(result.Error)
	}

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

func (r *productRepository) Update(ctx context.Context, product *entity.Product) error {
	result := r.db.WithContext(ctx).Model(product).Where("id = ?", product.ID).Updates(map[string]interface{}{
		"name":        product.Name,
		"description": product.Description,
		"price":       product.Price,
		"category_id": product.CategoryID,
	})

	if result.Error != nil {
		return wrapGormError(result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&entity.Product{}, "id = ?", id)

	if result.Error != nil {
		return wrapGormError(result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func wrapGormError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}

	errStr := err.Error()
	if strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "unique constraint") {
		return ErrDuplicateKey
	}
	if strings.Contains(errStr, "foreign key") || strings.Contains(errStr, "violates foreign key") {
		return ErrForeignKey
	}

	return err
}
