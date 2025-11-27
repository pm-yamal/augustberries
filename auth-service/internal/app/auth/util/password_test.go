package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword_Success(t *testing.T) {
	// Arrange
	password := "mysecretpassword123"

	// Act
	hash, err := HashPassword(password)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash) // Хэш не должен совпадать с паролем
}

func TestHashPassword_DifferentHashesForSamePassword(t *testing.T) {
	// Arrange
	password := "mysecretpassword123"

	// Act
	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, hash1, hash2) // bcrypt использует random salt, поэтому хэши разные
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	// Arrange
	password := ""

	// Act
	hash, err := HashPassword(password)

	// Assert
	require.NoError(t, err) // bcrypt позволяет пустые пароли
	assert.NotEmpty(t, hash)
}

func TestHashPassword_LongPassword(t *testing.T) {
	// Arrange - bcrypt обрезает пароли длиннее 72 байт
	password := "a"

	for i := 0; i < 100; i++ {
		password += "a"
	}

	// Act
	hash, err := HashPassword(password)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestCheckPassword_CorrectPassword(t *testing.T) {
	// Arrange
	password := "correctpassword123"
	hash, _ := HashPassword(password)

	// Act
	result := CheckPassword(password, hash)

	// Assert
	assert.True(t, result)
}

func TestCheckPassword_WrongPassword(t *testing.T) {
	// Arrange
	password := "correctpassword123"
	hash, _ := HashPassword(password)

	// Act
	result := CheckPassword("wrongpassword", hash)

	// Assert
	assert.False(t, result)
}

func TestCheckPassword_EmptyPassword(t *testing.T) {
	// Arrange
	password := "somepassword"
	hash, _ := HashPassword(password)

	// Act
	result := CheckPassword("", hash)

	// Assert
	assert.False(t, result)
}

func TestCheckPassword_EmptyHash(t *testing.T) {
	// Arrange
	password := "somepassword"

	// Act
	result := CheckPassword(password, "")

	// Assert
	assert.False(t, result)
}

func TestCheckPassword_InvalidHash(t *testing.T) {
	// Arrange
	password := "somepassword"

	// Act
	result := CheckPassword(password, "not-a-valid-bcrypt-hash")

	// Assert
	assert.False(t, result)
}

func TestCheckPassword_CaseSensitive(t *testing.T) {
	// Arrange
	password := "MyPassword123"
	hash, _ := HashPassword(password)

	// Act & Assert
	assert.True(t, CheckPassword("MyPassword123", hash))
	assert.False(t, CheckPassword("mypassword123", hash))
	assert.False(t, CheckPassword("MYPASSWORD123", hash))
}

func TestCheckPassword_SpecialCharacters(t *testing.T) {
	// Arrange
	passwords := []string{
		"password!@#$%^&*()",
		"пароль на русском",
		"密码中文",
		"🔐🔑password",
		"pass word with spaces",
		"pass\nword\twith\rwhitespace",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			// Act
			hash, err := HashPassword(password)

			// Assert
			require.NoError(t, err)
			assert.True(t, CheckPassword(password, hash))
			assert.False(t, CheckPassword(password+"x", hash))
		})
	}
}

func TestHashPassword_Consistency(t *testing.T) {
	// Проверяем что один и тот же пароль всегда проходит проверку
	// независимо от того, сколько раз мы хэшируем
	password := "consistentpassword"

	for i := 0; i < 10; i++ {
		hash, err := HashPassword(password)
		require.NoError(t, err)
		assert.True(t, CheckPassword(password, hash))
	}
}
