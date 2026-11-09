package app

import "runtime"

// panelCornerRadius is the radius, in logical pixels, to which macOS rounds a
// WindowPanel. It matches VIVA_PANEL_CORNER_RADIUS in
// internal/platform/darwin/native.m; panel_test.go keeps the two equal.
const panelCornerRadius = 14

// PanelCornerRadius is the radius, in logical pixels, at which the system cuts
// the corners of a WindowPanel: whatever the panel paints beyond it is not
// shown. Draw the panel's edge as a gfx.Canvas.RoundedBorder of this radius so
// the outline follows the corner instead of being clipped by it. It is zero
// where panels have square corners.
func PanelCornerRadius() float32 {
	if runtime.GOOS == "darwin" {
		return panelCornerRadius
	}
	return 0
}
