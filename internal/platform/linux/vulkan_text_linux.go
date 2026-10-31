package linux

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

const (
	vkStructureTextImageCreateInfo           = 14
	vkStructureTextSamplerCreateInfo         = 31
	vkStructureDescriptorSetLayoutCreateInfo = 32
	vkStructureDescriptorPoolCreateInfo      = 33
	vkStructureDescriptorSetAllocateInfo     = 34
	vkStructureWriteDescriptorSet            = 35
	vkStructureImageMemoryBarrier            = 45
	vkImageType2D                            = 1
	vkImageTilingOptimal                     = 0
	vkImageUsageTransferDestination          = 0x2
	vkImageUsageSampled                      = 0x4
	vkBufferUsageTransferSource              = 0x1
	vkMemoryDeviceLocal                      = 0x1
	vkImageLayoutTransferDestination         = 7
	vkImageLayoutShaderReadOnly              = 5
	vkPipelineStageTop                       = 0x1
	vkPipelineStageFragment                  = 0x80
	vkPipelineStageTransfer                  = 0x1000
	vkAccessShaderRead                       = 0x20
	vkAccessTransferWrite                    = 0x1000
	vkQueueFamilyIgnored                     = ^uint32(0)
	vkDescriptorCombinedImageSampler         = 1
	vkFilterLinear                           = 1
	vkSamplerMipmapLinear                    = 1
	vkSamplerAddressClampEdge                = 2
	vkCommandOneTimeSubmit                   = 1
	vkMaximumTextLabels                      = 2048
)

type vkExtent3D struct{ Width, Height, Depth uint32 }
type vkImageCreateInfo struct {
	SType            uint32
	Padding0         uint32
	Next             uintptr
	Flags            uint32
	ImageType        uint32
	Format           uint32
	Extent           vkExtent3D
	MipLevels        uint32
	ArrayLayers      uint32
	Samples          uint32
	Tiling           uint32
	Usage            uint32
	SharingMode      uint32
	QueueFamilyCount uint32
	Padding1         uint32
	QueueFamilies    uintptr
	InitialLayout    uint32
	Padding2         uint32
}
type vkSamplerCreateInfo struct {
	SType                   uint32
	Padding0                uint32
	Next                    uintptr
	Flags                   uint32
	MagFilter               uint32
	MinFilter               uint32
	MipmapMode              uint32
	AddressU                uint32
	AddressV                uint32
	AddressW                uint32
	MipLodBias              float32
	AnisotropyEnable        uint32
	MaxAnisotropy           float32
	CompareEnable           uint32
	CompareOperation        uint32
	MinLod                  float32
	MaxLod                  float32
	BorderColor             uint32
	UnnormalizedCoordinates uint32
}
type vkDescriptorSetLayoutBinding struct {
	Binding, DescriptorType, DescriptorCount, StageFlags uint32
	ImmutableSamplers                                    uintptr
}
type vkDescriptorSetLayoutCreateInfo struct {
	SType        uint32
	Padding0     uint32
	Next         uintptr
	Flags        uint32
	BindingCount uint32
	Bindings     *vkDescriptorSetLayoutBinding
}
type vkDescriptorPoolSize struct{ Type, Count uint32 }
type vkDescriptorPoolCreateInfo struct {
	SType     uint32
	Padding0  uint32
	Next      uintptr
	Flags     uint32
	MaxSets   uint32
	PoolCount uint32
	Padding1  uint32
	PoolSizes *vkDescriptorPoolSize
}
type vkDescriptorSetAllocateInfo struct {
	SType      uint32
	Padding0   uint32
	Next       uintptr
	Pool       uintptr
	SetCount   uint32
	Padding1   uint32
	SetLayouts *uintptr
}
type vkDescriptorImageInfo struct {
	Sampler, ImageView uintptr
	ImageLayout        uint32
	Padding0           uint32
}
type vkWriteDescriptorSet struct {
	SType           uint32
	Padding0        uint32
	Next            uintptr
	DestinationSet  uintptr
	Binding         uint32
	ArrayElement    uint32
	DescriptorCount uint32
	DescriptorType  uint32
	ImageInfo       *vkDescriptorImageInfo
	BufferInfo      uintptr
	TexelBufferView uintptr
}
type vkImageMemoryBarrier struct {
	SType                  uint32
	Padding0               uint32
	Next                   uintptr
	SourceAccess           uint32
	DestinationAccess      uint32
	OldLayout              uint32
	NewLayout              uint32
	SourceQueueFamily      uint32
	DestinationQueueFamily uint32
	Image                  uintptr
	Range                  vkSubresourceRange
	Padding1               uint32
}
type vkImageSubresourceLayers struct{ Aspect, MipLevel, BaseArrayLayer, LayerCount uint32 }
type vkBufferImageCopy struct {
	BufferOffset      uint64
	BufferRowLength   uint32
	BufferImageHeight uint32
	ImageSubresource  vkImageSubresourceLayers
	ImageOffset       [3]int32
	ImageExtent       vkExtent3D
}

