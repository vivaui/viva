package linux

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

const (
	vkStructureMemoryAllocateInfo                   = 5
	vkStructureBufferCreateInfo                     = 12
	vkStructureShaderModuleCreateInfo               = 16
	vkStructurePipelineShaderStageCreateInfo        = 18
	vkStructurePipelineVertexInputStateCreateInfo   = 19
	vkStructurePipelineInputAssemblyStateCreateInfo = 20
	vkStructurePipelineViewportStateCreateInfo      = 22
	vkStructurePipelineRasterizationStateCreateInfo = 23
	vkStructurePipelineMultisampleStateCreateInfo   = 24
	vkStructurePipelineColorBlendStateCreateInfo    = 26
	vkStructurePipelineDynamicStateCreateInfo       = 27
	vkStructureGraphicsPipelineCreateInfo           = 28
	vkStructurePipelineLayoutCreateInfo             = 30
	vkBufferUsageVertex                             = 0x80
	vkMemoryHostVisible                             = 0x2
	vkMemoryHostCoherent                            = 0x4
	vkShaderStageVertex                             = 0x1
	vkShaderStageFragment                           = 0x10
	vkPrimitiveTriangleList                         = 3
	vkPolygonFill                                   = 0
	vkCullNone                                      = 0
	vkFrontFaceCounterClockwise                     = 0
	vkVertexInputRateInstance                       = 1
	vkFormatRGBA32Float                             = 109
	vkBlendZero                                     = 0
	vkBlendOne                                      = 1
	vkBlendOneMinusSourceAlpha                      = 7
	vkBlendOneMinusSource1Color                     = 16
	vkBlendOneMinusSource1Alpha                     = 18
	// vkFeatureCount is the number of VkBool32 in VkPhysicalDeviceFeatures; the one used
	// is dualSrcBlend.
	vkFeatureCount, vkFeatureDualSourceBlend = 55, 7
	vkBlendAdd                               = 0
	vkColorComponentRGBA                     = 0xf
	vkDynamicViewport                        = 0
	vkDynamicScissor                         = 1
	vkInstanceBytes                          = 80
)

