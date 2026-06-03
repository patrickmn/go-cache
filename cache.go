package cache

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// RedisClient defines the interface for Redis operations.
// This allows using any Redis client (e.g., go-redis v8, v9) by providing an adapter.
type RedisClient interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
	TTL(ctx context.Context, key string) (time.Duration, error)
}

type Item[V any] struct {
	Object     V
	Expiration time.Time
}

type FoundItem uint8

const (
	Found FoundItem = iota
	Miss
	Expired
)

// Returns true if the item has expired.
func (item Item[V]) Expired() bool {
	if item.Expiration.IsZero() {
		return false
	}
	return time.Now().After(item.Expiration)
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache[V any] struct {
	*cache[V]
	// If this is confusing, see the comment at the bottom of New()
}

type cache[V any] struct {
	defaultExpiration time.Duration
	items             map[string]Item[V]
	mu                sync.RWMutex
	onEvicted         func(string, V)
	janitor           *janitor[V]
	// Redis & Capacity
	redisClient   RedisClient
	redisCh       chan redisItem[V]
	localCapacity int
	ctx           context.Context
	wg            sync.WaitGroup
	redisTimeout  time.Duration
}

type redisOp int

const (
	redisOpSet redisOp = iota
	redisOpDel
)

type redisItem[V any] struct {
	k  string
	v  V
	d  time.Duration
	op redisOp
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache[V]) Set(k string, x V, d time.Duration) {
	c.mu.Lock()
	c.set(k, x, d)
	c.mu.Unlock()
}

// SetNX adds an item to the cache only if it doesn't already exist.
// Returns true if the item was added, false otherwise.
// If Redis is configured, it performs a synchronous SETNX on Redis.
func (c *cache[V]) SetNX(k string, x V, d time.Duration) bool {
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}

	c.mu.Lock()
	item, found := c.items[k]
	if found && !item.Expired() {
		c.mu.Unlock()
		return false
	}

	if c.redisClient == nil {
		c.setLocal(k, x, d)
		c.mu.Unlock()
		return true
	}

	c.mu.Unlock()

	ctx := c.ctx
	var cancel context.CancelFunc
	if c.redisTimeout > 0 {
		ctx, cancel = context.WithTimeout(c.ctx, c.redisTimeout)
		defer cancel()
	}

	data, err := json.Marshal(x)
	if err != nil {
		return false
	}

	success, err := c.redisClient.SetNX(ctx, k, data, d)
	if err != nil || !success {
		return false
	}

	c.mu.Lock()
	c.setLocal(k, x, d)
	c.mu.Unlock()

	return true
}

func (c *cache[V]) set(k string, x V, d time.Duration) {
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

	c.evict()

	// Async Redis Write
	if c.redisClient != nil && c.redisCh != nil {
		// Use non-blocking send or buffered
		select {
		case c.redisCh <- redisItem[V]{k: k, v: x, d: d, op: redisOpSet}:
		default:
			// Channel full, drop write to avoid blocking app
		}
	}
}

// evict clears 25% of the cache if it's full.
// This prevents thrashing (add 1, delete 1, add 1, delete 1...)
func (c *cache[V]) evict() {
	if c.localCapacity > 0 && len(c.items) > c.localCapacity {
		// Target size is 75% of capacity
		target := c.localCapacity * 3 / 4
		for k := range c.items {
			// Check if we need to call OnEvicted
			if c.onEvicted != nil {
				if v, found := c.items[k]; found {
					c.onEvicted(k, v.Object)
				}
			}
			delete(c.items, k)
			if len(c.items) <= target {
				break
			}
		}
	}
}

// Add an item to the cache, replacing any existing item, using the default
// expiration.
func (c *cache[V]) SetDefault(k string, x V) {
	c.Set(k, x, DefaultExpiration)
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache[V]) Add(k string, x V, d time.Duration) error {
	c.mu.Lock()
	_, found := c.get(k)
	if found {
		c.mu.Unlock()
		return fmt.Errorf("Item %s already exists", k)
	}
	c.set(k, x, d)
	c.mu.Unlock()
	return nil
}

