//go:build linux

package vulkan

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"
)

// stagingChunk is one host-visible buffer the label pixels are copied through,
// mapped for as long as it lives.
type stagingChunk struct {
	buffer, memory uintptr
	mapped         unsafe.Pointer
	size, used     uint64
}

// textStaging is where an upload's label pixels wait for the copy into their
// images: chunks kept between uploads, so a steady page allocates no staging
// memory, and an upload fence instead of waiting for the whole device to idle.
// An upload that outgrows its chunk takes another; the next upload, once the
// fence says the copies are done, merges them into one of their total size.
type textStaging struct {
	chunks  []stagingChunk
	fence   uintptr
	pending bool // an upload was submitted and its fence not yet waited for
}

// stagingChunkMinimum is the smallest chunk: a few hundred terminal rows.
const stagingChunkMinimum = 1 << 20

// settle waits for the last upload's copies, after which its chunks and the
// images it wrote may be reused or destroyed.
func (staging *textStaging) settle(renderer *Renderer) error {
	if !staging.pending {
		return nil
	}
	if err := vkCheck(renderer.api.waitForFences(renderer.device, 1, &staging.fence, 1, math.MaxUint64), "wait for text upload"); err != nil {
		return err
	}
	staging.pending = false
	return nil
}

// begin readies the chunks for an upload: the last one has settled, and the
// chunks it needed are one.
func (staging *textStaging) begin(renderer *Renderer) error {
	if err := staging.settle(renderer); err != nil {
		return err
	}
	if staging.fence == 0 {
		info := vkFenceCreateInfo{SType: vkStructureFenceCreateInfo}
		if err := vkCheck(renderer.api.createFence(renderer.device, &info, nil, &staging.fence), "create text upload fence"); err != nil {
			return err
		}
	}
	if len(staging.chunks) > 1 {
		var total uint64
		for index := range staging.chunks {
			total += staging.chunks[index].size
		}
		staging.closeChunks(renderer)
		if err := staging.grow(renderer, total); err != nil {
			return err
		}
	}
	for index := range staging.chunks {
		staging.chunks[index].used = 0
	}
	return nil
}

// stage copies pixels into a chunk and returns the buffer and offset the copy
// into the image reads them from.
func (staging *textStaging) stage(renderer *Renderer, pixels []byte) (buffer uintptr, offset uint64, err error) {
	need := uint64(len(pixels))
	last := len(staging.chunks) - 1
	if last < 0 || staging.chunks[last].size-staging.chunks[last].used < need {
		size := uint64(stagingChunkMinimum)
		if last >= 0 {
			size = max(size, staging.chunks[last].size*2)
		}
		if err := staging.grow(renderer, max(size, need)); err != nil {
			return 0, 0, err
		}
		last = len(staging.chunks) - 1
	}
	chunk := &staging.chunks[last]
	offset = chunk.used
	copy(unsafe.Slice((*byte)(unsafe.Add(chunk.mapped, offset)), len(pixels)), pixels)
	runtime.KeepAlive(pixels)
	// A copy reads its texels from an offset that is a multiple of the texel's
	// size; 16 suits every format and the optimal copy alignment of common drivers.
	chunk.used = (offset + need + 15) &^ 15
	return chunk.buffer, offset, nil
}

// submit hands the upload command to the queue with the fence.
func (staging *textStaging) submit(renderer *Renderer, command uintptr) error {
	if err := vkCheck(renderer.api.resetFences(renderer.device, 1, &staging.fence), "reset text upload fence"); err != nil {
		return err
	}
	submit := vkSubmitInfo{SType: vkStructureSubmitInfo, CommandCount: 1, Commands: &command}
	if err := vkCheck(renderer.api.queueSubmit(renderer.queue, 1, &submit, staging.fence), "submit text upload"); err != nil {
		return err
	}
	staging.pending = true
	return nil
}

func (staging *textStaging) grow(renderer *Renderer, size uint64) (err error) {
	var chunk stagingChunk
	defer func() {
		if err != nil {
			if chunk.memory != 0 {
				renderer.api.freeMemory(renderer.device, chunk.memory, nil)
			}
			if chunk.buffer != 0 {
				renderer.api.destroyBuffer(renderer.device, chunk.buffer, nil)
			}
		}
	}()
	info := vkBufferCreateInfo{SType: vkStructureBufferCreateInfo, Size: size, Usage: vkBufferUsageTransferSource, SharingMode: vkSharingExclusive}
	if err := vkCheck(renderer.api.createBuffer(renderer.device, &info, nil, &chunk.buffer), "create text staging buffer"); err != nil {
		return err
	}
	var requirements vkMemoryRequirements
	renderer.api.getBufferMemoryRequirements(renderer.device, chunk.buffer, &requirements)
	memoryType, ok := renderer.memoryType(requirements.MemoryTypeBits, vkMemoryHostVisible|vkMemoryHostCoherent)
	if !ok {
		return fmt.Errorf("Vulkan device has no coherent staging memory")
	}
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &chunk.memory), "allocate text staging memory"); err != nil {
		return err
	}
	if err := vkCheck(renderer.api.bindBufferMemory(renderer.device, chunk.buffer, chunk.memory, 0), "bind text staging memory"); err != nil {
		return err
	}
	if err := vkCheck(renderer.api.mapMemory(renderer.device, chunk.memory, 0, size, 0, &chunk.mapped), "map text staging memory"); err != nil {
		return err
	}
	chunk.size = size
	staging.chunks = append(staging.chunks, chunk)
	return nil
}

func (staging *textStaging) closeChunks(renderer *Renderer) {
	for index := range staging.chunks {
		chunk := &staging.chunks[index]
		renderer.api.unmapMemory(renderer.device, chunk.memory)
		renderer.api.destroyBuffer(renderer.device, chunk.buffer, nil)
		renderer.api.freeMemory(renderer.device, chunk.memory, nil)
	}
	staging.chunks = staging.chunks[:0]
}

// close destroys the chunks and the fence; the device has finished with them.
func (staging *textStaging) close(renderer *Renderer) {
	if renderer == nil || renderer.api == nil || renderer.device == 0 {
		*staging = textStaging{}
		return
	}
	staging.closeChunks(renderer)
	if staging.fence != 0 {
		renderer.api.destroyFence(renderer.device, staging.fence, nil)
	}
	*staging = textStaging{}
}
