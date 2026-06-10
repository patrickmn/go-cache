# go-cache

![Go](https://github.com/mysamimi/go-cache/actions/workflows/test.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/mysamimi/go-cache/v3)](https://goreportcard.com/report/github.com/mysamimi/go-cache/v3)
[![Go Reference](https://pkg.go.dev/badge/github.com/mysamimi/go-cache/v3.svg)](https://pkg.go.dev/github.com/mysamimi/go-cache/v3)

go-cache is an in-memory key:value store/cache similar to memcached that is suitable for applications running on a single machine. Its major advantage is that, being essentially a thread-safe `map[string]interface{}` with expiration times, it doesn't need to serialize or transmit its contents over the network.

**Key Features:**

*   **Sharding**: Reduces lock contention for high-concurrency workloads.
*   **Redis Integration**: Optional L2 caching and persistence layer using `go-redis` (supports v8 and v9).
*   **Memory-Authoritative Mode**: Optional write-behind-only mode where Redis is *never* read synchronously on the hot path — after warmup, reads are fully in-memory while writes still persist asynchronously.
*   **Single Round-Trip Reads**: Redis reads fetch the value and its TTL in one round-trip (pipeline / `GETEX`) when the adapter supports it, instead of a separate `GET` + `TTL`.
*   **Startup Warmup**: `WarmFromRedis` rebuilds in-memory state from Redis at boot, so durability survives restarts without any synchronous reads at runtime.
*   **Capacity Management**: Internal LRU-like eviction when memory limits are reached.
*   **Generics**: Type-safe API (Go 1.18+).
*   **SetNX**: Atomic "Set if Not Exists" operation, either synchronized with Redis (default) or local-authoritative with async persistence.
*   **Numeric Operations**: Atomic increment/decrement support for numeric types, persisted to Redis.
*   **Set Cache**: Track unique members per key, each with its own TTL — ideal for counting active sessions/devices per user.
*   **Graceful Shutdown**: Ensures pending Redis operations are completed before exit.
*   **Sync**: Force refresh items from Redis.
*   **Back-Pressure Control**: Observe dropped async writes via `DroppedWrites`, and optionally block briefly instead of dropping when the write-behind queue is full.
*   **Configurable Timeouts**: Fine-tune Redis L2 operation timeouts for all cache types.
*   **Performance**: Extremely low latency local operations (see [BENCHMARKS.md](BENCHMARKS.md)).

### Installation

`go get github.com/mysamimi/go-cache/v3`

---

## Recent Updates

*   **Memory-Authoritative / Write-Behind-Only Mode**: `WithWriteThroughOnly()` keeps the hot path (`Get`/`ModifyNumeric`/`SetNX`) fully in-memory — Redis is used purely as an async write-behind layer and is never read synchronously on a local miss.
*   **Single Round-Trip Redis Reads**: Added the optional `RedisGetTTLClient` interface (implemented by the bundled v8/v9 adapters via a pipeline). When available, a Redis read fetches value + TTL in one round-trip instead of separate `GET` and `TTL` calls.
*   **Startup Warmup**: `WarmFromRedis(keys)` loads existing Redis state into local memory at startup, for durability across restarts without runtime synchronous reads.
*   **Local-Authoritative SetNX**: In write-through-only mode, `SetNX` guarantees atomicity via the local shard lock and pushes the Redis `SETNX` asynchronously. The original cluster-wide synchronous behavior remains the default.
*   **Back-Pressure Metrics & Blocking**: Async write drops (when the queue is full) are now counted and exposed via `DroppedWrites()`. `WithBlockOnFull(d)` blocks briefly instead of dropping immediately.
*   **SetNX Support**: Added atomic `SetNX` (Set if Not Exists) operations seamlessly synchronized with Redis.
*   **Automatic Redis Fetching**: Enabled automatic Redis fetching for local cache misses and expired items, enhancing multi-instance synchronization.
*   **Configurable Timeouts**: Fine-tune Redis L2 operation timeouts for all cache types.
*   **Manual Sync Support**: Added `Sync` method to force refresh items from Redis across all cache types.
*   **SetCache & ShardedSetCache**: Introduced sets for tracking unique members with individual TTLs.
*   **Numeric Convenience Methods**: Added `Incr` and `Decr` methods for numeric cache operations.
*   **Performance Improvements**: Enhanced Redis synchronization logic and added comprehensive performance benchmarks.

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

#### How reads work (default: read-through)

By default the cache is **read-through**: a `Get` first checks local memory, and on a local miss (or a locally-expired item) it transparently falls back to Redis. If the key exists in Redis, the value is pulled back into local memory and returned. This keeps multiple instances loosely synchronized — a value written by one worker becomes visible to others on their next miss.

The Redis read fetches both the value and its remaining TTL. When the configured adapter implements `RedisGetTTLClient` (the bundled `redisv8` / `redisv9` adapters do, via a pipeline), this is a **single round-trip**; otherwise it falls back to a separate `GET` + `TTL`.

---

### Memory-Authoritative Mode (Write-Behind Only)

For latency-critical, high-throughput workloads you can make memory the source of truth and demote Redis to a pure **async write-behind** layer. In this mode the hot path is *never* blocked by a synchronous Redis read.

Enable it with `WithWriteThroughOnly()` (available on `Cache`, `NumericCache`, `ShardedCache`, and `ShardedNumericCache`):

```go
c := cache.NewShardedCache[MyStruct](16, 5*time.Minute, 10*time.Minute)
c.WithRedis(redisv9.New(rdb)).
    WithRedisTimeout(500 * time.Millisecond).
    WithWriteThroughOnly()
defer c.Close()
```

What changes in this mode:

*   **Reads stay in memory.** `Get`, `ModifyNumeric`, `SetNX`, and the `SetCache` getters no longer fall back to Redis on a local miss — a miss is a miss. After warmup, the hot path is 100% in-memory.
*   **Writes still persist.** `Set`, `Delete`, `Incr`/`Decr`, etc. are still propagated to Redis asynchronously through the write-behind worker.
*   **`SetNX` becomes local-authoritative.** Atomicity is guaranteed by the local shard lock (exactly one winner per process), and the Redis `SETNX` is sent asynchronously. The signature and return value are unchanged. *Note:* cluster-wide uniqueness is no longer enforced synchronously — use the default mode if you need that guarantee.
*   **Explicit reads still hit Redis.** `Sync(key)` and `WarmFromRedis(keys)` deliberately bypass this mode, since they are not on the hot path.

#### Warming up from Redis at startup

To get durability across restarts *without* paying for synchronous reads at runtime, load your state from Redis once at boot:

```go
// Rebuild in-memory state for known keys (e.g. after a restart).
loaded, err := c.WarmFromRedis([]string{"user:1", "user:2", "user:3"})
fmt.Printf("warmed %d keys from Redis\n", loaded)
```

Keys missing from Redis are skipped silently; `loaded` is the number of keys actually restored. For `ShardedCache`, keys are automatically routed to the correct shard. An error is returned only if Redis is not configured.

---

### Back-Pressure: Dropped Writes & Blocking

Async Redis writes flow through a buffered queue. If writes are produced faster than the Redis worker can drain them, the queue fills up. By default an overflowing write is **dropped** (to avoid stalling the application), but it is now **counted** rather than silently lost:

```go
// Number of async Redis writes dropped because the queue was full.
// On ShardedCache this is summed across all shards.
dropped := c.DroppedWrites()
if dropped > 0 {
    log.Printf("WARNING: %d cache writes were dropped before reaching Redis", dropped)
}
```

If losing a write is unacceptable (for example, an item about to be evicted from memory), configure the producer to **block briefly** instead of dropping:

```go
// When the queue is full, block for up to 50ms waiting for room.
// If still full after the timeout, the write is dropped and counted.
c.WithBlockOnFull(50 * time.Millisecond)
```

> **Trade-off:** blocking applies back-pressure to the calling goroutine (and, for `Set`/`Delete`, briefly while holding the shard lock), so keep the timeout short. A zero or negative duration restores the default drop-immediately behavior.

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
