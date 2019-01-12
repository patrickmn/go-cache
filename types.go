package cache

import (
	"io"
	"time"
)

type Cacher interface {
	// Add an item to the cache only if an item doesn't already exist for the given
	// key, or if the existing item has expired. Returns an error otherwise.
	Add(k string, x interface{}, d time.Duration) error
	// Decrement an item of type int8 by n. Returns an error if the item's value is
	// not an int8, or if it was not found. If there is no error, the decremented
	// value is returned.
	Decrement(k string, n int64) error
	// Decrement an item of type float32 by n. Returns an error if the item's value
	// is not an float32, or if it was not found. If there is no error, the
	// decremented value is returned.
	DecrementFloat(k string, n float64) error
	// Decrement an item of type int16 by n. Returns an error if the item's value is
	// not an int16, or if it was not found. If there is no error, the decremented
	// value is returned.
	DecrementFloat32(k string, n float32) (float32, error)
	// Decrement an item of type float64 by n. Returns an error if the item's value
	// is not an float64, or if it was not found. If there is no error, the
	// decremented value is returned.
	DecrementFloat64(k string, n float64) (float64, error)
	// Decrement an item of type float32 or float64 by n. Returns an error if the
	// item's value is not floating point, if it was not found, or if it is not
	// possible to decrement it by n. Pass a negative number to decrement the
	// value. To retrieve the decremented value, use one of the specialized methods,
	// e.g. DecrementFloat64.
	DecrementInt(k string, n int) (int, error)
	// Decrement an item of type int, int8, int16, int32, int64, uintptr, uint,
	// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
	// item's value is not an integer, if it was not found, or if it is not
	// possible to decrement it by n. To retrieve the decremented value, use one
	// of the specialized methods, e.g. DecrementInt64.
	DecrementInt8(k string, n int8) (int8, error)
	// Decrement an item of type int by n. Returns an error if the item's value is
	// not an int, or if it was not found. If there is no error, the decremented
	// value is returned.
	DecrementInt16(k string, n int16) (int16, error)
	// Decrement an item of type int32 by n. Returns an error if the item's value is
	// not an int32, or if it was not found. If there is no error, the decremented
	// value is returned.
	DecrementInt32(k string, n int32) (int32, error)
	// Decrement an item of type int64 by n. Returns an error if the item's value is
	// not an int64, or if it was not found. If there is no error, the decremented
	// value is returned.
	DecrementInt64(k string, n int64) (int64, error)
	// Decrement an item of type uint8 by n. Returns an error if the item's value is
	// not an uint8, or if it was not found. If there is no error, the decremented
	// value is returned.
	DecrementUint(k string, n uint) (uint, error)
	// Decrement an item of type uintptr by n. Returns an error if the item's value
	// is not an uintptr, or if it was not found. If there is no error, the
	// decremented value is returned.
	DecrementUint8(k string, n uint8) (uint8, error)
	// Decrement an item of type uint16 by n. Returns an error if the item's value
	// is not an uint16, or if it was not found. If there is no error, the
	// decremented value is returned.
	DecrementUint16(k string, n uint16) (uint16, error)
	// Decrement an item of type uint32 by n. Returns an error if the item's value
	// is not an uint32, or if it was not found. If there is no error, the
	// decremented value is returned.
	DecrementUint32(k string, n uint32) (uint32, error)
	// Decrement an item of type uint64 by n. Returns an error if the item's value
	// is not an uint64, or if it was not found. If there is no error, the
	// decremented value is returned.
	DecrementUint64(k string, n uint64) (uint64, error)
	// Decrement an item of type uint by n. Returns an error if the item's value is
	// not an uint, or if it was not found. If there is no error, the decremented
	// value is returned.
	DecrementUintptr(k string, n uintptr) (uintptr, error)
	// Delete all expired items from the cache.
	DeleteExpired()
	// Delete an item from the cache. Does nothing if the key is not in the cache.
	Delete(k string)
	// Delete all items from the cache.
	Flush()
	// Get an item from the cache. Returns the item or nil, and a bool indicating
	// whether the key was found.
	Get(k string) (interface{}, bool)
	// GetWithExpiration returns an item and its expiration time from the cache.
	// It returns the item or nil, the expiration time if one is set (if the item
	// never expires a zero value for time.Time is returned), and a bool indicating
	// whether the key was found.
	GetWithExpiration(k string) (interface{}, time.Time, bool)
	// Increment an item of type int32 by n. Returns an error if the item's value is
	// not an int32, or if it was not found. If there is no error, the incremented
	// value is returned.
	Increment(k string, n int64) error
	// Increment an item of type uint16 by n. Returns an error if the item's value
	// is not an uint16, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementFloat(k string, n float64) error
	// Increment an item of type int16 by n. Returns an error if the item's value is
	// not an int16, or if it was not found. If there is no error, the incremented
	// value is returned.
	IncrementFloat32(k string, n float32) (float32, error)
	// Increment an item of type int64 by n. Returns an error if the item's value is
	// not an int64, or if it was not found. If there is no error, the incremented
	// value is returned.
	IncrementFloat64(k string, n float64) (float64, error)
	// Increment an item of type float32 or float64 by n. Returns an error if the
	// item's value is not floating point, if it was not found, or if it is not
	// possible to increment it by n. Pass a negative number to decrement the
	// value. To retrieve the incremented value, use one of the specialized methods,
	// e.g. IncrementFloat64.
	IncrementInt(k string, n int) (int, error)
	// Increment an item of type int, int8, int16, int32, int64, uintptr, uint,
	// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
	// item's value is not an integer, if it was not found, or if it is not
	// possible to increment it by n. To retrieve the incremented value, use one
	// of the specialized methods, e.g. IncrementInt64.
	IncrementInt8(k string, n int8) (int8, error)
	// Increment an item of type int by n. Returns an error if the item's value is
	// not an int, or if it was not found. If there is no error, the incremented
	// value is returned.
	IncrementInt16(k string, n int16) (int16, error)
	// Increment an item of type float32 by n. Returns an error if the item's value
	// is not an float32, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementInt32(k string, n int32) (int32, error)
	// Increment an item of type float64 by n. Returns an error if the item's value
	// is not an float64, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementInt64(k string, n int64) (int64, error)
	// Increment an item of type int8 by n. Returns an error if the item's value is
	// not an int8, or if it was not found. If there is no error, the incremented
	// value is returned.
	IncrementUint(k string, n uint) (uint, error)
	// Increment an item of type uintptr by n. Returns an error if the item's value
	// is not an uintptr, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementUint8(k string, n uint8) (uint8, error)
	// Increment an item of type uint by n. Returns an error if the item's value is
	// not an ui// Increment an item of type float32 by n. Returns an error if the item's value
	// is not an float32, or if it was not found. If there is no error, the
	// incremented value is returned.nt, or if it was not found. If there is no error, the incremented
	// value is returned.
	IncrementUint16(k string, n uint16) (uint16, error)
	// Increment an item of type uint32 by n. Returns an error if the item's value
	// is not an uint32, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementUint32(k string, n uint32) (uint32, error)
	// Increment an item of type uint64 by n. Returns an error if the item's value
	// is not an uint64, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementUint64(k string, n uint64) (uint64, error)
	// Increment an item of type uint8 by n. Returns an error if the item's value
	// is not an uint8, or if it was not found. If there is no error, the
	// incremented value is returned.
	IncrementUintptr(k string, n uintptr) (uintptr, error)
	// Returns the number of items in the cache. This may include items that have
	// expired, but have not yet been cleaned up.
	ItemCount() int
	// Copies all unexpired items in the cache into a new map and returns it.
	Items() map[string]Item
	// Load and add cache items from the given filename, excluding any items with
	// keys that already exist in the current cache.
	//
	// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
	// documentation for NewFrom().)
	LoadFile(fname string) error
	// Add (Gob-serialized) cache items from an io.Reader, excluding any items with
	// keys that already exist (and haven't expired) in the current cache.
	//
	// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
	// documentation for NewFrom().)
	Load(r io.Reader) error
	// Sets an (optional) function that is called with the key and value when an
	// item is evicted from the cache. (Including when it is deleted manually, but
	// not when it is overwritten.) Set to nil to disable.
	OnEvicted(f func(string, interface{}))
	// Set a new value for the cache key only if it already exists, and the existing
	// item hasn't expired. Returns an error otherwise.
	Replace(k string, x interface{}, d time.Duration) error
	// Save the cache's items to the given filename, creating the file if it
	// doesn't exist, and overwriting it if it does.
	//
	// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
	// documentation for NewFrom().)
	SaveFile(fname string) error
	// Write the cache's items (using Gob) to an io.Writer.
	//
	// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
	// documentation for NewFrom().)
	Save(w io.Writer) (err error)
	// Add an item to the cache, replacing any existing item, using the default
	// expiration.
	SetDefault(k string, x interface{})
	// Add an item to the cache, replacing any existing item. If the duration is 0
	// (DefaultExpiration), the cache's default expiration time is used. If it is -1
	// (NoExpiration), the item never expires.
	Set(k string, x interface{}, d time.Duration)
}
