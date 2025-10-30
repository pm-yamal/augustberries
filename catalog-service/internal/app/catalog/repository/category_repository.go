package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrCategoryNotFound Стандартные ошибки репозитория для обработки в service layer
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryAlreadyExists = errors.New("category with this name already exists")
	ErrCategoryHasProducts   = errors.New("cannot delete category with existing products")
)

type categoryRepository struct {
	db *pgxpool.Pool // Пул соединений с PostgreSQL для работы с категориями
}

// NewCategoryRepository создает новый репозиторий категорий
func NewCategoryRepository(db *pgxpool.Pool) CategoryRepository {
	return &categoryRepository{db: db}
}

// Create создает новую категорию в PostgreSQL
// Проверяет уникальность имени через UNIQUE constraint
func (r *categoryRepository) Create(ctx context.Context, category *entity.Category) error {
	query := `
		INSERT INTO categories (id, name, created_at)
		VALUES ($1, $2, $3)
	`

	_, err := r.db.Exec(ctx, query, category.ID, category.Name, category.CreatedAt)
	if err != nil {
		// ИСПРАВЛЕНО: Используем правильную проверку constraint через pgconn.PgError
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return ErrCategoryAlreadyExists
		}
		return fmt.Errorf("failed to create category: %w", err)
	}

	return nil
}

// GetByID получает категорию по ID из PostgreSQL
func (r *categoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Category, error) {
	query := `SELECT id, name, created_at FROM categories WHERE id = $1`

	var category entity.Category
	err := r.db.QueryRow(ctx, query, id).Scan(
		&category.ID,
		&category.Name,
		&category.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to get category by id: %w", err)
	}

	return &category, nil
}

// GetAll получает все категории отсортированные по имени
// Результат может быть закеширован в Redis через service layer
func (r *categoryRepository) GetAll(ctx context.Context) ([]entity.Category, error) {
	query := `SELECT id, name, created_at FROM categories ORDER BY name ASC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	var categories []entity.Category
	for rows.Next() {
		var category entity.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}

	// Проверяем ошибки итерации
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

// Update обновляет категорию в PostgreSQL
// Проверяет уникальность нового имени
func (r *categoryRepository) Update(ctx context.Context, category *entity.Category) error {
	query := `
		UPDATE categories 
		SET name = $1
		WHERE id = $2
	`

	result, err := r.db.Exec(ctx, query, category.Name, category.ID)
	if err != nil {
		// ИСПРАВЛЕНО: Используем правильную проверку constraint через pgconn.PgError
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return ErrCategoryAlreadyExists
		}
		return fmt.Errorf("failed to update category: %w", err)
	}

	// Проверяем что категория существует
	if result.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

// Delete удаляет категорию из PostgreSQL
// Если у категории есть товары, ON DELETE CASCADE автоматически удалит их
func (r *categoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Сначала проверяем есть ли товары в этой категории
	var productCount int
	checkQuery := `SELECT COUNT(*) FROM products WHERE category_id = $1`
	if err := r.db.QueryRow(ctx, checkQuery, id).Scan(&productCount); err != nil {
		return fmt.Errorf("failed to check products in category: %w", err)
	}

	// Если есть товары, возвращаем ошибку
	// Это более явный и безопасный подход чем полагаться только на CASCADE
	if productCount > 0 {
		return ErrCategoryHasProducts
	}

	// Удаляем категорию
	query := `DELETE FROM categories WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		// Проверяем foreign key constraint на случай race condition
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return ErrCategoryHasProducts
		}
		return fmt.Errorf("failed to delete category: %w", err)
	}

	// Проверяем что категория существовала
	if result.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

// GetByName для поиска категории по имени
// Может быть полезна для проверки уникальности перед созданием
func (r *categoryRepository) GetByName(ctx context.Context, name string) (*entity.Category, error) {
	query := `SELECT id, name, created_at FROM categories WHERE LOWER(name) = LOWER($1)`

	var category entity.Category
	err := r.db.QueryRow(ctx, query, strings.TrimSpace(name)).Scan(
		&category.ID,
		&category.Name,
		&category.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to get category by name: %w", err)
	}

	return &category, nil
}
