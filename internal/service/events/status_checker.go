package events

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
)

type EventStatusChecker struct {
	log    *zap.Logger
	events *events.EventsRepository
}

func NewEventStatusChecker(log *zap.Logger, events *events.EventsRepository) *EventStatusChecker {
	return &EventStatusChecker{
		log:    log,
		events: events,
	}
}

// CheckAndUpdateExpiredEvents checks for events that have passed their end_time and updates their status to 'expired'
func (s *EventStatusChecker) CheckAndUpdateExpiredEvents(ctx context.Context) (int, error) {
	updatedCount, err := s.events.UpdateExpiredEvents(ctx)
	if err != nil {
		s.log.Error("Failed to update expired events", zap.Error(err))
		return 0, err
	}

	if updatedCount > 0 {
		s.log.Info("Updated expired events", zap.Int("count", updatedCount))
	}

	return updatedCount, nil
}

// RunPeriodicCheck runs the expired events check periodically
func (s *EventStatusChecker) RunPeriodicCheck(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.log.Info("Starting periodic event status checker", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Stopping periodic event status checker")
			return
		case <-ticker.C:
			_, err := s.CheckAndUpdateExpiredEvents(ctx)
			if err != nil {
				s.log.Error("Periodic check failed", zap.Error(err))
			}
		}
	}
}
