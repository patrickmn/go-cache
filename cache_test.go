package cache

import (
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

type TestStruct struct {
	Num      int
	Children []*TestStruct
}

func TestCache(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})

	a, found := tc.Get("a")
	if found || a != 0 {
		t.Error("Getting A found value that shouldn't exist:", a)
	}

	b, found := tc.Get("b")
	if found || b != 0 {
		t.Error("Getting B found value that shouldn't exist:", b)
	}

	c, found := tc.Get("c")
	if found || c != 0 {
		t.Error("Getting C found value that shouldn't exist:", c)
	}

	tc.Set("a", 1, DefaultExpiration)
	tc.Set("b", 2, DefaultExpiration)
	tc.Set("c", 3, DefaultExpiration)

	x, found := tc.Get("a")
	if !found {
		t.Error("a was not found while getting a2")
	}
	if x == 0 {
		t.Error("x for a is nil")
	} else if a2 := x; a2+2 != 3 {
		t.Error("a2 (which should be 1) plus 2 does not equal 3; value:", a2)
	}
	x, found = tc.Get("b")
	if !found {
		t.Error("b was not found while getting b2")
	}
	if x == 0 {
		t.Error("x for b is nil")
	} else if b2 := x; b2+2 != 4 {
		t.Error("b2 (which should be 2) plus 2 does not equal 4; value:", b2)
	}

	x, found = tc.Get("c")
	if !found {
		t.Error("c was not found while getting c2")
	}
	if x == 0 {
		t.Error("x for c is nil")
	} else if c2 := x; c2+1 != 4 {
		t.Error("c2 (which should be 3) plus 1 does not equal 4; value:", c2)
	}

}

func TestCacheTimes(t *testing.T) {
	var found bool

	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      50 * time.Millisecond,
		DefaultCleanupInterval: 1 * time.Millisecond,
	})
	tc.Set("a", 1, DefaultExpiration)
	tc.Set("b", 2, NoExpiration)
	tc.Set("c", 3, 20*time.Millisecond)
	tc.Set("d", 4, 70*time.Millisecond)

	<-time.After(25 * time.Millisecond)
	_, found = tc.Get("c")
	if found {
		t.Error("Found c when it should have been automatically deleted")
	}

	<-time.After(30 * time.Millisecond)
	_, found = tc.Get("a")
	if found {
		t.Error("Found a when it should have been automatically deleted")
	}

	_, found = tc.Get("b")
	if !found {
		t.Error("Did not find b even though it was set to never expire")
	}

	_, found = tc.Get("d")
	if !found {
		t.Error("Did not find d even though it was set to expire later than the default")
	}

	<-time.After(20 * time.Millisecond)
	_, found = tc.Get("d")
	if found {
		t.Error("Found d when it should have been automatically deleted (later than the default)")
	}
}

func TestIncrement(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	tc.Set("tint", 1, DefaultExpiration)
	err := tc.Increment("tint", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tint")
	if !found {
		t.Error("tint was not found")
	}
	if x != 3 {
		t.Error("tint is not 3:", x)
	}
}

func TestDecrement(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	tc.Set("int", 5, DefaultExpiration)
	err := tc.Decrement("int", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("int")
	if !found {
		t.Error("int was not found")
	}
	if x != 3 {
		t.Error("int is not 3:", x)
	}
}

func TestAdd(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	err := tc.Add("foo", 1, DefaultExpiration)
	if err != nil {
		t.Error("Couldn't add foo even though it shouldn't exist")
	}
	err = tc.Add("foo", 2, DefaultExpiration)
	if err == nil {
		t.Error("Successfully added another foo when it should have returned an error")
	}
}

func TestReplace(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: -1,
	})
	err := tc.Replace("foo", 1, DefaultExpiration)
	if err == nil {
		t.Error("Replaced foo when it shouldn't exist")
	}
	tc.Set("foo", 1, DefaultExpiration)
	err = tc.Replace("foo", 2, DefaultExpiration)
	if err != nil {
		t.Error("Couldn't replace existing key foo")
	}
}
func TestDelete(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	tc.Set("foo", 1, DefaultExpiration)
	tc.Delete("foo")
	x, found := tc.Get("foo")
	if found {
		t.Error("foo was found, but it should have been deleted")
	}
	if x != 0 {
		t.Error("x is not nil:", x)
	}
}

func TestItemCount(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	tc.Set("foo", 1, DefaultExpiration)
	tc.Set("bar", 2, DefaultExpiration)
	tc.Set("baz", 3, DefaultExpiration)
	if n := tc.ItemCount(); n != 3 {
		t.Errorf("Item count is not 3: %d", n)
	}
}

func TestFlush(t *testing.T) {
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: -1,
	})
	tc.Set("foo", 1, DefaultExpiration)
	tc.Set("baz", 2, DefaultExpiration)
	tc.Flush()
	x, found := tc.Get("foo")
	if found {
		t.Error("foo was found, but it should have been deleted")
	}
	if x != 0 {
		t.Error("x is not nil:", x)
	}
	x, found = tc.Get("baz")
	if found {
		t.Error("baz was found, but it should have been deleted")
	}
	if x != 0 {
		t.Error("x is not nil:", x)
	}
}

func TestOnEvicted(t *testing.T) {
	works := false
	var tc *Cache_tpl
	tc = New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
		OnEvicted: func(k string, v ValueType_tpl) {
			if k == "foo" && v == 3 {
				works = true
			}
			tc.Set("bar", 4, DefaultExpiration)
		},
	})
	tc.Set("foo", 3, DefaultExpiration)
	if tc.onEvicted == nil {
		t.Fatal("tc.onEvicted is nil")
	}
	tc.Delete("foo")
	x, _ := tc.Get("bar")
	if !works {
		t.Error("works bool not true")
	}
	if x != 4 {
		t.Error("bar was not 4")
	}
}

