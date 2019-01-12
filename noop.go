package cache

import (
	"fmt"
	"io"
	"time"
)

type NoopCache struct {
	*noopCache
	// If this is confusing, see the comment at the bottom of New()
}

type noopCache struct {
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *noopCache) Set(k string, x interface{}, d time.Duration) {
}

// Add an item to the cache, replacing any existing item, using the default
// expiration.
func (c *noopCache) SetDefault(k string, x interface{}) {
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *noopCache) Add(k string, x interface{}, d time.Duration) error {
	return nil
}

// Set a new value for the cache key only if it already exists, and the existing
// item hasn't expired. Returns an error otherwise.
func (c *noopCache) Replace(k string, x interface{}, d time.Duration) error {
	return nil
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *noopCache) Get(k string) (interface{}, bool) {
	return nil, false
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or nil, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.
func (c *noopCache) GetWithExpiration(k string) (interface{}, time.Time, bool) {
	return nil, time.Time{}, false
}

// Increment an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to increment it by n. To retrieve the incremented value, use one
// of the specialized methods, e.g. IncrementInt64.
func (c *noopCache) Increment(k string, n int64) error {
	return nil
}

// Increment an item of type float32 or float64 by n. Returns an error if the
// item's value is not floating point, if it was not found, or if it is not
// possible to increment it by n. Pass a negative number to decrement the
// value. To retrieve the incremented value, use one of the specialized methods,
// e.g. IncrementFloat64.
func (c *noopCache) IncrementFloat(k string, n float64) error {
	return nil
}

// Increment an item of type int by n. Returns an error if the item's value is
// not an int, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *noopCache) IncrementInt(k string, n int) (int, error) {
	return 0, nil
}

// Increment an item of type int8 by n. Returns an error if the item's value is
// not an int8, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *noopCache) IncrementInt8(k string, n int8) (int8, error) {
	return 0, nil
}

// Increment an item of type int16 by n. Returns an error if the item's value is
// not an int16, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *noopCache) IncrementInt16(k string, n int16) (int16, error) {
	return 0, nil
}

// Increment an item of type int32 by n. Returns an error if the item's value is
// not an int32, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *noopCache) IncrementInt32(k string, n int32) (int32, error) {
	return 0, nil
}

// Increment an item of type int64 by n. Returns an error if the item's value is
// not an int64, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *noopCache) IncrementInt64(k string, n int64) (int64, error) {
	return 0, nil
}

// Increment an item of type uint by n. Returns an error if the item's value is
// not an uint, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *noopCache) IncrementUint(k string, n uint) (uint, error) {
	return 0, nil
}

// Increment an item of type uintptr by n. Returns an error if the item's value
// is not an uintptr, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementUintptr(k string, n uintptr) (uintptr, error) {
	return 0, nil
}

// Increment an item of type uint8 by n. Returns an error if the item's value
// is not an uint8, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementUint8(k string, n uint8) (uint8, error) {
	return 0, nil
}

// Increment an item of type uint16 by n. Returns an error if the item's value
// is not an uint16, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementUint16(k string, n uint16) (uint16, error) {
	return 0, nil
}

// Increment an item of type uint32 by n. Returns an error if the item's value
// is not an uint32, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementUint32(k string, n uint32) (uint32, error) {
	return 0, nil
}

// Increment an item of type uint64 by n. Returns an error if the item's value
// is not an uint64, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementUint64(k string, n uint64) (uint64, error) {
	return 0, nil
}

// Increment an item of type float32 by n. Returns an error if the item's value
// is not an float32, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementFloat32(k string, n float32) (float32, error) {
	return 0, nil
}

// Increment an item of type float64 by n. Returns an error if the item's value
// is not an float64, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *noopCache) IncrementFloat64(k string, n float64) (float64, error) {
	return 0, nil
}

// Decrement an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to decrement it by n. To retrieve the decremented value, use one
// of the specialized methods, e.g. DecrementInt64.
func (c *noopCache) Decrement(k string, n int64) error {
	return fmt.Errorf("Item not found")
}

// Decrement an item of type float32 or float64 by n. Returns an error if the
// item's value is not floating point, if it was not found, or if it is not
// possible to decrement it by n. Pass a negative number to decrement the
// value. To retrieve the decremented value, use one of the specialized methods,
// e.g. DecrementFloat64.
func (c *noopCache) DecrementFloat(k string, n float64) error {
	return nil
}

// Decrement an item of type int by n. Returns an error if the item's value is
// not an int, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementInt(k string, n int) (int, error) {
	return 0, nil
}

