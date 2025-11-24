package cache

import (
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

// --- Unit Tests ---

func TestCache_AnyType_WithHelper(t *testing.T) {
	// 1. Initialize as any type
	c := New[string, any](DefaultExpiration, 0)

	// 2. Store different types
	c.Set("myInt", 100, DefaultExpiration)
	c.Set("myString", "Hello", DefaultExpiration)
	c.Set("myStruct", struct{ ID int }{ID: 1}, DefaultExpiration)

	// 3. Use GetT helper
	if v, found := GetT[int](c, "myInt"); !found || v != 100 {
		t.Errorf("GetT failed for int. Found: %v, Val: %v", found, v)
	}

	if v, found := GetT[string](c, "myString"); !found || v != "Hello" {
		t.Errorf("GetT failed for string. Found: %v, Val: %v", found, v)
	}

	if _, found := GetT[int](c, "myString"); found {
		t.Error("GetT should return false when type assertion fails")
	}

	if _, found := GetT[int](c, "notExist"); found {
		t.Error("GetT should return false for non-existent key")
	}
}

func TestCache_BasicSetGet(t *testing.T) {
	tc := New[string, string](DefaultExpiration, 0)

	tc.Set("foo", "bar", DefaultExpiration)
	if v, found := tc.Get("foo"); !found || v != "bar" {
		t.Errorf("Get found: %v, value: %v, expected: bar", found, v)
	}

	tc.Set("int", "1", DefaultExpiration)
	if v, found := tc.Get("int"); !found || v != "1" {
		t.Errorf("Get found: %v, value: %v, expected: 1", found, v)
	}
}

func TestCache_Expiration(t *testing.T) {
	tc := New[string, int](50*time.Millisecond, 0)

	tc.Set("x", 1, DefaultExpiration)
	tc.Set("y", 2, NoExpiration)
	tc.Set("z", 3, 20*time.Millisecond)

	if _, found := tc.Get("x"); !found {
		t.Error("x should be found")
	}
	if _, found := tc.Get("y"); !found {
		t.Error("y should be found")
	}
	if _, found := tc.Get("z"); !found {
		t.Error("z should be found")
	}

	time.Sleep(30 * time.Millisecond)
	if _, found := tc.Get("z"); found {
		t.Error("z should have expired")
	}
	if _, found := tc.Get("x"); !found {
		t.Error("x should not have expired yet")
	}

	time.Sleep(30 * time.Millisecond)
	if _, found := tc.Get("x"); found {
		t.Error("x should have expired")
	}
	if _, found := tc.Get("y"); !found {
		t.Error("y should always be found (NoExpiration)")
	}
}

func TestCache_Add(t *testing.T) {
	tc := New[string, string](DefaultExpiration, 0)

	if ok := tc.Add("foo", "bar", DefaultExpiration); !ok {
		t.Error("Add should return true for new key")
	}

	if ok := tc.Add("foo", "baz", DefaultExpiration); ok {
		t.Error("Add should return false for existing key")
	}
	if v, _ := tc.Get("foo"); v != "bar" {
		t.Error("Value should remain 'bar'")
	}

	tc.Set("expired", "old", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if ok := tc.Add("expired", "new", DefaultExpiration); !ok {
		t.Error("Add should return true if previous key is expired")
	}
	if v, _ := tc.Get("expired"); v != "new" {
		t.Error("Value should be updated to 'new'")
	}
}

func TestCache_Replace(t *testing.T) {
	tc := New[string, int](DefaultExpiration, 0)

	if ok := tc.Replace("foo", 1, DefaultExpiration); ok {
		t.Error("Replace should return false for non-existent key")
	}

	tc.Set("foo", 1, DefaultExpiration)
	if ok := tc.Replace("foo", 2, DefaultExpiration); !ok {
		t.Error("Replace should return true for existing key")
	}
	if v, _ := tc.Get("foo"); v != 2 {
		t.Error("Value should be 2")
	}

	tc.Set("expired", 1, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if ok := tc.Replace("expired", 2, DefaultExpiration); ok {
		t.Error("Replace should return false for expired key")
	}
}

func TestCache_Delete(t *testing.T) {
	tc := New[string, int](DefaultExpiration, 0)
	tc.Set("foo", 1, DefaultExpiration)

	tc.Delete("foo")
	if _, found := tc.Get("foo"); found {
		t.Error("foo should be deleted")
	}

	tc.Delete("bar")
}

func TestCache_Flush(t *testing.T) {
	tc := New[string, int](DefaultExpiration, 0)
	tc.Set("foo", 1, DefaultExpiration)
	tc.Set("bar", 2, DefaultExpiration)

	tc.Flush()

	if tc.Count() != 0 {
		t.Errorf("Count should be 0 after Flush, got %d", tc.Count())
	}
}

func TestCache_OnEvicted(t *testing.T) {
	tc := New[int, int](DefaultExpiration, 0)

	var evictedKey, evictedVal int
	var executed bool

	tc.OnEvicted(func(k, v int) {
		evictedKey = k
		evictedVal = v
		executed = true
	})

	tc.Set(1, 100, DefaultExpiration)
	tc.Delete(1)

	if !executed || evictedKey != 1 || evictedVal != 100 {
		t.Error("OnEvicted callback not executed correctly")
	}
}

func TestCache_Janitor(t *testing.T) {
	tc := New[string, int](50*time.Millisecond, 20*time.Millisecond)
	defer tc.Close()

	tc.Set("foo", 1, 10*time.Millisecond)

	if _, found := tc.Get("foo"); !found {
		t.Error("Item should be in cache")
	}

	time.Sleep(100 * time.Millisecond)

	if tc.Count() != 0 {
		t.Errorf("Cache should be empty after janitor run. Count: %d", tc.Count())
	}
}

func TestCache_Generics(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}
	tc := New[int, *User](DefaultExpiration, 0)

	u := &User{Name: "Alice", Age: 30}
	tc.Set(1, u, DefaultExpiration)

	got, found := tc.Get(1)
	if !found {
		t.Fatal("User not found")
	}
	if got.Name != "Alice" {
		t.Errorf("Name mismatch")
	}
}

func TestCache_Concurrency(t *testing.T) {
	tc := New[int, int](DefaultExpiration, 0)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				tc.Set(base+j, j, DefaultExpiration)
			}
		}(i * 1000)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				tc.Get(rand.Intn(10000))
			}
		}()
	}

	wg.Wait()
}

