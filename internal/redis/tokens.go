package redisx

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
)

const reserveLua = `
local key = KEYS[1]
local n = tonumber(ARGV[1])
local current = tonumber(redis.call('GET', key) or '0')
if current >= n then
  redis.call('DECRBY', key, n)
  return 1
else
  return 0
end`

type TokenBucket struct{ client *redis.Client }

func NewTokenBucket(addr string) *TokenBucket {
	c := redis.NewClient(&redis.Options{Addr: addr})
	return &TokenBucket{client: c}
}

func (t *TokenBucket) key(eventID string) string { return fmt.Sprintf("event_tokens:%s", eventID) }

func (t *TokenBucket) InitTokens(ctx context.Context, eventID string, capacity int) error {
	return t.client.Set(ctx, t.key(eventID), capacity, 0).Err()
}

func (t *TokenBucket) Reserve(ctx context.Context, eventID string, n int) (bool, error) {
	res := t.client.Eval(ctx, reserveLua, []string{t.key(eventID)}, n)
	if res.Err() != nil {
		return false, res.Err()
	}
	v, _ := res.Int()
	return v == 1, nil
}

func (t *TokenBucket) Release(ctx context.Context, eventID string, n int) error {
	return t.client.IncrBy(ctx, t.key(eventID), int64(n)).Err()
}

func (t *TokenBucket) Remaining(ctx context.Context, eventID string) (int, error) {
	v, err := t.client.Get(ctx, t.key(eventID)).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (t *TokenBucket) Close() { _ = t.client.Close() }

// GetClient returns the underlying Redis client for OTP operations
func (t *TokenBucket) GetClient() *redis.Client {
	return t.client
}
