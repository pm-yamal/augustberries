package repository

import (
	"context"
	"testing"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ExchangeRateRepositoryTestSuite тестовый suite для Redis repository
type ExchangeRateRepositoryTestSuite struct {
	suite.Suite
	miniRedis *miniredis.Miniredis
	client    *redis.Client
	repo      ExchangeRateRepository
}

func TestExchangeRateRepositorySuite(t *testing.T) {
	suite.Run(t, new(ExchangeRateRepositoryTestSuite))
}

func (s *ExchangeRateRepositoryTestSuite) SetupSuite() {
	var err error
	s.miniRedis, err = miniredis.Run()
	require.NoError(s.T(), err)

	s.client = redis.NewClient(&redis.Options{
		Addr: s.miniRedis.Addr(),
	})

	s.repo = NewExchangeRateRepository(s.client, 30*time.Minute)
}

func (s *ExchangeRateRepositoryTestSuite) SetupTest() {
	s.miniRedis.FlushAll()
}

func (s *ExchangeRateRepositoryTestSuite) TearDownSuite() {
	s.client.Close()
	s.miniRedis.Close()
}

// ===================== Get Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestGet_Success() {
	ctx := context.Background()

	// Arrange - сначала сохраняем курс
	rate := &entity.ExchangeRate{
		Currency:  "USD",
		Rate:      1.0,
		UpdatedAt: time.Now(),
	}
	err := s.repo.Set(ctx, rate)
	s.NoError(err)

	// Act
	result, err := s.repo.Get(ctx, "USD")

	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal("USD", result.Currency)
	s.Equal(1.0, result.Rate)
}

func (s *ExchangeRateRepositoryTestSuite) TestGet_NotFound() {
	ctx := context.Background()

	// Act
	result, err := s.repo.Get(ctx, "XYZ")

	// Assert
	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "not found")
}

// ===================== Set Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestSet_Success() {
	ctx := context.Background()

	rate := &entity.ExchangeRate{
		Currency:  "EUR",
		Rate:      0.93,
		UpdatedAt: time.Now(),
	}

	// Act
	err := s.repo.Set(ctx, rate)

	// Assert
	s.NoError(err)

	// Проверяем что сохранилось
	result, err := s.repo.Get(ctx, "EUR")
	s.NoError(err)
	s.Equal(0.93, result.Rate)
}

func (s *ExchangeRateRepositoryTestSuite) TestSet_Overwrite() {
	ctx := context.Background()

	// Arrange - сохраняем первое значение
	rate1 := &entity.ExchangeRate{
		Currency:  "RUB",
		Rate:      90.0,
		UpdatedAt: time.Now(),
	}
	s.repo.Set(ctx, rate1)

	// Act - перезаписываем
	rate2 := &entity.ExchangeRate{
		Currency:  "RUB",
		Rate:      91.23,
		UpdatedAt: time.Now(),
	}
	err := s.repo.Set(ctx, rate2)

	// Assert
	s.NoError(err)
	result, _ := s.repo.Get(ctx, "RUB")
	s.Equal(91.23, result.Rate)
}

// ===================== SetMultiple Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestSetMultiple_Success() {
	ctx := context.Background()

	rates := []*entity.ExchangeRate{
		{Currency: "USD", Rate: 1.0, UpdatedAt: time.Now()},
		{Currency: "EUR", Rate: 0.93, UpdatedAt: time.Now()},
		{Currency: "RUB", Rate: 91.23, UpdatedAt: time.Now()},
	}

	// Act
	err := s.repo.SetMultiple(ctx, rates)

	// Assert
	s.NoError(err)

	// Проверяем все сохранились
	for _, expected := range rates {
		result, err := s.repo.Get(ctx, expected.Currency)
		s.NoError(err)
		s.Equal(expected.Rate, result.Rate)
	}
}

func (s *ExchangeRateRepositoryTestSuite) TestSetMultiple_Empty() {
	ctx := context.Background()

	// Act
	err := s.repo.SetMultiple(ctx, []*entity.ExchangeRate{})

	// Assert
	s.NoError(err)
}

