package linux

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
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
		{"VkWaylandSurfaceCreateInfoKHR", unsafe.Sizeof(vkWaylandSurfaceCreateInfo{}), 40},
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
		{"wl_interface", unsafe.Sizeof(wlInterface{}), 40},
		{"wl_message", unsafe.Sizeof(wlMessage{}), 24},
		{"wl_array", unsafe.Sizeof(wlArray{}), 24},
		{"wl_fixed_t", unsafe.Sizeof(wlFixed(0)), 4},
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

func TestSwapchainRecreationPassesOldHandle(t *testing.T) {
	var oldSwapchainPassed uintptr
	var eventOrder []string
	renderer := vulkanRenderer{
		device:    10,
		swapchain: 42,
		api: &vulkanAPI{
			deviceWaitIdle: func(uintptr) int32 { return vkSuccess },
			getSurfaceCapabilities: func(physical, surface uintptr, caps *vkSurfaceCapabilities) int32 {
				caps.Current = vkExtent2D{Width: 100, Height: 100}
				caps.MinImages = 2
				caps.CompositeAlpha = vkCompositeOpaque
				caps.Usage = vkImageUsageColorAttachment
				return vkSuccess
			},
			getSurfaceFormats: func(physical, surface uintptr, count *uint32, formats *vkSurfaceFormat) int32 {
				if formats == nil {
					*count = 1
					return vkSuccess
				}
				*formats = vkSurfaceFormat{Format: vkFormatBGRA8, ColorSpace: vkColorSpaceSRGB}
				return vkSuccess
			},
			createSwapchain: func(dev uintptr, info *vkSwapchainCreateInfo, p unsafe.Pointer, swapchain *uintptr) int32 {
				oldSwapchainPassed = info.OldSwapchain
				eventOrder = append(eventOrder, "createSwapchain")
				*swapchain = 99
				return vkSuccess
			},
			destroySwapchain: func(dev uintptr, sw uintptr, p unsafe.Pointer) {
				if sw == 42 {
					eventOrder = append(eventOrder, "destroyOldSwapchain")
				}
			},
			getSwapchainImages: func(dev, sw uintptr, count *uint32, images *uintptr) int32 {
				if images == nil {
					*count = 1
					return vkSuccess
				}
				*images = 101
				return vkSuccess
			},
			createImageView: func(dev uintptr, info *vkImageViewCreateInfo, p unsafe.Pointer, view *uintptr) int32 {
				*view = 201
				return vkSuccess
			},
			createRenderPass: func(dev uintptr, info *vkRenderPassCreateInfo, p unsafe.Pointer, rp *uintptr) int32 {
				*rp = 301
				return vkSuccess
			},
			createFramebuffer: func(dev uintptr, info *vkFramebufferCreateInfo, p unsafe.Pointer, fb *uintptr) int32 {
				*fb = 401
				return vkSuccess
			},
			createCommandPool: func(dev uintptr, info *vkCommandPoolCreateInfo, p unsafe.Pointer, pool *uintptr) int32 {
				*pool = 501
				return vkSuccess
			},
			allocateCommandBuffers: func(dev uintptr, info *vkCommandBufferAllocateInfo, buffers *uintptr) int32 {
				*buffers = 601
				return vkSuccess
			},
			createPipelineLayout: func(dev uintptr, info *vkPipelineLayoutCreateInfo, p unsafe.Pointer, layout *uintptr) int32 {
				*layout = 701
				return vkSuccess
			},
			createShaderModule: func(dev uintptr, info *vkShaderModuleCreateInfo, p unsafe.Pointer, module *uintptr) int32 {
				*module = 801
				return vkSuccess
			},
			destroyShaderModule: func(uintptr, uintptr, unsafe.Pointer) {},
			createGraphicsPipelines: func(dev uintptr, cache uintptr, count uint32, info *vkGraphicsPipelineCreateInfo, p unsafe.Pointer, pipe *uintptr) int32 {
				*pipe = 901
				return vkSuccess
			},
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 { return vkSuccess },
			cmdBeginRenderPass: func(uintptr, *vkRenderPassBeginInfo, uint32) {},
			cmdEndRenderPass:   func(uintptr) {},
			endCommandBuffer:   func(uintptr) int32 { return vkSuccess },
		},
	}
	if err := renderer.recreateSwapchain(); err != nil {
		t.Fatalf("recreateSwapchain failed: %v", err)
	}
	if oldSwapchainPassed != 42 {
		t.Fatalf("expected oldSwapchain 42, got %d", oldSwapchainPassed)
	}
	if !slices.Equal(eventOrder, []string{"createSwapchain", "destroyOldSwapchain"}) {
		t.Fatalf("expected createSwapchain then destroyOldSwapchain, got %v", eventOrder)
	}
	if renderer.swapchain != 99 {
		t.Fatalf("expected new swapchain 99, got %d", renderer.swapchain)
	}
}

