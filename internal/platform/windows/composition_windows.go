package windows

import (
	"fmt"
	"unsafe"
)

const wsExNoRedirectionBitmap = 0x00200000

var readCompositionWindowStyle = user.NewProc("GetWindowLongPtrW")
var writeCompositionWindowStyle = user.NewProc("SetWindowLongPtrW")

func restoreWindowRedirection(window uintptr) {
	style, _, _ := readCompositionWindowStyle.Call(window, ^uintptr(19)) // GWL_EXSTYLE = -20
	if style&wsExNoRedirectionBitmap == 0 {
		return
	}
	writeCompositionWindowStyle.Call(window, ^uintptr(19), style&^wsExNoRedirectionBitmap)
	setWindowPosition.Call(window, 0, 0, 0, 0, 0, 0x37) // FRAMECHANGED, no move, size, z-order or activation
}

// DXGI_SWAP_CHAIN_DESC1: a premultiplied BGRA flip chain for DirectComposition.
type compositionSwapDesc struct {
	Width, Height, Format, Stereo                         uint32
	Samples, Quality                                      uint32
	Usage, Buffers, Scaling, SwapEffect, AlphaMode, Flags uint32
}

var createD3DDevice = systemDLL("d3d11.dll").NewProc("D3D11CreateDevice")
var createDXGIFactory = systemDLL("dxgi.dll").NewProc("CreateDXGIFactory1")
var createCompositionDevice = systemDLL("dcomp.dll").NewProc("DCompositionCreateDevice")
var factory2IID = guid{0x50c83a1c, 0xe072, 0x4c48, [8]byte{0x87, 0xb0, 0x36, 0x30, 0xfa, 0x36, 0xa6, 0xd0}}
var compositionDeviceIID = guid{0xc37ea93a, 0xe7aa, 0x450d, [8]byte{0xb1, 0x6f, 0x97, 0x46, 0xcb, 0x04, 0x07, 0xf3}}

func (renderer *renderer) createCompositionDevice(software bool) error {
	if createD3DDevice.Find() != nil || createDXGIFactory.Find() != nil || createCompositionDevice.Find() != nil {
		return fmt.Errorf("DirectComposition is unavailable")
	}
	levels := [...]uint32{0xb000}
	drivers := []uintptr{1, 5}
	if software {
		drivers = drivers[1:]
	}
	var status uintptr
	for _, driver := range drivers {
		status, _, _ = createD3DDevice.Call(0, driver, 0, 0x20, uintptr(unsafe.Pointer(&levels[0])), 1, 7, uintptr(unsafe.Pointer(&renderer.device)), 0, uintptr(unsafe.Pointer(&renderer.context)))
		if hresult(status) == nil {
			break
		}
		release(&renderer.context)
		release(&renderer.device)
	}
	if err := hresult(status); err != nil {
		return err
	}
	var factory uintptr
	status, _, _ = createDXGIFactory.Call(uintptr(unsafe.Pointer(&factory2IID)), uintptr(unsafe.Pointer(&factory)))
	if err := hresult(status); err != nil {
		return err
	}
	defer release(&factory)
	desc := compositionSwapDesc{Width: 1, Height: 1, Format: 87, Samples: 1, Usage: 0x20, Buffers: 2, SwapEffect: 3, AlphaMode: 1}
	// IDXGIFactory2::CreateSwapChainForComposition.
	if err := hresult(comCall(factory, 24, renderer.device, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&renderer.swap)))); err != nil {
		return err
	}
	// A null rendering device is valid when visuals contain swap chains only.
	status, _, _ = createCompositionDevice.Call(0, uintptr(unsafe.Pointer(&compositionDeviceIID)), uintptr(unsafe.Pointer(&renderer.compositionDevice)))
	if err := hresult(status); err != nil {
		return err
	}
	// IDCompositionDevice::CreateTargetForHwnd, CreateVisual.
	if err := hresult(comCall(renderer.compositionDevice, 6, renderer.window, 1, uintptr(unsafe.Pointer(&renderer.compositionTarget)))); err != nil {
		return err
	}
	if err := hresult(comCall(renderer.compositionDevice, 7, uintptr(unsafe.Pointer(&renderer.compositionVisual)))); err != nil {
		return err
	}
	// IDCompositionVisual::SetContent; IDCompositionTarget::SetRoot.
	if err := hresult(comCall(renderer.compositionVisual, 15, renderer.swap)); err != nil {
		return err
	}
	if err := hresult(comCall(renderer.compositionTarget, 3, renderer.compositionVisual)); err != nil {
		return err
	}
	return hresult(comCall(renderer.compositionDevice, 3)) // Commit
}
