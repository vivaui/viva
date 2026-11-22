package app

import "runtime"

// panelCornerRadius is the radius, in logical pixels, to which macOS rounds a
// WindowPanel and a WindowNotification. It matches VIVA_PANEL_CORNER_RADIUS in
// internal/platform/darwin/native.m; panel_test.go keeps the two equal.
const panelCornerRadius = 14

// PanelCornerRadius is the radius, in logical pixels, at which the system cuts
// the corners of a WindowPanel or a WindowNotification: whatever the popup
// paints beyond it is not shown. Draw its edge as a gfx.Canvas.RoundedBorder of
// this radius so the outline follows the corner instead of being clipped by it.
// It is zero where popups have square corners (Linux), and on Windows the
// system's own window rounding applies.
func PanelCornerRadius() float32 {
	if runtime.GOOS == "darwin" {
		return panelCornerRadius
	}
	return 0
}
