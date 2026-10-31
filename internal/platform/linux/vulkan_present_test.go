//go:build linux

package linux

import (
	"testing"
	"unsafe"
)

func TestVulkanChoosePresentMode(t *testing.T) {
	// Case 1: api == nil
	r1 := &vulkanRenderer{}
	if got := r1.choosePresentMode(); got != vkPresentFIFO {
		t.Fatalf("expected FIFO when api is nil, got %d", got)
	}

	// Case 2: getSurfacePresentModes is nil
	r2 := &vulkanRenderer{
		api:      &vulkanAPI{},
		physical: 1,
		surface:  2,
	}
	if got := r2.choosePresentMode(); got != vkPresentFIFO {
		t.Fatalf("expected FIFO when getSurfacePresentModes is nil, got %d", got)
	}

	// Case 3: modes only has FIFO
	r3 := &vulkanRenderer{
		api: &vulkanAPI{
			getSurfacePresentModes: func(physical, surface uintptr, count *uint32, modes *uint32) int32 {
				if modes == nil {
					*count = 1
					return vkSuccess
				}
				*modes = vkPresentFIFO
				return vkSuccess
			},
		},
		physical: 1,
		surface:  2,
	}
	if got := r3.choosePresentMode(); got != vkPresentFIFO {
		t.Fatalf("expected FIFO when only FIFO available, got %d", got)
	}

	// Case 4: modes has FIFO, Immediate, and Mailbox -> Mailbox should be preferred!
	availableModes := []uint32{vkPresentImmediate, vkPresentFIFO, vkPresentMailbox}
	r4 := &vulkanRenderer{
		api: &vulkanAPI{
			getSurfacePresentModes: func(physical, surface uintptr, count *uint32, modes *uint32) int32 {
				if modes == nil {
					*count = uint32(len(availableModes))
					return vkSuccess
				}
				slice := unsafe.Slice(modes, len(availableModes))
				copy(slice, availableModes)
				return vkSuccess
			},
		},
		physical: 1,
		surface:  2,
	}
	if got := r4.choosePresentMode(); got != vkPresentMailbox {
		t.Fatalf("expected Mailbox when Mailbox is available, got %d", got)
	}
}

func TestVulkanSwapchainExtentClamping(t *testing.T) {
	var capturedExtent vkExtent2D
	api := &vulkanAPI{
		getSurfaceFormats: func(physical, surface uintptr, count *uint32, formats *vkSurfaceFormat) int32 {
			if formats == nil {
				*count = 1
				return vkSuccess
			}
			*formats = vkSurfaceFormat{Format: vkFormatBGRA8, ColorSpace: vkColorSpaceSRGB}
			return vkSuccess
		},
		createSwapchain: func(device uintptr, info *vkSwapchainCreateInfo, allocator unsafe.Pointer, swapchain *uintptr) int32 {
			if info != nil {
				capturedExtent = info.Extent
			}
			return -1 // Return error to stop createSwapchainWithOld after capturing Extent
		},
		destroySwapchain: func(device uintptr, swapchain uintptr, allocator unsafe.Pointer) {},
	}

	tests := []struct {
		name         string
		renderW      uint32
		renderH      uint32
		capabilities vkSurfaceCapabilities
		wantW        uint32
		wantH        uint32
	}{
		{
			name:    "degenerate 0x0 clamped to minimum 1x1",
			renderW: 0,
			renderH: 0,
			capabilities: vkSurfaceCapabilities{
				Current:        vkExtent2D{Width: ^uint32(0), Height: ^uint32(0)},
				Minimum:        vkExtent2D{Width: 0, Height: 0},
				Maximum:        vkExtent2D{Width: 4096, Height: 4096},
				MinImages:      2,
				Usage:          vkImageUsageColorAttachment,
				CompositeAlpha: vkCompositeOpaque,
			},
			wantW: 1,
			wantH: 1,
		},
		{
			name:    "oversized clamped to maximum",
			renderW: 5000,
			renderH: 4000,
			capabilities: vkSurfaceCapabilities{
				Current:        vkExtent2D{Width: ^uint32(0), Height: ^uint32(0)},
				Minimum:        vkExtent2D{Width: 100, Height: 100},
				Maximum:        vkExtent2D{Width: 1920, Height: 1080},
				MinImages:      2,
				Usage:          vkImageUsageColorAttachment,
				CompositeAlpha: vkCompositeOpaque,
			},
			wantW: 1920,
			wantH: 1080,
		},
		{
			name:    "fixed degenerate 0x0 current extent clamped to at least 1x1",
			renderW: 800,
			renderH: 600,
			capabilities: vkSurfaceCapabilities{
				Current:        vkExtent2D{Width: 0, Height: 0},
				Minimum:        vkExtent2D{Width: 0, Height: 0},
				Maximum:        vkExtent2D{Width: 1920, Height: 1080},
				MinImages:      2,
				Usage:          vkImageUsageColorAttachment,
				CompositeAlpha: vkCompositeOpaque,
			},
			wantW: 1,
			wantH: 1,
		},
		{
			name:    "fixed surface extent preserved when valid",
			renderW: 800,
			renderH: 600,
			capabilities: vkSurfaceCapabilities{
				Current:        vkExtent2D{Width: 1024, Height: 768},
				Minimum:        vkExtent2D{Width: 100, Height: 100},
				Maximum:        vkExtent2D{Width: 2000, Height: 2000},
				MinImages:      2,
				Usage:          vkImageUsageColorAttachment,
				CompositeAlpha: vkCompositeOpaque,
			},
			wantW: 1024,
			wantH: 768,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api.getSurfaceCapabilities = func(physical, surface uintptr, capabilities *vkSurfaceCapabilities) int32 {
				*capabilities = tt.capabilities
				return vkSuccess
			}
			renderer := &vulkanRenderer{
				api:      api,
				physical: 1,
				surface:  2,
				device:   3,
				width:    tt.renderW,
				height:   tt.renderH,
			}
			_ = renderer.createSwapchain()
			if capturedExtent.Width != tt.wantW || capturedExtent.Height != tt.wantH {
				t.Fatalf("extent = (%d, %d), want (%d, %d)", capturedExtent.Width, capturedExtent.Height, tt.wantW, tt.wantH)
			}
		})
	}
}
