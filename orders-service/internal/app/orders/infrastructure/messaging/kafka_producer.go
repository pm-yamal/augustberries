package messaging

import (
	"context"
	"fmt"
	"time"

	"augustberries/pkg/metrics"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Second,
	}

	return &KafkaProducer{writer: writer, topic: topic}
}

func (p *KafkaProducer) PublishMessage(ctx context.Context, key string, value []byte) error {
	start := time.Now()

	message := kafka.Message{
		Key:   []byte(key),
		Value: value,
		Time:  time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, message); err != nil {
		metrics.KafkaErrors.WithLabelValues("orders-service", p.topic, "produce").Inc()
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	metrics.KafkaMessagesProduced.WithLabelValues("orders-service", p.topic).Inc()
	metrics.KafkaProduceDuration.WithLabelValues("orders-service", p.topic).Observe(time.Since(start).Seconds())

	return nil
}

func (p *KafkaProducer) PublishMessages(ctx context.Context, messages []kafka.Message) error {
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("failed to write messages to kafka: %w", err)
	}
	return nil
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
