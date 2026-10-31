package windows

import (
	"sync"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const dwmSystemBackdropType = 38

var rtlGetVersion = systemDLL("ntdll.dll").NewProc("RtlGetVersion")

func windowBackdropSupported() bool {
	return nativeBackdropAPI() == backdropSystem
}

type backdropAPI uint8

const (
	backdropUnavailable backdropAPI = iota
	backdropAccent
	backdropSystem
)

func selectBackdropAPI(build uint32, system, accent bool) backdropAPI {
	if build >= 22621 && system {
		return backdropSystem
	}
	if build >= 17763 && build < 22621 && accent {
		return backdropAccent
	}
	return backdropUnavailable
}

// The OS build and exported functions cannot change during this process.
var nativeBackdropAPI = sync.OnceValue(func() backdropAPI {
	// OSVERSIONINFOEXW; RtlGetVersion does not depend on an app manifest.
	var version struct {
		Size, Major, Minor, Build, Platform           uint32
		ServicePack                                   [128]uint16
		ServicePackMajor, ServicePackMinor, SuiteMask uint16
		ProductType, Reserved                         byte
	}
	version.Size = uint32(unsafe.Sizeof(version))
	if rtlGetVersion.Find() != nil || dwmSetWindowAttribute.Find() != nil || dwmExtendFrame.Find() != nil {
		return backdropUnavailable
	}
	status, _, _ := rtlGetVersion.Call(uintptr(unsafe.Pointer(&version)))
	if status != 0 || version.Major < 10 {
		return backdropUnavailable
	}
	return selectBackdropAPI(version.Build, true, setWindowCompositionAttribute.Find() == nil)
})

func (api backdropAPI) effectiveKind(kind platform.BackdropKind) platform.BackdropKind {
	if kind > platform.BackdropMicaAlt || api == backdropUnavailable || (api == backdropAccent && kind != platform.BackdropAcrylic) {
		return platform.BackdropNone
	}
	return kind
}

func (state *windowState) applyBackdrop() {
	if state.config.WindowBackdrop == nil || state.window == 0 {
		return
	}
	style := state.config.WindowBackdrop()
	api := nativeBackdropAPI()
	style.Kind = api.effectiveKind(style.Kind)
	if !state.backdropPolicyKnown {
		state.backdropPolicy = readBackdropPolicy()
		state.backdropPolicyKnown = true
	}
	if !state.renderer.composition || !state.backdropPolicy.allows(style.Kind) {
		style.Kind = platform.BackdropNone
	}
	if state.backdropApplied && state.backdropStyle == style {
		return
	}
	active := false
	if state.renderer.composition || state.backdropActive {
		if api == backdropAccent {
			active = state.applyAccentBackdrop(style)
		} else if api == backdropSystem {
			active = state.applySystemBackdrop(style)
		}
	}
	first := !state.backdropNotified
	state.backdropApplied, state.backdropStyle = true, style
	state.renderer.color = state.config.ClearColor
	if active {
		state.renderer.color = platform.Color{}
	}
	if first || state.backdropActive != active {
		state.backdropNotified = true
		state.backdropActive = active
		// Text on transparent targets uses grayscale coverage, because the
		// compositor's background is unavailable to ClearType blending.
		for index := range state.renderer.labels {
			state.renderer.labels[index].scale = 0
		}
		if state.config.OnBackdropChange != nil {
			state.config.OnBackdropChange(active)
		}
		if state.ready {
			postMessage.Call(state.window, wmRedraw, 0, 0)
		}
	}
	state.applyTitlebarTheme()
}

func (state *windowState) applySystemBackdrop(style platform.BackdropStyle) bool {
	dark := int32(0)
	if style.Dark {
		dark = 1
	}
	if state.config.WindowMode != platform.WindowTitled {
		dwmSetWindowAttribute.Call(state.window, dwmUseImmersiveDarkMode, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
	}
	kind := uint32(1) // DWMSBT_NONE
	frame := margins{Bottom: 1}
	if style.Kind != platform.BackdropNone {
		kind = uint32(style.Kind) + 1 // MAINWINDOW (2), TRANSIENTWINDOW (3), TABBEDWINDOW (4)
		// DirectComposition already supplies the transparent client area.
		// Extending glass into a custom titlebar exposes DWM's caption
		// buttons behind the application's own controls. Keep only the
		// shadow's bottom pixel, as installFrame does, for custom chrome.
		if !state.customFrame() {
			frame = margins{Left: -1, Right: -1, Top: -1, Bottom: -1}
		}
	}
	extended, _, _ := dwmExtendFrame.Call(state.window, uintptr(unsafe.Pointer(&frame)))
	status, _, _ := dwmSetWindowAttribute.Call(state.window, dwmSystemBackdropType, uintptr(unsafe.Pointer(&kind)), unsafe.Sizeof(kind))
	active := style.Kind != platform.BackdropNone && hresult(extended) == nil && hresult(status) == nil
	if !active && style.Kind != platform.BackdropNone {
		kind = 1
		dwmSetWindowAttribute.Call(state.window, dwmSystemBackdropType, uintptr(unsafe.Pointer(&kind)), unsafe.Sizeof(kind))
		frame = margins{Bottom: 1}
		dwmExtendFrame.Call(state.window, uintptr(unsafe.Pointer(&frame)))
	}
	return active
}
