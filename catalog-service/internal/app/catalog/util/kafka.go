package util

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer обертка над Kafka writer для отправки событий
// Используется для отправки событий PRODUCT_UPDATED в топик product_events
type KafkaProducer struct {
	writer *kafka.Writer // Writer для асинхронной отправки сообщений в Kafka
}

// NewKafkaProducer создает новый Kafka producer
// brokers - список брокеров Kafka в формате ["host:port"]
// topic - имя топика для отправки событий (product_events согласно заданию)
func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:  kafka.TCP(brokers...), // Адреса брокеров Kafka
		Topic: topic,                 // Топик для событий о товарах
		// Балансировка по наименьшему количеству байт для равномерного распределения
		Balancer: &kafka.LeastBytes{},
		// Настройки для production окружения
		BatchSize:    100,              // Размер батча сообщений
		BatchTimeout: 10 * time.Second, // Таймаут батча
		// Async: true можно добавить для асинхронной отправки
	}

	return &KafkaProducer{writer: writer}
}

// PublishMessage отправляет сообщение в Kafka
// key - используется для партиционирования (обычно ProductID для сохранения порядка)
// value - JSON сериализованное событие ProductEvent
// Возвращает ошибку если не удалось отправить сообщение
func (p *KafkaProducer) PublishMessage(ctx context.Context, key string, value []byte) error {
	message := kafka.Message{
		Key:   []byte(key), // Ключ для партиционирования (обеспечивает порядок для одного продукта)
		Value: value,       // Тело сообщения (JSON с информацией о событии)
		Time:  time.Now(),  // Временная метка сообщения
	}

	// Отправляем сообщение в Kafka с контекстом для отмены
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

// PublishMessages отправляет несколько сообщений батчем
func (p *KafkaProducer) PublishMessages(ctx context.Context, messages []kafka.Message) error {
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("failed to write messages to kafka: %w", err)
	}
	return nil
}

// Close закрывает Kafka writer и освобождает ресурсы
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
