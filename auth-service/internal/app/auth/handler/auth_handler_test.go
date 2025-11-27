package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository/mocks"
	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Хелперы для создания тестового окружения

func newTestAuthHandler() (*AuthHandler, *mocks.MockUserRepository, *mocks.MockRoleRepository, *mocks.MockTokenRepository, *util.JWTManager) {
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := util.NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	authService := service.NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)
	handler := NewAuthHandler(authService)

	return handler, userRepo, roleRepo, tokenRepo, jwtManager
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
	}
}

// ==================== Register Handler Tests ====================

func TestAuthHandler_Register_Success(t *testing.T) {
	// Arrange
	handler, userRepo, roleRepo, tokenRepo, _ := newTestAuthHandler()

	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByEmail", mock.Anything, "newuser@example.com").Return(nil, pgx.ErrNoRows)
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.User")).Return(nil)
	roleRepo.On("GetByName", mock.Anything, "user").Return(role, nil)
	roleRepo.On("GetByID", mock.Anything, 1).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", mock.Anything, 1).Return(permissions, nil)
	tokenRepo.On("SaveRefreshToken", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	reqBody := entity.RegisterRequest{
		Email:    "newuser@example.com",
		Password: "password123",
		Name:     "New User",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.Register(rec, req)

	// Assert
	assert.Equal(t, http.StatusCreated, rec.Code)

	var response entity.AuthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "newuser@example.com", response.User.Email)
	assert.NotEmpty(t, response.Tokens.AccessToken)
	assert.NotEmpty(t, response.Tokens.RefreshToken)
}

func TestAuthHandler_Register_InvalidBody(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.Register(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Invalid request body", response.Message)
}

func TestAuthHandler_Register_ValidationErrors(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := newTestAuthHandler()

	testCases := []struct {
		name     string
		request  entity.RegisterRequest
		expected string
	}{
		{
			name:     "Empty email",
			request:  entity.RegisterRequest{Email: "", Password: "password123", Name: "Test"},
			expected: "Email is required",
		},
		{
			name:     "Invalid email",
			request:  entity.RegisterRequest{Email: "not-an-email", Password: "password123", Name: "Test"},
			expected: "Email must be a valid email",
		},
		{
			name:     "Short password",
			request:  entity.RegisterRequest{Email: "test@test.com", Password: "short", Name: "Test"},
			expected: "Password must be at least 8 characters",
		},
		{
			name:     "Empty name",
			request:  entity.RegisterRequest{Email: "test@test.com", Password: "password123", Name: ""},
			expected: "Name is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)

			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)

			var response entity.ErrorResponse
			json.Unmarshal(rec.Body.Bytes(), &response)
			assert.Contains(t, response.Message, tc.expected)
		})
	}
}

