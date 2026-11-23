//go:build linux

package vulkan

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

// vkCall is one Vulkan entry point called through purego.SyscallN. purego.RegisterLibFunc
// builds its callers with reflection, which allocates for every argument and result; here the
// arguments are written into args, a fixed array of the call, and the call itself allocates
// nothing. SyscallN passes integers and pointers only, and Vulkan's other arguments (viewports,
// extents, constants) go by address, so every function bound below fits.
//
// A vkCall is not safe for concurrent use. The renderer calls Vulkan from one thread and
// nothing in this package starts a goroutine.
//
// A Go pointer stored as a uintptr is not seen by the garbage collector, so each wrapper keeps
// its pointer arguments alive with runtime.KeepAlive until the call has returned. The pointees
// are heap objects already: they are passed through a function value, which makes the compiler
// allocate them on the heap, and the heap does not move.
type vkCall struct {
	address uintptr
	args    [8]uintptr
}

func newVkCall(library uintptr, name string) *vkCall {
	address, err := purego.Dlsym(library, name)
	if err != nil {
		panic(fmt.Sprintf("vulkan: Vulkan function %s: %v", name, err))
	}
	return &vkCall{address: address}
}

func (call *vkCall) invoke(count int) int32 {
	result, _, _ := purego.SyscallN(call.address, call.args[:count]...)
	return int32(uint32(result))
}