func TestSwapchainRecreationDestroysOldHandleOnceWhenCreateFails(t *testing.T) {
	var destroyed []uintptr
	renderer := vulkanRenderer{
		device:    10,
		swapchain: 42,
		api: &vulkanAPI{
			deviceWaitIdle: func(uintptr) int32 { return vkSuccess },
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
			createSwapchain: func(uintptr, *vkSwapchainCreateInfo, unsafe.Pointer, *uintptr) int32 {
				return vkErrorInitializationFailed
			},
			destroySwapchain: func(_ uintptr, swapchain uintptr, _ unsafe.Pointer) {
				destroyed = append(destroyed, swapchain)
			},
		},
	}

	// oldSwapchain is retired by the creation attempt even when it fails, so
	// the error cleanup must release that handle exactly once.
	if err := renderer.recreateSwapchain(); err == nil {
		t.Fatal("recreateSwapchain succeeded after a creation failure")
	}
	if !slices.Equal(destroyed, []uintptr{42}) {
		t.Fatalf("destroyed swapchains = %v, want [42]", destroyed)
	}
	if renderer.swapchain != 0 {
		t.Fatalf("swapchain retained after failure: %d", renderer.swapchain)
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

	if err := renderer.presentRebuilding(); err != nil {
		t.Fatalf("residual SUBOPTIMAL frame returned error: %v", err)
	}
	if recreations != 1 {
		t.Fatalf("residual SUBOPTIMAL recreated again: recreations = %d, want 1", recreations)
	}
}

func TestSuboptimalRecreationFailureIsReturned(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkSuccess, vkSuboptimal, &recreations)
	renderer.api.createSwapchain = func(uintptr, *vkSwapchainCreateInfo, unsafe.Pointer, *uintptr) int32 {
		return vkErrorInitializationFailed
	}

	if err := renderer.presentRebuilding(); err == nil {
		t.Fatal("presentRebuilding ignored recreation failure")
	}
	if renderer.submissions != 1 {
		t.Fatalf("completed submissions = %d, want 1", renderer.submissions)
	}
	if recreations != 1 {
		t.Fatalf("swapchain recreations = %d, want 1", recreations)
	}
}

func TestWaylandClientPureGoLoad(t *testing.T) {
	api, err := loadWaylandClient()
	if err != nil {
		t.Logf("libwayland-client.so.0 not available: %v", err)
		return
	}
	defer api.close()
	if api.displayConnect == nil || api.displayDisconnect == nil || api.displayGetFd == nil ||
		api.displayDispatch == nil || api.displayDispatchPending == nil || api.displayFlush == nil ||
		api.displayRoundtrip == nil || api.proxyMarshalFlags == nil || api.proxyAddListener == nil ||
		api.proxyDestroy == nil {
		t.Fatal("one or more waylandAPI function pointers were not bound")
	}
}

func TestWaylandABIStructureOffsets(t *testing.T) {
	for _, field := range []struct {
		name      string
		got, want uintptr
	}{
		{"wlMessage.Name", unsafe.Offsetof(wlMessage{}.Name), 0},
		{"wlMessage.Signature", unsafe.Offsetof(wlMessage{}.Signature), 8},
		{"wlMessage.Types", unsafe.Offsetof(wlMessage{}.Types), 16},

		{"wlInterface.Name", unsafe.Offsetof(wlInterface{}.Name), 0},
		{"wlInterface.Version", unsafe.Offsetof(wlInterface{}.Version), 8},
		{"wlInterface.MethodCount", unsafe.Offsetof(wlInterface{}.MethodCount), 12},
		{"wlInterface.Methods", unsafe.Offsetof(wlInterface{}.Methods), 16},
		{"wlInterface.EventCount", unsafe.Offsetof(wlInterface{}.EventCount), 24},
		{"wlInterface.Events", unsafe.Offsetof(wlInterface{}.Events), 32},

		{"wlArray.Size", unsafe.Offsetof(wlArray{}.Size), 0},
		{"wlArray.Alloc", unsafe.Offsetof(wlArray{}.Alloc), 8},
		{"wlArray.Data", unsafe.Offsetof(wlArray{}.Data), 16},
	} {
		if field.got != field.want {
			t.Errorf("%s offset = %d, want %d", field.name, field.got, field.want)
		}
	}
}

