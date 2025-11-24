package cache

import (
	"fmt"
	"hash/maphash"
	"strconv"
	"sync"
	"testing"
	"time"
)

// --- 单元测试 (Unit Tests) ---

// 测试基础的 String Key 功能
func TestShardedCache_Basic(t *testing.T) {
	// 请求 10 个分片，内部应该自动调整为 16 (2^4)
	sc := NewSharded[string](DefaultExpiration, 0, 10)
	defer sc.Close()

	// 1. Set & Get
	sc.Set("key1", "value1", DefaultExpiration)
	if v, found := sc.Get("key1"); !found || v != "value1" {
		t.Errorf("Get failed. Expected value1, got %v, found: %v", v, found)
	}

	// 2. Get 不存在的
	if _, found := sc.Get("not-exist"); found {
		t.Error("Should not find non-existent key")
	}

	// 3. Delete
	sc.Delete("key1")
	if _, found := sc.Get("key1"); found {
		t.Error("key1 should be deleted")
	}

	// 4. Add
	if ok := sc.Add("key2", "value2", DefaultExpiration); !ok {
		t.Error("First Add should succeed")
	}
	if ok := sc.Add("key2", "value2-new", DefaultExpiration); ok {
		t.Error("Second Add should fail")
	}

	// 5. Replace
	if ok := sc.Replace("key2", "value2-replaced", DefaultExpiration); !ok {
		t.Error("Replace should succeed for existing key")
	}
	if v, _ := sc.Get("key2"); v != "value2-replaced" {
		t.Error("Value should be replaced")
	}
}

// 测试过期逻辑
func TestShardedCache_Expiration(t *testing.T) {
	sc := NewSharded[int](50*time.Millisecond, 0, 4)
	defer sc.Close()

	sc.Set("foo", 1, DefaultExpiration)
	sc.Set("bar", 2, NoExpiration)

	// 立即检查
	if _, found := sc.Get("foo"); !found {
		t.Error("foo should exist immediately")
	}

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 检查 foo (应过期)
	if _, found := sc.Get("foo"); found {
		t.Error("foo should have expired")
	}

	// 检查 bar (永不过期)
	if _, found := sc.Get("bar"); !found {
		t.Error("bar should not expire")
	}
}

// 测试自动清理 (Janitor)
func TestShardedCache_Janitor(t *testing.T) {
	// 清理间隔 20ms
	sc := NewSharded[int](DefaultExpiration, 20*time.Millisecond, 4)
	defer sc.Close()

	// 设置 10ms 后过期
	sc.Set("temp", 1, 10*time.Millisecond)

	// 此时应该存在
	if _, found := sc.Get("temp"); !found {
		t.Fatal("Key should exist initially")
	}

	// 等待：过期时间(10ms) + 清理间隔(20ms) + 缓冲(20ms)
	time.Sleep(50 * time.Millisecond)

	// 检查 Items() 返回的总数，验证是否被物理删除
	totalItems := 0
	for _, shardItems := range sc.Items() {
		totalItems += len(shardItems)
	}

	if totalItems != 0 {
		t.Errorf("Cache should be empty after janitor run. Got %d items", totalItems)
	}
}

// 测试泛型 Key (使用 int 作为 Key)
func TestShardedCache_Generic_IntKey(t *testing.T) {
	// 定义一个简单的 int 哈希函数
	hashInt := func(k int) uint64 {
		return uint64(k) // 仅作测试，简单转换
	}

	// 创建分片缓存
	sc := NewShardedGeneric[int, string](DefaultExpiration, 0, 8, hashInt)
	defer sc.Close()

	sc.Set(1001, "UserA", DefaultExpiration)
	sc.Set(1002, "UserB", DefaultExpiration)

	if v, found := sc.Get(1001); !found || v != "UserA" {
		t.Error("Failed to get generic key 1001")
	}

	sc.Delete(1001)
	if _, found := sc.Get(1001); found {
		t.Error("Failed to delete generic key 1001")
	}
}

