//go:build linux

package vulkan

import (
	"fmt"
	"image"
	"math"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

const (
	vkImageUsageTransferSource       = 0x1
	vkBufferUsageTransferDestination = 0x2
	vkImageLayoutTransferSource      = 6
	vkAccessColorAttachmentWrite     = 0x100
	vkAccessTransferRead             = 0x800
	vkPipelineStageTransferBit       = 0x1000
	// maxScale is the largest display scale a scene is drawn at.
	maxScale = platform.MaxDisplayScale
)

// Offscreen renders scenes to pixels with the window's own Vulkan pipelines
// and command recording but without an X window, a surface or a swapchain: the
// target is a plain color image that is copied to host memory. A screenshot or
// a pixel test therefore shows what the window would. It is not safe for
// concurrent use.
type Offscreen struct {
	renderer *Renderer
}

// vulkanOffscreen is the renderer's offscreen target: the color image the
// scene is drawn into and the host buffer it is read back through.
type vulkanOffscreen struct {
	image, memory            uintptr
	readback, readbackMemory uintptr
	readbackCommand          uintptr
	readbackSize             uint64
}

// NewOffscreen creates a renderer for a width x height scene in logical pixels,
// drawn at scale device pixels per logical pixel (the image is
// ceil(width*scale) by ceil(height*scale)), cleared to clear before each render.
// It fails with platform.ErrNoGraphics when the machine has no Vulkan driver.
// rasterizer makes what draws the glyphs of the labels.
func NewOffscreen(width, height int, scale float64, clear platform.Color, rasterizer NewTextRasterizer) (*Offscreen, error) {
	if width < 1 || height < 1 || width > platform.MaxOffscreenSize || height > platform.MaxOffscreenSize || math.IsNaN(scale) || scale < 1 || scale > maxScale {
		return nil, fmt.Errorf("vulkan: invalid offscreen size %dx%d at %gx", width, height, scale)
	}
	deviceWidth, deviceHeight := int(math.Ceil(float64(width)*scale)), int(math.Ceil(float64(height)*scale))
	if deviceWidth > platform.MaxOffscreenSize || deviceHeight > platform.MaxOffscreenSize {
		return nil, fmt.Errorf("vulkan: offscreen size %dx%d at %gx is beyond %d device pixels", width, height, scale, platform.MaxOffscreenSize)
	}
	api, err := loadVulkan()
	if err != nil {
		return nil, err
	}
	renderer := &Renderer{api: api, width: uint32(deviceWidth), height: uint32(deviceHeight), scale: float32(scale), color: clear, newRasterizer: rasterizer}
	if err := renderer.initializeOffscreen(); err != nil {
		renderer.Close()
		return nil, err
	}
	return &Offscreen{renderer: renderer}, nil
}

// Set replaces the scene: rectangle and triangle instances in paint order and
// text labels.
func (offscreen *Offscreen) Set(rectangles []render.Rectangle, labels []platform.TextLabel) error {
	if offscreen.renderer == nil {
		return fmt.Errorf("vulkan: offscreen renderer is closed")
	}
	if labels == nil {
		labels = []platform.TextLabel{}
	}
	_, err := offscreen.renderer.Apply(platform.Frame{
		Update:     render.Update{Count: len(rectangles), Rebuild: true},
		Rectangles: rectangles,
		TextLabels: labels,
	})
	return err
}

// Image draws the scene and returns its pixels, top row first. The image is
// premultiplied RGBA, the format of the color target converted from the
// target's BGRA byte order; the target is not sRGB, so values are the ones the
// shaders wrote.
func (offscreen *Offscreen) Image() (*image.RGBA, error) {
	if offscreen.renderer == nil {
		return nil, fmt.Errorf("vulkan: offscreen renderer is closed")
	}
	return offscreen.renderer.readImage()
}

// Close releases the renderer.
// Stats reports the renderer's counters.
func (offscreen *Offscreen) Stats() Stats {
	if offscreen.renderer == nil {
		return Stats{}
	}
	return offscreen.renderer.Stats()
}

// RefreshText draws the labels again with the text engine's current settings.
func (offscreen *Offscreen) RefreshText() error {
	if offscreen.renderer == nil {
		return fmt.Errorf("vulkan: offscreen renderer is closed")
	}
	return offscreen.renderer.RefreshText()
}

// DualSource reports whether the device blends dual-source, which LCD text needs.
func (offscreen *Offscreen) DualSource() bool {
	return offscreen.renderer != nil && offscreen.renderer.dualSource
}

func (offscreen *Offscreen) Close() {
	if offscreen.renderer != nil {
		offscreen.renderer.Close()
		offscreen.renderer = nil
	}
}

func (renderer *Renderer) initializeOffscreen() error {
	device := vulkanDevice{api: renderer.api}
	if err := device.open(); err != nil {
		return err
	}
	renderer.adopt(&device)
	if err := renderer.scene.upload(renderer, nil, render.Update{Rebuild: true}); err != nil {
		return err
	}
	if err := renderer.createOffscreenTarget(); err != nil {
		return err
	}
	semaphoreInfo := vkSemaphoreCreateInfo{SType: vkStructureSemaphoreCreateInfo}
	if err := vkCheck(renderer.api.createSemaphore(renderer.device, &semaphoreInfo, nil, &renderer.imageAvailable), "create acquire semaphore"); err != nil {
		return err
	}
	fenceInfo := vkFenceCreateInfo{SType: vkStructureFenceCreateInfo, Flags: vkFenceSignaled}
	return vkCheck(renderer.api.createFence(renderer.device, &fenceInfo, nil, &renderer.fence), "create frame fence")
}

// createOffscreenTarget builds what createSwapchain builds for a window, with
// one color image instead of the swapchain's: image, view, render pass,
// framebuffer and commands, then the shared pipelines. The render pass leaves
// the image ready to be copied.
func (renderer *Renderer) createOffscreenTarget() (err error) {
	defer func() {
		if err != nil {
			renderer.destroySwapchain()
			renderer.destroyOffscreen()
		}
	}()
	target := &renderer.offscreen
	extent := vkExtent2D{Width: renderer.width, Height: renderer.height}
	imageInfo := vkImageCreateInfo{
		SType: vkStructureTextImageCreateInfo, ImageType: vkImageType2D, Format: vkFormatBGRA8,
		Extent: vkExtent3D{Width: extent.Width, Height: extent.Height, Depth: 1}, MipLevels: 1, ArrayLayers: 1,
		Samples: vkSampleCountOne, Tiling: vkImageTilingOptimal,
		Usage: vkImageUsageColorAttachment | vkImageUsageTransferSource, SharingMode: vkSharingExclusive,
	}
	if err := vkCheck(renderer.api.createImage(renderer.device, &imageInfo, nil, &target.image), "create offscreen image"); err != nil {
		return err
	}
	var requirements vkMemoryRequirements
	renderer.api.getImageMemoryRequirements(renderer.device, target.image, &requirements)
	memoryType, ok := renderer.memoryType(requirements.MemoryTypeBits, vkMemoryDeviceLocal)
	if !ok {
		return fmt.Errorf("vulkan: Vulkan device has no device-local image memory")
	}
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &target.memory), "allocate offscreen image memory"); err != nil {
		return err
	}
	if err := vkCheck(renderer.api.bindImageMemory(renderer.device, target.image, target.memory, 0), "bind offscreen image memory"); err != nil {
		return err
	}
	renderer.views = make([]uintptr, 1)
	viewInfo := vkImageViewCreateInfo{SType: vkStructureImageViewCreateInfo, Image: target.image, ViewType: vkImageViewType2D, Format: vkFormatBGRA8, Range: vkSubresourceRange{Aspect: vkImageAspectColor, LevelCount: 1, LayerCount: 1}}
	if err := vkCheck(renderer.api.createImageView(renderer.device, &viewInfo, nil, &renderer.views[0]), "create offscreen image view"); err != nil {
		return err
	}
	if err := renderer.createRenderPass(vkFormatBGRA8, vkImageLayoutTransferSource); err != nil {
		return err
	}
	renderer.framebuffers = make([]uintptr, 1)
	view := renderer.views[0]
	framebufferInfo := vkFramebufferCreateInfo{SType: vkStructureFramebufferCreateInfo, RenderPass: renderer.renderPass, AttachmentCount: 1, Attachments: &view, Width: extent.Width, Height: extent.Height, Layers: 1}
	if err := vkCheck(renderer.api.createFramebuffer(renderer.device, &framebufferInfo, nil, &renderer.framebuffers[0]), "create offscreen framebuffer"); err != nil {
		return err
	}
	if err := renderer.createCommands(1); err != nil {
		return err
	}
	// The readback buffer is host memory the image is copied into.
	target.readbackSize = uint64(extent.Width) * uint64(extent.Height) * 4
	bufferInfo := vkBufferCreateInfo{SType: vkStructureBufferCreateInfo, Size: target.readbackSize, Usage: vkBufferUsageTransferDestination, SharingMode: vkSharingExclusive}
	if err := vkCheck(renderer.api.createBuffer(renderer.device, &bufferInfo, nil, &target.readback), "create readback buffer"); err != nil {
		return err
	}
	renderer.api.getBufferMemoryRequirements(renderer.device, target.readback, &requirements)
	memoryType, ok = renderer.memoryType(requirements.MemoryTypeBits, vkMemoryHostVisible|vkMemoryHostCoherent)
	if !ok {
		return fmt.Errorf("vulkan: Vulkan device has no coherent host-visible memory for readback")
	}
	allocation = vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &target.readbackMemory), "allocate readback memory"); err != nil {
		return err
	}
	if err := vkCheck(renderer.api.bindBufferMemory(renderer.device, target.readback, target.readbackMemory, 0), "bind readback memory"); err != nil {
		return err
	}
	allocate := vkCommandBufferAllocateInfo{SType: vkStructureCommandAllocateInfo, Pool: renderer.commandPool, Level: vkCommandBufferPrimary, Count: 1}
	if err := vkCheck(renderer.api.allocateCommandBuffers(renderer.device, &allocate, &target.readbackCommand), "allocate readback command"); err != nil {
		return err
	}
	if err := renderer.createScenePipelines(extent); err != nil {
		return err
	}
	renderer.format = vkFormatBGRA8
	return nil
}

