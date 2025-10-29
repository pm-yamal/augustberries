package service

import "errors"

// Доменные ошибки сервисного слоя - не зависят от репозитория
var (
	// Ошибки аутентификации
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

	// Ошибки пользователей
	ErrUserExists   = errors.New("user with this email already exists")
	ErrUserNotFound = errors.New("user not found")

	// Ошибки ролей
	ErrRoleNotFound = errors.New("role not found")

	// Ошибки разрешений
	ErrPermissionNotFound = errors.New("permission not found")

	// Ошибки доступа
	ErrForbidden = errors.New("access forbidden")
)
