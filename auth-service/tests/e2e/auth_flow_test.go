//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// BaseURL - адрес запущенного auth-service
	// Для E2E тестов сервис должен быть запущен через docker-compose
	BaseURL = "http://localhost:8080"
)

// TestFullAuthenticationFlow тестирует полный цикл аутентификации:
// 1. Регистрация нового пользователя
// 2. Логин
// 3. Получение информации о себе
// 4. Обновление токена
// 5. Logout
// 6. Проверка что токен больше не работает
func TestFullAuthenticationFlow(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Уникальный email для теста
	email := fmt.Sprintf("e2e-test-%d@example.com", time.Now().UnixNano())
	password := "securepassword123"
	name := "E2E Test User"

	// ==================== Step 1: Register ====================
	t.Log("Step 1: Registering new user")

	registerReq := entity.RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}
	registerBody, _ := json.Marshal(registerReq)

	resp, err := client.Post(
		BaseURL+"/auth/register",
		"application/json",
		bytes.NewBuffer(registerBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Registration should succeed")

	var registerResponse entity.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&registerResponse)
	require.NoError(t, err)

	assert.Equal(t, email, registerResponse.User.Email)
	assert.Equal(t, name, registerResponse.User.Name)
	assert.NotEmpty(t, registerResponse.Tokens.AccessToken)
	assert.NotEmpty(t, registerResponse.Tokens.RefreshToken)

	accessToken := registerResponse.Tokens.AccessToken
	refreshToken := registerResponse.Tokens.RefreshToken

	t.Logf("Registered user: %s", email)

	// ==================== Step 2: Login ====================
	t.Log("Step 2: Logging in")

	loginReq := entity.LoginRequest{
		Email:    email,
		Password: password,
	}
	loginBody, _ := json.Marshal(loginReq)

	resp, err = client.Post(
		BaseURL+"/auth/login",
		"application/json",
		bytes.NewBuffer(loginBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Login should succeed")

	var loginResponse entity.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&loginResponse)
	require.NoError(t, err)

	assert.Equal(t, email, loginResponse.User.Email)
	assert.NotEmpty(t, loginResponse.Tokens.AccessToken)

	// Обновляем токены
	accessToken = loginResponse.Tokens.AccessToken
	refreshToken = loginResponse.Tokens.RefreshToken

	t.Log("Login successful")

	// ==================== Step 3: Get Me ====================
	t.Log("Step 3: Getting current user info")

	req, _ := http.NewRequest(http.MethodGet, BaseURL+"/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Get me should succeed")

	var userInfo entity.UserWithRole
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	require.NoError(t, err)

	assert.Equal(t, email, userInfo.Email)
	assert.Equal(t, name, userInfo.Name)
	assert.Equal(t, "user", userInfo.Role.Name)

	t.Log("Get me successful")

	// ==================== Step 4: Refresh Token ====================
	t.Log("Step 4: Refreshing token")

	refreshReq := entity.RefreshRequest{
		RefreshToken: refreshToken,
	}
	refreshBody, _ := json.Marshal(refreshReq)

	resp, err = client.Post(
		BaseURL+"/auth/refresh",
		"application/json",
		bytes.NewBuffer(refreshBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Refresh should succeed")

	var newTokens entity.TokenPair
	err = json.NewDecoder(resp.Body).Decode(&newTokens)
	require.NoError(t, err)

	assert.NotEmpty(t, newTokens.AccessToken)
	assert.NotEmpty(t, newTokens.RefreshToken)
	assert.NotEqual(t, refreshToken, newTokens.RefreshToken, "New refresh token should be different")

	// Старый refresh token больше не должен работать
	resp, err = client.Post(
		BaseURL+"/auth/refresh",
		"application/json",
		bytes.NewBuffer(refreshBody), // Используем старый токен
	)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Old refresh token should not work")

	accessToken = newTokens.AccessToken

	t.Log("Token refresh successful")

	// ==================== Step 5: Logout ====================
	t.Log("Step 5: Logging out")

	req, _ = http.NewRequest(http.MethodPost, BaseURL+"/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Logout should succeed")

	t.Log("Logout successful")

	// ==================== Step 6: Verify Token Invalidated ====================
	t.Log("Step 6: Verifying token is invalidated")

	req, _ = http.NewRequest(http.MethodGet, BaseURL+"/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Token should be invalidated after logout")

	t.Log("Token properly invalidated")
	t.Log("Full authentication flow completed successfully!")
}

// TestRegistrationValidation тестирует валидацию при регистрации
func TestRegistrationValidation(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name           string
		request        entity.RegisterRequest
		expectedStatus int
	}{
		{
			name: "Empty email",
			request: entity.RegisterRequest{
				Email:    "",
				Password: "password123",
				Name:     "Test User",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid email format",
			request: entity.RegisterRequest{
				Email:    "not-an-email",
				Password: "password123",
				Name:     "Test User",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Short password",
			request: entity.RegisterRequest{
				Email:    "test@example.com",
				Password: "short",
				Name:     "Test User",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Empty name",
			request: entity.RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)

			resp, err := client.Post(
				BaseURL+"/auth/register",
				"application/json",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// TestLoginValidation тестирует валидацию при логине
func TestLoginValidation(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name           string
		request        entity.LoginRequest
		expectedStatus int
	}{
		{
			name: "Empty email",
			request: entity.LoginRequest{
				Email:    "",
				Password: "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Empty password",
			request: entity.LoginRequest{
				Email:    "test@example.com",
				Password: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Non-existent user",
			request: entity.LoginRequest{
				Email:    "nonexistent@example.com",
				Password: "password123",
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)

			resp, err := client.Post(
				BaseURL+"/auth/login",
				"application/json",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// TestUnauthorizedAccess тестирует защиту эндпоинтов
func TestUnauthorizedAccess(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/auth/me"},
		{http.MethodPost, "/auth/logout"},
	}

	for _, endpoint := range protectedEndpoints {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			req, _ := http.NewRequest(endpoint.method, BaseURL+endpoint.path, nil)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should require authentication")
		})
	}
}

// TestInvalidToken тестирует обработку невалидных токенов
func TestInvalidToken(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	invalidTokens := []string{
		"invalid-token",
		"Bearer invalid",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		"",
	}

	for _, token := range invalidTokens {
		t.Run("Token: "+token, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, BaseURL+"/auth/me", nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// TestHealthCheck проверяет что сервис отвечает
func TestHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(BaseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
