package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestShardedCache_RedisIntegration(t *testing.T) {
	rdb := getRedisClient(t)
	// 4 shards, default exp 5m, cleanup 10m
	sc := NewShardedCache[RedisTestStruct](4, 5*time.Minute, 10*time.Minute)
	sc.WithRedis(rdb).WithRedisTimeout(1 * time.Second)

	ctx := context.Background()
	key := "sharded_test_key"
	val := RedisTestStruct{Name: "bar", Value: 100}

	// 1. Set in Sharded Cache -> Should go to Redis async
	sc.Set(key, val, 0)
	time.Sleep(200 * time.Millisecond) // Wait for async write

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
	sc.Flush()
	// Flush clears all shards.
	if sc.ItemCount() != 0 {
		t.Errorf("Expected 0 items after Flush, got %d", sc.ItemCount())
	}

	// 3. Get from Sharded Cache -> Should fallback to Redis
	got, found := sc.Get(key)
	if found != Found {
		t.Errorf("Expected to find item via Redis fallback")
	}
	if got != val {
		t.Errorf("Got %v, want %v", got, val)
	}
}

func TestShardedCache_Capacity(t *testing.T) {
	// 2 shards. Total capacity 10. => 5 per shard.
	sc := NewShardedCache[int](2, NoExpiration, 0)
	sc.WithCapacity(10)

	capacity := 10
	itemsToAdd := 30

	// Add enough items to likely trigger eviction in at least one shard.
	for i := 0; i < itemsToAdd; i++ {
		sc.Set(fmt.Sprintf("key-%d", i), i, 0)
	}

	count := sc.ItemCount()
	t.Logf("Total items after adding %d: %d", itemsToAdd, count)

	// Max allowed per shard is 5.
	// Target per shard after eviction is 75% of 5 = 3.
	// After adding one item, could be 4.
	// So max per shard is 5.
	// Total max = 5 * 2 = 10.

	if count > 10 {
		t.Errorf("Expected eviction to keep size low. Total capacity %d, got %d items", capacity, count)
	}

	if count == itemsToAdd {
		t.Errorf("No eviction happened. Got %d items", count)
	}
}

func TestShardedModifyNumeric_Redis(t *testing.T) {
	rdb := getRedisClient(t)
	sc := NewShardedCache[int](2, 5*time.Minute, 10*time.Minute)
	sc.WithRedis(rdb).WithRedisTimeout(1 * time.Second)
	numericSc := &ShardedNumericCache[int]{ShardedCache: sc}

	ctx := context.Background()
	key := "numeric_redis_key"
	val := 10

	// 1. Set initial value
	sc.Set(key, val, 0)
	time.Sleep(100 * time.Millisecond)

	// 2. Modify Numeric (Increment)
	newVal, err := numericSc.ModifyNumeric(key, 5, true)
	if err != nil {
		t.Fatalf("ModifyNumeric failed: %v", err)
	}
	if newVal != 15 {
		t.Errorf("Expected 15, got %d", newVal)
	}

	time.Sleep(100 * time.Millisecond) // Wait for async write

	// 3. Verify in Redis
	data, err := rdb.Get(ctx, key)
	if err != nil {
		t.Fatalf("Expected key in Redis, got error: %v", err)
	}
	var res int
	json.Unmarshal(data, &res)
	if res != 15 {
		t.Errorf("Redis value mismatch. Got %d, want 15", res)
	}

	// 4. Modify Numeric (Decrement)
	newVal, err = numericSc.ModifyNumeric(key, 3, false)
	if err != nil {
		t.Fatalf("ModifyNumeric failed: %v", err)
	}
	if newVal != 12 {
		t.Errorf("Expected 12, got %d", newVal)
	}

	time.Sleep(100 * time.Millisecond) // Wait for async write

	// 5. Verify in Redis
	data, err = rdb.Get(ctx, key)
	if err != nil {
		t.Fatalf("Expected key in Redis, got error: %v", err)
	}
	json.Unmarshal(data, &res)
	if res != 12 {
		t.Errorf("Redis value mismatch. Got %d, want 12", res)
	}
}

