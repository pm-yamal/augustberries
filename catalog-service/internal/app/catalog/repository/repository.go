package repository

import (
	"context"
	"errors"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrCategoryNotFound = errors.New("category not found")
	ErrProductNotFound  = errors.New("product not found")
	ErrDuplicateKey     = errors.New("duplicate key")
	ErrForeignKey       = errors.New("foreign key violation")
)

type CategoryRepository interface {
	Create(ctx context.Context, category *entity.Category) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Category, error)
	GetAll(ctx context.Context) ([]entity.Category, error)
	Update(ctx context.Context, category *entity.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProductRepository interface {
	Create(ctx context.Context, product *entity.Product) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Product, error)
	GetAll(ctx context.Context) ([]entity.Product, error)
	GetWithCategory(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error)
	GetAllWithCategories(ctx context.Context) ([]entity.ProductWithCategory, error)
	Update(ctx context.Context, product *entity.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}
