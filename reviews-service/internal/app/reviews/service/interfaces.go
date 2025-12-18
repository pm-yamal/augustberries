package service

import (
	"context"

	"augustberries/reviews-service/internal/app/reviews/entity"
)

type ReviewServiceInterface interface {
	CreateReview(ctx context.Context, req *entity.CreateReviewRequest, userID string) (*entity.Review, error)
	GetReviewsByProduct(ctx context.Context, productID string) ([]entity.Review, error)
	UpdateReview(ctx context.Context, reviewID string, req *entity.UpdateReviewRequest, userID string) (*entity.Review, error)
	DeleteReview(ctx context.Context, reviewID string, userID string) error
}
