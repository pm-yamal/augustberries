package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// =============================================================================
// HTTP Метрики (общие для всех сервисов)
// =============================================================================

// HttpRequestsTotal - счётчик всех HTTP запросов
// Labels: service, method, path, status
// Пример запроса PromQL: rate(http_requests_total{service="orders"}[5m])
var HttpRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	},
	[]string{"service", "method", "path", "status"},
)

// HttpRequestDuration - гистограмма времени ответа (latency_seconds из ТЗ)
// Labels: service, method, path
// Пример: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
var HttpRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of HTTP requests in seconds",
		// Бакеты для микросервисов: от 1ms до 10s
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	},
	[]string{"service", "method", "path"},
)

// HttpRequestsInFlight - текущее количество обрабатываемых запросов
var HttpRequestsInFlight = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Current number of HTTP requests being processed",
	},
	[]string{"service"},
)

// =============================================================================
// Database Метрики
// =============================================================================

// DbQueryDuration - время выполнения SQL запросов
var DbQueryDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Duration of database queries in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	},
	[]string{"service", "operation", "table"},
)

// DbConnectionsOpen - количество открытых соединений с БД
var DbConnectionsOpen = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_connections_open",
		Help: "Number of open database connections",
	},
	[]string{"service", "state"}, // state: idle, in_use
)

// DbErrors - счётчик ошибок базы данных
var DbErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "db_errors_total",
		Help: "Total number of database errors",
	},
	[]string{"service", "operation"},
)

// =============================================================================
// Redis Метрики (redis_ops из ТЗ)
// =============================================================================

// RedisCacheHits - попадания в кеш
var RedisCacheHits = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_cache_hits_total",
		Help: "Total number of Redis cache hits",
	},
	[]string{"service", "key_prefix"},
)

// RedisCacheMisses - промахи кеша
var RedisCacheMisses = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_cache_misses_total",
		Help: "Total number of Redis cache misses",
	},
	[]string{"service", "key_prefix"},
)

// RedisOperationDuration - время операций Redis
var RedisOperationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "redis_operation_duration_seconds",
		Help:    "Duration of Redis operations in seconds",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
	},
	[]string{"service", "operation"}, // operation: get, set, del, etc.
)

// RedisErrors - ошибки Redis
var RedisErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_errors_total",
		Help: "Total number of Redis errors",
	},
	[]string{"service", "operation"},
)

// =============================================================================
// Kafka Метрики (kafka_lag из ТЗ)
// =============================================================================

// KafkaMessagesProduced - отправленные сообщения
var KafkaMessagesProduced = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_produced_total",
		Help: "Total number of Kafka messages produced",
	},
	[]string{"service", "topic"},
)

// KafkaMessagesConsumed - полученные сообщения
var KafkaMessagesConsumed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "Total number of Kafka messages consumed",
	},
	[]string{"service", "topic", "group"},
)

// KafkaProduceDuration - время отправки сообщения
var KafkaProduceDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "kafka_produce_duration_seconds",
		Help:    "Duration of Kafka produce operations",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	},
	[]string{"service", "topic"},
)

// KafkaConsumeDuration - время обработки сообщения
var KafkaConsumeDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "kafka_consume_duration_seconds",
		Help:    "Duration of Kafka message processing",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10},
	},
	[]string{"service", "topic"},
)

// KafkaErrors - ошибки Kafka
var KafkaErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_errors_total",
		Help: "Total number of Kafka errors",
	},
	[]string{"service", "topic", "operation"}, // operation: produce, consume
)

// =============================================================================
// Business Метрики (специфичные для Augustberries)
// =============================================================================

// --- Auth Service ---

// AuthRegistrations - регистрации пользователей
var AuthRegistrations = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "auth_registrations_total",
		Help: "Total number of user registrations",
	},
)

// AuthLogins - попытки входа
var AuthLogins = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_logins_total",
		Help: "Total number of login attempts",
	},
	[]string{"status"}, // success, failed, blocked
)

// AuthTokensIssued - выданные токены
var AuthTokensIssued = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_tokens_issued_total",
		Help: "Total number of tokens issued",
	},
	[]string{"type"}, // access, refresh
)

// --- Orders Service ---

// OrdersCreated - созданные заказы
var OrdersCreated = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "orders_created_total",
		Help: "Total number of orders created",
	},
)

// OrdersTotal - общая сумма заказов
var OrdersTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "orders_total_amount",
		Help: "Total amount of all orders",
	},
)

// OrdersByStatus - заказы по статусам
var OrdersByStatus = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "orders_by_status",
		Help: "Number of orders by status",
	},
	[]string{"status"}, // pending, processing, shipped, delivered, cancelled
)

// --- Reviews Service ---

// ReviewsCreated - созданные отзывы
var ReviewsCreated = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "reviews_created_total",
		Help: "Total number of reviews created",
	},
)

// ReviewsRating - распределение оценок
var ReviewsRating = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "reviews_rating",
		Help:    "Distribution of review ratings",
		Buckets: []float64{1, 2, 3, 4, 5},
	},
	[]string{},
)

// --- Background Worker ---

// WorkerOrdersProcessed - обработанные заказы
var WorkerOrdersProcessed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "worker_orders_processed_total",
		Help: "Total number of orders processed by worker",
	},
	[]string{"status"}, // success, failed
)

// WorkerExchangeRateUpdates - обновления курсов валют
var WorkerExchangeRateUpdates = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "worker_exchange_rate_updates_total",
		Help: "Total number of exchange rate updates",
	},
	[]string{"status"}, // success, failed
)

// WorkerProcessingDuration - время обработки заказа
var WorkerProcessingDuration = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "worker_order_processing_duration_seconds",
		Help:    "Duration of order processing in worker",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	},
)
