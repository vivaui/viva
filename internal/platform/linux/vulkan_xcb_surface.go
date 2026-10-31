//go:build linux

package linux

var (
	vkExtensionSurface    = []byte("VK_KHR_surface\x00")
	vkExtensionXcbSurface = []byte("VK_KHR_xcb_surface\x00")
)

type xcbSurfaceProvider struct {
	connection uintptr
	window     uint32
}

// newXcbSurfaceProvider returns a vulkanSurfaceProvider backed by an XCB window.
func newXcbSurfaceProvider(connection uintptr, window uint32) *xcbSurfaceProvider {
	return &xcbSurfaceProvider{
		connection: connection,
		window:     window,
	}
}

func (p *xcbSurfaceProvider) RequiredExtensions() []*byte {
	return []*byte{
		&vkExtensionSurface[0],
		&vkExtensionXcbSurface[0],
	}
}

func (p *xcbSurfaceProvider) CreateSurface(api *vulkanAPI, instance uintptr) (uintptr, error) {
	surfaceInfo := vkXcbSurfaceCreateInfo{
		SType:      vkStructureXcbSurfaceCreateInfo,
		Connection: p.connection,
		Window:     p.window,
	}
	var surface uintptr
	if err := vkCheck(api.createXcbSurface(instance, &surfaceInfo, nil, &surface), "create XCB surface"); err != nil {
		return 0, err
	}
	return surface, nil
}

func (p *xcbSurfaceProvider) DestroySurface(api *vulkanAPI, instance uintptr, surface uintptr) {
	if api != nil && instance != 0 && surface != 0 {
		api.destroySurface(instance, surface, nil)
	}
}