// readImage submits the recorded scene and a copy of the target into the
// readback buffer, waits for them and converts the bytes to an image.
func (renderer *Renderer) readImage() (*image.RGBA, error) {
	target := &renderer.offscreen
	if err := vkCheck(renderer.api.waitForFences(renderer.device, 1, &renderer.fence, 1, math.MaxUint64), "wait before readback"); err != nil {
		return nil, err
	}
	if err := renderer.ensureRecorded(0); err != nil {
		return nil, err
	}
	begin := vkCommandBufferBeginInfo{SType: vkStructureCommandBeginInfo, Flags: vkCommandOneTimeSubmit}
	if err := vkCheck(renderer.api.beginCommandBuffer(target.readbackCommand, &begin), "begin readback command"); err != nil {
		return nil, err
	}
	// The render pass already left the image in the copy layout; the barrier
	// makes its writes visible to the copy.
	barrier := vkImageMemoryBarrier{
		SType: vkStructureImageMemoryBarrier, SourceAccess: vkAccessColorAttachmentWrite, DestinationAccess: vkAccessTransferRead,
		OldLayout: vkImageLayoutTransferSource, NewLayout: vkImageLayoutTransferSource,
		SourceQueueFamily: vkQueueFamilyIgnored, DestinationQueueFamily: vkQueueFamilyIgnored, Image: target.image,
		Range: vkSubresourceRange{Aspect: vkImageAspectColor, LevelCount: 1, LayerCount: 1},
	}
	renderer.api.cmdPipelineBarrier(target.readbackCommand, vkPipelineStageColorOutput, vkPipelineStageTransferBit, 0, 0, nil, 0, nil, 1, &barrier)
	region := vkBufferImageCopy{
		ImageSubresource: vkImageSubresourceLayers{Aspect: vkImageAspectColor, LayerCount: 1},
		ImageExtent:      vkExtent3D{Width: renderer.width, Height: renderer.height, Depth: 1},
	}
	renderer.api.cmdCopyImageToBuffer(target.readbackCommand, target.image, vkImageLayoutTransferSource, target.readback, 1, &region)
	if err := vkCheck(renderer.api.endCommandBuffer(target.readbackCommand), "end readback command"); err != nil {
		return nil, err
	}
	if err := vkCheck(renderer.api.resetFences(renderer.device, 1, &renderer.fence), "reset frame fence"); err != nil {
		return nil, err
	}
	commands := [2]uintptr{renderer.commands[0], target.readbackCommand}
	submit := vkSubmitInfo{SType: vkStructureSubmitInfo, CommandCount: 2, Commands: &commands[0]}
	if err := vkCheck(renderer.api.queueSubmit(renderer.queue, 1, &submit, renderer.fence), "submit offscreen render"); err != nil {
		return nil, err
	}
	if err := vkCheck(renderer.api.waitForFences(renderer.device, 1, &renderer.fence, 1, math.MaxUint64), "wait for offscreen render"); err != nil {
		return nil, err
	}
	var mapped unsafe.Pointer
	if err := vkCheck(renderer.api.mapMemory(renderer.device, target.readbackMemory, 0, target.readbackSize, 0, &mapped), "map readback memory"); err != nil {
		return nil, err
	}
	defer renderer.api.unmapMemory(renderer.device, target.readbackMemory)
	bgra := unsafe.Slice((*byte)(mapped), int(target.readbackSize))
	result := image.NewRGBA(image.Rect(0, 0, int(renderer.width), int(renderer.height)))
	for offset := 0; offset+3 < len(bgra); offset += 4 {
		result.Pix[offset], result.Pix[offset+1], result.Pix[offset+2], result.Pix[offset+3] = bgra[offset+2], bgra[offset+1], bgra[offset], bgra[offset+3]
	}
	renderer.submissions++
	return result, nil
}

func (renderer *Renderer) destroyOffscreen() {
	target := &renderer.offscreen
	if renderer.device != 0 {
		if target.readback != 0 {
			renderer.api.destroyBuffer(renderer.device, target.readback, nil)
		}
		if target.readbackMemory != 0 {
			renderer.api.freeMemory(renderer.device, target.readbackMemory, nil)
		}
		if target.image != 0 {
			renderer.api.destroyImage(renderer.device, target.image, nil)
		}
		if target.memory != 0 {
			renderer.api.freeMemory(renderer.device, target.memory, nil)
		}
	}
	*target = vulkanOffscreen{}
}