type vulkanLabelTexture struct {
	label                    platform.TextLabel
	runs                     []ui.TextRun
	image, memory, view, set uintptr
	scale                    float32
	// width and height are the texture's size in device pixels.
	width, height int
	// lcd is set when the texture holds the coverage of each channel of LCD text, to be
	// blended with the label's color by the LCD pipeline.
	lcd bool
	// spans are the columns of an lcd grid label's texture that are text of one color;
	// empty for a label that is all in label.Color.
	spans []colorSpan
}

type vulkanText struct {
	services                          *textServices
	descriptorLayout, descriptorPool  uintptr
	sampler, pipelineLayout, pipeline uintptr
	lcdPipeline                       uintptr
	labels                            []vulkanLabelTexture
	order                             []int
}

type vulkanStagingBuffer struct{ buffer, memory uintptr }

func registerVulkanText(api *vulkanAPI, library uintptr) {
	puregoRegister(&api.createImage, library, "vkCreateImage")
	puregoRegister(&api.destroyImage, library, "vkDestroyImage")
	puregoRegister(&api.getImageMemoryRequirements, library, "vkGetImageMemoryRequirements")
	puregoRegister(&api.bindImageMemory, library, "vkBindImageMemory")
	puregoRegister(&api.createSampler, library, "vkCreateSampler")
	puregoRegister(&api.destroySampler, library, "vkDestroySampler")
	puregoRegister(&api.createDescriptorSetLayout, library, "vkCreateDescriptorSetLayout")
	puregoRegister(&api.destroyDescriptorSetLayout, library, "vkDestroyDescriptorSetLayout")
	puregoRegister(&api.createDescriptorPool, library, "vkCreateDescriptorPool")
	puregoRegister(&api.destroyDescriptorPool, library, "vkDestroyDescriptorPool")
	puregoRegister(&api.allocateDescriptorSets, library, "vkAllocateDescriptorSets")
	puregoRegister(&api.updateDescriptorSets, library, "vkUpdateDescriptorSets")
	puregoRegister(&api.cmdPipelineBarrier, library, "vkCmdPipelineBarrier")
	puregoRegister(&api.cmdCopyBufferToImage, library, "vkCmdCopyBufferToImage")
}

func (textState *vulkanText) initialize(renderer *vulkanRenderer) error {
	if textState.services != nil {
		return nil
	}
	services, err := newTextServices()
	if err != nil {
		return err
	}
	textState.services = services
	binding := vkDescriptorSetLayoutBinding{DescriptorType: vkDescriptorCombinedImageSampler, DescriptorCount: 1, StageFlags: vkShaderStageFragment}
	layoutInfo := vkDescriptorSetLayoutCreateInfo{SType: vkStructureDescriptorSetLayoutCreateInfo, BindingCount: 1, Bindings: &binding}
	if err := vkCheck(renderer.api.createDescriptorSetLayout(renderer.device, &layoutInfo, nil, &textState.descriptorLayout), "create text descriptor layout"); err != nil {
		textState.close(renderer)
		return err
	}
	samplerInfo := vkSamplerCreateInfo{
		SType: vkStructureTextSamplerCreateInfo, MagFilter: vkFilterLinear, MinFilter: vkFilterLinear,
		MipmapMode: vkSamplerMipmapLinear, AddressU: vkSamplerAddressClampEdge,
		AddressV: vkSamplerAddressClampEdge, AddressW: vkSamplerAddressClampEdge, MaxLod: 1,
	}
	if err := vkCheck(renderer.api.createSampler(renderer.device, &samplerInfo, nil, &textState.sampler), "create text sampler"); err != nil {
		textState.close(renderer)
		return err
	}
	poolSize := vkDescriptorPoolSize{Type: vkDescriptorCombinedImageSampler, Count: vkMaximumTextLabels}
	poolInfo := vkDescriptorPoolCreateInfo{SType: vkStructureDescriptorPoolCreateInfo, MaxSets: vkMaximumTextLabels, PoolCount: 1, PoolSizes: &poolSize}
	if err := vkCheck(renderer.api.createDescriptorPool(renderer.device, &poolInfo, nil, &textState.descriptorPool), "create text descriptor pool"); err != nil {
		textState.close(renderer)
		return err
	}
	return nil
}

