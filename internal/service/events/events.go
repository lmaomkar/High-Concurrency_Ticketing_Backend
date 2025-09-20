package events

import (
	"context"
	"time"

	"go.uber.org/zap"

	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
)

type EventsService struct {
	log    *zap.Logger
	repo   *events.EventsRepository
	tokens *redisx.TokenBucket
}

func NewEventsService(log *zap.Logger, repo *events.EventsRepository, tokens *redisx.TokenBucket) *EventsService {
	return &EventsService{log: log, repo: repo, tokens: tokens}
}

func (s *EventsService) List(ctx context.Context, limit, offset int, q string, from, to *time.Time) ([]*events.Event, error) {
	return s.repo.List(ctx, limit, offset, q, from, to)
}

func (s *EventsService) ListAll(ctx context.Context, limit, offset int) ([]*events.Event, error) {
	return s.repo.ListAll(ctx, limit, offset)
}

func (s *EventsService) ListUpcoming(ctx context.Context, limit, offset int) ([]*events.Event, error) {
	return s.repo.ListUpcoming(ctx, limit, offset)
}

func (s *EventsService) ListPopular(ctx context.Context, limit, offset int) ([]*events.Event, error) {
	return s.repo.ListPopular(ctx, limit, offset)
}

func (s *EventsService) Get(ctx context.Context, id string) (*events.Event, int, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	rem, _ := s.tokens.Remaining(ctx, id)
	return e, rem, nil
}

func (s *EventsService) LikeEvent(ctx context.Context, eventID, userID string) error {
	return s.repo.LikeEvent(ctx, eventID, userID)
}

func (s *EventsService) UnlikeEvent(ctx context.Context, eventID, userID string) error {
	return s.repo.UnlikeEvent(ctx, eventID, userID)
}

func (s *EventsService) IsLiked(ctx context.Context, eventID, userID string) (bool, error) {
	return s.repo.IsLiked(ctx, eventID, userID)
}

func (s *EventsService) GetAvailableSeats(ctx context.Context, eventID string) ([]string, error) {
	return s.repo.GetAvailableSeats(ctx, eventID)
}
