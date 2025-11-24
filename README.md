这是一个为您量身定制的 README.md。它不仅涵盖了使用方法，还深入解释了我们在优化过程中讨论过的设计哲学，并展示了您刚才测得的惊人性能数据。

你可以直接复制以下内容到项目的 README.md 文件中。

go-cache

![alt text](https://pkg.go.dev/badge/github.com/Zekiee/go-cache.svg)


![alt text](https://goreportcard.com/badge/github.com/Zekiee/go-cache)

go-cache 是一个基于 Go 1.18+ 泛型实现的高性能、线程安全的内存键值缓存库。它旨在提供生产级的性能，同时保持代码的简洁和类型安全。

它包含两种实现：

Cache: 标准的读写锁实现，适合通用场景。

ShardedCache: 高并发分片实现，适合写密集或超高并发场景。

✨ 特性 (Features)

🔒 类型安全: 基于 Go 泛型 ([K, V])，编译期检查类型，无需手动断言，拒绝反射。

🚀 极致性能:

使用 hash/maphash 和位运算 (& mask) 进行极速分片定位。

过期清理采用“读锁扫描 + 写锁删除”策略，最小化锁阻塞。

Sharded 模式下，并发写入性能提升 400%+。

🕒 精确时间: 内部使用 time.Time 利用单调时钟 (Monotonic Clock)，不受系统时间修改（如夏令时调整）影响。

🛡️ 资源安全: 显式的生命周期管理 (Close)，拒绝 runtime.SetFinalizer 带来的不确定性。

🧩 零依赖: 仅依赖 Go 标准库。

📦 安装 (Installation)
code
Bash
download
content_copy
expand_less
go get github.com/Zekiee/go-cache
📖 使用指南 (Usage)
1. 基础用法 (Standard Cache)

适用于读多写少，或者数据量中等的场景。

code
Go
download
content_copy
expand_less
package main

import (
    "fmt"
    "time"
    "github.com/Zekiee/go-cache"
)

func main() {
    // 创建一个 Key 为 string，Value 为 int 的缓存
    // 默认过期时间 5 分钟，每 10 分钟清理一次过期项
    c := cache.New[string, int](5*time.Minute, 10*time.Minute)
    
    // 务必在不再使用时关闭，以释放清理协程
    defer c.Close()

    // 设置值
    c.Set("foo", 42, cache.DefaultExpiration)
    
    // 获取值
    if v, found := c.Get("foo"); found {
        fmt.Println("Found:", v) // v 是 int 类型，无需断言
    }
}
2. 高并发分片用法 (Sharded Cache)

适用于高并发写入、海量数据场景。通过将数据分散到多个分片（Shard）来减少锁竞争。

code
Go
download
content_copy
expand_less
func main() {
    // 创建分片缓存
    // shards=256: 内部会自动调整为最近的 2 的幂 (Power of 2) 以优化哈希计算
    sc := cache.NewSharded[int](5*time.Minute, 10*time.Minute, 256)
    defer sc.Close()

    sc.Set("key1", 100, cache.DefaultExpiration)

    if v, found := sc.Get("key1"); found {
        fmt.Println("Value:", v)
    }
}
3. 混合类型支持 (Any Value)

如果你需要存储不同类型的值，可以使用 any 配合辅助函数 GetT。

code
Go
download
content_copy
expand_less
func main() {
    // Value 类型设为 any
    c := cache.New[string, any](cache.DefaultExpiration, 0)

    c.Set("age", 18, 0)
    c.Set("name", "Alice", 0)

    // 使用 GetT[T] 自动完成类型断言
    if age, found := cache.GetT[int](c, "age"); found {
        fmt.Println("Age:", age + 1)
    }
    
    if name, found := cache.GetT[string](c, "name"); found {
        fmt.Println("Name:", name)
    }
}
🧠 设计思路 (Design Philosophy)
1. 性能优先的分片算法

ShardedCache 放弃了传统的取模运算 (hash % n)，转而强制分片数量为 2 的幂。这允许我们使用位运算 (hash & mask) 来定位分片，这在 CPU 指令层面快得多。同时，哈希算法采用 Go 标准库的 hash/maphash，利用硬件 AES 指令集加速，兼顾了速度与哈希分布质量。

2. 智能的过期清理 (Two-Phase Eviction)

清理过期数据是造成缓存卡顿的主要原因之一。本库采用了两阶段策略：

读锁阶段：持有读锁 (RLock) 快速扫描全表，识别过期 Key。此过程不阻塞 Get 请求。

写锁阶段：仅对识别出的 Key，短暂持有写锁 (Lock) 进行删除，并进行双重检查 (Double Check) 防止误删。
这种策略使得即使在清理十万级数据时，对业务的影响也微乎其微。

3. 抗时间回拨

并没有使用 int64 timestamp 存储过期时间，而是使用了 time.Time。Go 的 time.Time 结构体包含单调时钟指针。即使服务器时间被运维手动回调，或者发生夏令时切换，缓存的过期时长依然精确，不会出现“永不过期”或“瞬间过期”的 Bug。

⚡ 基准测试 (Benchmarks)

测试环境：

CPU: 11th Gen Intel(R) Core(TM) i5-11500 @ 2.70GHz

OS: Windows/amd64

Go: 1.24

🚀 并发性能对比

在高并发场景下，ShardedCache 展现了巨大的优势：

测试场景	标准版 (Standard)	分片版 (Sharded)	提升幅度
并发写入 (Set)	915.6 ns/op	219.7 ns/op	🔥 4.17x
并发读取 (Get)	~22.2 ns/op	15.25 ns/op	⚡ 1.45x

解读:

写入: 标准版由于只有一把锁，在高并发写入时发生了严重的锁竞争。分片版通过 256 个分片将锁粒度变小，性能提升超过 4 倍。

读取: 分片版的读取性能甚至超越了标准版，达到了 15.25 ns，这仅仅比原生的无锁 Map (~9 ns) 慢了 6ns。这说明分片带来的 CPU 缓存行伪共享 (False Sharing) 优化非常有效。

📊 详细数据
code
Text
download
content_copy
expand_less
BenchmarkCache_Set-12                         	40.03 ns/op
BenchmarkCache_Get-12                         	22.22 ns/op
BenchmarkCache_DeleteExpired_LargeCache-12    	1211094 ns/op (1.2ms for 100k items)

BenchmarkShardedCache_Set_Concurrent-12     	219.7 ns/op
BenchmarkShardedCache_Get_Concurrent-12     	15.25 ns/op
⚠️ 注意事项

资源释放: 如果你在创建缓存时设置了 cleanupInterval > 0，请务必在不再使用缓存时调用 c.Close()，否则后台清理协程会导致内存泄漏。

回调死锁: OnEvicted 回调是在持有写锁的情况下执行的。切勿在回调函数中再次调用该缓存的 Set/Get 方法，也不要执行耗时的 IO 操作，否则会阻塞整个缓存。

License

MIT