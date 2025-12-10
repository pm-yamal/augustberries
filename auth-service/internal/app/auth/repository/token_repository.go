package repository

import (
	"context"
	"fmt"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tokenRepository struct {
	db *pgxpool.Pool
}

func NewTokenRepository(db *pgxpool.Pool) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.Exec(ctx, query, userID, token, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}

	return nil
}

func (r *tokenRepository) GetRefreshToken(ctx context.Context, token string) (*entity.RefreshToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at 
		FROM refresh_tokens 
		WHERE token = $1 AND expires_at > $2
	`

	var refreshToken entity.RefreshToken
	err := r.db.QueryRow(ctx, query, token, time.Now()).Scan(
		&refreshToken.ID,
		&refreshToken.UserID,
		&refreshToken.Token,
		&refreshToken.ExpiresAt,
		&refreshToken.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &refreshToken, nil
}

func (r *tokenRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`

	_, err := r.db.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	return nil
}

func (r *tokenRepository) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`

	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user refresh tokens: %w", err)
	}

	return nil
}

func (r *tokenRepository) AddToBlacklist(ctx context.Context, token string, expiresAt time.Time) error {
	query := `
		INSERT INTO blacklisted_tokens (token, expires_at, created_at)
		VALUES ($1, $2, $3)
	`

	_, err := r.db.Exec(ctx, query, token, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add token to blacklist: %w", err)
	}

	return nil
}

func (r *tokenRepository) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM blacklisted_tokens WHERE token = $1 AND expires_at > $2)`

	var exists bool
	err := r.db.QueryRow(ctx, query, token, time.Now()).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check if token is blacklisted: %w", err)
	}

	return exists, nil
}

func (r *tokenRepository) CleanupExpiredTokens(ctx context.Context) error {
	query1 := `DELETE FROM refresh_tokens WHERE expires_at < $1`
	if _, err := r.db.Exec(ctx, query1, time.Now()); err != nil {
		return fmt.Errorf("failed to cleanup expired refresh tokens: %w", err)
	}

	query2 := `DELETE FROM blacklisted_tokens WHERE expires_at < $1`
	if _, err := r.db.Exec(ctx, query2, time.Now()); err != nil {
		return fmt.Errorf("failed to cleanup expired blacklisted tokens: %w", err)
	}

	return nil
}
