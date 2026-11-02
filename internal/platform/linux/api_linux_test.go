package linux

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestNativeStructureABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Fatal("only Linux amd64 and arm64 are supported")
	}
	for _, layout := range []struct {
		name string
		size uintptr
		want uintptr
	}{
		{"xcb_screen_t", unsafe.Sizeof(xcbScreen{}), 40},
		{"xcb_screen_iterator_t", unsafe.Sizeof(xcbScreenIterator{}), 16},
		{"xcb_setup_t", unsafe.Sizeof(xcbSetup{}), 40},
		{"xcb_generic_event_t", unsafe.Sizeof(xcbGenericEvent{}), 32},
		{"xcb_key_press_event_t", unsafe.Sizeof(xcbInputEvent{}), 32},
		{"xcb_mapping_notify_event_t", unsafe.Sizeof(xcbMappingEvent{}), 32},
		{"xcb_get_keyboard_mapping_reply_t", unsafe.Sizeof(xcbKeyboardMappingReply{}), 32},
		{"xcb_configure_notify_event_t", unsafe.Sizeof(xcbConfigureEvent{}), 28},
		{"xcb_client_message_event_t", unsafe.Sizeof(xcbClientEvent{}), 32},
		{"xcb_focus_out_event_t", unsafe.Sizeof(xcbFocusEvent{}), 32},
		{"xcb_expose_event_t", unsafe.Sizeof(xcbExposeEvent{}), 32},
		{"xcb_selection_owner_reply_t", unsafe.Sizeof(xcbSelectionOwnerReply{}), 12},
		{"xcb_get_property_reply_t", unsafe.Sizeof(xcbPropertyReply{}), 32},
		{"xcb_selection_request_event_t", unsafe.Sizeof(xcbSelectionRequestEvent{}), 28},
		{"xcb_selection_notify_event_t", unsafe.Sizeof(xcbSelectionNotifyEvent{}), 24},
		{"xcb_selection_clear_event_t", unsafe.Sizeof(xcbSelectionClearEvent{}), 16},
		{"xcb_selection_notify_wire", unsafe.Sizeof(xcbSelectionNotifyWire{}), 32},
		{"pollfd", unsafe.Sizeof(pollFD{}), 8},
		{"xcb_translate_coordinates_reply_t", unsafe.Sizeof(xcbTranslateReply{}), 16},
		{"VkApplicationInfo", unsafe.Sizeof(vkApplicationInfo{}), 48},
		{"VkInstanceCreateInfo", unsafe.Sizeof(vkInstanceCreateInfo{}), 64},
		{"VkXcbSurfaceCreateInfoKHR", unsafe.Sizeof(vkXcbSurfaceCreateInfo{}), 40},
		{"VkQueueFamilyProperties", unsafe.Sizeof(vkQueueFamilyProperties{}), 24},
		{"VkDeviceQueueCreateInfo", unsafe.Sizeof(vkDeviceQueueCreateInfo{}), 40},
		{"VkDeviceCreateInfo", unsafe.Sizeof(vkDeviceCreateInfo{}), 72},
		{"VkSurfaceCapabilitiesKHR", unsafe.Sizeof(vkSurfaceCapabilities{}), 52},
		{"VkSwapchainCreateInfoKHR", unsafe.Sizeof(vkSwapchainCreateInfo{}), 104},
		{"VkImageViewCreateInfo", unsafe.Sizeof(vkImageViewCreateInfo{}), 80},
		{"VkAttachmentDescription", unsafe.Sizeof(vkAttachmentDescription{}), 36},
		{"VkSubpassDescription", unsafe.Sizeof(vkSubpassDescription{}), 72},
		{"VkRenderPassCreateInfo", unsafe.Sizeof(vkRenderPassCreateInfo{}), 64},
		{"VkFramebufferCreateInfo", unsafe.Sizeof(vkFramebufferCreateInfo{}), 64},
		{"VkCommandPoolCreateInfo", unsafe.Sizeof(vkCommandPoolCreateInfo{}), 24},
		{"VkCommandBufferAllocateInfo", unsafe.Sizeof(vkCommandBufferAllocateInfo{}), 32},
		{"VkCommandBufferBeginInfo", unsafe.Sizeof(vkCommandBufferBeginInfo{}), 32},
		{"VkClearAttachment", unsafe.Sizeof(vkClearAttachment{}), 24},
		{"VkClearRect", unsafe.Sizeof(vkClearRect{}), 24},
		{"VkRenderPassBeginInfo", unsafe.Sizeof(vkRenderPassBeginInfo{}), 64},
		{"VkSemaphoreCreateInfo", unsafe.Sizeof(vkSemaphoreCreateInfo{}), 24},
		{"VkSubmitInfo", unsafe.Sizeof(vkSubmitInfo{}), 72},
		{"VkPresentInfoKHR", unsafe.Sizeof(vkPresentInfo{}), 64},
		{"VkPhysicalDeviceMemoryProperties", unsafe.Sizeof(vkPhysicalDeviceMemoryProperties{}), 520},
		{"VkMemoryRequirements", unsafe.Sizeof(vkMemoryRequirements{}), 24},
		{"VkBufferCreateInfo", unsafe.Sizeof(vkBufferCreateInfo{}), 56},
		{"VkMemoryAllocateInfo", unsafe.Sizeof(vkMemoryAllocateInfo{}), 32},
		{"VkShaderModuleCreateInfo", unsafe.Sizeof(vkShaderModuleCreateInfo{}), 40},
		{"VkPipelineShaderStageCreateInfo", unsafe.Sizeof(vkPipelineShaderStageCreateInfo{}), 48},
		{"VkVertexInputBindingDescription", unsafe.Sizeof(vkVertexInputBindingDescription{}), 12},
		{"VkVertexInputAttributeDescription", unsafe.Sizeof(vkVertexInputAttributeDescription{}), 16},
		{"VkPipelineVertexInputStateCreateInfo", unsafe.Sizeof(vkPipelineVertexInputStateCreateInfo{}), 48},
		{"VkPipelineInputAssemblyStateCreateInfo", unsafe.Sizeof(vkPipelineInputAssemblyStateCreateInfo{}), 32},
		{"VkPipelineViewportStateCreateInfo", unsafe.Sizeof(vkPipelineViewportStateCreateInfo{}), 48},
		{"VkPipelineRasterizationStateCreateInfo", unsafe.Sizeof(vkPipelineRasterizationStateCreateInfo{}), 64},
		{"VkPipelineMultisampleStateCreateInfo", unsafe.Sizeof(vkPipelineMultisampleStateCreateInfo{}), 48},
		{"VkPipelineColorBlendAttachmentState", unsafe.Sizeof(vkPipelineColorBlendAttachmentState{}), 32},
		{"VkPipelineColorBlendStateCreateInfo", unsafe.Sizeof(vkPipelineColorBlendStateCreateInfo{}), 56},
		{"VkPipelineDynamicStateCreateInfo", unsafe.Sizeof(vkPipelineDynamicStateCreateInfo{}), 32},
		{"VkPushConstantRange", unsafe.Sizeof(vkPushConstantRange{}), 12},
		{"VkPipelineLayoutCreateInfo", unsafe.Sizeof(vkPipelineLayoutCreateInfo{}), 48},
		{"VkGraphicsPipelineCreateInfo", unsafe.Sizeof(vkGraphicsPipelineCreateInfo{}), 144},
		{"VkViewport", unsafe.Sizeof(vkViewport{}), 24},
		{"VkExtent3D", unsafe.Sizeof(vkExtent3D{}), 12},
		{"VkImageCreateInfo", unsafe.Sizeof(vkImageCreateInfo{}), 88},
		{"VkSamplerCreateInfo", unsafe.Sizeof(vkSamplerCreateInfo{}), 80},
		{"VkDescriptorSetLayoutBinding", unsafe.Sizeof(vkDescriptorSetLayoutBinding{}), 24},
		{"VkDescriptorSetLayoutCreateInfo", unsafe.Sizeof(vkDescriptorSetLayoutCreateInfo{}), 32},
		{"VkDescriptorPoolSize", unsafe.Sizeof(vkDescriptorPoolSize{}), 8},
		{"VkDescriptorPoolCreateInfo", unsafe.Sizeof(vkDescriptorPoolCreateInfo{}), 40},
		{"VkDescriptorSetAllocateInfo", unsafe.Sizeof(vkDescriptorSetAllocateInfo{}), 40},
		{"VkDescriptorImageInfo", unsafe.Sizeof(vkDescriptorImageInfo{}), 24},
		{"VkWriteDescriptorSet", unsafe.Sizeof(vkWriteDescriptorSet{}), 64},
		{"VkImageMemoryBarrier", unsafe.Sizeof(vkImageMemoryBarrier{}), 72},
		{"VkImageSubresourceLayers", unsafe.Sizeof(vkImageSubresourceLayers{}), 16},
		{"VkBufferImageCopy", unsafe.Sizeof(vkBufferImageCopy{}), 56},
		{"scene instance", unsafe.Sizeof(vulkanRectangleInstance{}), 80},
	} {
		if layout.size != layout.want {
			t.Errorf("%s size %d, want %d", layout.name, layout.size, layout.want)
		}
	}
	if offset := unsafe.Offsetof(vkImageCreateInfo{}.QueueFamilies); offset != 72 {
		t.Errorf("VkImageCreateInfo.pQueueFamilyIndices offset %d, want 72", offset)
	}
	if offset := unsafe.Offsetof(vkImageCreateInfo{}.InitialLayout); offset != 80 {
		t.Errorf("VkImageCreateInfo.initialLayout offset %d, want 80", offset)
	}
}

