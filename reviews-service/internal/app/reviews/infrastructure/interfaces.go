package infrastructure

import "context"

type MessagePublisher interface {
	PublishMessage(ctx context.Context, key string, value []byte) error
	Close() error
}
