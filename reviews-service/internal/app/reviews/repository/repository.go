package repository

import (
	"context"

	"augustberries/reviews-service/internal/app/reviews/entity"
)

// ReviewRepository определяет методы для работы с отзывами в MongoDB
type ReviewRepository interface {
	Create(ctx context.Context, review *entity.Review) error
	GetByProductID(ctx context.Context, productID string) ([]entity.Review, error)
	GetByID(ctx context.Context, id string) (*entity.Review, error)
	Update(ctx context.Context, review *entity.Review) error
	Delete(ctx context.Context, id string) error
	GetByUserID(ctx context.Context, userID string) ([]entity.Review, error)
}
