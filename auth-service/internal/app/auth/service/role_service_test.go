package service

import (
	"context"
	"errors"
	"testing"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository/mocks"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== RoleService Tests ====================

// ==================== GetByID Tests ====================

func TestRoleService_GetByID_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	role := &entity.Role{ID: 1, Name: "admin", Description: "Administrator"}
	roleRepo.On("GetByID", ctx, 1).Return(role, nil)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.GetByID(ctx, 1)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.ID)
	assert.Equal(t, "admin", result.Name)

	roleRepo.AssertExpectations(t)
}

func TestRoleService_GetByID_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("GetByID", ctx, 999).Return(nil, pgx.ErrNoRows)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.GetByID(ctx, 999)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== GetByName Tests ====================

func TestRoleService_GetByName_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	role := &entity.Role{ID: 1, Name: "user", Description: "Regular user"}
	roleRepo.On("GetByName", ctx, "user").Return(role, nil)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.GetByName(ctx, "user")

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "user", result.Name)

	roleRepo.AssertExpectations(t)
}

func TestRoleService_GetByName_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("GetByName", ctx, "superadmin").Return(nil, pgx.ErrNoRows)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.GetByName(ctx, "superadmin")

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== List Tests ====================

func TestRoleService_List_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roles := []entity.Role{
		{ID: 1, Name: "user", Description: "Regular user"},
		{ID: 2, Name: "admin", Description: "Administrator"},
		{ID: 3, Name: "manager", Description: "Manager"},
	}
	roleRepo.On("List", ctx).Return(roles, nil)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "user", result[0].Name)
	assert.Equal(t, "admin", result[1].Name)
	assert.Equal(t, "manager", result[2].Name)

	roleRepo.AssertExpectations(t)
}

func TestRoleService_List_Empty(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("List", ctx).Return([]entity.Role{}, nil)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestRoleService_List_Error(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("List", ctx).Return(nil, errors.New("database error"))

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list roles")
}

// ==================== Create Tests ====================

func TestRoleService_Create_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("Create", ctx, &entity.Role{
		Name:        "moderator",
		Description: "Content moderator",
	}).Return(nil)

	service := NewRoleService(roleRepo)

	req := &entity.CreateRoleRequest{
		Name:        "moderator",
		Description: "Content moderator",
	}

	// Act
	result, err := service.Create(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "moderator", result.Name)
	assert.Equal(t, "Content moderator", result.Description)

	roleRepo.AssertExpectations(t)
}

func TestRoleService_Create_Error(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("Create", ctx, &entity.Role{
		Name:        "duplicate",
		Description: "",
	}).Return(errors.New("unique constraint violation"))

	service := NewRoleService(roleRepo)

	req := &entity.CreateRoleRequest{
		Name: "duplicate",
	}

	// Act
	result, err := service.Create(ctx, req)

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create role")
}

// ==================== Update Tests ====================

func TestRoleService_Update_Success_AllFields(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	existingRole := &entity.Role{ID: 1, Name: "user", Description: "Regular user"}
	roleRepo.On("GetByID", ctx, 1).Return(existingRole, nil)
	roleRepo.On("Update", ctx, existingRole).Return(nil)

	service := NewRoleService(roleRepo)

	req := &entity.UpdateRoleRequest{
		Name:        "updated_user",
		Description: "Updated regular user",
	}

	// Act
	result, err := service.Update(ctx, 1, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "updated_user", result.Name)
	assert.Equal(t, "Updated regular user", result.Description)

	roleRepo.AssertExpectations(t)
}

func TestRoleService_Update_Success_PartialFields(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	existingRole := &entity.Role{ID: 1, Name: "user", Description: "Regular user"}
	roleRepo.On("GetByID", ctx, 1).Return(existingRole, nil)
	roleRepo.On("Update", ctx, existingRole).Return(nil)

	service := NewRoleService(roleRepo)

	// Обновляем только описание
	req := &entity.UpdateRoleRequest{
		Description: "New description",
	}

	// Act
	result, err := service.Update(ctx, 1, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "user", result.Name) // Имя не изменилось
	assert.Equal(t, "New description", result.Description)
}

func TestRoleService_Update_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("GetByID", ctx, 999).Return(nil, pgx.ErrNoRows)

	service := NewRoleService(roleRepo)

	req := &entity.UpdateRoleRequest{Name: "new_name"}

	// Act
	result, err := service.Update(ctx, 999, req)

	// Assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== Delete Tests ====================

func TestRoleService_Delete_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("Delete", ctx, 1).Return(nil)

	service := NewRoleService(roleRepo)

	// Act
	err := service.Delete(ctx, 1)

	// Assert
	require.NoError(t, err)
	roleRepo.AssertExpectations(t)
}

