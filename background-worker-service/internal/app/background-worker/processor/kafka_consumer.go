package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/segmentio/kafka-go"
)

// KafkaConsumer обрабатывает события из Kafka топика order_events
type KafkaConsumer struct {
	reader      *kafka.Reader
	orderSvc    service.OrderProcessingServiceInterface
	exchangeSvc service.ExchangeRateServiceInterface
	stopChan    chan struct{}
	doneChan    chan struct{}
}

// NewKafkaConsumer создает новый Kafka consumer
func NewKafkaConsumer(
	brokers []string,
	topic string,
	groupID string,
	minBytes int,
	maxBytes int,
	orderSvc service.OrderProcessingServiceInterface,
	exchangeSvc service.ExchangeRateServiceInterface,
) *KafkaConsumer {
	// Настраиваем Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    minBytes,         // Минимум байт для fetch запроса
		MaxBytes:    maxBytes,         // Максимум байт для fetch запроса
		StartOffset: kafka.LastOffset, // Начинаем читать с последнего сообщения
		// Настройки для автоматического коммита offset
		CommitInterval: time.Second,
		// Таймауты
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 1 * time.Second,
	})

	return &KafkaConsumer{
		reader:      reader,
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
	}
}

// Start запускает consumer в отдельной горутине
func (c *KafkaConsumer) Start(ctx context.Context) {
	log.Println("Starting Kafka consumer...")

	// Убеждаемся что курсы валют доступны перед началом обработки
	if err := c.exchangeSvc.EnsureRatesAvailable(ctx); err != nil {
		log.Printf("WARNING: Failed to ensure exchange rates available: %v", err)
		log.Println("Will try to fetch rates on first order processing")
	}

	go c.consume(ctx)
}

// Stop останавливает consumer
func (c *KafkaConsumer) Stop() {
	log.Println("Stopping Kafka consumer...")
	close(c.stopChan)
	<-c.doneChan
	c.reader.Close()
	log.Println("Kafka consumer stopped")
}

// consume читает и обрабатывает сообщения из Kafka
func (c *KafkaConsumer) consume(ctx context.Context) {
	defer close(c.doneChan)

	for {
		select {
		case <-c.stopChan:
			return
		default:
			// Читаем сообщение с таймаутом
			readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			message, err := c.reader.FetchMessage(readCtx)
			cancel()

			if err != nil {
				// Если контекст был отменен, выходим
				if ctx.Err() != nil {
					return
				}

				// Логируем ошибку и продолжаем
				log.Printf("Error fetching message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			// Обрабатываем сообщение
			if err := c.processMessage(ctx, message); err != nil {
				log.Printf("Error processing message: %v", err)
				// Не коммитим offset при ошибке - сообщение будет повторно обработано
			} else {
				// Коммитим offset после успешной обработки
				if err := c.reader.CommitMessages(ctx, message); err != nil {
					log.Printf("Error committing message: %v", err)
				}
			}
		}
	}
}

// processMessage обрабатывает одно сообщение из Kafka
func (c *KafkaConsumer) processMessage(ctx context.Context, message kafka.Message) error {
	// Парсим событие заказа
	var event entity.OrderEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal order event: %w", err)
	}

	log.Printf("Received %s event for order %s (offset: %d, partition: %d)",
		event.EventType, event.OrderID, message.Offset, message.Partition)

	// Обрабатываем событие
	if err := c.orderSvc.ProcessOrderEvent(ctx, &event); err != nil {
		return fmt.Errorf("failed to process order event: %w", err)
	}

	return nil
}

// GetStats возвращает статистику consumer
func (c *KafkaConsumer) GetStats() kafka.ReaderStats {
	return c.reader.Stats()
}