type vkMemoryType struct{ PropertyFlags, HeapIndex uint32 }
type vkMemoryHeap struct {
	Size    uint64
	Flags   uint32
	Padding uint32
}
type vkPhysicalDeviceMemoryProperties struct {
	MemoryTypeCount uint32
	MemoryTypes     [32]vkMemoryType
	MemoryHeapCount uint32
	MemoryHeaps     [16]vkMemoryHeap
}
type vkMemoryRequirements struct {
	Size, Alignment uint64
	MemoryTypeBits  uint32
	Padding         uint32
}
type vkBufferCreateInfo struct {
	SType            uint32
	Padding0         uint32
	Next             uintptr
	Flags            uint32
	Padding1         uint32
	Size             uint64
	Usage            uint32
	SharingMode      uint32
	QueueFamilyCount uint32
	Padding2         uint32
	QueueFamilies    uintptr
}
type vkMemoryAllocateInfo struct {
	SType           uint32
	Padding0        uint32
	Next            uintptr
	AllocationSize  uint64
	MemoryTypeIndex uint32
	Padding1        uint32
}
type vkShaderModuleCreateInfo struct {
	SType    uint32
	Padding0 uint32
	Next     uintptr
	Flags    uint32
	Padding1 uint32
	CodeSize uintptr
	Code     *uint32
}
type vkPipelineShaderStageCreateInfo struct {
	SType          uint32
	Padding0       uint32
	Next           uintptr
	Flags          uint32
	Stage          uint32
	Module         uintptr
	Name           *byte
	Specialization uintptr
}
type vkVertexInputBindingDescription struct{ Binding, Stride, InputRate uint32 }
type vkVertexInputAttributeDescription struct{ Location, Binding, Format, Offset uint32 }
type vkPipelineVertexInputStateCreateInfo struct {
	SType          uint32
	Padding0       uint32
	Next           uintptr
	Flags          uint32
	BindingCount   uint32
	Bindings       *vkVertexInputBindingDescription
	AttributeCount uint32
	Padding1       uint32
	Attributes     *vkVertexInputAttributeDescription
}
type vkPipelineInputAssemblyStateCreateInfo struct {
	SType            uint32
	Padding0         uint32
	Next             uintptr
	Flags            uint32
	Topology         uint32
	PrimitiveRestart uint32
}
type vkPipelineViewportStateCreateInfo struct {
	SType         uint32
	Padding0      uint32
	Next          uintptr
	Flags         uint32
	ViewportCount uint32
	Viewports     uintptr
	ScissorCount  uint32
	Padding1      uint32
	Scissors      uintptr
}
type vkPipelineRasterizationStateCreateInfo struct {
	SType             uint32
	Padding0          uint32
	Next              uintptr
	Flags             uint32
	DepthClamp        uint32
	RasterizerDiscard uint32
	PolygonMode       uint32
	CullMode          uint32
	FrontFace         uint32
	DepthBias         uint32
	DepthBiasConstant float32
	DepthBiasClamp    float32
	DepthBiasSlope    float32
	LineWidth         float32
}
type vkPipelineMultisampleStateCreateInfo struct {
	SType                uint32
	Padding0             uint32
	Next                 uintptr
	Flags                uint32
	RasterizationSamples uint32
	SampleShading        uint32
	MinSampleShading     float32
	SampleMask           uintptr
	AlphaToCoverage      uint32
	AlphaToOne           uint32
}
type vkPipelineColorBlendAttachmentState struct {
	BlendEnable               uint32
	SourceColor, DestColor    uint32
	ColorOperation            uint32
	SourceAlpha, DestAlpha    uint32
	AlphaOperation, WriteMask uint32
}
type vkPipelineColorBlendStateCreateInfo struct {
	SType           uint32
	Padding0        uint32
	Next            uintptr
	Flags           uint32
	LogicOpEnable   uint32
	LogicOp         uint32
	AttachmentCount uint32
	Attachments     *vkPipelineColorBlendAttachmentState
	BlendConstants  [4]float32
}
type vkPipelineDynamicStateCreateInfo struct {
	SType        uint32
	Padding0     uint32
	Next         uintptr
	Flags        uint32
	DynamicCount uint32
	Dynamics     *uint32
}
type vkPushConstantRange struct{ StageFlags, Offset, Size uint32 }
type vkPipelineLayoutCreateInfo struct {
	SType             uint32
	Padding0          uint32
	Next              uintptr
	Flags             uint32
	SetLayoutCount    uint32
	SetLayouts        uintptr
	PushConstantCount uint32
	Padding1          uint32
	PushConstants     *vkPushConstantRange
}
type vkGraphicsPipelineCreateInfo struct {
	SType             uint32
	Padding0          uint32
	Next              uintptr
	Flags             uint32
	StageCount        uint32
	Stages            *vkPipelineShaderStageCreateInfo
	VertexInput       *vkPipelineVertexInputStateCreateInfo
	InputAssembly     *vkPipelineInputAssemblyStateCreateInfo
	Tessellation      uintptr
	Viewport          *vkPipelineViewportStateCreateInfo
	Rasterization     *vkPipelineRasterizationStateCreateInfo
	Multisample       *vkPipelineMultisampleStateCreateInfo
	DepthStencil      uintptr
	ColorBlend        *vkPipelineColorBlendStateCreateInfo
	Dynamic           *vkPipelineDynamicStateCreateInfo
	Layout            uintptr
	RenderPass        uintptr
	Subpass           uint32
	Padding2          uint32
	BasePipeline      uintptr
	BasePipelineIndex int32
	Padding3          uint32
}
type vkViewport struct{ X, Y, Width, Height, MinimumDepth, MaximumDepth float32 }

// vulkanRectangleInstance is one instance: bounds, color and style as in the other
// backends, then a triangle's colors at its second and third corner.
type vulkanRectangleInstance struct{ Bounds, Color, Style, Shade1, Shade2 [4]float32 }

type vulkanScene struct {
	pipelineLayout, pipeline uintptr
	buffer, memory           uintptr
	mapped                   unsafe.Pointer
	capacity                 int
	instances                []vulkanRectangleInstance
}

