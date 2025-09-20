package redisx

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

type TimeoutBucket struct {
	client *redis.Client
}

func NewTimeoutBucket(addr string) *TimeoutBucket {
	c := redis.NewClient(&redis.Options{Addr: addr})
	return &TimeoutBucket{client: c}
}

func (t *TimeoutBucket) NilError() error {
	return redis.Nil
}

func (t *TimeoutBucket) AddBooking(ctx context.Context, eventID string, bookingID string) error {
	key := eventID + ":" + bookingID
	return t.client.Set(ctx, key, "processing", 0).Err()
}

func (t *TimeoutBucket) GetBooking(ctx context.Context, eventID string, bookingID string) (string, error) {
	key := eventID + ":" + bookingID
	v, err := t.client.Get(ctx, key).Result()
	if err == t.NilError() {
		return "processing", nil
	}
	return v, err
}

func (t *TimeoutBucket) DeleteBooking(ctx context.Context, eventID string, bookingID string) (int, error) {
	key := eventID + ":" + bookingID
	deletedCount, err := t.client.Del(ctx, key).Result()
	if err != nil {
		return 1, err
	}
	return int(deletedCount), err
}

func (t *TimeoutBucket) Close() { _ = t.client.Close() }
