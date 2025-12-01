package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"augustberries/reviews-service/internal/app/reviews/entity"
	"augustberries/reviews-service/internal/app/reviews/repository"
	"augustberries/reviews-service/internal/app/reviews/repository/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCreateReview_Success(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	userID := "user-123"
	req := &entity.CreateReviewRequest{ProductID: "product-456", Rating: 5, Text: "Great product!"}

	reviewRepo.On("Create", ctx, mock.AnythingOfType("*entity.Review")).Return(nil).Run(func(args mock.Arguments) {
		review := args.Get(1).(*entity.Review)
		review.ID = primitive.NewObjectID()
	})
	kafkaProducer.On("PublishMessage", ctx, mock.Anything, mock.Anything).Return(nil)

	result, err := service.CreateReview(ctx, userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, 5, result.Rating)
}

func TestCreateReview_RepoError(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	req := &entity.CreateReviewRequest{ProductID: "product-456", Rating: 4, Text: "Good product."}

	reviewRepo.On("Create", ctx, mock.Anything).Return(errors.New("db error"))

	result, err := service.CreateReview(ctx, "user-123", req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreateReview_KafkaErrorIgnored(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	req := &entity.CreateReviewRequest{ProductID: "product-456", Rating: 3, Text: "Average product."}

	reviewRepo.On("Create", ctx, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		review := args.Get(1).(*entity.Review)
		review.ID = primitive.NewObjectID()
	})
	kafkaProducer.On("PublishMessage", ctx, mock.Anything, mock.Anything).Return(errors.New("kafka error"))

	result, err := service.CreateReview(ctx, "user-123", req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGetReviewsByProduct_Success(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	productID := "product-456"
	reviews := []entity.Review{
		{ID: primitive.NewObjectID(), ProductID: productID, UserID: "user-1", Rating: 5},
		{ID: primitive.NewObjectID(), ProductID: productID, UserID: "user-2", Rating: 4},
	}

	reviewRepo.On("GetByProductID", ctx, productID).Return(reviews, nil)

	result, err := service.GetReviewsByProduct(ctx, productID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetReviewsByProduct_Empty(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewRepo.On("GetByProductID", ctx, "no-reviews").Return([]entity.Review{}, nil)

	result, err := service.GetReviewsByProduct(ctx, "no-reviews")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetReview_Success(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID()
	review := &entity.Review{ID: reviewID, ProductID: "product-456", UserID: "user-123", Rating: 5}

	reviewRepo.On("GetByID", ctx, reviewID.Hex()).Return(review, nil)

	result, err := service.GetReview(ctx, reviewID.Hex())

	assert.NoError(t, err)
	assert.Equal(t, reviewID, result.ID)
}

func TestGetReview_NotFound(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID().Hex()

	reviewRepo.On("GetByID", ctx, reviewID).Return(nil, repository.ErrReviewNotFound)

	result, err := service.GetReview(ctx, reviewID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrReviewNotFound)
}

func TestUpdateReview_Success(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID()
	userID := "user-123"
	existing := &entity.Review{ID: reviewID, ProductID: "product-456", UserID: userID, Rating: 3, Text: "Old text"}
	req := &entity.UpdateReviewRequest{Rating: 5, Text: "Updated text"}

	reviewRepo.On("GetByID", ctx, reviewID.Hex()).Return(existing, nil)
	reviewRepo.On("Update", ctx, mock.AnythingOfType("*entity.Review")).Return(nil)

	result, err := service.UpdateReview(ctx, reviewID.Hex(), userID, req)

	assert.NoError(t, err)
	assert.Equal(t, 5, result.Rating)
	assert.Equal(t, "Updated text", result.Text)
}

func TestUpdateReview_NotFound(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID().Hex()

	reviewRepo.On("GetByID", ctx, reviewID).Return(nil, repository.ErrReviewNotFound)

	result, err := service.UpdateReview(ctx, reviewID, "user-123", &entity.UpdateReviewRequest{Rating: 5})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrReviewNotFound)
}

func TestUpdateReview_Unauthorized(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID()
	existing := &entity.Review{ID: reviewID, UserID: "owner-user", Rating: 4}

	reviewRepo.On("GetByID", ctx, reviewID.Hex()).Return(existing, nil)

	result, err := service.UpdateReview(ctx, reviewID.Hex(), "another-user", &entity.UpdateReviewRequest{Rating: 1})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestDeleteReview_Success(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID()
	userID := "user-123"
	review := &entity.Review{ID: reviewID, UserID: userID}

	reviewRepo.On("GetByID", ctx, reviewID.Hex()).Return(review, nil)
	reviewRepo.On("Delete", ctx, reviewID.Hex()).Return(nil)

	err := service.DeleteReview(ctx, reviewID.Hex(), userID)

	assert.NoError(t, err)
}

func TestDeleteReview_NotFound(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID().Hex()

	reviewRepo.On("GetByID", ctx, reviewID).Return(nil, repository.ErrReviewNotFound)

	err := service.DeleteReview(ctx, reviewID, "user-123")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrReviewNotFound)
}

func TestDeleteReview_Unauthorized(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewID := primitive.NewObjectID()
	review := &entity.Review{ID: reviewID, UserID: "owner-user"}

	reviewRepo.On("GetByID", ctx, reviewID.Hex()).Return(review, nil)

	err := service.DeleteReview(ctx, reviewID.Hex(), "another-user")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestGetUserReviews_Success(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	userID := "user-123"
	reviews := []entity.Review{
		{ID: primitive.NewObjectID(), UserID: userID, ProductID: "product-1", Rating: 5, CreatedAt: time.Now()},
		{ID: primitive.NewObjectID(), UserID: userID, ProductID: "product-2", Rating: 4, CreatedAt: time.Now()},
	}

	reviewRepo.On("GetByUserID", ctx, userID).Return(reviews, nil)

	result, err := service.GetUserReviews(ctx, userID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetUserReviews_Empty(t *testing.T) {
	reviewRepo := new(mocks.MockReviewRepository)
	kafkaProducer := &mocks.MockMessagePublisher{Messages: make([][]byte, 0)}
	service := NewReviewService(reviewRepo, kafkaProducer)

	ctx := context.Background()
	reviewRepo.On("GetByUserID", ctx, "no-reviews-user").Return([]entity.Review{}, nil)

	result, err := service.GetUserReviews(ctx, "no-reviews-user")

	assert.NoError(t, err)
	assert.Empty(t, result)
}