// Set a new value for the cache key only if it already exists, and the existing
// item hasn't expired. Returns an error otherwise.
func (c *cache[V]) Replace(k string, x V, d time.Duration) error {
	c.mu.Lock()
	_, found := c.get(k)
	if !found {
		c.mu.Unlock()
		return fmt.Errorf("Item %s doesn't exist", k)
	}
	c.set(k, x, d)
	c.mu.Unlock()
	return nil
}

// Get an item from the cache. Returns the item or the zero value of V, and a FoundItem(Found,Miss,Expired) indicating
// whether the key was found.
func (c *cache[V]) Get(k string) (V, FoundItem) {
	c.mu.RLock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return c.getFallback(k)
	}
	if !item.Expiration.IsZero() {
		if time.Now().After(item.Expiration) {
			c.mu.RUnlock()
			return c.getFallback(k)
		}
	}
	c.mu.RUnlock()
	return item.Object, Found
}

func (c *cache[V]) getFallback(k string) (V, FoundItem) {
	// Try Redis
	obj, ttl, found := c.fetchFromRedis(k)
	if found {
		c.mu.Lock()
		c.setLocal(k, obj, ttl)
		c.mu.Unlock()
		return obj, Found
	}

	var zero V
	return zero, Miss
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or the zero value of V, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.
func (c *cache[V]) GetWithExpiration(k string) (V, time.Time, bool) {
	c.mu.RLock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return c.getFallbackWithExpiration(k)
	}

	if !item.Expiration.IsZero() {
		if time.Now().After(item.Expiration) {
			c.mu.RUnlock()
			return c.getFallbackWithExpiration(k)
		}

		// Return the item and the expiration time
		c.mu.RUnlock()
		return item.Object, item.Expiration, true
	}

	// If expiration <= 0 (i.e. no expiration time set) then return the item
	// and a zeroed time.Time
	c.mu.RUnlock()
	return item.Object, time.Time{}, true
}

func (c *cache[V]) getFallbackWithExpiration(k string) (V, time.Time, bool) {
	// Try Redis
	obj, ttl, found := c.fetchFromRedis(k)
	if found {
		c.mu.Lock()
		c.setLocal(k, obj, ttl)
		c.mu.Unlock()
		var exp time.Time
		if ttl > 0 {
			exp = time.Now().Add(ttl)
		}
		return obj, exp, true
	}

	var zero V
	return zero, time.Time{}, false
}

// Sync fetches the value for the given key from Redis (if available) and updates the local cache.
// It returns the value and an error if the key was not found in Redis or if Redis is not configured.
func (c *cache[V]) Sync(k string) (V, error) {
	val, ttl, found := c.fetchFromRedis(k)
	if !found {
		var zero V
		return zero, fmt.Errorf("item %s not found in Redis", k)
	}

	c.mu.Lock()
	c.setLocal(k, val, ttl)
	c.mu.Unlock()

	return val, nil
}

func (c *cache[V]) get(k string) (V, bool) {
	item, found := c.items[k]
	if !found || item.Expired() {
		// Local miss or local expiry: try Redis
		if val, ttl, ok := c.fetchFromRedis(k); ok {
			c.setLocal(k, val, ttl)
			return val, true
		}
		var zero V
		return zero, false
	}
	return item.Object, true
}

// Fetches from Redis without modifying local cache. Safe to call without lock.
func (c *cache[V]) fetchFromRedis(k string) (V, time.Duration, bool) {
	var zero V
	if c.redisClient == nil {
		return zero, 0, false
	}

	ctx := c.ctx
	var cancel context.CancelFunc
	if c.redisTimeout > 0 {
		ctx, cancel = context.WithTimeout(c.ctx, c.redisTimeout)
		defer cancel()
	}

	val, err := c.redisClient.Get(ctx, k)
	if err != nil {
		return zero, 0, false
	}
	var obj V
	if err := json.Unmarshal(val, &obj); err != nil {
		return zero, 0, false
	}
	ttl, err := c.redisClient.TTL(ctx, k)
	if err != nil || ttl < 0 {
		ttl = DefaultExpiration
	}
	return obj, ttl, true
}

