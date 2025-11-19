package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"augustberries/reviews-service/internal/app/reviews/entity"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	// Стандартные ошибки репозитория для обработки в service layer
	ErrReviewNotFound = errors.New("review not found")
)

type reviewRepository struct {
	collection *mongo.Collection
}

// NewReviewRepository создает новый репозиторий отзывов
// Автоматически создает индекс по product_id для быстрой выборки
func NewReviewRepository(db *mongo.Database) ReviewRepository {
	collection := db.Collection("reviews")

	// Создаем индекс по product_id для быстрого поиска отзывов по товару
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "product_id", Value: 1},
		},
		Options: options.Index().SetName("product_id_idx"),
	}

	_, err := collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Логируем ошибку, но не прерываем работу - индекс может уже существовать
		fmt.Printf("Warning: failed to create index on product_id: %v\n", err)
	}

	// Создаем также индекс по user_id для поиска отзывов пользователя
	userIndexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
		},
		Options: options.Index().SetName("user_id_idx"),
	}

	_, err = collection.Indexes().CreateOne(ctx, userIndexModel)
	if err != nil {
		fmt.Printf("Warning: failed to create index on user_id: %v\n", err)
	}

	return &reviewRepository{
		collection: collection,
	}
}

// Create создает новый отзыв в MongoDB
func (r *reviewRepository) Create(ctx context.Context, review *entity.Review) error {
	review.CreatedAt = time.Now()
	review.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, review)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}

	// Устанавливаем ID из результата вставки
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		review.ID = oid
	}

	return nil
}

// GetByProductID получает все отзывы по ID товара
// Использует индекс product_id_idx для быстрой выборки
func (r *reviewRepository) GetByProductID(ctx context.Context, productID string) ([]entity.Review, error) {
	filter := bson.M{"product_id": productID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find reviews: %w", err)
	}
	defer cursor.Close(ctx)

	var reviews []entity.Review
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, fmt.Errorf("failed to decode reviews: %w", err)
	}

	return reviews, nil
}

// GetByID получает отзыв по ID
func (r *reviewRepository) GetByID(ctx context.Context, id string) (*entity.Review, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid review ID: %w", err)
	}

	filter := bson.M{"_id": objectID}

	var review entity.Review
	err = r.collection.FindOne(ctx, filter).Decode(&review)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("failed to get review: %w", err)
	}

	return &review, nil
}

// Update обновляет отзыв в MongoDB
func (r *reviewRepository) Update(ctx context.Context, review *entity.Review) error {
	review.UpdatedAt = time.Now()

	filter := bson.M{"_id": review.ID}
	update := bson.M{
		"$set": bson.M{
			"rating":     review.Rating,
			"text":       review.Text,
			"updated_at": review.UpdatedAt,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update review: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrReviewNotFound
	}

	return nil
}

// Delete удаляет отзыв из MongoDB
func (r *reviewRepository) Delete(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid review ID: %w", err)
	}

	filter := bson.M{"_id": objectID}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrReviewNotFound
	}

	return nil
}

// GetByUserID получает все отзывы пользователя
// Использует индекс user_id_idx для быстрой выборки
func (r *reviewRepository) GetByUserID(ctx context.Context, userID string) ([]entity.Review, error) {
	filter := bson.M{"user_id": userID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find reviews: %w", err)
	}
	defer cursor.Close(ctx)

	var reviews []entity.Review
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, fmt.Errorf("failed to decode reviews: %w", err)
	}

	return reviews, nil
}
