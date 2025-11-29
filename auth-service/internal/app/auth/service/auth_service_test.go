package service

import (
	"context"
	"testing"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository/mocks"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Хелперы для создания тестовых данных
func newTestJWTManager() *util.JWTManager {
	return util.NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
}

func newTestUser() *entity.User {
	hash, _ := util.HashPassword("password123")
	return &entity.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: hash,
		Name:         "Test User",
		RoleID:       1,
		CreatedAt:    time.Now(),
	}
}

func newTestRole() *entity.Role {
	return &entity.Role{
		ID:          1,
		Name:        "user",
		Description: "Regular user",
	}
}

func newTestPermissions() []entity.Permission {
	return []entity.Permission{
		{ID: 1, Code: "product.read", Description: "Read products"},
		{ID: 2, Code: "order.create", Description: "Create orders"},
	}
}

// ==================== Register Tests ====================

func TestAuthService_Register_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	role := newTestRole()
	permissions := newTestPermissions()

	// Настраиваем моки
	userRepo.On("GetByEmail", ctx, "newuser@example.com").Return(nil, pgx.ErrNoRows)
	userRepo.On("Create", ctx, mock.AnythingOfType("*entity.User")).Return(nil)
	roleRepo.On("GetByName", ctx, "user").Return(role, nil)
	roleRepo.On("GetByID", ctx, 1).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, 1).Return(permissions, nil)
	tokenRepo.On("SaveRefreshToken", ctx, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	req := &entity.RegisterRequest{
		Email:    "newuser@example.com",
		Password: "password123",
		Name:     "New User",
	}

	// Act
	response, err := service.Register(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "newuser@example.com", response.User.Email)
	assert.Equal(t, "New User", response.User.Name)
	assert.NotEmpty(t, response.Tokens.AccessToken)
	assert.NotEmpty(t, response.Tokens.RefreshToken)
	assert.Equal(t, "user", response.User.Role.Name)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	tokenRepo.AssertExpectations(t)
}

func TestAuthService_Register_UserAlreadyExists(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	existingUser := newTestUser()
	userRepo.On("GetByEmail", ctx, "existing@example.com").Return(existingUser, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	req := &entity.RegisterRequest{
		Email:    "existing@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	// Act
	response, err := service.Register(ctx, req)

	// Assert
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrUserExists)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_RoleNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	userRepo.On("GetByEmail", ctx, "test@example.com").Return(nil, pgx.ErrNoRows)
	roleRepo.On("GetByName", ctx, "user").Return(nil, pgx.ErrNoRows)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	req := &entity.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	// Act
	response, err := service.Register(ctx, req)

	// Assert
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== Login Tests ====================

func TestAuthService_Login_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()
	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)
	roleRepo.On("GetByID", ctx, user.RoleID).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, user.RoleID).Return(permissions, nil)
	tokenRepo.On("SaveRefreshToken", ctx, user.ID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	req := &entity.LoginRequest{
		Email:    user.Email,
		Password: "password123",
	}

	// Act
	response, err := service.Login(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, user.Email, response.User.Email)
	assert.NotEmpty(t, response.Tokens.AccessToken)
	assert.NotEmpty(t, response.Tokens.RefreshToken)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
	tokenRepo.AssertExpectations(t)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	userRepo.On("GetByEmail", ctx, "notfound@example.com").Return(nil, pgx.ErrNoRows)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	req := &entity.LoginRequest{
		Email:    "notfound@example.com",
		Password: "password123",
	}

	// Act
	response, err := service.Login(ctx, req)

	// Assert
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()
	userRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	req := &entity.LoginRequest{
		Email:    user.Email,
		Password: "wrongpassword",
	}

	// Act
	response, err := service.Login(ctx, req)

	// Assert
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

// ==================== RefreshTokens Tests ====================

func TestAuthService_RefreshTokens_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()
	role := newTestRole()
	permissions := newTestPermissions()
	refreshToken := "valid-refresh-token"

	storedToken := &entity.RefreshToken{
		ID:        1,
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	tokenRepo.On("GetRefreshToken", ctx, refreshToken).Return(storedToken, nil)
	tokenRepo.On("DeleteRefreshToken", ctx, refreshToken).Return(nil)
	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	roleRepo.On("GetByID", ctx, user.RoleID).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, user.RoleID).Return(permissions, nil)
	tokenRepo.On("SaveRefreshToken", ctx, user.ID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	tokenPair, err := service.RefreshTokens(ctx, refreshToken)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, tokenPair)
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.NotEqual(t, refreshToken, tokenPair.RefreshToken) // Новый токен должен отличаться

	tokenRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
}

