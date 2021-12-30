package cache

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

type Item [T comparable] struct {
	Object     T
	Expiration int64
}

// Returns true if the item has expired.
func (item Item[T]) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache [T comparable] struct {
	*cache[T]
	// If this is confusing, see the comment at the bottom of New()
}



type cache [T comparable] struct {
	defaultExpiration time.Duration
	items             map[string]Item[T]
	mu                sync.RWMutex
	onEvicted         func(string, T)
	janitor           *janitor
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache[T]) Set(k string, x T, d time.Duration) {
	// "Inlining" of set
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	c.items[k] = Item[T]{
		Object:     x,
		Expiration: e,
	}
	// TODO: Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mu.Unlock()
}

func (c *cache[T]) set(k string, x T, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = Item[T]{
		Object:     x,
		Expiration: e,
	}
}

// Add an item to the cache, replacing any existing item, using the default
// expiration.
func (c *cache[T]) SetDefault(k string, x T) {
	c.Set(k, x, DefaultExpiration)
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache[T]) Add(k string, x T, d time.Duration) error {
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
func (c *cache[T]) Replace(k string, x T, d time.Duration) error {
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

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache[T]) Get(k string) (T, bool) {
	c.mu.RLock()
	var zero T
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return zero, false
	}
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return zero, false
		}
	}
	c.mu.RUnlock()
	return item.Object, true
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or nil, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.
func (c *cache[T]) GetWithExpiration(k string) (T, time.Time, bool) {
	c.mu.RLock()
	var zero T
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return zero, time.Time{}, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return zero, time.Time{}, false
		}

		// Return the item and the expiration time
		c.mu.RUnlock()
		return item.Object, time.Unix(0, item.Expiration), true
	}

	// If expiration <= 0 (i.e. no expiration time set) then return the item
	// and a zeroed time.Time
	c.mu.RUnlock()
	return item.Object, time.Time{}, true
}

func (c *cache[T]) get(k string) (T, bool) {
	item, found := c.items[k]
	var zero T
	if !found {
		return zero, false
	}
	// "Inlining" of Expired
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			return zero, false
		}
	}
	return item.Object, true
}

// Increment and return an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to increment it by n.
func (c *cache[T]) Increment(k string, n int64) (T, error) {

	// TODO: Consider adding a constraint to avoid the type switch and provide
	// compile-time safety
	
	c.mu.Lock()
	var zero T
	v, found := c.items[k]
	if !found || v.Expired() {
		c.mu.Unlock()
		return zero, fmt.Errorf("Item %s not found", k)
	}
	// Generics does not (currently?) support type switching
	// To workaround, we convert the value into a interface{}, and switching on that
	var untypedValue interface{}
	
	untypedValue = v.Object
	switch untypedValue.(type) {
	case int:
		untypedValue = untypedValue.(int) + int(n)
	case int8:
		untypedValue = untypedValue.(int8) + int8(n)
	case int16:
		untypedValue = untypedValue.(int16) + int16(n)
	case int32:
		untypedValue= untypedValue.(int32) + int32(n)
	case int64:
		untypedValue = untypedValue.(int64) + n
	case uint:
		untypedValue = untypedValue.(uint) + uint(n)
	case uintptr:
		untypedValue = untypedValue.(uintptr) + uintptr(n)
	case uint8:
		untypedValue = untypedValue.(uint8) + uint8(n)
	case uint16:
		untypedValue = untypedValue.(uint16) + uint16(n)
	case uint32:
		untypedValue = untypedValue.(uint32) + uint32(n)
	case uint64:
		untypedValue = untypedValue.(uint64) + uint64(n)
	case float32:
		untypedValue = untypedValue.(float32) + float32(n)
	case float64:
		untypedValue = untypedValue.(float64) + float64(n)
	default:
		c.mu.Unlock()
		return zero, fmt.Errorf("The value for %s is not an integer", k)
	}
	v.Object = untypedValue.(T)
	c.items[k] = v
	c.mu.Unlock()
	return zero, nil
}

