package windows

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
)

// A Direct3D device can be lost while the window lives: the driver is reset or
// updated, the GPU hangs and Windows resets it, the adapter is unplugged or the
// system switches to another one. Everything made on the device goes with it, but
// what the window shows is in memory in the renderer, so the device is made again
// and the scene drawn once more, instead of closing the window.

// deviceLost reports whether the renderer's device is gone. It asks the device
// itself (ID3D11Device::GetDeviceRemovedReason), which any failing call can then
// be blamed on.
func (renderer *renderer) deviceLost() bool {
	return renderer.device != 0 && platform.DeviceLostCode(uint32(comCall(renderer.device, 39)))
}

// recover makes the device, the swap chain and the pipeline again, on the
// software rasterizer when software is set, and uploads the rectangles to the new
// device. Label textures are made again as they are drawn.
func (renderer *renderer) recover(software bool) error {
	renderer.releaseDevice()
	if err := renderer.createDeviceAndResources(software); err != nil {
		return err
	}
	if len(renderer.instances) == 0 {
		return nil
	}
	if err := renderer.growInstanceBuffer(len(renderer.instances)); err != nil {
		return err
	}
	region := box{Right: uint32(len(renderer.instances) * rectangleInstanceBytes), Bottom: 1, Back: 1}
	comCall(renderer.context, 48, renderer.buffer, 0, uintptr(unsafe.Pointer(&region)), uintptr(unsafe.Pointer(&renderer.instances[0])), 0, 0)
	return nil
}

// recoverDevice answers a failure that came from a lost device. It refuses when
// the device has been lost again and again, which means a driver that cannot be
// relied on and not one that was reset once.
func (state *windowState) recoverDevice(cause error) error {
	plan := state.devices.Next(time.Now())
	if plan == platform.RecoverGiveUp {
		if diag.Enabled(diag.Error) {
			diag.Report(diag.Error, "render", "device-lost", "the graphics device was lost again and again",
				diag.String("error", cause.Error()))
		}
		return fmt.Errorf("windows: the graphics device was lost again and again: %w", cause)
	}
	if err := state.renderer.recover(plan == platform.RecoverSoftware); err != nil {
		if diag.Enabled(diag.Error) {
			diag.Report(diag.Error, "render", "device-recovery-failed", "the graphics device could not be made again",
				diag.String("error", err.Error()), diag.String("cause", cause.Error()))
		}
		return fmt.Errorf("windows: the graphics device was lost and could not be made again: %w (%w)", err, cause)
	}
	if diag.Enabled(diag.Warn) {
		diag.Report(diag.Warn, "render", "device-recovered", "the graphics device was lost and made again",
			diag.Bool("software", plan == platform.RecoverSoftware))
	}
	state.refreshBackdrop()
	return nil
}

// present draws the window, and draws it again on a new device if the one it had
// was lost.
func (state *windowState) present(width, height int) error {
	return state.presentWithSync(width, height, 1)
}

// presentImmediate draws the window without waiting for the display refresh. A
// resize uses it so the frame drawn for the new client size reaches the swap
// chain before the window manager returns; the swap chain itself does not scale
// stale frames (see createWindowSwapChain).
func (state *windowState) presentImmediate(width, height int) error {
	return state.presentWithSync(width, height, 0)
}

func (state *windowState) presentWithSync(width, height int, sync uintptr) error {
	err := state.renderer.presentWithSync(width, height, state.scale, sync)
	if err == nil || !state.renderer.deviceLost() {
		return err
	}
	if err := state.recoverDevice(err); err != nil {
		return err
	}
	return state.renderer.presentWithSync(width, height, state.scale, sync)
}

// applyFrame hands a frame to the renderer, making the device again first if it
// had been lost.
func (state *windowState) applyFrame(frame platform.Frame) error {
	err := state.renderer.apply(frame)
	if err == nil || !state.renderer.deviceLost() {
		return err
	}
	if err := state.recoverDevice(err); err != nil {
		return err
	}
	return state.renderer.apply(frame)
}
