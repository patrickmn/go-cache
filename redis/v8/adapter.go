package redisv8

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// Wrapper for go-redis/v8
type Adapter struct {
	client *redis.Client
}

func New(client *redis.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) Get(ctx context.Context, key string) ([]byte, error) {
	return a.client.Get(ctx, key).Bytes()
}

// GetWithTTL fetches the value and its remaining TTL in a single round-trip
// using a pipeline (GET + TTL). It implements cache.RedisGetTTLClient so the
// cache can avoid issuing two separate round-trips on a Redis read.
func (a *Adapter) GetWithTTL(ctx context.Context, key string) ([]byte, time.Duration, error) {
	pipe := a.client.Pipeline()
	getCmd := pipe.Get(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		// Exec returns the first command error (e.g. redis.Nil when the key
		// is missing), which we surface as a miss.
		return nil, 0, err
	}
	b, err := getCmd.Bytes()
	if err != nil {
		return nil, 0, err
	}
	ttl, err := ttlCmd.Result()
	if err != nil {
		ttl = 0
	}
	return b, ttl, nil
}

func (a *Adapter) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	if expiration < time.Millisecond {
		expiration = time.Millisecond
	}
	return a.client.Set(ctx, key, value, expiration).Err()
}

func (a *Adapter) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	if expiration > 0 && expiration < time.Millisecond {
		expiration = time.Millisecond
	} else if expiration < 0 {
		expiration = 0
	}
	return a.client.SetNX(ctx, key, value, expiration).Result()
}

func (a *Adapter) Del(ctx context.Context, key string) error {
	return a.client.Del(ctx, key).Err()
}

func (a *Adapter) TTL(ctx context.Context, key string) (time.Duration, error) {
	return a.client.TTL(ctx, key).Result()
}