func TestRoleService_Delete_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("Delete", ctx, 999).Return(pgx.ErrNoRows)

	service := NewRoleService(roleRepo)

	// Act
	err := service.Delete(ctx, 999)

	// Assert
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== GetPermissions Tests ====================

func TestRoleService_GetPermissions_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	permissions := []entity.Permission{
		{ID: 1, Code: "product.read", Description: "Read products"},
		{ID: 2, Code: "product.create", Description: "Create products"},
	}
	roleRepo.On("GetPermissionsByRoleID", ctx, 1).Return(permissions, nil)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.GetPermissions(ctx, 1)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "product.read", result[0].Code)
	assert.Equal(t, "product.create", result[1].Code)

	roleRepo.AssertExpectations(t)
}

func TestRoleService_GetPermissions_Empty(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("GetPermissionsByRoleID", ctx, 1).Return([]entity.Permission{}, nil)

	service := NewRoleService(roleRepo)

	// Act
	result, err := service.GetPermissions(ctx, 1)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ==================== AssignPermissions Tests ====================

func TestRoleService_AssignPermissions_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	role := &entity.Role{ID: 1, Name: "user"}
	roleRepo.On("GetByID", ctx, 1).Return(role, nil)
	roleRepo.On("AssignPermissions", ctx, 1, []int{1, 2, 3}).Return(nil)

	service := NewRoleService(roleRepo)

	// Act
	err := service.AssignPermissions(ctx, 1, []int{1, 2, 3})

	// Assert
	require.NoError(t, err)
	roleRepo.AssertExpectations(t)
}

func TestRoleService_AssignPermissions_RoleNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("GetByID", ctx, 999).Return(nil, pgx.ErrNoRows)

	service := NewRoleService(roleRepo)

	// Act
	err := service.AssignPermissions(ctx, 999, []int{1, 2})

	// Assert
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== RemovePermissions Tests ====================

func TestRoleService_RemovePermissions_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	role := &entity.Role{ID: 1, Name: "user"}
	roleRepo.On("GetByID", ctx, 1).Return(role, nil)
	roleRepo.On("RemovePermissions", ctx, 1, []int{2, 3}).Return(nil)

	service := NewRoleService(roleRepo)

	// Act
	err := service.RemovePermissions(ctx, 1, []int{2, 3})

	// Assert
	require.NoError(t, err)
	roleRepo.AssertExpectations(t)
}

func TestRoleService_RemovePermissions_RoleNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("GetByID", ctx, 999).Return(nil, pgx.ErrNoRows)

	service := NewRoleService(roleRepo)

	// Act
	err := service.RemovePermissions(ctx, 999, []int{1})

	// Assert
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

// ==================== PermissionService Tests ====================

func TestPermissionService_List_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	permissions := []entity.Permission{
		{ID: 1, Code: "product.read", Description: "Read products"},
		{ID: 2, Code: "product.create", Description: "Create products"},
		{ID: 3, Code: "order.create", Description: "Create orders"},
	}
	roleRepo.On("ListPermissions", ctx).Return(permissions, nil)

	service := NewPermissionService(roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 3)

	roleRepo.AssertExpectations(t)
}

func TestPermissionService_List_Empty(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("ListPermissions", ctx).Return([]entity.Permission{}, nil)

	service := NewPermissionService(roleRepo)

	// Act
	result, err := service.List(ctx)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestPermissionService_Create_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("CreatePermission", ctx, &entity.Permission{
		Code:        "review.create",
		Description: "Create reviews",
	}).Return(nil)

	service := NewPermissionService(roleRepo)

	req := &entity.CreatePermissionRequest{
		Code:        "review.create",
		Description: "Create reviews",
	}

	// Act
	result, err := service.Create(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "review.create", result.Code)

	roleRepo.AssertExpectations(t)
}

func TestPermissionService_Delete_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("DeletePermission", ctx, 1).Return(nil)

	service := NewPermissionService(roleRepo)

	// Act
	err := service.Delete(ctx, 1)

	// Assert
	require.NoError(t, err)
	roleRepo.AssertExpectations(t)
}

func TestPermissionService_Delete_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	roleRepo := new(mocks.MockRoleRepository)

	roleRepo.On("DeletePermission", ctx, 999).Return(pgx.ErrNoRows)

	service := NewPermissionService(roleRepo)

	// Act
	err := service.Delete(ctx, 999)

	// Assert
	assert.ErrorIs(t, err, ErrPermissionNotFound)
}
