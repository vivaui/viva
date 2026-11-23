//go:build linux

package vulkan

import "slices"

// imagePool hands out device memory for label images from large blocks, so a
// page whose rows change does not make a driver allocation per row: drivers
// allow a few thousand allocations, and each is a call into the kernel. An image
// too large for a block gets memory of its own, as before.
type imagePool struct{ blocks []*imageBlock }

// imageBlock is one device allocation, carved into image memory.
type imageBlock struct {
	memory     uintptr
	memoryType uint32
	size       uint64
	free       []memorySpan // ordered by offset, never touching
}

type memorySpan struct{ offset, size uint64 }

// imageSlice is where a label's image memory is: a span of a block, or, with no
// block, memory of its own.
type imageSlice struct {
	block        *imageBlock
	offset, size uint64
}

// imageBlockSize is a block's size: a few hundred terminal rows at 2x.
const imageBlockSize = 16 << 20

// allocate finds memory of memoryType for an image of requirements, and returns
// the memory and offset to bind it at.
func (pool *imagePool) allocate(renderer *Renderer, memoryType uint32, requirements vkMemoryRequirements) (uintptr, imageSlice, error) {
	alignment := max(requirements.Alignment, 1)
	if requirements.Size > imageBlockSize/4 {
		memory, err := allocateDeviceMemory(renderer, memoryType, requirements.Size, "allocate text image memory")
		return memory, imageSlice{}, err
	}
	for _, block := range pool.blocks {
		if block.memoryType != memoryType {
			continue
		}
		if offset, ok := block.carve(requirements.Size, alignment); ok {
			return block.memory, imageSlice{block: block, offset: offset, size: requirements.Size}, nil
		}
	}
	memory, err := allocateDeviceMemory(renderer, memoryType, imageBlockSize, "allocate text image block")
	if err != nil {
		return 0, imageSlice{}, err
	}
	block := &imageBlock{memory: memory, memoryType: memoryType, size: imageBlockSize, free: []memorySpan{{0, imageBlockSize}}}
	pool.blocks = append(pool.blocks, block)
	offset, _ := block.carve(requirements.Size, alignment)
	return memory, imageSlice{block: block, offset: offset, size: requirements.Size}, nil
}

// carve takes size bytes at an offset aligned to alignment from the first free
// span that holds them.
func (block *imageBlock) carve(size, alignment uint64) (uint64, bool) {
	for index, span := range block.free {
		start := (span.offset + alignment - 1) / alignment * alignment
		end := span.offset + span.size
		if start+size > end {
			continue
		}
		// What is left before and after the image stays free.
		var parts [2]memorySpan
		count := 0
		if start > span.offset {
			parts[count] = memorySpan{span.offset, start - span.offset}
			count++
		}
		if start+size < end {
			parts[count] = memorySpan{start + size, end - start - size}
			count++
		}
		block.free = slices.Replace(block.free, index, index+1, parts[:count]...)
		return start, true
	}
	return 0, false
}

// release gives a label's memory back: to its block, or to the driver when it
// had memory of its own. A block that is left empty is freed unless it is the
// pool's last one, which a page that changes keeps using.
func (pool *imagePool) release(renderer *Renderer, memory uintptr, slice imageSlice) {
	block := slice.block
	if block == nil {
		if memory != 0 {
			renderer.api.freeMemory(renderer.device, memory, nil)
		}
		return
	}
	block.give(slice.offset, slice.size)
	if len(block.free) == 1 && block.free[0].size == block.size && len(pool.blocks) > 1 {
		renderer.api.freeMemory(renderer.device, block.memory, nil)
		pool.blocks = slices.DeleteFunc(pool.blocks, func(candidate *imageBlock) bool { return candidate == block })
	}
}

// give returns [offset, offset+size) to the free spans, joining its neighbours.
func (block *imageBlock) give(offset, size uint64) {
	index, _ := slices.BinarySearchFunc(block.free, offset, func(span memorySpan, offset uint64) int {
		switch {
		case span.offset < offset:
			return -1
		case span.offset > offset:
			return 1
		}
		return 0
	})
	block.free = slices.Insert(block.free, index, memorySpan{offset, size})
	if index+1 < len(block.free) && block.free[index].offset+block.free[index].size == block.free[index+1].offset {
		block.free[index].size += block.free[index+1].size
		block.free = slices.Delete(block.free, index+1, index+2)
	}
	if index > 0 && block.free[index-1].offset+block.free[index-1].size == block.free[index].offset {
		block.free[index-1].size += block.free[index].size
		block.free = slices.Delete(block.free, index, index+1)
	}
}

// close frees every block; the images in them are destroyed already.
func (pool *imagePool) close(renderer *Renderer) {
	if renderer != nil && renderer.api != nil && renderer.device != 0 {
		for _, block := range pool.blocks {
			renderer.api.freeMemory(renderer.device, block.memory, nil)
		}
	}
	pool.blocks = nil
}

func allocateDeviceMemory(renderer *Renderer, memoryType uint32, size uint64, action string) (uintptr, error) {
	var memory uintptr
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: size, MemoryTypeIndex: memoryType}
	err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &memory), action)
	return memory, err
}
