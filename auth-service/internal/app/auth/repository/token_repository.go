package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"augustberries/auth-service/internal/app/auth/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
)

type tokenRepository struct {
	db *pgxpool.Pool
}

// NewTokenRepository создает новый репозиторий токенов
func NewTokenRepository(db *pgxpool.Pool) TokenRepository {
	return &tokenRepository{db: db}
}

// SaveRefreshToken сохраняет refresh токен в БД
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

// GetRefreshToken получает refresh токен из БД
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &refreshToken, nil
}

// DeleteRefreshToken удаляет конкретный refresh токен
func (r *tokenRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`

	_, err := r.db.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	return nil
}

// DeleteUserRefreshTokens удаляет все refresh токены пользователя
func (r *tokenRepository) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`

	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user refresh tokens: %w", err)
	}

	return nil
}

// AddToBlacklist добавляет токен в черный список
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

// IsBlacklisted проверяет, находится ли токен в черном списке
func (r *tokenRepository) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM blacklisted_tokens WHERE token = $1 AND expires_at > $2)`

	var exists bool
	err := r.db.QueryRow(ctx, query, token, time.Now()).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check if token is blacklisted: %w", err)
	}

	return exists, nil
}

// CleanupExpiredTokens удаляет истекшие токены из обеих таблиц
func (r *tokenRepository) CleanupExpiredTokens(ctx context.Context) error {
	// Удаляем истекшие refresh токены
	query1 := `DELETE FROM refresh_tokens WHERE expires_at < $1`
	if _, err := r.db.Exec(ctx, query1, time.Now()); err != nil {
		return fmt.Errorf("failed to cleanup expired refresh tokens: %w", err)
	}

	// Удаляем истекшие токены из черного списка
	query2 := `DELETE FROM blacklisted_tokens WHERE expires_at < $1`
	if _, err := r.db.Exec(ctx, query2, time.Now()); err != nil {
		return fmt.Errorf("failed to cleanup expired blacklisted tokens: %w", err)
	}

	return nil
}
