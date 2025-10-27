package service

import (
	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository"
	"augustberries/auth-service/internal/app/auth/util"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserService обрабатывает бизнес-логику работы с пользователями
type UserService struct {
	userRepo repository.UserRepository
	roleRepo repository.RoleRepository
}

// NewUserService создает новый сервис пользователей
func NewUserService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
) *UserService {
	return &UserService{
		userRepo: userRepo,
		roleRepo: roleRepo,
	}
}

// GetByID получает пользователя по ID
func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*entity.UserWithRole, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	return &entity.UserWithRole{
		User:        *user,
		Role:        *role,
		Permissions: permissions,
	}, nil
}

// GetByEmail получает пользователя по email
func (s *UserService) GetByEmail(ctx context.Context, email string) (*entity.UserWithRole, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	return &entity.UserWithRole{
		User:        *user,
		Role:        *role,
		Permissions: permissions,
	}, nil
}

// Update обновляет данные пользователя
func (s *UserService) Update(ctx context.Context, id uuid.UUID, req *entity.UpdateUserRequest) (*entity.User, error) {
	// Проверяем существование пользователя
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Обновляем поля
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.RoleID != 0 {
		// Проверяем существование роли
		_, err := s.roleRepo.GetByID(ctx, req.RoleID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrRoleNotFound
			}
			return nil, fmt.Errorf("failed to verify role: %w", err)
		}
		user.RoleID = req.RoleID
	}

	// Сохраняем изменения
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

// UpdatePassword обновляет пароль пользователя
func (s *UserService) UpdatePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error {
	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем старый пароль
	if !util.CheckPassword(oldPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	// Хэшируем новый пароль
	newPasswordHash, err := util.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Обновляем пароль
	user.PasswordHash = newPasswordHash
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	return nil
}

// Delete удаляет пользователя
func (s *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.userRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// List получает список всех пользователей (добавим метод в репозиторий)
func (s *UserService) List(ctx context.Context) ([]entity.UserWithRole, error) {
	users, err := s.userRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	result := make([]entity.UserWithRole, 0, len(users))
	for _, user := range users {
		role, err := s.roleRepo.GetByID(ctx, user.RoleID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue // Пропускаем пользователей с несуществующими ролями
			}
			return nil, fmt.Errorf("failed to get role for user %s: %w", user.ID, err)
		}

		permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, user.RoleID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for user %s: %w", user.ID, err)
		}

		result = append(result, entity.UserWithRole{
			User:        user,
			Role:        *role,
			Permissions: permissions,
		})
	}

	return result, nil
}
