package service

import (
	"context"
	"errors"
	"fmt"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository"

	"github.com/jackc/pgx/v5"
)

type RoleService struct {
	roleRepo repository.RoleRepository
}

func NewRoleService(roleRepo repository.RoleRepository) *RoleService {
	return &RoleService{
		roleRepo: roleRepo,
	}
}

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

func (s *RoleService) List(ctx context.Context) ([]entity.Role, error) {
	roles, err := s.roleRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return roles, nil
}

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

func (s *RoleService) Update(ctx context.Context, id int, req *entity.UpdateRoleRequest) (*entity.Role, error) {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

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

func (s *RoleService) Delete(ctx context.Context, id int) error {
	if err := s.roleRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to delete role: %w", err)
	}

	return nil
}

func (s *RoleService) GetPermissions(ctx context.Context, roleID int) ([]entity.Permission, error) {
	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	return permissions, nil
}

func (s *RoleService) AssignPermissions(ctx context.Context, roleID int, permissionIDs []int) error {
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

func (s *RoleService) RemovePermissions(ctx context.Context, roleID int, permissionIDs []int) error {
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

type PermissionService struct {
	roleRepo repository.RoleRepository
}

func NewPermissionService(roleRepo repository.RoleRepository) *PermissionService {
	return &PermissionService{
		roleRepo: roleRepo,
	}
}

func (s *PermissionService) List(ctx context.Context) ([]entity.Permission, error) {
	permissions, err := s.roleRepo.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, nil
}

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

func (s *PermissionService) Delete(ctx context.Context, id int) error {
	if err := s.roleRepo.DeletePermission(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPermissionNotFound
		}
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	return nil
}
