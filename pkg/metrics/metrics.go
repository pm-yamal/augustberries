package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP Metrics

var HttpRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	},
	[]string{"service", "method", "path", "status"},
)

var HttpRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	},
	[]string{"service", "method", "path"},
)

var HttpRequestsInFlight = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Current number of HTTP requests being processed",
	},
	[]string{"service"},
)

// Database Metrics

var DbQueryDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Duration of database queries in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	},
	[]string{"service", "operation", "table"},
)

var DbConnectionsOpen = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_connections_open",
		Help: "Number of open database connections",
	},
	[]string{"service", "state"},
)

var DbErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "db_errors_total",
		Help: "Total number of database errors",
	},
	[]string{"service", "operation"},
)

// Redis Metrics

var RedisCacheHits = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_cache_hits_total",
		Help: "Total number of Redis cache hits",
	},
	[]string{"service", "key_prefix"},
)

var RedisCacheMisses = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_cache_misses_total",
		Help: "Total number of Redis cache misses",
	},
	[]string{"service", "key_prefix"},
)

var RedisOperationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "redis_operation_duration_seconds",
		Help:    "Duration of Redis operations in seconds",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
	},
	[]string{"service", "operation"},
)

var RedisErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "redis_errors_total",
		Help: "Total number of Redis errors",
	},
	[]string{"service", "operation"},
)

// Kafka Metrics

var KafkaMessagesProduced = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_produced_total",
		Help: "Total number of Kafka messages produced",
	},
	[]string{"service", "topic"},
)

var KafkaMessagesConsumed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "Total number of Kafka messages consumed",
	},
	[]string{"service", "topic", "group"},
)

var KafkaProduceDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "kafka_produce_duration_seconds",
		Help:    "Duration of Kafka produce operations",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	},
	[]string{"service", "topic"},
)

var KafkaConsumeDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "kafka_consume_duration_seconds",
		Help:    "Duration of Kafka message processing",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10},
	},
	[]string{"service", "topic"},
)

var KafkaErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_errors_total",
		Help: "Total number of Kafka errors",
	},
	[]string{"service", "topic", "operation"},
)

// Auth Service Metrics

var AuthRegistrations = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "auth_registrations_total",
		Help: "Total number of user registrations",
	},
)

var AuthLogins = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_logins_total",
		Help: "Total number of login attempts",
	},
	[]string{"status"},
)

var AuthTokensIssued = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_tokens_issued_total",
		Help: "Total number of tokens issued",
	},
	[]string{"type"},
)

// Orders Service Metrics

var OrdersCreated = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "orders_created_total",
		Help: "Total number of orders created",
	},
)

var OrdersTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "orders_total_amount",
		Help: "Total amount of all orders",
	},
)

var OrdersByStatus = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "orders_by_status",
		Help: "Number of orders by status",
	},
	[]string{"status"},
)

// Reviews Service Metrics

var ReviewsCreated = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "reviews_created_total",
		Help: "Total number of reviews created",
	},
)

var ReviewsRating = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "reviews_rating",
		Help:    "Distribution of review ratings",
		Buckets: []float64{1, 2, 3, 4, 5},
	},
	[]string{},
)

// Background Worker Metrics

var WorkerOrdersProcessed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "worker_orders_processed_total",
		Help: "Total number of orders processed by worker",
	},
	[]string{"status"},
)

var WorkerExchangeRateUpdates = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "worker_exchange_rate_updates_total",
		Help: "Total number of exchange rate updates",
	},
	[]string{"status"},
)

var WorkerProcessingDuration = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "worker_order_processing_duration_seconds",
		Help:    "Duration of order processing in worker",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	},
)
