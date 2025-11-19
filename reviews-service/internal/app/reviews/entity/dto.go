package entity

// CreateReviewRequest - запрос на создание отзыва
type CreateReviewRequest struct {
	ProductID string `json:"product_id" validate:"required"`
	Rating    int    `json:"rating" validate:"required,min=1,max=5"`
	Text      string `json:"text" validate:"required,min=10,max=1000"`
}

// UpdateReviewRequest - запрос на обновление отзыва
type UpdateReviewRequest struct {
	Rating int    `json:"rating" validate:"omitempty,min=1,max=5"`
	Text   string `json:"text" validate:"omitempty,min=10,max=1000"`
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

// ReviewListResponse - ответ со списком отзывов
type ReviewListResponse struct {
	Reviews []Review `json:"reviews"`
	Total   int      `json:"total"`
}
