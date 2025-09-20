package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/config"
	kafkax "github.com/samirwankhede/lewly-pgpyewj/internal/kafka"
	"github.com/samirwankhede/lewly-pgpyewj/internal/logger"
	"github.com/samirwankhede/lewly-pgpyewj/internal/mailer"
	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	mailerService "github.com/samirwankhede/lewly-pgpyewj/internal/service/mailer"
	workerService "github.com/samirwankhede/lewly-pgpyewj/internal/service/worker"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
	storeBookings "github.com/samirwankhede/lewly-pgpyewj/internal/store/bookings"
	storeEvents "github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
	storeUsers "github.com/samirwankhede/lewly-pgpyewj/internal/store/users"
	storeWaitlist "github.com/samirwankhede/lewly-pgpyewj/internal/store/waitlist"
	"github.com/samirwankhede/lewly-pgpyewj/internal/worker"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	log := logger.New(cfg.Env)
	log.Info("worker starting")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	bookingTimeoutStore := redisx.NewTimeoutBucket(cfg.RedisAddr)
	db, err := store.NewDB(ctx, cfg.PostgresURL, int32(cfg.MaxDBConnections))
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	// Create repositories
	bookingsRepo := storeBookings.NewBookingsRepository(db, log)
	eventsRepo := storeEvents.NewEventsRepository(db, log)
	waitlistRepo := storeWaitlist.NewWaitlistRepository(db, log)
	usersRepository := storeUsers.NewUsersRepository(db, log)

	// Create mailer service
	mailerSender := &mailer.SMTPSender{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	}
	mailerSvc := mailerService.NewMailerService(log, mailerSender)

	// Create finalize service
	finalizeSvc := workerService.NewFinalizeService(log, bookingsRepo, eventsRepo, usersRepository, waitlistRepo, cfg.PaymentURL, mailerSvc, bookingTimeoutStore)

	// Create Kafka consumer and producer
	consumer := kafkax.NewConsumer([]string{cfg.KafkaBrokers}, "evently-finalizer", "bookings")
	defer consumer.Close()
	dlq := kafkax.NewProducer([]string{cfg.KafkaBrokers}, "bookings-dlq")
	defer dlq.Close()

	// Create and run finalizer
	f := worker.NewFinalizer(log, finalizeSvc, consumer, dlq, cfg.MaxWorkerRoutineCount)
	_ = f.Run(ctx)

	<-ctx.Done()
	log.Info("worker stopped")
}
