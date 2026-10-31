package windows

import (
	"fmt"
	"time"
	"unsafe"

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
		return fmt.Errorf("windows: the graphics device was lost again and again: %w", cause)
	}
	if err := state.renderer.recover(plan == platform.RecoverSoftware); err != nil {
		return fmt.Errorf("windows: the graphics device was lost and could not be made again: %w (%w)", err, cause)
	}
	state.backdropApplied = false
	state.applyBackdrop()
	return nil
}

// present draws the window, and draws it again on a new device if the one it had
// was lost.
func (state *windowState) present(width, height int) error {
	err := state.renderer.present(width, height, state.scale)
	if err == nil || !state.renderer.deviceLost() {
		return err
	}
	if err := state.recoverDevice(err); err != nil {
		return err
	}
	return state.renderer.present(width, height, state.scale)
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