func BenchmarkCacheGetExpiring(b *testing.B) {
	benchmarkCacheGet(b, 5*time.Minute)
}

func BenchmarkCacheGetNotExpiring(b *testing.B) {
	benchmarkCacheGet(b, NoExpiration)
}

func benchmarkCacheGet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      exp,
		DefaultCleanupInterval: 0,
	})
	tc.Set("foo", 1, DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Get("foo")
	}
}

func BenchmarkRWMutexMapGet(b *testing.B) {
	b.StopTimer()
	m := map[string]string{
		"foo": "bar",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m["foo"]
		mu.RUnlock()
	}
}

func BenchmarkRWMutexInterfaceMapGetStruct(b *testing.B) {
	b.StopTimer()
	s := struct{ name string }{name: "foo"}
	m := map[interface{}]string{
		s: "bar",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m[s]
		mu.RUnlock()
	}
}

func BenchmarkRWMutexInterfaceMapGetString(b *testing.B) {
	b.StopTimer()
	m := map[interface{}]string{
		"foo": "bar",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m["foo"]
		mu.RUnlock()
	}
}

func BenchmarkCacheGetConcurrentExpiring(b *testing.B) {
	benchmarkCacheGetConcurrent(b, 5*time.Minute)
}

func BenchmarkCacheGetConcurrentNotExpiring(b *testing.B) {
	benchmarkCacheGetConcurrent(b, NoExpiration)
}

func benchmarkCacheGetConcurrent(b *testing.B, exp time.Duration) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      exp,
		DefaultCleanupInterval: 0,
	})
	tc.Set("foo", 1, DefaultExpiration)
	wg := new(sync.WaitGroup)
	workers := runtime.NumCPU()
	each := b.N / workers
	wg.Add(workers)
	b.StartTimer()
	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < each; j++ {
				tc.Get("foo")
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkRWMutexMapGetConcurrent(b *testing.B) {
	b.StopTimer()
	m := map[string]string{
		"foo": "bar",
	}
	mu := sync.RWMutex{}
	wg := new(sync.WaitGroup)
	workers := runtime.NumCPU()
	each := b.N / workers
	wg.Add(workers)
	b.StartTimer()
	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < each; j++ {
				mu.RLock()
				_, _ = m["foo"]
				mu.RUnlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkCacheGetManyConcurrentExpiring(b *testing.B) {
	benchmarkCacheGetManyConcurrent(b, 5*time.Minute)
}

func BenchmarkCacheGetManyConcurrentNotExpiring(b *testing.B) {
	benchmarkCacheGetManyConcurrent(b, NoExpiration)
}

func benchmarkCacheGetManyConcurrent(b *testing.B, exp time.Duration) {
	// This is the same as BenchmarkCacheGetConcurrent, but its result
	// can be compared against BenchmarkShardedCacheGetManyConcurrent
	// in sharded_test.go.
	b.StopTimer()
	n := 10000
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      exp,
		DefaultCleanupInterval: 0,
	})
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "foo" + strconv.Itoa(i)
		keys[i] = k
		tc.Set(k, ValueType_tpl(1), DefaultExpiration)
	}
	each := b.N / n
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for _, v := range keys {
		x := v
		go func() {
			for j := 0; j < each; j++ {
				tc.Get(x)
			}
			wg.Done()
		}()
	}
	b.StartTimer()
	wg.Wait()
}

func BenchmarkCacheSetExpiring(b *testing.B) {
	benchmarkCacheSet(b, 5*time.Minute)
}

func BenchmarkCacheSetNotExpiring(b *testing.B) {
	benchmarkCacheSet(b, NoExpiration)
}

func benchmarkCacheSet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      exp,
		DefaultCleanupInterval: 0,
	})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Set("foo", 1, DefaultExpiration)
	}
}

func BenchmarkRWMutexMapSet(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["foo"] = "bar"
		mu.Unlock()
	}
}

func BenchmarkCacheSetDelete(b *testing.B) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Set("foo", 1, DefaultExpiration)
		tc.Delete("foo")
	}
}

func BenchmarkRWMutexMapSetDelete(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["foo"] = "bar"
		mu.Unlock()
		mu.Lock()
		delete(m, "foo")
		mu.Unlock()
	}
}

func BenchmarkCacheSetDeleteSingleLock(b *testing.B) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.mu.Lock()
		tc.set("foo", 1, DefaultExpiration)
		tc.delete("foo")
		tc.mu.Unlock()
	}
}

func BenchmarkRWMutexMapSetDeleteSingleLock(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["foo"] = "bar"
		delete(m, "foo")
		mu.Unlock()
	}
}

func BenchmarkIncrementInt(b *testing.B) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      DefaultExpiration,
		DefaultCleanupInterval: 0,
	})
	tc.Set("foo", 0, DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Increment("foo", 1)
	}
}

func BenchmarkDeleteExpiredLoop(b *testing.B) {
	b.StopTimer()
	tc := New_tpl(Attr_tpl{
		DefaultExpiration:      5 * time.Minute,
		DefaultCleanupInterval: 0,
	})
	tc.mu.Lock()
	for i := 0; i < 100000; i++ {
		tc.set(strconv.Itoa(i), 1, DefaultExpiration)
	}
	tc.mu.Unlock()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.DeleteExpired()
	}
}
