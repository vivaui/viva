package linux

import (
	"fmt"
	"unsafe"
)

const (
	vkCompositePreMultiplied = 2
	vkInstanceSlots          = 2
)

// vulkanFrame is what draws the frame of a window the application draws the
// frame of (window_frame.go): the content is drawn in a rectangle inside a
// margin, the content's corners are cut round, and a shadow is drawn in the
// margin. The target is transparent outside the content, so the swapchain
// composites with alpha.
//
// The cut is a draw that takes away: the shadow shader casts a shadow that has
// the content's rounded rectangle cut out of it, and with a shadow so large that
// it covers the whole corner, what is left is exactly the part outside the
// rounded corner. Blended to keep 1 - alpha of the target, it clears that.
// Two instances do it all, kept in a buffer of their own beside the scene's.
type vulkanFrame struct {
	// margin is the margin on every side in device pixels, 0 for a window with
	// the system's own frame. alpha says the swapchain composites with alpha, which
	// the window needs to know to keep the margin.
	margin uint32
	alpha  bool

	cutPipeline    uintptr
	buffer, memory uintptr
	mapped         unsafe.Pointer
}

func (frame *vulkanFrame) active() bool { return frame.margin > 0 }

// content is the rectangle of the target the scene is drawn in.
func (frame *vulkanFrame) content(width, height uint32) (x, y int32, size vkExtent2D) {
	box := frameContent(width, height, frame.margin)
	return box.x, box.y, vkExtent2D{Width: uint32(box.width), Height: uint32(box.height)}
}

// setFrameMargin gives the window a frame margin (0 takes it away) and draws
// again with it.
func (renderer *vulkanRenderer) setFrameMargin(margin uint32) error {
	if margin == renderer.frame.margin {
		return nil
	}
	renderer.frame.margin = margin
	if renderer.device == 0 || renderer.commandPool == 0 {
		return nil
	}
	if err := vkCheck(renderer.api.deviceWaitIdle(renderer.device), "wait before frame change"); err != nil {
		return err
	}
	if err := renderer.resetCommands("reset commands before frame change"); err != nil {
		return err
	}
	if margin > 0 {
		if err := renderer.frame.create(renderer); err != nil {
			return err
		}
	}
	return renderer.recordCommands(vkExtent2D{Width: renderer.width, Height: renderer.height})
}

// create makes the pipeline and the buffer, which the render pass and the
// device own; it is called with a swapchain's pipelines, and again when a
// margin is given to a window that had none.
func (frame *vulkanFrame) create(renderer *vulkanRenderer) error {
	if frame.cutPipeline == 0 {
		pipeline, err := renderer.scene.buildPipeline(renderer, true)
		if err != nil {
			return err
		}
		frame.cutPipeline = pipeline
	}
	if frame.buffer != 0 {
		return nil
	}
	size := uint64(vkInstanceSlots * vkInstanceBytes)
	info := vkBufferCreateInfo{SType: vkStructureBufferCreateInfo, Size: size, Usage: vkBufferUsageVertex, SharingMode: vkSharingExclusive}
	if err := vkCheck(renderer.api.createBuffer(renderer.device, &info, nil, &frame.buffer), "create frame buffer"); err != nil {
		return err
	}
	var requirements vkMemoryRequirements
	renderer.api.getBufferMemoryRequirements(renderer.device, frame.buffer, &requirements)
	memoryType, ok := renderer.memoryType(requirements.MemoryTypeBits, vkMemoryHostVisible|vkMemoryHostCoherent)
	if !ok {
		return fmt.Errorf("linux: Vulkan device has no coherent host-visible memory for the window frame")
	}
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &frame.memory), "allocate frame memory"); err != nil {
		return err
	}
	if err := vkCheck(renderer.api.bindBufferMemory(renderer.device, frame.buffer, frame.memory, 0), "bind frame memory"); err != nil {
		return err
	}
	return vkCheck(renderer.api.mapMemory(renderer.device, frame.memory, 0, requirements.Size, 0, &frame.mapped), "map frame memory")
}

// destroyPipeline releases what a render pass owns.
func (frame *vulkanFrame) destroyPipeline(renderer *vulkanRenderer) {
	if frame.cutPipeline != 0 {
		renderer.api.destroyPipeline(renderer.device, frame.cutPipeline, nil)
		frame.cutPipeline = 0
	}
}

