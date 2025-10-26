package repository

import (
	"context"
	"errors"
	"fmt"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRoleNotFound = errors.New("role not found")
)

type roleRepository struct {
	db *pgxpool.Pool
}

// NewRoleRepository создает новый репозиторий ролей
func NewRoleRepository(db *pgxpool.Pool) RoleRepository {
	return &roleRepository{db: db}
}

// GetByID получает роль по ID
func (r *roleRepository) GetByID(ctx context.Context, id int) (*entity.Role, error) {
	query := `SELECT id, name, description FROM roles WHERE id = $1`

	var role entity.Role
	err := r.db.QueryRow(ctx, query, id).Scan(&role.ID, &role.Name, &role.Description)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by id: %w", err)
	}

	return &role, nil
}

// GetByName получает роль по имени
func (r *roleRepository) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	query := `SELECT id, name, description FROM roles WHERE name = $1`

	var role entity.Role
	err := r.db.QueryRow(ctx, query, name).Scan(&role.ID, &role.Name, &role.Description)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by name: %w", err)
	}

	return &role, nil
}

// GetPermissionsByRoleID получает все разрешения для роли
func (r *roleRepository) GetPermissionsByRoleID(ctx context.Context, roleID int) ([]entity.Permission, error) {
	query := `
		SELECT p.id, p.code, p.description
		FROM permissions p
		INNER JOIN roles_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.code
	`

	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions for role: %w", err)
	}
	defer rows.Close()

	var permissions []entity.Permission
	for rows.Next() {
		var p entity.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Description); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}

	return permissions, nil
}