func TestClamp(t *testing.T) {
	if clamp(1, 2, 4) != 2 || clamp(3, 2, 4) != 3 || clamp(5, 2, 4) != 4 {
		t.Fatal("clamp result mismatch")
	}
}

func TestVulkanResult(t *testing.T) {
	if vkCheck(vkSuccess, "test") != nil || vkCheck(-1, "test") == nil {
		t.Fatal("Vulkan result classification mismatch")
	}
}

func TestValidateConfig(t *testing.T) {
	valid := platform.Config{
		Width:      960,
		Height:     600,
		WindowMode: platform.WindowTitled,
		Rectangles: []render.Rectangle{{Color: scene.Color{Alpha: 1}}},
		OnPointer:  func(platform.PointerEvent) (platform.Frame, error) { return platform.Frame{}, nil },
		OnResize:   func(platform.Size) (platform.Frame, error) { return platform.Frame{}, nil },
		OnKey:      func(platform.KeyEvent) (platform.Frame, error) { return platform.Frame{}, nil },
		OnScroll:   func(platform.ScrollEvent) (platform.Frame, error) { return platform.Frame{}, nil },
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	valid.Rectangles = []render.Rectangle{
		{Bounds: scene.Rect{Width: 24, Height: 12}, Radius: 4, Color: scene.Color{Alpha: 0.5}},
		{Kind: scene.KindTriangle, Points: [3]scene.Point{{}, {X: 10}, {Y: 10}}, EdgeMask: 7, Color: scene.Color{Alpha: 0.5}},
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("rounded translucent scene: %v", err)
	}
	valid.TextLabels = []platform.TextLabel{{Value: "Viva 原生界面", Width: 200, Height: 28, FontSize: 16, Color: platform.Color{Alpha: 1}}}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("system text label: %v", err)
	}
	valid.WindowMode = platform.WindowCustomTitlebar
	valid.TitlebarHeight = 42
	if err := validateConfig(valid); err != nil {
		t.Fatalf("custom titlebar: %v", err)
	}
	valid.WindowMode = platform.WindowTitled
	for _, config := range []platform.Config{
		{Width: 0, Height: 600, WindowMode: platform.WindowTitled},
		{Width: 960, Height: 1 << 16, WindowMode: platform.WindowTitled},
		{Width: 960, Height: 600, WindowMode: platform.WindowMode(99)},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, TextLabels: make([]platform.TextLabel, 1)},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, Rectangles: []render.Rectangle{{Radius: -1, Color: scene.Color{Alpha: 1}}}},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, Rectangles: []render.Rectangle{{Kind: scene.KindTriangle, EdgeMask: 8, Color: scene.Color{Alpha: 1}}}},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, Rectangles: []render.Rectangle{{Bounds: scene.Rect{X: float32(math.NaN())}, Color: scene.Color{Alpha: 1}}}},
	} {
		if err := validateConfig(config); err == nil {
			t.Fatalf("validateConfig(%+v) succeeded", config)
		}
	}
}

