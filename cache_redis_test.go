package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisAdapter implements the cache.RedisClient interface using go-redis/v9
type RedisAdapter struct {
	client *redis.Client
}

func (r *RedisAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	return r.client.Get(ctx, key).Bytes()
}

// GetWithTTL fetches value + remaining TTL in a single round-trip via a pipeline,
// implementing cache.RedisGetTTLClient.
func (r *RedisAdapter) GetWithTTL(ctx context.Context, key string) ([]byte, time.Duration, error) {
	pipe := r.client.Pipeline()
	getCmd := pipe.Get(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
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

func (r *RedisAdapter) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisAdapter) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisAdapter) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

func (r *RedisAdapter) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

type RedisTestStruct struct {
	Name  string
	Value int
}

func getRedisClient(t *testing.T) *RedisAdapter {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test:", err)
	}
	rdb.FlushDB(ctx)
	return &RedisAdapter{client: rdb}
}

func TestRedisCache_SetGet(t *testing.T) {
	rdb := getRedisClient(t)
	c := New[RedisTestStruct](5*time.Minute, 10*time.Minute)
	c.WithRedis(rdb).WithRedisTimeout(1 * time.Second)

	ctx := context.Background()
	key := "test_key"
	val := RedisTestStruct{Name: "foo", Value: 42}

	// 1. Set in Cache -> Should go to Redis async
	c.Set(key, val, 0)
	time.Sleep(100 * time.Millisecond) // Wait for async write

	// Verify in Redis
	data, err := rdb.Get(ctx, key)
	if err != nil {
		t.Fatalf("Expected key in Redis, got error: %v", err)
	}
	var res RedisTestStruct
	json.Unmarshal(data, &res)
	if res != val {
		t.Errorf("Redis value mismatch. Got %v, want %v", res, val)
	}

	// 2. Flush Local Cache
	c.Flush()
	if c.ItemCount() != 0 {
		t.Errorf("Expected 0 items after Flush, got %d", c.ItemCount())
	}

	// 3. Get from Cache -> Should fallback to Redis
	got, found := c.Get(key)
	if found != Found {
		t.Errorf("Expected to find item via Redis fallback")
	}
	if got != val {
		t.Errorf("Got %v, want %v", got, val)
	}
}

func TestBoundedCache_Eviction(t *testing.T) {
	// this test doesn't need redis
	c := New[int](NoExpiration, 0)
	capacity := 10
	c.WithCapacity(capacity)

	// Fill to capacity
	for i := 0; i < capacity; i++ {
		c.Set(string(rune(i)), i, 0)
	}

	if c.ItemCount() != capacity {
		t.Errorf("Expected %d items, got %d", capacity, c.ItemCount())
	}

	// Add 11th item -> should evict down to 75% (7 items) + 1 new item = 8
	c.Set("new", 99, 0)

	// Target size is 7 (75% of 10), so after eviction (7) + new item (1) = 8
	// We expect count to be <= 8.
	// Note: Since map iteration is random, relying on exact count usually works if logic is deterministic.
	// Our logic: loops until len <= target. So len will be exactly target (7). Then we add 1. So 8.
	maxExpected := int(float64(capacity)*0.75) + 1
	if c.ItemCount() > maxExpected {
		t.Errorf("Expected max %d items after bulk eviction, got %d", maxExpected, c.ItemCount())
	}
}

func TestRedisCache_ExpiryFallback(t *testing.T) {
	rdb := getRedisClient(t)
	c := New[string](DefaultExpiration, 0)
	c.WithRedis(rdb).WithRedisTimeout(1 * time.Second)

	key := "expired_local"

	// Manually set in Redis with long TTL
	rdb.Set(context.Background(), key, "\"still_valid_in_redis\"", time.Hour)

	// Set in local with short TTL
	c.Set(key, "old_local", 5*time.Millisecond)
	time.Sleep(1 * time.Millisecond) // Let it expire locally
	got, found := c.Get(key)
	if found != Found {
		t.Errorf("Expected to find item")
	}
	if got != "old_local" {
		t.Errorf("Got %v, want %v", got, "old_local")
	}
	time.Sleep(10 * time.Millisecond) // Let it expire locally

	// Get -> Should see it's expired locally (or check expiry) and fetch from Redis
	_, found = c.Get(key)
	if found != Miss {
		t.Errorf("Expected to miss item")
	}
}
