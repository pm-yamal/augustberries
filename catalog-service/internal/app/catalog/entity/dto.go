package entity

import "github.com/google/uuid"

// CreateCategoryRequest - запрос на создание категории
type CreateCategoryRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

// UpdateCategoryRequest - запрос на обновление категории
type UpdateCategoryRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

// CreateProductRequest - запрос на создание товара
type CreateProductRequest struct {
	Name        string    `json:"name" validate:"required,min=2,max=200"`
	Description string    `json:"description" validate:"required,min=10,max=2000"`
	Price       float64   `json:"price" validate:"required,gt=0"`
	CategoryID  uuid.UUID `json:"category_id" validate:"required"`
}

// UpdateProductRequest - запрос на обновление товара
type UpdateProductRequest struct {
	Name        string    `json:"name" validate:"omitempty,min=2,max=200"`
	Description string    `json:"description" validate:"omitempty,min=10,max=2000"`
	Price       float64   `json:"price" validate:"omitempty,gt=0"`
	CategoryID  uuid.UUID `json:"category_id" validate:"omitempty"`
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

// ProductListResponse - ответ со списком товаров
type ProductListResponse struct {
	Products []ProductWithCategory `json:"products"`
	Total    int                   `json:"total"`
}

// CategoryListResponse - ответ со списком категорий
type CategoryListResponse struct {
	Categories []Category `json:"categories"`
	Total      int        `json:"total"`
}
