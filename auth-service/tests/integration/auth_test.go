//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/handler"
	"augustberries/auth-service/internal/app/auth/repository"
	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AuthIntegrationTestSuite содержит интеграционные тесты для auth-service
// Требует запущенные PostgreSQL и Redis
type AuthIntegrationTestSuite struct {
	suite.Suite
	db          *pgxpool.Pool
	redisClient *redis.Client
	router      http.Handler
	jwtManager  *util.JWTManager
}

// SetupSuite выполняется один раз перед всеми тестами
func (s *AuthIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()

	// Подключение к PostgreSQL (тестовая БД)
	// Эти значения должны соответствовать docker-compose.test.yml
	dbURL := "postgres://postgres:postgres@localhost:5432/auth_service_test?sslmode=disable"
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(s.T(), err, "Failed to connect to PostgreSQL")
	s.db = pool

	// Подключение к Redis
	s.redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "redis_password",
		DB:       15, // Используем отдельную БД для тестов
	})
	err = s.redisClient.Ping(ctx).Err()
	require.NoError(s.T(), err, "Failed to connect to Redis")

	// Инициализируем JWT Manager
	s.jwtManager = util.NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	// Инициализируем репозитории
	userRepo := repository.NewUserRepository(s.db)
	roleRepo := repository.NewRoleRepository(s.db)
	tokenRepo := repository.NewRedisTokenRepository(s.redisClient)

	// Инициализируем сервис
	authService := service.NewAuthService(userRepo, roleRepo, tokenRepo, s.jwtManager)

	// Инициализируем handlers
	authHandler := handler.NewAuthHandler(authService)
	authMiddleware := handler.NewAuthMiddleware(authService)

	// Настраиваем router
	s.router = handler.SetupRoutes(authHandler, authMiddleware)

	// Применяем миграции и seed данные
	s.setupDatabase(ctx)
}

// TearDownSuite выполняется один раз после всех тестов
func (s *AuthIntegrationTestSuite) TearDownSuite() {
	ctx := context.Background()

	// Очищаем тестовые данные
	s.cleanupDatabase(ctx)

	// Закрываем соединения
	if s.db != nil {
		s.db.Close()
	}
	if s.redisClient != nil {
		s.redisClient.Close()
	}
}

// SetupTest выполняется перед каждым тестом
func (s *AuthIntegrationTestSuite) SetupTest() {
	ctx := context.Background()
	// Очищаем данные пользователей перед каждым тестом
	s.db.Exec(ctx, "DELETE FROM users")
	s.redisClient.FlushDB(ctx)
}

