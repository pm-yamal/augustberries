package service

import (
	"context"
	"errors"
	"fmt"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository"

	"github.com/jackc/pgx/v5"
)

// RoleService обрабатывает бизнес-логику работы с ролями
type RoleService struct {
	roleRepo repository.RoleRepository
}

// NewRoleService создает новый сервис ролей
func NewRoleService(roleRepo repository.RoleRepository) *RoleService {
	return &RoleService{
		roleRepo: roleRepo,
	}
}

// GetByID получает роль по ID
func (s *RoleService) GetByID(ctx context.Context, id int) (*entity.Role, error) {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return role, nil
}

// GetByName получает роль по имени
func (s *RoleService) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	role, err := s.roleRepo.GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return role, nil
}

// List получает список всех ролей
func (s *RoleService) List(ctx context.Context) ([]entity.Role, error) {
	roles, err := s.roleRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return roles, nil
}

// Create создает новую роль
func (s *RoleService) Create(ctx context.Context, req *entity.CreateRoleRequest) (*entity.Role, error) {
	role := &entity.Role{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return role, nil
}

// Update обновляет роль
func (s *RoleService) Update(ctx context.Context, id int, req *entity.UpdateRoleRequest) (*entity.Role, error) {
	// Проверяем существование роли
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Обновляем поля
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	if err := s.roleRepo.Update(ctx, role); err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return role, nil
}

// Delete удаляет роль
func (s *RoleService) Delete(ctx context.Context, id int) error {
	if err := s.roleRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to delete role: %w", err)
	}

	return nil
}

// GetPermissions получает разрешения роли
func (s *RoleService) GetPermissions(ctx context.Context, roleID int) ([]entity.Permission, error) {
	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	return permissions, nil
}

// AssignPermissions назначает разрешения роли
func (s *RoleService) AssignPermissions(ctx context.Context, roleID int, permissionIDs []int) error {
	// Проверяем существование роли
	_, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	if err := s.roleRepo.AssignPermissions(ctx, roleID, permissionIDs); err != nil {
		return fmt.Errorf("failed to assign permissions: %w", err)
	}

	return nil
}

// RemovePermissions удаляет разрешения у роли
func (s *RoleService) RemovePermissions(ctx context.Context, roleID int, permissionIDs []int) error {
	// Проверяем существование роли
	_, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	if err := s.roleRepo.RemovePermissions(ctx, roleID, permissionIDs); err != nil {
		return fmt.Errorf("failed to remove permissions: %w", err)
	}

	return nil
}

// PermissionService обрабатывает бизнес-логику работы с разрешениями
type PermissionService struct {
	roleRepo repository.RoleRepository
}

// NewPermissionService создает новый сервис разрешений
func NewPermissionService(roleRepo repository.RoleRepository) *PermissionService {
	return &PermissionService{
		roleRepo: roleRepo,
	}
}

// List получает список всех разрешений
func (s *PermissionService) List(ctx context.Context) ([]entity.Permission, error) {
	permissions, err := s.roleRepo.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, nil
}

// Create создает новое разрешение
func (s *PermissionService) Create(ctx context.Context, req *entity.CreatePermissionRequest) (*entity.Permission, error) {
	permission := &entity.Permission{
		Code:        req.Code,
		Description: req.Description,
	}

	if err := s.roleRepo.CreatePermission(ctx, permission); err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	return permission, nil
}

// Delete удаляет разрешение
func (s *PermissionService) Delete(ctx context.Context, id int) error {
	if err := s.roleRepo.DeletePermission(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPermissionNotFound
		}
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	return nil
}
