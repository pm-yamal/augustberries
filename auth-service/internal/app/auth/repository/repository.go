package repository

import (
	"context"
	"errors"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("not found")
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]entity.User, error)
}

type RoleRepository interface {
	GetByID(ctx context.Context, id int) (*entity.Role, error)
	GetByName(ctx context.Context, name string) (*entity.Role, error)
	List(ctx context.Context) ([]entity.Role, error)
	Create(ctx context.Context, role *entity.Role) error
	Update(ctx context.Context, role *entity.Role) error
	Delete(ctx context.Context, id int) error

	GetPermissionsByRoleID(ctx context.Context, roleID int) ([]entity.Permission, error)
	ListPermissions(ctx context.Context) ([]entity.Permission, error)
	CreatePermission(ctx context.Context, permission *entity.Permission) error
	DeletePermission(ctx context.Context, id int) error

	AssignPermissions(ctx context.Context, roleID int, permissionIDs []int) error
	RemovePermissions(ctx context.Context, roleID int, permissionIDs []int) error
}

type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, token string) (*entity.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error

	AddToBlacklist(ctx context.Context, token string, expiresAt time.Time) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
	CleanupExpiredTokens(ctx context.Context) error
}
