package service

import "errors"

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	ErrUserExists          = errors.New("user with this email already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrRoleNotFound        = errors.New("role not found")
	ErrPermissionNotFound  = errors.New("permission not found")
	ErrForbidden           = errors.New("access forbidden")
	ErrValidation          = errors.New("validation error")
	ErrTokenExpired        = errors.New("token has expired")
	ErrInvalidToken        = errors.New("invalid token")
	ErrTokenBlacklisted    = errors.New("token is blacklisted")
	ErrInternal            = errors.New("internal error")
)