func registerVulkanScene(api *vulkanAPI, library uintptr) {
	puregoRegister(&api.getPhysicalDeviceMemoryProperties, library, "vkGetPhysicalDeviceMemoryProperties")
	puregoRegister(&api.createBuffer, library, "vkCreateBuffer")
	puregoRegister(&api.destroyBuffer, library, "vkDestroyBuffer")
	puregoRegister(&api.getBufferMemoryRequirements, library, "vkGetBufferMemoryRequirements")
	puregoRegister(&api.allocateMemory, library, "vkAllocateMemory")
	puregoRegister(&api.freeMemory, library, "vkFreeMemory")
	puregoRegister(&api.bindBufferMemory, library, "vkBindBufferMemory")
	puregoRegister(&api.mapMemory, library, "vkMapMemory")
	puregoRegister(&api.unmapMemory, library, "vkUnmapMemory")
	puregoRegister(&api.createShaderModule, library, "vkCreateShaderModule")
	puregoRegister(&api.destroyShaderModule, library, "vkDestroyShaderModule")
	puregoRegister(&api.createPipelineLayout, library, "vkCreatePipelineLayout")
	puregoRegister(&api.destroyPipelineLayout, library, "vkDestroyPipelineLayout")
	puregoRegister(&api.createGraphicsPipelines, library, "vkCreateGraphicsPipelines")
	puregoRegister(&api.destroyPipeline, library, "vkDestroyPipeline")
}

func puregoRegister(destination any, library uintptr, name string) {
	purego.RegisterLibFunc(destination, library, name)
}

func makeVulkanRectangleInstance(rectangle render.Rectangle) vulkanRectangleInstance {
	instance := vulkanRectangleInstance{
		Bounds: [4]float32{rectangle.Bounds.X, rectangle.Bounds.Y, rectangle.Bounds.Width, rectangle.Bounds.Height},
		Color:  [4]float32{rectangle.Color.Red, rectangle.Color.Green, rectangle.Color.Blue, rectangle.Color.Alpha},
		Style:  [4]float32{rectangle.Radius},
	}
	if rectangle.Kind == scene.KindTriangle {
		instance.Bounds = [4]float32{rectangle.Points[0].X, rectangle.Points[0].Y, rectangle.Points[1].X, rectangle.Points[1].Y}
		instance.Style = [4]float32{rectangle.Points[2].X, float32(rectangle.Layer), rectangle.Points[2].Y, float32(rectangle.EdgeMask + 1)}
		colors := rectangle.VertexColors()
		instance.Shade1 = [4]float32{colors[1].Red, colors[1].Green, colors[1].Blue, colors[1].Alpha}
		instance.Shade2 = [4]float32{colors[2].Red, colors[2].Green, colors[2].Blue, colors[2].Alpha}
	}
	if rectangle.Kind == scene.KindShadow && rectangle.Shadow.Blur > 0 {
		// The casting rectangle keeps the slot's bounds and radius; the blur is the
		// third style value and the offset and spread the first three shade1 values.
		instance.Style[2] = rectangle.Shadow.Blur
		instance.Shade1 = [4]float32{rectangle.Shadow.OffsetX, rectangle.Shadow.OffsetY, rectangle.Shadow.Spread}
	}
	return instance
}

func (sceneState *vulkanScene) upload(renderer *vulkanRenderer, rectangles []render.Rectangle, update render.Update) error {
	first, count := update.First, update.Count
	if update.Rebuild {
		if len(rectangles) > cap(sceneState.instances) {
			sceneState.instances = make([]vulkanRectangleInstance, len(rectangles))
		} else {
			sceneState.instances = sceneState.instances[:len(rectangles)]
		}
		first, count = 0, len(rectangles)
	} else if len(sceneState.instances) < len(rectangles) {
		// A pure append grows the instance store; the new range was validated
		// by the caller. Existing instances stay in their slots.
		if len(rectangles) > cap(sceneState.instances) {
			next := make([]vulkanRectangleInstance, len(rectangles))
			copy(next, sceneState.instances)
			sceneState.instances = next
		} else {
			old := len(sceneState.instances)
			sceneState.instances = sceneState.instances[:len(rectangles)]
			clear(sceneState.instances[old:])
		}
	}
	for index := first; index < first+count; index++ {
		sceneState.instances[index] = makeVulkanRectangleInstance(rectangles[index])
	}
	if len(sceneState.instances) > sceneState.capacity {
		if err := sceneState.grow(renderer, len(sceneState.instances)); err != nil {
			return err
		}
		first, count = 0, len(sceneState.instances)
	}
	if count == 0 {
		return nil
	}
	destination := unsafe.Slice((*byte)(unsafe.Add(sceneState.mapped, first*vkInstanceBytes)), count*vkInstanceBytes)
	source := unsafe.Slice((*byte)(unsafe.Pointer(&sceneState.instances[first])), count*vkInstanceBytes)
	copy(destination, source)
	runtime.KeepAlive(sceneState.instances)
	return nil
}

