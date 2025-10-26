package entity

import (
	"time"

	"github.com/google/uuid"
)

// User представляет пользователя в системе
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"` // не возвращаем в JSON
	Name         string    `json:"name" db:"name"`
	RoleID       int       `json:"role_id" db:"role_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Role представляет роль пользователя (user, manager, admin)
type Role struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description,omitempty" db:"description"`
}

// Permission представляет разрешение (например, product.create)
type Permission struct {
	ID          int    `json:"id" db:"id"`
	Code        string `json:"code" db:"code"`
	Description string `json:"description,omitempty" db:"description"`
}

// RefreshToken хранит refresh токены для обновления JWT
type RefreshToken struct {
	ID        int       `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// BlacklistedToken хранит токены, которые были отозваны
type BlacklistedToken struct {
	ID        int       `json:"id" db:"id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TokenPair содержит access и refresh токены
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // время жизни access token в секундах
}

// UserWithRole содержит информацию о пользователе с его ролью
type UserWithRole struct {
	User
	Role        Role         `json:"role"`
	Permissions []Permission `json:"permissions"`
}
