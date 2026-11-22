//go:build android

package android

/*
#cgo LDFLAGS: -landroid
#include <android/native_window.h>
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/vivaui/viva/internal/platform/vulkan"
)

const (
	vkStructureAndroidSurfaceCreateInfo = 1000008000
)

type vkAndroidSurfaceCreateInfo struct {
	SType  uint32
	_      uint32
	Next   uintptr
	Flags  uint32
	_      uint32
	Window uintptr
}

// surfaceSource makes the Vulkan surface of the ANativeWindow the window holds now:
// the renderer asks again whenever it has to make its surface again, and the window
// then is the one the host last gave.
func (win *window) surfaceSource() (vulkan.SurfaceSource, error) {
	var create func(uintptr, *vkAndroidSurfaceCreateInfo, unsafe.Pointer, *uintptr) int32
	if err := vulkan.Bind(&create, "vkCreateAndroidSurfaceKHR"); err != nil {
		return vulkan.SurfaceSource{}, err
	}
	return vulkan.SurfaceSource{
		Extension: "VK_KHR_android_surface",
		Create: func(instance uintptr) (uintptr, error) {
			native := win.native
			if native == nil {
				return 0, errors.New("android: the window has no surface")
			}
			info := vkAndroidSurfaceCreateInfo{SType: vkStructureAndroidSurfaceCreateInfo, Window: uintptr(unsafe.Pointer(native))}
			var surface uintptr
			if err := vulkan.Check(create(instance, &info, nil, &surface), "create Android surface"); err != nil {
				return 0, err
			}
			return surface, nil
		},
	}, nil
}