func (sceneState *vulkanScene) grow(renderer *vulkanRenderer, required int) error {
	capacity := max(required, max(64, sceneState.capacity*2))
	if capacity > int(^uint32(0))/vkInstanceBytes {
		return fmt.Errorf("linux: rectangle buffer exceeds device limit")
	}
	sceneState.closeBuffer(renderer)
	info := vkBufferCreateInfo{
		SType: vkStructureBufferCreateInfo, Size: uint64(capacity * vkInstanceBytes),
		Usage: vkBufferUsageVertex, SharingMode: vkSharingExclusive,
	}
	if err := vkCheck(renderer.api.createBuffer(renderer.device, &info, nil, &sceneState.buffer), "create scene buffer"); err != nil {
		return err
	}
	var requirements vkMemoryRequirements
	renderer.api.getBufferMemoryRequirements(renderer.device, sceneState.buffer, &requirements)
	memoryType, ok := renderer.memoryType(requirements.MemoryTypeBits, vkMemoryHostVisible|vkMemoryHostCoherent)
	if !ok {
		sceneState.closeBuffer(renderer)
		return fmt.Errorf("linux: Vulkan device has no coherent host-visible memory for scene data")
	}
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &sceneState.memory), "allocate scene memory"); err != nil {
		sceneState.closeBuffer(renderer)
		return err
	}
	if err := vkCheck(renderer.api.bindBufferMemory(renderer.device, sceneState.buffer, sceneState.memory, 0), "bind scene memory"); err != nil {
		sceneState.closeBuffer(renderer)
		return err
	}
	if err := vkCheck(renderer.api.mapMemory(renderer.device, sceneState.memory, 0, requirements.Size, 0, &sceneState.mapped), "map scene memory"); err != nil {
		sceneState.closeBuffer(renderer)
		return err
	}
	sceneState.capacity = capacity
	return nil
}

func (renderer *vulkanRenderer) memoryType(bits, properties uint32) (uint32, bool) {
	var memory vkPhysicalDeviceMemoryProperties
	renderer.api.getPhysicalDeviceMemoryProperties(renderer.physical, &memory)
	for index := uint32(0); index < memory.MemoryTypeCount && index < uint32(len(memory.MemoryTypes)); index++ {
		if bits&(1<<index) != 0 && memory.MemoryTypes[index].PropertyFlags&properties == properties {
			return index, true
		}
	}
	return 0, false
}

func (sceneState *vulkanScene) createPipeline(renderer *vulkanRenderer) (err error) {
	if sceneState.pipeline != 0 {
		return nil
	}
	// The fragment stage reads the content origin and the pixel size to place
	// its sample in the same device space the vertices were given in.
	push := vkPushConstantRange{StageFlags: vkShaderStageVertex | vkShaderStageFragment, Size: 24}
	layoutInfo := vkPipelineLayoutCreateInfo{SType: vkStructurePipelineLayoutCreateInfo, PushConstantCount: 1, PushConstants: &push}
	if err := vkCheck(renderer.api.createPipelineLayout(renderer.device, &layoutInfo, nil, &sceneState.pipelineLayout), "create scene pipeline layout"); err != nil {
		return err
	}
	defer func() {
		if err != nil && sceneState.pipelineLayout != 0 {
			renderer.api.destroyPipelineLayout(renderer.device, sceneState.pipelineLayout, nil)
			sceneState.pipelineLayout = 0
		}
	}()
	sceneState.pipeline, err = sceneState.buildPipeline(renderer, false)
	return err
}

