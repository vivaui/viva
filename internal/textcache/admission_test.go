package textcache

import (
	"encoding/json"
	"fmt"
	"hash/maphash"
	"os"
	"strings"
	"testing"
)

func TestOversizedScanRetainsReusedLinesWithCapturedCollisions(t *testing.T) {
	data, err := os.ReadFile("testdata/adverse-scan-traces.json")
	if err != nil {
		t.Fatal(err)
	}
	var traces []struct {
		Hits   int      `json:"hits"`
		Hashes []uint64 `json:"hashes"`
	}
	if err := json.Unmarshal(data, &traces); err != nil {
		t.Fatal(err)
	}
	if len(traces) != 3 {
		t.Fatalf("got %d captured traces, want 3", len(traces))
	}
	const limit = 32
	keys := workingSet(8 * limit)
	for index, trace := range traces {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			if len(trace.Hashes) != len(keys) {
				t.Fatalf("trace has %d hashes, want %d", len(trace.Hashes), len(keys))
			}
			cache := New[int](limit, nil)
			transformed := make(map[uint64]uint64, len(keys))
			for index, key := range keys {
				transformed[maphash.Comparable(cache.seed, key.identity())] = trace.Hashes[index]
			}
			cache.hashTransform = func(hash uint64) uint64 {
				captured, ok := transformed[hash]
				if !ok {
					t.Fatalf("hash %x is absent from the captured working set", hash)
				}
				return captured
			}
			queryLines(cache, keys)
			hits := 0
			for frame := 0; frame < 40; frame++ {
				hits += len(keys) - queryLines(cache, keys)
				if cache.Len() > 2*limit {
					t.Fatalf("cache exceeded its capacity: %d", cache.Len())
				}
			}
			if hits < 40*limit/2 {
				t.Fatalf("captured collisions retained %d hits, want at least %d (baseline %d)", hits, 40*limit/2, trace.Hits)
			}
		})
	}
}

func TestInitialWindowHitSurvivesCollidingColdAdmissions(t *testing.T) {
	cache := New[int](2, nil)
	hot := Key{Value: "hot"}
	hotHash := maphash.Comparable(cache.seed, hot.identity())
	cache.hashTransform = func(hash uint64) uint64 {
		if hash == hotHash {
			return 0
		}
		return 0x0001000100010001
	}
	cache.Put(hot, 1)
	cache.Put(Key{Value: "idle"}, 2)
	for generation := 0; generation < 60; generation++ {
		if _, ok := cache.Get(hot); !ok {
			t.Fatalf("generation %d: the repeatedly accessed initial line was evicted", generation)
		}
		cache.Put(Key{Value: strings.Repeat("x", generation+1)}, 100+generation)
	}
}

func TestReplacementWorkingSetReclaimsProtectedCapacity(t *testing.T) {
	const limit = 16
	cache := New[int](limit, nil)
	old := workingSet(2*limit + 1)
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
	t.Fatal("protected obsolete lines prevented the full replacement working set from warming")
}
