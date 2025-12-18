package metrics

import (
	"context"
	"time"
)


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

type RedisTimer struct {
	service   string
	operation RedisOperation
	start     time.Time
}

func NewRedisTimer(service string, op RedisOperation) *RedisTimer {
	return &RedisTimer{
		service:   service,
		operation: op,
		start:     time.Now(),
	}
}

func (rt *RedisTimer) ObserveDuration() {
	duration := time.Since(rt.start).Seconds()
	RedisOperationDuration.WithLabelValues(rt.service, string(rt.operation)).Observe(duration)
}

func RecordCacheHit(service, keyPrefix string) {
	RedisCacheHits.WithLabelValues(service, keyPrefix).Inc()
}

func RecordCacheMiss(service, keyPrefix string) {
	RedisCacheMisses.WithLabelValues(service, keyPrefix).Inc()
}

func RecordRedisError(service string, op RedisOperation) {
	RedisErrors.WithLabelValues(service, string(op)).Inc()
}


func RecordKafkaMessageProduced(service, topic string, duration time.Duration) {
	KafkaMessagesProduced.WithLabelValues(service, topic).Inc()
	KafkaProduceDuration.WithLabelValues(service, topic).Observe(duration.Seconds())
}

func RecordKafkaMessageConsumed(service, topic, group string, processingDuration time.Duration) {
	KafkaMessagesConsumed.WithLabelValues(service, topic, group).Inc()
	KafkaConsumeDuration.WithLabelValues(service, topic).Observe(processingDuration.Seconds())
}

func RecordKafkaError(service, topic, operation string) {
	KafkaErrors.WithLabelValues(service, topic, operation).Inc()
}

type KafkaProduceTimer struct {
	service string
	topic   string
	start   time.Time
}

func NewKafkaProduceTimer(service, topic string) *KafkaProduceTimer {
	return &KafkaProduceTimer{
		service: service,
		topic:   topic,
		start:   time.Now(),
	}
}

func (kt *KafkaProduceTimer) Success() {
	RecordKafkaMessageProduced(kt.service, kt.topic, time.Since(kt.start))
}

func (kt *KafkaProduceTimer) Error() {
	RecordKafkaError(kt.service, kt.topic, "produce")
}


type DbOperation string

const (
	DbOpSelect DbOperation = "select"
	DbOpInsert DbOperation = "insert"
	DbOpUpdate DbOperation = "update"
	DbOpDelete DbOperation = "delete"
)

type DbTimer struct {
	service   string
	operation DbOperation
	table     string
	start     time.Time
}

func NewDbTimer(service string, op DbOperation, table string) *DbTimer {
	return &DbTimer{
		service:   service,
		operation: op,
		table:     table,
		start:     time.Now(),
	}
}

func (dt *DbTimer) ObserveDuration() {
	duration := time.Since(dt.start).Seconds()
	DbQueryDuration.WithLabelValues(dt.service, string(dt.operation), dt.table).Observe(duration)
}

func RecordDbError(service string, op DbOperation) {
	DbErrors.WithLabelValues(service, string(op)).Inc()
}


type Timer struct {
	start time.Time
}

func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

func (t *Timer) Duration() time.Duration {
	return time.Since(t.start)
}

func (t *Timer) Seconds() float64 {
	return time.Since(t.start).Seconds()
}


type contextKey string

const timerContextKey contextKey = "metrics_timer"

func StartTimer(ctx context.Context) context.Context {
	return context.WithValue(ctx, timerContextKey, NewTimer())
}

func GetDuration(ctx context.Context) time.Duration {
	if timer, ok := ctx.Value(timerContextKey).(*Timer); ok {
		return timer.Duration()
	}
	return 0
}
