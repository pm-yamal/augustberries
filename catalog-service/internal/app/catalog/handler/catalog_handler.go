package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"augustberries/catalog-service/internal/app/catalog/entity"
	"augustberries/catalog-service/internal/app/catalog/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// CatalogHandler обрабатывает HTTP запросы для каталога
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

// === CATEGORIES HANDLERS ===

// CreateCategory обрабатывает POST /categories/
func (h *CatalogHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req entity.CreateCategoryRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	category, err := h.catalogService.CreateCategory(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	respondJSON(w, http.StatusCreated, category)
}

// GetCategory обрабатывает GET /categories/{id}
func (h *CatalogHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	category, err := h.catalogService.GetCategory(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			respondError(w, http.StatusNotFound, "Category not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get category")
		return
	}

	respondJSON(w, http.StatusOK, category)
}

// GetAllCategories обрабатывает GET /categories/ (с кешированием)
func (h *CatalogHandler) GetAllCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.catalogService.GetAllCategories(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get categories")
		return
	}

	response := entity.CategoryListResponse{
		Categories: categories,
		Total:      len(categories),
	}

	respondJSON(w, http.StatusOK, response)
}

// UpdateCategory обрабатывает PUT /categories/{id}
func (h *CatalogHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	var req entity.UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	category, err := h.catalogService.UpdateCategory(r.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			respondError(w, http.StatusNotFound, "Category not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to update category")
		return
	}

	respondJSON(w, http.StatusOK, category)
}

// DeleteCategory обрабатывает DELETE /categories/{id}
func (h *CatalogHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	if err := h.catalogService.DeleteCategory(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			respondError(w, http.StatusNotFound, "Category not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}

	respondJSON(w, http.StatusOK, entity.SuccessResponse{
		Message: "Category deleted successfully",
	})
}

// === PRODUCTS HANDLERS ===

// CreateProduct обрабатывает POST /products/
func (h *CatalogHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req entity.CreateProductRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	product, err := h.catalogService.CreateProduct(r.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			respondError(w, http.StatusBadRequest, "Category not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to create product")
		return
	}

	respondJSON(w, http.StatusCreated, product)
}

// GetProduct обрабатывает GET /products/{id}
func (h *CatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	product, err := h.catalogService.GetProduct(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			respondError(w, http.StatusNotFound, "Product not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get product")
		return
	}

	respondJSON(w, http.StatusOK, product)
}

// GetAllProducts обрабатывает GET /products/
func (h *CatalogHandler) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.catalogService.GetAllProducts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get products")
		return
	}

	response := entity.ProductListResponse{
		Products: products,
		Total:    len(products),
	}

	respondJSON(w, http.StatusOK, response)
}

// UpdateProduct обрабатывает PUT /products/{id}
func (h *CatalogHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var req entity.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if err := h.validator.Struct(req); err != nil {
		respondError(w, http.StatusBadRequest, formatValidationError(err))
		return
	}

	product, err := h.catalogService.UpdateProduct(r.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			respondError(w, http.StatusNotFound, "Product not found")
			return
		}
		if errors.Is(err, service.ErrCategoryNotFound) {
			respondError(w, http.StatusBadRequest, "Category not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to update product")
		return
	}

	respondJSON(w, http.StatusOK, product)
}

// DeleteProduct обрабатывает DELETE /products/{id}
func (h *CatalogHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	if err := h.catalogService.DeleteProduct(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			respondError(w, http.StatusNotFound, "Product not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to delete product")
		return
	}

	respondJSON(w, http.StatusOK, entity.SuccessResponse{
		Message: "Product deleted successfully",
	})
}

// === HELPER FUNCTIONS ===

// respondJSON отправляет JSON ответ
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError отправляет ответ об ошибке
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, entity.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

// formatValidationError форматирует ошибки валидации
func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		if len(validationErrors) > 0 {
			return validationErrors[0].Field() + " validation failed"
		}
	}
	return "Validation failed"
}
