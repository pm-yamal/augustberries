package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"github.com/redis/go-redis/v9"
)

// exchangeRateRepository реализует ExchangeRateRepository для работы с Redis
type exchangeRateRepository struct {
	client *redis.Client
	ttl    time.Duration // TTL для курсов валют
}

// NewExchangeRateRepository создает новый репозиторий курсов валют
func NewExchangeRateRepository(client *redis.Client, ttl time.Duration) ExchangeRateRepository {
	return &exchangeRateRepository{
		client: client,
		ttl:    ttl,
	}
}

// Get получает курс валюты из Redis
func (r *exchangeRateRepository) Get(ctx context.Context, currency string) (*entity.ExchangeRate, error) {
	key := entity.GetRedisKeyForRate(currency)

	// Получаем JSON из Redis
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("exchange rate for %s not found", currency)
		}
		return nil, fmt.Errorf("failed to get exchange rate from redis: %w", err)
	}

	// Десериализуем JSON
	var rate entity.ExchangeRate
	if err := json.Unmarshal([]byte(data), &rate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exchange rate: %w", err)
	}

	return &rate, nil
}

// Set сохраняет курс валюты в Redis с TTL
func (r *exchangeRateRepository) Set(ctx context.Context, rate *entity.ExchangeRate) error {
	key := entity.GetRedisKeyForRate(rate.Currency)

	// Сериализуем в JSON
	data, err := json.Marshal(rate)
	if err != nil {
		return fmt.Errorf("failed to marshal exchange rate: %w", err)
	}

	// Сохраняем в Redis с TTL
	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set exchange rate in redis: %w", err)
	}

	return nil
}

// SetMultiple сохраняет несколько курсов валют батчем для оптимизации
func (r *exchangeRateRepository) SetMultiple(ctx context.Context, rates []*entity.ExchangeRate) error {
	// Используем Pipeline для батчевой отправки команд
	pipe := r.client.Pipeline()

	for _, rate := range rates {
		key := entity.GetRedisKeyForRate(rate.Currency)

		data, err := json.Marshal(rate)
		if err != nil {
			return fmt.Errorf("failed to marshal exchange rate for %s: %w", rate.Currency, err)
		}

		pipe.Set(ctx, key, data, r.ttl)
	}

	// Выполняем все команды батчем
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set multiple exchange rates: %w", err)
	}

	return nil
}

// GetMultiple получает несколько курсов валют
func (r *exchangeRateRepository) GetMultiple(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error) {
	// Используем Pipeline для батчевого получения
	pipe := r.client.Pipeline()

	// Создаем команды для каждой валюты
	cmds := make(map[string]*redis.StringCmd)
	for _, currency := range currencies {
		key := entity.GetRedisKeyForRate(currency)
		cmds[currency] = pipe.Get(ctx, key)
	}

	// Выполняем все команды
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get multiple exchange rates: %w", err)
	}

	// Обрабатываем результаты
	result := make(map[string]*entity.ExchangeRate)
	for currency, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			if err == redis.Nil {
				// Курс не найден - пропускаем
				continue
			}
			return nil, fmt.Errorf("failed to get rate for %s: %w", currency, err)
		}

		var rate entity.ExchangeRate
		if err := json.Unmarshal([]byte(data), &rate); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rate for %s: %w", currency, err)
		}

		result[currency] = &rate
	}

	return result, nil
}

// Exists проверяет существование курса в Redis
func (r *exchangeRateRepository) Exists(ctx context.Context, currency string) (bool, error) {
	key := entity.GetRedisKeyForRate(currency)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return exists > 0, nil
}