func TestShardedCache_GracefulShutdown(t *testing.T) {
	rdb := getRedisClient(t)
	sc := NewShardedCache[string](4, 5*time.Minute, 10*time.Minute)
	sc.WithRedis(rdb).WithRedisTimeout(1 * time.Second)

	key := "shutdown_key"
	val := "shutdown_val"

	// 1. Set item
	sc.Set(key, val, 0)

	// 2. Immediately Close (should wait for async write)
	sc.Close()

	// 3. Verify in Redis
	ctx := context.Background()
	data, err := rdb.Get(ctx, key)
	if err != nil {
		t.Fatalf("Expected key in Redis after graceful shutdown, got error: %v", err)
	}
	var res string
	json.Unmarshal(data, &res)
	if res != val {
		t.Errorf("Redis value mismatch. Got %s, want %s", res, val)
	}
}

func TestShardedCache_Sync(t *testing.T) {
	rdb := getRedisClient(t)
	sc := NewShardedCache[string](4, 5*time.Minute, 10*time.Minute)
	sc.WithRedis(rdb).WithRedisTimeout(1 * time.Second)

	key := "sync_key"
	val := "initial_val"
	newVal := "updated_in_redis"

	// 1. Set initial value
	sc.Set(key, val, 0)
	time.Sleep(100 * time.Millisecond)

	// 2. Update directly in Redis (simulating another service)
	rdb.Set(context.Background(), key, "\""+newVal+"\"", 0)

	// 3. Verify Local Cache has old value
	got, found := sc.Get(key)
	if found != Found || got != val {
		t.Errorf("Expected old value, got %s", got)
	}

	// 4. Call Sync
	gotSync, err := sc.Sync(key)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if gotSync != newVal {
		t.Errorf("Sync returned %s, want %s", gotSync, newVal)
	}

	// 5. Verify Local Cache updated
	got, found = sc.Get(key)
	if found != Found || got != newVal {
		t.Errorf("Expected new value after Sync, got %s", got)
	}
}

func TestRedisIntegration_NumericAndSet(t *testing.T) {
	rdb := getRedisClient(t)

	t.Run("NumericCache Sync Across Instances", func(t *testing.T) {
		key := "num_test_key"
		worker1 := NewShardedNumeric[int](0, time.Minute, time.Minute)
		worker1.WithRedis(rdb).WithRedisTimeout(time.Second)

		worker2 := NewShardedNumeric[int](0, time.Minute, time.Minute)
		worker2.WithRedis(rdb).WithRedisTimeout(time.Second)

		// Worker1 increments
		val, err := worker1.Incr(key, 5)
		if err != nil {
			t.Fatal(err)
		}
		if val != 5 {
			t.Fatalf("Expected 5, got %d", val)
		}

		time.Sleep(200 * time.Millisecond) // wait for async redis write

		// Worker2 increments
		val2, err := worker2.Incr(key, 3)
		if err != nil {
			t.Fatal(err)
		}
		// Worker2 SHOULD read 5 from Redis, add 3, and return 8.
		if val2 != 8 {
			t.Errorf("Worker2 failed to read from Redis. Expected 8, got %d", val2)
		}
	})

	t.Run("SetCache Sync Across Instances", func(t *testing.T) {
		key := "set_test_key"
		worker1 := NewShardedSetCache(0, time.Minute, time.Minute).WithRedis(rdb).WithRedisTimeout(time.Second)
		worker2 := NewShardedSetCache(0, time.Minute, time.Minute).WithRedis(rdb).WithRedisTimeout(time.Second)

		_, _ = worker1.AddMember(key, "session1", time.Minute, time.Minute)
		time.Sleep(200 * time.Millisecond) // wait for async redis write

		// Worker2 adds a DIFFERENT member to the SAME set
		worker2.AddMember(key, "session2", time.Minute, time.Minute)
		time.Sleep(500 * time.Millisecond) // wait for async redis write

		// Worker1 must Sync to see the change from Worker2 before its own write
		_, err := worker1.Sync(key)
		if err != nil {
			t.Fatalf("Worker1 Sync failed: %v", err)
		}

		cnt, isNew := worker1.AddMember(key, "session1", time.Minute, time.Minute)
		time.Sleep(500 * time.Millisecond) // wait for async redis write

		if isNew {
			t.Errorf("Worker1 added a duplicate session. Expected false, got %v", isNew)
		}
		if cnt != 2 {
			t.Errorf("Worker1 added a duplicate session. Expected 2, got %d (isNew: %v)", cnt, isNew)
		}

		// Worker2 should now see BOTH sessions
		count := worker2.Count(key)
		if count != 2 {
			t.Errorf("Worker2 failed to sync Set from Redis. Expected 2 members, got %d", count)
		}
	})
}

// newMiniRedisAdapter spins up an in-process miniredis and returns an adapter
// plus the *miniredis.Miniredis handle (for direct inspection) and a cleanup.
func newMiniRedisAdapter(t *testing.T) (*RedisAdapter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Could not start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return &RedisAdapter{client: rdb}, mr
}

