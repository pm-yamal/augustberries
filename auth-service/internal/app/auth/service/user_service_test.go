package service

import (
	"context"
	"testing"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository/mocks"
	"augustberries/auth-service/internal/app/auth/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== GetByID Tests ====================

func TestUserService_GetByID_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()
	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	roleRepo.On("GetByID", ctx, user.RoleID).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, user.RoleID).Return(permissions, nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.GetByID(ctx, user.ID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.ID, result.ID)
	assert.Equal(t, user.Email, result.Email)
	assert.Equal(t, role.Name, result.Role.Name)
	assert.Len(t, result.Permissions, 2)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
}

func TestUserService_GetByID_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userID := uuid.New()
	userRepo.On("GetByID", ctx, userID).Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.GetByID(ctx, userID)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestUserService_GetByID_RoleNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	roleRepo.On("GetByID", ctx, user.RoleID).Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.GetByID(ctx, user.ID)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== GetByEmail Tests ====================

func TestUserService_GetByEmail_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()
	role := newTestRole()
	permissions := newTestPermissions()

	userRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)
	roleRepo.On("GetByID", ctx, user.RoleID).Return(role, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, user.RoleID).Return(permissions, nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.GetByEmail(ctx, user.Email)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.Email, result.Email)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
}

func TestUserService_GetByEmail_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userRepo.On("GetByEmail", ctx, "notfound@example.com").Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.GetByEmail(ctx, "notfound@example.com")

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ==================== Update Tests ====================

func TestUserService_Update_Success_AllFields(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()
	newRole := &entity.Role{ID: 2, Name: "admin", Description: "Administrator"}

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	roleRepo.On("GetByID", ctx, 2).Return(newRole, nil)
	userRepo.On("Update", ctx, user).Return(nil)

	service := NewUserService(userRepo, roleRepo)

	req := &entity.UpdateUserRequest{
		Name:   "Updated Name",
		Email:  "updated@example.com",
		RoleID: 2,
	}

	// Act
	result, err := service.Update(ctx, user.ID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated Name", result.Name)
	assert.Equal(t, "updated@example.com", result.Email)
	assert.Equal(t, 2, result.RoleID)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
}

func TestUserService_Update_Success_PartialFields(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()
	originalEmail := user.Email
	originalRoleID := user.RoleID

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	userRepo.On("Update", ctx, user).Return(nil)

	service := NewUserService(userRepo, roleRepo)

	// Обновляем только имя
	req := &entity.UpdateUserRequest{
		Name: "Only Name Updated",
	}

	// Act
	result, err := service.Update(ctx, user.ID, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Only Name Updated", result.Name)
	assert.Equal(t, originalEmail, result.Email)       // Email не изменился
	assert.Equal(t, originalRoleID, result.RoleID)     // RoleID не изменился
}

func TestUserService_Update_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userID := uuid.New()
	userRepo.On("GetByID", ctx, userID).Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	req := &entity.UpdateUserRequest{Name: "New Name"}

	// Act
	result, err := service.Update(ctx, userID, req)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestUserService_Update_RoleNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	roleRepo.On("GetByID", ctx, 999).Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	req := &entity.UpdateUserRequest{RoleID: 999}

	// Act
	result, err := service.Update(ctx, user.ID, req)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== UpdatePassword Tests ====================

func TestUserService_UpdatePassword_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser() // Пароль: password123

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	userRepo.On("Update", ctx, user).Return(nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	err := service.UpdatePassword(ctx, user.ID, "password123", "newpassword456")

	// Assert
	require.NoError(t, err)

	// Проверяем что новый пароль работает
	assert.True(t, util.CheckPassword("newpassword456", user.PasswordHash))

	userRepo.AssertExpectations(t)
}

func TestUserService_UpdatePassword_WrongOldPassword(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	user := newTestUser()

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	err := service.UpdatePassword(ctx, user.ID, "wrongpassword", "newpassword456")

	// Assert
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestUserService_UpdatePassword_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userID := uuid.New()
	userRepo.On("GetByID", ctx, userID).Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	// Act
	err := service.UpdatePassword(ctx, userID, "old", "new")

	// Assert
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ==================== Delete Tests ====================

func TestUserService_Delete_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userID := uuid.New()
	userRepo.On("Delete", ctx, userID).Return(nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	err := service.Delete(ctx, userID)

	// Assert
	require.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_Delete_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userID := uuid.New()
	userRepo.On("Delete", ctx, userID).Return(pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	// Act
	err := service.Delete(ctx, userID)

	// Assert
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ==================== List Tests ====================

func TestUserService_List_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	users := []entity.User{
		{
			ID:           uuid.New(),
			Email:        "user1@example.com",
			PasswordHash: "hash1",
			Name:         "User 1",
			RoleID:       1,
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			Email:        "user2@example.com",
			PasswordHash: "hash2",
			Name:         "User 2",
			RoleID:       2,
			CreatedAt:    time.Now(),
		},
	}

	role1 := &entity.Role{ID: 1, Name: "user"}
	role2 := &entity.Role{ID: 2, Name: "admin"}
	permissions := newTestPermissions()

	userRepo.On("List", ctx).Return(users, nil)
	roleRepo.On("GetByID", ctx, 1).Return(role1, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, 1).Return(permissions, nil)
	roleRepo.On("GetByID", ctx, 2).Return(role2, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, 2).Return(permissions, nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "user1@example.com", result[0].Email)
	assert.Equal(t, "user", result[0].Role.Name)
	assert.Equal(t, "user2@example.com", result[1].Email)
	assert.Equal(t, "admin", result[1].Role.Name)

	userRepo.AssertExpectations(t)
	roleRepo.AssertExpectations(t)
}

func TestUserService_List_Empty(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	userRepo.On("List", ctx).Return([]entity.User{}, nil)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestUserService_List_SkipsUsersWithMissingRole(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userRepo := new(mocks.MockUserRepository)
	roleRepo := new(mocks.MockRoleRepository)

	users := []entity.User{
		{
			ID:           uuid.New(),
			Email:        "user1@example.com",
			Name:         "User 1",
			RoleID:       1,
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			Email:        "user2@example.com",
			Name:         "User 2",
			RoleID:       999, // Несуществующая роль
			CreatedAt:    time.Now(),
		},
	}

	role1 := &entity.Role{ID: 1, Name: "user"}
	permissions := newTestPermissions()

	userRepo.On("List", ctx).Return(users, nil)
	roleRepo.On("GetByID", ctx, 1).Return(role1, nil)
	roleRepo.On("GetPermissionsByRoleID", ctx, 1).Return(permissions, nil)
	roleRepo.On("GetByID", ctx, 999).Return(nil, pgx.ErrNoRows)

	service := NewUserService(userRepo, roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 1) // Только один пользователь с валидной ролью
	assert.Equal(t, "user1@example.com", result[0].Email)
}
