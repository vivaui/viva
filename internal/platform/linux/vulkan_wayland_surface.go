//go:build linux

package linux

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	vkStructureWaylandSurfaceCreateInfo uint32 = 1000006000
)

var (
	vkExtensionWaylandSurface = []byte("VK_KHR_wayland_surface\x00")
)

type vkWaylandSurfaceCreateInfo struct {
	SType    uint32
	Padding0 uint32
	Next     unsafe.Pointer
	Flags    uint32
	Padding1 uint32
	Display  uintptr
	Surface  uintptr
}

type waylandSurfaceProvider struct {
	display uintptr // *wl_display
	surface uintptr // *wl_surface
}

// newWaylandSurfaceProvider constructs a vulkanSurfaceProvider backed by a Wayland wl_surface.
func newWaylandSurfaceProvider(display, surface uintptr) *waylandSurfaceProvider {
	return &waylandSurfaceProvider{
		display: display,
		surface: surface,
	}
}

func (p *waylandSurfaceProvider) RequiredExtensions() []*byte {
	return []*byte{
		&vkExtensionSurface[0],
		&vkExtensionWaylandSurface[0],
	}
}

func (p *waylandSurfaceProvider) CreateSurface(api *vulkanAPI, instance uintptr) (uintptr, error) {
	var createWaylandSurface func(uintptr, *vkWaylandSurfaceCreateInfo, unsafe.Pointer, *uintptr) int32
	purego.RegisterLibFunc(&createWaylandSurface, api.library, "vkCreateWaylandSurfaceKHR")

	info := vkWaylandSurfaceCreateInfo{
		SType:   vkStructureWaylandSurfaceCreateInfo,
		Display: p.display,
		Surface: p.surface,
	}
	var surface uintptr
	if err := vkCheck(createWaylandSurface(instance, &info, nil, &surface), "create Wayland surface"); err != nil {
		return 0, err
	}
	return surface, nil
}

func (p *waylandSurfaceProvider) DestroySurface(api *vulkanAPI, instance uintptr, surface uintptr) {
	if api != nil && instance != 0 && surface != 0 {
		api.destroySurface(instance, surface, nil)
	}
}
