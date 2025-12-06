package repository

import (
	"context"
	"fmt"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type redisTokenRepository struct {
	client *redis.Client
}

func NewRedisTokenRepository(client *redis.Client) TokenRepository {
	return &redisTokenRepository{client: client}
}

func (r *redisTokenRepository) SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	key := fmt.Sprintf("refresh_token:%s", token)

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return fmt.Errorf("token already expired")
	}

	err := r.client.Set(ctx, key, userID.String(), ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to save refresh token to Redis: %w", err)
	}

	userTokensKey := fmt.Sprintf("user_tokens:%s", userID.String())
	err = r.client.SAdd(ctx, userTokensKey, token).Err()
	if err != nil {
		return fmt.Errorf("failed to add token to user tokens set: %w", err)
	}

	r.client.Expire(ctx, userTokensKey, ttl)

	return nil
}

func (r *redisTokenRepository) GetRefreshToken(ctx context.Context, token string) (*entity.RefreshToken, error) {
	key := fmt.Sprintf("refresh_token:%s", token)

	userIDStr, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("refresh token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token from Redis: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in Redis: %w", err)
	}

	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get token TTL: %w", err)
	}

	return &entity.RefreshToken{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}, nil
}

func (r *redisTokenRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh_token:%s", token)

	userIDStr, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get user ID for token: %w", err)
	}

	err = r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete refresh token from Redis: %w", err)
	}

	if userIDStr != "" {
		userTokensKey := fmt.Sprintf("user_tokens:%s", userIDStr)
		r.client.SRem(ctx, userTokensKey, token)
	}

	return nil
}

func (r *redisTokenRepository) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	userTokensKey := fmt.Sprintf("user_tokens:%s", userID.String())

	tokens, err := r.client.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	for _, token := range tokens {
		key := fmt.Sprintf("refresh_token:%s", token)
		r.client.Del(ctx, key)
	}

	err = r.client.Del(ctx, userTokensKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user tokens set: %w", err)
	}

	return nil
}

func (r *redisTokenRepository) AddToBlacklist(ctx context.Context, token string, expiresAt time.Time) error {
	key := fmt.Sprintf("blacklist:%s", token)

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	err := r.client.Set(ctx, key, "1", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to add token to blacklist: %w", err)
	}

	return nil
}

func (r *redisTokenRepository) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", token)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if token is blacklisted: %w", err)
	}

	return exists > 0, nil
}

// CleanupExpiredTokens не требуется для Redis - TTL автоматически удаляет истекшие ключи
func (r *redisTokenRepository) CleanupExpiredTokens(ctx context.Context) error {
	return nil
}