func TestWaylandFixedPointConversion(t *testing.T) {
	cases := []struct {
		val   float64
		fixed wlFixed
	}{
		{0.0, 0},
		{1.0, 256},
		{1.5, 384},
		{0.25, 64},
		{-2.5, -640},
	}
	for _, c := range cases {
		gotFixed := float64ToWlFixed(c.val)
		if gotFixed != c.fixed {
			t.Errorf("float64ToWlFixed(%v) = %d, want %d", c.val, gotFixed, c.fixed)
		}
		gotFloat := wlFixedToFloat64(c.fixed)
		if gotFloat != c.val {
			t.Errorf("wlFixedToFloat64(%d) = %v, want %v", c.fixed, gotFloat, c.val)
		}
	}
}

func TestWaylandRegistryDiscovery(t *testing.T) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Log("WAYLAND_DISPLAY not set, skipping live discovery (verified in headless or Wayland sessions)")
		return
	}
	api, err := loadWaylandClient()
	if err != nil {
		t.Skipf("cannot load libwayland-client: %v", err)
	}
	defer api.close()
	display := api.displayConnect(nil)
	if display == 0 {
		t.Skip("cannot connect to Wayland display")
	}
	defer api.displayDisconnect(display)

	var globals waylandGlobals
	if err := globals.bindGlobals(api, display); err != nil {
		t.Fatalf("bindGlobals failed: %v", err)
	}
	defer globals.destroy(api)

	if globals.compositor == 0 || globals.xdgWmBase == 0 {
		t.Fatalf("expected discovery and binding of compositor and xdg_wm_base, got: %+v", globals)
	}
}

func TestWaylandRegistryRetainedUntilDestroy(t *testing.T) {
	destroyed := make(map[uintptr]int)
	var globals waylandGlobals
	api := &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
			switch opcode {
			case wlDisplayGetRegistryOpcode:
				return 100
			case wlRegistryBindOpcode:
				switch args[0].(uint32) {
				case 1:
					return 200
				case 2:
					return 201
				case 7:
					return 700
				}
			}
			return 0
		},
		proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
		proxyDestroy:     func(proxy uintptr) { destroyed[proxy]++ },
		displayRoundtrip: func(uintptr) int32 {
			globals.handleGlobal(100, 1, "wl_compositor", 4)
			globals.handleGlobal(100, 2, "xdg_wm_base", 1)
			return 0
		},
	}

	if err := globals.bindGlobals(api, 1); err != nil {
		t.Fatalf("bindGlobals: %v", err)
	}
	if globals.registry != 100 {
		t.Fatalf("retained registry = %d, want 100", globals.registry)
	}
	if destroyed[100] != 0 {
		t.Fatal("registry was destroyed after initial discovery")
	}

	globals.handleGlobal(globals.registry, 7, "wl_output", 2)
	if globals.outputs[700] == nil {
		t.Fatal("late output global was not bound")
	}
	globals.handleGlobalRemove(7)
	if destroyed[700] != 1 || len(globals.outputs) != 0 {
		t.Fatalf("late output removal destroyed=%d remaining=%d", destroyed[700], len(globals.outputs))
	}

	globals.destroy(api)
	if destroyed[100] != 1 || globals.registry != 0 {
		t.Fatalf("registry teardown destroyed=%d retained=%d", destroyed[100], globals.registry)
	}
	if destroyed[200] != 1 || destroyed[201] != 1 {
		t.Fatalf("global proxy destruction counts = %v", destroyed)
	}
}

func TestWaylandRegistryDiscoveryFailureReleasesRegistry(t *testing.T) {
	destroyed := make(map[uintptr]int)
	var globals waylandGlobals
	api := &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, _ ...any) uintptr {
			if opcode == wlDisplayGetRegistryOpcode {
				return 100
			}
			return 0
		},
		proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
		proxyDestroy:     func(proxy uintptr) { destroyed[proxy]++ },
		displayRoundtrip: func(uintptr) int32 { return -1 },
	}
	if err := globals.bindGlobals(api, 1); err == nil {
		t.Fatal("expected registry discovery failure")
	}
	if destroyed[100] != 1 || globals.registry != 0 {
		t.Fatalf("failed discovery destroyed=%d retained=%d", destroyed[100], globals.registry)
	}
}

func installWaylandLoopReadTestCallbacks(api *waylandAPI) {
	if api.displayPrepareRead == nil {
		api.displayPrepareRead = func(uintptr) int32 { return 0 }
	}
	if api.displayReadEvents == nil {
		api.displayReadEvents = func(uintptr) int32 { return 0 }
	}
	if api.displayCancelRead == nil {
		api.displayCancelRead = func(uintptr) {}
	}
	if api.displayDispatchPending == nil {
		api.displayDispatchPending = func(uintptr) int32 { return 0 }
	}
}

