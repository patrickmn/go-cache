package cache

import (
	"hash/fnv"
	"math"
	"runtime"
	"time"
)

// This is an experimental and unexported (for now) attempt at making a cache
// with better algorithmic complexity than the standard one, namely by
// preventing write locks of the entire cache when an item is added. As of the
// time of writing, the overhead of selecting buckets results in cache
// operations being about twice as slow as for the standard cache with small
// total cache sizes, and faster for larger ones.
//
// See cache_test.go for a few benchmarks.
const defaultShards = 16

type unexportedShardedCache[V any] struct {
	*ShardedCache[V]
}

type ShardedCache[V any] struct {
	numBuckets int
	cs         []*cache[V]
	janitor    *shardedJanitor
}

func shardKey(key string, numBuckets int) int {
	// Create an FNV-1a hasher
	hasher := fnv.New32a()
	// Write the key to the hasher
	hasher.Write([]byte(key))
	// Get the hash value
	hash := hasher.Sum32()
	// Map to a bucket
	return int(hash % uint32(numBuckets))
}

func nearestPowerOfTwo(n int) int {
	if n <= 0 {
		return 0 // Or handle as an error, depending on requirements
	}
	if n == 1 {
		return 1
	}

	log2n := math.Log2(float64(n))
	prevPower := 1 << uint(math.Floor(log2n))
	nextPower := 1 << uint(math.Ceil(log2n))

	if math.Abs(float64(n-prevPower)) <= math.Abs(float64(n-nextPower)) {
		return prevPower
	}
	return nextPower
}

func (sc *ShardedCache[V]) bucket(k string) *cache[V] {
	return sc.cs[shardKey(k, sc.numBuckets)]
}

func (sc *ShardedCache[V]) Set(k string, x V, d time.Duration) {
	sc.bucket(k).Set(k, x, d)
}

func (sc *ShardedCache[V]) SetNX(k string, x V, d time.Duration) bool {
	return sc.bucket(k).SetNX(k, x, d)
}

func (sc *ShardedCache[V]) Add(k string, x V, d time.Duration) error {
	return sc.bucket(k).Add(k, x, d)
}

func (sc *ShardedCache[V]) Replace(k string, x V, d time.Duration) error {
	return sc.bucket(k).Replace(k, x, d)
}

func (sc *ShardedCache[V]) Get(k string) (V, FoundItem) {
	return sc.bucket(k).Get(k)
}

func (sc *ShardedCache[V]) Delete(k string) {
	sc.bucket(k).Delete(k)
}

func (sc *ShardedCache[V]) Sync(k string) (V, error) {
	return sc.bucket(k).Sync(k)
}

func (sc *ShardedCache[V]) DeleteExpired() {
	for _, v := range sc.cs {
		v.DeleteExpired()
	}
}

func (sc *ShardedCache[V]) OnEvicted(f func(string, V)) {
	for _, v := range sc.cs {
		v.mu.Lock()
		v.onEvicted = f
		v.mu.Unlock()
	}
}

// Returns the items in the cache. This may include items that have expired,
// but have not yet been cleaned up. If this is significant, the Expiration
// fields of the items should be checked. Note that explicit synchronization
// is needed to use a cache and its corresponding Items() return values at
// the same time, as the maps are shared.
func (sc *ShardedCache[V]) Items() []map[string]Item[V] {
	res := make([]map[string]Item[V], len(sc.cs))
	for i, v := range sc.cs {
		res[i] = v.Items()
	}
	return res
}

func (sc *ShardedCache[V]) ItemCount() (cnt int) {
	for _, v := range sc.cs {
		cnt += v.ItemCount()
	}
	return
}

func (sc *ShardedCache[V]) Flush() {
	for _, v := range sc.cs {
		v.Flush()
	}
}

// Close stops the background Redis workers for all shards and waits for pending operations to complete.
func (sc *ShardedCache[V]) Close() {
	for _, v := range sc.cs {
		v.Close()
	}
}

type shardedJanitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *shardedJanitor) Run(sc interface{ DeleteExpired() }) {
	j.stop = make(chan bool)
	tick := time.NewTicker(j.Interval)
	for {
		select {
		case <-tick.C:
			sc.DeleteExpired()
		case <-j.stop:
			return
		}
	}
}