func TestRectangleClearClipsAndClamps(t *testing.T) {
	attachment, clear, visible := rectangleClear(render.Rectangle{
		Bounds: scene.Rect{X: -2.25, Y: 1.2, Width: 5, Height: 3.1},
		Color:  scene.Color{Red: -1, Green: 0.5, Blue: 2, Alpha: 1},
	}, 4, 4)
	if !visible {
		t.Fatal("partially visible rectangle was discarded")
	}
	if clear.Rect.Offset != [2]int32{0, 1} || clear.Rect.Extent != (vkExtent2D{Width: 3, Height: 3}) {
		t.Fatalf("clear bounds = %+v", clear.Rect)
	}
	if clear.BaseArrayLayer != 0 || clear.LayerCount != 1 {
		t.Fatalf("clear layers = %d..%d", clear.BaseArrayLayer, clear.LayerCount)
	}
	if attachment.AspectMask != vkImageAspectColor || attachment.ColorAttachment != 0 {
		t.Fatalf("clear attachment = %+v", attachment)
	}
	if attachment.ClearValue.Color != [4]float32{0, 0.5, 1, 1} {
		t.Fatalf("clear color = %v", attachment.ClearValue.Color)
	}

	for _, bounds := range []scene.Rect{
		{X: 4, Width: 1, Height: 1},
		{Y: 4, Width: 1, Height: 1},
		{Width: 0, Height: 1},
		{Width: 1, Height: -1},
	} {
		if _, _, visible := rectangleClear(render.Rectangle{Bounds: bounds}, 4, 4); visible {
			t.Fatalf("invisible bounds %+v produced a clear", bounds)
		}
	}
}

