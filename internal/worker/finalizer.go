package worker

import (
	"context"
	"encoding/json"

	kafkax "github.com/samirwankhede/lewly-pgpyewj/internal/kafka"
	workerService "github.com/samirwankhede/lewly-pgpyewj/internal/service/worker"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Finalizer struct {
	log        *zap.Logger
	service    *workerService.FinalizeService
	c          *kafkax.Consumer
	dlq        *kafkax.Producer
	maxWorkers int
}

func NewFinalizer(log *zap.Logger, service *workerService.FinalizeService, c *kafkax.Consumer, dlq *kafkax.Producer, maxWorkers int) *Finalizer {
	return &Finalizer{
		log:        log,
		service:    service,
		c:          c,
		dlq:        dlq,
		maxWorkers: maxWorkers,
	}
}

func (f *Finalizer) Run(ctx context.Context) error {
	workerCount := f.maxWorkers
	sem := make(chan struct{}, workerCount) // concurrency limit

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			m, err := f.c.Fetch(ctx)
			if err != nil {
				f.log.Error("failed to read message", zap.Error(err))
				continue
			}

			// Acquire semaphore
			sem <- struct{}{}
			go func(m kafka.Message) {
				defer func() { <-sem }() // Release semaphore

				if err := f.handleMessage(ctx, m); err != nil {
					f.log.Error("failed to handle message", zap.Error(err))
					// Send to DLQ for manual inspection
					_ = f.dlq.Publish(ctx, m.Key, m.Value)
				} else {
					// Commit on success
					_ = f.c.Commit(ctx, m)
				}
			}(m)
		}
	}
}

func (f *Finalizer) handleMessage(ctx context.Context, m kafka.Message) error {
	var p workerService.FinalizePayload
	if err := json.Unmarshal(m.Value, &p); err != nil {
		return err
	}

	// Handle normal finalization
	return f.service.HandleBookingFinalization(ctx, p)
}
