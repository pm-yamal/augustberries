package handler

import (
	"errors"
	"net/http"

	"augustberries/catalog-service/internal/app/catalog/entity"
	"augustberries/catalog-service/internal/app/catalog/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// CatalogHandler обрабатывает HTTP запросы для каталога с использованием Gin
type CatalogHandler struct {
	catalogService *service.CatalogService
	validator      *validator.Validate
}

// NewCatalogHandler создает новый обработчик каталога
func NewCatalogHandler(catalogService *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{
		catalogService: catalogService,
		validator:      validator.New(),
	}
}

// CreateCategory обрабатывает POST /categories
func (h *CatalogHandler) CreateCategory(c *gin.Context) {
	var req entity.CreateCategoryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	category, err := h.catalogService.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, category)
}

// GetCategory обрабатывает GET /categories/:id
func (h *CatalogHandler) GetCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	category, err := h.catalogService.GetCategory(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get category"})
		return
	}

	c.JSON(http.StatusOK, category)
}

// GetAllCategories обрабатывает GET /categories (с кешированием)
func (h *CatalogHandler) GetAllCategories(c *gin.Context) {
	categories, err := h.catalogService.GetAllCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get categories"})
		return
	}

	response := entity.CategoryListResponse{
		Categories: categories,
		Total:      len(categories),
	}

	c.JSON(http.StatusOK, response)
}

// UpdateCategory обрабатывает PUT /categories/:id
func (h *CatalogHandler) UpdateCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	var req entity.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	category, err := h.catalogService.UpdateCategory(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update category"})
		return
	}

	c.JSON(http.StatusOK, category)
}

// DeleteCategory обрабатывает DELETE /categories/:id
func (h *CatalogHandler) DeleteCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	if err := h.catalogService.DeleteCategory(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}

	c.JSON(http.StatusOK, entity.SuccessResponse{
		Message: "Category deleted successfully",
	})
}

// CreateProduct обрабатывает POST /products
func (h *CatalogHandler) CreateProduct(c *gin.Context) {
	var req entity.CreateProductRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	product, err := h.catalogService.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// GetProduct обрабатывает GET /products/:id
func (h *CatalogHandler) GetProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	product, err := h.catalogService.GetProduct(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get product"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// GetAllProducts обрабатывает GET /products
func (h *CatalogHandler) GetAllProducts(c *gin.Context) {
	products, err := h.catalogService.GetAllProducts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get products"})
		return
	}

	response := entity.ProductListResponse{
		Products: products,
		Total:    len(products),
	}

	c.JSON(http.StatusOK, response)
}

// UpdateProduct обрабатывает PUT /products/:id
// При изменении цены отправляет событие PRODUCT_UPDATED в Kafka
func (h *CatalogHandler) UpdateProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var req entity.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	product, err := h.catalogService.UpdateProduct(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		if errors.Is(err, service.ErrCategoryNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// DeleteProduct обрабатывает DELETE /products/:id
func (h *CatalogHandler) DeleteProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	if err := h.catalogService.DeleteProduct(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, entity.SuccessResponse{
		Message: "Product deleted successfully",
	})
}

// formatValidationError форматирует ошибки валидации
func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			return fieldError.Field() + " is " + fieldError.Tag()
		}
	}
	return "Validation failed"
}