func (c *cache[V]) setLocal(k string, x V, d time.Duration) {
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

	c.evict()
}

// SignedInteger is a constraint for signed integer types.
type SignedInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// UnsignedInteger is a constraint for unsigned integer types.
type UnsignedInteger interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Float is a constraint for float types.
type Float interface {
	~float32 | ~float64
}

// Number is a constraint for any numeric type that supports addition and subtraction.
type Number interface {
	SignedInteger | UnsignedInteger | Float
}

// ModifyNumeric atomically modifies a numeric item in the cache.
// If the item does not exist or is expired, it is set to `operand`.
// If isIncrement is true, `operand` is added to the existing value.
// If isIncrement is false, `operand` is subtracted from the existing value.
// It returns the new value and an error if the existing item is not of type N.
// This method replaces all previous Increment<Type> and Decrement<Type> methods.
type NumericCache[N Number] struct {
	*cache[N]
}

// WithRedis attaches a Redis L2 layer.
func (c *NumericCache[N]) WithRedis(cli RedisClient) *NumericCache[N] {
	c.withRedis(cli)
	return c
}

func (c *Cache[V]) WithRedisTimeout(d time.Duration) *Cache[V] {
	c.cache.withRedisTimeout(d)
	return c
}

func (c *NumericCache[N]) WithRedisTimeout(d time.Duration) *NumericCache[N] {
	c.cache.withRedisTimeout(d)
	return c
}

func (c *cache[V]) withRedisTimeout(d time.Duration) {
	c.mu.Lock()
	c.redisTimeout = d
	c.mu.Unlock()
}

// ModifyNumeric atomically modifies a numeric item in the cache.
// If the item does not exist or is expired, it is set to `operand`.
// If isIncrement is true, `operand` is added to the existing value.
// If isIncrement is false, `operand` is subtracted from the existing value.
// It returns the new value and an error if the existing item is not of type N.
func (c *NumericCache[N]) ModifyNumeric(k string, operand N, isIncrement bool) (N, error) {
	c.mu.Lock()
	item, itemFound := c.items[k]

	if !itemFound || item.Expired() {
		if val, ttl, ok := c.fetchFromRedis(k); ok {
			c.setLocal(k, val, ttl)
			item = c.items[k]
			itemFound = true
		}
	}

	// Handle cases where item is not found or is expired
	if !itemFound || item.Expired() {
		if itemFound && item.Expired() {
			if c.onEvicted != nil {
				c.onEvicted(k, item.Object)
			}
		}
		c.set(k, operand, DefaultExpiration)
		c.mu.Unlock()
		return operand, nil
	}

	currentVal := item.Object

	var newVal N
	if isIncrement {
		newVal = currentVal + operand
	} else {
		newVal = currentVal - operand
	}

	item.Object = newVal
	c.items[k] = item
	c.mu.Unlock()

	// Async Redis Write
	if c.redisClient != nil && c.redisCh != nil {
		// Calculate remaining TTL
		var d time.Duration
		if !item.Expiration.IsZero() {
			d = time.Until(item.Expiration)
			if d <= 0 {
				d = 100 * time.Millisecond // Minimum expiration for near-expired items
			}
		} else {
			d = -1 // NoExpiration
		}

		select {
		case c.redisCh <- redisItem[N]{k: k, v: newVal, d: d, op: redisOpSet}:
		default:
		}
	}

	return newVal, nil
}

// Incr increments a numeric item in the cache by the specified operand.
// This is a convenience method that calls ModifyNumeric with isIncrement=true.
func (c *NumericCache[N]) Incr(k string, operand N) (N, error) {
	return c.ModifyNumeric(k, operand, true)
}

