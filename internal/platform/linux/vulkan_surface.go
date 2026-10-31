//go:build linux

package linux

// vulkanSurfaceProvider abstracts window-system specific Vulkan surface creation
// and teardown (e.g. XCB on X11, wl_surface on Wayland).
type vulkanSurfaceProvider interface {
	// RequiredExtensions returns the Vulkan instance extension names
	// (null-terminated byte pointers) required to create this surface.
	RequiredExtensions() []*byte

	// CreateSurface creates a VkSurfaceKHR handle using the provided Vulkan API and instance.
	CreateSurface(api *vulkanAPI, instance uintptr) (uintptr, error)

	// DestroySurface destroys the VkSurfaceKHR and any associated surface resources.
	DestroySurface(api *vulkanAPI, instance uintptr, surface uintptr)
}
