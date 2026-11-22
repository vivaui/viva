package linux

import (
	"errors"
	"fmt"
)

// Vulkan results that mean the graphics device or the window's surface is gone:
// a driver reset, a GPU that was switched or unplugged, an X server that
// dropped the window's resources. Nothing the renderer holds is usable after
// them, but everything it needs to draw again is kept on the CPU.
const (
	vkErrorDeviceLost  int32 = -4
	vkErrorSurfaceLost int32 = -1000000000
)

// maxLossRecoveries is how many times in a row one call rebuilds the renderer
// before it gives up: a device that is lost again at once will not come back.
const maxLossRecoveries = 2

// vulkanResultError is a Vulkan call that did not succeed.
type vulkanResultError struct {
	action string
	status int32
}

func (err *vulkanResultError) Error() string {
	return fmt.Sprintf("linux: %s failed with Vulkan result %d", err.action, err.status)
}

// isGraphicsLoss reports whether err says the device or the surface was lost.
func isGraphicsLoss(err error) bool {
	var result *vulkanResultError
	return errors.As(err, &result) && (result.status == vkErrorDeviceLost || result.status == vkErrorSurfaceLost)
}

// isDeviceLoss reports whether err says the device itself was lost, not only a
// window's surface.
func isDeviceLoss(err error) bool {
	var result *vulkanResultError
	return errors.As(err, &result) && result.status == vkErrorDeviceLost
}
