package repository

import (
	"context"
	"errors"
	"fmt"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type roleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) GetByID(ctx context.Context, id int) (*entity.Role, error) {
	query := `SELECT id, name, description FROM roles WHERE id = $1`

	var role entity.Role
	err := r.db.QueryRow(ctx, query, id).Scan(&role.ID, &role.Name, &role.Description)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get role by id: %w", err)
	}

	return &role, nil
}

func (r *roleRepository) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	query := `SELECT id, name, description FROM roles WHERE name = $1`

	var role entity.Role
	err := r.db.QueryRow(ctx, query, name).Scan(&role.ID, &role.Name, &role.Description)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get role by name: %w", err)
	}

	return &role, nil
}

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

func (r *roleRepository) List(ctx context.Context) ([]entity.Role, error) {
	query := `SELECT id, name, description FROM roles ORDER BY name`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []entity.Role
	for rows.Next() {
		var role entity.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating roles: %w", err)
	}

	return roles, nil
}

func (r *roleRepository) Create(ctx context.Context, role *entity.Role) error {
	query := `
		INSERT INTO roles (name, description)
		VALUES ($1, $2)
		RETURNING id
	`

	err := r.db.QueryRow(ctx, query, role.Name, role.Description).Scan(&role.ID)
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	return nil
}

func (r *roleRepository) Update(ctx context.Context, role *entity.Role) error {
	query := `
		UPDATE roles 
		SET name = $1, description = $2
		WHERE id = $3
	`

	result, err := r.db.Exec(ctx, query, role.Name, role.Description, role.ID)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *roleRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM roles WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *roleRepository) ListPermissions(ctx context.Context) ([]entity.Permission, error) {
	query := `SELECT id, code, description FROM permissions ORDER BY code`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
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

func (r *roleRepository) CreatePermission(ctx context.Context, permission *entity.Permission) error {
	query := `
		INSERT INTO permissions (code, description)
		VALUES ($1, $2)
		RETURNING id
	`

	err := r.db.QueryRow(ctx, query, permission.Code, permission.Description).Scan(&permission.ID)
	if err != nil {
		return fmt.Errorf("failed to create permission: %w", err)
	}

	return nil
}

func (r *roleRepository) DeletePermission(ctx context.Context, id int) error {
	query := `DELETE FROM permissions WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *roleRepository) AssignPermissions(ctx context.Context, roleID int, permissionIDs []int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	deleteQuery := `DELETE FROM roles_permissions WHERE role_id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, roleID); err != nil {
		return fmt.Errorf("failed to delete old permissions: %w", err)
	}

	insertQuery := `INSERT INTO roles_permissions (role_id, permission_id) VALUES ($1, $2)`
	for _, permID := range permissionIDs {
		if _, err := tx.Exec(ctx, insertQuery, roleID, permID); err != nil {
			return fmt.Errorf("failed to assign permission %d: %w", permID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *roleRepository) RemovePermissions(ctx context.Context, roleID int, permissionIDs []int) error {
	query := `DELETE FROM roles_permissions WHERE role_id = $1 AND permission_id = ANY($2)`

	_, err := r.db.Exec(ctx, query, roleID, permissionIDs)
	if err != nil {
		return fmt.Errorf("failed to remove permissions: %w", err)
	}

	return nil
}
