package windows

import "github.com/vivaui/viva/internal/platform"

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
	if state.config.WindowBackdrop != nil && nativeBackdropAPI() != backdropUnavailable {
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

func (state *windowState) handleBackdropMessage(kind uintptr) bool {
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
	if err := renderer.createCompositionDevice(software); err == nil {
		if err := renderer.createResources(); err == nil {
			return true
		}
	}
	// A failed optional compositor must leave the ordinary window usable.
	renderer.releaseDevice()
	renderer.composition = false
	restoreWindowRedirection(renderer.window)
	return false
}

func (renderer *renderer) releaseComposition() {
	for _, object := range [...]*uintptr{&renderer.compositionVisual, &renderer.compositionTarget, &renderer.compositionDevice} {
		release(object)
	}
}
