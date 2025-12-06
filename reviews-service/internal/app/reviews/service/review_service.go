package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"augustberries/pkg/metrics"
	"augustberries/reviews-service/internal/app/reviews/entity"
	"augustberries/reviews-service/internal/app/reviews/infrastructure"
	"augustberries/reviews-service/internal/app/reviews/repository"
)

var (
	ErrReviewNotFound = errors.New("review not found")
	ErrUnauthorized   = errors.New("unauthorized access to review")
)

type ReviewService struct {
	reviewRepo    repository.ReviewRepository
	kafkaProducer infrastructure.MessagePublisher
}

func NewReviewService(
	reviewRepo repository.ReviewRepository,
	kafkaProducer infrastructure.MessagePublisher,
) *ReviewService {
	return &ReviewService{
		reviewRepo:    reviewRepo,
		kafkaProducer: kafkaProducer,
	}
}

func (s *ReviewService) CreateReview(ctx context.Context, userID string, req *entity.CreateReviewRequest) (*entity.Review, error) {
	review := &entity.Review{
		ProductID: req.ProductID,
		UserID:    userID,
		Rating:    req.Rating,
		Text:      req.Text,
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	event := entity.ReviewEvent{
		EventType: "REVIEW_CREATED",
		ReviewID:  review.ID.Hex(),
		ProductID: review.ProductID,
		UserID:    review.UserID,
		Rating:    review.Rating,
		Timestamp: time.Now(),
	}

	if err := s.publishReviewEvent(ctx, event); err != nil {
		fmt.Printf("failed to publish review created event: %v\n", err)
	}

	metrics.ReviewsCreated.Inc()
	metrics.ReviewsRating.WithLabelValues().Observe(float64(review.Rating))

	return review, nil
}

func (s *ReviewService) GetReviewsByProduct(ctx context.Context, productID string) ([]entity.Review, error) {
	reviews, err := s.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews: %w", err)
	}
	return reviews, nil
}

func (s *ReviewService) GetReview(ctx context.Context, reviewID string) (*entity.Review, error) {
	review, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrReviewNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("failed to get review: %w", err)
	}
	return review, nil
}

func (s *ReviewService) UpdateReview(ctx context.Context, reviewID string, userID string, req *entity.UpdateReviewRequest) (*entity.Review, error) {
	review, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrReviewNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("failed to get review: %w", err)
	}

	if review.UserID != userID {
		return nil, ErrUnauthorized
	}

	if req.Rating > 0 {
		review.Rating = req.Rating
	}
	if req.Text != "" {
		review.Text = req.Text
	}

	if err := s.reviewRepo.Update(ctx, review); err != nil {
		return nil, fmt.Errorf("failed to update review: %w", err)
	}

	return review, nil
}

func (s *ReviewService) DeleteReview(ctx context.Context, reviewID string, userID string) error {
	review, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrReviewNotFound) {
			return ErrReviewNotFound
		}
		return fmt.Errorf("failed to get review: %w", err)
	}

	if review.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.reviewRepo.Delete(ctx, reviewID); err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}

	return nil
}

func (s *ReviewService) GetUserReviews(ctx context.Context, userID string) ([]entity.Review, error) {
	reviews, err := s.reviewRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user reviews: %w", err)
	}
	return reviews, nil
}

func (s *ReviewService) publishReviewEvent(ctx context.Context, event entity.ReviewEvent) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal review event: %w", err)
	}

	if err := s.kafkaProducer.PublishMessage(ctx, event.ReviewID, eventData); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	return nil
}