// close releases the buffer.
func (frame *vulkanFrame) close(renderer *vulkanRenderer) {
	frame.destroyPipeline(renderer)
	if frame.mapped != nil && frame.memory != 0 {
		renderer.api.unmapMemory(renderer.device, frame.memory)
	}
	frame.mapped = nil
	if frame.buffer != 0 {
		renderer.api.destroyBuffer(renderer.device, frame.buffer, nil)
		frame.buffer = 0
	}
	if frame.memory != 0 {
		renderer.api.freeMemory(renderer.device, frame.memory, nil)
		frame.memory = 0
	}
}

// upload writes the two instances for a target of width x height device pixels:
// slot 0 is the cut, slot 1 the shadow. Both cast from the content rectangle,
// in logical pixels of the whole target.
func (frame *vulkanFrame) upload(renderer *vulkanRenderer, width, height uint32) {
	if frame.mapped == nil {
		return
	}
	scale := renderer.pixelScale()
	box := frameContent(width, height, frame.margin)
	bounds := [4]float32{float32(box.x) / scale, float32(box.y) / scale, float32(box.width) / scale, float32(box.height) / scale}
	cut := vulkanRectangleInstance{
		Bounds: bounds, Color: [4]float32{0, 0, 0, 1},
		// The shadow grows by its spread around the corner's circle, and a spread
		// of the radius's size or more makes it cover the corner square entirely:
		// the far point of the square is the radius times the square root of two
		// from the circle's center, less than the radius plus the spread plus the
		// one pixel of blur that still counts as full strength.
		Style: [4]float32{frameRadiusLogical, 0, 1, 0}, Shade1: [4]float32{0, 0, frameRadiusLogical},
	}
	shadow := vulkanRectangleInstance{
		Bounds: bounds, Color: [4]float32{0, 0, 0, frameShadowAlpha},
		Style: [4]float32{frameRadiusLogical, 0, frameShadowBlur, 0}, Shade1: [4]float32{0, frameShadowOffset, 0},
	}
	instances := (*[vkInstanceSlots]vulkanRectangleInstance)(frame.mapped)
	instances[0], instances[1] = cut, shadow
}

// record draws the corner cuts and the shadow after the scene. The scene's
// viewport is the content; these are positioned in the whole target.
func (frame *vulkanFrame) record(renderer *vulkanRenderer, command uintptr, extent vkExtent2D) {
	if frame.mapped == nil || frame.cutPipeline == 0 {
		return
	}
	scratch := &renderer.scratch
	scale := renderer.pixelScale()
	scratch.viewport = vkViewport{Width: float32(extent.Width), Height: float32(extent.Height), MaximumDepth: 1}
	renderer.api.cmdSetViewport(command, 0, 1, &scratch.viewport)
	scratch.sceneFrame = [6]float32{float32(extent.Width) / scale, float32(extent.Height) / scale, 1 / scale, 0, 0, 0}
	renderer.api.cmdPushConstants(command, renderer.scene.pipelineLayout, vkShaderStageVertex|vkShaderStageFragment, 0, uint32(unsafe.Sizeof(scratch.sceneFrame)), unsafe.Pointer(&scratch.sceneFrame[0]))
	scratch.offset = 0
	renderer.api.cmdBindVertexBuffers(command, 0, 1, &frame.buffer, &scratch.offset)
	renderer.api.cmdBindPipeline(command, vkPipelineBindPointGraphics, frame.cutPipeline)
	for _, corner := range frameCorners(extent.Width, extent.Height, frame.margin, frameRadiusLogical) {
		frame.draw(renderer, command, corner, 0)
	}
	// The shadow goes behind the content, so it is drawn where nothing of the
	// content is left: the margin, and the corner squares the cut just emptied.
	renderer.api.cmdBindPipeline(command, vkPipelineBindPointGraphics, renderer.scene.pipeline)
	for _, band := range frameBands(extent.Width, extent.Height, frame.margin) {
		frame.draw(renderer, command, band, 1)
	}
	for _, corner := range frameCorners(extent.Width, extent.Height, frame.margin, frameRadiusLogical) {
		frame.draw(renderer, command, corner, 1)
	}
}

func (frame *vulkanFrame) draw(renderer *vulkanRenderer, command uintptr, box pixelBox, instance uint32) {
	if box.width <= 0 || box.height <= 0 {
		return
	}
	scratch := &renderer.scratch
	scratch.scissor = vkRect2D{Offset: [2]int32{box.x, box.y}, Extent: vkExtent2D{Width: uint32(box.width), Height: uint32(box.height)}}
	renderer.api.cmdSetScissor(command, 0, 1, &scratch.scissor)
	renderer.api.cmdDraw(command, 6, 1, 0, instance)
}
