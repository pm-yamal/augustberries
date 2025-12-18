package entity

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type AuthResponse struct {
	User   UserWithRole `json:"user"`
	Tokens TokenPair    `json:"tokens"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type UpdateUserRequest struct {
	Name   string `json:"name,omitempty"`
	Email  string `json:"email,omitempty" validate:"omitempty,email"`
	RoleID int    `json:"role_id,omitempty"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

type UpdateRoleRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []int `json:"permission_ids" validate:"required,min=1"`
}

type CreatePermissionRequest struct {
	Code        string `json:"code" validate:"required"`
	Description string `json:"description"`
}