func (textState *vulkanText) createPipeline(renderer *vulkanRenderer) (err error) {
	if textState.services == nil || textState.pipeline != 0 {
		return nil
	}
	vertex, err := renderer.shaderModule(textVertexShader)
	if err != nil {
		return err
	}
	defer renderer.api.destroyShaderModule(renderer.device, vertex, nil)
	fragment, err := renderer.shaderModule(textFragmentShader)
	if err != nil {
		return err
	}
	defer renderer.api.destroyShaderModule(renderer.device, fragment, nil)
	entry := append([]byte("main"), 0)
	stages := [2]vkPipelineShaderStageCreateInfo{
		{SType: vkStructurePipelineShaderStageCreateInfo, Stage: vkShaderStageVertex, Module: vertex, Name: &entry[0]},
		{SType: vkStructurePipelineShaderStageCreateInfo, Stage: vkShaderStageFragment, Module: fragment, Name: &entry[0]},
	}
	vertexInput := vkPipelineVertexInputStateCreateInfo{SType: vkStructurePipelineVertexInputStateCreateInfo}
	assembly := vkPipelineInputAssemblyStateCreateInfo{SType: vkStructurePipelineInputAssemblyStateCreateInfo, Topology: vkPrimitiveTriangleList}
	viewport := vkPipelineViewportStateCreateInfo{SType: vkStructurePipelineViewportStateCreateInfo, ViewportCount: 1, ScissorCount: 1}
	raster := vkPipelineRasterizationStateCreateInfo{SType: vkStructurePipelineRasterizationStateCreateInfo, PolygonMode: vkPolygonFill, CullMode: vkCullNone, FrontFace: vkFrontFaceCounterClockwise, LineWidth: 1}
	multisample := vkPipelineMultisampleStateCreateInfo{SType: vkStructurePipelineMultisampleStateCreateInfo, RasterizationSamples: vkSampleCountOne}
	blendAttachment := vkPipelineColorBlendAttachmentState{
		BlendEnable: 1, SourceColor: vkBlendOne, DestColor: vkBlendOneMinusSourceAlpha, ColorOperation: vkBlendAdd,
		SourceAlpha: vkBlendOne, DestAlpha: vkBlendOneMinusSourceAlpha, AlphaOperation: vkBlendAdd, WriteMask: vkColorComponentRGBA,
	}
	blend := vkPipelineColorBlendStateCreateInfo{SType: vkStructurePipelineColorBlendStateCreateInfo, AttachmentCount: 1, Attachments: &blendAttachment}
	dynamics := [2]uint32{vkDynamicViewport, vkDynamicScissor}
	dynamic := vkPipelineDynamicStateCreateInfo{SType: vkStructurePipelineDynamicStateCreateInfo, DynamicCount: 2, Dynamics: &dynamics[0]}
	// The vertex shader takes the label's frame; the LCD fragment shader its color.
	push := [2]vkPushConstantRange{{StageFlags: vkShaderStageVertex, Size: 32}, {StageFlags: vkShaderStageFragment, Offset: 32, Size: 16}}
	layoutInfo := vkPipelineLayoutCreateInfo{
		SType: vkStructurePipelineLayoutCreateInfo, SetLayoutCount: 1,
		SetLayouts: uintptr(unsafe.Pointer(&textState.descriptorLayout)), PushConstantCount: 2, PushConstants: &push[0],
	}
	if err := vkCheck(renderer.api.createPipelineLayout(renderer.device, &layoutInfo, nil, &textState.pipelineLayout), "create text pipeline layout"); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			textState.destroyPipeline(renderer)
		}
	}()
	info := vkGraphicsPipelineCreateInfo{
		SType: vkStructureGraphicsPipelineCreateInfo, StageCount: 2, Stages: &stages[0],
		VertexInput: &vertexInput, InputAssembly: &assembly, Viewport: &viewport, Rasterization: &raster,
		Multisample: &multisample, ColorBlend: &blend, Dynamic: &dynamic,
		Layout: textState.pipelineLayout, RenderPass: renderer.renderPass,
	}
	if err := vkCheck(renderer.api.createGraphicsPipelines(renderer.device, 0, 1, &info, nil, &textState.pipeline), "create text pipeline"); err != nil {
		return err
	}
	if !renderer.dualSource {
		return nil
	}
	// LCD text: the fragment shader's second output is the factor that keeps the target
	// per channel (dual-source blending).
	lcdFragment, err := renderer.shaderModule(textLCDFragmentShader)
	if err != nil {
		return err
	}
	defer renderer.api.destroyShaderModule(renderer.device, lcdFragment, nil)
	lcdStages := stages
	lcdStages[1].Module = lcdFragment
	lcdAttachment := blendAttachment
	lcdAttachment.DestColor, lcdAttachment.DestAlpha = vkBlendOneMinusSource1Color, vkBlendOneMinusSource1Alpha
	lcdBlend := blend
	lcdBlend.Attachments = &lcdAttachment
	lcdInfo := info
	lcdInfo.Stages, lcdInfo.ColorBlend = &lcdStages[0], &lcdBlend
	return vkCheck(renderer.api.createGraphicsPipelines(renderer.device, 0, 1, &lcdInfo, nil, &textState.lcdPipeline), "create LCD text pipeline")
}

