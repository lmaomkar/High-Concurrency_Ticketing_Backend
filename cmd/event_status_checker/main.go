package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/config"
	"github.com/samirwankhede/lewly-pgpyewj/internal/logger"
	"github.com/samirwankhede/lewly-pgpyewj/internal/service/events"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
	eventsrepo "github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	log := logger.New(cfg.Env)
	ctx := context.Background()

	// Connect to database
	db, err := store.NewDB(ctx, cfg.PostgresURL, int32(cfg.MaxDBConnections))
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Create events repository
	eventsRepo := eventsrepo.NewEventsRepository(db, log)

	// Create event status checker
	statusChecker := events.NewEventStatusChecker(log, eventsRepo)

	// Run initial check
	log.Info("Running initial expired events check")
	_, err = statusChecker.CheckAndUpdateExpiredEvents(ctx)
	if err != nil {
		log.Error("Initial check failed", zap.Error(err))
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start periodic checking (every 5 minutes)
	checkInterval := 5 * time.Minute
	go statusChecker.RunPeriodicCheck(ctx, checkInterval)

	log.Info("Event status checker started", zap.Duration("check_interval", checkInterval))

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutting down event status checker")
}