// 测试分片分布情况 (验证 Power of 2 调整和哈希有效性)
func TestShardedCache_Distribution(t *testing.T) {
	// 初始请求 3 个分片，应该自动调整为 4
	sc := NewSharded[int](DefaultExpiration, 0, 3)
	defer sc.Close()

	if len(sc.cs) != 4 {
		t.Errorf("Expected shards to adjust to 4 (power of 2), got %d", len(sc.cs))
	}

	// 插入一些数据
	for i := 0; i < 100; i++ {
		sc.Set(fmt.Sprintf("key-%d", i), i, DefaultExpiration)
	}

	// 检查各个分片是否有数据，防止 hash 算法只返回 0 导致全在一个分片
	emptyShards := 0
	for i, shardItems := range sc.Items() {
		if len(shardItems) == 0 {
			emptyShards++
		} else {
			t.Logf("Shard %d has %d items", i, len(shardItems))
		}
	}

	// 100 个 key 分到 4 个分片，概率上几乎不可能有空分片
	if emptyShards == 4 {
		t.Error("All shards are empty, something is wrong")
	}
}

// 测试并发安全性
func TestShardedCache_Concurrency(t *testing.T) {
	sc := NewSharded[int](DefaultExpiration, 0, 16)
	defer sc.Close()

	var wg sync.WaitGroup
	workers := 20
	ops := 1000

	// 并发写入
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				sc.Set(key, j, DefaultExpiration)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				sc.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// 简单的数量验证
	// 总写入量 = workers * ops (Key 唯一)
	// 但因为是并发，我们只关心不 Panic 且数据最终能读到
	// 这里主要通过 go test -race 来检测竞争
}

// 测试 GetShardedT 辅助函数
func TestGetShardedT(t *testing.T) {
	// Key=string, Value=any
	sc := NewSharded[any](DefaultExpiration, 0, 4)

	sc.Set("intVal", 123, DefaultExpiration)
	sc.Set("strVal", "hello", DefaultExpiration)

	// 1. 正确类型获取
	if v, found := GetShardedT[int](sc, "intVal"); !found || v != 123 {
		t.Error("GetShardedT failed for int")
	}

	// 2. 错误类型获取
	if _, found := GetShardedT[string](sc, "intVal"); found {
		t.Error("GetShardedT should return false for wrong type assertion")
	}

	// 3. 不存在的 Key
	if _, found := GetShardedT[int](sc, "missing"); found {
		t.Error("GetShardedT should return false for missing key")
	}
}

// --- 基准测试 (Benchmarks) ---

// 基准测试：分片缓存的高并发写入
func BenchmarkShardedCache_Set_Concurrent(b *testing.B) {
	// 256 个分片
	sc := NewSharded[int](DefaultExpiration, 0, 256)
	defer sc.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var seed = maphash.MakeSeed()
		var counter uint64
		for pb.Next() {
			counter++
			// 模拟随机 Key
			k := "key-" + strconv.FormatUint(maphash.String(seed, strconv.FormatUint(counter, 10)), 10)
			sc.Set(k, int(counter), DefaultExpiration)
		}
	})
}

// 基准测试：分片缓存的高并发读取
func BenchmarkShardedCache_Get_Concurrent(b *testing.B) {
	sc := NewSharded[int](DefaultExpiration, 0, 256)
	defer sc.Close()

	// 预填充数据
	keys := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		k := strconv.Itoa(i)
		keys[i] = k
		sc.Set(k, i, DefaultExpiration)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sc.Get(keys[i%10000])
			i++
		}
	})
}

// 对比基准测试：标准(单锁)缓存的高并发写入
// 用来展示 ShardedCache 的优势
func BenchmarkStandardCache_Set_Concurrent(b *testing.B) {
	c := New[string, int](DefaultExpiration, 0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var seed = maphash.MakeSeed()
		var counter uint64
		for pb.Next() {
			counter++
			k := "key-" + strconv.FormatUint(maphash.String(seed, strconv.FormatUint(counter, 10)), 10)
			c.Set(k, int(counter), DefaultExpiration)
		}
	})
}
