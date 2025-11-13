package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims структура claims для JWT токена
type JWTClaims struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	RoleID      int      `json:"role_id"`
	RoleName    string   `json:"role_name"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// AuthMiddleware проверяет JWT токен в запросах для Gin
type AuthMiddleware struct {
	jwtSecret string
}

// NewAuthMiddleware создает новый middleware для аутентификации
func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: jwtSecret,
	}
}

// Authenticate проверяет JWT токен и добавляет данные пользователя в контекст Gin
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Извлекаем токен из заголовка Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Проверяем формат "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Сохраняем полный токен для передачи в Catalog Service
		c.Set("auth_token", tokenString)

		// Парсим и валидируем токен
		token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(m.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Извлекаем claims
		claims, ok := token.Claims.(*JWTClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Парсим UserID из string в UUID
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			c.Abort()
			return
		}

		// Добавляем данные пользователя в контекст Gin
		c.Set("user_id", userID)
		c.Set("email", claims.Email)
		c.Set("role_id", claims.RoleID)
		c.Set("role_name", claims.RoleName)
		c.Set("permissions", claims.Permissions)

		// Передаем управление следующему обработчику
		c.Next()
	}
}

// RequireRole проверяет, что у пользователя есть требуемая роль
func (m *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName, exists := c.Get("role_name")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		roleNameStr, ok := roleName.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid role data"})
			c.Abort()
			return
		}

		// Проверяем, есть ли роль пользователя в списке разрешенных
		hasRole := false
		for _, role := range roles {
			if roleNameStr == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}