func TestWaylandLoopWaker(t *testing.T) {
	var executed atomic.Bool
	wakeCh := make(chan struct{}, 1)

	mockLibc := &libcAPI{
		poll: func(fds *pollFD, nfds uintptr, timeout int32) int32 {
			slice := unsafe.Slice(fds, int(nfds))
			select {
			case <-wakeCh:
				slice[1].REvents = 1
				return 1
			case <-time.After(100 * time.Millisecond):
				return 0
			}
		},
		read: func(fd int32, buf unsafe.Pointer, n uintptr) int {
			return int(n)
		},
		write: func(fd int32, buf unsafe.Pointer, n uintptr) int {
			select {
			case wakeCh <- struct{}{}:
			default:
			}
			return int(n)
		},
		close: func(int32) int32 { return 0 },
	}

	loop := &waylandLoop{
		libc:      mockLibc,
		wayland:   &waylandAPI{},
		display:   1,
		waylandFD: 10,
		wakeFD:    11,
	}
	installWaylandLoopReadTestCallbacks(loop.wayland)
	loop.running.Store(true)
	loop.rebuildPollFDs()

	start := time.Now()
	go func() {
		time.Sleep(1 * time.Millisecond)
		loop.post(func() {
			executed.Store(true)
			loop.stop()
		})
	}()

	for loop.step(50) {
	}

	elapsed := time.Since(start)
	if !executed.Load() {
		t.Fatal("posted command was not executed by waylandLoop")
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("waker took too long: %v", elapsed)
	}
}

func TestWaylandLoopPostWakeErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		errno      syscall.Errno
		wantWrites int
	}{
		{name: "EINTR retries", errno: syscall.EINTR, wantWrites: 2},
		{name: "EAGAIN does not retry", errno: syscall.EAGAIN, wantWrites: 1},
		{name: "EBADF does not retry", errno: syscall.EBADF, wantWrites: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			errno := int32(test.errno)
			writes := 0
			executed := 0
			loop := &waylandLoop{
				wakeFD: 11,
				libc: &libcAPI{
					write: func(_ int32, _ unsafe.Pointer, size uintptr) int {
						writes++
						if writes == 1 {
							return -1
						}
						errno = 0
						return int(size)
					},
					errnoLocation: func() *int32 { return &errno },
				},
			}

			loop.post(func() { executed++ })
			if writes != test.wantWrites {
				t.Fatalf("wake writes = %d, want %d", writes, test.wantWrites)
			}
			loop.drainCommands()
			if executed != 1 {
				t.Fatalf("command executed %d times, want 1", executed)
			}
		})
	}
}

func TestWaylandLoopPollErrorStopsUnlessInterrupted(t *testing.T) {
	tests := []struct {
		name    string
		errno   syscall.Errno
		running bool
	}{
		{name: "EINTR retries", errno: syscall.EINTR, running: true},
		{name: "EBADF stops", errno: syscall.EBADF},
		{name: "EINVAL stops", errno: syscall.EINVAL},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errno := int32(test.errno)
			calls := 0
			cancels := 0
			loop := &waylandLoop{
				libc: &libcAPI{
					poll: func(fds *pollFD, nfds uintptr, timeout int32) int32 {
						calls++
						return -1
					},
					errnoLocation: func() *int32 { return &errno },
				},
				wayland: &waylandAPI{displayCancelRead: func(uintptr) { cancels++ }},
				display: 1, waylandFD: 10, wakeFD: 11,
			}
			installWaylandLoopReadTestCallbacks(loop.wayland)
			loop.running.Store(true)
			loop.rebuildPollFDs()

			if got := loop.step(0); got != test.running {
				t.Fatalf("step returned %v after %v, want %v", got, test.errno, test.running)
			}
			if loop.running.Load() != test.running {
				t.Fatalf("running = %v after %v, want %v", loop.running.Load(), test.errno, test.running)
			}
			if test.running && loop.err != nil {
				t.Fatalf("EINTR recorded an error: %v", loop.err)
			}
			if !test.running && loop.err == nil {
				t.Fatalf("%v stopped the loop without recording an error", test.errno)
			}
			if cancels != 1 {
				t.Fatalf("cancel read calls after %v = %d, want 1", test.errno, cancels)
			}
			if loop.step(0) != test.running {
				t.Fatalf("second step disagrees about whether the loop remains active")
			}
			if calls != 1 && !test.running {
				t.Fatalf("stopped loop called poll %d times", calls)
			}
		})
	}
}