func sameTextRuns(left, right []ui.TextRun) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (label *vulkanLabelTexture) update(renderer *vulkanRenderer, next platform.TextLabel) {
	if labelContentChanged(label.label, next) {
		label.closeImage(renderer)
	}
	label.runs = append(label.runs[:0], next.Runs...)
	label.label = next
	label.label.Runs = label.runs
}

func labelContentChanged(previous, next platform.TextLabel) bool {
	bitmapChanged := len(previous.Bitmap) > 0 || len(next.Bitmap) > 0
	return previous.Value != next.Value || (!bitmapChanged && (previous.Width != next.Width || previous.Height != next.Height)) || previous.FontSize != next.FontSize ||
		previous.Weight != next.Weight || previous.Color != next.Color || previous.Mono != next.Mono || previous.MonoFace != next.MonoFace || previous.CellWidth != next.CellWidth ||
		!sameTextRuns(previous.Runs, next.Runs) || previous.BitmapID != next.BitmapID || previous.BitmapVersion != next.BitmapVersion ||
		previous.BitmapWidth != next.BitmapWidth || previous.BitmapHeight != next.BitmapHeight || previous.BitmapStride != next.BitmapStride ||
		!sameBitmapBytes(previous.Bitmap, next.Bitmap)
}

func sameBitmapBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	return len(left) == 0 || &left[0] == &right[0]
}

func (textState *vulkanText) update(renderer *vulkanRenderer, labels []platform.TextLabel) error {
	if len(labels) > vkMaximumTextLabels {
		return fmt.Errorf("linux: text label count %d exceeds limit %d", len(labels), vkMaximumTextLabels)
	}
	for index := len(labels); index < len(textState.labels); index++ {
		textState.labels[index].closeImage(renderer)
	}
	if len(labels) > cap(textState.labels) {
		textState.labels = append(textState.labels, make([]vulkanLabelTexture, len(labels)-len(textState.labels))...)
	} else {
		textState.labels = textState.labels[:len(labels)]
	}
	for index, label := range labels {
		if err := validateTextLabel(index, label); err != nil {
			return err
		}
		textState.labels[index].update(renderer, label)
	}
	textState.order = groupTextLayers(textState.labels, textState.order)
	return nil
}

func validateTextLabel(index int, label platform.TextLabel) error {
	for _, value := range []float32{label.X, label.Y, label.Width, label.Height, label.FontSize, label.Weight, label.CellWidth} {
		if !finite32(value) {
			return fmt.Errorf("linux: text label %d has a non-finite metric", index)
		}
	}
	if label.Width < 0 || label.Height < 0 || len(label.Bitmap) == 0 && !validFontSize(label.FontSize) {
		return fmt.Errorf("linux: text label %d has invalid bounds or font size", index)
	}
	if len(label.Runs) > 0 && label.CellWidth <= 0 {
		return fmt.Errorf("linux: text label %d has invalid cell width", index)
	}
	if len(label.Bitmap) > 0 {
		if label.BitmapWidth <= 0 || label.BitmapHeight <= 0 || label.BitmapStride < label.BitmapWidth*4 {
			return fmt.Errorf("linux: bitmap label %d has invalid dimensions", index)
		}
		required := (label.BitmapHeight-1)*label.BitmapStride + label.BitmapWidth*4
		if required < 0 || len(label.Bitmap) < required {
			return fmt.Errorf("linux: bitmap label %d has a short payload", index)
		}
	}
	return nil
}

