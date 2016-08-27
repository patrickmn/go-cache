[![Build Status](https://travis-ci.org/ggaaooppeenngg/cachemap.svg?branch=master)](https://travis-ci.org/ggaaooppeenngg/cachemap)
[![Go Report Card](https://goreportcard.com/badge/github.com/ggaaooppeenngg/cachemap)](https://goreportcard.com/report/github.com/ggaaooppeenngg/cachemap)
[![GoDoc](https://godoc.org/github.com/ggaaooppeenngg/cachemap?status.svg)](https://godoc.org/github.com/ggaaooppeenngg/cachemap)
# cachemap

cachemap is an in-memory key:value store/cache similar to memcached that is
suitable for applications running on a single machine. Its major advantage is
that, being essentially a thread-safe `map[string]interface{}` with expiration
times, it doesn't need to serialize or transmit its contents over the network.

Any object can be stored, for a given duration or forever, and the cache can be
safely used by multiple goroutines.

Although cachemap isn't meant to be used as a persistent datastore, the entire
cache can be saved to and loaded from a file (using `c.Items()` to retrieve the
items map to serialize, and `NewFrom()` to create a cache from a deserialized
one) to recover from downtime quickly. (See the docs for `NewFrom()` for caveats.)

### Installation

`go get github.com/ggaaooppeenngg/cachemap`

### Usage

```go
	import (
		"fmt"
		"github.com/patrickmn/cachemap"
		"time"
	)

	func main() {

		// Create a cache with a default expiration time of 5 minutes, and which
		// purges expired items every 30 seconds
		c := cache.New(5*time.Minute, 30*time.Second)

		// Set the value of the key "foo" to "bar", with the default expiration time
		c.Set("foo", "bar", cache.DefaultExpiration)

		// Set the value of the key "baz" to 42, with no expiration time
		// (the item won't be removed until it is re-set, or removed using
		// c.Delete("baz")
		c.Set("baz", 42, cache.NoExpiration)

		// Get the string associated with the key "foo" from the cache
		foo, found := c.Get("foo")
		if found {
			fmt.Println(foo)
		}

		// Since Go is statically typed, and cache values can be anything, type
		// assertion is needed when values are being passed to functions that don't
		// take arbitrary types, (i.e. interface{}). The simplest way to do this for
		// values which will only be used once--e.g. for passing to another
		// function--is:
		foo, found := c.Get("foo")
		if found {
			MyFunction(foo.(string))
		}

		// This gets tedious if the value is used several times in the same function.
		// You might do either of the following instead:
		if x, found := c.Get("foo"); found {
			foo := x.(string)
			// ...
		}
		// or
		var foo string
		if x, found := c.Get("foo"); found {
			foo = x.(string)
		}
		// ...
		// foo can then be passed around freely as a string

		// Want performance? Store pointers!
		c.Set("foo", &MyStruct, cache.DefaultExpiration)
		if x, found := c.Get("foo"); found {
			foo := x.(*MyStruct)
			// ...
		}

		// If you store a reference type like a pointer, slice, map or channel, you
		// do not need to run Set if you modify the underlying data. The cached
		// reference points to the same memory, so if you modify a struct whose
		// pointer you've stored in the cache, retrieving that pointer with Get will
		// point you to the same data:
		foo := &MyStruct{Num: 1}
		c.Set("foo", foo, cache.DefaultExpiration)
		// ...
		x, _ := c.Get("foo")
		foo := x.(*MyStruct)
		fmt.Println(foo.Num)
		// ...
		foo.Num++
		// ...
		x, _ := c.Get("foo")
		foo := x.(*MyStruct)
		foo.Println(foo.Num)

		// will print:
		// 1
		// 2

	}
```

### Benchmark

| benchmark\package                                   | go-cache              | cachemap             |
|-----------------------------------------------------|-----------------------|----------------------|
| BenchmarkCacheGetExpiring-v                         | 30000000,46.3 ns/op   | 20000000,43.4 ns/op  |
| BenchmarkCacheGetNotExpiring-v                      | 50000000,29.6 ns/op   | 50000000,29.6 ns/op  |
| BenchmarkRWMutexMapGet-x                            | 50000000,26.7 ns/op   | 50000000,26.6 ns/op  |
| BenchmarkRWMutexInterfaceMapGetStruct-x             | 20000000,75.1 ns/op   | 20000000,66.1 ns/op  |
| BenchmarkRWMutexInterfaceMapGetString-x             | 20000000,75.3 ns/op   | 20000000,67.6 ns/op  |
| BenchmarkCacheGetConcurrentExpiring-v               | 20000000,67.8 ns/op   | 20000000,68.9 ns/op  |
| BenchmarkCacheGetConcurrentNotExpiring-v            | 20000000,69.2 ns/op   | 20000000,68.6 ns/op  |
| BenchmarkRWMutexMapGetConcurrent-x                  | 30000000,57.4 ns/op   | 20000000,64.7 ns/op  |
| BenchmarkCacheGetManyConcurrentExpiring-v           | 100000000,68.0 ns/op  | 100000000,66.7 ns/op |
| BenchmarkCacheGetManyConcurrentNotExpiring-v        | 2000000000,68.3 ns/op | 20000000,69.3 ns/op  |
| BenchmarkCacheSetExpiring-4                         | 10000000,173 ns/op    | 20000000,91.4 ns/op  |
| BenchmarkCacheSetNotExpiring-4                      | 10000000,123 ns/op    | 20000000,100 ns/op   |
| BenchmarkRWMutexMapSet-4                            | 20000000,88.5 ns/op   | 20000000,74.5 ns/op  |
| BenchmarkCacheSetDelete-4                           | 5000000,257 ns/op     | 10000000,151 ns/op   |
| BenchmarkRWMutexMapSetDelete-4                      | 10000000,180 ns/op    | 10000000,154 ns/op   |
| BenchmarkCacheSetDeleteSingleLock-4                 | 10000000,211 ns/op    | 20000000,118 ns/op   |
| BenchmarkRWMutexMapSetDeleteSingleLock-4            | 10000000,142 ns/op    | 20000000,118 ns/op   |
| BenchmarkIncrementInt-4                             | 10000000,167 ns/op    |                      |
| BenchmarkDeleteExpiredLoop-4                        | 500,2584384 ns/op     | 1000,2173019 ns/op   |
| BenchmarkShardedCacheGetExpiring-4                  | 20000000,79.5 ns/op   | 20000000,67.9 ns/op  |
| BenchmarkShardedCacheGetNotExpiring-4               | 30000000,59.3 ns/op   | 20000000,49.9 ns/op  |
| BenchmarkShardedCacheGetManyConcurrentExpiring-4    | 2000000000,52.4 ns/op | 10000000,75.8 ns/op  |
| BenchmarkShardedCacheGetManyConcurrentNotExpiring-4 | 100000000,68.2 ns/op  | 20000000,75.8 ns/op  |
