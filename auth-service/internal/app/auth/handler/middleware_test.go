package handler

import (
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Хелпер для создания тестового middleware
func newTestAuthMiddleware() (*AuthMiddleware, *mocks.MockTokenRepository, *util.JWTManager) {
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)
	tokenRepo := new(mocks.MockTokenRepository)
	jwtManager := util.NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	authService := service.NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)
	middleware := NewAuthMiddleware(authService)

	return middleware, tokenRepo, jwtManager
}

// Тестовый handler который проверяет контекст
func testHandler(t *testing.T, expectedUserID uuid.UUID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("user_id").(uuid.UUID)
		assert.True(t, ok, "user_id should be in context")
		assert.Equal(t, expectedUserID, userID)

		email, ok := r.Context().Value("email").(string)
		assert.True(t, ok, "email should be in context")
		assert.NotEmpty(t, email)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// ==================== Authenticate Tests ====================

func TestAuthMiddleware_Authenticate_Success(t *testing.T) {
	// Arrange
	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	permissions := []string{"product.read", "order.create"}
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", permissions)

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	handler := middleware.Authenticate(testHandler(t, userID))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())

	tokenRepo.AssertExpectations(t)
}

func TestAuthMiddleware_Authenticate_NoAuthHeader(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Authorization header required", response.Message)
}

func TestAuthMiddleware_Authenticate_InvalidFormat(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	testCases := []struct {
		name       string
		authHeader string
	}{
		{"No Bearer prefix", "token-without-bearer"},
		{"Wrong prefix", "Basic token"},
		{"Only Bearer", "Bearer"},
		{"Extra parts", "Bearer token extra"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tc.authHeader)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestAuthMiddleware_Authenticate_InvalidToken(t *testing.T) {
	// Arrange
	middleware, tokenRepo, _ := newTestAuthMiddleware()

	tokenRepo.On("IsBlacklisted", mock.Anything, "invalid-token").Return(false, nil)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Invalid token", response.Message)
}

func TestAuthMiddleware_Authenticate_ExpiredToken(t *testing.T) {
	// Arrange
	middleware, tokenRepo, _ := newTestAuthMiddleware()

	// Создаём JWT manager с коротким временем жизни
	shortJWTManager := util.NewJWTManager("test-secret-key", 1*time.Nanosecond, 7*24*time.Hour)
	userID := uuid.New()
	accessToken, _ := shortJWTManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	time.Sleep(10 * time.Millisecond) // Ждём пока токен истечёт

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Token has expired", response.Message)
}

func TestAuthMiddleware_Authenticate_BlacklistedToken(t *testing.T) {
	// Arrange
	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(true, nil)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_Authenticate_ContextValues(t *testing.T) {
	// Arrange
	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	email := "test@example.com"
	roleID := 2
	roleName := "admin"
	permissions := []string{"product.create", "product.delete"}

	accessToken, _ := jwtManager.GenerateAccessToken(userID, email, roleID, roleName, permissions)

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	// Handler который проверяет все значения контекста
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, userID, r.Context().Value("user_id"))
		assert.Equal(t, email, r.Context().Value("email"))
		assert.Equal(t, roleID, r.Context().Value("role_id"))
		assert.Equal(t, roleName, r.Context().Value("role_name"))
		assert.ElementsMatch(t, permissions, r.Context().Value("permissions"))

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ==================== RequireRole Tests ====================

func TestAuthMiddleware_RequireRole_Success(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequireRole("admin", "manager")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), "role_name", "admin")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_RequireRole_MatchSecondRole(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequireRole("admin", "manager")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), "role_name", "manager")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_RequireRole_Forbidden(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), "role_name", "user")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Insufficient permissions", response.Message)
}

func TestAuthMiddleware_RequireRole_NoRoleInContext(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	// Не добавляем role_name в контекст
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ==================== RequirePermission Tests ====================

func TestAuthMiddleware_RequirePermission_Success(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequirePermission("product.create")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	ctx := context.WithValue(req.Context(), "permissions", []string{"product.read", "product.create", "order.read"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_RequirePermission_Forbidden(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequirePermission("product.delete")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodDelete, "/products/1", nil)
	ctx := context.WithValue(req.Context(), "permissions", []string{"product.read", "product.create"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response entity.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Insufficient permissions", response.Message)
}

func TestAuthMiddleware_RequirePermission_NoPermissionsInContext(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequirePermission("product.create")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	// Не добавляем permissions в контекст
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_RequirePermission_EmptyPermissions(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	handler := middleware.RequirePermission("product.create")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	ctx := context.WithValue(req.Context(), "permissions", []string{})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// ==================== Integration Tests (Chained Middleware) ====================

func TestAuthMiddleware_ChainedMiddlewares(t *testing.T) {
	// Тест полной цепочки: Authenticate -> RequireRole -> RequirePermission -> Handler

	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	permissions := []string{"product.create", "product.read"}
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "admin@example.com", 2, "admin", permissions)

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	// Цепочка middleware
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	chain := middleware.Authenticate(
		middleware.RequireRole("admin")(
			middleware.RequirePermission("product.create")(
				finalHandler,
			),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	chain.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Success", rec.Body.String())
}

func TestAuthMiddleware_ChainedMiddlewares_FailsAtRole(t *testing.T) {
	// Тест: авторизация проходит, но роль не подходит

	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	permissions := []string{"product.create"}
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "user@example.com", 1, "user", permissions)

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	chain := middleware.Authenticate(
		middleware.RequireRole("admin")(
			middleware.RequirePermission("product.create")(
				finalHandler,
			),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	chain.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