func (textState *vulkanText) upload(renderer *vulkanRenderer, scale float32) error {
	if textState.services == nil || renderer.uploadCommand == 0 {
		return nil
	}
	dirty := false
	for index := range textState.labels {
		label := &textState.labels[index]
		if (label.label.Value != "" || len(label.label.Bitmap) > 0) && label.label.Width > 0 && label.label.Height > 0 && (label.view == 0 || label.scale != scale) {
			dirty = true
			break
		}
	}
	if !dirty {
		return nil
	}
	begin := vkCommandBufferBeginInfo{SType: vkStructureCommandBeginInfo, Flags: vkCommandOneTimeSubmit}
	if err := vkCheck(renderer.api.beginCommandBuffer(renderer.uploadCommand, &begin), "begin text upload"); err != nil {
		return err
	}
	staging := make([]vulkanStagingBuffer, 0, len(textState.labels))
	closeStaging := func() {
		for index := range staging {
			staging[index].close(renderer)
		}
	}
	defer closeStaging()
	for index := range textState.labels {
		label := &textState.labels[index]
		if label.label.Value == "" && len(label.label.Bitmap) == 0 || label.label.Width <= 0 || label.label.Height <= 0 || label.view != 0 && label.scale == scale {
			continue
		}
		width := int(math.Ceil(float64(label.label.Width * scale)))
		height := int(math.Ceil(float64(label.label.Height * scale)))
		var pixels []byte
		lcd := false
		var spans []colorSpan
		if len(label.label.Bitmap) > 0 {
			width, height = label.label.BitmapWidth, label.label.BitmapHeight
			shown, high := platform.DeviceExtent(label.label.X, label.label.Width, scale), platform.DeviceExtent(label.label.Y, label.label.Height, scale)
			if shown > 0 && high > 0 && shown <= width && high <= height && (shown < width || high < height) {
				// A bitmap drawn smaller than it is stored is averaged down to the device
				// pixels it covers; sampling a few of its texels would alias.
				pixels = platform.ResampleBGRA(label.label.Bitmap, label.label.BitmapStride, width, height, shown, high)
				width, height = shown, high
			}
			if pixels == nil {
				pixels = tightBitmap(label.label.Bitmap, width, height, label.label.BitmapStride)
			}
		} else {
			var err error
			// ClearType-style blending needs an opaque target: a window or an offscreen
			// image cleared opaque.
			pixels, lcd, err = textState.services.rasterize(label.label, scale, width, height, renderer.dualSource && textState.lcdPipeline != 0 && renderer.color.Alpha >= 1)
			if err != nil {
				return fmt.Errorf("linux: rasterize label %d: %w", index, err)
			}
			spans = textState.services.spans
		}
		stage, err := label.createImage(renderer, textState, renderer.uploadCommand, pixels, uint32(width), uint32(height))
		if err != nil {
			return fmt.Errorf("linux: upload label %d: %w", index, err)
		}
		staging = append(staging, stage)
		label.scale, label.lcd = scale, lcd
		label.width, label.height = width, height
		label.spans = append(label.spans[:0], spans...) // after createImage, which clears the label
	}
	if err := vkCheck(renderer.api.endCommandBuffer(renderer.uploadCommand), "end text upload"); err != nil {
		return err
	}
	command := renderer.uploadCommand
	submit := vkSubmitInfo{SType: vkStructureSubmitInfo, CommandCount: 1, Commands: &command}
	if err := vkCheck(renderer.api.queueSubmit(renderer.queue, 1, &submit, 0), "submit text upload"); err != nil {
		return err
	}
	return vkCheck(renderer.api.deviceWaitIdle(renderer.device), "wait for text upload")
}

func tightBitmap(source []byte, width, height, stride int) []byte {
	if stride == width*4 {
		return source[:width*height*4]
	}
	pixels := make([]byte, width*height*4)
	for row := 0; row < height; row++ {
		copy(pixels[row*width*4:(row+1)*width*4], source[row*stride:row*stride+width*4])
	}
	return pixels
}