func TestAuthService_RefreshTokens_InvalidToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	tokenRepo.On("GetRefreshToken", ctx, "invalid-token").Return(nil, pgx.ErrNoRows)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	tokenPair, err := service.RefreshTokens(ctx, "invalid-token")

	// Assert
	assert.Nil(t, tokenPair)
	assert.ErrorIs(t, err, ErrInvalidRefreshToken)
}

func TestAuthService_RefreshTokens_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	userID := uuid.New()
	refreshToken := "valid-refresh-token"

	storedToken := &entity.RefreshToken{
		ID:        1,
		UserID:    userID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	tokenRepo.On("GetRefreshToken", ctx, refreshToken).Return(storedToken, nil)
	tokenRepo.On("DeleteRefreshToken", ctx, refreshToken).Return(nil)
	userRepo.On("GetByID", ctx, userID).Return(nil, pgx.ErrNoRows)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	tokenPair, err := service.RefreshTokens(ctx, refreshToken)

	// Assert
	assert.Nil(t, tokenPair)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ==================== GetCurrentUser Tests ====================

func TestAuthService_GetCurrentUser_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()
	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	roleRepo.On("GetByID", ctx, user.RoleID).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, user.RoleID).Return(permissions, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	result, err := service.GetCurrentUser(ctx, user.ID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.ID, result.ID)
	assert.Equal(t, user.Email, result.Email)
	assert.Equal(t, role.Name, result.Role.Name)
	assert.Len(t, result.Permissions, 2)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
}

func TestAuthService_GetCurrentUser_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	userID := uuid.New()
	userRepo.On("GetByID", ctx, userID).Return(nil, pgx.ErrNoRows)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	result, err := service.GetCurrentUser(ctx, userID)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ==================== Logout Tests ====================

func TestAuthService_Logout_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()

	// Генерируем валидный access токен
	accessToken, _ := jwtManager.GenerateAccessToken(user.ID, user.Email, user.RoleID, "user", []string{"product.read"})

	tokenRepo.On("AddToBlacklist", ctx, accessToken, mock.AnythingOfType("time.Time")).Return(nil)
	tokenRepo.On("DeleteUserRefreshTokens", ctx, user.ID).Return(nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	err := service.Logout(ctx, user.ID, accessToken)

	// Assert
	require.NoError(t, err)
	tokenRepo.AssertExpectations(t)
}

func TestAuthService_Logout_InvalidToken_StillSucceeds(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	userID := uuid.New()

	// При невалидном токене Logout не должен падать
	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	err := service.Logout(ctx, userID, "invalid-token")

	// Assert
	require.NoError(t, err) // Не должно быть ошибки даже с невалидным токеном
}

// ==================== ValidateToken Tests ====================

func TestAuthService_ValidateToken_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()
	permissions := []string{"product.read", "order.create"}

	// Генерируем валидный токен
	accessToken, _ := jwtManager.GenerateAccessToken(user.ID, user.Email, user.RoleID, "user", permissions)

	tokenRepo.On("IsBlacklisted", ctx, accessToken).Return(false, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	claims, err := service.ValidateToken(ctx, accessToken)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, user.RoleID, claims.RoleID)
	assert.Equal(t, "user", claims.RoleName)
	assert.ElementsMatch(t, permissions, claims.Permissions)

	tokenRepo.AssertExpectations(t)
}

func TestAuthService_ValidateToken_Blacklisted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	user := newTestUser()
	accessToken, _ := jwtManager.GenerateAccessToken(user.ID, user.Email, user.RoleID, "user", []string{})

	tokenRepo.On("IsBlacklisted", ctx, accessToken).Return(true, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	claims, err := service.ValidateToken(ctx, accessToken)

	// Assert
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, util.ErrInvalidToken)
}

func TestAuthService_ValidateToken_InvalidToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := newTestJWTManager()

	tokenRepo.On("IsBlacklisted", ctx, "invalid-token").Return(false, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	claims, err := service.ValidateToken(ctx, "invalid-token")

	// Assert
	assert.Nil(t, claims)
	assert.Error(t, err)
}

func TestAuthService_ValidateToken_ExpiredToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	// JWT Manager с очень коротким временем жизни токена
	jwtManager := util.NewJWTManager("test-secret", 1*time.Nanosecond, 1*time.Hour)

	user := newTestUser()
	accessToken, _ := jwtManager.GenerateAccessToken(user.ID, user.Email, user.RoleID, "user", []string{})

	// Ждём чтобы токен истёк
	time.Sleep(10 * time.Millisecond)

	tokenRepo.On("IsBlacklisted", ctx, accessToken).Return(false, nil)

	service := NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Act
	claims, err := service.ValidateToken(ctx, accessToken)

	// Assert
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, util.ErrExpiredToken)
}
