package service

import (
	"context"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
)

type CatalogServiceInterface interface {
	CreateCategory(ctx context.Context, req *entity.CreateCategoryRequest) (*entity.Category, error)
	GetCategory(ctx context.Context, id uuid.UUID) (*entity.Category, error)
	GetAllCategories(ctx context.Context) ([]entity.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, req *entity.UpdateCategoryRequest) (*entity.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error

	CreateProduct(ctx context.Context, req *entity.CreateProductRequest) (*entity.Product, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error)
	GetAllProducts(ctx context.Context) ([]entity.Product, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, req *entity.UpdateProductRequest) (*entity.Product, error)
	DeleteProduct(ctx context.Context, id uuid.UUID) error
}
