package linux

import (
	"encoding/binary"
	"sync"

	"github.com/ebitengine/purego"
)

// vkPhysicalDeviceTypeCPU is the type a software rasterizer reports (lavapipe).
const vkPhysicalDeviceTypeCPU = 4

var softwareProbe struct {
	once     sync.Once
	software bool
}

// vulkanDeviceIsSoftware reports whether the device a window would draw with is a
// CPU rasterizer: the hardware device with a graphics queue if there is one, else
// the renderer the window will use (ADR 0075). The answer decides whether the
// window gets a frame with a transparent margin (window_margin_linux.go): older
// software drivers cannot present to a window with alpha, and the CPU renderer
// cannot composite one either. A machine with no usable Vulkan draws on the CPU,
// so it answers true. It asks once.
func vulkanDeviceIsSoftware() bool {
	softwareProbe.once.Do(func() {
		api, err := loadVulkan()
		if err != nil {
			softwareProbe.software = true
			return
		}
		defer purego.Dlclose(api.library)
		renderer := &vulkanRenderer{api: api}
		if err := renderer.createInstance(nil); err != nil {
			softwareProbe.software = true
			return
		}
		defer api.destroyInstance(renderer.instance, nil)
		var count uint32
		if api.enumeratePhysicalDevices(renderer.instance, &count, nil) != vkSuccess || count == 0 || count > 32 {
			softwareProbe.software = true
			return
		}
		devices := make([]uintptr, count)
		if api.enumeratePhysicalDevices(renderer.instance, &count, &devices[0]) != vkSuccess {
			softwareProbe.software = true
			return
		}
		for _, device := range devices[:count] {
			var families uint32
			api.getQueueFamilyProperties(device, &families, nil)
			if families == 0 || families > 64 {
				continue
			}
			list := make([]vkQueueFamilyProperties, families)
			api.getQueueFamilyProperties(device, &families, &list[0])
			for _, family := range list[:families] {
				if family.Flags&vkQueueGraphics != 0 && family.Count > 0 {
					var properties [vkPropertiesSize]byte
					api.getPhysicalDeviceProperties(device, &properties)
					kind := binary.LittleEndian.Uint32(properties[vkPropertiesDeviceTypeOffset:])
					if kind != vkPhysicalDeviceTypeCPU {
						// A hardware device is the one the renderer prefers.
						softwareProbe.software = false
						return
					}
					softwareProbe.software = true
					break
				}
			}
		}
	})
	return softwareProbe.software
}
