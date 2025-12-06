package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"augustberries/auth-service/internal/app/auth/repository/mocks"
	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

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

// ==================== Authenticate Tests ====================

func TestAuthMiddleware_Authenticate_Success(t *testing.T) {
	// Arrange
	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	permissions := []string{"product.read", "order.create"}
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", permissions)

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(false, nil)

	router := gin.New()
	router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
		gotUserID, _ := c.Get("user_id")
		assert.Equal(t, userID, gotUserID)
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())

	tokenRepo.AssertExpectations(t)
}

func TestAuthMiddleware_Authenticate_NoAuthHeader(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Authorization header required", response["message"])
}

func TestAuthMiddleware_Authenticate_InvalidFormat(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

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
			router := gin.New()
			router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
				t.Error("Handler should not be called")
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tc.authHeader)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestAuthMiddleware_Authenticate_InvalidToken(t *testing.T) {
	// Arrange
	middleware, tokenRepo, _ := newTestAuthMiddleware()

	tokenRepo.On("IsBlacklisted", mock.Anything, "invalid-token").Return(false, nil)

	router := gin.New()
	router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Invalid token", response["message"])
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

	router := gin.New()
	router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Token has expired", response["message"])
}

func TestAuthMiddleware_Authenticate_BlacklistedToken(t *testing.T) {
	// Arrange
	middleware, tokenRepo, jwtManager := newTestAuthMiddleware()

	userID := uuid.New()
	accessToken, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	tokenRepo.On("IsBlacklisted", mock.Anything, accessToken).Return(true, nil)

	router := gin.New()
	router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

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

	router := gin.New()
	router.GET("/protected", middleware.Authenticate(), func(c *gin.Context) {
		gotUserID, _ := c.Get("user_id")
		gotEmail, _ := c.Get("email")
		gotRoleID, _ := c.Get("role_id")
		gotRoleName, _ := c.Get("role_name")
		gotPermissions, _ := c.Get("permissions")

		assert.Equal(t, userID, gotUserID)
		assert.Equal(t, email, gotEmail)
		assert.Equal(t, roleID, gotRoleID)
		assert.Equal(t, roleName, gotRoleName)
		assert.ElementsMatch(t, permissions, gotPermissions)

		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ==================== RequireRole Tests ====================

func TestAuthMiddleware_RequireRole_Success(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.GET("/admin", func(c *gin.Context) {
		c.Set("role_name", "admin")
		c.Next()
	}, middleware.RequireRole("admin", "manager"), func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_RequireRole_MatchSecondRole(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.GET("/admin", func(c *gin.Context) {
		c.Set("role_name", "manager")
		c.Next()
	}, middleware.RequireRole("admin", "manager"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_RequireRole_Forbidden(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.GET("/admin", func(c *gin.Context) {
		c.Set("role_name", "user")
		c.Next()
	}, middleware.RequireRole("admin"), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response map[string]string
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Insufficient permissions", response["message"])
}

func TestAuthMiddleware_RequireRole_NoRoleInContext(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.GET("/admin", middleware.RequireRole("admin"), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ==================== RequirePermission Tests ====================

func TestAuthMiddleware_RequirePermission_Success(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.POST("/products", func(c *gin.Context) {
		c.Set("permissions", []string{"product.read", "product.create", "order.read"})
		c.Next()
	}, middleware.RequirePermission("product.create"), func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_RequirePermission_Forbidden(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.DELETE("/products/1", func(c *gin.Context) {
		c.Set("permissions", []string{"product.read", "product.create"})
		c.Next()
	}, middleware.RequirePermission("product.delete"), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodDelete, "/products/1", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response map[string]string
	json.Unmarshal(rec.Body.Bytes(), &response)
	assert.Equal(t, "Insufficient permissions", response["message"])
}

func TestAuthMiddleware_RequirePermission_NoPermissionsInContext(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.POST("/products", middleware.RequirePermission("product.create"), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_RequirePermission_EmptyPermissions(t *testing.T) {
	// Arrange
	middleware, _, _ := newTestAuthMiddleware()

	router := gin.New()
	router.POST("/products", func(c *gin.Context) {
		c.Set("permissions", []string{})
		c.Next()
	}, middleware.RequirePermission("product.create"), func(c *gin.Context) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

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

	router := gin.New()
	router.POST("/admin/products",
		middleware.Authenticate(),
		middleware.RequireRole("admin"),
		middleware.RequirePermission("product.create"),
		func(c *gin.Context) {
			c.String(http.StatusOK, "Success")
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

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

	router := gin.New()
	router.POST("/admin/products",
		middleware.Authenticate(),
		middleware.RequireRole("admin"),
		middleware.RequirePermission("product.create"),
		func(c *gin.Context) {
			t.Error("Handler should not be called")
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
