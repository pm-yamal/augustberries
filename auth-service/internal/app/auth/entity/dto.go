package entity

// RegisterRequest - запрос на регистрацию
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2"`
}

// LoginRequest - запрос на вход
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest - запрос на обновление токена
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// AuthResponse - ответ с токенами
type AuthResponse struct {
	User   UserWithRole `json:"user"`
	Tokens TokenPair    `json:"tokens"`
}

// ErrorResponse - стандартный ответ об ошибке
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse - стандартный ответ об успехе
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// UpdateUserRequest - запрос на обновление пользователя
type UpdateUserRequest struct {
	Name   string `json:"name,omitempty"`
	Email  string `json:"email,omitempty" validate:"omitempty,email"`
	RoleID int    `json:"role_id,omitempty"`
}

// UpdatePasswordRequest - запрос на обновление пароля
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// CreateRoleRequest - запрос на создание роли
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

// UpdateRoleRequest - запрос на обновление роли
type UpdateRoleRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// AssignPermissionsRequest - запрос на назначение разрешений
type AssignPermissionsRequest struct {
	PermissionIDs []int `json:"permission_ids" validate:"required,min=1"`
}

// CreatePermissionRequest - запрос на создание разрешения
type CreatePermissionRequest struct {
	Code        string `json:"code" validate:"required"`
	Description string `json:"description"`
}
