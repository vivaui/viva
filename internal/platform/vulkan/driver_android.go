//go:build android

package vulkan

// Android ships the loader as libvulkan.so, with the drivers of the device behind
// it, and has no manifests to look for: whether a driver works is found when the
// instance is made.
const vulkanLibraryName = "libvulkan.so"

func vulkanDriverConfigured() bool { return true }