// Decr decrements a numeric item in the cache by the specified operand.
// This is a convenience method that calls ModifyNumeric with isIncrement=false.
func (c *NumericCache[N]) Decr(k string, operand N) (N, error) {
	return c.ModifyNumeric(k, operand, false)
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache[V]) Delete(k string) {
	c.mu.Lock()
	v, evicted := c.delete(k)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}
}

func (c *cache[V]) delete(k string) (V, bool) {
	if c.onEvicted != nil {
		if v, found := c.items[k]; found {
			delete(c.items, k)
			// Redis Async Delete
			if c.redisClient != nil && c.redisCh != nil {
				select {
				case c.redisCh <- redisItem[V]{k: k, op: redisOpDel}:
				default:
				}
			}
			return v.Object, true
		}
	}
	delete(c.items, k)
	// Redis Async Delete (even if onEvicted is nil)
	if c.redisClient != nil && c.redisCh != nil {
		select {
		case c.redisCh <- redisItem[V]{k: k, op: redisOpDel}:
		default:
		}
	}
	var zero V
	return zero, false
}

type keyAndValue[V any] struct {
	key   string
	value V
}

// Delete all expired items from the cache.
func (c *cache[V]) DeleteExpired() {
	var evictedItems []keyAndValue[V]
	now := time.Now()
	c.mu.Lock()
	for k, v := range c.items {
		// "Inlining" of expired
		if !v.Expiration.IsZero() && now.After(v.Expiration) {
			ov, evicted := c.delete(k) // delete will return V, bool
			if evicted {
				// ov is of type V, so this is type-safe
				evictedItems = append(evictedItems, keyAndValue[V]{k, ov})
			}
		}
	}
	c.mu.Unlock()
	if c.onEvicted != nil {
		for _, v := range evictedItems {
			c.onEvicted(v.key, v.value) // v.value is of type V
		}
	}
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *cache[V]) OnEvicted(f func(string, V)) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}

// Write the cache's items (using Gob) to an io.Writer.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *cache[V]) Save(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("error registering item types with Gob library")
		}
	}()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, v := range c.items {
		gob.Register(v.Object)
	}
	err = enc.Encode(&c.items)
	return
}

// Save the cache's items to the given filename, creating the file if it
// doesn't exist, and overwriting it if it does.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *cache[V]) SaveFile(fname string) error {
	fp, err := os.Create(fname)
	if err != nil {
		return err
	}
	err = c.Save(fp)
	if err != nil {
		fp.Close()
		return err
	}
	return fp.Close()
}

// Add (Gob-serialized) cache items from an io.Reader, excluding any items with
// keys that already exist (and haven't expired) in the current cache.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *cache[V]) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	items := map[string]Item[V]{}
	err := dec.Decode(&items)
	if err == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		for k, v := range items {
			ov, found := c.items[k]
			if !found || ov.Expired() {
				c.items[k] = v
			}
		}
	}
	return err
}

// Load and add cache items from the given filename, excluding any items with
// keys that already exist in the current cache.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *cache[V]) LoadFile(fname string) error {
	fp, err := os.Open(fname)
	if err != nil {
		return err
	}
	err = c.Load(fp)
	if err != nil {
		fp.Close()
		return err
	}
	return fp.Close()
}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *cache[V]) Items() map[string]Item[V] {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := make(map[string]Item[V], len(c.items))
	now := time.Now()
	for k, v := range c.items {
		// "Inlining" of Expired
		if !v.Expiration.IsZero() {
			if now.After(v.Expiration) {
				continue
			}
		}
		m[k] = v
	}
	return m
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *cache[V]) ItemCount() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Delete all items from the cache.
func (c *cache[V]) Flush() {
	c.mu.Lock()
	c.items = map[string]Item[V]{}
	c.mu.Unlock()
}

type janitor[V any] struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor[V]) Run(c *cache[V]) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor[V any](c *Cache[V]) {
	c.janitor.stop <- true
}

