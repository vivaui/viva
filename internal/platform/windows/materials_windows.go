package windows

import (
	"unsafe"

	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
)

type backdropState struct {
	backdropStyle                         platform.BackdropStyle
	backdropApplied, backdropActive       bool
	backdropNotified, backdropPolicyKnown bool
	backdropPolicy                        backdropPolicy
	backdropPowerNotification             uintptr
}

type compositionState struct {
	composition                                             bool
	compositionDevice, compositionTarget, compositionVisual uintptr
}

func (state *windowState) backdropWindowStyle() uintptr {
	if state.config.WindowMode == platform.WindowPreview {
		return wsExNoRedirectionBitmap
	}
	if state.config.WindowBackdrop != nil && nativeBackdropAPI() != backdropUnavailable {
		return wsExNoRedirectionBitmap
	}
	// A transparent window is composed by DirectComposition from a premultiplied
	// swap chain, which is what lets the clear color's alpha reach the desktop.
	if state.config.Transparent {
		return wsExNoRedirectionBitmap
	}
	return 0
}

func (state *windowState) configureBackdropRenderer() {
	state.renderer.composition = state.backdropWindowStyle() != 0
}

func (state *windowState) activeBackdrop() bool { return state.backdropActive }

func (state *windowState) backdropCaptionDark(dark bool) bool {
	if state.backdropActive {
		return state.backdropStyle.Dark
	}
	return dark
}

func (state *windowState) refreshBackdrop() {
	state.backdropApplied, state.backdropPolicyKnown = false, false
	state.applyBackdrop()
}

// handleBackdropMessage takes the messages that change what a material looks like. A window
// that did not ask for one leaves them to the ordinary window procedure: they would otherwise
// rewrite its title bar theme on every theme change and swallow the power broadcasts it never
// subscribed to.
func (state *windowState) handleBackdropMessage(kind uintptr) bool {
	if state.config.WindowBackdrop == nil {
		return false
	}
	switch kind {
	case wmBackdropPower, wmBackdropTheme, wmBackdropComposition:
		state.settingsChanged()
		return kind == wmBackdropPower
	}
	return false
}

func (renderer *renderer) usesComposition() bool { return renderer.composition }

func (renderer *renderer) initializeComposition(software bool) bool {
	if !renderer.composition {
		return false
	}
	err := renderer.createCompositionDevice(software)
	if err == nil {
		err = renderer.createResources()
		if err == nil {
			renderer.describeDevice()
			return true
		}
	}
	// A failed optional compositor must leave the ordinary window usable.
	renderer.releaseDevice()
	renderer.composition = false
	restoreWindowRedirection(renderer.window)
	if diag.Enabled(diag.Warn) {
		diag.Report(diag.Warn, "render", "composition-fallback",
			"DirectComposition could not be made, so the window presents through its own swap chain",
			diag.String("error", err.Error()))
	}
	return false
}

func (renderer *renderer) releaseComposition() {
	for _, object := range [...]*uintptr{&renderer.compositionVisual, &renderer.compositionTarget, &renderer.compositionDevice} {
		release(object)
	}
}

var dwmEnableBlurBehind = dwm.NewProc("DwmEnableBlurBehindWindow")

// dwmBlurBehind is DWM_BLURBEHIND.
type dwmBlurBehind struct {
	Flags                 uint32
	Enable                int32
	Region                uintptr
	TransitionOnMaximized int32
}

// applyTransparent lets the swap chain's alpha reach the desktop. Without a
// DWM composition attribute the window is composed against an opaque white
// sheet; an enabled blur-behind with an empty region turns that sheet off and
// blurs nothing.
func (state *windowState) applyTransparent() {
	if !state.config.Transparent || !state.renderer.composition || dwmEnableBlurBehind.Find() != nil {
		return
	}
	empty, _, _ := createRectRgn.Call(0, 0, ^uintptr(0), ^uintptr(0))
	blur := dwmBlurBehind{Flags: 1 | 2, Enable: 1, Region: empty} // DWM_BB_ENABLE | DWM_BB_BLURREGION
	dwmEnableBlurBehind.Call(state.window, uintptr(unsafe.Pointer(&blur)))
	if empty != 0 {
		deleteObject.Call(empty)
	}
}
