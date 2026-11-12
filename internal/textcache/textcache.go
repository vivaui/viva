// Package textcache holds what a backend's text engine has shaped for a line, so a
// width, a caret, a click and a draw of the same line shape it once.
//
// A working set within capacity stays warm without allocations. A Window TinyLFU
// admission policy keeps oversized scans from flushing every reused line: recent
// lines enter an LRU window and compete by frequency for the main LRU cache.
// Admission tracking starts only when the resident budget is exceeded. Frequency
// history is bounded and ages with requests, without background work.
// Not safe for concurrent use; the owning engine serializes access.
package textcache

import (
	"hash/maphash"
	"math"
	"strings"
)

// Key names one shaped line.
type Key struct {
	Value        string
	Size, Weight float32
}

type cacheKey struct {
	value        string
	size, weight uint32
}

func (key Key) identity() cacheKey {
	size, weight := key.Size, key.Weight
	if size == 0 {
		size = 0
	}
	if weight == 0 {
		weight = 0
	}
	return cacheKey{value: key.Value, size: math.Float32bits(size), weight: math.Float32bits(weight)}
}

// Cache maps keys to a platform's shaped lines.
type Cache[V any] struct {
	index     map[cacheKey]int
	entries   []entry[V]
	window    queue
	main      queue
	free      int
	limit     int
	release   func(V)
	seed      maphash.Seed
	frequency frequencySketch
	admission bool
}

type entry[V any] struct {
	key        cacheKey
	value      V
	hash       uint64
	prev, next int
	window     bool
	epoch      uint32
}

type queue struct {
	head, tail int
	length     int
}

// New returns a cache that keeps up to 2*limit entries. release, when not nil, is
// called once for each value the cache drops, so a native handle is freed with it.
func New[V any](limit int, release func(V)) *Cache[V] {
	limit = max(limit, 1)
	cache := &Cache[V]{
		index: make(map[cacheKey]int, 2*limit), entries: make([]entry[V], 2*limit+1),
		limit: limit, release: release, seed: maphash.MakeSeed(),
	}
	cache.resetFree()
	return cache
}

// Get is the value stored for key.
func (cache *Cache[V]) Get(key Key) (V, bool) {
	identity := key.identity()
	if index, ok := cache.index[identity]; ok {
		stored := &cache.entries[index]
		if cache.admission {
			cache.frequency.record(stored.hash)
			stored.epoch = cache.frequency.epoch
			cache.touch(index)
		}
		return stored.value, true
	}
	if cache.admission {
		cache.frequency.record(maphash.Comparable(cache.seed, identity))
	}
	var zero V
	return zero, false
}

// Put stores value for key. The key's string is copied, so the caller may pass text
// that aliases a buffer it reuses. A value already stored for key is released.
// The new value stays resident until another Put or Clear, even when admission
// rejects an older line; a backend can use the newly shaped line immediately.
func (cache *Cache[V]) Put(key Key, value V) {
	identity := key.identity()
	if index, ok := cache.index[identity]; ok {
		stored := &cache.entries[index]
		cache.drop(stored.value)
		stored.value = value
		stored.epoch = cache.frequency.epoch
		cache.touch(index)
		return
	}
	if cache.window.length == cache.limit {
		candidate := cache.window.tail
		if cache.main.length < cache.limit {
			cache.admit(candidate)
		} else {
			if !cache.admission {
				if cache.frequency.counters == nil {
					cache.frequency = newFrequencySketch(2 * cache.limit)
				}
				cache.admission = true
				cache.frequency.record(maphash.Comparable(cache.seed, identity))
			}
			victim := cache.main.tail
			if cache.score(candidate) > cache.score(victim) {
				cache.evict(victim)
				cache.admit(candidate)
			} else {
				cache.evict(candidate)
			}
		}
	}
	identity.value = strings.Clone(identity.value)
	index := cache.free
	cache.free = cache.entries[index].next
	hash := maphash.Comparable(cache.seed, identity)
	cache.entries[index] = entry[V]{
		key: identity, value: value, hash: hash, window: true,
		epoch: cache.frequency.epoch,
	}
	cache.index[identity] = index
	cache.pushFront(&cache.window, index)
}

func (cache *Cache[V]) score(index int) uint8 {
	stored := &cache.entries[index]
	if cache.frequency.epoch-stored.epoch >= frequencyBits {
		return 0
	}
	return cache.frequency.estimate(stored.hash)
}

func (cache *Cache[V]) touch(index int) {
	queue := &cache.main
	if cache.entries[index].window {
		queue = &cache.window
	}
	if queue.head != index {
		cache.remove(queue, index)
		cache.pushFront(queue, index)
	}
}

func (cache *Cache[V]) admit(index int) {
	cache.remove(&cache.window, index)
	cache.entries[index].window = false
	cache.pushFront(&cache.main, index)
}

func (cache *Cache[V]) evict(index int) {
	stored := &cache.entries[index]
	queue := &cache.main
	if stored.window {
		queue = &cache.window
	}
	cache.remove(queue, index)
	delete(cache.index, stored.key)
	cache.drop(stored.value)
	*stored = entry[V]{next: cache.free}
	cache.free = index
}

func (cache *Cache[V]) pushFront(queue *queue, index int) {
	stored := &cache.entries[index]
	stored.prev = 0
	stored.next = queue.head
	if queue.head != 0 {
		cache.entries[queue.head].prev = index
	} else {
		queue.tail = index
	}
	queue.head = index
	queue.length++
}

func (cache *Cache[V]) remove(queue *queue, index int) {
	stored := &cache.entries[index]
	if stored.prev != 0 {
		cache.entries[stored.prev].next = stored.next
	} else {
		queue.head = stored.next
	}
	if stored.next != 0 {
		cache.entries[stored.next].prev = stored.prev
	} else {
		queue.tail = stored.prev
	}
	queue.length--
}

func (cache *Cache[V]) drop(value V) {
	if cache.release != nil {
		cache.release(value)
	}
}

// Clear releases every entry: the font set or the display scale changed and no
// shaped line is valid.
func (cache *Cache[V]) Clear() {
	for _, index := range cache.index {
		cache.drop(cache.entries[index].value)
	}
	clear(cache.index)
	clear(cache.entries)
	cache.window = queue{}
	cache.main = queue{}
	cache.resetFree()
	cache.frequency.clear()
	cache.admission = false
}

func (cache *Cache[V]) resetFree() {
	cache.free = 1
	for index := 1; index < len(cache.entries)-1; index++ {
		cache.entries[index].next = index + 1
	}
}

// Len is the number of entries held.
func (cache *Cache[V]) Len() int { return len(cache.index) }