func TestWaylandLoopExtraFDErrorsAreOneShot(t *testing.T) {
	for _, test := range []struct {
		name    string
		revents int16
	}{
		{name: "POLLERR", revents: 0x0008},
		{name: "POLLHUP", revents: 0x0010},
		{name: "POLLNVAL", revents: 0x0020},
	} {
		t.Run(test.name, func(t *testing.T) {
			callbacks := 0
			loop := &waylandLoop{
				libc: &libcAPI{poll: func(fds *pollFD, nfds uintptr, _ int32) int32 {
					slice := unsafe.Slice(fds, int(nfds))
					if len(slice) > 2 {
						slice[2].REvents = test.revents
						return 1
					}
					return 0
				}},
				wayland: &waylandAPI{}, display: 1, waylandFD: 10, wakeFD: 11,
			}
			installWaylandLoopReadTestCallbacks(loop.wayland)
			loop.running.Store(true)
			loop.rebuildPollFDs()
			loop.addExtraFD(42, func() { callbacks++ })

			if !loop.step(0) {
				t.Fatal("extra FD failure stopped the whole Wayland loop")
			}
			if callbacks != 1 || len(loop.extraSources) != 0 || len(loop.pollFDs) != 2 {
				t.Fatalf("failed source callbacks=%d extraSources=%v pollFDs=%d", callbacks, loop.extraSources, len(loop.pollFDs))
			}
			if !loop.step(0) || callbacks != 1 {
				t.Fatal("failed source remained registered or dispatched twice")
			}
		})
	}
}

func TestWaylandLoopFlushBackpressureUsesPollOut(t *testing.T) {
	errno := syscall.Errno(0)
	flushes := 0
	var pollEvents []int16
	loop := &waylandLoop{
		libc: &libcAPI{
			poll: func(fds *pollFD, nfds uintptr, timeout int32) int32 {
				slice := unsafe.Slice(fds, int(nfds))
				pollEvents = append(pollEvents, slice[0].Events)
				if flushes == 1 {
					slice[0].REvents = 4 // POLLOUT
					return 1
				}
				return 0
			},
			errnoLocation: func() *int32 {
				value := int32(errno)
				return &value
			},
		},
		wayland: &waylandAPI{
			displayFlush: func(display uintptr) int32 {
				flushes++
				if flushes == 1 {
					errno = syscall.EAGAIN
					return -1
				}
				errno = 0
				return 0
			},
		},
		display:   1,
		waylandFD: 10,
		wakeFD:    11,
	}
	installWaylandLoopReadTestCallbacks(loop.wayland)
	loop.running.Store(true)
	loop.rebuildPollFDs()

	if !loop.step(0) {
		t.Fatal("EAGAIN stopped the Wayland loop")
	}
	if !loop.flushPending {
		t.Fatal("EAGAIN did not arm POLLOUT recovery")
	}
	if !loop.step(0) {
		t.Fatal("successful flush stopped the Wayland loop")
	}
	if loop.flushPending {
		t.Fatal("successful flush left POLLOUT armed")
	}
	if flushes != 2 {
		t.Fatalf("flush called %d times, want 2", flushes)
	}
	if len(pollEvents) != 2 || pollEvents[0]&4 == 0 || pollEvents[1]&4 != 0 {
		t.Fatalf("Wayland poll events = %v, want POLLOUT only while flush is blocked", pollEvents)
	}
}

func TestWaylandLoopReadAndCancelPairing(t *testing.T) {
	for _, test := range []struct {
		name        string
		revents     int16
		wantReads   int
		wantCancels int
	}{
		{name: "readable", revents: pollIn, wantReads: 1},
		{name: "timeout", wantCancels: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads, cancels, dispatches := 0, 0, 0
			api := &waylandAPI{
				displayReadEvents: func(uintptr) int32 { reads++; return 0 },
				displayCancelRead: func(uintptr) { cancels++ },
				displayDispatchPending: func(uintptr) int32 {
					dispatches++
					return 0
				},
			}
			installWaylandLoopReadTestCallbacks(api)
			loop := &waylandLoop{
				libc: &libcAPI{poll: func(fds *pollFD, _ uintptr, _ int32) int32 {
					fds.REvents = test.revents
					if test.revents != 0 {
						return 1
					}
					return 0
				}},
				wayland: api, display: 1, waylandFD: 10, wakeFD: 11,
			}
			loop.running.Store(true)
			loop.rebuildPollFDs()

			if !loop.step(0) {
				t.Fatalf("step stopped: %v", loop.err)
			}
			if reads != test.wantReads || cancels != test.wantCancels {
				t.Fatalf("reads=%d cancels=%d, want %d/%d", reads, cancels, test.wantReads, test.wantCancels)
			}
			if dispatches != 2 {
				t.Fatalf("pending dispatches=%d, want before and after poll", dispatches)
			}
		})
	}
}

