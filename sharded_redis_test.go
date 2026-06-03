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