func TestRecordCommandsDrawsSceneInstances(t *testing.T) {
	var begins, renderBegins, renderEnds, ends, binds, draws int
	renderer := vulkanRenderer{
		api: &vulkanAPI{
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 {
				begins++
				return vkSuccess
			},
			cmdBeginRenderPass: func(uintptr, *vkRenderPassBeginInfo, uint32) {
				renderBegins++
			},
			cmdBindPipeline: func(_ uintptr, bindPoint uint32, pipeline uintptr) {
				if bindPoint != vkPipelineBindPointGraphics || pipeline != 7 {
					t.Fatalf("pipeline = %d, %d", bindPoint, pipeline)
				}
				binds++
			},
			cmdSetViewport:   func(uintptr, uint32, uint32, *vkViewport) {},
			cmdSetScissor:    func(uintptr, uint32, uint32, *vkRect2D) {},
			cmdPushConstants: func(uintptr, uintptr, uint32, uint32, uint32, unsafe.Pointer) {},
			cmdBindVertexBuffers: func(_ uintptr, _ uint32, count uint32, buffers *uintptr, offsets *uint64) {
				if count != 1 || *buffers != 8 || *offsets != 0 {
					t.Fatalf("vertex binding = %d, %d, %d", count, *buffers, *offsets)
				}
			},
			cmdDraw: func(_ uintptr, vertices, instances, firstVertex, firstInstance uint32) {
				if vertices != 6 || instances != 2 || firstVertex != 0 || firstInstance != 0 {
					t.Fatalf("draw = %d, %d, %d, %d", vertices, instances, firstVertex, firstInstance)
				}
				draws++
			},
			cmdEndRenderPass: func(uintptr) {
				renderEnds++
			},
			endCommandBuffer: func(uintptr) int32 {
				ends++
				return vkSuccess
			},
		},
		commands:           []uintptr{1, 2},
		framebuffers:       []uintptr{3, 4},
		scene:              vulkanScene{pipeline: 7, pipelineLayout: 9, buffer: 8, instances: make([]vulkanRectangleInstance, 2)},
		rectangleBatches:   []render.Batch{{First: 0, Count: 2}},
		compositingBatches: []compositingBatch{{Kind: compositeRectangles, First: 0, Count: 2}},
	}
	if err := renderer.recordCommands(vkExtent2D{Width: 4, Height: 4}); err != nil {
		t.Fatal(err)
	}
	if begins != 2 || renderBegins != 2 || renderEnds != 2 || ends != 2 {
		t.Fatalf("command lifecycle = %d %d %d %d", begins, renderBegins, renderEnds, ends)
	}
	if binds != 2 || draws != 2 {
		t.Fatalf("scene commands = %d binds, %d draws", binds, draws)
	}
}

func TestRecordCommandsScissorsClippedBatches(t *testing.T) {
	var events []string
	var scissor vkRect2D
	renderer := vulkanRenderer{
		api: &vulkanAPI{
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 { return vkSuccess },
			cmdBeginRenderPass: func(uintptr, *vkRenderPassBeginInfo, uint32) {},
			cmdBindPipeline:    func(uintptr, uint32, uintptr) {},
			cmdSetViewport:     func(uintptr, uint32, uint32, *vkViewport) {},
			cmdSetScissor: func(_ uintptr, _, _ uint32, rect *vkRect2D) {
				scissor = *rect
				events = append(events, fmt.Sprintf("scissor %d,%d %dx%d", rect.Offset[0], rect.Offset[1], rect.Extent.Width, rect.Extent.Height))
			},
			cmdPushConstants:     func(uintptr, uintptr, uint32, uint32, uint32, unsafe.Pointer) {},
			cmdBindVertexBuffers: func(uintptr, uint32, uint32, *uintptr, *uint64) {},
			cmdDraw: func(_ uintptr, _, _, _, first uint32) {
				events = append(events, fmt.Sprintf("draw %d with %d,%d %dx%d", first, scissor.Offset[0], scissor.Offset[1], scissor.Extent.Width, scissor.Extent.Height))
			},
			cmdEndRenderPass: func(uintptr) {},
			endCommandBuffer: func(uintptr) int32 { return vkSuccess },
		},
		commands:     []uintptr{1},
		framebuffers: []uintptr{2},
		scene:        vulkanScene{pipeline: 7, pipelineLayout: 9, buffer: 8, instances: make([]vulkanRectangleInstance, 3)},
		compositingBatches: []compositingBatch{
			{Kind: compositeRectangles, Clipped: true, Clip: scene.Rect{X: 2, Y: 3, Width: 4, Height: 5}, First: 0, Count: 1},
			{Kind: compositeRectangles, Clipped: true, Clip: scene.Rect{X: 100, Y: 100, Width: 4, Height: 4}, First: 1, Count: 1},
			{Kind: compositeRectangles, First: 2, Count: 1},
		},
	}
	if err := renderer.recordCommands(vkExtent2D{Width: 20, Height: 30}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"scissor 2,3 4x5", "draw 0 with 2,3 4x5",
		"scissor 0,0 20x30", "draw 2 with 0,0 20x30",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %q, want %q", events, want)
	}
}

