package util

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_GenerateAccessToken_Success(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()
	email := "test@example.com"
	roleID := 1
	roleName := "admin"
	permissions := []string{"product.create", "product.read", "order.create"}

	// Act
	token, err := jwtManager.GenerateAccessToken(userID, email, roleID, roleName, permissions)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Проверяем что токен можно распарсить
	claims, err := jwtManager.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, roleID, claims.RoleID)
	assert.Equal(t, roleName, claims.RoleName)
	assert.ElementsMatch(t, permissions, claims.Permissions)
}

func TestJWTManager_GenerateRefreshToken_Success(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	// Act
	token1, err1 := jwtManager.GenerateRefreshToken()
	token2, err2 := jwtManager.GenerateRefreshToken()

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2) // Токены должны быть уникальными
}

func TestJWTManager_ValidateToken_Success(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()
	email := "test@example.com"
	roleID := 2
	roleName := "user"
	permissions := []string{"product.read"}

	token, _ := jwtManager.GenerateAccessToken(userID, email, roleID, roleName, permissions)

	// Act
	claims, err := jwtManager.ValidateToken(token)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, roleID, claims.RoleID)
	assert.Equal(t, roleName, claims.RoleName)
	assert.Equal(t, permissions, claims.Permissions)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestJWTManager_ValidateToken_InvalidToken(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	// Act
	claims, err := jwtManager.ValidateToken("invalid-token")

	// Assert
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWTManager_ValidateToken_WrongSecret(t *testing.T) {
	// Arrange
	jwtManager1 := NewJWTManager("secret-key-1", 15*time.Minute, 7*24*time.Hour)
	jwtManager2 := NewJWTManager("secret-key-2", 15*time.Minute, 7*24*time.Hour)

	userID := uuid.New()
	token, _ := jwtManager1.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	// Act
	claims, err := jwtManager2.ValidateToken(token)

	// Assert
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWTManager_ValidateToken_ExpiredToken(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 1*time.Nanosecond, 7*24*time.Hour)
	userID := uuid.New()

	token, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	// Ждём пока токен истечёт
	time.Sleep(10 * time.Millisecond)

	// Act
	claims, err := jwtManager.ValidateToken(token)

	// Assert
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestJWTManager_ValidateToken_EmptyToken(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	// Act
	claims, err := jwtManager.ValidateToken("")

	// Assert
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWTManager_ValidateToken_MalformedToken(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	testCases := []struct {
		name  string
		token string
	}{
		{"single part", "onlyonepart"},
		{"two parts", "header.payload"},
		{"invalid base64", "invalid.base64.token"},
		{"modified signature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.wrongsignature"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			claims, err := jwtManager.ValidateToken(tc.token)

			// Assert
			assert.Nil(t, claims)
			assert.Error(t, err)
		})
	}
}

func TestJWTManager_GetAccessTokenDuration(t *testing.T) {
	// Arrange
	expectedDuration := 30 * time.Minute
	jwtManager := NewJWTManager("secret", expectedDuration, 7*24*time.Hour)

	// Act
	duration := jwtManager.GetAccessTokenDuration()

	// Assert
	assert.Equal(t, expectedDuration, duration)
}

func TestJWTManager_GetRefreshTokenDuration(t *testing.T) {
	// Arrange
	expectedDuration := 14 * 24 * time.Hour
	jwtManager := NewJWTManager("secret", 15*time.Minute, expectedDuration)

	// Act
	duration := jwtManager.GetRefreshTokenDuration()

	// Assert
	assert.Equal(t, expectedDuration, duration)
}

func TestJWTManager_TokenContainsCorrectExpiration(t *testing.T) {
	// Arrange
	accessDuration := 15 * time.Minute
	jwtManager := NewJWTManager("test-secret-key", accessDuration, 7*24*time.Hour)
	userID := uuid.New()

	beforeGeneration := time.Now()
	token, _ := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})
	afterGeneration := time.Now()

	// Act
	claims, err := jwtManager.ValidateToken(token)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, claims.ExpiresAt)

	expectedExpirationMin := beforeGeneration.Add(accessDuration)
	expectedExpirationMax := afterGeneration.Add(accessDuration)

	assert.True(t, claims.ExpiresAt.Time.After(expectedExpirationMin) || claims.ExpiresAt.Time.Equal(expectedExpirationMin))
	assert.True(t, claims.ExpiresAt.Time.Before(expectedExpirationMax) || claims.ExpiresAt.Time.Equal(expectedExpirationMax))
}

func TestJWTManager_EmptyPermissions(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	// Act
	token, err := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", []string{})

	// Assert
	require.NoError(t, err)

	claims, err := jwtManager.ValidateToken(token)
	require.NoError(t, err)
	assert.Empty(t, claims.Permissions)
}

func TestJWTManager_NilPermissions(t *testing.T) {
	// Arrange
	jwtManager := NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	// Act
	token, err := jwtManager.GenerateAccessToken(userID, "test@example.com", 1, "user", nil)

	// Assert
	require.NoError(t, err)

	claims, err := jwtManager.ValidateToken(token)
	require.NoError(t, err)
	assert.Nil(t, claims.Permissions)
}