func (s *AuthIntegrationTestSuite) setupDatabase(ctx context.Context) {
	// Создаём таблицы если их нет
	queries := []string{
		`CREATE TABLE IF NOT EXISTS roles (
			id SERIAL PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			description TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			code TEXT UNIQUE NOT NULL,
			description TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS roles_permissions (
			role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
			permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
			PRIMARY KEY (role_id, permission_id)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			role_id INTEGER NOT NULL REFERENCES roles(id),
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		// Seed roles
		`INSERT INTO roles (name, description) VALUES 
			('user', 'Regular user'),
			('admin', 'Administrator')
		ON CONFLICT (name) DO NOTHING`,
		// Seed permissions
		`INSERT INTO permissions (code, description) VALUES 
			('product.read', 'Read products'),
			('order.create', 'Create orders')
		ON CONFLICT (code) DO NOTHING`,
	}

	for _, query := range queries {
		_, err := s.db.Exec(ctx, query)
		require.NoError(s.T(), err)
	}
}

func (s *AuthIntegrationTestSuite) cleanupDatabase(ctx context.Context) {
	s.db.Exec(ctx, "DELETE FROM users")
}

// ==================== Test Cases ====================

func (s *AuthIntegrationTestSuite) TestRegister_Success() {
	// Arrange
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
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusCreated, rec.Code)

	var response entity.AuthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "newuser@example.com", response.User.Email)
	assert.Equal(s.T(), "New User", response.User.Name)
	assert.NotEmpty(s.T(), response.Tokens.AccessToken)
	assert.NotEmpty(s.T(), response.Tokens.RefreshToken)
}

func (s *AuthIntegrationTestSuite) TestRegister_DuplicateEmail() {
	// Arrange - сначала регистрируем пользователя
	firstReq := entity.RegisterRequest{
		Email:    "duplicate@example.com",
		Password: "password123",
		Name:     "First User",
	}
	body, _ := json.Marshal(firstReq)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	require.Equal(s.T(), http.StatusCreated, rec.Code)

	// Act - пытаемся зарегистрировать с тем же email
	secondReq := entity.RegisterRequest{
		Email:    "duplicate@example.com",
		Password: "password456",
		Name:     "Second User",
	}
	body, _ = json.Marshal(secondReq)

	req = httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusConflict, rec.Code)
}

func (s *AuthIntegrationTestSuite) TestLogin_Success() {
	// Arrange - регистрируем пользователя
	registerReq := entity.RegisterRequest{
		Email:    "login@example.com",
		Password: "password123",
		Name:     "Login User",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	require.Equal(s.T(), http.StatusCreated, rec.Code)

	// Act - логинимся
	loginReq := entity.LoginRequest{
		Email:    "login@example.com",
		Password: "password123",
	}
	body, _ = json.Marshal(loginReq)

	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var response entity.AuthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "login@example.com", response.User.Email)
	assert.NotEmpty(s.T(), response.Tokens.AccessToken)
}

func (s *AuthIntegrationTestSuite) TestLogin_WrongPassword() {
	// Arrange - регистрируем пользователя
	registerReq := entity.RegisterRequest{
		Email:    "wrongpass@example.com",
		Password: "correctpassword",
		Name:     "User",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	require.Equal(s.T(), http.StatusCreated, rec.Code)

	// Act - пытаемся залогиниться с неправильным паролем
	loginReq := entity.LoginRequest{
		Email:    "wrongpass@example.com",
		Password: "wrongpassword",
	}
	body, _ = json.Marshal(loginReq)

	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusUnauthorized, rec.Code)
}

func (s *AuthIntegrationTestSuite) TestGetMe_Success() {
	// Arrange - регистрируемся и получаем токен
	registerReq := entity.RegisterRequest{
		Email:    "me@example.com",
		Password: "password123",
		Name:     "Me User",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	require.Equal(s.T(), http.StatusCreated, rec.Code)

	var authResponse entity.AuthResponse
	json.Unmarshal(rec.Body.Bytes(), &authResponse)

	// Act - запрашиваем /auth/me
	req = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authResponse.Tokens.AccessToken))
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var userResponse entity.UserWithRole
	err := json.Unmarshal(rec.Body.Bytes(), &userResponse)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "me@example.com", userResponse.Email)
	assert.Equal(s.T(), "Me User", userResponse.Name)
}

func (s *AuthIntegrationTestSuite) TestGetMe_Unauthorized() {
	// Act - запрашиваем без токена
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusUnauthorized, rec.Code)
}

func (s *AuthIntegrationTestSuite) TestRefreshToken_Success() {
	// Arrange - регистрируемся
	registerReq := entity.RegisterRequest{
		Email:    "refresh@example.com",
		Password: "password123",
		Name:     "Refresh User",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	require.Equal(s.T(), http.StatusCreated, rec.Code)

	var authResponse entity.AuthResponse
	json.Unmarshal(rec.Body.Bytes(), &authResponse)

	// Act - обновляем токен
	refreshReq := entity.RefreshRequest{
		RefreshToken: authResponse.Tokens.RefreshToken,
	}
	body, _ = json.Marshal(refreshReq)

	req = httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	var tokenPair entity.TokenPair
	err := json.Unmarshal(rec.Body.Bytes(), &tokenPair)
	require.NoError(s.T(), err)

	assert.NotEmpty(s.T(), tokenPair.AccessToken)
	assert.NotEmpty(s.T(), tokenPair.RefreshToken)
	assert.NotEqual(s.T(), authResponse.Tokens.RefreshToken, tokenPair.RefreshToken) // Новый refresh token
}

func (s *AuthIntegrationTestSuite) TestLogout_Success() {
	// Arrange - регистрируемся
	registerReq := entity.RegisterRequest{
		Email:    "logout@example.com",
		Password: "password123",
		Name:     "Logout User",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	require.Equal(s.T(), http.StatusCreated, rec.Code)

	var authResponse entity.AuthResponse
	json.Unmarshal(rec.Body.Bytes(), &authResponse)

	// Act - выходим
	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authResponse.Tokens.AccessToken))
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)

	// Проверяем что токен больше не работает для /auth/me
	req = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authResponse.Tokens.AccessToken))
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	assert.Equal(s.T(), http.StatusUnauthorized, rec.Code)
}

func (s *AuthIntegrationTestSuite) TestHealthCheck() {
	// Act
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(s.T(), http.StatusOK, rec.Code)
}

// Запуск test suite
func TestAuthIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthIntegrationTestSuite))
}
