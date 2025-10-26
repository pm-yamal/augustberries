package repository

import (
	"context"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"github.com/google/uuid"
)

// UserRepository определяет методы для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// RoleRepository определяет методы для работы с ролями
type RoleRepository interface {
	GetByID(ctx context.Context, id int) (*entity.Role, error)
	GetByName(ctx context.Context, name string) (*entity.Role, error)
	GetPermissionsByRoleID(ctx context.Context, roleID int) ([]entity.Permission, error)
}

// TokenRepository определяет методы для работы с токенами
type TokenRepository interface {
	// Refresh tokens
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, token string) (*entity.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error

	// Blacklisted tokens
	AddToBlacklist(ctx context.Context, token string, expiresAt time.Time) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
	CleanupExpiredTokens(ctx context.Context) error
}