func (label *vulkanLabelTexture) createImage(renderer *vulkanRenderer, textState *vulkanText, command uintptr, pixels []byte, width, height uint32) (vulkanStagingBuffer, error) {
	label.closeImage(renderer)
	stage, err := createTextStaging(renderer, pixels)
	if err != nil {
		return vulkanStagingBuffer{}, err
	}
	failed := true
	defer func() {
		if failed {
			stage.close(renderer)
			label.closeImage(renderer)
		}
	}()
	imageInfo := vkImageCreateInfo{
		SType: vkStructureTextImageCreateInfo, ImageType: vkImageType2D, Format: vkFormatBGRA8,
		Extent: vkExtent3D{Width: width, Height: height, Depth: 1}, MipLevels: 1, ArrayLayers: 1,
		Samples: vkSampleCountOne, Tiling: vkImageTilingOptimal,
		Usage: vkImageUsageTransferDestination | vkImageUsageSampled, SharingMode: vkSharingExclusive,
	}
	if err := vkCheck(renderer.api.createImage(renderer.device, &imageInfo, nil, &label.image), "create text image"); err != nil {
		return vulkanStagingBuffer{}, err
	}
	var requirements vkMemoryRequirements
	renderer.api.getImageMemoryRequirements(renderer.device, label.image, &requirements)
	memoryType, ok := renderer.memoryType(requirements.MemoryTypeBits, vkMemoryDeviceLocal)
	if !ok {
		return vulkanStagingBuffer{}, fmt.Errorf("Vulkan device has no device-local image memory")
	}
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &label.memory), "allocate text image memory"); err != nil {
		return vulkanStagingBuffer{}, err
	}
	if err := vkCheck(renderer.api.bindImageMemory(renderer.device, label.image, label.memory, 0), "bind text image memory"); err != nil {
		return vulkanStagingBuffer{}, err
	}
	barrier := vkImageMemoryBarrier{
		SType: vkStructureImageMemoryBarrier, OldLayout: vkImageLayoutUndefined, NewLayout: vkImageLayoutTransferDestination,
		SourceQueueFamily: vkQueueFamilyIgnored, DestinationQueueFamily: vkQueueFamilyIgnored, Image: label.image,
		Range: vkSubresourceRange{Aspect: vkImageAspectColor, LevelCount: 1, LayerCount: 1}, DestinationAccess: vkAccessTransferWrite,
	}
	renderer.api.cmdPipelineBarrier(command, vkPipelineStageTop, vkPipelineStageTransfer, 0, 0, nil, 0, nil, 1, &barrier)
	region := vkBufferImageCopy{ImageSubresource: vkImageSubresourceLayers{Aspect: vkImageAspectColor, LayerCount: 1}, ImageExtent: vkExtent3D{Width: width, Height: height, Depth: 1}}
	renderer.api.cmdCopyBufferToImage(command, stage.buffer, label.image, vkImageLayoutTransferDestination, 1, &region)
	barrier.SourceAccess = vkAccessTransferWrite
	barrier.DestinationAccess = vkAccessShaderRead
	barrier.OldLayout = vkImageLayoutTransferDestination
	barrier.NewLayout = vkImageLayoutShaderReadOnly
	renderer.api.cmdPipelineBarrier(command, vkPipelineStageTransfer, vkPipelineStageFragment, 0, 0, nil, 0, nil, 1, &barrier)
	viewInfo := vkImageViewCreateInfo{SType: vkStructureImageViewCreateInfo, Image: label.image, ViewType: vkImageViewType2D, Format: vkFormatBGRA8, Range: vkSubresourceRange{Aspect: vkImageAspectColor, LevelCount: 1, LayerCount: 1}}
	if err := vkCheck(renderer.api.createImageView(renderer.device, &viewInfo, nil, &label.view), "create text image view"); err != nil {
		return vulkanStagingBuffer{}, err
	}
	if label.set == 0 {
		allocate := vkDescriptorSetAllocateInfo{SType: vkStructureDescriptorSetAllocateInfo, Pool: textState.descriptorPool, SetCount: 1, SetLayouts: &textState.descriptorLayout}
		if err := vkCheck(renderer.api.allocateDescriptorSets(renderer.device, &allocate, &label.set), "allocate text descriptor"); err != nil {
			return vulkanStagingBuffer{}, err
		}
	}
	imageDescriptor := vkDescriptorImageInfo{Sampler: textState.sampler, ImageView: label.view, ImageLayout: vkImageLayoutShaderReadOnly}
	write := vkWriteDescriptorSet{SType: vkStructureWriteDescriptorSet, DestinationSet: label.set, DescriptorCount: 1, DescriptorType: vkDescriptorCombinedImageSampler, ImageInfo: &imageDescriptor}
	renderer.api.updateDescriptorSets(renderer.device, 1, &write, 0, nil)
	failed = false
	return stage, nil
}

func createTextStaging(renderer *vulkanRenderer, pixels []byte) (vulkanStagingBuffer, error) {
	var stage vulkanStagingBuffer
	info := vkBufferCreateInfo{SType: vkStructureBufferCreateInfo, Size: uint64(len(pixels)), Usage: vkBufferUsageTransferSource, SharingMode: vkSharingExclusive}
	if err := vkCheck(renderer.api.createBuffer(renderer.device, &info, nil, &stage.buffer), "create text staging buffer"); err != nil {
		return stage, err
	}
	var requirements vkMemoryRequirements
	renderer.api.getBufferMemoryRequirements(renderer.device, stage.buffer, &requirements)
	memoryType, ok := renderer.memoryType(requirements.MemoryTypeBits, vkMemoryHostVisible|vkMemoryHostCoherent)
	if !ok {
		stage.close(renderer)
		return stage, fmt.Errorf("Vulkan device has no coherent staging memory")
	}
	allocation := vkMemoryAllocateInfo{SType: vkStructureMemoryAllocateInfo, AllocationSize: requirements.Size, MemoryTypeIndex: memoryType}
	if err := vkCheck(renderer.api.allocateMemory(renderer.device, &allocation, nil, &stage.memory), "allocate text staging memory"); err != nil {
		stage.close(renderer)
		return stage, err
	}
	if err := vkCheck(renderer.api.bindBufferMemory(renderer.device, stage.buffer, stage.memory, 0), "bind text staging memory"); err != nil {
		stage.close(renderer)
		return stage, err
	}
	var mapped unsafe.Pointer
	if err := vkCheck(renderer.api.mapMemory(renderer.device, stage.memory, 0, uint64(len(pixels)), 0, &mapped), "map text staging memory"); err != nil {
		stage.close(renderer)
		return stage, err
	}
	copy(unsafe.Slice((*byte)(mapped), len(pixels)), pixels)
	runtime.KeepAlive(pixels)
	renderer.api.unmapMemory(renderer.device, stage.memory)
	return stage, nil
}

func (stage *vulkanStagingBuffer) close(renderer *vulkanRenderer) {
	if stage.buffer != 0 {
		renderer.api.destroyBuffer(renderer.device, stage.buffer, nil)
	}
	if stage.memory != 0 {
		renderer.api.freeMemory(renderer.device, stage.memory, nil)
	}
	stage.buffer, stage.memory = 0, 0
}