// bindVulkanHotPath replaces the entry points a repaint calls every time with allocation-free
// callers. Setup and teardown calls stay on purego.RegisterLibFunc: they run once, and a
// reflection call is simpler to read than one more wrapper.
func bindVulkanHotPath(api *vulkanAPI, library uintptr) {
	resetCommandPool := newVkCall(library, "vkResetCommandPool")
	api.resetCommandPool = func(device, pool uintptr, flags uint32) int32 {
		resetCommandPool.args = [8]uintptr{device, pool, uintptr(flags)}
		return resetCommandPool.invoke(3)
	}
	beginCommandBuffer := newVkCall(library, "vkBeginCommandBuffer")
	api.beginCommandBuffer = func(command uintptr, info *vkCommandBufferBeginInfo) int32 {
		beginCommandBuffer.args = [8]uintptr{command, uintptr(unsafe.Pointer(info))}
		result := beginCommandBuffer.invoke(2)
		runtime.KeepAlive(info)
		return result
	}
	endCommandBuffer := newVkCall(library, "vkEndCommandBuffer")
	api.endCommandBuffer = func(command uintptr) int32 {
		endCommandBuffer.args = [8]uintptr{command}
		return endCommandBuffer.invoke(1)
	}
	cmdBeginRenderPass := newVkCall(library, "vkCmdBeginRenderPass")
	api.cmdBeginRenderPass = func(command uintptr, info *vkRenderPassBeginInfo, contents uint32) {
		cmdBeginRenderPass.args = [8]uintptr{command, uintptr(unsafe.Pointer(info)), uintptr(contents)}
		cmdBeginRenderPass.invoke(3)
		runtime.KeepAlive(info)
	}
	cmdEndRenderPass := newVkCall(library, "vkCmdEndRenderPass")
	api.cmdEndRenderPass = func(command uintptr) {
		cmdEndRenderPass.args = [8]uintptr{command}
		cmdEndRenderPass.invoke(1)
	}
	waitForFences := newVkCall(library, "vkWaitForFences")
	api.waitForFences = func(device uintptr, count uint32, fences *uintptr, waitAll uint32, timeout uint64) int32 {
		waitForFences.args = [8]uintptr{device, uintptr(count), uintptr(unsafe.Pointer(fences)), uintptr(waitAll), uintptr(timeout)}
		result := waitForFences.invoke(5)
		runtime.KeepAlive(fences)
		return result
	}
	resetFences := newVkCall(library, "vkResetFences")
	api.resetFences = func(device uintptr, count uint32, fences *uintptr) int32 {
		resetFences.args = [8]uintptr{device, uintptr(count), uintptr(unsafe.Pointer(fences))}
		result := resetFences.invoke(3)
		runtime.KeepAlive(fences)
		return result
	}
	acquireNextImage := newVkCall(library, "vkAcquireNextImageKHR")
	api.acquireNextImage = func(device, swapchain uintptr, timeout uint64, semaphore, fence uintptr, index *uint32) int32 {
		acquireNextImage.args = [8]uintptr{device, swapchain, uintptr(timeout), semaphore, fence, uintptr(unsafe.Pointer(index))}
		result := acquireNextImage.invoke(6)
		runtime.KeepAlive(index)
		return result
	}
	queueSubmit := newVkCall(library, "vkQueueSubmit")
	api.queueSubmit = func(queue uintptr, count uint32, submits *vkSubmitInfo, fence uintptr) int32 {
		queueSubmit.args = [8]uintptr{queue, uintptr(count), uintptr(unsafe.Pointer(submits)), fence}
		result := queueSubmit.invoke(4)
		runtime.KeepAlive(submits)
		return result
	}
	queuePresent := newVkCall(library, "vkQueuePresentKHR")
	api.queuePresent = func(queue uintptr, info *vkPresentInfo) int32 {
		queuePresent.args = [8]uintptr{queue, uintptr(unsafe.Pointer(info))}
		result := queuePresent.invoke(2)
		runtime.KeepAlive(info)
		return result
	}
	cmdBindPipeline := newVkCall(library, "vkCmdBindPipeline")
	api.cmdBindPipeline = func(command uintptr, bindPoint uint32, pipeline uintptr) {
		cmdBindPipeline.args = [8]uintptr{command, uintptr(bindPoint), pipeline}
		cmdBindPipeline.invoke(3)
	}
	cmdSetViewport := newVkCall(library, "vkCmdSetViewport")
	api.cmdSetViewport = func(command uintptr, first, count uint32, viewports *vkViewport) {
		cmdSetViewport.args = [8]uintptr{command, uintptr(first), uintptr(count), uintptr(unsafe.Pointer(viewports))}
		cmdSetViewport.invoke(4)
		runtime.KeepAlive(viewports)
	}
	cmdSetScissor := newVkCall(library, "vkCmdSetScissor")
	api.cmdSetScissor = func(command uintptr, first, count uint32, scissors *vkRect2D) {
		cmdSetScissor.args = [8]uintptr{command, uintptr(first), uintptr(count), uintptr(unsafe.Pointer(scissors))}
		cmdSetScissor.invoke(4)
		runtime.KeepAlive(scissors)
	}
	cmdPushConstants := newVkCall(library, "vkCmdPushConstants")
	api.cmdPushConstants = func(command, layout uintptr, stages, offset, size uint32, values unsafe.Pointer) {
		cmdPushConstants.args = [8]uintptr{command, layout, uintptr(stages), uintptr(offset), uintptr(size), uintptr(values)}
		cmdPushConstants.invoke(6)
		runtime.KeepAlive(values)
	}
	cmdBindVertexBuffers := newVkCall(library, "vkCmdBindVertexBuffers")
	api.cmdBindVertexBuffers = func(command uintptr, first, count uint32, buffers *uintptr, offsets *uint64) {
		cmdBindVertexBuffers.args = [8]uintptr{command, uintptr(first), uintptr(count), uintptr(unsafe.Pointer(buffers)), uintptr(unsafe.Pointer(offsets))}
		cmdBindVertexBuffers.invoke(5)
		runtime.KeepAlive(buffers)
		runtime.KeepAlive(offsets)
	}
	cmdDraw := newVkCall(library, "vkCmdDraw")
	api.cmdDraw = func(command uintptr, vertices, instances, firstVertex, firstInstance uint32) {
		cmdDraw.args = [8]uintptr{command, uintptr(vertices), uintptr(instances), uintptr(firstVertex), uintptr(firstInstance)}
		cmdDraw.invoke(5)
	}
	cmdBindDescriptorSets := newVkCall(library, "vkCmdBindDescriptorSets")
	api.cmdBindDescriptorSets = func(command uintptr, bindPoint uint32, layout uintptr, first, count uint32, sets *uintptr, dynamicCount uint32, dynamicOffsets *uint32) {
		cmdBindDescriptorSets.args = [8]uintptr{command, uintptr(bindPoint), layout, uintptr(first), uintptr(count), uintptr(unsafe.Pointer(sets)), uintptr(dynamicCount), uintptr(unsafe.Pointer(dynamicOffsets))}
		cmdBindDescriptorSets.invoke(8)
		runtime.KeepAlive(sets)
		runtime.KeepAlive(dynamicOffsets)
	}
}