func TestVulkanRectangleInstancesMatchNativeBackends(t *testing.T) {
	rectangle := makeVulkanRectangleInstance(render.Rectangle{
		Bounds: scene.Rect{X: 1, Y: 2, Width: 3, Height: 4},
		Color:  scene.Color{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 0.4}, Radius: 5,
	})
	if rectangle.Bounds != [4]float32{1, 2, 3, 4} || rectangle.Color != [4]float32{0.1, 0.2, 0.3, 0.4} || rectangle.Style != [4]float32{5, 0, 0, 0} {
		t.Fatalf("rectangle instance = %+v", rectangle)
	}
	triangle := makeVulkanRectangleInstance(render.Rectangle{
		Kind: scene.KindTriangle, Points: [3]scene.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}},
		Color: scene.Color{Alpha: 1}, Layer: 9, EdgeMask: 5,
	})
	if triangle.Bounds != [4]float32{1, 2, 3, 4} || triangle.Style != [4]float32{5, 9, 6, 6} {
		t.Fatalf("triangle instance = %+v", triangle)
	}
}

func TestVulkanSceneUploadWritesOnlyChangedRange(t *testing.T) {
	storage := make([]byte, 3*vkInstanceBytes)
	sceneState := vulkanScene{capacity: 3, mapped: unsafe.Pointer(&storage[0])}
	rectangles := []render.Rectangle{
		{Bounds: scene.Rect{X: 1, Width: 10}, Color: scene.Color{Alpha: 1}},
		{Bounds: scene.Rect{X: 2, Width: 20}, Color: scene.Color{Alpha: 1}},
		{Bounds: scene.Rect{X: 3, Width: 30}, Color: scene.Color{Alpha: 1}},
	}
	if err := sceneState.upload(&vulkanRenderer{}, rectangles, render.Update{Count: len(rectangles), Rebuild: true}); err != nil {
		t.Fatal(err)
	}
	first := append([]byte(nil), storage[:vkInstanceBytes]...)
	last := append([]byte(nil), storage[2*vkInstanceBytes:]...)
	rectangles[1].Bounds.Width = 99
	if err := sceneState.upload(&vulkanRenderer{}, rectangles, render.Update{First: 1, Count: 1}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storage[:vkInstanceBytes], first) || !bytes.Equal(storage[2*vkInstanceBytes:], last) {
		t.Fatal("partial upload changed untouched GPU instance ranges")
	}
	if sceneState.instances[1].Bounds[2] != 99 {
		t.Fatalf("updated instance width = %g", sceneState.instances[1].Bounds[2])
	}
}

func TestSwapchainStatusClassification(t *testing.T) {
	if swapchainNeedsRebuild(vkSuccess) || swapchainNeedsRebuild(vkSuboptimal) || !swapchainNeedsRebuild(vkErrorOutOfDate) {
		t.Fatal("unexpected swapchain status classification")
	}
}

