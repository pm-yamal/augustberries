package entity

import "github.com/google/uuid"

type CreateCategoryRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type UpdateCategoryRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type CreateProductRequest struct {
	Name        string    `json:"name" validate:"required,min=2,max=200"`
	Description string    `json:"description" validate:"required,min=10,max=2000"`
	Price       float64   `json:"price" validate:"required,gt=0"`
	CategoryID  uuid.UUID `json:"category_id" validate:"required"`
}

type UpdateProductRequest struct {
	Name        string    `json:"name" validate:"omitempty,min=2,max=200"`
	Description string    `json:"description" validate:"omitempty,min=10,max=2000"`
	Price       float64   `json:"price" validate:"omitempty,gt=0"`
	CategoryID  uuid.UUID `json:"category_id" validate:"omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProductListResponse struct {
	Products []ProductWithCategory `json:"products"`
	Total    int                   `json:"total"`
}

type CategoryListResponse struct {
	Categories []Category `json:"categories"`
	Total      int        `json:"total"`
}