// Decrement an item of type int8 by n. Returns an error if the item's value is
// not an int8, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementInt8(k string, n int8) (int8, error) {
	return 0, nil
}

// Decrement an item of type int16 by n. Returns an error if the item's value is
// not an int16, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementInt16(k string, n int16) (int16, error) {
	return 0, nil
}

// Decrement an item of type int32 by n. Returns an error if the item's value is
// not an int32, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementInt32(k string, n int32) (int32, error) {
	return 0, nil
}

// Decrement an item of type int64 by n. Returns an error if the item's value is
// not an int64, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementInt64(k string, n int64) (int64, error) {
	return 0, nil
}

// Decrement an item of type uint by n. Returns an error if the item's value is
// not an uint, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementUint(k string, n uint) (uint, error) {
	return 0, nil
}

// Decrement an item of type uintptr by n. Returns an error if the item's value
// is not an uintptr, or if it was not found. If there is no error, the
// decremented value is returned.
func (c *noopCache) DecrementUintptr(k string, n uintptr) (uintptr, error) {
	return 0, nil
}

// Decrement an item of type uint8 by n. Returns an error if the item's value is
// not an uint8, or if it was not found. If there is no error, the decremented
// value is returned.
func (c *noopCache) DecrementUint8(k string, n uint8) (uint8, error) {
	return 0, nil
}

// Decrement an item of type uint16 by n. Returns an error if the item's value
// is not an uint16, or if it was not found. If there is no error, the
// decremented value is returned.
func (c *noopCache) DecrementUint16(k string, n uint16) (uint16, error) {
	return 0, nil
}

// Decrement an item of type uint32 by n. Returns an error if the item's value
// is not an uint32, or if it was not found. If there is no error, the
// decremented value is returned.
func (c *noopCache) DecrementUint32(k string, n uint32) (uint32, error) {
	return 0, nil
}

// Decrement an item of type uint64 by n. Returns an error if the item's value
// is not an uint64, or if it was not found. If there is no error, the
// decremented value is returned.
func (c *noopCache) DecrementUint64(k string, n uint64) (uint64, error) {
	return 0, nil
}

// Decrement an item of type float32 by n. Returns an error if the item's value
// is not an float32, or if it was not found. If there is no error, the
// decremented value is returned.
func (c *noopCache) DecrementFloat32(k string, n float32) (float32, error) {
	return 0, nil
}

// Decrement an item of type float64 by n. Returns an error if the item's value
// is not an float64, or if it was not found. If there is no error, the
// decremented value is returned.
func (c *noopCache) DecrementFloat64(k string, n float64) (float64, error) {
	return 0, nil
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *noopCache) Delete(k string) {
}

// Delete all expired items from the cache.
func (c *noopCache) DeleteExpired() {
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *noopCache) OnEvicted(f func(string, interface{})) {
}

// Write the cache's items (using Gob) to an io.Writer.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *noopCache) Save(w io.Writer) (err error) {
	return nil
}

// Save the cache's items to the given filename, creating the file if it
// doesn't exist, and overwriting it if it does.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *noopCache) SaveFile(fname string) error {
	return nil
}

// Add (Gob-serialized) cache items from an io.Reader, excluding any items with
// keys that already exist (and haven't expired) in the current cache.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *noopCache) Load(r io.Reader) error {
	return nil
}

// Load and add cache items from the given filename, excluding any items with
// keys that already exist in the current cache.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *noopCache) LoadFile(fname string) error {
	return nil
	return nil
}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *noopCache) Items() map[string]Item {
	m := make(map[string]Item, 0)
	return m
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *noopCache) ItemCount() int {
	return 0
}

// Delete all items from the cache.
func (c *noopCache) Flush() {
}

func newNoopCache() *NoopCache {
	c := &noopCache{}
	C := &NoopCache{c}
	return C
}

// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func NewNoop(defaultExpiration, cleanupInterval time.Duration) *NoopCache {
	return newNoopCache()
}

// Return a Cacher interface implementing NoopCache
func NewNoopCacher(defaultExpiration, cleanupInterval time.Duration) Cacher {
	return newNoopCache()
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
func NewNoopFrom(defaultExpiration, cleanupInterval time.Duration, items map[string]Item) *NoopCache {
	return newNoopCache()
}

// Return a Cacher interface implementing NoopCache
func NewNoopCacherFrom(defaultExpiration, cleanupInterval time.Duration, items map[string]Item) Cacher {
	return NewNoopFrom(defaultExpiration, cleanupInterval, items)
}
