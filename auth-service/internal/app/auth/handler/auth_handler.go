package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"
	"augustberries/pkg/metrics"
)

// AuthHandler обрабатывает HTTP запросы для аутентификации
type AuthHandler struct {
	authService *service.AuthService
	validator   *validator.Validate
}

// NewAuthHandler создает новый обработчик аутентификации
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validator:   validator.New(),
	}
}

// Register обрабатывает POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req entity.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	// Валидация с помощью validator
	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": formatValidationErrors(validationErrors),
		})
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Conflict",
				"message": "User with this email already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to register user",
		})
		return
	}

	// Записываем метрику успешной регистрации
	metrics.AuthRegistrations.Inc()

	c.JSON(http.StatusCreated, resp)
}

// Login обрабатывает POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req entity.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": formatValidationErrors(validationErrors),
		})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			// Записываем неудачную попытку входа
			metrics.AuthLogins.WithLabelValues("failed").Inc()
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid email or password",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to login",
		})
		return
	}

	// Записываем успешный вход
	metrics.AuthLogins.WithLabelValues("success").Inc()
	// Также записываем выдачу токенов
	metrics.AuthTokensIssued.WithLabelValues("access").Inc()
	metrics.AuthTokensIssued.WithLabelValues("refresh").Inc()

	c.JSON(http.StatusOK, resp)
}

// RefreshToken обрабатывает POST /auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req entity.RefreshRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": formatValidationErrors(validationErrors),
		})
		return
	}

	tokens, err := h.authService.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRefreshToken) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid or expired refresh token",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// GetMe обрабатывает GET /auth/me
func (h *AuthHandler) GetMe(c *gin.Context) {
	// Получаем userID из контекста (устанавливается middleware)
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Unauthorized",
		})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Unauthorized",
		})
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to get user info",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Logout обрабатывает POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Получаем userID из контекста
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Unauthorized",
		})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Unauthorized",
		})
		return
	}

	// Извлекаем access токен из заголовка
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Authorization header required",
		})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid authorization header format",
		})
		return
	}

	if err := h.authService.Logout(c.Request.Context(), userID, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to logout",
		})
		return
	}

	c.JSON(http.StatusOK, entity.SuccessResponse{
		Message: "Successfully logged out",
	})
}

// ValidateToken обрабатывает POST /auth/validate (для других микросервисов)
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	// Извлекаем токен из заголовка
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Authorization header required",
		})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid authorization header format",
		})
		return
	}

	claims, err := h.authService.ValidateToken(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, util.ErrExpiredToken) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Token has expired",
			})
			return
		}
		if errors.Is(err, util.ErrInvalidToken) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid token",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to validate token",
		})
		return
	}

	c.JSON(http.StatusOK, claims)
}

// formatValidationErrors форматирует ошибки валидации в читаемый формат
func formatValidationErrors(errs validator.ValidationErrors) string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		switch err.Tag() {
		case "required":
			messages = append(messages, err.Field()+" is required")
		case "email":
			messages = append(messages, err.Field()+" must be a valid email")
		case "min":
			messages = append(messages, err.Field()+" must be at least "+err.Param()+" characters")
		default:
			messages = append(messages, err.Field()+" is invalid")
		}
	}
	return strings.Join(messages, ", ")
}
