package entity

type CreateReviewRequest struct {
	ProductID string `json:"product_id" validate:"required"`
	Rating    int    `json:"rating" validate:"required,min=1,max=5"`
	Text      string `json:"text" validate:"required,min=10,max=1000"`
}

type UpdateReviewRequest struct {
	Rating int    `json:"rating" validate:"omitempty,min=1,max=5"`
	Text   string `json:"text" validate:"omitempty,min=10,max=1000"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ReviewListResponse struct {
	Reviews []Review `json:"reviews"`
	Total   int      `json:"total"`
}
