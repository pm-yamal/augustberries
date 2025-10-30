package repository

import (
	"context"
	"errors"
	"fmt"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrProductNotFound = errors.New("product not found")
)

type productRepository struct {
	db *pgxpool.Pool
}

// NewProductRepository создает новый репозиторий товаров
func NewProductRepository(db *pgxpool.Pool) ProductRepository {
	return &productRepository{db: db}
}

// Create создает новый товар
func (r *productRepository) Create(ctx context.Context, product *entity.Product) error {
	query := `
		INSERT INTO products (id, name, description, price, category_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Exec(
		ctx, query,
		product.ID, product.Name, product.Description,
		product.Price, product.CategoryID, product.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}

	return nil
}

// GetByID получает товар по ID
func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Product, error) {
	query := `
		SELECT id, name, description, price, category_id, created_at 
		FROM products 
		WHERE id = $1
	`

	var product entity.Product
	err := r.db.QueryRow(ctx, query, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.CategoryID,
		&product.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product by id: %w", err)
	}

	return &product, nil
}

// GetAll получает все товары
func (r *productRepository) GetAll(ctx context.Context) ([]entity.Product, error) {
	query := `
		SELECT id, name, description, price, category_id, created_at 
		FROM products 
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}
	defer rows.Close()

	var products []entity.Product
	for rows.Next() {
		var product entity.Product
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.Price,
			&product.CategoryID,
			&product.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}

	return products, nil
}

// GetWithCategory получает товар с информацией о категории
func (r *productRepository) GetWithCategory(ctx context.Context, id uuid.UUID) (*entity.ProductWithCategory, error) {
	query := `
		SELECT 
			p.id, p.name, p.description, p.price, p.category_id, p.created_at,
			c.id, c.name, c.created_at
		FROM products p
		INNER JOIN categories c ON p.category_id = c.id
		WHERE p.id = $1
	`

	var pwc entity.ProductWithCategory
	err := r.db.QueryRow(ctx, query, id).Scan(
		&pwc.ID,
		&pwc.Name,
		&pwc.Description,
		&pwc.Price,
		&pwc.CategoryID,
		&pwc.CreatedAt,
		&pwc.Category.ID,
		&pwc.Category.Name,
		&pwc.Category.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product with category: %w", err)
	}

	return &pwc, nil
}

// GetAllWithCategories получает все товары с информацией о категориях
func (r *productRepository) GetAllWithCategories(ctx context.Context) ([]entity.ProductWithCategory, error) {
	query := `
		SELECT 
			p.id, p.name, p.description, p.price, p.category_id, p.created_at,
			c.id, c.name, c.created_at
		FROM products p
		INNER JOIN categories c ON p.category_id = c.id
		ORDER BY p.created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get products with categories: %w", err)
	}
	defer rows.Close()

	var products []entity.ProductWithCategory
	for rows.Next() {
		var pwc entity.ProductWithCategory
		if err := rows.Scan(
			&pwc.ID,
			&pwc.Name,
			&pwc.Description,
			&pwc.Price,
			&pwc.CategoryID,
			&pwc.CreatedAt,
			&pwc.Category.ID,
			&pwc.Category.Name,
			&pwc.Category.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product with category: %w", err)
		}
		products = append(products, pwc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}

	return products, nil
}

// Update обновляет товар
func (r *productRepository) Update(ctx context.Context, product *entity.Product) error {
	query := `
		UPDATE products 
		SET name = $1, description = $2, price = $3, category_id = $4
		WHERE id = $5
	`

	result, err := r.db.Exec(
		ctx, query,
		product.Name, product.Description, product.Price,
		product.CategoryID, product.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrProductNotFound
	}

	return nil
}

// Delete удаляет товар
func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM products WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrProductNotFound
	}

	return nil
}