func TestVulkanRectangleSmoke(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_VULKAN_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_VULKAN_SMOKE=1 with an X11 display and Vulkan driver")
	}
	t.Setenv("VIVA_TEST_AUTOCLOSE", "1")
	err := Run(platform.Config{
		Title:      "Viva Linux text smoke",
		Width:      160,
		Height:     100,
		WindowMode: platform.WindowTitled,
		ClearColor: platform.Color{Red: 0.05, Green: 0.05, Blue: 0.05, Alpha: 1},
		Rectangles: []render.Rectangle{
			{Bounds: scene.Rect{X: 8, Y: 8, Width: 96, Height: 56}, Radius: 12, Color: scene.Color{Red: 1, Alpha: 0.8}},
			{Bounds: scene.Rect{X: 48, Y: 32, Width: 96, Height: 56}, Radius: 18, Color: scene.Color{Blue: 1, Alpha: 0.8}},
			{Kind: scene.KindTriangle, Points: [3]scene.Point{{X: 20, Y: 90}, {X: 80, Y: 20}, {X: 140, Y: 90}}, Color: scene.Color{Green: 1, Alpha: 0.7}, EdgeMask: 7},
		},
		TextLabels: []platform.TextLabel{
			{Value: "Viva 原生界面", X: 16, Y: 10, Width: 132, Height: 24, FontSize: 16, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Layer: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSwapchainCleansPartialResources(t *testing.T) {
	var destroyed uintptr
	renderer := vulkanRenderer{
		api: &vulkanAPI{
			getSurfaceCapabilities: func(_ uintptr, _ uintptr, capabilities *vkSurfaceCapabilities) int32 {
				*capabilities = vkSurfaceCapabilities{
					MinImages:      1,
					MaxImages:      2,
					Current:        vkExtent2D{Width: 640, Height: 480},
					CompositeAlpha: vkCompositeOpaque,
					Usage:          vkImageUsageColorAttachment,
				}
				return vkSuccess
			},
			getSurfaceFormats: func(_ uintptr, _ uintptr, count *uint32, formats *vkSurfaceFormat) int32 {
				*count = 1
				if formats != nil {
					*formats = vkSurfaceFormat{Format: vkFormatBGRA8, ColorSpace: vkColorSpaceSRGB}
				}
				return vkSuccess
			},
			createSwapchain: func(_ uintptr, _ *vkSwapchainCreateInfo, _ unsafe.Pointer, swapchain *uintptr) int32 {
				*swapchain = 42
				return vkSuccess
			},
			getSwapchainImages: func(_ uintptr, _ uintptr, _ *uint32, _ *uintptr) int32 {
				return -1
			},
			destroySwapchain: func(_ uintptr, swapchain uintptr, _ unsafe.Pointer) {
				destroyed = swapchain
			},
		},
		physical: 1,
		device:   2,
		surface:  3,
		width:    640,
		height:   480,
	}

	if err := renderer.createSwapchain(); err == nil {
		t.Fatal("createSwapchain succeeded")
	}
	if destroyed != 42 {
		t.Fatalf("destroyed swapchain %d, want 42", destroyed)
	}
	if renderer.swapchain != 0 {
		t.Fatalf("swapchain retained after failure: %d", renderer.swapchain)
	}
}

// The fields the status-area code reads sit where the X11 protocol puts them: a
// wrong offset would read the wrong byte of an event without any error.
func TestStatusAreaEventFieldOffsets(t *testing.T) {
	for _, field := range []struct {
		name      string
		got, want uintptr
	}{
		{"FocusOut detail", unsafe.Offsetof(xcbFocusEvent{}.Detail), 1},
		{"FocusOut event window", unsafe.Offsetof(xcbFocusEvent{}.Event), 4},
		{"FocusOut mode", unsafe.Offsetof(xcbFocusEvent{}.Mode), 8},
		{"Expose window", unsafe.Offsetof(xcbExposeEvent{}.Window), 4},
		{"Expose count", unsafe.Offsetof(xcbExposeEvent{}.Count), 16},
	} {
		if field.got != field.want {
			t.Errorf("%s is at offset %d, want %d", field.name, field.got, field.want)
		}
	}
}

// A machine with no Vulkan reports it as platform.ErrNoGraphics, so an
// application can tell "this server has no graphics" from a real failure, and the
// text carries the cause and what to install.
func TestMissingVulkanIsNamedAndSaysWhatToInstall(t *testing.T) {
	cause := errors.New("libvulkan.so.1: cannot open shared object file")
	for name, err := range map[string]error{
		"no loader": noVulkan("libvulkan.so.1 is not installed", cause),
		"no device": noVulkan("Vulkan reports no device", nil),
	} {
		if !errors.Is(err, platform.ErrNoGraphics) {
			t.Errorf("%s: %v does not wrap platform.ErrNoGraphics", name, err)
		}
		if !strings.Contains(err.Error(), "mesa-vulkan-drivers") {
			t.Errorf("%s: %q does not say what to install", name, err)
		}
	}
	if err := noVulkan("x", cause); !errors.Is(err, cause) {
		t.Errorf("the cause is lost: %v", err)
	}
}

func newSuboptimalTestRenderer(acquireStatus, presentStatus int32, recreations *int) *vulkanRenderer {
	return &vulkanRenderer{
		device:    10,
		queue:     20,
		swapchain: 30,
		commands:  []uintptr{40},
		fence:     50,
		api: &vulkanAPI{
			waitForFences: func(uintptr, uint32, *uintptr, uint32, uint64) int32 { return vkSuccess },
			resetFences:   func(uintptr, uint32, *uintptr) int32 { return vkSuccess },
			acquireNextImage: func(_, swapchain uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
				*image = 0
				if swapchain == 30 {
					return acquireStatus
				}
				return vkSuccess
			},
			queueSubmit: func(uintptr, uint32, *vkSubmitInfo, uintptr) int32 { return vkSuccess },
			queuePresent: func(_ uintptr, present *vkPresentInfo) int32 {
				if *present.Swapchains == 30 {
					return presentStatus
				}
				return vkSuccess
			},
			deviceWaitIdle: func(uintptr) int32 {
				*recreations++
				return vkSuccess
			},
			getSurfaceCapabilities: func(_, _ uintptr, capabilities *vkSurfaceCapabilities) int32 {
				capabilities.Current = vkExtent2D{Width: 100, Height: 100}
				capabilities.MinImages = 2
				capabilities.CompositeAlpha = vkCompositeOpaque
				capabilities.Usage = vkImageUsageColorAttachment
				return vkSuccess
			},
			getSurfaceFormats: func(_, _ uintptr, count *uint32, formats *vkSurfaceFormat) int32 {
				*count = 1
				if formats != nil {
					*formats = vkSurfaceFormat{Format: vkFormatBGRA8, ColorSpace: vkColorSpaceSRGB}
				}
				return vkSuccess
			},
			createSwapchain: func(_ uintptr, _ *vkSwapchainCreateInfo, _ unsafe.Pointer, swapchain *uintptr) int32 {
				*swapchain = 99
				return vkSuccess
			},
			destroySwapchain:      func(uintptr, uintptr, unsafe.Pointer) {},
			destroyImageView:      func(uintptr, uintptr, unsafe.Pointer) {},
			destroyRenderPass:     func(uintptr, uintptr, unsafe.Pointer) {},
			destroyFramebuffer:    func(uintptr, uintptr, unsafe.Pointer) {},
			destroyCommandPool:    func(uintptr, uintptr, unsafe.Pointer) {},
			destroyPipelineLayout: func(uintptr, uintptr, unsafe.Pointer) {},
			destroyPipeline:       func(uintptr, uintptr, unsafe.Pointer) {},
			getSwapchainImages: func(_, _ uintptr, count *uint32, images *uintptr) int32 {
				if images == nil {
					*count = 1
					return vkSuccess
				}
				*images = 101
				return vkSuccess
			},
			createImageView: func(_ uintptr, _ *vkImageViewCreateInfo, _ unsafe.Pointer, view *uintptr) int32 {
				*view = 201
				return vkSuccess
			},
			createRenderPass: func(_ uintptr, _ *vkRenderPassCreateInfo, _ unsafe.Pointer, renderPass *uintptr) int32 {
				*renderPass = 301
				return vkSuccess
			},
			createFramebuffer: func(_ uintptr, _ *vkFramebufferCreateInfo, _ unsafe.Pointer, framebuffer *uintptr) int32 {
				*framebuffer = 401
				return vkSuccess
			},
			createCommandPool: func(_ uintptr, _ *vkCommandPoolCreateInfo, _ unsafe.Pointer, pool *uintptr) int32 {
				*pool = 501
				return vkSuccess
			},
			allocateCommandBuffers: func(_ uintptr, _ *vkCommandBufferAllocateInfo, buffers *uintptr) int32 {
				*buffers = 601
				return vkSuccess
			},
			createPipelineLayout: func(_ uintptr, _ *vkPipelineLayoutCreateInfo, _ unsafe.Pointer, layout *uintptr) int32 {
				*layout = 701
				return vkSuccess
			},
			createShaderModule: func(_ uintptr, _ *vkShaderModuleCreateInfo, _ unsafe.Pointer, module *uintptr) int32 {
				*module = 801
				return vkSuccess
			},
			destroyShaderModule: func(uintptr, uintptr, unsafe.Pointer) {},
			createGraphicsPipelines: func(_, _ uintptr, _ uint32, _ *vkGraphicsPipelineCreateInfo, _ unsafe.Pointer, pipeline *uintptr) int32 {
				*pipeline = 901
				return vkSuccess
			},
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 { return vkSuccess },
			cmdBeginRenderPass: func(uintptr, *vkRenderPassBeginInfo, uint32) {},
			cmdEndRenderPass:   func(uintptr) {},
			endCommandBuffer:   func(uintptr) int32 { return vkSuccess },
		},
	}
}

func TestSuboptimalRecreatesAfterCompletedFrame(t *testing.T) {
	for _, test := range []struct {
		name          string
		acquireStatus int32
		presentStatus int32
	}{
		{name: "acquire", acquireStatus: vkSuboptimal, presentStatus: vkSuccess},
		{name: "present", acquireStatus: vkSuccess, presentStatus: vkSuboptimal},
	} {
		t.Run(test.name, func(t *testing.T) {
			recreations := 0
			renderer := newSuboptimalTestRenderer(test.acquireStatus, test.presentStatus, &recreations)
			var presented []uintptr
			basePresent := renderer.api.queuePresent
			renderer.api.queuePresent = func(queue uintptr, present *vkPresentInfo) int32 {
				presented = append(presented, *present.Swapchains)
				return basePresent(queue, present)
			}
			if err := renderer.presentRebuilding(); err != nil {
				t.Fatalf("presentRebuilding returned error: %v", err)
			}
			if renderer.submissions != 2 {
				t.Fatalf("completed submissions = %d, want old and new swapchain frames", renderer.submissions)
			}
			if recreations != 1 {
				t.Fatalf("swapchain recreations = %d, want 1", recreations)
			}
			if renderer.swapchain != 99 {
				t.Fatalf("swapchain = %d, want recreated handle 99", renderer.swapchain)
			}
			if !slices.Equal(presented, []uintptr{30, 99}) {
				t.Fatalf("presented swapchains = %v, want [30 99]", presented)
			}
			if renderer.swapchainSuboptimal {
				t.Fatal("suboptimal flag survived successful recreation")
			}
		})
	}
}

func TestSuboptimalRecreationConvergesAcrossFrames(t *testing.T) {
	for _, test := range []struct {
		name          string
		acquireStatus int32
		presentStatus int32
		keepReporting func(*vulkanRenderer)
	}{
		{
			name:          "acquire",
			acquireStatus: vkSuboptimal,
			presentStatus: vkSuccess,
			keepReporting: func(renderer *vulkanRenderer) {
				renderer.api.acquireNextImage = func(_, _ uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
					*image = 0
					return vkSuboptimal
				}
			},
		},
		{
			name:          "present",
			acquireStatus: vkSuccess,
			presentStatus: vkSuboptimal,
			keepReporting: func(renderer *vulkanRenderer) {
				renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkSuboptimal }
			},
		},
		{
			name:          "both",
			acquireStatus: vkSuboptimal,
			presentStatus: vkSuboptimal,
			keepReporting: func(renderer *vulkanRenderer) {
				renderer.api.acquireNextImage = func(_, _ uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
					*image = 0
					return vkSuboptimal
				}
				renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkSuboptimal }
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recreations := 0
			renderer := newSuboptimalTestRenderer(test.acquireStatus, test.presentStatus, &recreations)
			test.keepReporting(renderer)

			for frame := 0; frame < 3; frame++ {
				if err := renderer.presentRebuilding(); err != nil {
					t.Fatalf("frame %d: presentRebuilding returned error: %v", frame, err)
				}
				if recreations != 1 {
					t.Fatalf("frame %d: swapchain recreations = %d, want 1", frame, recreations)
				}
				if renderer.swapchain != 99 {
					t.Fatalf("frame %d: swapchain = %d, want recreated handle 99", frame, renderer.swapchain)
				}
				if !renderer.swapchainSuboptimal {
					t.Fatalf("frame %d: accepted swapchain forgot its SUBOPTIMAL status", frame)
				}
			}
			if renderer.submissions != 4 {
				t.Fatalf("completed submissions = %d, want the old and recreated frame plus two later frames", renderer.submissions)
			}
		})
	}
}

func TestSuboptimalStatusClearsAfterHealthyFrame(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkSuccess, vkSuboptimal, &recreations)
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkSuboptimal }

	if err := renderer.presentRebuilding(); err != nil {
		t.Fatalf("presentRebuilding returned error: %v", err)
	}
	if recreations != 1 {
		t.Fatalf("swapchain recreations = %d, want 1", recreations)
	}

	// A healthy frame clears the accepted status without another recreation.
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkSuccess }
	if err := renderer.presentRebuilding(); err != nil {
		t.Fatalf("healthy frame returned error: %v", err)
	}
	if renderer.swapchainSuboptimal {
		t.Fatal("healthy frame kept the accepted SUBOPTIMAL status")
	}
	if recreations != 1 {
		t.Fatalf("healthy frame recreated the swapchain: recreations = %d, want 1", recreations)
	}

	// A later SUBOPTIMAL is a fresh signal and gets its own single rebuild.
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkSuboptimal }
	if err := renderer.presentRebuilding(); err != nil {
		t.Fatalf("SUBOPTIMAL after healthy frame returned error: %v", err)
	}
	if recreations != 2 {
		t.Fatalf("swapchain recreations = %d, want 2 after a fresh SUBOPTIMAL", recreations)
	}
	if !renderer.swapchainSuboptimal {
		t.Fatal("fresh SUBOPTIMAL was not recorded")
	}
}

