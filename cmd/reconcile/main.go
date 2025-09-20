package main

import (
	"context"
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/config"
	"github.com/samirwankhede/lewly-pgpyewj/internal/logger"
	"github.com/samirwankhede/lewly-pgpyewj/internal/metrics"
	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	log := logger.New(cfg.Env)
	ctx := context.Background()

	db, err := store.NewDB(ctx, cfg.PostgresURL, int32(cfg.MaxDBConnections))
	if err != nil {
		log.Fatal("db", zap.Error(err))
	}
	defer db.Close()
	tokens := redisx.NewTokenBucket(cfg.RedisAddr)

	// Enhanced reconciliation: compare event_capacity table vs Redis tokens
	metrics.ReconciliationRunsTotal.Inc()

	// First, ensure all events have corresponding event_capacity entries
	rows, err := db.Pool.Query(ctx, `
		SELECT e.id, e.capacity, e.reserved 
		FROM events e 
		LEFT JOIN event_capacity ec ON e.id = ec.event_id 
		WHERE ec.event_id IS NULL
	`)
	if err != nil {
		log.Fatal("query events without capacity", zap.Error(err))
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var capacity, reserved int
		if err := rows.Scan(&id, &capacity, &reserved); err != nil {
			log.Error("scan event without capacity", zap.Error(err))
			continue
		}

		// Insert missing event_capacity entry
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO event_capacity (event_id, capacity, reserved_count) 
			VALUES ($1, $2, $3)
		`, id, capacity, reserved)
		if err != nil {
			log.Error("insert event_capacity", zap.Error(err), zap.String("event_id", id))
			continue
		}

		log.Info("created event_capacity entry", zap.String("event_id", id))
	}
	rows.Close()

	// Now reconcile event_capacity vs Redis tokens
	rows, err = db.Pool.Query(ctx, `SELECT event_id, capacity, reserved_count FROM event_capacity`)
	if err != nil {
		log.Fatal("query event_capacity", zap.Error(err))
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var capacity, reserved int
		if err := rows.Scan(&id, &capacity, &reserved); err != nil {
			log.Error("scan event_capacity", zap.Error(err))
			continue
		}

		desired := capacity - reserved
		rem, _ := tokens.Remaining(ctx, id)
		if rem != desired {
			diff := desired - rem
			if diff > 0 {
				_ = tokens.Release(ctx, id, diff)
			} else if diff < 0 {
				// consume extra tokens
				for i := 0; i < -diff; i++ {
					_, _ = tokens.Reserve(ctx, id, 1)
				}
			}
			metrics.ReconciliationFixesTotal.Inc()
			log.Info("reconciled", zap.String("event", id), zap.Int("desired", desired), zap.Int("was", rem))
		}
	}
	fmt.Println("reconciliation complete at", time.Now())
}
