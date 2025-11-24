package cache

import (
	"sync"
	"time"
)

// Item represents a single item in the cache.
// Item 代表缓存中的一个条目。
type Item[V any] struct {
	Object     V
	Expiration time.Time // Use time.Time to utilize the monotonic clock (抗系统时间回拨)
}

// Expired returns true if the item has expired.
// Expired 如果条目已过期则返回 true。
func (item Item[V]) Expired() bool {
	if item.Expiration.IsZero() {
		return false
	}
	return time.Now().After(item.Expiration)
}

const (
	// NoExpiration indicates that the item never expires.
	// NoExpiration 表示条目永不过期。
	NoExpiration time.Duration = -1

	// DefaultExpiration indicates that the default expiration time should be used.
	// DefaultExpiration 表示使用缓存创建时的默认过期时间。
	DefaultExpiration time.Duration = 0
)

// Cache is a thread-safe, generic in-memory key-value store.
// Cache 是一个线程安全的、泛型的内存键值存储。
type Cache[K comparable, V any] struct {
	defaultExpiration time.Duration
	items             map[K]Item[V]
	mu                sync.RWMutex
	onEvicted         func(K, V)
	stopJanitor       chan struct{}
	once              sync.Once // Ensures the janitor is closed only once (保证只关闭一次)
}

// New returns a new cache with a given default expiration duration and cleanup interval.
//
// IMPORTANT: If cleanupInterval > 0, a background goroutine is started.
// You MUST call c.Close() when the cache is no longer needed to release resources
// and prevent memory leaks.
//
// New 创建一个新的缓存。
// 注意：如果 cleanupInterval > 0，会启动后台清理协程。当不再使用缓存时，
// 必须调用 c.Close() 以释放资源并防止内存泄漏。
func New[K comparable, V any](defaultExpiration, cleanupInterval time.Duration) *Cache[K, V] {
	items := make(map[K]Item[V])
	return newCache(defaultExpiration, cleanupInterval, items)
}

// NewFrom returns a new cache from an existing map.
// The caller should not retain the reference to the items map after calling this.
//
// NewFrom 基于现有的 map 创建缓存。
// 调用后，调用者不应保留对 items map 的引用。
func NewFrom[K comparable, V any](defaultExpiration, cleanupInterval time.Duration, items map[K]Item[V]) *Cache[K, V] {
	return newCache(defaultExpiration, cleanupInterval, items)
}

func newCache[K comparable, V any](de, ci time.Duration, m map[K]Item[V]) *Cache[K, V] {
	if de == 0 {
		de = -1
	}
	c := &Cache[K, V]{
		defaultExpiration: de,
		items:             m,
	}
	if ci > 0 {
		c.stopJanitor = make(chan struct{})
		go c.janitor(ci)
	}
	return c
}

// Set adds an item to the cache, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the cache's default expiration time is used.
// If it is -1 (NoExpiration), the item never expires.
//
// Set 添加一个条目到缓存中，如果 key 已存在则替换。
func (c *Cache[K, V]) Set(k K, x V, d time.Duration) {
	// Logic to calculate expiration time
	var e time.Time
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[k] = Item[V]{
		Object:     x,
		Expiration: e,
	}
}

// SetDefault adds an item to the cache using the default expiration.
// SetDefault 使用默认过期时间添加条目。
func (c *Cache[K, V]) SetDefault(k K, x V) {
	c.Set(k, x, DefaultExpiration)
}

// Add adds an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns true if the item was added.
//
// Add 仅当 key 不存在或已过期时才添加条目。如果添加成功返回 true。
func (c *Cache[K, V]) Add(k K, x V, d time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[k]
	if found && !item.Expired() {
		return false
	}

	var e time.Time
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d)
	}
	c.items[k] = Item[V]{
		Object:     x,
		Expiration: e,
	}
	return true
}

// Replace sets a new value for the cache key only if it already exists and hasn't expired.
// Returns true if the item was replaced.
//
// Replace 仅当 key 存在且未过期时才替换值。如果替换成功返回 true。
func (c *Cache[K, V]) Replace(k K, x V, d time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[k]
	if !found || item.Expired() {
		return false
	}

	var e time.Time
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d)
	}
	c.items[k] = Item[V]{
		Object:     x,
		Expiration: e,
	}
	return true
}