func TestCache_Close_DoubleCalls(t *testing.T) {
	tc := New[string, int](DefaultExpiration, 1*time.Minute)
	tc.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close panicked: %v", r)
		}
	}()
	tc.Close()
}

// --- Benchmarks ---

func BenchmarkCache_Set(b *testing.B) {
	tc := New[string, int](DefaultExpiration, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc.Set("foo", i, DefaultExpiration)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	tc := New[string, int](DefaultExpiration, 0)
	tc.Set("foo", 1, DefaultExpiration)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc.Get("foo")
	}
}

func BenchmarkCache_SetGet_Concurrent(b *testing.B) {
	tc := New[string, int](DefaultExpiration, 0)
	for i := 0; i < 1000; i++ {
		tc.Set(strconv.Itoa(i), i, DefaultExpiration)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			counter++
			key := strconv.Itoa(counter % 1000)
			if counter%10 == 0 {
				tc.Set(key, counter, DefaultExpiration)
			} else {
				tc.Get(key)
			}
		}
	})
}

func BenchmarkCache_DeleteExpired_LargeCache(b *testing.B) {
	count := 100000
	tc := New[int, int](DefaultExpiration, 0)

	for i := 0; i < count; i++ {
		if i%2 == 0 {
			tc.Set(i, i, 1*time.Nanosecond)
		} else {
			tc.Set(i, i, 1*time.Hour)
		}
	}
	time.Sleep(1 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for k := 0; k < 1000; k++ {
			tc.Set(k, k, 1*time.Nanosecond)
		}
		time.Sleep(time.Nanosecond)
		b.StartTimer()

		tc.DeleteExpired()
	}
}

func BenchmarkMap_Get(b *testing.B) {
	m := make(map[string]int)
	m["foo"] = 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m["foo"]
	}
}

func BenchmarkMutexMap_Get_Concurrent(b *testing.B) {
	m := make(map[string]int)
	var mu sync.RWMutex
	for i := 0; i < 1000; i++ {
		m[strconv.Itoa(i)] = i
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			counter++
			key := strconv.Itoa(counter % 1000)
			mu.RLock()
			_ = m[key]
			mu.RUnlock()
		}
	})
}

func TestMemoryUsage(t *testing.T) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	startAlloc := m.Alloc

	tc := New[int, int](DefaultExpiration, 0)
	for i := 0; i < 100000; i++ {
		tc.Set(i, i, DefaultExpiration)
	}

	runtime.ReadMemStats(&m)
	fmt.Printf("Memory Alloc: %v MB\n", (m.Alloc-startAlloc)/1024/1024)

	tc.Flush()
	runtime.GC()
	runtime.ReadMemStats(&m)

	if tc.Count() != 0 {
		t.Error("Cache should be empty")
	}
}
