package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"augustberries/reviews-service/internal/app/reviews/entity"
	"augustberries/reviews-service/internal/app/reviews/infrastructure"
	"augustberries/reviews-service/internal/app/reviews/repository"
)

var (
	// Ошибки бизнес-логики для обработки в handlers
	ErrReviewNotFound = errors.New("review not found")
	ErrUnauthorized   = errors.New("unauthorized access to review")
)

// ReviewService обрабатывает бизнес-логику отзывов
// Координирует работу репозитория и Kafka
type ReviewService struct {
	reviewRepo    repository.ReviewRepository
	kafkaProducer infrastructure.MessagePublisher
}

// NewReviewService создает новый сервис отзывов с внедрением зависимостей
func NewReviewService(
	reviewRepo repository.ReviewRepository,
	kafkaProducer infrastructure.MessagePublisher,
) *ReviewService {
	return &ReviewService{
		reviewRepo:    reviewRepo,
		kafkaProducer: kafkaProducer,
	}
}

// CreateReview создает новый отзыв
// 1. Сохраняет отзыв в MongoDB
// 2. Отправляет событие REVIEW_CREATED в Kafka
func (s *ReviewService) CreateReview(ctx context.Context, userID string, req *entity.CreateReviewRequest) (*entity.Review, error) {
	// Создаем отзыв
	review := &entity.Review{
		ProductID: req.ProductID,
		UserID:    userID,
		Rating:    req.Rating,
		Text:      req.Text,
	}

	// Сохраняем в MongoDB
	if err := s.reviewRepo.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	// Отправляем событие REVIEW_CREATED в Kafka
	event := entity.ReviewEvent{
		EventType: "REVIEW_CREATED",
		ReviewID:  review.ID.Hex(),
		ProductID: review.ProductID,
		UserID:    review.UserID,
		Rating:    review.Rating,
		Timestamp: time.Now(),
	}

	if err := s.publishReviewEvent(ctx, event); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		// Отзыв уже создан, проблемы с Kafka не критичны
		fmt.Printf("failed to publish review created event: %v\n", err)
	}

	return review, nil
}

// GetReviewsByProduct получает все отзывы по ID товара
// Использует индекс product_id для быстрой выборки
func (s *ReviewService) GetReviewsByProduct(ctx context.Context, productID string) ([]entity.Review, error) {
	reviews, err := s.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews: %w", err)
	}

	return reviews, nil
}

// GetReview получает отзыв по ID
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

// UpdateReview обновляет отзыв с проверкой прав доступа
func (s *ReviewService) UpdateReview(ctx context.Context, reviewID string, userID string, req *entity.UpdateReviewRequest) (*entity.Review, error) {
	// Получаем существующий отзыв
	review, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrReviewNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("failed to get review: %w", err)
	}

	// Проверяем что пользователь является автором отзыва
	if review.UserID != userID {
		return nil, ErrUnauthorized
	}

	// Обновляем только переданные поля
	if req.Rating > 0 {
		review.Rating = req.Rating
	}
	if req.Text != "" {
		review.Text = req.Text
	}

	// Сохраняем изменения
	if err := s.reviewRepo.Update(ctx, review); err != nil {
		return nil, fmt.Errorf("failed to update review: %w", err)
	}

	return review, nil
}

// DeleteReview удаляет отзыв с проверкой прав доступа
func (s *ReviewService) DeleteReview(ctx context.Context, reviewID string, userID string) error {
	// Получаем отзыв для проверки доступа
	review, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrReviewNotFound) {
			return ErrReviewNotFound
		}
		return fmt.Errorf("failed to get review: %w", err)
	}

	// Проверяем что пользователь является автором отзыва
	if review.UserID != userID {
		return ErrUnauthorized
	}

	// Удаляем отзыв
	if err := s.reviewRepo.Delete(ctx, reviewID); err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}

	return nil
}

// GetUserReviews получает все отзывы пользователя
func (s *ReviewService) GetUserReviews(ctx context.Context, userID string) ([]entity.Review, error) {
	reviews, err := s.reviewRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user reviews: %w", err)
	}

	return reviews, nil
}

// publishReviewEvent отправляет событие об отзыве в Kafka
func (s *ReviewService) publishReviewEvent(ctx context.Context, event entity.ReviewEvent) error {
	// Сериализуем событие в JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal review event: %w", err)
	}

	// Отправляем в Kafka с ключом = ReviewID для партиционирования
	if err := s.kafkaProducer.PublishMessage(ctx, event.ReviewID, eventData); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	return nil
}
