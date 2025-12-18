package service

import (
	"context"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/google/uuid"
)

type AuthServiceInterface interface {
	Register(ctx context.Context, req *entity.RegisterRequest) (*entity.AuthResponse, error)
	Login(ctx context.Context, req *entity.LoginRequest) (*entity.AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*entity.AuthResponse, error)
	Logout(ctx context.Context, accessToken, refreshToken string) error
	ValidateToken(ctx context.Context, accessToken string) (*entity.TokenValidationResponse, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*entity.UserWithRole, error)
}

type UserServiceInterface interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, id uuid.UUID, req *entity.UpdateUserRequest) (*entity.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]entity.User, int, error)
}

type RoleServiceInterface interface {
	Create(ctx context.Context, req *entity.CreateRoleRequest) (*entity.Role, error)
	GetByID(ctx context.Context, id int) (*entity.Role, error)
	GetByName(ctx context.Context, name string) (*entity.Role, error)
	List(ctx context.Context) ([]entity.Role, error)
	Update(ctx context.Context, id int, req *entity.UpdateRoleRequest) (*entity.Role, error)
	Delete(ctx context.Context, id int) error
	GetPermissions(ctx context.Context, roleID int) ([]entity.Permission, error)
	AssignPermissions(ctx context.Context, roleID int, permissionIDs []int) error
	RemovePermissions(ctx context.Context, roleID int, permissionIDs []int) error
	CreatePermission(ctx context.Context, req *entity.CreatePermissionRequest) (*entity.Permission, error)
	ListPermissions(ctx context.Context) ([]entity.Permission, error)
	DeletePermission(ctx context.Context, id int) error
}

type JWTManagerInterface interface {
	GenerateTokenPair(userID uuid.UUID, email, role string, permissions []string) (*entity.TokenPair, error)
	ValidateAccessToken(tokenString string) (*entity.TokenClaims, error)
	ValidateRefreshToken(tokenString string) (*entity.TokenClaims, error)
	GetAccessDuration() int64
}
