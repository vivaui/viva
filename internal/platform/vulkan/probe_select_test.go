//go:build linux

package vulkan

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

// fakeVulkanAPI answers just enough for selectPhysical: the given physical
// devices in order and one graphics family that presents everywhere.
func fakeVulkanAPI(devices []uintptr, kinds map[uintptr]uint32) *vulkanAPI {
	return &vulkanAPI{
		enumeratePhysicalDevices: func(_ uintptr, count *uint32, list *uintptr) int32 {
			*count = uint32(len(devices))
			if list != nil {
				copy(unsafe.Slice(list, len(devices)), devices)
			}
			return vkSuccess
		},
		getQueueFamilyProperties: func(_ uintptr, count *uint32, families *vkQueueFamilyProperties) {
			*count = 1
			if families != nil {
				*families = vkQueueFamilyProperties{Flags: vkQueueGraphics, Count: 1}
			}
		},
		getSurfaceSupport: func(_ uintptr, _ uint32, _ uintptr, supported *uint32) int32 {
			*supported = 1
			return vkSuccess
		},
		getPhysicalDeviceProperties: func(device uintptr, properties *[vkPropertiesSize]byte) {
			binary.LittleEndian.PutUint32(properties[vkPropertiesDeviceTypeOffset:], kinds[device])
		},
	}
}

// A software Vulkan device is never preferred over a hardware one, and a machine
// whose only device is software is reported as such, so the CPU renderer can
// take over.
func TestSelectDevicePrefersHardwareAndReportsSoftware(t *testing.T) {
	hardware := uint32(2) // VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU
	for _, test := range []struct {
		name     string
		devices  []uintptr
		kinds    map[uintptr]uint32
		physical uintptr
		software bool
	}{
		{"software first", []uintptr{11, 22}, map[uintptr]uint32{11: vkPhysicalDeviceTypeCPU, 22: hardware}, 22, false},
		{"hardware first", []uintptr{22, 11}, map[uintptr]uint32{22: hardware, 11: vkPhysicalDeviceTypeCPU}, 22, false},
		{"only software", []uintptr{11}, map[uintptr]uint32{11: vkPhysicalDeviceTypeCPU}, 11, true},
	} {
		device := &vulkanDevice{api: fakeVulkanAPI(test.devices, test.kinds), instance: 1}
		if err := device.selectPhysical(2); err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if device.physical != test.physical || device.software != test.software {
			t.Errorf("%s: device %d software=%v, want %d software=%v", test.name, device.physical, device.software, test.physical, test.software)
		}
	}
}
