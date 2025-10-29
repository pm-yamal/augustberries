package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
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
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req entity.RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация с помощью validator
	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		respondError(w, http.StatusBadRequest, formatValidationErrors(validationErrors))
		return
	}

	resp, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			respondError(w, http.StatusConflict, "User with this email already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// Login обрабатывает POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req entity.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		respondError(w, http.StatusBadRequest, formatValidationErrors(validationErrors))
		return
	}

	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to login")
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// RefreshToken обрабатывает POST /auth/refresh
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req entity.RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		respondError(w, http.StatusBadRequest, formatValidationErrors(validationErrors))
		return
	}

	tokens, err := h.authService.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRefreshToken) {
			respondError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// GetMe обрабатывает GET /auth/me
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста (устанавливается middleware)
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.authService.GetCurrentUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get user info")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// Logout обрабатывает POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Извлекаем access токен из заголовка
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		respondError(w, http.StatusBadRequest, "Authorization header required")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		respondError(w, http.StatusBadRequest, "Invalid authorization header format")
		return
	}

	if err := h.authService.Logout(r.Context(), userID, token); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to logout")
		return
	}

	respondJSON(w, http.StatusOK, entity.SuccessResponse{
		Message: "Successfully logged out",
	})
}

// ValidateToken обрабатывает POST /auth/validate (для других микросервисов)
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	// Извлекаем токен из заголовка
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		respondError(w, http.StatusBadRequest, "Authorization header required")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		respondError(w, http.StatusBadRequest, "Invalid authorization header format")
		return
	}

	claims, err := h.authService.ValidateToken(r.Context(), token)
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

	respondJSON(w, http.StatusOK, claims)
}

// respondJSON отправляет JSON ответ
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError отправляет ответ об ошибке
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, entity.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
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
