package linux

import (
	"errors"
	"fmt"
	"math"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

const (
	vkStructureApplicationInfo       = 0
	vkStructureInstanceCreateInfo    = 1
	vkStructureDeviceQueueCreateInfo = 2
	vkStructureDeviceCreateInfo      = 3
	vkStructureSubmitInfo            = 4
	vkStructureFenceCreateInfo       = 8
	vkStructureSemaphoreCreateInfo   = 9
	vkStructureImageViewCreateInfo   = 15
	vkStructureRenderPassCreateInfo  = 38
	vkStructureFramebufferCreateInfo = 37
	vkStructureCommandPoolCreateInfo = 39
	vkStructureCommandAllocateInfo   = 40
	vkStructureCommandBeginInfo      = 42
	vkStructureRenderPassBeginInfo   = 43
	vkStructureXcbSurfaceCreateInfo  = 1000005000
	vkStructureSwapchainCreateInfo   = 1000001000
	vkStructurePresentInfo           = 1000001001
	vkQueueGraphics                  = 1
	vkImageUsageColorAttachment      = 0x10
	vkImageAspectColor               = 1
	vkPipelineBindPointGraphics      = 0
	vkPipelineStageColorOutput       = 0x400
	vkImageLayoutUndefined           = 0
	vkImageLayoutColorAttachment     = 2
	vkImageLayoutPresent             = 1000001002
	vkAttachmentLoadClear            = 1
	vkAttachmentLoadDontCare         = 2
	vkAttachmentStore                = 0
	vkAttachmentStoreDontCare        = 1
	vkSampleCountOne                 = 1
	vkImageViewType2D                = 1
	vkSharingExclusive               = 0
	vkCompositeOpaque                = 1
	vkPresentImmediate               = 0
	vkPresentMailbox                 = 1
	vkPresentFIFO                    = 2
	vkPresentFIFORelaxed             = 3
	vkSubpassContentsInline          = 0
	vkCommandBufferPrimary           = 0
	vkFenceSignaled                  = 1
	vkFormatBGRA8                    = 44
	vkColorSpaceSRGB                 = 0
	vkSuccess                        = 0
	vkSuboptimal                     = 1000001003
	vkErrorOutOfDate                 = -1000001004
	vkErrorInitializationFailed      = -3
	vkErrorIncompatibleDriver        = -9
)

type vkExtent2D struct{ Width, Height uint32 }
type vkApplicationInfo struct {
	SType       uint32
	Padding0    uint32
	Next        uintptr
	Application *byte
	AppVersion  uint32
	Padding1    uint32
	Engine      *byte
	EngineVer   uint32
	APIVersion  uint32
}
type vkInstanceCreateInfo struct {
	SType          uint32
	Padding0       uint32
	Next           uintptr
	Flags          uint32
	Padding1       uint32
	Application    *vkApplicationInfo
	LayerCount     uint32
	Padding2       uint32
	Layers         uintptr
	ExtensionCount uint32
	Padding3       uint32
	Extensions     uintptr
}
type vkXcbSurfaceCreateInfo struct {
	SType      uint32
	Padding0   uint32
	Next       uintptr
	Flags      uint32
	Padding1   uint32
	Connection uintptr
	Window     uint32
	Padding2   uint32
}
type vkQueueFamilyProperties struct {
	Flags, Count, TimestampBits uint32
	Granularity                 [3]uint32
}
type vkDeviceQueueCreateInfo struct {
	SType      uint32
	Padding0   uint32
	Next       uintptr
	Flags      uint32
	Family     uint32
	Count      uint32
	Padding1   uint32
	Priorities *float32
}
type vkDeviceCreateInfo struct {
	SType          uint32
	Padding0       uint32
	Next           uintptr
	Flags          uint32
	QueueCount     uint32
	Queues         *vkDeviceQueueCreateInfo
	LayerCount     uint32
	Padding1       uint32
	Layers         uintptr
	ExtensionCount uint32
	Padding2       uint32
	Extensions     uintptr
	Features       uintptr
}
type vkSurfaceCapabilities struct {
	MinImages, MaxImages uint32
	Current              vkExtent2D
	Minimum              vkExtent2D
	Maximum              vkExtent2D
	MaxLayers            uint32
	SupportedTransforms  uint32
	CurrentTransform     uint32
	CompositeAlpha       uint32
	Usage                uint32
}
type vkSurfaceFormat struct{ Format, ColorSpace uint32 }
type vkSwapchainCreateInfo struct {
	SType            uint32
	Padding0         uint32
	Next             uintptr
	Flags            uint32
	Padding1         uint32
	Surface          uintptr
	MinImages        uint32
	Format           uint32
	ColorSpace       uint32
	Extent           vkExtent2D
	Layers           uint32
	Usage            uint32
	SharingMode      uint32
	QueueFamilyCount uint32
	QueueFamilies    uintptr
	PreTransform     uint32
	CompositeAlpha   uint32
	PresentMode      uint32
	Clipped          uint32
	OldSwapchain     uintptr
}
type vkComponentMapping struct{ Red, Green, Blue, Alpha uint32 }
type vkSubresourceRange struct{ Aspect, BaseMip, LevelCount, BaseLayer, LayerCount uint32 }
type vkImageViewCreateInfo struct {
	SType      uint32
	Padding0   uint32
	Next       uintptr
	Flags      uint32
	Padding1   uint32
	Image      uintptr
	ViewType   uint32
	Format     uint32
	Components vkComponentMapping
	Range      vkSubresourceRange
}
type vkAttachmentDescription struct {
	Flags, Format, Samples, Load, Store, StencilLoad, StencilStore, InitialLayout, FinalLayout uint32
}
type vkAttachmentReference struct{ Attachment, Layout uint32 }
type vkSubpassDescription struct {
	Flags, BindPoint, InputCount uint32
	Padding0                     uint32
	Inputs                       uintptr
	ColorCount                   uint32
	Padding1                     uint32
	Colors, Resolves, Depth      uintptr
	PreserveCount                uint32
	Padding2                     uint32
	Preserves                    uintptr
}
type vkRenderPassCreateInfo struct {
	SType           uint32
	Padding0        uint32
	Next            uintptr
	Flags           uint32
	AttachmentCount uint32
	Attachments     *vkAttachmentDescription
	SubpassCount    uint32
	Padding1        uint32
	Subpasses       *vkSubpassDescription
	DependencyCount uint32
	Padding2        uint32
	Dependencies    uintptr
}
type vkFramebufferCreateInfo struct {
	SType           uint32
	Padding0        uint32
	Next            uintptr
	Flags           uint32
	Padding1        uint32
	RenderPass      uintptr
	AttachmentCount uint32
	Padding2        uint32
	Attachments     *uintptr
	Width, Height   uint32
	Layers          uint32
	Padding3        uint32
}
type vkCommandPoolCreateInfo struct {
	SType    uint32
	Padding0 uint32
	Next     uintptr
	Flags    uint32
	Family   uint32
}
type vkCommandBufferAllocateInfo struct {
	SType    uint32
	Padding0 uint32
	Next     uintptr
	Pool     uintptr
	Level    uint32
	Count    uint32
}
type vkCommandBufferBeginInfo struct {
	SType       uint32
	Padding0    uint32
	Next        uintptr
	Flags       uint32
	Padding1    uint32
	Inheritance uintptr
}
type vkRect2D struct {
	Offset [2]int32
	Extent vkExtent2D
}
type vkClearValue struct{ Color [4]float32 }
type vkClearAttachment struct {
	AspectMask      uint32
	ColorAttachment uint32
	ClearValue      vkClearValue
}
type vkClearRect struct {
	Rect           vkRect2D
	BaseArrayLayer uint32
	LayerCount     uint32
}
type vkRenderPassBeginInfo struct {
	SType       uint32
	Padding0    uint32
	Next        uintptr
	RenderPass  uintptr
	Framebuffer uintptr
	Area        vkRect2D
	ClearCount  uint32
	Padding1    uint32
	Clear       *vkClearValue
}
type vkSemaphoreCreateInfo struct {
	SType    uint32
	Padding0 uint32
	Next     uintptr
	Flags    uint32
	Padding1 uint32
}
type vkFenceCreateInfo = vkSemaphoreCreateInfo
type vkSubmitInfo struct {
	SType        uint32
	Padding0     uint32
	Next         uintptr
	WaitCount    uint32
	Padding1     uint32
	Waits        *uintptr
	WaitStages   *uint32
	CommandCount uint32
	Padding2     uint32
	Commands     *uintptr
	SignalCount  uint32
	Padding3     uint32
	Signals      *uintptr
}
type vkPresentInfo struct {
	SType      uint32
	Padding0   uint32
	Next       uintptr
	WaitCount  uint32
	Padding1   uint32
	Waits      *uintptr
	SwapCount  uint32
	Padding2   uint32
	Swapchains *uintptr
	Indices    *uint32
	Results    *int32
}

type vulkanAPI struct {
	library                           uintptr
	createInstance                    func(*vkInstanceCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyInstance                   func(uintptr, unsafe.Pointer)
	enumeratePhysicalDevices          func(uintptr, *uint32, *uintptr) int32
	getQueueFamilyProperties          func(uintptr, *uint32, *vkQueueFamilyProperties)
	getPhysicalDeviceFeatures         func(uintptr, *[vkFeatureCount]uint32)
	createXcbSurface                  func(uintptr, *vkXcbSurfaceCreateInfo, unsafe.Pointer, *uintptr) int32
	destroySurface                    func(uintptr, uintptr, unsafe.Pointer)
	getSurfaceSupport                 func(uintptr, uint32, uintptr, *uint32) int32
	getSurfaceCapabilities            func(uintptr, uintptr, *vkSurfaceCapabilities) int32
	getSurfaceFormats                 func(uintptr, uintptr, *uint32, *vkSurfaceFormat) int32
	getSurfacePresentModes            func(uintptr, uintptr, *uint32, *uint32) int32
	createDevice                      func(uintptr, *vkDeviceCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyDevice                     func(uintptr, unsafe.Pointer)
	getDeviceQueue                    func(uintptr, uint32, uint32, *uintptr)
	createSwapchain                   func(uintptr, *vkSwapchainCreateInfo, unsafe.Pointer, *uintptr) int32
	destroySwapchain                  func(uintptr, uintptr, unsafe.Pointer)
	getSwapchainImages                func(uintptr, uintptr, *uint32, *uintptr) int32
	createImageView                   func(uintptr, *vkImageViewCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyImageView                  func(uintptr, uintptr, unsafe.Pointer)
	createRenderPass                  func(uintptr, *vkRenderPassCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyRenderPass                 func(uintptr, uintptr, unsafe.Pointer)
	createFramebuffer                 func(uintptr, *vkFramebufferCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyFramebuffer                func(uintptr, uintptr, unsafe.Pointer)
	createCommandPool                 func(uintptr, *vkCommandPoolCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyCommandPool                func(uintptr, uintptr, unsafe.Pointer)
	resetCommandPool                  func(uintptr, uintptr, uint32) int32
	allocateCommandBuffers            func(uintptr, *vkCommandBufferAllocateInfo, *uintptr) int32
	beginCommandBuffer                func(uintptr, *vkCommandBufferBeginInfo) int32
	endCommandBuffer                  func(uintptr) int32
	cmdBeginRenderPass                func(uintptr, *vkRenderPassBeginInfo, uint32)
	cmdClearAttachments               func(uintptr, uint32, *vkClearAttachment, uint32, *vkClearRect)
	cmdEndRenderPass                  func(uintptr)
	createSemaphore                   func(uintptr, *vkSemaphoreCreateInfo, unsafe.Pointer, *uintptr) int32
	destroySemaphore                  func(uintptr, uintptr, unsafe.Pointer)
	createFence                       func(uintptr, *vkFenceCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyFence                      func(uintptr, uintptr, unsafe.Pointer)
	waitForFences                     func(uintptr, uint32, *uintptr, uint32, uint64) int32
	resetFences                       func(uintptr, uint32, *uintptr) int32
	acquireNextImage                  func(uintptr, uintptr, uint64, uintptr, uintptr, *uint32) int32
	queueSubmit                       func(uintptr, uint32, *vkSubmitInfo, uintptr) int32
	queuePresent                      func(uintptr, *vkPresentInfo) int32
	deviceWaitIdle                    func(uintptr) int32
	getPhysicalDeviceMemoryProperties func(uintptr, *vkPhysicalDeviceMemoryProperties)
	createBuffer                      func(uintptr, *vkBufferCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyBuffer                     func(uintptr, uintptr, unsafe.Pointer)
	getBufferMemoryRequirements       func(uintptr, uintptr, *vkMemoryRequirements)
	allocateMemory                    func(uintptr, *vkMemoryAllocateInfo, unsafe.Pointer, *uintptr) int32
	freeMemory                        func(uintptr, uintptr, unsafe.Pointer)
	bindBufferMemory                  func(uintptr, uintptr, uintptr, uint64) int32
	mapMemory                         func(uintptr, uintptr, uint64, uint64, uint32, *unsafe.Pointer) int32
	unmapMemory                       func(uintptr, uintptr)
	createShaderModule                func(uintptr, *vkShaderModuleCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyShaderModule               func(uintptr, uintptr, unsafe.Pointer)
	createPipelineLayout              func(uintptr, *vkPipelineLayoutCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyPipelineLayout             func(uintptr, uintptr, unsafe.Pointer)
	createGraphicsPipelines           func(uintptr, uintptr, uint32, *vkGraphicsPipelineCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyPipeline                   func(uintptr, uintptr, unsafe.Pointer)
	cmdBindPipeline                   func(uintptr, uint32, uintptr)
	cmdSetViewport                    func(uintptr, uint32, uint32, *vkViewport)
	cmdSetScissor                     func(uintptr, uint32, uint32, *vkRect2D)
	cmdPushConstants                  func(uintptr, uintptr, uint32, uint32, uint32, unsafe.Pointer)
	cmdBindVertexBuffers              func(uintptr, uint32, uint32, *uintptr, *uint64)
	cmdDraw                           func(uintptr, uint32, uint32, uint32, uint32)
	createImage                       func(uintptr, *vkImageCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyImage                      func(uintptr, uintptr, unsafe.Pointer)
	getImageMemoryRequirements        func(uintptr, uintptr, *vkMemoryRequirements)
	bindImageMemory                   func(uintptr, uintptr, uintptr, uint64) int32
	createSampler                     func(uintptr, *vkSamplerCreateInfo, unsafe.Pointer, *uintptr) int32
	destroySampler                    func(uintptr, uintptr, unsafe.Pointer)
	createDescriptorSetLayout         func(uintptr, *vkDescriptorSetLayoutCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyDescriptorSetLayout        func(uintptr, uintptr, unsafe.Pointer)
	createDescriptorPool              func(uintptr, *vkDescriptorPoolCreateInfo, unsafe.Pointer, *uintptr) int32
	destroyDescriptorPool             func(uintptr, uintptr, unsafe.Pointer)
	allocateDescriptorSets            func(uintptr, *vkDescriptorSetAllocateInfo, *uintptr) int32
	updateDescriptorSets              func(uintptr, uint32, *vkWriteDescriptorSet, uint32, unsafe.Pointer)
	cmdPipelineBarrier                func(uintptr, uint32, uint32, uint32, uint32, unsafe.Pointer, uint32, unsafe.Pointer, uint32, *vkImageMemoryBarrier)
	cmdCopyBufferToImage              func(uintptr, uintptr, uintptr, uint32, uint32, *vkBufferImageCopy)
	cmdBindDescriptorSets             func(uintptr, uint32, uintptr, uint32, uint32, *uintptr, uint32, *uint32)
	cmdCopyImageToBuffer              func(uintptr, uintptr, uint32, uintptr, uint32, *vkBufferImageCopy)
}

type vulkanRenderer struct {
	api                                        *vulkanAPI
	instance, surface, physical, device, queue uintptr
	swapchain, renderPass, commandPool         uintptr
	uploadCommand                              uintptr
	imageAvailable, renderFinished, fence      uintptr
	views, framebuffers, commands              []uintptr
	queueFamily, width, height, format         uint32
	// dualSource is set when the device was created with dual-source blending, which
	// blends LCD text per channel.
	dualSource      bool
	enabledFeatures [vkFeatureCount]uint32
	// scale is device pixels per logical pixel: width and height are device
	// pixels, a scene is in logical pixels. Zero means 1.
	scale                           float32
	color                           platform.Color
	rectangles                      []render.Rectangle
	scene                           vulkanScene
	text                            vulkanText
	rectangleBatches                []render.Batch
	compositingBatches              []compositingBatch
	offscreen                       vulkanOffscreen
	submissions, rebuilds, partials uint64
	scratch                         vulkanScratch
	// provider is what the surface is made from, kept to make it
	// again after a loss; provider is nil for an offscreen renderer, which has
	// no surface and is not rebuilt. recoveries counts the rebuilds.
	provider            vulkanSurfaceProvider
	swapchainSuboptimal bool
	// swapchainSuboptimalRebuilt means the current swapchain was recreated
	// after SUBOPTIMAL was reported and still reports it. Further frames
	// present it instead of recreating it again until a frame reports a
	// healthy status or the swapchain is recreated for another reason.
	swapchainSuboptimalRebuilt bool
	connection                 uintptr
	window                     uint32
	recoveries                 uint64
	// frame is the window frame the renderer draws, if any (vulkan_frame_linux.go).
	frame vulkanFrame
}

// vulkanScratch holds the structs a repaint passes to Vulkan by address. A pointer handed to
// a function value escapes, so a local would be a heap allocation per call; Vulkan copies
// these during the call, so one set in the renderer serves every call. A renderer is used
// from one thread, and the commands that share a field never nest.
type vulkanScratch struct {
	clear       vkClearValue
	begin       vkCommandBufferBeginInfo
	renderBegin vkRenderPassBeginInfo
	viewport    vkViewport
	scissor     vkRect2D
	cut         vkRect2D
	offset      uint64
	sceneFrame  [6]float32
	textFrame   [8]float32
	tint        [4]float32
	submit      vkSubmitInfo
	present     vkPresentInfo
	waitStage   uint32
	image       uint32
	command     uintptr
	// contentX and contentY are where the scene's rectangle starts in the target:
	// zero, unless a window frame leaves a margin.
	contentX, contentY int32
}

func loadVulkan() (*vulkanAPI, error) {
	library, err := purego.Dlopen("libvulkan.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, noVulkan("libvulkan.so.1 is not installed", err)
	}
	api := &vulkanAPI{library: library}
	purego.RegisterLibFunc(&api.createInstance, library, "vkCreateInstance")
	purego.RegisterLibFunc(&api.destroyInstance, library, "vkDestroyInstance")
	purego.RegisterLibFunc(&api.enumeratePhysicalDevices, library, "vkEnumeratePhysicalDevices")
	purego.RegisterLibFunc(&api.getQueueFamilyProperties, library, "vkGetPhysicalDeviceQueueFamilyProperties")
	purego.RegisterLibFunc(&api.getPhysicalDeviceFeatures, library, "vkGetPhysicalDeviceFeatures")
	purego.RegisterLibFunc(&api.createXcbSurface, library, "vkCreateXcbSurfaceKHR")
	purego.RegisterLibFunc(&api.destroySurface, library, "vkDestroySurfaceKHR")
	purego.RegisterLibFunc(&api.getSurfaceSupport, library, "vkGetPhysicalDeviceSurfaceSupportKHR")
	purego.RegisterLibFunc(&api.getSurfaceCapabilities, library, "vkGetPhysicalDeviceSurfaceCapabilitiesKHR")
	purego.RegisterLibFunc(&api.getSurfaceFormats, library, "vkGetPhysicalDeviceSurfaceFormatsKHR")
	purego.RegisterLibFunc(&api.getSurfacePresentModes, library, "vkGetPhysicalDeviceSurfacePresentModesKHR")
	purego.RegisterLibFunc(&api.createDevice, library, "vkCreateDevice")
	purego.RegisterLibFunc(&api.destroyDevice, library, "vkDestroyDevice")
	purego.RegisterLibFunc(&api.getDeviceQueue, library, "vkGetDeviceQueue")
	purego.RegisterLibFunc(&api.createSwapchain, library, "vkCreateSwapchainKHR")
	purego.RegisterLibFunc(&api.destroySwapchain, library, "vkDestroySwapchainKHR")
	purego.RegisterLibFunc(&api.getSwapchainImages, library, "vkGetSwapchainImagesKHR")
	purego.RegisterLibFunc(&api.createImageView, library, "vkCreateImageView")
	purego.RegisterLibFunc(&api.destroyImageView, library, "vkDestroyImageView")
	purego.RegisterLibFunc(&api.createRenderPass, library, "vkCreateRenderPass")
	purego.RegisterLibFunc(&api.destroyRenderPass, library, "vkDestroyRenderPass")
	purego.RegisterLibFunc(&api.createFramebuffer, library, "vkCreateFramebuffer")
	purego.RegisterLibFunc(&api.destroyFramebuffer, library, "vkDestroyFramebuffer")
	purego.RegisterLibFunc(&api.createCommandPool, library, "vkCreateCommandPool")
	purego.RegisterLibFunc(&api.destroyCommandPool, library, "vkDestroyCommandPool")
	purego.RegisterLibFunc(&api.allocateCommandBuffers, library, "vkAllocateCommandBuffers")
	purego.RegisterLibFunc(&api.cmdClearAttachments, library, "vkCmdClearAttachments")
	purego.RegisterLibFunc(&api.createSemaphore, library, "vkCreateSemaphore")
	purego.RegisterLibFunc(&api.destroySemaphore, library, "vkDestroySemaphore")
	purego.RegisterLibFunc(&api.createFence, library, "vkCreateFence")
	purego.RegisterLibFunc(&api.destroyFence, library, "vkDestroyFence")
	purego.RegisterLibFunc(&api.deviceWaitIdle, library, "vkDeviceWaitIdle")
	purego.RegisterLibFunc(&api.cmdCopyImageToBuffer, library, "vkCmdCopyImageToBuffer")
	registerVulkanScene(api, library)
	registerVulkanText(api, library)
	bindVulkanHotPath(api, library)
	return api, nil
}

func newVulkanRenderer(provider vulkanSurfaceProvider, width, height uint32, scale float32, color platform.Color, rectangles []render.Rectangle, labels []platform.TextLabel, margin uint32) (*vulkanRenderer, error) {
	api, err := loadVulkan()
	if err != nil {
		return nil, err
	}
	renderer := &vulkanRenderer{api: api, width: width, height: height, scale: scale, color: color, provider: provider}
	if xcb, ok := provider.(*xcbSurfaceProvider); ok {
		renderer.connection, renderer.window = xcb.connection, xcb.window
	}
	renderer.frame.margin = margin
	renderer.rectangles = append(renderer.rectangles, rectangles...)
	renderer.rectangleBatches = render.GroupBatches(renderer.rectangleBatches, renderer.rectangles)
	if err := renderer.text.update(nil, labels); err != nil {
		renderer.close()
		return nil, err
	}
	renderer.compositingBatches = groupCompositingLayers(renderer.rectangleBatches, renderer.text.labels, renderer.text.order, renderer.compositingBatches)
	if err := renderer.initialize(provider); err != nil {
		renderer.close()
		return nil, err
	}
	return renderer, nil
}

func (renderer *vulkanRenderer) initialize(provider vulkanSurfaceProvider) error {
	extensions := provider.RequiredExtensions()
	if err := renderer.createInstance(extensions); err != nil {
		return err
	}
	surface, err := provider.CreateSurface(renderer.api, renderer.instance)
	if err != nil {
		return err
	}
	renderer.surface = surface
	if err := renderer.selectDevice(); err != nil {
		return err
	}
	swapExtension := append([]byte("VK_KHR_swapchain"), 0)
	if err := renderer.createDevice([]*byte{&swapExtension[0]}); err != nil {
		return err
	}
	if len(renderer.text.labels) > 0 {
		if err := renderer.text.initialize(renderer); err != nil {
			return err
		}
	}
	if err := renderer.scene.upload(renderer, renderer.rectangles, render.Update{Count: len(renderer.rectangles), Rebuild: true}); err != nil {
		return err
	}
	if err := renderer.createSwapchain(); err != nil {
		return err
	}
	semaphoreInfo := vkSemaphoreCreateInfo{SType: vkStructureSemaphoreCreateInfo}
	if err := vkCheck(renderer.api.createSemaphore(renderer.device, &semaphoreInfo, nil, &renderer.imageAvailable), "create acquire semaphore"); err != nil {
		return err
	}
	if err := vkCheck(renderer.api.createSemaphore(renderer.device, &semaphoreInfo, nil, &renderer.renderFinished), "create render semaphore"); err != nil {
		return err
	}
	fenceInfo := vkFenceCreateInfo{SType: vkStructureFenceCreateInfo, Flags: vkFenceSignaled}
	return vkCheck(renderer.api.createFence(renderer.device, &fenceInfo, nil, &renderer.fence), "create frame fence")
}

// createInstance creates the Vulkan instance with the given extensions: the
// window path needs the surface ones, an offscreen renderer none.
func (renderer *vulkanRenderer) createInstance(extensions []*byte) error {
	applicationName := append([]byte("Viva"), 0)
	engineName := append([]byte("Viva"), 0)
	application := vkApplicationInfo{SType: vkStructureApplicationInfo, Application: &applicationName[0], Engine: &engineName[0], APIVersion: 1 << 22}
	instanceInfo := vkInstanceCreateInfo{SType: vkStructureInstanceCreateInfo, Application: &application, ExtensionCount: uint32(len(extensions))}
	if len(extensions) > 0 {
		instanceInfo.Extensions = uintptr(unsafe.Pointer(&extensions[0]))
	}
	switch status := renderer.api.createInstance(&instanceInfo, nil, &renderer.instance); status {
	case vkSuccess:
		return nil
	case vkErrorIncompatibleDriver, vkErrorInitializationFailed:
		return noVulkan(fmt.Sprintf("Vulkan has no driver (result %d)", status), nil)
	default:
		return vkCheck(status, "create instance")
	}
}

// createDevice creates the logical device and its graphics queue on the
// selected physical device and queue family.
func (renderer *vulkanRenderer) createDevice(extensions []*byte) error {
	priority := float32(1)
	queueInfo := vkDeviceQueueCreateInfo{SType: vkStructureDeviceQueueCreateInfo, Family: renderer.queueFamily, Count: 1, Priorities: &priority}
	deviceInfo := vkDeviceCreateInfo{SType: vkStructureDeviceCreateInfo, QueueCount: 1, Queues: &queueInfo, ExtensionCount: uint32(len(extensions))}
	// Dual-source blending (a core feature of nearly every desktop device) blends the
	// per-channel coverage of LCD text; without it text is blended in grayscale.
	var supported [vkFeatureCount]uint32
	renderer.api.getPhysicalDeviceFeatures(renderer.physical, &supported)
	if supported[vkFeatureDualSourceBlend] != 0 {
		renderer.enabledFeatures[vkFeatureDualSourceBlend] = 1
		deviceInfo.Features = uintptr(unsafe.Pointer(&renderer.enabledFeatures))
		renderer.dualSource = true
	}
	if len(extensions) > 0 {
		deviceInfo.Extensions = uintptr(unsafe.Pointer(&extensions[0]))
	}
	if err := vkCheck(renderer.api.createDevice(renderer.physical, &deviceInfo, nil, &renderer.device), "create device"); err != nil {
		renderer.dualSource = false
		return err
	}
	renderer.api.getDeviceQueue(renderer.device, renderer.queueFamily, 0, &renderer.queue)
	if renderer.queue == 0 {
		return fmt.Errorf("linux: Vulkan returned no graphics queue")
	}
	return nil
}

func (renderer *vulkanRenderer) selectDevice() error {
	var deviceCount uint32
	if err := vkCheck(renderer.api.enumeratePhysicalDevices(renderer.instance, &deviceCount, nil), "count physical devices"); err != nil {
		return err
	}
	if deviceCount == 0 {
		return noVulkan("Vulkan reports no device", nil)
	}
	if deviceCount > 32 {
		return fmt.Errorf("linux: Vulkan physical device count %d", deviceCount)
	}
	devices := make([]uintptr, deviceCount)
	if err := vkCheck(renderer.api.enumeratePhysicalDevices(renderer.instance, &deviceCount, &devices[0]), "enumerate physical devices"); err != nil {
		return err
	}
	for _, device := range devices[:deviceCount] {
		var familyCount uint32
		renderer.api.getQueueFamilyProperties(device, &familyCount, nil)
		if familyCount == 0 || familyCount > 64 {
			continue
		}
		families := make([]vkQueueFamilyProperties, familyCount)
		renderer.api.getQueueFamilyProperties(device, &familyCount, &families[0])
		for index, family := range families[:familyCount] {
			if family.Flags&vkQueueGraphics == 0 || family.Count == 0 {
				continue
			}
			var supported uint32
			if renderer.surface == 0 || renderer.api.getSurfaceSupport(device, uint32(index), renderer.surface, &supported) == vkSuccess && supported != 0 {
				renderer.physical = device
				renderer.queueFamily = uint32(index)
				return nil
			}
		}
	}
	return fmt.Errorf("linux: no Vulkan graphics queue can present to XCB")
}

func (renderer *vulkanRenderer) choosePresentMode() uint32 {
	if renderer.api == nil || renderer.api.getSurfacePresentModes == nil || renderer.physical == 0 || renderer.surface == 0 {
		return vkPresentFIFO
	}
	var count uint32
	if err := vkCheck(renderer.api.getSurfacePresentModes(renderer.physical, renderer.surface, &count, nil), "count surface present modes"); err != nil || count == 0 {
		return vkPresentFIFO
	}
	modes := make([]uint32, count)
	if err := vkCheck(renderer.api.getSurfacePresentModes(renderer.physical, renderer.surface, &count, &modes[0]), "query surface present modes"); err != nil {
		return vkPresentFIFO
	}
	for _, mode := range modes {
		if mode == vkPresentMailbox {
			return vkPresentMailbox
		}
	}
	return vkPresentFIFO
}

func (renderer *vulkanRenderer) createSwapchain() error {
	return renderer.createSwapchainWithOld(0)
}

func (renderer *vulkanRenderer) createSwapchainWithOld(oldSwapchain uintptr) (err error) {
	defer func() {
		if err != nil {
			if oldSwapchain != 0 {
				renderer.api.destroySwapchain(renderer.device, oldSwapchain, nil)
			}
			renderer.destroySwapchain()
		}
	}()
	var capabilities vkSurfaceCapabilities
	if err := vkCheck(renderer.api.getSurfaceCapabilities(renderer.physical, renderer.surface, &capabilities), "query surface capabilities"); err != nil {
		return err
	}
	var formatCount uint32
	if err := vkCheck(renderer.api.getSurfaceFormats(renderer.physical, renderer.surface, &formatCount, nil), "count surface formats"); err != nil {
		return err
	}
	if formatCount == 0 || formatCount > 64 {
		return fmt.Errorf("linux: Vulkan surface format count %d", formatCount)
	}
	formats := make([]vkSurfaceFormat, formatCount)
	if err := vkCheck(renderer.api.getSurfaceFormats(renderer.physical, renderer.surface, &formatCount, &formats[0]), "query surface formats"); err != nil {
		return err
	}
	selected := formats[0]
	if formatCount == 1 && selected.Format == 0 {
		selected = vkSurfaceFormat{Format: vkFormatBGRA8, ColorSpace: vkColorSpaceSRGB}
	} else {
		for _, candidate := range formats[:formatCount] {
			if candidate.Format == vkFormatBGRA8 && candidate.ColorSpace == vkColorSpaceSRGB {
				selected = candidate
				break
			}
		}
	}
	extent := capabilities.Current
	minW := max(1, capabilities.Minimum.Width)
	minH := max(1, capabilities.Minimum.Height)
	maxW := max(minW, capabilities.Maximum.Width)
	maxH := max(minH, capabilities.Maximum.Height)
	if extent.Width == math.MaxUint32 {
		extent.Width = clamp(renderer.width, minW, maxW)
		extent.Height = clamp(renderer.height, minH, maxH)
	} else {
		extent.Width = clamp(extent.Width, minW, maxW)
		extent.Height = clamp(extent.Height, minH, maxH)
	}
	presentMode := renderer.choosePresentMode()
	imageCount := capabilities.MinImages + 1
	if presentMode == vkPresentMailbox && imageCount < 3 {
		imageCount = 3
	}
	if capabilities.MaxImages > 0 && imageCount > capabilities.MaxImages {
		imageCount = capabilities.MaxImages
	}
	composite := uint32(vkCompositeOpaque)
	renderer.frame.alpha = false
	if renderer.frame.active() && capabilities.CompositeAlpha&vkCompositePreMultiplied != 0 {
		// The margin of a window frame is transparent: the scene draws
		// premultiplied colors.
		composite, renderer.frame.alpha = vkCompositePreMultiplied, true
	} else if capabilities.CompositeAlpha&composite == 0 {
		composite = capabilities.CompositeAlpha & -capabilities.CompositeAlpha
	}
	if composite == 0 {
		return fmt.Errorf("linux: Vulkan surface supports no composite alpha mode")
	}
	if capabilities.Usage&vkImageUsageColorAttachment == 0 {
		return fmt.Errorf("linux: Vulkan surface does not support color attachments")
	}
	info := vkSwapchainCreateInfo{SType: vkStructureSwapchainCreateInfo, Surface: renderer.surface, MinImages: imageCount, Format: selected.Format, ColorSpace: selected.ColorSpace, Extent: extent, Layers: 1, Usage: vkImageUsageColorAttachment, SharingMode: vkSharingExclusive, PreTransform: capabilities.CurrentTransform, CompositeAlpha: composite, PresentMode: presentMode, Clipped: 1, OldSwapchain: oldSwapchain}
	var newSwapchain uintptr
	if err := vkCheck(renderer.api.createSwapchain(renderer.device, &info, nil, &newSwapchain), "create swapchain"); err != nil {
		return err
	}
	if oldSwapchain != 0 {
		renderer.api.destroySwapchain(renderer.device, oldSwapchain, nil)
		oldSwapchain = 0
	}
	renderer.swapchain = newSwapchain
	var imageTotal uint32
	if err := vkCheck(renderer.api.getSwapchainImages(renderer.device, renderer.swapchain, &imageTotal, nil), "count swapchain images"); err != nil {
		return err
	}
	if imageTotal == 0 || imageTotal > 16 {
		return fmt.Errorf("linux: Vulkan swapchain image count %d", imageTotal)
	}
	images := make([]uintptr, imageTotal)
	if err := vkCheck(renderer.api.getSwapchainImages(renderer.device, renderer.swapchain, &imageTotal, &images[0]), "query swapchain images"); err != nil {
		return err
	}
	renderer.views = make([]uintptr, imageTotal)
	for index, image := range images[:imageTotal] {
		viewInfo := vkImageViewCreateInfo{SType: vkStructureImageViewCreateInfo, Image: image, ViewType: vkImageViewType2D, Format: selected.Format, Range: vkSubresourceRange{Aspect: vkImageAspectColor, LevelCount: 1, LayerCount: 1}}
		if err := vkCheck(renderer.api.createImageView(renderer.device, &viewInfo, nil, &renderer.views[index]), "create swapchain image view"); err != nil {
			return err
		}
	}
	if err := renderer.createRenderPass(selected.Format, vkImageLayoutPresent); err != nil {
		return err
	}
	renderer.framebuffers = make([]uintptr, imageTotal)
	for index := range renderer.views {
		view := renderer.views[index]
		framebufferInfo := vkFramebufferCreateInfo{SType: vkStructureFramebufferCreateInfo, RenderPass: renderer.renderPass, AttachmentCount: 1, Attachments: &view, Width: extent.Width, Height: extent.Height, Layers: 1}
		if err := vkCheck(renderer.api.createFramebuffer(renderer.device, &framebufferInfo, nil, &renderer.framebuffers[index]), "create swapchain framebuffer"); err != nil {
			return err
		}
	}
	if err := renderer.createCommands(imageTotal); err != nil {
		return err
	}
	if err := renderer.createScenePipelines(extent); err != nil {
		return err
	}
	renderer.width, renderer.height, renderer.format = extent.Width, extent.Height, selected.Format
	renderer.swapchainSuboptimal = false
	renderer.swapchainSuboptimalRebuilt = false
	return nil
}

// createRenderPass creates the single-subpass render pass that clears one color
// attachment of the given format and leaves it in finalLayout.
func (renderer *vulkanRenderer) createRenderPass(format, finalLayout uint32) error {
	attachment := vkAttachmentDescription{Format: format, Samples: vkSampleCountOne, Load: vkAttachmentLoadClear, Store: vkAttachmentStore, StencilLoad: vkAttachmentLoadDontCare, StencilStore: vkAttachmentStoreDontCare, InitialLayout: vkImageLayoutUndefined, FinalLayout: finalLayout}
	reference := vkAttachmentReference{Layout: vkImageLayoutColorAttachment}
	subpass := vkSubpassDescription{BindPoint: vkPipelineBindPointGraphics, ColorCount: 1, Colors: uintptr(unsafe.Pointer(&reference))}
	renderPassInfo := vkRenderPassCreateInfo{SType: vkStructureRenderPassCreateInfo, AttachmentCount: 1, Attachments: &attachment, SubpassCount: 1, Subpasses: &subpass}
	return vkCheck(renderer.api.createRenderPass(renderer.device, &renderPassInfo, nil, &renderer.renderPass), "create clear render pass")
}

// createCommands creates the command pool and one command buffer per target
// image, plus the one the text uploads use.
func (renderer *vulkanRenderer) createCommands(images uint32) error {
	poolInfo := vkCommandPoolCreateInfo{SType: vkStructureCommandPoolCreateInfo, Family: renderer.queueFamily}
	if err := vkCheck(renderer.api.createCommandPool(renderer.device, &poolInfo, nil, &renderer.commandPool), "create command pool"); err != nil {
		return err
	}
	allocatedCommands := make([]uintptr, images+1)
	allocate := vkCommandBufferAllocateInfo{SType: vkStructureCommandAllocateInfo, Pool: renderer.commandPool, Level: vkCommandBufferPrimary, Count: images + 1}
	if err := vkCheck(renderer.api.allocateCommandBuffers(renderer.device, &allocate, &allocatedCommands[0]), "allocate rendering commands"); err != nil {
		return err
	}
	renderer.commands = allocatedCommands[:images]
	renderer.uploadCommand = allocatedCommands[images]
	return nil
}

// createScenePipelines builds the rectangle and text pipelines for the render
// pass, uploads the label textures and records the commands of every target
// image: everything a window and an offscreen target share after their images
// exist.
func (renderer *vulkanRenderer) createScenePipelines(extent vkExtent2D) error {
	if err := renderer.scene.createPipeline(renderer); err != nil {
		return err
	}
	if err := renderer.text.createPipeline(renderer); err != nil {
		return err
	}
	if renderer.frame.active() {
		if err := renderer.frame.create(renderer); err != nil {
			return err
		}
	}
	if err := renderer.text.upload(renderer, renderer.pixelScale()); err != nil {
		return err
	}
	return renderer.recordCommands(extent)
}

func (renderer *vulkanRenderer) recordCommands(extent vkExtent2D) error {
	scratch := &renderer.scratch
	scratch.clear = vkClearValue{Color: [4]float32{float32(renderer.color.Red), float32(renderer.color.Green), float32(renderer.color.Blue), float32(renderer.color.Alpha)}}
	// A window frame leaves the margin transparent and draws the scene inside it.
	content, contentX, contentY := extent, int32(0), int32(0)
	background := scratch.clear
	if renderer.frame.active() {
		contentX, contentY, content = renderer.frame.content(extent.Width, extent.Height)
		scratch.clear = vkClearValue{}
		renderer.frame.upload(renderer, extent.Width, extent.Height)
	}
	scratch.contentX, scratch.contentY = contentX, contentY
	for index, command := range renderer.commands {
		scratch.begin = vkCommandBufferBeginInfo{SType: vkStructureCommandBeginInfo}
		if err := vkCheck(renderer.api.beginCommandBuffer(command, &scratch.begin), "begin clear command"); err != nil {
			return err
		}
		scratch.renderBegin = vkRenderPassBeginInfo{SType: vkStructureRenderPassBeginInfo, RenderPass: renderer.renderPass, Framebuffer: renderer.framebuffers[index], Area: vkRect2D{Extent: extent}, ClearCount: 1, Clear: &scratch.clear}
		renderer.api.cmdBeginRenderPass(command, &scratch.renderBegin, vkSubpassContentsInline)
		if renderer.frame.active() {
			attachment := vkClearAttachment{AspectMask: vkImageAspectColor, ClearValue: background}
			area := vkClearRect{Rect: vkRect2D{Offset: [2]int32{contentX, contentY}, Extent: content}, LayerCount: 1}
			renderer.api.cmdClearAttachments(command, 1, &attachment, 1, &area)
		}
		for _, batch := range renderer.compositingBatches {
			switch batch.Kind {
			case compositeRectangles:
				renderer.scene.record(renderer, command, content, batch)
			case compositeText:
				for offset := batch.First; offset < batch.First+batch.Count; offset++ {
					renderer.text.record(renderer, command, content, renderer.text.order[offset])
				}
			}
		}
		if renderer.frame.active() {
			renderer.frame.record(renderer, command, extent)
		}
		renderer.api.cmdEndRenderPass(command)
		if err := vkCheck(renderer.api.endCommandBuffer(command), "end clear command"); err != nil {
			return err
		}
	}
	scratch.contentX, scratch.contentY = 0, 0
	return nil
}

func validateRectangles(rectangles []render.Rectangle) error {
	for index, rectangle := range rectangles {
		bounds := rectangle.Bounds
		if !finite32(bounds.X) || !finite32(bounds.Y) || !finite32(bounds.Width) || !finite32(bounds.Height) {
			return fmt.Errorf("linux: rectangle %d has non-finite bounds", index)
		}
		if !finite32(rectangle.Radius) || rectangle.Radius < 0 {
			return fmt.Errorf("linux: rectangle %d has invalid radius %g", index, rectangle.Radius)
		}
		if !finite32(rectangle.Color.Red) || !finite32(rectangle.Color.Green) || !finite32(rectangle.Color.Blue) || !finite32(rectangle.Color.Alpha) {
			return fmt.Errorf("linux: rectangle %d has a non-finite color", index)
		}
		if rectangle.Kind == scene.KindShadow {
			shadow := rectangle.Shadow
			if !finite32(shadow.OffsetX) || !finite32(shadow.OffsetY) || !finite32(shadow.Blur) || !finite32(shadow.Spread) || shadow.Blur < 0 {
				return fmt.Errorf("linux: shadow %d has an invalid blur or offset", index)
			}
		}
		if rectangle.Kind == scene.KindTriangle {
			for pointIndex, point := range rectangle.Points {
				if !finite32(point.X) || !finite32(point.Y) {
					return fmt.Errorf("linux: triangle %d point %d is non-finite", index, pointIndex)
				}
			}
			if rectangle.EdgeMask > 7 {
				return fmt.Errorf("linux: triangle %d has invalid edge mask %d", index, rectangle.EdgeMask)
			}
		}
	}
	return nil
}

func rectangleClear(rectangle render.Rectangle, width, height uint32) (vkClearAttachment, vkClearRect, bool) {
	bounds := rectangle.Bounds
	left := max(float64(bounds.X), 0)
	top := max(float64(bounds.Y), 0)
	right := min(float64(bounds.X)+float64(bounds.Width), float64(width))
	bottom := min(float64(bounds.Y)+float64(bounds.Height), float64(height))
	if right <= left || bottom <= top {
		return vkClearAttachment{}, vkClearRect{}, false
	}
	left = math.Floor(left)
	top = math.Floor(top)
	right = math.Ceil(right)
	bottom = math.Ceil(bottom)
	attachment := vkClearAttachment{
		AspectMask: vkImageAspectColor,
		ClearValue: vkClearValue{Color: [4]float32{
			clampUnit(rectangle.Color.Red),
			clampUnit(rectangle.Color.Green),
			clampUnit(rectangle.Color.Blue),
			1,
		}},
	}
	clear := vkClearRect{
		Rect: vkRect2D{
			Offset: [2]int32{int32(left), int32(top)},
			Extent: vkExtent2D{Width: uint32(right - left), Height: uint32(bottom - top)},
		},
		LayerCount: 1,
	}
	return attachment, clear, true
}

func finite32(value float32) bool {
	converted := float64(value)
	return !math.IsNaN(converted) && !math.IsInf(converted, 0)
}

func clampUnit(value float32) float32 {
	return min(max(value, 0), 1)
}

func swapchainNeedsRebuild(status int32) bool {
	return status == vkErrorOutOfDate
}

// present draws the scene to the next swapchain image. A lost device or
// surface is rebuilt and the frame drawn again; only a device that keeps
// failing ends the window.
func (renderer *vulkanRenderer) present() error {
	for losses := 0; ; losses++ {
		err := renderer.presentRebuilding()
		retry, err := renderer.recoverFrom(err, losses)
		if !retry {
			return err
		}
	}
}

// recoverFrom rebuilds the renderer when err says the device or the surface
// was lost, and reports whether the operation that failed is to be run again.
// A rebuild that fails gives that failure with err.
func (renderer *vulkanRenderer) recoverFrom(err error, losses int) (retry bool, failure error) {
	if err == nil || !isGraphicsLoss(err) || (renderer.provider == nil && renderer.connection == 0) || losses >= maxLossRecoveries {
		return false, err
	}
	if recoverErr := renderer.recoverDevice(); recoverErr != nil {
		return false, errors.Join(err, recoverErr)
	}
	return true, nil
}

func (renderer *vulkanRenderer) presentRebuilding() error {
	for attempt := 0; attempt < 2; attempt++ {
		rebuild, err := renderer.presentOnce()
		if err != nil {
			return err
		}
		if !rebuild {
			if !renderer.swapchainSuboptimal {
				// A healthy frame ends any SUBOPTIMAL episode; a later
				// SUBOPTIMAL is a fresh signal that may rebuild once.
				renderer.swapchainSuboptimalRebuilt = false
				return nil
			}
			if renderer.swapchainSuboptimalRebuilt || attempt == 1 {
				// SUBOPTIMAL presents the frame. Once the swapchain has been
				// recreated for it, recreating again cannot make the status
				// converge; present this swapchain until the status clears.
				renderer.swapchainSuboptimalRebuilt = true
				return nil
			}
		}
		if attempt == 1 {
			break
		}
		// OUT_OF_DATE has not completed a frame. SUBOPTIMAL has, but its new
		// swapchain must also present once before this transaction completes.
		if err := renderer.recreateSwapchain(); err != nil {
			return err
		}
	}
	return fmt.Errorf("linux: Vulkan swapchain remained out of date after recreation")
}

func (renderer *vulkanRenderer) presentOnce() (bool, error) {
	if err := vkCheck(renderer.api.waitForFences(renderer.device, 1, &renderer.fence, 1, math.MaxUint64), "wait for frame fence"); err != nil {
		return false, err
	}
	scratch := &renderer.scratch
	status := renderer.api.acquireNextImage(renderer.device, renderer.swapchain, math.MaxUint64, renderer.imageAvailable, 0, &scratch.image)
	if status == vkErrorOutOfDate {
		return true, nil
	}
	if status != vkSuccess && status != vkSuboptimal {
		return false, vkCheck(status, "acquire swapchain image")
	}
	suboptimal := status == vkSuboptimal
	if suboptimal {
		renderer.swapchainSuboptimal = true
	}
	image := scratch.image
	if image >= uint32(len(renderer.commands)) {
		return false, fmt.Errorf("linux: Vulkan acquired invalid image %d", image)
	}
	if err := vkCheck(renderer.api.resetFences(renderer.device, 1, &renderer.fence), "reset frame fence"); err != nil {
		return false, err
	}
	scratch.waitStage = vkPipelineStageColorOutput
	scratch.command = renderer.commands[image]
	scratch.submit = vkSubmitInfo{SType: vkStructureSubmitInfo, WaitCount: 1, Waits: &renderer.imageAvailable, WaitStages: &scratch.waitStage, CommandCount: 1, Commands: &scratch.command, SignalCount: 1, Signals: &renderer.renderFinished}
	if err := vkCheck(renderer.api.queueSubmit(renderer.queue, 1, &scratch.submit, renderer.fence), "submit clear command"); err != nil {
		return false, err
	}
	scratch.present = vkPresentInfo{SType: vkStructurePresentInfo, WaitCount: 1, Waits: &renderer.renderFinished, SwapCount: 1, Swapchains: &renderer.swapchain, Indices: &scratch.image}
	status = renderer.api.queuePresent(renderer.queue, &scratch.present)
	if status == vkSuboptimal {
		renderer.swapchainSuboptimal = true
		suboptimal = true
	}
	if status != vkSuccess && !swapchainNeedsRebuild(status) && status != vkSuboptimal {
		return false, vkCheck(status, "present swapchain image")
	}
	if !suboptimal {
		// The swapchain presented at its preferred configuration. A later
		// SUBOPTIMAL is new information and may rebuild it once.
		renderer.swapchainSuboptimal = false
	}
	renderer.submissions++
	return swapchainNeedsRebuild(status), nil
}

// apply takes a frame into the renderer's scene. A frame is idempotent, so one
// that met a lost device is applied again to the rebuilt renderer.
func (renderer *vulkanRenderer) apply(frame platform.Frame) (bool, error) {
	for losses := 0; ; losses++ {
		changed, err := renderer.applyFrame(frame)
		retry, err := renderer.recoverFrom(err, losses)
		if !retry {
			return changed, err
		}
	}
}

func (renderer *vulkanRenderer) applyFrame(frame platform.Frame) (bool, error) {
	if frame.Update.Empty() && frame.TextLabels == nil {
		return false, nil
	}
	if !frame.Update.Empty() {
		if err := validateRectangles(frame.Rectangles); err != nil {
			return false, err
		}
		if frame.Update.First < 0 || frame.Update.Count < 0 || frame.Update.First > len(frame.Rectangles) || frame.Update.Count > len(frame.Rectangles)-frame.Update.First {
			return false, fmt.Errorf("linux: invalid rectangle update")
		}
	}
	if frame.TextLabels != nil {
		if err := validateInitialTextLabels(frame.TextLabels); err != nil {
			return false, err
		}
	}
	if len(frame.TextLabels) > 0 && renderer.text.services == nil {
		if err := renderer.text.initialize(renderer); err != nil {
			return false, err
		}
		if err := renderer.text.createPipeline(renderer); err != nil {
			return false, err
		}
	}
	if err := vkCheck(renderer.api.waitForFences(renderer.device, 1, &renderer.fence, 1, math.MaxUint64), "wait before scene update"); err != nil {
		return false, err
	}
	if err := vkCheck(renderer.api.resetCommandPool(renderer.device, renderer.commandPool, 0), "reset scene commands"); err != nil {
		return false, err
	}
	if !frame.Update.Empty() {
		if frame.Update.Rebuild {
			renderer.rectangles = append(renderer.rectangles[:0], frame.Rectangles...)
			renderer.rebuilds++
		} else {
			if len(frame.Rectangles) < len(renderer.rectangles) {
				return false, fmt.Errorf("linux: rectangle count shrank without rebuild")
			}
			if len(frame.Rectangles) > len(renderer.rectangles) {
				if frame.Update.First > len(renderer.rectangles) || frame.Update.First+frame.Update.Count != len(frame.Rectangles) {
					return false, fmt.Errorf("linux: rectangle count grew outside a suffix update")
				}
				renderer.rectangles = append(renderer.rectangles, frame.Rectangles[len(renderer.rectangles):]...)
			}
			copy(renderer.rectangles[frame.Update.First:frame.Update.First+frame.Update.Count], frame.Rectangles[frame.Update.First:frame.Update.First+frame.Update.Count])
			renderer.partials++
		}
		renderer.rectangleBatches = render.GroupBatches(renderer.rectangleBatches, renderer.rectangles)
		if err := renderer.scene.upload(renderer, renderer.rectangles, frame.Update); err != nil {
			return false, err
		}
	}
	if frame.TextLabels != nil {
		if err := renderer.text.update(renderer, frame.TextLabels); err != nil {
			return false, err
		}
	}
	renderer.compositingBatches = groupCompositingLayers(renderer.rectangleBatches, renderer.text.labels, renderer.text.order, renderer.compositingBatches)
	if err := renderer.text.upload(renderer, renderer.pixelScale()); err != nil {
		return false, err
	}
	if err := renderer.recordCommands(vkExtent2D{Width: renderer.width, Height: renderer.height}); err != nil {
		return false, err
	}
	return true, nil
}

// pixelScale is the device pixels per logical pixel, at least 1.
func (renderer *vulkanRenderer) pixelScale() float32 {
	if renderer.scale < 1 {
		return 1
	}
	return renderer.scale
}

// setScale changes the scale of a window that already has a swapchain: the
// labels are rasterized again at the new one and every command is recorded
// again. The caller sets the new device size with resize.
func (renderer *vulkanRenderer) setScale(scale float32) error {
	for losses := 0; ; losses++ {
		retry, err := renderer.recoverFrom(renderer.setScaleOnce(scale), losses)
		if !retry {
			return err
		}
	}
}

func (renderer *vulkanRenderer) setScaleOnce(scale float32) error {
	if scale < 1 || scale == renderer.pixelScale() {
		return nil
	}
	renderer.scale = scale
	if renderer.device == 0 || renderer.commandPool == 0 {
		return nil
	}
	if err := vkCheck(renderer.api.deviceWaitIdle(renderer.device), "wait before scale change"); err != nil {
		return err
	}
	// Re-recording reuses command buffers that the previous frame left
	// executable; the pool was created without implicit reset.
	if err := renderer.resetCommands("reset commands before scale change"); err != nil {
		return err
	}
	if err := renderer.text.upload(renderer, scale); err != nil {
		return err
	}
	return renderer.recordCommands(vkExtent2D{Width: renderer.width, Height: renderer.height})
}

// resetCommands makes every buffer in the pool initial again. Callers must
// have waited for the device (or the frame fence) before resetting; recording
// paths that reuse buffers without this reset violate the command pool's
// single-use begin rule.
func (renderer *vulkanRenderer) resetCommands(operation string) error {
	if renderer.device == 0 || renderer.commandPool == 0 {
		return nil
	}
	return vkCheck(renderer.api.resetCommandPool(renderer.device, renderer.commandPool, 0), operation)
}

func (renderer *vulkanRenderer) resize(width, height uint32) error {
	for losses := 0; ; losses++ {
		retry, err := renderer.recoverFrom(renderer.resizeOnce(width, height), losses)
		if !retry {
			return err
		}
	}
}

func (renderer *vulkanRenderer) resizeOnce(width, height uint32) error {
	if width == 0 || height == 0 || (width == renderer.width && height == renderer.height) {
		return nil
	}
	renderer.width, renderer.height = width, height
	return renderer.recreateSwapchain()
}

func (renderer *vulkanRenderer) recreateSwapchain() error {
	if err := vkCheck(renderer.api.deviceWaitIdle(renderer.device), "wait before swapchain recreation"); err != nil {
		return err
	}
	oldSwapchain := renderer.swapchain
	renderer.swapchain = 0
	renderer.destroySwapchainResources()
	return renderer.createSwapchainWithOld(oldSwapchain)
}

func (renderer *vulkanRenderer) destroySwapchainResources() {
	if renderer.device == 0 {
		return
	}
	if renderer.commandPool != 0 {
		renderer.api.destroyCommandPool(renderer.device, renderer.commandPool, nil)
		renderer.commandPool = 0
	}
	renderer.uploadCommand = 0
	renderer.scene.destroyPipeline(renderer)
	renderer.text.destroyPipeline(renderer)
	renderer.frame.destroyPipeline(renderer)
	for _, framebuffer := range renderer.framebuffers {
		if framebuffer != 0 {
			renderer.api.destroyFramebuffer(renderer.device, framebuffer, nil)
		}
	}
	renderer.framebuffers = nil
	renderer.commands = nil
	if renderer.renderPass != 0 {
		renderer.api.destroyRenderPass(renderer.device, renderer.renderPass, nil)
		renderer.renderPass = 0
	}
	for _, view := range renderer.views {
		if view != 0 {
			renderer.api.destroyImageView(renderer.device, view, nil)
		}
	}
	renderer.views = nil
}

func (renderer *vulkanRenderer) destroySwapchain() {
	renderer.destroySwapchainResources()
	if renderer.swapchain != 0 {
		renderer.api.destroySwapchain(renderer.device, renderer.swapchain, nil)
		renderer.swapchain = 0
	}
}

func (renderer *vulkanRenderer) close() {
	if renderer.api == nil {
		return
	}
	if renderer.device != 0 {
		renderer.api.deviceWaitIdle(renderer.device)
		renderer.text.close(renderer)
		renderer.destroySwapchain()
		renderer.destroyOffscreen()
		renderer.frame.close(renderer)
		renderer.scene.closeBuffer(renderer)
		if renderer.fence != 0 {
			renderer.api.destroyFence(renderer.device, renderer.fence, nil)
		}
		if renderer.renderFinished != 0 {
			renderer.api.destroySemaphore(renderer.device, renderer.renderFinished, nil)
		}
		if renderer.imageAvailable != 0 {
			renderer.api.destroySemaphore(renderer.device, renderer.imageAvailable, nil)
		}
		renderer.api.destroyDevice(renderer.device, nil)
	}
	if renderer.surface != 0 && renderer.instance != 0 {
		if renderer.provider != nil {
			renderer.provider.DestroySurface(renderer.api, renderer.instance, renderer.surface)
		} else {
			renderer.api.destroySurface(renderer.instance, renderer.surface, nil)
		}
	}
	if renderer.instance != 0 {
		renderer.api.destroyInstance(renderer.instance, nil)
	}
	if renderer.api.library != 0 {
		purego.Dlclose(renderer.api.library)
	}
	renderer.api = nil
}

// noVulkan is the error for a machine that cannot draw: it wraps
// platform.ErrNoGraphics and says what to install, because "load Vulkan failed"
// does not tell a person on a cloud server with no GPU what to do next.
func noVulkan(reason string, cause error) error {
	const advice = "install the Vulkan loader (libvulkan1) and a driver; a machine without a GPU needs Mesa's software driver (mesa-vulkan-drivers, lavapipe), and a server with no display needs a virtual one such as Xvfb or the headless build"
	if cause != nil {
		return fmt.Errorf("linux: %s: %w (%w); %s", reason, platform.ErrNoGraphics, cause, advice)
	}
	return fmt.Errorf("linux: %s: %w; %s", reason, platform.ErrNoGraphics, advice)
}

func vkCheck(status int32, action string) error {
	if status != vkSuccess {
		return &vulkanResultError{action: action, status: status}
	}
	return nil
}

func clamp(value, minimum, maximum uint32) uint32 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

// recoverDevice makes the renderer again after the device or the surface was
// lost: every GPU object is released and the instance, surface, device and
// swapchain are created anew, then the scene and the label textures are
// uploaded again from what the renderer kept. Nothing is lost that the CPU
// does not hold, and the window stays where it is.
func (renderer *vulkanRenderer) recoverDevice() error {
	if renderer.provider == nil && renderer.connection == 0 {
		return fmt.Errorf("linux: only a window renderer can be rebuilt")
	}
	renderer.releaseDevice()
	provider := renderer.provider
	if provider == nil && renderer.connection != 0 {
		provider = newXcbSurfaceProvider(renderer.connection, renderer.window)
	}
	if err := renderer.initialize(provider); err != nil {
		return err
	}
	renderer.recoveries++
	return nil
}

// releaseDevice destroys everything the renderer made on the GPU and in the
// instance and clears the handles; the scene instances, the label descriptions
// and the swapchain size stay. Destroying objects of a lost device is allowed
// and its results are not looked at.
func (renderer *vulkanRenderer) releaseDevice() {
	if renderer.device != 0 {
		renderer.api.deviceWaitIdle(renderer.device)
		renderer.text.release(renderer)
		renderer.destroySwapchain()
		renderer.destroyOffscreen()
		renderer.frame.close(renderer)
		renderer.scene.close(renderer)
		if renderer.fence != 0 {
			renderer.api.destroyFence(renderer.device, renderer.fence, nil)
		}
		if renderer.renderFinished != 0 {
			renderer.api.destroySemaphore(renderer.device, renderer.renderFinished, nil)
		}
		if renderer.imageAvailable != 0 {
			renderer.api.destroySemaphore(renderer.device, renderer.imageAvailable, nil)
		}
		renderer.api.destroyDevice(renderer.device, nil)
	}
	if renderer.surface != 0 && renderer.instance != 0 {
		if renderer.provider != nil {
			renderer.provider.DestroySurface(renderer.api, renderer.instance, renderer.surface)
		} else {
			renderer.api.destroySurface(renderer.instance, renderer.surface, nil)
		}
	}
	if renderer.instance != 0 {
		renderer.api.destroyInstance(renderer.instance, nil)
	}
	renderer.instance, renderer.surface, renderer.physical, renderer.device, renderer.queue = 0, 0, 0, 0, 0
	renderer.fence, renderer.renderFinished, renderer.imageAvailable = 0, 0, 0
	renderer.dualSource = false
}