func stopShardedJanitor[V any](sc *unexportedShardedCache[V]) {
	sc.janitor.stop <- true
}

func runShardedJanitor[V any](sc *ShardedCache[V], ci time.Duration) {
	j := &shardedJanitor{
		Interval: ci,
	}
	sc.janitor = j
	go j.Run(sc)
}

func NewShardedCache[V any](numShards int, defaultExpiration, cleanupInterval time.Duration) *ShardedCache[V] {
	if numShards == 0 {
		numShards = runtime.NumCPU() * 2
		if numShards < defaultShards {
			numShards = defaultShards
		}
	}
	numShards = nearestPowerOfTwo(numShards)

	sc := &ShardedCache[V]{
		numBuckets: int(numShards),
		cs:         make([]*cache[V], numShards),
	}
	for i := 0; i < numShards; i++ {
		c := &cache[V]{
			defaultExpiration: defaultExpiration,
			items:             map[string]Item[V]{},
		}
		sc.cs[i] = c
	}

	if defaultExpiration == 0 {
		defaultExpiration = -1
	}
	if cleanupInterval > 0 {
		runShardedJanitor(sc, cleanupInterval)
		SC := &unexportedShardedCache[V]{sc}
		runtime.SetFinalizer(SC, stopShardedJanitor[V])
	}
	return sc
}

// Configures the cache to use a Redis client for L2 caching and async persistence.
// This also starts the async worker for Redis writes.
func (sc *ShardedCache[V]) WithRedis(cli RedisClient) *ShardedCache[V] {
	for _, c := range sc.cs {
		c.withRedis(cli)
	}
	return sc
}

// Configures a maximum capacity for the local in-memory cache.
// When the limit is reached, items are evicted randomly to make space.
func (sc *ShardedCache[V]) WithCapacity(totalCap int) *ShardedCache[V] {
	capPerShard := totalCap / sc.numBuckets
	if capPerShard < 1 {
		capPerShard = 1
	}
	for _, c := range sc.cs {
		c.withCapacity(capPerShard)
	}
	return sc
}

// Add the ShardedNumericCache type that wraps shardedCache
type ShardedNumericCache[N Number] struct {
	*ShardedCache[N]
}

// ModifyNumeric atomically modifies a numeric item in the cache.
// If the item does not exist or is expired, it is set to `operand`.
// If isIncrement is true, `operand` is added to the existing value.
// If isIncrement is false, `operand` is subtracted from the existing value.
// It returns the new value and an error if the existing item is not of type N.
func (sc *ShardedNumericCache[N]) ModifyNumeric(k string, operand N, isIncrement bool) (N, error) {
	// Delegate to the bucket's ModifyNumeric implementation via a temporary NumericCache wrapper
	return (&NumericCache[N]{cache: sc.bucket(k)}).ModifyNumeric(k, operand, isIncrement)
}

// Incr increments a numeric item in the cache by the specified operand.
// This is a convenience method that calls ModifyNumeric with isIncrement=true.
func (sc *ShardedNumericCache[N]) Incr(k string, operand N) (N, error) {
	return sc.ModifyNumeric(k, operand, true)
}

// Decr decrements a numeric item in the cache by the specified operand.
// This is a convenience method that calls ModifyNumeric with isIncrement=false.
func (sc *ShardedNumericCache[N]) Decr(k string, operand N) (N, error) {
	return sc.ModifyNumeric(k, operand, false)
}

// Helper function to create a new ShardedNumericCache
func NewShardedNumeric[N Number](shards int, defaultExpiration, cleanupInterval time.Duration) *ShardedNumericCache[N] {
	// Create the underlying shardedCache
	sc := NewShardedCache[N](shards, defaultExpiration, cleanupInterval)

	// Create and return the ShardedNumericCache wrapper
	result := &ShardedNumericCache[N]{ShardedCache: sc}

	return result
}

func (sc *ShardedCache[V]) WithRedisTimeout(d time.Duration) *ShardedCache[V] {
	for _, c := range sc.cs {
		c.withRedisTimeout(d)
	}
	return sc
}

func (sc *ShardedNumericCache[N]) WithRedisTimeout(d time.Duration) *ShardedNumericCache[N] {
	sc.ShardedCache.WithRedisTimeout(d)
	return sc
}
