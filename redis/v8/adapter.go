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
