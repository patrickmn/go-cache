package cache

import (
	"fmt"
	"time"

	"golang.org/x/exp/constraints"
)

type OrderedCache[K comparable, V constraints.Ordered] struct {
	*orderedCache[K, V]
}

type orderedCache[K comparable, V constraints.Ordered] struct {
	*Cache[K, V]
}

// Increment an item of type by n.
// Returns incremented item or an error if it was not found.
func (c *orderedCache[K, V]) Increment(k K, n V) (V, error) {
	var zeroValue V
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.mu.Unlock()
		return zeroValue, fmt.Errorf("Item %v not found", k)
	}
	res := v.Object + n
	v.Object = res
	c.items[k] = v
	c.mu.Unlock()
	return res, nil
}

// Return a new ordered cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func NewOrderedCache[K comparable, V constraints.Ordered](defaultExpiration, cleanupInterval time.Duration) *OrderedCache[K, V] {
	return &OrderedCache[K, V]{
		orderedCache: &orderedCache[K, V]{
			Cache: New[K, V](defaultExpiration, cleanupInterval),
		},
	}
}

// Return a new ordered cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
//
// NewFrom() also accepts an items map which will serve as the underlying map
// for the cache. This is useful for starting from a deserialized cache
// (serialized using e.g. gob.Encode() on c.Items()), or passing in e.g.
// make(map[string]Item[int], 500) to improve startup performance when the cache
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
func NewOrderedCacheFrom[K comparable, V constraints.Ordered](defaultExpiration, cleanupInterval time.Duration, items map[K]Item[V]) *OrderedCache[K, V] {
	return &OrderedCache[K, V]{
		orderedCache: &orderedCache[K, V]{
			Cache: NewFrom(defaultExpiration, cleanupInterval, items),
		},
	}
}
