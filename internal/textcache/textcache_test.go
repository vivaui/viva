package textcache

import (
	"strings"
	"testing"
)

func TestAHitAllocatesNothing(t *testing.T) {
	cache := New[int](8, nil)
	key := Key{Value: "hello", Size: 14}
	cache.Put(key, 1)
	if allocs := testing.AllocsPerRun(100, func() {
		if _, ok := cache.Get(key); !ok {
			t.Fatal("missing")
		}
	}); allocs != 0 {
		t.Fatalf("a hit allocated %v times", allocs)
	}
}

func TestFrequentlyUsedLineSurvivesAColdScan(t *testing.T) {
	var released []int
	cache := New(2, func(value int) { released = append(released, value) })
	hot, idle := Key{Value: "hot"}, Key{Value: "idle"}
	cache.Put(hot, 1)
	cache.Put(idle, 2)
	for generation := 0; generation < 60; generation++ {
		if _, ok := cache.Get(hot); !ok {
			t.Fatalf("generation %d: the line in use was dropped", generation)
		}
		cache.Put(Key{Value: strings.Repeat("x", generation+1)}, 100+generation)
	}
	for _, value := range released {
		if value == 1 {
			t.Fatal("the line in use was released")
		}
	}
}

func TestClearReleasesEverythingOnce(t *testing.T) {
	count := map[int]int{}
	cache := New(2, func(value int) { count[value]++ })
	for value := 1; value <= 3; value++ {
		cache.Put(Key{Value: strings.Repeat("a", value)}, value)
	}
	cache.Clear()
	for value := 1; value <= 3; value++ {
		if count[value] != 1 {
			t.Fatalf("value %d released %d times", value, count[value])
		}
	}
	if cache.Len() != 0 {
		t.Fatalf("%d entries left", cache.Len())
	}
}

func TestPutCopiesTheKeyText(t *testing.T) {
	cache := New[int](4, nil)
	buffer := []byte("abc")
	cache.Put(Key{Value: string(buffer)}, 1)
	// A string built over a reused buffer must not change under the cache.
	buffer[0] = 'z'
	if _, ok := cache.Get(Key{Value: "abc"}); !ok {
		t.Fatal("the key changed with the caller's buffer")
	}
}

func TestReplacingAValueReleasesTheOldOne(t *testing.T) {
	var released []int
	cache := New(4, func(value int) { released = append(released, value) })
	key := Key{Value: "a"}
	cache.Put(key, 1)
	cache.Put(key, 2)
	if len(released) != 1 || released[0] != 1 {
		t.Fatalf("released %v", released)
	}
	if value, _ := cache.Get(key); value != 2 {
		t.Fatalf("got %d", value)
	}
}

func BenchmarkHit(b *testing.B) {
	cache := New[int](256, nil)
	key := Key{Value: "192.168.1.1 主机", Size: 14, Weight: 0}
	cache.Put(key, 1)
	b.ReportAllocs()
	for b.Loop() {
		cache.Get(key)
	}
}
