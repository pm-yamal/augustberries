package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type AuthService struct {
	userRepo   repository.UserRepository
	roleRepo   repository.RoleRepository
	tokenRepo  repository.TokenRepository
	jwtManager JWTManagerInterface
	validator  *validator.Validate
}

func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	jwtManager JWTManagerInterface,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		roleRepo:   roleRepo,
		tokenRepo:  tokenRepo,
		jwtManager: jwtManager,
		validator:  validator.New(),
	}
}

func (s *AuthService) Register(ctx context.Context, req *entity.RegisterRequest) (*entity.AuthResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, formatValidationError(err))
	}

	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, ErrUserExists
	}

	passwordHash, err := util.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userRole, err := s.roleRepo.GetByName(ctx, "user")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get default role: %w", err)
	}

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

	return s.generateAuthResponse(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, req *entity.LoginRequest) (*entity.AuthResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, formatValidationError(err))
	}

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !util.CheckPassword(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	return s.generateAuthResponse(ctx, user)
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*entity.AuthResponse, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("%w: refresh_token is required", ErrValidation)
	}

	storedToken, err := s.tokenRepo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	if err := s.tokenRepo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("failed to delete refresh token: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return s.generateAuthResponse(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := s.jwtManager.ValidateAccessToken(accessToken)
	if err == nil && claims != nil {
		if err := s.tokenRepo.AddToBlacklist(ctx, accessToken, claims.ExpiresAt); err != nil {
			return fmt.Errorf("failed to blacklist token: %w", err)
		}
	}

	if refreshToken != "" {
		s.tokenRepo.DeleteRefreshToken(ctx, refreshToken)
	}

	return nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*entity.TokenValidationResponse, error) {
	isBlacklisted, err := s.tokenRepo.IsBlacklisted(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to check blacklist: %w", err)
	}
	if isBlacklisted {
		return nil, ErrTokenBlacklisted
	}

	claims, err := s.jwtManager.ValidateAccessToken(token)
	if err != nil {
		if errors.Is(err, util.ErrExpiredToken) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	return &entity.TokenValidationResponse{
		Valid:       true,
		UserID:      claims.UserID,
		Email:       claims.Email,
		Role:        claims.Role,
		Permissions: claims.Permissions,
	}, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*entity.UserWithRole, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get user role: %w", err)
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

func (s *AuthService) generateAuthResponse(ctx context.Context, user *entity.User) (*entity.AuthResponse, error) {
	role, err := s.roleRepo.GetByID(ctx, user.RoleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	permissions, err := s.roleRepo.GetPermissionsByRoleID(ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	permissionCodes := make([]string, len(permissions))
	for i, p := range permissions {
		permissionCodes[i] = p.Code
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, role.Name, permissionCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	if err := s.tokenRepo.SaveRefreshToken(ctx, user.ID, tokenPair.RefreshToken, time.Now().Add(7*24*time.Hour)); err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
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

func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			switch e.Tag() {
			case "required":
				return e.Field() + " is required"
			case "email":
				return e.Field() + " must be a valid email"
			case "min":
				return e.Field() + " must be at least " + e.Param() + " characters"
			}
		}
	}
	return err.Error()
}
