package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/service"
	"augustberries/pkg/metrics"

	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader      *kafka.Reader
	orderSvc    service.OrderProcessingServiceInterface
	exchangeSvc service.ExchangeRateServiceInterface
	topic       string
	groupID     string
	stopChan    chan struct{}
	doneChan    chan struct{}
}

func NewKafkaConsumer(
	brokers []string,
	topic string,
	groupID string,
	minBytes int,
	maxBytes int,
	orderSvc service.OrderProcessingServiceInterface,
	exchangeSvc service.ExchangeRateServiceInterface,
) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       minBytes,
		MaxBytes:       maxBytes,
		StartOffset:    kafka.LastOffset,
		CommitInterval: time.Second,
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 1 * time.Second,
	})

	return &KafkaConsumer{
		reader:      reader,
		orderSvc:    orderSvc,
		exchangeSvc: exchangeSvc,
		topic:       topic,
		groupID:     groupID,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
	}
}

func (c *KafkaConsumer) Start(ctx context.Context) {
	log.Println("Starting Kafka consumer...")

	if err := c.exchangeSvc.EnsureRatesAvailable(ctx); err != nil {
		log.Printf("WARNING: Failed to ensure exchange rates available: %v", err)
	}

	go c.consume(ctx)
}

func (c *KafkaConsumer) Stop() {
	log.Println("Stopping Kafka consumer...")
	close(c.stopChan)
	<-c.doneChan
	c.reader.Close()
	log.Println("Kafka consumer stopped")
}

func (c *KafkaConsumer) consume(ctx context.Context) {
	defer close(c.doneChan)

	for {
		select {
		case <-c.stopChan:
			return
		default:
			readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			message, err := c.reader.FetchMessage(readCtx)
			cancel()

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Error fetching message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if err := c.processMessage(ctx, message); err != nil {
				log.Printf("Error processing message: %v", err)
				metrics.KafkaErrors.WithLabelValues("background-worker", c.topic, "consume").Inc()
			} else {
				if err := c.reader.CommitMessages(ctx, message); err != nil {
					log.Printf("Error committing message: %v", err)
				}
			}
		}
	}
}

func (c *KafkaConsumer) processMessage(ctx context.Context, message kafka.Message) error {
	start := time.Now()

	var event entity.OrderEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal order event: %w", err)
	}

	log.Printf("Received %s event for order %s", event.EventType, event.OrderID)

	if err := c.orderSvc.ProcessOrderEvent(ctx, &event); err != nil {
		metrics.WorkerOrdersProcessed.WithLabelValues("failed").Inc()
		return fmt.Errorf("failed to process order event: %w", err)
	}

	metrics.KafkaMessagesConsumed.WithLabelValues("background-worker", c.topic, c.groupID).Inc()
	metrics.KafkaConsumeDuration.WithLabelValues("background-worker", c.topic).Observe(time.Since(start).Seconds())
	metrics.WorkerOrdersProcessed.WithLabelValues("success").Inc()
	metrics.WorkerProcessingDuration.Observe(time.Since(start).Seconds())

	return nil
}

func (c *KafkaConsumer) GetStats() kafka.ReaderStats {
	return c.reader.Stats()
}