func (textState *vulkanText) record(renderer *vulkanRenderer, command uintptr, extent vkExtent2D, index int) {
	if index < 0 || index >= len(textState.labels) {
		return
	}
	label := &textState.labels[index]
	if label.view == 0 || label.set == 0 || label.label.Value == "" && len(label.label.Bitmap) == 0 || label.label.Width <= 0 || label.label.Height <= 0 {
		return
	}
	pipeline := textState.pipeline
	if label.lcd && textState.lcdPipeline != 0 {
		pipeline = textState.lcdPipeline
	}
	scratch := &renderer.scratch
	renderer.api.cmdBindPipeline(command, vkPipelineBindPointGraphics, pipeline)
	// Like the scene, text is drawn in the content rectangle of the target.
	originX, originY := scratch.contentX, scratch.contentY
	scratch.viewport = vkViewport{X: float32(originX), Y: float32(originY), Width: float32(extent.Width), Height: float32(extent.Height), MaximumDepth: 1}
	scale := renderer.pixelScale()
	scissor, visible := labelScissor(label.label, scale, extent)
	if !visible {
		return
	}
	scissor.Offset[0] += originX
	scissor.Offset[1] += originY
	scratch.scissor = scissor
	renderer.api.cmdSetViewport(command, 0, 1, &scratch.viewport)
	renderer.api.cmdSetScissor(command, 0, 1, &scratch.scissor)
	quad := label.quad(scale)
	scratch.textFrame = [8]float32{float32(extent.Width) / scale, float32(extent.Height) / scale, quad[0], quad[1], quad[2], quad[3]}
	renderer.api.cmdPushConstants(command, textState.pipelineLayout, vkShaderStageVertex, 0, uint32(unsafe.Sizeof(scratch.textFrame)), unsafe.Pointer(&scratch.textFrame[0]))
	renderer.api.cmdBindDescriptorSets(command, vkPipelineBindPointGraphics, textState.pipelineLayout, 0, 1, &label.set, 0, nil)
	if !label.lcd || textState.lcdPipeline == 0 {
		renderer.api.cmdDraw(command, 6, 1, 0, 0)
		return
	}
	scratch.tint = [4]float32{float32(label.label.Color.Red), float32(label.label.Color.Green), float32(label.label.Color.Blue), float32(label.label.Color.Alpha)}
	if len(label.spans) == 0 {
		renderer.api.cmdPushConstants(command, textState.pipelineLayout, vkShaderStageFragment, 32, uint32(unsafe.Sizeof(scratch.tint)), unsafe.Pointer(&scratch.tint[0]))
		renderer.api.cmdDraw(command, 6, 1, 0, 0)
		return
	}
	// A grid line has a color per run: its coverage is drawn once for each, cut to the
	// columns of the run.
	origin, _ := label.pixelOrigin(scale)
	for spanIndex := range label.spans {
		span := &label.spans[spanIndex]
		scratch.cut = scratch.scissor
		left := max(int64(scratch.cut.Offset[0]), int64(originX)+int64(origin+span.left))
		right := min(int64(scratch.cut.Offset[0])+int64(scratch.cut.Extent.Width), int64(originX)+int64(origin+span.right))
		if right <= left {
			continue
		}
		scratch.cut.Offset[0], scratch.cut.Extent.Width = int32(left), uint32(right-left)
		renderer.api.cmdSetScissor(command, 0, 1, &scratch.cut)
		renderer.api.cmdPushConstants(command, textState.pipelineLayout, vkShaderStageFragment, 32, uint32(unsafe.Sizeof(span.color)), unsafe.Pointer(&span.color[0]))
		renderer.api.cmdDraw(command, 6, 1, 0, 0)
	}
}

// quad is where the label's texture is drawn, in logical pixels: its top left and its size are
// whole device pixels, so every texel lands on one pixel and the sampler blends none with its
// neighbour. Text takes the texture's own size; an image takes the device pixels
// its logical bounds cover.
func (label *vulkanLabelTexture) quad(scale float32) [4]float32 {
	l := label.label
	x, y := label.pixelOrigin(scale)
	width, height := label.width, label.height
	if len(l.Bitmap) > 0 {
		width, height = platform.DeviceExtent(l.X, l.Width, scale), platform.DeviceExtent(l.Y, l.Height, scale)
	}
	return [4]float32{float32(x) / scale, float32(y) / scale, float32(width) / scale, float32(height) / scale}
}

// pixelOrigin is the device pixel the label's texture starts at.
func (label *vulkanLabelTexture) pixelOrigin(scale float32) (x, y int) {
	l := label.label
	if len(l.Bitmap) > 0 {
		return platform.DevicePosition(l.X, 0, scale), platform.DevicePosition(l.Y, 0, scale)
	}
	return platform.DevicePosition(l.X, l.OriginX, scale), platform.DevicePosition(l.Y, l.OriginY, scale)
}

