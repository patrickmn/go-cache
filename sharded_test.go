package cache

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// func TestDjb33(t *testing.T) {
// }

var shardedKeys = []string{
	"f",
	"fo",
	"foo",
	"barf",
	"barfo",
	"foobar",
	"bazbarf",
	"bazbart",
	"bazbarr",
	"bazbare",
	"bazbarw",
	"bazbarq",
	"bazbara",
	"bazbarfo",
	"bazbarfs",
	"bazbarff",
	"bazbarfg",
	"bazbarfr",
	"bazbarfe",
	"bazbarfoo",
	"bazbarfoi",
	"bazbarfou",
	"bazbarfoy",
	"bazbarfog",
	"bazbarfod",
	"bazbarfoz",
	"bazbarfox",
	"bazbarfoc",
	"foobarbazq",
	"foobarbazy",
	"foobarbazu",
	"foobarbazi",
	"foobarbazo",
	"foobarbazl",
	"foobarbazk",
	"foobarbazj",
	"foobarbazh",
	"foobarbazf",
	"foobarbazs",
	"foobarbazz",
	"foobarbazqu",
	"foobarbazquu",
	"foobarbazquux",
}

func TestShardedCache(t *testing.T) {
	tc := NewShardedCache[string](13, DefaultExpiration, DefaultExpiration)
	for _, v := range shardedKeys {
		tc.Set(v, "value", DefaultExpiration)
	}
}

// Test for ShardedNumericCache
func TestShardedModifyNumeric(t *testing.T) {
	// Create a sharded numeric cache with 5 shards
	snc := NewShardedNumeric[int](5, DefaultExpiration, 0)

	// Test incrementing a non-existent key
	newVal, err := snc.ModifyNumeric("counter", 5, true)
	if err != nil {
		t.Error("Error incrementing counter:", err)
	}
	if newVal != 5 {
		t.Errorf("Expected counter to be 5, got %d", newVal)
	}

	// Test incrementing an existing key
	newVal, err = snc.ModifyNumeric("counter", 3, true)
	if err != nil {
		t.Error("Error incrementing counter:", err)
	}
	if newVal != 8 {
		t.Errorf("Expected counter to be 8, got %d", newVal)
	}

	// Test decrementing
	newVal, err = snc.ModifyNumeric("counter", 2, false)
	if err != nil {
		t.Error("Error decrementing counter:", err)
	}
	if newVal != 6 {
		t.Errorf("Expected counter to be 6, got %d", newVal)
	}

	// Test with multiple shards
	var wg sync.WaitGroup
	numKeys := 100
	wg.Add(numKeys)

	for i := 0; i < numKeys; i++ {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			snc.Set(key, 0, DefaultExpiration)
			for j := 0; j < 10; j++ {
				snc.ModifyNumeric(key, 1, true)
			}
		}(i)
	}

	wg.Wait()

	// Verify all keys have been incremented to 10
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		val, found := snc.Get(key)
		if found != Found {
			t.Errorf("Key %s not found", key)
			continue
		}
		if val != 10 {
			t.Errorf("Expected value for %s to be 10, got %d", key, val)
		}
	}
}

func TestShardedIncrDecr(t *testing.T) {
	snc := NewShardedNumeric[int](5, DefaultExpiration, 0)
	snc.Set("val", 10, DefaultExpiration)

	// Test Incr
	newVal, err := snc.Incr("val", 5)
	if err != nil {
		t.Fatalf("Incr failed: %v", err)
	}
	if newVal != 15 {
		t.Errorf("Expected 15 after Incr, got %d", newVal)
	}

	// Test Decr
	newVal, err = snc.Decr("val", 3)
	if err != nil {
		t.Fatalf("Decr failed: %v", err)
	}
	if newVal != 12 {
		t.Errorf("Expected 12 after Decr, got %d", newVal)
	}
}

func TestShardedIncrOverflow(t *testing.T) {
	snc := NewShardedNumeric[int8](5, DefaultExpiration, 0)
	snc.Set("int8", int8(127), DefaultExpiration)

	newVal, err := snc.Incr("int8", 1)
	if err != nil {
		t.Fatalf("Incr failed: %v", err)
	}
	if newVal != -128 {
		t.Errorf("Expected -128 after overflow, got %d", newVal)
	}
}

func TestShardedDecrUnderflow(t *testing.T) {
	snc := NewShardedNumeric[uint8](5, DefaultExpiration, 0)
	snc.Set("uint8", uint8(0), DefaultExpiration)

	newVal, err := snc.Decr("uint8", 1)
	if err != nil {
		t.Fatalf("Decr failed: %v", err)
	}
	if newVal != 255 {
		t.Errorf("Expected 255 after underflow, got %d", newVal)
	}
}

func TestShardedIncrWithoutSet(t *testing.T) {
	snc := NewShardedNumeric[int](5, DefaultExpiration, 0)

	newVal, err := snc.Incr("newval", 5)
	if err != nil {
		t.Fatalf("Incr failed: %v", err)
	}
	if newVal != 5 {
		t.Errorf("Expected 5 after Incr on empty key, got %d", newVal)
	}
}

func BenchmarkShardedCacheGetExpiring(b *testing.B) {
	benchmarkShardedCacheGet(b, 5*time.Minute)
}

func BenchmarkShardedCacheGetNotExpiring(b *testing.B) {
	benchmarkShardedCacheGet(b, NoExpiration)
}

