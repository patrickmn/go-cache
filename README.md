# go-cache

![Go](https://github.com/mysamimi/go-cache/actions/workflows/test.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/mysamimi/go-cache/v3)](https://goreportcard.com/report/github.com/mysamimi/go-cache/v3)
[![Go Reference](https://pkg.go.dev/badge/github.com/mysamimi/go-cache/v3.svg)](https://pkg.go.dev/github.com/mysamimi/go-cache/v3)

go-cache is an in-memory key:value store/cache similar to memcached that is suitable for applications running on a single machine. Its major advantage is that, being essentially a thread-safe `map[string]interface{}` with expiration times, it doesn't need to serialize or transmit its contents over the network.

**Key Features:**

*   **Sharding**: Reduces lock contention for high-concurrency workloads.
*   **Redis Integration**: Optional L2 caching and persistence layer using `go-redis` (supports v8 and v9).
*   **Capacity Management**: Internal LRU-like eviction when memory limits are reached.
*   **Generics**: Type-safe API (Go 1.18+).
*   **SetNX**: Atomic "Set if Not Exists" operation, seamlessly synchronized with Redis.
*   **Numeric Operations**: Atomic increment/decrement support for numeric types, persisted to Redis.
*   **Set Cache**: Track unique members per key, each with its own TTL — ideal for counting active sessions/devices per user.
*   **Graceful Shutdown**: Ensures pending Redis operations are completed before exit.
*   **Sync**: Force refresh items from Redis.
*   **Configurable Timeouts**: Fine-tune Redis L2 operation timeouts for all cache types.
*   **Performance**: Extremely low latency local operations (see [BENCHMARKS.md](BENCHMARKS.md)).

### Installation

`go get github.com/mysamimi/go-cache/v3`

---

## Usage

### Basic Cache

```go
import (
    "fmt"
    "time"
    "github.com/mysamimi/go-cache/v3"
)

func main() {
    // Create a cache with a default expiration of 5 minutes,
    // purging expired items every 10 minutes.
    c := cache.New[string](5*time.Minute, 10*time.Minute)

    c.Set("foo", "bar", cache.DefaultExpiration)

    // SetNX: Only sets the key if it does not already exist.
    // Seamlessly integrates with Redis SetNX if configured.
    set := c.SetNX("foo", "baz", cache.DefaultExpiration)
    fmt.Println(set) // false, since "foo" already exists

    foo, found := c.Get("foo")
    if found == cache.Found {
        fmt.Println(foo) // bar
    }
}
```

---

### Sharded Cache (Recommended for High Concurrency)

`ShardedCache` automatically partitions keys into multiple buckets to reduce lock contention.

```go
// 16 shards, 5-min default expiry, 10-min janitor interval
c := cache.NewShardedCache[string](16, 5*time.Minute, 10*time.Minute)

c.Set("foo", "bar", cache.DefaultExpiration)
val, found := c.Get("foo")
```

---

### Redis Integration (L2 Cache & Persistence)

Writes (Set/Delete/ModifyNumeric) are **asynchronous** to Redis. However, modification operations like `ModifyNumeric` and `AddMember` will automatically **synchronize** with Redis before updating the local cache to ensure consistency across multiple instances.

Supports both `go-redis/v8` and `go-redis/v9` via adapters.

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/mysamimi/go-cache/v3"
    redisv9 "github.com/mysamimi/go-cache/v3/redis/v9"
)

func main() {
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

    c := cache.NewShardedCache[MyStruct](16, 5*time.Minute, 10*time.Minute)
    c.WithRedis(redisv9.New(rdb))
    defer c.Close() // flush pending Redis writes before exit

    c.Set("key", MyStruct{Val: 1}, cache.DefaultExpiration)
}
```

#### Redis configuration (Timeout & Sync)

Control how the cache interacts with Redis:

```go
// Set a 500ms timeout for Redis operations (optional)
c.WithRedisTimeout(500 * time.Millisecond)

// Force-refresh a key from Redis (e.g. when another service has updated it)
// Available for all cache types (Cache, NumericCache, SetCache)
val, err := c.Sync("key")
```

---

### Capacity & Eviction

```go
// Evict when more than 1 000 items are held (split across shards)
c.WithCapacity(1000)

c.OnEvicted(func(k string, v MyStruct) {
    fmt.Printf("evicted: %s\n", k)
})
```

---

### Numeric Operations

`NumericCache` / `ShardedNumericCache` provide atomic increment and decrement, persisted to Redis.

```go
nc := cache.NewShardedNumeric[int64](16, 5*time.Minute, 10*time.Minute)
nc.WithRedis(redisv9.New(rdb))

newVal, err := nc.Incr("counter", 1)  // +1
newVal, err  = nc.Decr("counter", 5)  // -5

// Low-level form:
newVal, err = nc.ModifyNumeric("counter", 5, false)
```

---

### Set Cache — Unique Member Tracking

`SetCache` and `ShardedSetCache` track a **set of unique members** per key, where each member has its own TTL. This is useful for counting active sessions, devices, or viewers — anything where membership is time-bounded and keyed by a parent entity.

> This pattern is the Go equivalent of the Redis `SMEMBERS` + per-key `EXISTS` cleanup pattern commonly used in Node.js.

#### SetCache

```go
sc := cache.NewSetCache(5*time.Minute, 10*time.Minute)
```

##### AddMember — add or refresh a member

```go
// setKey   = the parent key (e.g. user ID)
// member   = the unique member (e.g. device/session ID)
// memberTTL = how long this member is considered active
// setTTL   = TTL of the parent set key itself
count, isNew := sc.AddMember("user:42", "device-abc", 30*time.Second, 5*time.Minute)
// count=1, isNew=true

// Re-add the same device (heartbeat / refresh)
count, isNew = sc.AddMember("user:42", "device-abc", 30*time.Second, 5*time.Minute)
// count=1, isNew=false  ← same member, TTL refreshed

// Add a second device
count, isNew = sc.AddMember("user:42", "device-xyz", 30*time.Second, 5*time.Minute)
// count=2, isNew=true
```

##### CheckAndClean — enforce a concurrency limit

`CheckAndClean` is the Go port of the Node.js `checkValueInListAndCleanUp` function. It:

1. Checks whether the member is already active.
2. Returns the (virtual) count immediately **without cleanup** if `count ≤ limit`.
3. Otherwise prunes every expired member, then re-counts.

> **Important:** `CheckAndClean` does **not** write the member to the set. Call `AddMember` separately once you decide to allow the session.

```go
// Example: allow at most 3 simultaneous devices per user

// Suppose user:42 already has device-abc (alive) and device-old (expired).
count, isNew := sc.CheckAndClean("user:42", "device-new", 3)
// Projected count (2+1) = 3 ≤ limit=3 → no cleanup, fast return.
// count=3, isNew=true

// If there were 3 alive + 1 expired and we add a 4th:
// count=5 > 3 → prune expired → count=4 → still count=4, isNew=true
// Caller can decide to reject or allow.

if count <= 3 {
    // Allow and record the new session
    sc.AddMember("user:42", "device-new", 30*time.Second, 5*time.Minute)
} else {
    fmt.Println("too many active devices")
}
```

#### Full device-limit example

```go
func canWatch(sc *cache.SetCache, userID, deviceID string) (bool, int) {
    const maxDevices = 3

    count, isNew := sc.CheckAndClean(userID, deviceID, maxDevices)

    if !isNew {
        // Device already registered — renew its heartbeat TTL
        sc.AddMember(userID, deviceID, 30*time.Second, 5*time.Minute)
        return true, count
    }

    if count > maxDevices {
        return false, count // limit exceeded even after cleanup
    }

    // New device, within limit — register it
    sc.AddMember(userID, deviceID, 30*time.Second, 5*time.Minute)
    return true, count
}
```

##### Other SetCache methods

```go
// Check if a member is alive
alive := sc.HasMember("user:42", "device-abc") // true / false

// Get all live member IDs
members := sc.Members("user:42") // []string{"device-abc", "device-xyz"}

// Count without cleanup
n := sc.Count("user:42") // 2

// Prune expired members in-place and return remaining live count
n = sc.CleanAndCount("user:42")

// Remove a specific member (e.g. on explicit logout)
sc.RemoveMember("user:42", "device-abc")

// Remove all members for a key
sc.DeleteSet("user:42")

// Clear everything
sc.Flush()
```

#### ShardedSetCache (Recommended for High Concurrency)

`ShardedSetCache` wraps multiple `SetCache` shards; different set keys are handled by independent shards, eliminating cross-key lock contention.

```go
// 16 shards, 5-min parent-key TTL, 10-min janitor interval
ssc := cache.NewShardedSetCache(16, 5*time.Minute, 10*time.Minute)

// All SetCache methods are available on ShardedSetCache
count, isNew := ssc.AddMember("user:42", "device-abc", 30*time.Second, 5*time.Minute)
count, isNew  = ssc.CheckAndClean("user:42", "device-new", 3)
members       := ssc.Members("user:42")
ssc.RemoveMember("user:42", "device-abc")
ssc.DeleteSet("user:42")

// Redis backend (all shards)
ssc.WithRedis(redisv9.New(rdb))
defer ssc.Close()
```

#### SetCache + Redis

When Redis is attached, the `setData` (the member→expiry map) is persisted as JSON. Shared across instances, the set automatically **synchronizes** with Redis during modification operations (like `AddMember`), ensuring that updates from one worker are visible to others.

You can also manually synchronize a set:

```go
// Fetches latest set data from Redis, merges change, then writes back asynchronously
count, err := sc.Sync("user:42")
```
