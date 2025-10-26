package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"
)

// AuthMiddleware проверяет JWT токен в запросах
type AuthMiddleware struct {
	authService *service.AuthService
}

// NewAuthMiddleware создает новый middleware для аутентификации
func NewAuthMiddleware(authService *service.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// Authenticate проверяет JWT токен и добавляет данные пользователя в контекст
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем токен из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		// Проверяем формат "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondError(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		token := parts[1]

		// Валидируем токен
		claims, err := m.authService.ValidateToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, util.ErrExpiredToken) {
				respondError(w, http.StatusUnauthorized, "Token has expired")
				return
			}
			if errors.Is(err, util.ErrInvalidToken) {
				respondError(w, http.StatusUnauthorized, "Invalid token")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to validate token")
			return
		}

		// Добавляем данные пользователя в контекст
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		ctx = context.WithValue(ctx, "role_id", claims.RoleID)
		ctx = context.WithValue(ctx, "role_name", claims.RoleName)
		ctx = context.WithValue(ctx, "permissions", claims.Permissions)

		// Передаем управление следующему обработчику
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole проверяет, что у пользователя есть требуемая роль
func (m *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleName, ok := r.Context().Value("role_name").(string)
			if !ok {
				respondError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			// Проверяем, есть ли роль пользователя в списке разрешенных
			hasRole := false
			for _, role := range roles {
				if roleName == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				respondError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission проверяет, что у пользователя есть требуемое разрешение
func (m *AuthMiddleware) RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			permissions, ok := r.Context().Value("permissions").([]string)
			if !ok {
				respondError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			// Проверяем, есть ли требуемое разрешение
			hasPermission := false
			for _, p := range permissions {
				if p == permission {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				respondError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