// Get retrieves an item from the cache.
// It returns the value or zero value, and a bool indicating whether the key was found.
//
// Get 从缓存中获取条目。返回 value 和是否找到的布尔值。
func (c *Cache[K, V]) Get(k K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[k]
	if !found {
		var zero V
		return zero, false
	}

	if item.Expired() {
		var zero V
		return zero, false
	}

	return item.Object, true
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or zero value, the expiration time, and a bool indicating whether the key was found.
//
// GetWithExpiration 获取条目及其过期时间。
func (c *Cache[K, V]) GetWithExpiration(k K) (V, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[k]
	if !found {
		var zero V
		return zero, time.Time{}, false
	}

	if item.Expired() {
		var zero V
		return zero, time.Time{}, false
	}

	return item.Object, item.Expiration, true
}

// Delete removes an item from the cache. Does nothing if the key is not in the cache.
// Delete 从缓存中删除条目。
func (c *Cache[K, V]) Delete(k K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.delete(k)
}

// delete is the internal helper, assuming the lock is held.
func (c *Cache[K, V]) delete(k K) {
	item, found := c.items[k]
	if found {
		delete(c.items, k)
		if c.onEvicted != nil {
			c.onEvicted(k, item.Object)
		}
	}
}

// DeleteExpired deletes all expired items from the cache.
// This method is optimized to minimize lock contention by using a two-phase strategy:
// 1. Scan keys with a Read Lock.
// 2. Delete confirmed expired keys with a Write Lock.
//
// DeleteExpired 删除所有过期的条目。
// 采用了两阶段锁策略以减少阻塞：先加读锁扫描，再加写锁删除。
func (c *Cache[K, V]) DeleteExpired() {
	var evictedItems []K
	now := time.Now()

	// Phase 1: Identify expired items using a Read Lock (Does not block Get/GetWithExpiration)
	// 阶段 1：使用读锁识别过期条目（不阻塞读操作）
	c.mu.RLock()
	for k, v := range c.items {
		if !v.Expiration.IsZero() && now.After(v.Expiration) {
			evictedItems = append(evictedItems, k)
		}
	}
	c.mu.RUnlock()

	// If no items are expired, we are done.
	if len(evictedItems) == 0 {
		return
	}

	// Phase 2: Delete items using a Write Lock
	// 阶段 2：使用写锁删除
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check time to be safe
	now = time.Now()
	for _, k := range evictedItems {
		item, found := c.items[k]
		if found {
			// Double check: The item might have been updated (and thus un-expired)
			// between the RUnlock and Lock phases.
			// 双重检查：在读锁释放和写锁获取之间，条目可能被更新（从而未过期）。
			if !item.Expiration.IsZero() && now.After(item.Expiration) {
				delete(c.items, k)
				if c.onEvicted != nil {
					c.onEvicted(k, item.Object)
				}
			}
		}
	}
}

// OnEvicted sets an (optional) function that is called with the key and value when an
// item is evicted from the cache.
//
// OnEvicted 设置一个回调函数，当条目被驱逐（删除）时触发。
func (c *Cache[K, V]) OnEvicted(f func(K, V)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvicted = f
}

// Items returns a copy of all unexpired items in the cache.
// This involves a read lock and map copy, so use it sparingly on large caches.
//
// Items 返回缓存中所有未过期条目的副本。
func (c *Cache[K, V]) Items() map[K]Item[V] {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := make(map[K]Item[V], len(c.items))
	now := time.Now()
	for k, v := range c.items {
		if !v.Expiration.IsZero() && now.After(v.Expiration) {
			continue
		}
		m[k] = v
	}
	return m
}

// Count returns the number of items in the cache.
// This may include items that have expired but have not yet been cleaned up.
//
// Count 返回缓存中的条目数量（可能包含已过期但未清理的条目）。
func (c *Cache[K, V]) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Flush deletes all items from the cache.
// Flush 清空所有条目。
func (c *Cache[K, V]) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]Item[V])
}

// Close stops the janitor goroutine and releases resources.
// It is safe to call Close multiple times.
//
// Close 停止清理协程并释放资源。多次调用是安全的。
func (c *Cache[K, V]) Close() {
	c.once.Do(func() {
		if c.stopJanitor != nil {
			close(c.stopJanitor)
		}
	})
}

// janitor is the background goroutine that cleans up expired items.
func (c *Cache[K, V]) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-c.stopJanitor:
			return
		}
	}
}

// GetT (Get Typed) retrieves an item from a Cache[K, any] and asserts it to type T.
// This is useful when you use the cache to store mixed types.
//
// Usage:
//
//	c := cache.New[string, any](...)
//	val, found := cache.GetT[int](c, "myKey")
//
// GetT 从 Cache[K, any] 中获取一个条目并将其断言为类型 T。
// 适用于使用 any 存储多种类型数据的场景。
func GetT[T any, K comparable](c *Cache[K, any], k K) (T, bool) {
	// We access the underlying cache securely via its public method
	// 我们通过公开方法安全地访问底层缓存
	val, found := c.Get(k)
	if !found {
		var zero T
		return zero, false
	}

	// Type assertion
	// 类型断言
	if v, ok := val.(T); ok {
		return v, true
	}

	// Found but type mismatch
	// 找到了但类型不匹配
	var zero T
	return zero, false
}
