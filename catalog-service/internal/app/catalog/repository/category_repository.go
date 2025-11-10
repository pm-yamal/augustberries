package repository

import (
	"context"
	"errors"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrCategoryNotFound Стандартные ошибки репозитория для обработки в service layer
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryAlreadyExists = errors.New("category with this name already exists")
	ErrCategoryHasProducts   = errors.New("cannot delete category with existing products")
)

type categoryRepository struct {
	db *gorm.DB // GORM DB для работы с PostgreSQL
}

// NewCategoryRepository создает новый репозиторий категорий
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

// Create создает новую категорию в PostgreSQL
// Проверяет уникальность имени через UNIQUE constraint
func (r *categoryRepository) Create(ctx context.Context, category *entity.Category) error {
	result := r.db.WithContext(ctx).Create(category)
	if result.Error != nil {
		// Проверяем на ошибку уникальности
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return ErrCategoryAlreadyExists
		}
		return result.Error
	}
	return nil
}

// GetByID получает категорию по ID из PostgreSQL
func (r *categoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Category, error) {
	var category entity.Category
	result := r.db.WithContext(ctx).First(&category, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, result.Error
	}

	return &category, nil
}

// GetAll получает все категории отсортированные по имени
// Результат может быть закеширован в Redis через service layer
func (r *categoryRepository) GetAll(ctx context.Context) ([]entity.Category, error) {
	var categories []entity.Category
	result := r.db.WithContext(ctx).Order("name ASC").Find(&categories)

	if result.Error != nil {
		return nil, result.Error
	}

	return categories, nil
}

// Update обновляет категорию в PostgreSQL
// Проверяет уникальность нового имени
func (r *categoryRepository) Update(ctx context.Context, category *entity.Category) error {
	result := r.db.WithContext(ctx).Model(category).Where("id = ?", category.ID).Updates(map[string]interface{}{
		"name": category.Name,
	})

	if result.Error != nil {
		// Проверяем на ошибку уникальности
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return ErrCategoryAlreadyExists
		}
		return result.Error
	}

	// Проверяем что категория существует
	if result.RowsAffected == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

// Delete удаляет категорию из PostgreSQL
// Проверяет наличие товаров в категории перед удалением
func (r *categoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Сначала проверяем есть ли товары в этой категории
	var productCount int64
	r.db.WithContext(ctx).Model(&entity.Product{}).Where("category_id = ?", id).Count(&productCount)

	// Если есть товары, возвращаем ошибку
	if productCount > 0 {
		return ErrCategoryHasProducts
	}

	// Удаляем категорию
	result := r.db.WithContext(ctx).Delete(&entity.Category{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	// Проверяем что категория существовала
	if result.RowsAffected == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

// GetByName для поиска категории по имени
// Может быть полезна для проверки уникальности перед созданием
func (r *categoryRepository) GetByName(ctx context.Context, name string) (*entity.Category, error) {
	var category entity.Category
	result := r.db.WithContext(ctx).Where("LOWER(name) = LOWER(?)", name).First(&category)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, result.Error
	}

	return &category, nil
}