// buildPipeline creates the scene pipeline for the render pass. With cut it
// blends so that what it draws takes away from what is there instead of adding:
// the target keeps 1 - alpha of itself (see vulkan_frame_linux.go).
func (sceneState *vulkanScene) buildPipeline(renderer *vulkanRenderer, cut bool) (pipeline uintptr, err error) {
	vertex, err := renderer.shaderModule(sceneVertexShader)
	if err != nil {
		return 0, err
	}
	defer renderer.api.destroyShaderModule(renderer.device, vertex, nil)
	fragment, err := renderer.shaderModule(sceneFragmentShader)
	if err != nil {
		return 0, err
	}
	defer renderer.api.destroyShaderModule(renderer.device, fragment, nil)
	entry := append([]byte("main"), 0)
	stages := [2]vkPipelineShaderStageCreateInfo{
		{SType: vkStructurePipelineShaderStageCreateInfo, Stage: vkShaderStageVertex, Module: vertex, Name: &entry[0]},
		{SType: vkStructurePipelineShaderStageCreateInfo, Stage: vkShaderStageFragment, Module: fragment, Name: &entry[0]},
	}
	binding := vkVertexInputBindingDescription{Stride: vkInstanceBytes, InputRate: vkVertexInputRateInstance}
	attributes := [5]vkVertexInputAttributeDescription{
		{Location: 0, Format: vkFormatRGBA32Float, Offset: 0},
		{Location: 1, Format: vkFormatRGBA32Float, Offset: 16},
		{Location: 2, Format: vkFormatRGBA32Float, Offset: 32},
		{Location: 3, Format: vkFormatRGBA32Float, Offset: 48},
		{Location: 4, Format: vkFormatRGBA32Float, Offset: 64},
	}
	vertexInput := vkPipelineVertexInputStateCreateInfo{
		SType: vkStructurePipelineVertexInputStateCreateInfo, BindingCount: 1, Bindings: &binding,
		AttributeCount: uint32(len(attributes)), Attributes: &attributes[0],
	}
	assembly := vkPipelineInputAssemblyStateCreateInfo{SType: vkStructurePipelineInputAssemblyStateCreateInfo, Topology: vkPrimitiveTriangleList}
	viewport := vkPipelineViewportStateCreateInfo{SType: vkStructurePipelineViewportStateCreateInfo, ViewportCount: 1, ScissorCount: 1}
	raster := vkPipelineRasterizationStateCreateInfo{
		SType: vkStructurePipelineRasterizationStateCreateInfo, PolygonMode: vkPolygonFill,
		CullMode: vkCullNone, FrontFace: vkFrontFaceCounterClockwise, LineWidth: 1,
	}
	multisample := vkPipelineMultisampleStateCreateInfo{SType: vkStructurePipelineMultisampleStateCreateInfo, RasterizationSamples: vkSampleCountOne}
	blendAttachment := vkPipelineColorBlendAttachmentState{
		BlendEnable: 1, SourceColor: vkBlendOne, DestColor: vkBlendOneMinusSourceAlpha, ColorOperation: vkBlendAdd,
		SourceAlpha: vkBlendOne, DestAlpha: vkBlendOneMinusSourceAlpha, AlphaOperation: vkBlendAdd, WriteMask: vkColorComponentRGBA,
	}
	if cut {
		blendAttachment.SourceColor, blendAttachment.SourceAlpha = vkBlendZero, vkBlendZero
	}
	blend := vkPipelineColorBlendStateCreateInfo{SType: vkStructurePipelineColorBlendStateCreateInfo, AttachmentCount: 1, Attachments: &blendAttachment}
	dynamics := [2]uint32{vkDynamicViewport, vkDynamicScissor}
	dynamic := vkPipelineDynamicStateCreateInfo{SType: vkStructurePipelineDynamicStateCreateInfo, DynamicCount: uint32(len(dynamics)), Dynamics: &dynamics[0]}
	info := vkGraphicsPipelineCreateInfo{
		SType: vkStructureGraphicsPipelineCreateInfo, StageCount: uint32(len(stages)), Stages: &stages[0],
		VertexInput: &vertexInput, InputAssembly: &assembly, Viewport: &viewport, Rasterization: &raster,
		Multisample: &multisample, ColorBlend: &blend, Dynamic: &dynamic,
		Layout: sceneState.pipelineLayout, RenderPass: renderer.renderPass,
	}
	return pipeline, vkCheck(renderer.api.createGraphicsPipelines(renderer.device, 0, 1, &info, nil, &pipeline), "create scene pipeline")
}

