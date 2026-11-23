package vulkan

import (
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"
)

// vulkanDevice is a Vulkan instance, the physical device chosen on it, the
// logical device and its graphics queue: what a renderer draws with, apart from
// its surface, swapchain and the objects it draws (ADR 0093). Every window of
// the process shares one (acquireWindowDevice), so a second window costs no
// instance, device or driver start-up; an offscreen renderer owns its own.
//
// A shared device's queue is used without a lock: the windows render on the
// application's one loop thread, which is all the external synchronisation
// Vulkan asks for.
type vulkanDevice struct {
	api                               *vulkanAPI
	instance, physical, handle, queue uintptr
	queueFamily                       uint32
	// software is set when the device is a CPU rasterizer (lavapipe); dualSource
	// when it was created with dual-source blending.
	software, dualSource bool
	features             [vkFeatureCount]uint32
	// users counts the renderers holding the window device; lost says the
	// device was lost, so no renderer acquires it again.
	users int
	lost  bool
}

// windowDevice is the device the windows draw with, made by the first window
// and destroyed when the last one releases it.
var windowDevice struct {
	sync.Mutex
	current *vulkanDevice
}

// acquireWindowDevice returns the window device, with its instance made; the
// window makes its surface on that instance and then attaches it.
func acquireWindowDevice(api *vulkanAPI, surfaceExtension string) (*vulkanDevice, error) {
	windowDevice.Lock()
	defer windowDevice.Unlock()
	device := windowDevice.current
	if device == nil {
		device = &vulkanDevice{api: api}
		baseExtension := append([]byte("VK_KHR_surface"), 0)
		systemExtension := append([]byte(surfaceExtension), 0)
		if err := device.createInstance([]*byte{&baseExtension[0], &systemExtension[0]}); err != nil {
			return nil, err
		}
		windowDevice.current = device
	}
	device.users++
	return device, nil
}

// attach makes sure the device can present to surface: the first window chooses
// the physical device for its surface and creates the logical device; a later
// one checks that the chosen queue can present to its surface too.
func (device *vulkanDevice) attach(surface uintptr) error {
	windowDevice.Lock()
	defer windowDevice.Unlock()
	if device.handle != 0 {
		var supported uint32
		if status := device.api.getSurfaceSupport(device.physical, device.queueFamily, surface, &supported); status != vkSuccess || supported == 0 {
			return fmt.Errorf("vulkan: the Vulkan device the other windows draw with cannot present to this window (result %d)", status)
		}
		return nil
	}
	if err := device.selectPhysical(surface); err != nil {
		return err
	}
	swapExtension := append([]byte("VK_KHR_swapchain"), 0)
	return device.createLogical([]*byte{&swapExtension[0]})
}

// release gives the window device back; the last renderer to hold it destroys
// it, after destroying every object it made on it.
func (device *vulkanDevice) release() {
	windowDevice.Lock()
	defer windowDevice.Unlock()
	if device.users--; device.users > 0 {
		return
	}
	if windowDevice.current == device {
		windowDevice.current = nil
	}
	device.destroy()
}

// markLost retires a lost device: the renderers that hold it release it as they
// find out, and the first to make itself again gets a new one.
func (device *vulkanDevice) markLost() {
	windowDevice.Lock()
	defer windowDevice.Unlock()
	device.lost = true
	if windowDevice.current == device {
		windowDevice.current = nil
	}
}

// open makes a device a renderer owns, with no surface to present to.
func (device *vulkanDevice) open() error {
	err := device.createInstance(nil)
	if err == nil {
		err = device.selectPhysical(0)
	}
	if err == nil {
		err = device.createLogical(nil)
	}
	if err != nil {
		device.destroy()
	}
	return err
}

// destroy destroys the logical device and the instance.
func (device *vulkanDevice) destroy() {
	if device.handle != 0 {
		device.api.destroyDevice(device.handle, nil)
	}
	if device.instance != 0 {
		device.api.destroyInstance(device.instance, nil)
	}
	*device = vulkanDevice{api: device.api, users: device.users, lost: device.lost}
}

// createInstance creates the Vulkan instance with the given extensions: the
// window path needs the surface ones, an offscreen renderer none.
func (device *vulkanDevice) createInstance(extensions []*byte) error {
	applicationName := append([]byte("Viva"), 0)
	engineName := append([]byte("Viva"), 0)
	application := vkApplicationInfo{SType: vkStructureApplicationInfo, Application: &applicationName[0], Engine: &engineName[0], APIVersion: 1 << 22}
	instanceInfo := vkInstanceCreateInfo{SType: vkStructureInstanceCreateInfo, Application: &application, ExtensionCount: uint32(len(extensions))}
	if len(extensions) > 0 {
		instanceInfo.Extensions = uintptr(unsafe.Pointer(&extensions[0]))
	}
	switch status := device.api.createInstance(&instanceInfo, nil, &device.instance); status {
	case vkSuccess:
		return nil
	case vkErrorIncompatibleDriver, vkErrorInitializationFailed:
		return noVulkan(fmt.Sprintf("Vulkan has no driver (result %d)", status), nil)
	default:
		return vkCheck(status, "create instance")
	}
}

