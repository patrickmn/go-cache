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
	tc := New(DefaultExpiration, 0)

	a := tc.Get("a")
	if a != nil {
		t.Error("Getting A found value that shouldn't exist:", a)
	}

	b := tc.Get("b")
	if b != nil {
		t.Error("Getting B found value that shouldn't exist:", b)
	}

	c := tc.Get("c")
	if c != nil {
		t.Error("Getting C found value that shouldn't exist:", c)
	}

	tc.Set("a", ValueType(1), DefaultExpiration)

	x := tc.Get("a")
	if x == nil {
		t.Error("x for a is nil")
	} else if a2 := int(*x); a2+2 != 3 {
		t.Error("a2 (which should be 1) plus 2 does not equal 3; value:", a2)
	}
}

func TestCacheTimes(t *testing.T) {
	var x *ValueType

	tc := New(50*time.Millisecond, 1*time.Millisecond)
	tc.Set("a", 1, DefaultExpiration)
	tc.Set("b", 2, NoExpiration)
	tc.Set("c", 3, 20*time.Millisecond)
	tc.Set("d", 4, 70*time.Millisecond)

	<-time.After(25 * time.Millisecond)
	x = tc.Get("c")
	if x != nil {
		t.Error("Found c when it should have been automatically deleted", *x)
	}

	<-time.After(30 * time.Millisecond)
	x = tc.Get("a")
	if x != nil {
		t.Error("Found a when it should have been automatically deleted", *x)
	}

	x = tc.Get("b")
	if x == nil {
		t.Error("Did not find b even though it was set to never expire")
	}

	x = tc.Get("d")
	if x == nil {
		t.Error("Did not find d even though it was set to expire later than the default")
	}

	<-time.After(20 * time.Millisecond)
	x = tc.Get("d")
	if x != nil {
		t.Error("Found d when it should have been automatically deleted (later than the default)")
	}
}

// TODO: test increment.
func TestIncrementWithInt(t *testing.T) {
}

func TestAdd(t *testing.T) {
	tc := New(DefaultExpiration, 0)
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
	tc := New(DefaultExpiration, 0)
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
	tc := New(DefaultExpiration, 0)
	tc.Set("foo", 1, DefaultExpiration)
	tc.Delete("foo")
	x := tc.Get("foo")
	if x != nil {
		t.Error("x is not nil:", x)
	}
}

func TestItemCount(t *testing.T) {
	tc := New(DefaultExpiration, 0)
	tc.Set("foo", 1, DefaultExpiration)
	tc.Set("bar", 2, DefaultExpiration)
	tc.Set("baz", 3, DefaultExpiration)
	if n := tc.ItemCount(); n != 3 {
		t.Errorf("Item count is not 3: %d", n)
	}
}

func TestFlush(t *testing.T) {
	tc := New(DefaultExpiration, 0)
	tc.Set("foo", 1, DefaultExpiration)
	tc.Set("baz", 2, DefaultExpiration)
	tc.Flush()
	x := tc.Get("foo")
	if x != nil {
		t.Error("x is not nil:", x)
	}
	x = tc.Get("baz")
	if x != nil {
		t.Error("x is not nil:", x)
	}
}

func TestOnEvicted(t *testing.T) {
	tc := New(DefaultExpiration, 0)
	tc.Set("foo", ValueType(3), DefaultExpiration)
	if tc.onEvicted != nil {
		t.Fatal("tc.onEvicted is not nil")
	}
	works := false
	tc.OnEvicted(func(k string, v *ValueType) {
		if k == "foo" && int(*v) == 3 {
			works = true
		}
		tc.Set("bar", ValueType(4), DefaultExpiration)
	})
	tc.Delete("foo")
	x := tc.Get("bar")
	if !works {
		t.Error("works bool not true")
	}
	if int(*x) != 4 {
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
	tc := New(exp, 0)
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
	tc := New(exp, 0)
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
	tc := New(exp, 0)
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "foo" + strconv.Itoa(i)
		keys[i] = k
		tc.Set(k, ValueType(1), DefaultExpiration)
	}
	each := b.N / n
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for _, v := range keys {
		go func() {
			for j := 0; j < each; j++ {
				tc.Get(v)
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
	tc := New(exp, 0)
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
	tc := New(DefaultExpiration, 0)
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
	tc := New(DefaultExpiration, 0)
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
	b.Skip()
	b.StopTimer()
	tc := New(DefaultExpiration, 0)
	tc.Set("foo", 0, DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		// tc.IncrementInt("foo", 1)
	}
}

func BenchmarkDeleteExpiredLoop(b *testing.B) {
	b.StopTimer()
	tc := New(5*time.Minute, 0)
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