func TestWaylandLoopPrepareReadRetriesEagain(t *testing.T) {
	errno := int32(0)
	prepares, dispatches, cancels := 0, 0, 0
	api := &waylandAPI{
		displayPrepareRead: func(uintptr) int32 {
			prepares++
			if prepares == 1 {
				errno = int32(syscall.EAGAIN)
				return -1
			}
			errno = 0
			return 0
		},
		displayDispatchPending: func(uintptr) int32 { dispatches++; return 0 },
		displayCancelRead:      func(uintptr) { cancels++ },
	}
	installWaylandLoopReadTestCallbacks(api)
	loop := &waylandLoop{
		libc: &libcAPI{
			poll:          func(*pollFD, uintptr, int32) int32 { return 0 },
			errnoLocation: func() *int32 { return &errno },
		},
		wayland: api, display: 1, waylandFD: 10, wakeFD: 11,
	}
	loop.running.Store(true)
	loop.rebuildPollFDs()

	if !loop.step(0) {
		t.Fatalf("EAGAIN retry stopped the loop: %v", loop.err)
	}
	if prepares != 2 || dispatches != 3 || cancels != 1 {
		t.Fatalf("prepares=%d dispatches=%d cancels=%d, want 2/3/1", prepares, dispatches, cancels)
	}
}

func TestWaylandLoopReadProtocolFailures(t *testing.T) {
	t.Run("prepare error", func(t *testing.T) {
		errno := int32(syscall.EIO)
		polls, cancels := 0, 0
		api := &waylandAPI{
			displayPrepareRead: func(uintptr) int32 { return -1 },
			displayCancelRead:  func(uintptr) { cancels++ },
		}
		installWaylandLoopReadTestCallbacks(api)
		loop := &waylandLoop{
			libc: &libcAPI{
				poll:          func(*pollFD, uintptr, int32) int32 { polls++; return 0 },
				errnoLocation: func() *int32 { return &errno },
			},
			wayland: api, display: 1, waylandFD: 10, wakeFD: 11,
		}
		loop.running.Store(true)
		loop.rebuildPollFDs()
		if loop.step(0) || !errors.Is(loop.err, syscall.EIO) {
			t.Fatalf("prepare failure running=%v error=%v", loop.running.Load(), loop.err)
		}
		if polls != 0 || cancels != 0 {
			t.Fatalf("failed prepare polled=%d canceled=%d, want 0/0", polls, cancels)
		}
	})

	t.Run("flush error cancels", func(t *testing.T) {
		errno := int32(syscall.EPIPE)
		polls, cancels := 0, 0
		api := &waylandAPI{
			displayFlush:      func(uintptr) int32 { return -1 },
			displayCancelRead: func(uintptr) { cancels++ },
		}
		installWaylandLoopReadTestCallbacks(api)
		loop := &waylandLoop{
			libc: &libcAPI{
				poll:          func(*pollFD, uintptr, int32) int32 { polls++; return 0 },
				errnoLocation: func() *int32 { return &errno },
			},
			wayland: api, display: 1, waylandFD: 10, wakeFD: 11,
		}
		loop.running.Store(true)
		loop.rebuildPollFDs()
		if loop.step(0) || !errors.Is(loop.err, syscall.EPIPE) {
			t.Fatalf("flush failure running=%v error=%v", loop.running.Load(), loop.err)
		}
		if polls != 0 || cancels != 1 {
			t.Fatalf("flush failure polled=%d canceled=%d, want 0/1", polls, cancels)
		}
	})

	t.Run("read error does not cancel", func(t *testing.T) {
		errno := int32(syscall.EPIPE)
		cancels := 0
		api := &waylandAPI{
			displayReadEvents: func(uintptr) int32 { return -1 },
			displayCancelRead: func(uintptr) { cancels++ },
		}
		installWaylandLoopReadTestCallbacks(api)
		loop := &waylandLoop{
			libc: &libcAPI{
				poll: func(fds *pollFD, _ uintptr, _ int32) int32 {
					fds.REvents = pollIn
					return 1
				},
				errnoLocation: func() *int32 { return &errno },
			},
			wayland: api, display: 1, waylandFD: 10, wakeFD: 11,
		}
		loop.running.Store(true)
		loop.rebuildPollFDs()
		if loop.step(0) || !errors.Is(loop.err, syscall.EPIPE) {
			t.Fatalf("read failure running=%v error=%v", loop.running.Load(), loop.err)
		}
		if cancels != 0 {
			t.Fatalf("read failure called cancel %d times", cancels)
		}
	})
}

