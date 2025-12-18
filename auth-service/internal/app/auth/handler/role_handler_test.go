package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/repository/mocks"
	"augustberries/auth-service/internal/app/auth/service"
)

func newTestRoleHandler() (*RoleHandler, *mocks.MockRoleRepository) {
	mockRoleRepo := new(mocks.MockRoleRepository)
	roleService := service.NewRoleService(mockRoleRepo)
	permissionService := service.NewPermissionService(mockRoleRepo)
	handler := NewRoleHandler(roleService, permissionService)
	return handler, mockRoleRepo
}

func setupRoleTestRouter(method, path string, handlerFunc gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, path, handlerFunc)
	return router
}

func TestListRoles(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	expectedRoles := []entity.Role{
		{ID: 1, Name: "admin", Description: "Administrator"},
		{ID: 2, Name: "user", Description: "Regular user"},
	}

	mockRepo.On("List", mock.Anything).Return(expectedRoles, nil)

	router := setupRoleTestRouter(http.MethodGet, "/admin/roles", handler.ListRoles)
	req := httptest.NewRequest(http.MethodGet, "/admin/roles", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var roles []entity.Role
	err := json.Unmarshal(rec.Body.Bytes(), &roles)
	assert.NoError(t, err)
	assert.Len(t, roles, 2)
	mockRepo.AssertExpectations(t)
}

func TestGetRole(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	expectedRole := &entity.Role{ID: 1, Name: "admin", Description: "Administrator"}

	mockRepo.On("GetByID", mock.Anything, 1).Return(expectedRole, nil)

	router := setupRoleTestRouter(http.MethodGet, "/admin/roles/:id", handler.GetRole)
	req := httptest.NewRequest(http.MethodGet, "/admin/roles/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateRole(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Role")).Return(nil)

	reqBody := entity.CreateRoleRequest{
		Name:        "manager",
		Description: "Manager role",
	}
	body, _ := json.Marshal(reqBody)

	router := setupRoleTestRouter(http.MethodPost, "/admin/roles", handler.CreateRole)
	req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoleValidationError(t *testing.T) {
	handler, _ := newTestRoleHandler()

	reqBody := entity.CreateRoleRequest{
		Name: "",
	}
	body, _ := json.Marshal(reqBody)

	router := setupRoleTestRouter(http.MethodPost, "/admin/roles", handler.CreateRole)
	req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteRole(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	mockRepo.On("Delete", mock.Anything, 1).Return(nil)

	router := setupRoleTestRouter(http.MethodDelete, "/admin/roles/:id", handler.DeleteRole)
	req := httptest.NewRequest(http.MethodDelete, "/admin/roles/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestListPermissions(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	expectedPermissions := []entity.Permission{
		{ID: 1, Code: "product.create", Description: "Create products"},
		{ID: 2, Code: "product.delete", Description: "Delete products"},
	}

	mockRepo.On("ListPermissions", mock.Anything).Return(expectedPermissions, nil)

	router := setupRoleTestRouter(http.MethodGet, "/admin/permissions", handler.ListPermissions)
	req := httptest.NewRequest(http.MethodGet, "/admin/permissions", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var permissions []entity.Permission
	err := json.Unmarshal(rec.Body.Bytes(), &permissions)
	assert.NoError(t, err)
	assert.Len(t, permissions, 2)
	mockRepo.AssertExpectations(t)
}

func TestCreatePermission(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	mockRepo.On("CreatePermission", mock.Anything, mock.AnythingOfType("*entity.Permission")).Return(nil)

	reqBody := entity.CreatePermissionRequest{
		Code:        "order.create",
		Description: "Create orders",
	}
	body, _ := json.Marshal(reqBody)

	router := setupRoleTestRouter(http.MethodPost, "/admin/permissions", handler.CreatePermission)
	req := httptest.NewRequest(http.MethodPost, "/admin/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestAssignPermissions(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	mockRepo.On("GetByID", mock.Anything, 1).Return(&entity.Role{ID: 1, Name: "admin"}, nil)
	mockRepo.On("AssignPermissions", mock.Anything, 1, []int{1, 2}).Return(nil)

	reqBody := entity.AssignPermissionsRequest{
		PermissionIDs: []int{1, 2},
	}
	body, _ := json.Marshal(reqBody)

	router := setupRoleTestRouter(http.MethodPost, "/admin/roles/:id/permissions", handler.AssignPermissions)
	req := httptest.NewRequest(http.MethodPost, "/admin/roles/1/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetRolePermissions(t *testing.T) {
	handler, mockRepo := newTestRoleHandler()

	expectedPermissions := []entity.Permission{
		{ID: 1, Code: "product.create", Description: "Create products"},
	}

	mockRepo.On("GetPermissionsByRoleID", mock.Anything, 1).Return(expectedPermissions, nil)

	router := setupRoleTestRouter(http.MethodGet, "/admin/roles/:id/permissions", handler.GetRolePermissions)
	req := httptest.NewRequest(http.MethodGet, "/admin/roles/1/permissions", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetRoleInvalidID(t *testing.T) {
	handler, _ := newTestRoleHandler()

	router := setupRoleTestRouter(http.MethodGet, "/admin/roles/:id", handler.GetRole)
	req := httptest.NewRequest(http.MethodGet, "/admin/roles/invalid", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