// createLogical creates the logical device and its graphics queue on the
// selected physical device and queue family.
func (device *vulkanDevice) createLogical(extensions []*byte) error {
	priority := float32(1)
	queueInfo := vkDeviceQueueCreateInfo{SType: vkStructureDeviceQueueCreateInfo, Family: device.queueFamily, Count: 1, Priorities: &priority}
	deviceInfo := vkDeviceCreateInfo{SType: vkStructureDeviceCreateInfo, QueueCount: 1, Queues: &queueInfo, ExtensionCount: uint32(len(extensions))}
	// Dual-source blending (a core feature of nearly every desktop device) blends the
	// per-channel coverage of LCD text; without it text is blended in grayscale.
	var supported [vkFeatureCount]uint32
	device.api.getPhysicalDeviceFeatures(device.physical, &supported)
	if supported[vkFeatureDualSourceBlend] != 0 {
		device.features[vkFeatureDualSourceBlend] = 1
		deviceInfo.Features = uintptr(unsafe.Pointer(&device.features))
		device.dualSource = true
	}
	if len(extensions) > 0 {
		deviceInfo.Extensions = uintptr(unsafe.Pointer(&extensions[0]))
	}
	if err := vkCheck(device.api.createDevice(device.physical, &deviceInfo, nil, &device.handle), "create device"); err != nil {
		device.dualSource = false
		return err
	}
	device.api.getDeviceQueue(device.handle, device.queueFamily, 0, &device.queue)
	if device.queue == 0 {
		return fmt.Errorf("vulkan: Vulkan returned no graphics queue")
	}
	return nil
}

func (device *vulkanDevice) selectPhysical(surface uintptr) error {
	var deviceCount uint32
	// A loader whose installed drivers fail to initialize fails enumeration with
	// VK_ERROR_INITIALIZATION_FAILED, not only with the
	// VK_ERROR_INCOMPATIBLE_DRIVER an empty driver list gives. Both are a machine
	// with no usable Vulkan driver and need the same advice as one without a
	// loader; any other result is a real failure.
	switch status := device.api.enumeratePhysicalDevices(device.instance, &deviceCount, nil); status {
	case vkSuccess:
	case vkErrorInitializationFailed, vkErrorIncompatibleDriver:
		return noVulkan(fmt.Sprintf("Vulkan has no driver (result %d)", status), nil)
	default:
		return vkCheck(status, "count physical devices")
	}
	if deviceCount == 0 {
		return noVulkan("Vulkan reports no device", nil)
	}
	if deviceCount > 32 {
		return fmt.Errorf("vulkan: Vulkan physical device count %d", deviceCount)
	}
	devices := make([]uintptr, deviceCount)
	if err := vkCheck(device.api.enumeratePhysicalDevices(device.instance, &deviceCount, &devices[0]), "enumerate physical devices"); err != nil {
		return err
	}
	// A CPU rasterizer (lavapipe) is not a hardware accelerator: the CPU
	// renderer draws a changed region only, while software Vulkan redraws the
	// whole window every frame. Prefer a hardware device; remember the software
	// one in case no hardware device can present.
	var softwareDevice uintptr
	var softwareFamily uint32
	for _, candidate := range devices[:deviceCount] {
		var familyCount uint32
		device.api.getQueueFamilyProperties(candidate, &familyCount, nil)
		if familyCount == 0 || familyCount > 64 {
			continue
		}
		families := make([]vkQueueFamilyProperties, familyCount)
		device.api.getQueueFamilyProperties(candidate, &familyCount, &families[0])
		for index, family := range families[:familyCount] {
			if family.Flags&vkQueueGraphics == 0 || family.Count == 0 {
				continue
			}
			var supported uint32
			if surface == 0 || device.api.getSurfaceSupport(candidate, uint32(index), surface, &supported) == vkSuccess && supported != 0 {
				if device.physicalDeviceIsCPU(candidate) {
					if softwareDevice == 0 {
						softwareDevice, softwareFamily = candidate, uint32(index)
					}
					continue
				}
				device.physical, device.queueFamily, device.software = candidate, uint32(index), false
				return nil
			}
		}
	}
	if softwareDevice != 0 {
		device.physical, device.queueFamily, device.software = softwareDevice, softwareFamily, true
		return nil
	}
	return noVulkan("no Vulkan graphics queue can present to the window system", nil)
}

// physicalDeviceIsCPU reports whether physical is a CPU rasterizer, such as
// Mesa's lavapipe: VkPhysicalDeviceType CPU. Its properties struct is 824 bytes
// and deviceType sits at offset 16.
func (device *vulkanDevice) physicalDeviceIsCPU(physical uintptr) bool {
	var properties [vkPropertiesSize]byte
	device.api.getPhysicalDeviceProperties(physical, &properties)
	return binary.LittleEndian.Uint32(properties[vkPropertiesDeviceTypeOffset:]) == vkPhysicalDeviceTypeCPU
}