func TestAuthHandler_Register_UserAlreadyExists(t *testing.T) {
	// Arrange
	handler, userRepo, _, _, _ := newTestAuthHandler()

	existingUser := &entity.User{
		ID:    uuid.New(),
		Email: "existing@example.com",
	}
	userRepo.On("GetByEmail", mock.Anything, "existing@example.com").Return(existingUser, nil)

	reqBody := entity.RegisterRequest{
		Email:    "existing@example.com",
		Password: "password123",
		Name:     "Test User",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.Register(rec, req)

	// Assert
	assert.Equal(t, http.StatusConflict, rec.Code)
}

// ==================== Login Handler Tests ====================

func TestAuthHandler_Login_Success(t *testing.T) {
	// Arrange
	handler, userRepo, roleRepo, tokenRepo, _ := newTestAuthHandler()

	passwordHash, _ := util.HashPassword("password123")
	user := &entity.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: passwordHash,
		Name:         "Test User",
		RoleID:       1,
		CreatedAt:    time.Now(),
	}
	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
	roleRepo.On("GetByID", mock.Anything, 1).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", mock.Anything, 1).Return(permissions, nil)
	tokenRepo.On("SaveRefreshToken", mock.Anything, user.ID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	reqBody := entity.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.Login(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var response entity.AuthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test@example.com", response.User.Email)
	assert.NotEmpty(t, response.Tokens.AccessToken)
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	// Arrange
	handler, userRepo, _, _, _ := newTestAuthHandler()

	passwordHash, _ := util.HashPassword("correctpassword")
	user := &entity.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: passwordHash,
	}

	userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)

	reqBody := entity.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.Login(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	// Arrange
	handler, userRepo, _, _, _ := newTestAuthHandler()

	userRepo.On("GetByEmail", mock.Anything, "notfound@example.com").Return(nil, pgx.ErrNoRows)

	reqBody := entity.LoginRequest{
		Email:    "notfound@example.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.Login(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ==================== RefreshToken Handler Tests ====================

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	// Arrange
	handler, userRepo, roleRepo, tokenRepo, _ := newTestAuthHandler()

	userID := uuid.New()
	refreshToken := "valid-refresh-token"

	storedToken := &entity.RefreshToken{
		ID:        1,
		UserID:    userID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	user := &entity.User{
		ID:     userID,
		Email:  "test@example.com",
		Name:   "Test",
		RoleID: 1,
	}
	role := newTestRole()
	permissions := newTestPermissions()

	tokenRepo.On("GetRefreshToken", mock.Anything, refreshToken).Return(storedToken, nil)
	tokenRepo.On("DeleteRefreshToken", mock.Anything, refreshToken).Return(nil)
	userRepo.On("GetByID", mock.Anything, userID).Return(user, nil)
	roleRepo.On("GetByID", mock.Anything, 1).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", mock.Anything, 1).Return(permissions, nil)
	tokenRepo.On("SaveRefreshToken", mock.Anything, userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

	reqBody := entity.RefreshRequest{
		RefreshToken: refreshToken,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.RefreshToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var response entity.TokenPair
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
}

func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	// Arrange
	handler, _, _, tokenRepo, _ := newTestAuthHandler()

	tokenRepo.On("GetRefreshToken", mock.Anything, "invalid-token").Return(nil, pgx.ErrNoRows)

	reqBody := entity.RefreshRequest{
		RefreshToken: "invalid-token",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	handler.RefreshToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ==================== GetMe Handler Tests ====================

func TestAuthHandler_GetMe_Success(t *testing.T) {
	// Arrange
	handler, userRepo, roleRepo, _, _ := newTestAuthHandler()

	userID := uuid.New()
	user := &entity.User{
		ID:     userID,
		Email:  "test@example.com",
		Name:   "Test User",
		RoleID: 1,
	}
	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByID", mock.Anything, userID).Return(user, nil)
	roleRepo.On("GetByID", mock.Anything, 1).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", mock.Anything, 1).Return(permissions, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	// Добавляем userID в контекст (как делает middleware)
	ctx := context.WithValue(req.Context(), "user_id", userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.GetMe(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var response entity.UserWithRole
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "user", response.Role.Name)
}

func TestAuthHandler_GetMe_Unauthorized(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	// Не добавляем userID в контекст
	rec := httptest.NewRecorder()

	// Act
	handler.GetMe(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ==================== Logout Handler Tests ====================

func TestAuthHandler_Logout_Success(t *testing.T) {
	// Arrange
	handler, _, _, tokenRepo, jwtManager := newTestAuthHandler()

	userID := uuid.New()
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	tokenRepo.On("AddToBlacklist", mock.Anything, accessToken, mock.AnythingOfType("time.Time")).Return(nil)
	tokenRepo.On("DeleteUserRefreshTokens", mock.Anything, userID).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	ctx := context.WithValue(req.Context(), "user_id", userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.Logout(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthHandler_Logout_NoAuthHeader(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := newTestAuthHandler()

	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	ctx := context.WithValue(req.Context(), "user_id", userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.Logout(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_Logout_InvalidAuthFormat(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := newTestAuthHandler()

	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "InvalidFormat token")
	ctx := context.WithValue(req.Context(), "user_id", userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.Logout(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// ==================== ValidateToken Handler Tests ====================

func TestAuthHandler_ValidateToken_Success(t *testing.T) {
	// Arrange
	handler, _, _, tokenRepo, jwtManager := newTestAuthHandler()

	userID := uuid.New()
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{"product.read"})

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ValidateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var claims util.JWTClaims
	err := json.Unmarshal(rec.Body.Bytes(), &claims)
	require.NoError(t, err)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
}

func TestAuthHandler_ValidateToken_NoAuthHeader(t *testing.T) {
	// Arrange
	handler, _, _, _, _ := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/auth/validate", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.ValidateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_ValidateToken_BlacklistedToken(t *testing.T) {
	// Arrange
	handler, _, _, tokenRepo, jwtManager := newTestAuthHandler()

	userID := uuid.New()
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(true, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ValidateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_ValidateToken_ExpiredToken(t *testing.T) {
	// Arrange
	handler, _, _, tokenRepo, _ := newTestAuthHandler()

	// Создаём JWT manager с очень коротким временем жизни
	shortJWTManager := util.NewJWTManager("test-secret-key", 1*time.Nanosecond, 7*24*time.Hour)
	userID := uuid.New()
	accessToken, _ := shortJWTManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	time.Sleep(10 * time.Millisecond) // Ждём пока токен истечёт

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ValidateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ==================== Helper Function Tests ====================

func TestFormatValidationErrors(t *testing.T) {
	// Этот тест проверяет форматирование ошибок валидации
	// Создаём невалидный запрос и проверяем формат ошибки

	handler, _, _, _, _ := newTestAuthHandler()

	reqBody := entity.RegisterRequest{
		Email:    "",        // required
		Password: "short",   // min=8
		Name:     "",        // required
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)

	// Проверяем что все ошибки валидации присутствуют в сообщении
	assert.Contains(t, response.Message, "Email")
	assert.Contains(t, response.Message, "Password")
	assert.Contains(t, response.Message, "Name")
}
