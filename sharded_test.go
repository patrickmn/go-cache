package cache

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestDjb33(t *testing.T) {
	// Verify that keys of length 1 differing in that byte yield different hash values
	h1 := djb33(0, "a")
	h2 := djb33(0, "b")
	if h1 == h2 {
		t.Errorf("djb33 returned identical hash for keys of length 1: %v == %v", h1, h2)
	}

	// Verify that keys of lengths 4*n + 1 differing in the final byte yield different hash values
	prefixes := []string{
		"",                 // n = 0, len = 1
		"abcd",             // n = 1, len = 5
		"abcdefgh",         // n = 2, len = 9
		"abcdefghijkl",     // n = 3, len = 13
		"abcdefghijklmnop", // n = 4, len = 17
	}
	for _, p := range prefixes {
		k1 := p + "x"
		k2 := p + "y"
		hash1 := djb33(0, k1)
		hash2 := djb33(0, k2)
		if hash1 == hash2 {
			t.Errorf("djb33 returned identical hash for keys differing in final byte (len %d): %q and %q both got %v", len(k1), k1, k2, hash1)
		}
	}

	// Verify djb33 distribution across shards
	shards := uint32(10)
	buckets := make(map[uint32]int)
	for i := 0; i < 100; i++ {
		k := "key-" + strconv.Itoa(i)
		b := djb33(12345, k) % shards
		buckets[b]++
	}
	if len(buckets) < 5 {
		t.Errorf("djb33 distribution too concentrated: only %d buckets used out of %d", len(buckets), shards)
	}
}

var shardedKeys = []string{
	"f",
	"fo",
	"foo",
	"barf",
	"barfo",
	"foobar",
	"bazbarf",
	"bazbarfo",
	"bazbarfoo",
	"foobarbazq",
	"foobarbazqu",
	"foobarbazquu",
	"foobarbazquux",
}

func TestShardedCache(t *testing.T) {
	tc := unexportedNewSharded(DefaultExpiration, 0, 13)
	for _, v := range shardedKeys {
		tc.Set(v, "value", DefaultExpiration)
	}
}

func BenchmarkShardedCacheGetExpiring(b *testing.B) {
	benchmarkShardedCacheGet(b, 5*time.Minute)
}

func BenchmarkShardedCacheGetNotExpiring(b *testing.B) {
	benchmarkShardedCacheGet(b, NoExpiration)
}

func benchmarkShardedCacheGet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	tc := unexportedNewSharded(exp, 0, 10)
	tc.Set("foobarba", "zquux", DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Get("foobarba")
	}
}

func BenchmarkShardedCacheGetManyConcurrentExpiring(b *testing.B) {
	benchmarkShardedCacheGetManyConcurrent(b, 5*time.Minute)
}

func BenchmarkShardedCacheGetManyConcurrentNotExpiring(b *testing.B) {
	benchmarkShardedCacheGetManyConcurrent(b, NoExpiration)
}

func benchmarkShardedCacheGetManyConcurrent(b *testing.B, exp time.Duration) {
	b.StopTimer()
	n := 10000
	tsc := unexportedNewSharded(exp, 0, 20)
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "foo" + strconv.Itoa(i)
		keys[i] = k
		tsc.Set(k, "bar", DefaultExpiration)
	}
	each := b.N / n
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for _, v := range keys {
		go func(k string) {
			for j := 0; j < each; j++ {
				tsc.Get(k)
			}
			wg.Done()
		}(v)
	}
	b.StartTimer()
	wg.Wait()
}
