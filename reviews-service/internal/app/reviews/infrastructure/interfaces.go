package infrastructure

import "context"

// MessagePublisher интерфейс для отправки сообщений в очередь (Kafka)
// Используется для dependency injection и упрощения тестирования
type MessagePublisher interface {
	PublishMessage(ctx context.Context, key string, value []byte) error
	Close() error
}