func TestWaylandLoopEventSourceContract(t *testing.T) {
	dispatchErr := errors.New("source dispatch failed")
	var gotRevents uint16
	source := &waylandExtraSource{
		fd:       42,
		interest: pollIn | pollOut,
		dispatch: func(revents uint16) error {
			gotRevents = revents
			return dispatchErr
		},
	}
	api := &waylandAPI{}
	installWaylandLoopReadTestCallbacks(api)
	loop := &waylandLoop{
		libc: &libcAPI{poll: func(fds *pollFD, nfds uintptr, _ int32) int32 {
			slice := unsafe.Slice(fds, int(nfds))
			if slice[2].Events != pollIn|pollOut {
				t.Fatalf("source poll interest=%d, want POLLIN|POLLOUT", slice[2].Events)
			}
			slice[2].REvents = pollOut | 0x0008
			return 1
		}},
		wayland: api, display: 1, waylandFD: 10, wakeFD: 11,
	}
	loop.running.Store(true)
	loop.rebuildPollFDs()
	loop.addEventSource(source)

	if loop.step(0) || !errors.Is(loop.err, dispatchErr) {
		t.Fatalf("source failure running=%v error=%v", loop.running.Load(), loop.err)
	}
	if gotRevents != pollOut|0x0008 {
		t.Fatalf("dispatch revents=%#x, want POLLOUT|POLLERR", gotRevents)
	}
	if len(loop.extraSources) != 0 {
		t.Fatal("failed event source remained registered")
	}
}

func TestWaylandLoopTransportFailuresRecordErrors(t *testing.T) {
	tests := []struct {
		name    string
		api     *waylandAPI
		errno   syscall.Errno
		revents int16
		want    string
	}{
		{
			name: "flush",
			api: &waylandAPI{displayFlush: func(display uintptr) int32 {
				return -1
			}},
			errno: syscall.EPIPE,
			want:  "flush",
		},
		{
			name: "read events",
			api: &waylandAPI{
				displayReadEvents: func(display uintptr) int32 { return -1 },
				displayGetError:   func(display uintptr) int32 { return int32(syscall.EPIPE) },
			},
			revents: 1,
			want:    "read events",
		},
		{
			name: "dispatch pending",
			api: &waylandAPI{displayDispatchPending: func(display uintptr) int32 {
				return -1
			}},
			want: "dispatch pending events",
		},
		{
			name:    "socket hangup",
			api:     &waylandAPI{},
			revents: 0x0010,
			want:    "socket",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errno := int32(test.errno)
			loop := &waylandLoop{
				libc: &libcAPI{
					poll: func(fds *pollFD, nfds uintptr, timeout int32) int32 {
						fds.REvents = test.revents
						return 1
					},
					errnoLocation: func() *int32 { return &errno },
				},
				wayland:   test.api,
				display:   1,
				waylandFD: 10,
				wakeFD:    11,
			}
			installWaylandLoopReadTestCallbacks(loop.wayland)
			loop.running.Store(true)
			loop.rebuildPollFDs()

			if loop.step(0) {
				t.Fatal("transport failure left the loop running")
			}
			if loop.err == nil || !strings.Contains(loop.err.Error(), test.want) {
				t.Fatalf("loop error = %v, want a reason containing %q", loop.err, test.want)
			}
		})
	}
}

func TestApplicationPostWithoutWindows(t *testing.T) {
	var quitCalled atomic.Bool
	wakeCh := make(chan struct{}, 1)

	mockLibc := &libcAPI{
		poll: func(fds *pollFD, nfds uintptr, timeout int32) int32 {
			slice := unsafe.Slice(fds, int(nfds))
			select {
			case <-wakeCh:
				slice[1].REvents = 1
				return 1
			case <-time.After(50 * time.Millisecond):
				return 0
			}
		},
		read: func(fd int32, buf unsafe.Pointer, n uintptr) int { return int(n) },
		write: func(fd int32, buf unsafe.Pointer, n uintptr) int {
			select {
			case wakeCh <- struct{}{}:
			default:
			}
			return int(n)
		},
		close: func(int32) int32 { return 0 },
	}

	loop := &waylandLoop{
		libc:      mockLibc,
		wayland:   &waylandAPI{},
		display:   1,
		waylandFD: 10,
		wakeFD:    11,
	}
	installWaylandLoopReadTestCallbacks(loop.wayland)
	loop.running.Store(true)
	loop.rebuildPollFDs()

	app := &waylandApplication{
		loop: loop,
	}

	go func() {
		time.Sleep(1 * time.Millisecond)
		quitCalled.Store(true)
		app.Quit()
	}()

	for loop.step(20) {
	}

	if !quitCalled.Load() {
		t.Fatal("Application.Quit was not delivered without windows")
	}
	if loop.running.Load() {
		t.Fatal("loop was not stopped cleanly after application quit")
	}
}

