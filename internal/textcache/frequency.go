package textcache

import "math/bits"

const frequencyBits = 4
const frequencyMax = 1<<frequencyBits - 1

type frequencySketch struct {
	counters []uint8
	width    int
	mask     uint64
	requests int
	period   int
	epoch    uint32
}

func newFrequencySketch(capacity int) frequencySketch {
	width := 1 << bits.Len(uint(capacity-1))
	return frequencySketch{counters: make([]uint8, 4*width), width: width, mask: uint64(width - 1), period: 10 * capacity}
}

func (sketch *frequencySketch) record(hash uint64) {
	for row := 0; row < 4; row++ {
		index := row*sketch.width + int((hash>>uint(16*row))&sketch.mask)
		if sketch.counters[index] < frequencyMax {
			sketch.counters[index]++
		}
	}
	sketch.requests++
	if sketch.requests == sketch.period {
		for index := range sketch.counters {
			sketch.counters[index] >>= 1
		}
		sketch.requests = 0
		sketch.epoch++
	}
}

func (sketch *frequencySketch) estimate(hash uint64) uint8 {
	frequency := uint8(frequencyMax)
	for row := 0; row < 4; row++ {
		index := row*sketch.width + int((hash>>uint(16*row))&sketch.mask)
		frequency = min(frequency, sketch.counters[index])
	}
	return frequency
}

func (sketch *frequencySketch) clear() {
	clear(sketch.counters)
	sketch.requests = 0
	sketch.epoch = 0
}
