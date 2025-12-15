package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"augustberries/auth-service/internal/app/auth/entity"
	"augustberries/auth-service/internal/app/auth/service"
)

type RoleHandler struct {
	roleService       *service.RoleService
	permissionService *service.PermissionService
	validator         *validator.Validate
}

func NewRoleHandler(roleService *service.RoleService, permissionService *service.PermissionService) *RoleHandler {
	return &RoleHandler{
		roleService:       roleService,
		permissionService: permissionService,
		validator:         validator.New(),
	}
}

func (h *RoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.roleService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to list roles",
		})
		return
	}

	c.JSON(http.StatusOK, roles)
}

func (h *RoleHandler) GetRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid role ID",
		})
		return
	}

	role, err := h.roleService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to get role",
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req entity.CreateRoleRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": formatRoleValidationErrors(validationErrors),
		})
		return
	}

	role, err := h.roleService.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to create role",
		})
		return
	}

	c.JSON(http.StatusCreated, role)
}

func (h *RoleHandler) UpdateRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid role ID",
		})
		return
	}

	var req entity.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	role, err := h.roleService.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to update role",
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

func (h *RoleHandler) DeleteRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid role ID",
		})
		return
	}

	if err := h.roleService.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to delete role",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Role deleted successfully",
	})
}

func (h *RoleHandler) GetRolePermissions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid role ID",
		})
		return
	}

	permissions, err := h.roleService.GetPermissions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to get role permissions",
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid role ID",
		})
		return
	}

	var req entity.AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": formatRoleValidationErrors(validationErrors),
		})
		return
	}

	if err := h.roleService.AssignPermissions(c.Request.Context(), id, req.PermissionIDs); err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to assign permissions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permissions assigned successfully",
	})
}

func (h *RoleHandler) RemovePermissions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid role ID",
		})
		return
	}

	var req entity.AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	if err := h.roleService.RemovePermissions(c.Request.Context(), id, req.PermissionIDs); err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to remove permissions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permissions removed successfully",
	})
}

func (h *RoleHandler) ListPermissions(c *gin.Context) {
	permissions, err := h.permissionService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to list permissions",
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

func (h *RoleHandler) CreatePermission(c *gin.Context) {
	var req entity.CreatePermissionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body",
		})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": formatRoleValidationErrors(validationErrors),
		})
		return
	}

	permission, err := h.permissionService.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to create permission",
		})
		return
	}

	c.JSON(http.StatusCreated, permission)
}

func (h *RoleHandler) DeletePermission(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid permission ID",
		})
		return
	}

	if err := h.permissionService.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrPermissionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Permission not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": "Failed to delete permission",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permission deleted successfully",
	})
}

func formatRoleValidationErrors(errs validator.ValidationErrors) string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		switch err.Tag() {
		case "required":
			messages = append(messages, err.Field()+" is required")
		case "min":
			messages = append(messages, err.Field()+" must have at least "+err.Param()+" items")
		default:
			messages = append(messages, err.Field()+" is invalid")
		}
	}
	if len(messages) == 0 {
		return "Validation failed"
	}
	return messages[0]
}
