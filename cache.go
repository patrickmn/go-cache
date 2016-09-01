package cache

// The package is used as a template, don't use it directly!

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Attr is cachmap attribute
type Attr_tpl struct {
	// An (optional) function that is called with the key and value when an
	// item is evicted from the cache. (Including when it is deleted manually, but
	// not when it is overwritten.) Set to nil to disable.
	OnEvicted func(k string, v ValueType_tpl)

	DefaultCleanupInterval time.Duration // Default clean interval, this is a time interval to cleanup expired items
	DefaultExpiration      time.Duration // Default expiration duration
	Size                   int64         // Initial size of map
}

// Item struct
type Item_tpl struct {
	Object     ValueType_tpl
	Expiration int64
}

// Expired returns true if the item has expired.
func (item Item_tpl) Expired() bool {
	return item.Expiration != 0 && time.Now().UnixNano() > item.Expiration
}

// Cache struct
type Cache_tpl struct {
	*cache_tpl
	// If this is confusing, see the comment at the bottom of New()
}

type cache_tpl struct {
	defaultExpiration time.Duration
	items             map[string]Item_tpl
	mu                sync.RWMutex
	onEvicted         func(string, ValueType_tpl)
	stop              chan bool
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache_tpl) Set(k string, x ValueType_tpl, d time.Duration) {
	// "Inlining" of set
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	c.items[k] = Item_tpl{
		Object:     x,
		Expiration: e,
	}
	// TODO: Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mu.Unlock()
}

func (c *cache_tpl) set(k string, x ValueType_tpl, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = Item_tpl{
		Object:     x,
		Expiration: e,
	}
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache_tpl) Add(k string, x ValueType_tpl, d time.Duration) error {
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
func (c *cache_tpl) Replace(k string, x ValueType_tpl, d time.Duration) error {
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
func (c *cache_tpl) Get(k string) (ValueType_tpl, bool) {
	c.mu.RLock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	// TODO: inline time.Now implementation
	if !found || item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		c.mu.RUnlock()
		return ValueType_tpl(0), false
	}
	c.mu.RUnlock()
	return item.Object, true
}

func (c *cache_tpl) get(k string) (*ValueType_tpl, bool) {
	item, found := c.items[k]
	if !found || item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		return nil, false
	}
	return &item.Object, true
}

// MARK_Numberic_tpl_begin

// Increment an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to increment it by n. To retrieve the incremented value, use one
// of the specialized methods, e.g. IncrementInt64.
func (c *cache_tpl) Increment(k string, n ValueType_tpl) error {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.mu.Unlock()
		return fmt.Errorf("Item %s not found", k)
	}
	v.Object += n
	c.items[k] = v
	c.mu.Unlock()
	return nil
}

// Decrement an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to decrement it by n. To retrieve the decremented value, use one
// of the specialized methods, e.g. DecrementInt64.
func (c *cache_tpl) Decrement(k string, n ValueType_tpl) error {
	// TODO: Implement Increment and Decrement more cleanly.
	// (Cannot do Increment(k, n*-1) for uints.)
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.mu.Unlock()
		return fmt.Errorf("Item not found")
	}
	v.Object -= n
	c.items[k] = v
	c.mu.Unlock()
	return nil
}

// MARK_Numberic_tpl_end

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache_tpl) Delete(k string) {
	// fast path
	if c.onEvicted == nil {
		c.mu.Lock()
		c.deleteFast(k)
		c.mu.Unlock()
		return
	}
	// slow path
	c.mu.Lock()
	v, evicted := c.delete(k)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}
}

func (c *cache_tpl) delete(k string) (ValueType_tpl, bool) {
	if v, found := c.items[k]; found {
		delete(c.items, k)
		return v.Object, true
	}
	//TODO: zeroValue
	return 0, false
}

func (c *cache_tpl) deleteFast(k string) {
	delete(c.items, k)
}

// Delete all expired items from the cache.
func (c *cache_tpl) DeleteExpired() {
	var evictedItems []struct {
		key   string
		value ValueType_tpl
	}
	now := time.Now().UnixNano()
	// fast path
	if c.onEvicted == nil {
		c.mu.Lock()
		for k, v := range c.items {
			// "Inlining" of expired
			if v.Expiration > 0 && now > v.Expiration {
				c.deleteFast(k)
			}
		}
		c.mu.Unlock()
		return
	}
	// slow path
	c.mu.Lock()
	for k, v := range c.items {
		// "Inlining" of expired
		if v.Expiration > 0 && now > v.Expiration {
			ov, evicted := c.delete(k)
			if evicted {
				evictedItems = append(evictedItems, struct {
					key   string
					value ValueType_tpl
				}{k, ov})
			}
		}
	}
	c.mu.Unlock()
	for _, v := range evictedItems {
		c.onEvicted(v.key, v.value)
	}
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up. Equivalent to len(c.Items()).
func (c *cache_tpl) ItemCount() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Delete all items from the cache.
func (c *cache_tpl) Flush() {
	c.mu.Lock()
	c.items = map[string]Item_tpl{}
	c.mu.Unlock()
}

// _ *cache_tpl is used as a namespace, so that "stopJanitor" will not conflicts for the name.
func (_ *cache_tpl) stopJanitor(c *Cache_tpl) {
	c.stop <- true
}

func (c *cache_tpl) runJanitor(ci time.Duration) {
	c.stop = make(chan bool)
	go func() {
		ticker := time.NewTicker(ci)
		for {
			select {
			case <-ticker.C:
				c.DeleteExpired()
			case <-c.stop:
				ticker.Stop()
				return
			}
		}
	}()

}

// New Returns a new cache with a given default expiration duration and
// cleanup interval. If the expiration duration is less than one
// (or NoExpiration), the items in the cache never expire (by default),
// and must be deleted manually. If the cleanup interval is less than one,
// expired items are not deleted from the cache before calling c.DeleteExpired().
//
func New_tpl(attr Attr_tpl) *Cache_tpl {
	items := make(map[string]Item_tpl, attr.Size)
	if attr.DefaultExpiration == 0 {
		attr.DefaultExpiration = -1
	}
	c := &cache_tpl{
		defaultExpiration: attr.DefaultExpiration,
		items:             items,
	}
	c.onEvicted = attr.OnEvicted
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Cache_tpl{c}
	if attr.DefaultCleanupInterval > 0 {
		c.runJanitor(attr.DefaultCleanupInterval)
		runtime.SetFinalizer(C, c.stopJanitor)
	}
	return C
}