func TestWaylandLoopCloseReleasesWakeFD(t *testing.T) {
	var closedFD int32 = -1
	mockLibc := &libcAPI{
		close: func(fd int32) int32 {
			closedFD = fd
			return 0
		},
	}
	loop := &waylandLoop{
		libc:   mockLibc,
		wakeFD: 99,
	}
	loop.close()
	if closedFD != 99 {
		t.Fatalf("expected close to be called for wakeFD 99, got %d", closedFD)
	}
	if loop.wakeFD != -1 {
		t.Fatalf("expected wakeFD to be reset to -1, got %d", loop.wakeFD)
	}
}

func TestWaylandLoopCloseWaitsForWakeWrite(t *testing.T) {
	writeStarted := make(chan struct{})
	releaseWrite := make(chan struct{})
	var writeCalls atomic.Int32
	var closed atomic.Bool
	var writeAfterClose atomic.Bool
	mockLibc := &libcAPI{
		write: func(_ int32, _ unsafe.Pointer, size uintptr) int {
			if writeCalls.Add(1) == 1 {
				close(writeStarted)
				<-releaseWrite
			}
			if closed.Load() {
				writeAfterClose.Store(true)
			}
			return int(size)
		},
		close: func(int32) int32 {
			closed.Store(true)
			return 0
		},
	}
	loop := &waylandLoop{libc: mockLibc, wakeFD: 11}

	postDone := make(chan struct{})
	go func() {
		loop.post(func() {})
		close(postDone)
	}()
	<-writeStarted
	closeDone := make(chan struct{})
	go func() {
		loop.close()
		close(closeDone)
	}()

	prematureClose := false
	select {
	case <-closeDone:
		prematureClose = true
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseWrite)
	<-postDone
	<-closeDone
	if prematureClose || writeAfterClose.Load() {
		t.Fatalf("wake write raced close: premature=%v writeAfterClose=%v", prematureClose, writeAfterClose.Load())
	}
}

func TestWaylandLoopPostAfterCloseIsRejected(t *testing.T) {
	writes := 0
	loop := &waylandLoop{
		wakeFD: 11,
		libc: &libcAPI{
			write: func(_ int32, _ unsafe.Pointer, size uintptr) int {
				writes++
				return int(size)
			},
			close: func(int32) int32 { return 0 },
		},
	}
	loop.close()
	writesAfterClose := writes
	queueAfterClose := len(loop.cmdQueue)
	executed := 0
	loop.post(func() { executed++ })
	if writes != writesAfterClose || len(loop.cmdQueue) != queueAfterClose || executed != 0 {
		t.Fatalf("post after close: writes=%d/%d queue=%d/%d executed=%d", writes, writesAfterClose, len(loop.cmdQueue), queueAfterClose, executed)
	}
	loop.drainCommands()
	if executed != 0 {
		t.Fatal("post after close executed a command")
	}
}

func TestWaylandConnectDisconnectFDLeak(t *testing.T) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Log("WAYLAND_DISPLAY not set, skipping live 100x connect/disconnect test")
		return
	}
	api, err := loadWaylandClient()
	if err != nil {
		t.Skipf("cannot load libwayland-client: %v", err)
	}
	defer api.close()

	// Warm up
	d := api.displayConnect(nil)
	if d == 0 {
		t.Skip("cannot connect to Wayland display")
	}
	api.displayDisconnect(d)

	countFDs := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return -1
		}
		return len(entries)
	}

	initialFDs := countFDs()
	for i := 0; i < 100; i++ {
		disp := api.displayConnect(nil)
		if disp == 0 {
			t.Fatalf("displayConnect failed at iteration %d", i)
		}
		api.displayDisconnect(disp)
	}
	finalFDs := countFDs()
	if initialFDs > 0 && finalFDs > initialFDs {
		t.Fatalf("FD leak detected after 100 connections: was %d, now %d", initialFDs, finalFDs)
	}
}