func benchmarkShardedCacheGet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	tc := NewShardedCache[any](10, exp, 0)
	tc.Set("foobarba", "zquux", DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Get("foobarba")
	}
}

func BenchmarkShardedCacheGetManyConcurrentExpiring(b *testing.B) {
	benchmarkShardedCacheGetManyConcurrent(b, 5*time.Minute)
}

func BenchmarkShardedCacheGetManyConcurrentNotExpiring(b *testing.B) {
	benchmarkShardedCacheGetManyConcurrent(b, NoExpiration)
}

func benchmarkShardedCacheGetManyConcurrent(b *testing.B, exp time.Duration) {
	b.StopTimer()
	n := 10000
	tsc := NewShardedCache[any](20, exp, 0)
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "foo" + strconv.Itoa(i)
		keys[i] = k
		tsc.Set(k, "bar", DefaultExpiration)
	}
	each := b.N / n
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for _, v := range keys {
		go func(k string) {
			for j := 0; j < each; j++ {
				tsc.Get(k)
			}
			wg.Done()
		}(v)
	}
	b.StartTimer()
	wg.Wait()
}

// Add benchmark for ShardedNumericCache ModifyNumeric
func BenchmarkShardedModifyNumericInt(b *testing.B) {
	b.StopTimer()
	sc := NewShardedNumeric[int](5, DefaultExpiration, 0)
	sc.Set("foo", 0, DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sc.ModifyNumeric("foo", 1, true) // Increment
	}
}

// Add benchmark comparing sharded vs non-sharded numeric operations
func BenchmarkCompareShardedVsRegularModifyNumeric(b *testing.B) {
	b.Run("Sharded", func(b *testing.B) {
		sc := NewShardedNumeric[int](10, DefaultExpiration, 0)
		sc.Set("counter", 0, DefaultExpiration)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sc.ModifyNumeric("counter", 1, true)
		}
	})

	b.Run("Regular", func(b *testing.B) {
		nc := newNumericTestCache[int](nil, DefaultExpiration, 0)
		nc.Set("counter", 0, DefaultExpiration)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			nc.ModifyNumeric("counter", 1, true)
		}
	})
}

func TestShardedCacheItemExpiration(t *testing.T) {
	ts := NewShardedCache[int](2, 50*time.Millisecond, 10*time.Millisecond)
	ts.OnEvicted(func(s string, i int) {
		fmt.Printf("delete %s (%d)\n", s, i)
	})
	ts.Set("short-lived", 1, DefaultExpiration)
	ts.Set("custom-expiry", 2, 20*time.Millisecond)
	ts.Set("no-expiry", 3, NoExpiration)

	// Wait for short-lived to expire but custom-expiry to still be alive
	time.Sleep(300 * time.Millisecond)

	// short-lived should be gone
	_, found := ts.Get("short-lived")
	if found == Found {
		t.Error("short-lived item should have expired")
	}

	// custom-expiry should be gone
	_, found = ts.Get("custom-expiry")
	if found == Found {
		t.Error("custom-expiry item should have expired")
	}

	// no-expiry should still be there
	val, found := ts.Get("no-expiry")
	if found != Found || val != 3 {
		t.Error("no-expiry item should still exist with value 3")
	}

	// Wait for janitor to run multiple times
	time.Sleep(50 * time.Millisecond)

	// Count should be 1 (just no-expiry)
	count := ts.ItemCount()
	if count != 1 {
		t.Errorf("Expected 1 item, got %d", count)
		for i := 0; i < len(ts.Items()); i++ {
			bi := ts.Items()[i]
			for k, v := range bi {
				t.Errorf("item: %s (%v)", k, v)

			}
		}
	}
}

func TestShardedCache_OnEvicted_Capacity(t *testing.T) {
	// Create a cache with 4 shards and total capacity of 4 (1 per shard)
	sc := NewShardedCache[int](4, DefaultExpiration, 0)
	sc.WithCapacity(4)

	var mu sync.Mutex
	evictedKeys := make([]string, 0)

	sc.OnEvicted(func(k string, v int) {
		mu.Lock()
		evictedKeys = append(evictedKeys, k)
		mu.Unlock()
	})

	// Fill the cache to trigger eviction
	// With 4 shards and cap 4, cap per shard is 1.
	// Add 20 items to ensure overflow across shards.
	for i := 0; i < 20; i++ {
		sc.Set(fmt.Sprintf("key-%d", i), i, DefaultExpiration)
	}

	mu.Lock()
	count := len(evictedKeys)
	mu.Unlock()

	if count == 0 {
		t.Errorf("Expected eviction callbacks to be called, got 0")
	}
}

func TestShardedCache_SetNX_LocalConcurrency(t *testing.T) {
	sc := NewShardedCache[string](4, DefaultExpiration, 0)
	key := "concurrent_setnx_key"
	val := "my_value"

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex

	numGoroutines := 100
	wg.Add(numGoroutines)

	// Start all goroutines at roughly the same time
	startCh := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-startCh // Wait for signal
			
			if sc.SetNX(key, val, DefaultExpiration) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	// Trigger all
	close(startCh)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to succeed with SetNX, but got %d", successCount)
	}

	// Verify the value is correctly set
	v, found := sc.Get(key)
	if found != Found {
		t.Errorf("Expected item to be found in cache")
	}
	if v != val {
		t.Errorf("Expected value %s, got %s", val, v)
	}
}
