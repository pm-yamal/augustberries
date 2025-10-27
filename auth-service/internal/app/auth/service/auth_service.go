package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuthService обрабатывает бизнес-логику аутентификации
type AuthService struct {
	userRepo   repository.UserRepository
	roleRepo   repository.RoleRepository
	tokenRepo  repository.TokenRepository
	jwtManager *util.JWTManager
}

// NewAuthService создает новый сервис аутентификации
func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	jwtManager *util.JWTManager,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		roleRepo:   roleRepo,
		tokenRepo:  tokenRepo,
		jwtManager: jwtManager,
	}
}

// Register регистрирует нового пользователя
func (s *AuthService) Register(ctx context.Context, req *entity.RegisterRequest) (*entity.AuthResponse, error) {
	// Проверяем, существует ли пользователь с таким email
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, ErrUserExists
	}

	// Хэшируем пароль
	passwordHash, err := util.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Получаем роль "user" по умолчанию
	userRole, err := s.roleRepo.GetByName(ctx, "user")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get default role: %w", err)
	}

	// Создаем нового пользователя
	user := &entity.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: passwordHash,
		Name:         req.Name,
		RoleID:       userRole.ID,
		CreatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Генерируем токены
	return s.generateAuthResponse(ctx, user)
}

// Login выполняет вход пользователя
func (s *AuthService) Login(ctx context.Context, req *entity.LoginRequest) (*entity.AuthResponse, error) {
	// Получаем пользователя по email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем пароль
	if !util.CheckPassword(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Генерируем токены
	return s.generateAuthResponse(ctx, user)
}

// RefreshTokens обновляет access и refresh токены
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*entity.TokenPair, error) {
	// Проверяем refresh токен в БД
	storedToken, err := s.tokenRepo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	// Удаляем использованный refresh токен
	if err := s.tokenRepo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("failed to delete refresh token: %w", err)
	}

	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Получаем роль и разрешения
	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Генерируем новую пару токенов
	return s.generateTokenPair(ctx, user, role, permissions)
}

// GetCurrentUser получает информацию о текущем пользователе
func (s *AuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*entity.UserWithRole, error) {
	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Получаем роль
	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	// Получаем разрешения
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

// Logout выполняет выход пользователя (инвалидирует токены)
func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID, accessToken string) error {
	// Добавляем access токен в черный список
	claims, err := s.jwtManager.ValidateToken(accessToken)
	if err != nil {
		// Если токен невалидный, все равно продолжаем
		return nil
	}

	if err := s.tokenRepo.AddToBlacklist(ctx, accessToken, claims.ExpiresAt.Time); err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	// Удаляем все refresh токены пользователя
	if err := s.tokenRepo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete refresh tokens: %w", err)
	}

	return nil
}

// ValidateToken проверяет JWT токен
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*util.JWTClaims, error) {
	// Проверяем, не находится ли токен в черном списке
	isBlacklisted, err := s.tokenRepo.IsBlacklisted(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to check blacklist: %w", err)
	}
	if isBlacklisted {
		return nil, util.ErrInvalidToken
	}

	// Валидируем токен
	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// generateAuthResponse создает полный ответ с пользователем и токенами
func (s *AuthService) generateAuthResponse(ctx context.Context, user *entity.User) (*entity.AuthResponse, error) {
	// Получаем роль
	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	// Получаем разрешения
	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Генерируем токены
	tokenPair, err := s.generateTokenPair(ctx, user, role, permissions)
	if err != nil {
		return nil, err
	}

	return &entity.AuthResponse{
		User: entity.UserWithRole{
			User:        *user,
			Role:        *role,
			Permissions: permissions,
		},
		Tokens: *tokenPair,
	}, nil
}

// generateTokenPair генерирует пару токенов (access + refresh)
func (s *AuthService) generateTokenPair(
	ctx context.Context,
	user *entity.User,
	role *entity.Role,
	permissions []entity.Permission,
) (*entity.TokenPair, error) {
	// Создаем список кодов разрешений
	permissionCodes := make([]string, len(permissions))
	for i, p := range permissions {
		permissionCodes[i] = p.Code
	}

	// Генерируем access токен
	accessToken, err := s.jwtManager.GenerateAccessToken(
		user.ID,
		user.Email,
		user.RoleID,
		role.Name,
		permissionCodes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Генерируем refresh токен
	refreshToken, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Сохраняем refresh токен в БД
	expiresAt := time.Now().Add(s.jwtManager.GetRefreshTokenDuration())
	if err := s.tokenRepo.SaveRefreshToken(ctx, user.ID, refreshToken, expiresAt); err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtManager.GetAccessTokenDuration().Seconds()),
	}, nil
}
