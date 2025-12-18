package processor

import (
	"context"
	"log"

	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/robfig/cron/v3"
)

type CronScheduler struct {
	cron        *cron.Cron
	exchangeSvc service.ExchangeRateServiceInterface
}

func NewCronScheduler(exchangeSvc service.ExchangeRateServiceInterface) *CronScheduler {
	c := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.Default())))

	return &CronScheduler{
		cron:        c,
		exchangeSvc: exchangeSvc,
	}
}

func (s *CronScheduler) Start(ctx context.Context, schedule string) error {
	log.Printf("Starting cron scheduler with schedule: %s", schedule)

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

	s.cron.Start()
	log.Println("Cron scheduler started")

	log.Println("Performing initial exchange rates update...")
	if err := s.exchangeSvc.FetchAndStoreRates(ctx); err != nil {
		log.Printf("WARNING: Failed initial exchange rates update: %v", err)
	} else {
		log.Println("Initial exchange rates update completed")
	}

	return nil
}

func (s *CronScheduler) Stop() {
	log.Println("Stopping cron scheduler...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Cron scheduler stopped")
}

func (s *CronScheduler) GetEntries() []cron.Entry {
	return s.cron.Entries()
}
