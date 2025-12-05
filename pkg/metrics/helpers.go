package metrics

import (
	"context"
	"time"
)

// =============================================================================
// Redis Instrumentation Helpers
// =============================================================================

// RedisOperation представляет тип операции Redis
type RedisOperation string

const (
	RedisOpGet    RedisOperation = "get"
	RedisOpSet    RedisOperation = "set"
	RedisOpDel    RedisOperation = "del"
	RedisOpExists RedisOperation = "exists"
	RedisOpExpire RedisOperation = "expire"
	RedisOpHGet   RedisOperation = "hget"
	RedisOpHSet   RedisOperation = "hset"
)

// RedisTimer помогает измерять время операций Redis
type RedisTimer struct {
	service   string
	operation RedisOperation
	start     time.Time
}

// NewRedisTimer создаёт новый таймер для Redis операции
func NewRedisTimer(service string, op RedisOperation) *RedisTimer {
	return &RedisTimer{
		service:   service,
		operation: op,
		start:     time.Now(),
	}
}

// ObserveDuration записывает время выполнения операции
func (rt *RedisTimer) ObserveDuration() {
	duration := time.Since(rt.start).Seconds()
	RedisOperationDuration.WithLabelValues(rt.service, string(rt.operation)).Observe(duration)
}

// RecordCacheHit записывает попадание в кеш
func RecordCacheHit(service, keyPrefix string) {
	RedisCacheHits.WithLabelValues(service, keyPrefix).Inc()
}

// RecordCacheMiss записывает промах кеша
func RecordCacheMiss(service, keyPrefix string) {
	RedisCacheMisses.WithLabelValues(service, keyPrefix).Inc()
}

// RecordRedisError записывает ошибку Redis
func RecordRedisError(service string, op RedisOperation) {
	RedisErrors.WithLabelValues(service, string(op)).Inc()
}

// =============================================================================
// Kafka Instrumentation Helpers
// =============================================================================

// RecordKafkaMessageProduced записывает отправку сообщения в Kafka
func RecordKafkaMessageProduced(service, topic string, duration time.Duration) {
	KafkaMessagesProduced.WithLabelValues(service, topic).Inc()
	KafkaProduceDuration.WithLabelValues(service, topic).Observe(duration.Seconds())
}

// RecordKafkaMessageConsumed записывает получение сообщения из Kafka
func RecordKafkaMessageConsumed(service, topic, group string, processingDuration time.Duration) {
	KafkaMessagesConsumed.WithLabelValues(service, topic, group).Inc()
	KafkaConsumeDuration.WithLabelValues(service, topic).Observe(processingDuration.Seconds())
}

// RecordKafkaError записывает ошибку Kafka
func RecordKafkaError(service, topic, operation string) {
	KafkaErrors.WithLabelValues(service, topic, operation).Inc()
}

// KafkaProduceTimer помогает измерять время отправки в Kafka
type KafkaProduceTimer struct {
	service string
	topic   string
	start   time.Time
}

// NewKafkaProduceTimer создаёт таймер для produce операции
func NewKafkaProduceTimer(service, topic string) *KafkaProduceTimer {
	return &KafkaProduceTimer{
		service: service,
		topic:   topic,
		start:   time.Now(),
	}
}

// Success записывает успешную отправку
func (kt *KafkaProduceTimer) Success() {
	RecordKafkaMessageProduced(kt.service, kt.topic, time.Since(kt.start))
}

// Error записывает ошибку отправки
func (kt *KafkaProduceTimer) Error() {
	RecordKafkaError(kt.service, kt.topic, "produce")
}

// =============================================================================
// Database Instrumentation Helpers
// =============================================================================

// DbOperation представляет тип операции с БД
type DbOperation string

const (
	DbOpSelect DbOperation = "select"
	DbOpInsert DbOperation = "insert"
	DbOpUpdate DbOperation = "update"
	DbOpDelete DbOperation = "delete"
)

// DbTimer помогает измерять время запросов к БД
type DbTimer struct {
	service   string
	operation DbOperation
	table     string
	start     time.Time
}

// NewDbTimer создаёт новый таймер для DB операции
func NewDbTimer(service string, op DbOperation, table string) *DbTimer {
	return &DbTimer{
		service:   service,
		operation: op,
		table:     table,
		start:     time.Now(),
	}
}

// ObserveDuration записывает время выполнения запроса
func (dt *DbTimer) ObserveDuration() {
	duration := time.Since(dt.start).Seconds()
	DbQueryDuration.WithLabelValues(dt.service, string(dt.operation), dt.table).Observe(duration)
}

// RecordDbError записывает ошибку БД
func RecordDbError(service string, op DbOperation) {
	DbErrors.WithLabelValues(service, string(op)).Inc()
}

// =============================================================================
// Generic Timer (для произвольных операций)
// =============================================================================

// Timer общий таймер для измерения операций
type Timer struct {
	start time.Time
}

// NewTimer создаёт новый таймер
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// Duration возвращает прошедшее время
func (t *Timer) Duration() time.Duration {
	return time.Since(t.start)
}

// Seconds возвращает прошедшее время в секундах
func (t *Timer) Seconds() float64 {
	return time.Since(t.start).Seconds()
}

// =============================================================================
// Context-based instrumentation
// =============================================================================

type contextKey string

const timerContextKey contextKey = "metrics_timer"

// StartTimer добавляет таймер в контекст
func StartTimer(ctx context.Context) context.Context {
	return context.WithValue(ctx, timerContextKey, NewTimer())
}

// GetDuration получает время из контекста
func GetDuration(ctx context.Context) time.Duration {
	if timer, ok := ctx.Value(timerContextKey).(*Timer); ok {
		return timer.Duration()
	}
	return 0
}