func runJanitor[V any](c *cache[V], ci time.Duration) {
	j := &janitor[V]{
		Interval: ci,
		stop:     make(chan bool),
	}
	c.janitor = j
	go j.Run(c)
}

func newCache[V any](de time.Duration, m map[string]Item[V]) *cache[V] {
	if de == 0 {
		de = -1
	}
	c := &cache[V]{
		defaultExpiration: de,
		items:             m,
	}
	return c
}

func newCacheWithJanitor[V any](de time.Duration, ci time.Duration, m map[string]Item[V]) *Cache[V] {
	c := newCache(de, m)
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Cache[V]{c}
	if ci > 0 {
		runJanitor(c, ci)
		runtime.SetFinalizer(C, stopJanitor[V])
	}
	return C
}

// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func New[V any](defaultExpiration, cleanupInterval time.Duration) *Cache[V] {
	items := make(map[string]Item[V])
	return newCacheWithJanitor[V](defaultExpiration, cleanupInterval, items)
}

// Configures a maximum capacity for the local in-memory cache.
// When the limit is reached, items are evicted randomly to make space.
func (c *Cache[V]) WithCapacity(cap int) *Cache[V] {
	c.cache.withCapacity(cap)
	return c
}

func (c *cache[V]) withCapacity(cap int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localCapacity = cap
}

func (c *cache[V]) redisWorker(ch chan redisItem[V]) {
	defer c.wg.Done()
	for item := range ch {
		if c.redisClient != nil {
			switch item.op {
			case redisOpSet:
				// Serialize/Marshal is handled by go-redis if we pass structs?
				// go-redis handles basic types. For generic V, we might need to marshal.
				// But go-redis Set accepts 'any'. It uses internal formatting.
				// If V is a struct, it uses fmt.Sprint by default or implements BinaryMarshaler.
				// It's safer to marshal to JSON explicitly to ensure we can unmarshal back.
				data, err := json.Marshal(item.v)
				if err == nil {
					c.redisClient.Set(c.ctx, item.k, data, item.d)
				}
			case redisOpDel:
				c.redisClient.Del(c.ctx, item.k)
			}
		}
	}
}

// Close stops the background Redis worker and waits for pending operations to complete.
func (c *cache[V]) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.redisCh != nil {
		close(c.redisCh)
		c.redisCh = nil
	}
	c.wg.Wait()
}

// Configures the cache to use a Redis client for L2 caching and async persistence.
// This also starts the async worker for Redis writes.
func (c *Cache[V]) WithRedis(cli RedisClient) *Cache[V] {
	c.cache.withRedis(cli)
	return c
}

func (c *cache[V]) withRedis(cli RedisClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.redisClient = cli
	if c.redisCh == nil {
		c.redisCh = make(chan redisItem[V], 1000) // Buffer for async writes
		c.ctx = context.Background()              // Or accept context
		c.wg.Add(1)
		go c.redisWorker(c.redisCh)
	}
}

// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
//
// NewFrom() also accepts an items map which will serve as the underlying map
// for the cache. This is useful for starting from a deserialized cache
// (serialized using e.g. gob.Encode() on c.Items()), or passing in e.g.
// make(map[string]Item, 500) to improve startup performance when the cache
// is expected to reach a certain minimum size.
//
// Only the cache's methods synchronize access to this map, so it is not
// recommended to keep any references to the map around after creating a cache.
// If need be, the map can be accessed at a later point using c.Items() (subject
// to the same caveat.)
//
// Note regarding serialization: When using e.g. gob, make sure to
// gob.Register() the individual types stored in the cache before encoding a
// map retrieved with c.Items(), and to register those same types before
// decoding a blob containing an items map.
func NewFrom[V any](defaultExpiration, cleanupInterval time.Duration, items map[string]Item[V]) *Cache[V] {
	return newCacheWithJanitor[V](defaultExpiration, cleanupInterval, items)
}