func TestOutOfDateRecoveryStaysBounded(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkErrorOutOfDate, vkSuccess, &recreations)

	// The recreated swapchain is out of date as well.
	renderer.api.acquireNextImage = func(_, _ uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
		*image = 0
		return vkErrorOutOfDate
	}

	if err := renderer.presentRebuilding(); err == nil {
		t.Fatal("presentRebuilding accepted a swapchain that stayed out of date")
	}
	if recreations != 1 {
		t.Fatalf("swapchain recreations = %d, want 1", recreations)
	}
	if renderer.swapchain != 99 {
		t.Fatalf("swapchain = %d, want recreated handle 99", renderer.swapchain)
	}
	if renderer.submissions != 0 {
		t.Fatalf("completed submissions = %d, want none for out-of-date frames", renderer.submissions)
	}
}

func TestOutOfDateRecoveryAcceptsResidualSuboptimal(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkErrorOutOfDate, vkSuccess, &recreations)
	// The old swapchain is out of date; the recreated one is suboptimal.
	renderer.api.acquireNextImage = func(_, swapchain uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
		*image = 0
		if swapchain == 30 {
			return vkErrorOutOfDate
		}
		return vkSuboptimal
	}

	if err := renderer.presentRebuilding(); err != nil {
		t.Fatalf("presentRebuilding returned error: %v", err)
	}
	if recreations != 1 {
		t.Fatalf("swapchain recreations = %d, want 1", recreations)
	}
	if !renderer.swapchainSuboptimal || !renderer.swapchainSuboptimalRebuilt {
		t.Fatal("residual SUBOPTIMAL after OUT_OF_DATE recovery was not accepted")
	}
}

func TestPresentOutOfDateRebuilds(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkSuccess, vkErrorOutOfDate, &recreations)
	// Only the old swapchain presents out of date; the recreated one presents.
	renderer.api.queuePresent = func(_ uintptr, present *vkPresentInfo) int32 {
		if *present.Swapchains == 30 {
			return vkErrorOutOfDate
		}
		return vkSuccess
	}

	if err := renderer.presentRebuilding(); err != nil {
		t.Fatalf("presentRebuilding returned error: %v", err)
	}
	if recreations != 1 {
		t.Fatalf("swapchain recreations = %d, want 1", recreations)
	}
	if renderer.swapchain != 99 {
		t.Fatalf("swapchain = %d, want recreated handle 99", renderer.swapchain)
	}
	if renderer.submissions != 2 {
		t.Fatalf("completed submissions = %d, want the out-of-date present and the recreated one", renderer.submissions)
	}
}