func (renderer *vulkanRenderer) shaderModule(code []byte) (uintptr, error) {
	if len(code) < 4 || len(code)%4 != 0 {
		return 0, fmt.Errorf("linux: invalid embedded SPIR-V shader")
	}
	info := vkShaderModuleCreateInfo{
		SType: vkStructureShaderModuleCreateInfo, CodeSize: uintptr(len(code)),
		Code: (*uint32)(unsafe.Pointer(&code[0])),
	}
	var module uintptr
	if err := vkCheck(renderer.api.createShaderModule(renderer.device, &info, nil, &module), "create shader module"); err != nil {
		return 0, err
	}
	return module, nil
}

func (sceneState *vulkanScene) record(renderer *vulkanRenderer, command uintptr, extent vkExtent2D, batch compositingBatch) {
	first, count := batch.First, batch.Count
	if count <= 0 || first < 0 || first+count > len(sceneState.instances) {
		return
	}
	scratch := &renderer.scratch
	// The scene is drawn in the content rectangle of the target: all of it, or
	// what a window frame leaves inside its margin.
	originX, originY := scratch.contentX, scratch.contentY
	scratch.scissor = vkRect2D{Offset: [2]int32{originX, originY}, Extent: extent}
	scale := renderer.pixelScale()
	if batch.Clipped {
		// The clip is in logical pixels, the target in device pixels.
		clip, visible := render.ScissorOf(batch.Clip, scale, int(extent.Width), int(extent.Height))
		if !visible {
			return
		}
		scratch.scissor = vkRect2D{
			Offset: [2]int32{originX + int32(clip.Left), originY + int32(clip.Top)},
			Extent: vkExtent2D{Width: uint32(clip.Right - clip.Left), Height: uint32(clip.Bottom - clip.Top)},
		}
	}
	renderer.api.cmdBindPipeline(command, vkPipelineBindPointGraphics, sceneState.pipeline)
	scratch.viewport = vkViewport{X: float32(originX), Y: float32(originY), Width: float32(extent.Width), Height: float32(extent.Height), MaximumDepth: 1}
	renderer.api.cmdSetViewport(command, 0, 1, &scratch.viewport)
	renderer.api.cmdSetScissor(command, 0, 1, &scratch.scissor)
	// The viewport is the target in device pixels; the scene is positioned in
	// logical ones, so the shader divides by the target's size in those.
	scratch.sceneFrame = [6]float32{float32(extent.Width) / scale, float32(extent.Height) / scale, 1 / scale, 0, float32(originX), float32(originY)}
	renderer.api.cmdPushConstants(command, sceneState.pipelineLayout, vkShaderStageVertex|vkShaderStageFragment, 0, uint32(unsafe.Sizeof(scratch.sceneFrame)), unsafe.Pointer(&scratch.sceneFrame[0]))
	scratch.offset = 0
	renderer.api.cmdBindVertexBuffers(command, 0, 1, &sceneState.buffer, &scratch.offset)
	renderer.api.cmdDraw(command, 6, uint32(count), 0, uint32(first))
}

func (sceneState *vulkanScene) destroyPipeline(renderer *vulkanRenderer) {
	if sceneState.pipeline != 0 {
		renderer.api.destroyPipeline(renderer.device, sceneState.pipeline, nil)
		sceneState.pipeline = 0
	}
	if sceneState.pipelineLayout != 0 {
		renderer.api.destroyPipelineLayout(renderer.device, sceneState.pipelineLayout, nil)
		sceneState.pipelineLayout = 0
	}
}

func (sceneState *vulkanScene) closeBuffer(renderer *vulkanRenderer) {
	if sceneState.mapped != nil && sceneState.memory != 0 {
		renderer.api.unmapMemory(renderer.device, sceneState.memory)
		sceneState.mapped = nil
	}
	if sceneState.buffer != 0 {
		renderer.api.destroyBuffer(renderer.device, sceneState.buffer, nil)
		sceneState.buffer = 0
	}
	if sceneState.memory != 0 {
		renderer.api.freeMemory(renderer.device, sceneState.memory, nil)
		sceneState.memory = 0
	}
	sceneState.capacity = 0
}

func (sceneState *vulkanScene) close(renderer *vulkanRenderer) {
	sceneState.destroyPipeline(renderer)
	sceneState.closeBuffer(renderer)
}
