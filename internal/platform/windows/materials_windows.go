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