// TestWriteThroughOnly_NoSyncRead verifies that in memory-authoritative mode the
// hot path never reads Redis on a local miss, while async writes still land in
// Redis and explicit Sync/WarmFromRedis still read it.
func TestWriteThroughOnly_NoSyncRead(t *testing.T) {
	adapter, _ := newMiniRedisAdapter(t)

	sc := NewShardedCache[RedisTestStruct](4, 5*time.Minute, 0)
	sc.WithRedis(adapter).WithRedisTimeout(time.Second).WithWriteThroughOnly()
	defer sc.Close()

	key := "wto_key"
	val := RedisTestStruct{Name: "wto", Value: 7}

	// Write goes to local immediately and to Redis asynchronously.
	sc.Set(key, val, 0)

	// Hot-path read hits local cache.
	if got, found := sc.Get(key); found != Found || got != val {
		t.Fatalf("expected local hit, got %v found=%v", got, found)
	}

	// Wait for async write-behind to reach Redis.
	ctx := context.Background()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := adapter.Get(ctx, key); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("async write never reached Redis")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Flush local cache: in write-through-only mode the hot path must NOT
	// fall back to Redis, so Get should now be a Miss even though the key
	// still exists in Redis.
	sc.Flush()
	if _, found := sc.Get(key); found != Miss {
		t.Errorf("expected Miss in write-through-only mode after flush, got found=%v", found)
	}

	// Explicit Sync bypasses write-through-only and reads Redis.
	got, err := sc.Sync(key)
	if err != nil {
		t.Fatalf("Sync should read Redis even in write-through-only mode: %v", err)
	}
	if got != val {
		t.Errorf("Sync returned %v, want %v", got, val)
	}

	// After Sync the value is back in local cache and served from memory.
	if got, found := sc.Get(key); found != Found || got != val {
		t.Errorf("expected local hit after Sync, got %v found=%v", got, found)
	}
}

// TestWarmFromRedis verifies that startup warmup loads existing Redis state into
// local memory, routed to the correct shards.
func TestWarmFromRedis(t *testing.T) {
	adapter, _ := newMiniRedisAdapter(t)
	ctx := context.Background()

	// Pre-populate Redis directly (as if a previous process had persisted it).
	keys := []string{"warm_a", "warm_b", "warm_c"}
	for i, k := range keys {
		data, _ := json.Marshal(RedisTestStruct{Name: k, Value: i})
		if err := adapter.Set(ctx, k, data, time.Hour); err != nil {
			t.Fatalf("seed Redis: %v", err)
		}
	}

	sc := NewShardedCache[RedisTestStruct](4, 5*time.Minute, 0)
	sc.WithRedis(adapter).WithRedisTimeout(time.Second).WithWriteThroughOnly()
	defer sc.Close()

	// Nothing in local cache yet, and the hot path won't read Redis.
	if _, found := sc.Get("warm_a"); found != Miss {
		t.Fatalf("expected Miss before warmup")
	}

	// Warm a subset (plus one missing key, which must be skipped).
	loaded, err := sc.WarmFromRedis([]string{"warm_a", "warm_b", "missing"})
	if err != nil {
		t.Fatalf("WarmFromRedis: %v", err)
	}
	if loaded != 2 {
		t.Errorf("expected 2 keys loaded, got %d", loaded)
	}

	// Warmed keys are now served from memory.
	if got, found := sc.Get("warm_a"); found != Found || got.Name != "warm_a" {
		t.Errorf("warm_a not loaded into local cache: got %v found=%v", got, found)
	}
	if got, found := sc.Get("warm_b"); found != Found || got.Name != "warm_b" {
		t.Errorf("warm_b not loaded into local cache: got %v found=%v", got, found)
	}
	// Un-warmed key remains a local miss (no sync read).
	if _, found := sc.Get("warm_c"); found != Miss {
		t.Errorf("warm_c should not be loaded, got found=%v", found)
	}
}