// labelScissor is the part of the target a label may paint: all of it, or the
// label's clip when a canvas clip cuts it partway. visible is false when that
// clip leaves no pixel. Every draw sets its own scissor, so a clipped label
// does not leak into the next.
func labelScissor(label platform.TextLabel, scale float32, extent vkExtent2D) (scissor vkRect2D, visible bool) {
	if !label.Clipped {
		return vkRect2D{Extent: extent}, true
	}
	// The clip is in logical pixels; the label's glyphs are partial coverage at
	// its edge, so the scissor takes every device pixel the clip touches.
	left := max(int64(math.Floor(float64(label.Clip.X*scale))), 0)
	top := max(int64(math.Floor(float64(label.Clip.Y*scale))), 0)
	right := min(int64(math.Ceil(float64((label.Clip.X+label.Clip.Width)*scale))), int64(extent.Width))
	bottom := min(int64(math.Ceil(float64((label.Clip.Y+label.Clip.Height)*scale))), int64(extent.Height))
	if right <= left || bottom <= top {
		return vkRect2D{}, false
	}
	return vkRect2D{
		Offset: [2]int32{int32(left), int32(top)},
		Extent: vkExtent2D{Width: uint32(right - left), Height: uint32(bottom - top)},
	}, true
}

func (label *vulkanLabelTexture) closeImage(renderer *vulkanRenderer) {
	if renderer == nil || renderer.api == nil || renderer.device == 0 {
		label.image, label.memory, label.view, label.scale, label.lcd = 0, 0, 0, 0, false
		label.width, label.height, label.spans = 0, 0, label.spans[:0]
		return
	}
	if label.view != 0 {
		renderer.api.destroyImageView(renderer.device, label.view, nil)
	}
	if label.image != 0 {
		renderer.api.destroyImage(renderer.device, label.image, nil)
	}
	if label.memory != 0 {
		renderer.api.freeMemory(renderer.device, label.memory, nil)
	}
	label.image, label.memory, label.view, label.scale, label.lcd = 0, 0, 0, 0, false
	label.width, label.height, label.spans = 0, 0, label.spans[:0]
}

func (textState *vulkanText) destroyPipeline(renderer *vulkanRenderer) {
	if textState.pipeline != 0 {
		renderer.api.destroyPipeline(renderer.device, textState.pipeline, nil)
	}
	if textState.lcdPipeline != 0 {
		renderer.api.destroyPipeline(renderer.device, textState.lcdPipeline, nil)
	}
	if textState.pipelineLayout != 0 {
		renderer.api.destroyPipelineLayout(renderer.device, textState.pipelineLayout, nil)
	}
	textState.pipeline, textState.lcdPipeline, textState.pipelineLayout = 0, 0, 0
}

func (textState *vulkanText) close(renderer *vulkanRenderer) {
	textState.destroyPipeline(renderer)
	for index := range textState.labels {
		textState.labels[index].closeImage(renderer)
	}
	if renderer != nil && renderer.api != nil && renderer.device != 0 {
		if textState.descriptorPool != 0 {
			renderer.api.destroyDescriptorPool(renderer.device, textState.descriptorPool, nil)
		}
		if textState.sampler != 0 {
			renderer.api.destroySampler(renderer.device, textState.sampler, nil)
		}
		if textState.descriptorLayout != 0 {
			renderer.api.destroyDescriptorSetLayout(renderer.device, textState.descriptorLayout, nil)
		}
	}
	if textState.services != nil {
		textState.services.close()
	}
	textState.services = nil
	textState.descriptorPool, textState.sampler, textState.descriptorLayout = 0, 0, 0
	textState.labels, textState.order = nil, nil
}

// release destroys the text objects that belong to the device and the text
// services, and keeps the labels' descriptions: each label is uploaded again
// the next time the textures are made.
func (textState *vulkanText) release(renderer *vulkanRenderer) {
	textState.destroyPipeline(renderer)
	for index := range textState.labels {
		textState.labels[index].closeImage(renderer)
		// The sets went with the pool below.
		textState.labels[index].set = 0
	}
	if renderer.api != nil && renderer.device != 0 {
		if textState.descriptorPool != 0 {
			renderer.api.destroyDescriptorPool(renderer.device, textState.descriptorPool, nil)
		}
		if textState.sampler != 0 {
			renderer.api.destroySampler(renderer.device, textState.sampler, nil)
		}
		if textState.descriptorLayout != 0 {
			renderer.api.destroyDescriptorSetLayout(renderer.device, textState.descriptorLayout, nil)
		}
	}
	if textState.services != nil {
		textState.services.close()
	}
	textState.services = nil
	textState.descriptorPool, textState.sampler, textState.descriptorLayout = 0, 0, 0
}
