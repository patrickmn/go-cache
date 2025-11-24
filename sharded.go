package cache

import (
	"hash/maphash"
	"sync"
	"time"
)

// ShardedCache is a cache consisting of multiple shards to reduce lock contention.
// ShardedCache 通过分片减少锁竞争。
type ShardedCache[K comparable, V any] struct {
	mask   uint64
	cs     []*Cache[K, V]
	hasher func(K) uint64
	close  chan struct{}
	once   sync.Once
}

// NewSharded returns a new sharded cache for string keys.
// shards will be automatically adjusted to the nearest power of 2.
//
// NewSharded 创建一个 Key 为 string 的分片缓存。
// shards 参数会自动调整为最近的 2 的幂（例如 10 -> 16），以优化哈希计算性能。
func NewSharded[V any](defaultExpiration, cleanupInterval time.Duration, shards int) *ShardedCache[string, V] {
	// 1. 使用标准库 hash/maphash，无需手动生成随机种子，安全且极快
	var seed = maphash.MakeSeed()
	hasher := func(k string) uint64 {
		return maphash.String(seed, k)
	}

	return NewShardedGeneric[string, V](defaultExpiration, cleanupInterval, shards, hasher)
}

// NewShardedGeneric returns a new sharded cache for generic keys.
// You must provide a hashing function that returns a uint64.
//
// NewShardedGeneric 创建一个泛型 Key 的分片缓存。
// 你必须提供一个返回 uint64 的哈希函数。
func NewShardedGeneric[K comparable, V any](defaultExpiration, cleanupInterval time.Duration, shards int, hasher func(K) uint64) *ShardedCache[K, V] {
	if defaultExpiration == 0 {
		defaultExpiration = -1
	}

	// 2. 强制 shards 为 2 的幂，以便使用位运算 (&) 代替取模 (%)
	// 比如 shards=10 -> adjusted=16 (mask=15, 二进制 1111)
	adjustedShards := 1
	for adjustedShards < shards {
		adjustedShards *= 2
	}
	mask := uint64(adjustedShards - 1)

	sc := &ShardedCache[K, V]{
		mask:   mask,
		cs:     make([]*Cache[K, V], adjustedShards),
		hasher: hasher,
	}

	// 初始化所有分片，注意 cleanupInterval 传 0，
	// 因为分片不需要自己启动清理协程，由 ShardedCache 统一管理。
	for i := 0; i < adjustedShards; i++ {
		sc.cs[i] = newCache(defaultExpiration, 0, make(map[K]Item[V]))
	}

	// 3. 统一启动一个清理协程
	if cleanupInterval > 0 {
		sc.close = make(chan struct{})
		go sc.janitor(cleanupInterval)
	}

	return sc
}

// bucket selects the shard for the given key using bitwise AND.
// 使用位运算快速定位分片。
func (sc *ShardedCache[K, V]) bucket(k K) *Cache[K, V] {
	// 优化：使用 & 运算比 % 快得多
	return sc.cs[sc.hasher(k)&sc.mask]
}

func (sc *ShardedCache[K, V]) Set(k K, x V, d time.Duration) {
	sc.bucket(k).Set(k, x, d)
}

func (sc *ShardedCache[K, V]) SetDefault(k K, x V) {
	sc.bucket(k).SetDefault(k, x)
}

func (sc *ShardedCache[K, V]) Add(k K, x V, d time.Duration) bool {
	return sc.bucket(k).Add(k, x, d)
}

func (sc *ShardedCache[K, V]) Replace(k K, x V, d time.Duration) bool {
	return sc.bucket(k).Replace(k, x, d)
}

func (sc *ShardedCache[K, V]) Get(k K) (V, bool) {
	return sc.bucket(k).Get(k)
}

func (sc *ShardedCache[K, V]) GetWithExpiration(k K) (V, time.Time, bool) {
	return sc.bucket(k).GetWithExpiration(k)
}

func (sc *ShardedCache[K, V]) Delete(k K) {
	sc.bucket(k).Delete(k)
}

// DeleteExpired iterates over all shards and deletes expired items.
func (sc *ShardedCache[K, V]) DeleteExpired() {
	for _, v := range sc.cs {
		v.DeleteExpired()
	}
}

// Items returns the items from all shards.
func (sc *ShardedCache[K, V]) Items() []map[K]Item[V] {
	res := make([]map[K]Item[V], len(sc.cs))
	for i, v := range sc.cs {
		res[i] = v.Items()
	}
	return res
}

func (sc *ShardedCache[K, V]) Flush() {
	for _, v := range sc.cs {
		v.Flush()
	}
}

// Close stops the background janitor goroutine.
func (sc *ShardedCache[K, V]) Close() {
	sc.once.Do(func() {
		if sc.close != nil {
			close(sc.close)
		}
		// Optionally close all shards if they had resources,
		// but currently Cache.Close() just stops its own janitor (which shards don't have).
	})
}

// janitor runs a single loop to clean all shards.
func (sc *ShardedCache[K, V]) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.DeleteExpired()
		case <-sc.close:
			return
		}
	}
}

// Helper for ShardedCache[string, any] to get typed values easily
func GetShardedT[T any](sc *ShardedCache[string, any], k string) (T, bool) {
	// 这里其实就是直接调 bucket 里的 GetT 逻辑，或者手动断言
	// 为了极致性能，直接手动展开：
	v, found := sc.Get(k)
	if !found {
		var zero T
		return zero, false
	}
	if val, ok := v.(T); ok {
		return val, true
	}
	var zero T
	return zero, false
}