// ===================== GetMultiple Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestGetMultiple_Success() {
	ctx := context.Background()

	// Arrange
	rates := []*entity.ExchangeRate{
		{Currency: "USD", Rate: 1.0, UpdatedAt: time.Now()},
		{Currency: "EUR", Rate: 0.93, UpdatedAt: time.Now()},
		{Currency: "RUB", Rate: 91.23, UpdatedAt: time.Now()},
	}
	s.repo.SetMultiple(ctx, rates)

	// Act
	result, err := s.repo.GetMultiple(ctx, []string{"USD", "EUR", "RUB"})

	// Assert
	s.NoError(err)
	s.Len(result, 3)
	s.Equal(1.0, result["USD"].Rate)
	s.Equal(0.93, result["EUR"].Rate)
	s.Equal(91.23, result["RUB"].Rate)
}

func (s *ExchangeRateRepositoryTestSuite) TestGetMultiple_Partial() {
	ctx := context.Background()

	// Arrange - сохраняем только USD
	rate := &entity.ExchangeRate{Currency: "USD", Rate: 1.0, UpdatedAt: time.Now()}
	s.repo.Set(ctx, rate)

	// Act - запрашиваем USD и EUR (EUR не существует)
	result, err := s.repo.GetMultiple(ctx, []string{"USD", "EUR"})

	// Assert
	s.NoError(err)
	s.Len(result, 1)
	s.Equal(1.0, result["USD"].Rate)
	_, hasEUR := result["EUR"]
	s.False(hasEUR)
}

func (s *ExchangeRateRepositoryTestSuite) TestGetMultiple_AllMissing() {
	ctx := context.Background()

	// Act
	result, err := s.repo.GetMultiple(ctx, []string{"ABC", "XYZ"})

	// Assert
	s.NoError(err)
	s.Empty(result)
}

// ===================== Exists Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestExists_True() {
	ctx := context.Background()

	// Arrange
	rate := &entity.ExchangeRate{Currency: "USD", Rate: 1.0, UpdatedAt: time.Now()}
	s.repo.Set(ctx, rate)

	// Act
	exists, err := s.repo.Exists(ctx, "USD")

	// Assert
	s.NoError(err)
	s.True(exists)
}

func (s *ExchangeRateRepositoryTestSuite) TestExists_False() {
	ctx := context.Background()

	// Act
	exists, err := s.repo.Exists(ctx, "XYZ")

	// Assert
	s.NoError(err)
	s.False(exists)
}

// ===================== TTL Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestTTL_Expiration() {
	// Создаём repository с очень коротким TTL
	shortTTLRepo := NewExchangeRateRepository(s.client, 1*time.Second)
	ctx := context.Background()

	rate := &entity.ExchangeRate{Currency: "TTL_TEST", Rate: 1.0, UpdatedAt: time.Now()}
	err := shortTTLRepo.Set(ctx, rate)
	assert.NoError(s.T(), err)

	// Проверяем что сохранилось
	result, err := shortTTLRepo.Get(ctx, "TTL_TEST")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), result)

	// Ждём истечения TTL (miniredis поддерживает FastForward)
	s.miniRedis.FastForward(2 * time.Second)

	// Проверяем что истекло
	result, err = shortTTLRepo.Get(ctx, "TTL_TEST")
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

// ===================== Redis Key Format Tests =====================

func (s *ExchangeRateRepositoryTestSuite) TestRedisKeyFormat() {
	ctx := context.Background()

	rate := &entity.ExchangeRate{Currency: "GBP", Rate: 0.79, UpdatedAt: time.Now()}
	s.repo.Set(ctx, rate)

	// Проверяем что ключ имеет правильный формат: rates:GBP
	keys, err := s.client.Keys(ctx, "rates:*").Result()
	s.NoError(err)
	s.Contains(keys, "rates:GBP")
}
