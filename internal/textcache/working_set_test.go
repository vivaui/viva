package textcache

import (
	"fmt"
	"math"
	"testing"
)

func workingSet(distinct int) []Key {
	keys := make([]Key, distinct)
	for index := range keys {
		keys[index] = Key{Value: fmt.Sprintf("row %04d 主机", index), Size: 14}
	}
	return keys
}

func queryLines(cache *Cache[int], keys []Key) int {
	misses := 0
	for index, key := range keys {
		if _, ok := cache.Get(key); !ok {
			cache.Put(key, index)
			misses++
		}
	}
	return misses
}

func TestWorkingSetWithinCapacityStaysWarm(t *testing.T) {
	const limit = 32
	for _, distinct := range []int{limit, limit + 1, 2*limit - 1, 2 * limit} {
		t.Run(fmt.Sprint(distinct), func(t *testing.T) {
			cache := New[int](limit, nil)
			keys := workingSet(distinct)
			queryLines(cache, keys)
			for frame := 0; frame < 20; frame++ {
				if misses := queryLines(cache, keys); misses != 0 {
					t.Fatalf("warm frame %d reshaped %d of %d lines", frame, misses, distinct)
				}
			}
			if allocs := testing.AllocsPerRun(100, func() { queryLines(cache, keys) }); allocs != 0 {
				t.Fatalf("warm frame allocated %v times", allocs)
			}
		})
	}
}

func TestOversizedScanKeepsReusedLines(t *testing.T) {
	const limit = 32
	for _, distinct := range []int{2*limit + 1, 3 * limit, 8 * limit} {
		t.Run(fmt.Sprint(distinct), func(t *testing.T) {
			cache := New[int](limit, nil)
			keys := workingSet(distinct)
			queryLines(cache, keys)
			hits := 0
			for frame := 0; frame < 40; frame++ {
				hits += distinct - queryLines(cache, keys)
				if cache.Len() > 2*limit {
					t.Fatalf("cache exceeded its capacity: %d", cache.Len())
				}
			}
			if hits < 40*limit/2 {
				t.Fatalf("an oversized scan failed to retain a useful reused subset: %d hits", hits)
			}
		})
	}
}

func TestCacheAdaptsToAReplacementWorkingSet(t *testing.T) {
	const limit = 16
	cache := New[int](limit, nil)
	old := workingSet(2 * limit)
	for frame := 0; frame < 20; frame++ {
		queryLines(cache, old)
	}
	current := workingSet(2 * limit)
	for index := range current {
		current[index].Value += " new"
	}
	for frame := 0; frame < 100; frame++ {
		if queryLines(cache, current) == 0 {
			return
		}
	}
	t.Fatal("an obsolete working set prevented the replacement from warming")
}

func TestEvictionAndReplacementReleaseEachValueOnce(t *testing.T) {
	released := make(map[int]int)
	cache := New(8, func(value int) { released[value]++ })
	keys := workingSet(40)
	created := 0
	for frame := 0; frame < 40; frame++ {
		for _, key := range keys {
			cache.Get(key)
			created++
			cache.Put(key, created)
			if value, ok := cache.Get(key); !ok || value != created || released[value] != 0 {
				t.Fatalf("Put did not retain its new value: value=%d present=%v", value, ok)
			}
		}
	}
	cache.Clear()
	cache.Clear()
	for value := 1; value <= created; value++ {
		if released[value] != 1 {
			t.Fatalf("value %d released %d times", value, released[value])
		}
	}
	queryLines(cache, keys[:8])
	if misses := queryLines(cache, keys[:8]); misses != 0 {
		t.Fatalf("cleared cache could not warm again: %d misses", misses)
	}
}

func TestNonFiniteKeysDoNotEscapeTheResourceBudget(t *testing.T) {
	released := make(map[int]int)
	cache := New(2, func(value int) { released[value]++ })
	for value := 0; value < 40; value++ {
		key := Key{Value: fmt.Sprint(value), Size: float32(math.NaN()), Weight: float32(math.NaN())}
		cache.Get(key)
		cache.Put(key, value)
		if cache.Len() > 4 {
			t.Fatalf("non-finite keys exceeded the resource budget: %d", cache.Len())
		}
	}
	cache.Clear()
	for value := 0; value < 40; value++ {
		if released[value] != 1 {
			t.Fatalf("value %d released %d times", value, released[value])
		}
	}
}

func TestSignedZeroWeightsShareTheSameLine(t *testing.T) {
	cache := New[int](2, nil)
	key := Key{Value: "regular", Size: 14}
	cache.Put(key, 1)
	key.Weight = float32(math.Copysign(0, -1))
	if value, ok := cache.Get(key); !ok || value != 1 {
		t.Fatal("negative zero weight did not reuse the regular line")
	}
}

func BenchmarkWorkingSet(b *testing.B) {
	for _, distinct := range []int{512, 513, 600, 1000, 1024, 1025, 1500, 5000} {
		b.Run(fmt.Sprint(distinct), func(b *testing.B) {
			cache := New[int](512, nil)
			keys := workingSet(distinct)
			for frame := 0; frame < 20; frame++ {
				queryLines(cache, keys)
			}
			b.ReportAllocs()
			b.ResetTimer()
			misses := 0
			for b.Loop() {
				misses += queryLines(cache, keys)
			}
			b.ReportMetric(float64(misses)/float64(b.N), "misses/frame")
		})
	}
}

func BenchmarkCacheCreation(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		cache := New[int](512, nil)
		cache.Put(Key{Value: "first line"}, 1)
		cache.Clear()
	}
}