// Decrement and return an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to decrement it by n.
func (c *cache[T]) Decrement(k string, n int64) (T, error) {

	// TODO: Consider adding a constraint to avoid the type switch and provide
	// compile-time safety
	c.mu.Lock()
	var zero T
	v, found := c.items[k]
	if !found || v.Expired() {
		c.mu.Unlock()
		return zero, fmt.Errorf("Item %s not found", k)
	}
	// Generics does not (currently?) support type switching
	// To workaround, we convert the value into a interface{}, and switching on that
	var untypedValue interface{}
	
	untypedValue = v.Object
	switch untypedValue.(type) {
	case int:
		untypedValue = untypedValue.(int) - int(n)
	case int8:
		untypedValue = untypedValue.(int8) - int8(n)
	case int16:
		untypedValue = untypedValue.(int16) - int16(n)
	case int32:
		untypedValue= untypedValue.(int32) - int32(n)
	case int64:
		untypedValue = untypedValue.(int64) - n
	case uint:
		untypedValue = untypedValue.(uint) - uint(n)
	case uintptr:
		untypedValue = untypedValue.(uintptr) - uintptr(n)
	case uint8:
		untypedValue = untypedValue.(uint8) - uint8(n)
	case uint16:
		untypedValue = untypedValue.(uint16) - uint16(n)
	case uint32:
		untypedValue = untypedValue.(uint32) - uint32(n)
	case uint64:
		untypedValue = untypedValue.(uint64) - uint64(n)
	case float32:
		untypedValue = untypedValue.(float32) - float32(n)
	case float64:
		untypedValue = untypedValue.(float64) - float64(n)
	default:
		c.mu.Unlock()
		return zero, fmt.Errorf("The value for %s is not an integer", k)
	}
	v.Object = untypedValue.(T)
	c.items[k] = v
	c.mu.Unlock()
	return zero, nil
}


// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache[T]) Delete(k string) {
	c.mu.Lock()
	v, evicted := c.delete(k)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}
}

func (c *cache[T]) delete(k string) (T, bool) {
	var zero T
	if c.onEvicted != nil {
		if v, found := c.items[k]; found {
			delete(c.items, k)
			return v.Object, true
		}
	}
	delete(c.items, k)
	return zero, false
}

type keyAndValue[T comparable] struct {
	key   string
	value T
}

// Delete all expired items from the cache.
func (c *cache[T]) DeleteExpired() {
	var evictedItems []keyAndValue[T]
	now := time.Now().UnixNano()
	c.mu.Lock()
	for k, v := range c.items {
		// "Inlining" of expired
		if v.Expiration > 0 && now > v.Expiration {
			ov, evicted := c.delete(k)
			if evicted {
				evictedItems = append(evictedItems, keyAndValue[T]{k, ov})
			}
		}
	}
	c.mu.Unlock()
	for _, v := range evictedItems {
		c.onEvicted(v.key, v.value)
	}
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *cache[T]) OnEvicted(f func(string, T)) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}


// Write the cache's items (using Gob) to an io.Writer.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *cache[T]) Save(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("Error registering item types with Gob library")
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
func (c *cache[T]) SaveFile(fname string) error {
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
func (c *cache[T]) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	items := map[string]Item[T]{}
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
func (c *cache[T]) LoadFile(fname string) error {
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
func (c *cache[T]) Items() map[string]Item[T] {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := make(map[string]Item[T], len(c.items))
	now := time.Now().UnixNano()
	for k, v := range c.items {
		// "Inlining" of Expired
		if v.Expiration > 0 {
			if now > v.Expiration {
				continue
			}
		}
		m[k] = v
	}
	return m
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *cache[T]) ItemCount() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Delete all items from the cache.
func (c *cache[T]) Flush() {
	c.mu.Lock()
	c.items = map[string]Item[T]{}
	c.mu.Unlock()
}

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func runJanitor[T comparable](j *janitor, c *cache[T]) {
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

func stopJanitor[T comparable](c *Cache[T]) {
	c.janitor.stop <- true
}

func startJanitor[T comparable](c *cache[T], ci time.Duration) {
	j := &janitor{
		Interval: ci,
		stop:     make(chan bool),
	}
	c.janitor = j
	go runJanitor(j, c)
}

func newCache[T comparable](de time.Duration, m map[string]Item[T]) *cache[T] {
	if de == 0 {
		de = -1
	}
	c := &cache[T]{
		defaultExpiration: de,
		items:             m,
	}
	return c
}

func newCacheWithJanitor[T comparable](de time.Duration, ci time.Duration, m map[string]Item[T]) *Cache[T] {
	c := newCache(de, m)
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Cache[T]{c}
	if ci > 0 {
		startJanitor(c, ci)
		runtime.SetFinalizer(C, stopJanitor[T])
	}
	return C
}


// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func New[T comparable](defaultExpiration, cleanupInterval time.Duration) *Cache[T] {
	items := make(map[string]Item[T])
	return newCacheWithJanitor[T](defaultExpiration, cleanupInterval, items)
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
func NewFrom[T comparable](defaultExpiration, cleanupInterval time.Duration, items map[string]Item[T]) *Cache[T] {
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, items)
}