// TestSetNX_WriteThroughOnly_LocalAuthoritative verifies that in
// memory-authoritative mode SetNX is decided by the local shard lock (exactly
// one winner) and the Redis SETNX is performed asynchronously.
func TestSetNX_WriteThroughOnly_LocalAuthoritative(t *testing.T) {
	adapter, _ := newMiniRedisAdapter(t)

	sc := NewShardedCache[string](4, DefaultExpiration, 0)
	sc.WithRedis(adapter).WithRedisTimeout(time.Second).WithWriteThroughOnly()
	defer sc.Close()

	key := "wto_setnx_key"
	val := "wto_setnx_val"

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	numGoroutines := 100
	startCh := make(chan struct{})
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			if sc.SetNX(key, val, DefaultExpiration) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	close(startCh)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 local-authoritative SetNX winner, got %d", successCount)
	}

	// Value is present in local cache immediately.
	if v, found := sc.Get(key); found != Found || v != val {
		t.Errorf("expected local value %q, got %q found=%v", val, v, found)
	}

	// The async SETNX should eventually persist to Redis.
	ctx := context.Background()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if data, err := adapter.Get(ctx, key); err == nil {
			var res string
			json.Unmarshal(data, &res)
			if res == val {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("async SETNX never reached Redis")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestDroppedWrites_Counter verifies that overflowing the async write-behind
// queue increments the dropped-writes counter instead of silently losing writes.
func TestDroppedWrites_Counter(t *testing.T) {
	// Block the worker by using an adapter whose Set sleeps, so the buffered
	// channel fills up and subsequent non-blocking enqueues are dropped.
	adapter, _ := newMiniRedisAdapter(t)
	slow := &slowSetAdapter{RedisAdapter: adapter, delay: 2 * time.Millisecond}

	c := New[int](NoExpiration, 0)
	c.WithRedis(slow)
	defer c.Close()

	// Fire far more writes than the channel buffer (1000) can absorb while the
	// worker is stalled, so some must be dropped.
	for i := 0; i < 5000; i++ {
		c.Set(fmt.Sprintf("k-%d", i), i, 0)
	}

	if c.DroppedWrites() == 0 {
		t.Errorf("expected some dropped writes when the queue overflows, got 0")
	}
}

// TestBlockOnFull_ReducesDrops verifies WithBlockOnFull lets a saturated queue
// drain rather than dropping immediately. With blocking enabled, the producer is
// throttled to the worker's rate, so nothing is dropped.
func TestBlockOnFull_ReducesDrops(t *testing.T) {
	adapter, _ := newMiniRedisAdapter(t)
	slow := &slowSetAdapter{RedisAdapter: adapter, delay: time.Millisecond}

	c := New[int](NoExpiration, 0)
	c.WithRedis(slow).WithBlockOnFull(5 * time.Second)
	defer c.Close()

	// 1500 writes > 1000 buffer. With a generous block timeout the producer
	// waits for room instead of dropping.
	for i := 0; i < 1500; i++ {
		c.Set(fmt.Sprintf("bk-%d", i), i, 0)
	}

	if dropped := c.DroppedWrites(); dropped != 0 {
		t.Errorf("expected 0 dropped writes with WithBlockOnFull, got %d", dropped)
	}
}

// slowSetAdapter wraps RedisAdapter and slows down Set/SetNX to simulate a Redis
// worker that cannot keep up with the producer.
type slowSetAdapter struct {
	*RedisAdapter
	delay time.Duration
}

func (s *slowSetAdapter) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	time.Sleep(s.delay)
	return s.RedisAdapter.Set(ctx, key, value, expiration)
}

func (s *slowSetAdapter) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	time.Sleep(s.delay)
	return s.RedisAdapter.SetNX(ctx, key, value, expiration)
}

func TestShardedCache_SetNX_RedisConcurrency(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Could not start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	adapter := &RedisAdapter{client: rdb}

	sc := NewShardedCache[string](4, DefaultExpiration, 0)
	sc.WithRedis(adapter).WithRedisTimeout(1 * time.Second)

	key := "concurrent_redis_setnx_key"
	val := "my_redis_value"

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex

	numGoroutines := 100
	wg.Add(numGoroutines)

	startCh := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			
			if sc.SetNX(key, val, DefaultExpiration) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	close(startCh)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to succeed with Redis SetNX, but got %d", successCount)
	}

	// Verify in local cache
	v, found := sc.Get(key)
	if found != Found {
		t.Errorf("Expected item to be found in local cache")
	}
	if v != val {
		t.Errorf("Expected local value %s, got %s", val, v)
	}

	// Verify in Redis
	ctx := context.Background()
	data, err := adapter.Get(ctx, key)
	if err != nil {
		t.Fatalf("Expected key in Redis, got error: %v", err)
	}
	var res string
	json.Unmarshal(data, &res)
	if res != val {
		t.Errorf("Expected Redis value %s, got %s", val, res)
	}
}
