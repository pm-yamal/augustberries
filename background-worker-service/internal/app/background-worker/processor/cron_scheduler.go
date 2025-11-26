package processor

import (
	"context"
	"log"

	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/robfig/cron/v3"
)

// CronScheduler управляет периодическими задачами
type CronScheduler struct {
	cron        *cron.Cron
	exchangeSvc service.ExchangeRateServiceInterface
}

// NewCronScheduler создает новый планировщик задач
func NewCronScheduler(exchangeSvc service.ExchangeRateServiceInterface) *CronScheduler {
	// Создаем cron с логированием
	c := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.Default())))

	return &CronScheduler{
		cron:        c,
		exchangeSvc: exchangeSvc,
	}
}

// Start запускает планировщик задач
func (s *CronScheduler) Start(ctx context.Context, schedule string) error {
	log.Printf("Starting cron scheduler with schedule: %s", schedule)

	// Добавляем задачу обновления курсов валют
	_, err := s.cron.AddFunc(schedule, func() {
		log.Println("Cron job triggered: updating exchange rates")

		if err := s.exchangeSvc.FetchAndStoreRates(ctx); err != nil {
			log.Printf("ERROR: Failed to update exchange rates: %v", err)
		} else {
			log.Println("Cron job completed: exchange rates updated successfully")
		}
	})

	if err != nil {
		return err
	}

	// Запускаем планировщик
	s.cron.Start()
	log.Println("Cron scheduler started")

	// Выполняем первое обновление курсов сразу при старте
	log.Println("Performing initial exchange rates update...")
	if err := s.exchangeSvc.FetchAndStoreRates(ctx); err != nil {
		log.Printf("WARNING: Failed initial exchange rates update: %v", err)
	} else {
		log.Println("Initial exchange rates update completed")
	}

	return nil
}

// Stop останавливает планировщик задач
func (s *CronScheduler) Stop() {
	log.Println("Stopping cron scheduler...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Cron scheduler stopped")
}

// GetEntries возвращает список запланированных задач
func (s *CronScheduler) GetEntries() []cron.Entry {
	return s.cron.Entries()
}
