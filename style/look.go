// Package style is the vocabulary a control's look is chosen with: its variant,
// its size and the looks a recipe returns. The engine and the styled controls
// both speak it, so the engine does not depend on a particular design system;
// package components aliases these types.
package style

import "github.com/vivaui/viva/gfx"

type ButtonLook struct {
	// Background, Foreground and Border are the colors of the surface, the label and the
	// icon, and the line. Filled says the background is painted, Bordered the 1px line,
	// and Underlined the label (a hovered link).
	Background, Foreground, Border gfx.Color
	Filled, Bordered, Underlined   bool
	// Radius is the corner radius of the surface.
	Radius float32
	// Shadow is the elevation under the surface; only the outline variant has one.
	Shadow gfx.ShadowLevel
	// Ring is the halo of a focused or invalid button, transparent with a zero RingWidth
	// when there is none. RingOutline says it is drawn as a line over a translucent theme
	// rather than a disc under the surface.
	Ring                  gfx.Color
	RingWidth, RingRadius float32
	RingOutline           bool
}

type SwitchLook struct {
	// Track and Thumb are the colors of the track and the knob at that point of the
	// travel; Border is the track's line, drawn only when Bordered.
	Track, Thumb, Border gfx.Color
	Bordered             bool
	// Ring is the halo of a focused or invalid switch, transparent when there is none;
	// RingWidth is how far it reaches beyond the track.
	Ring      gfx.Color
	RingWidth float32
	// Shadow is the elevation under the track.
	Shadow gfx.ShadowLevel
}

type CheckboxLook struct {
	// Fill, Border and Mark are the colors of the box, its line and its check or dash.
	// Over an opaque theme Fill and Border are flattened onto the page background, as
	// TextEntryLook's are, so the box paints as one solid shape (RoundedBorder).
	Fill, Border, Mark gfx.Color
	// Radius is the corner radius of the box.
	Radius float32
	// Shadow is the elevation under the box.
	Shadow gfx.ShadowLevel
	// Ring is the halo of a focused or invalid box; it is transparent, with a zero
	// RingWidth, when there is none. RingRadius is its outer corner radius and
	// RingWidth how far it reaches beyond the box.
	Ring                  gfx.Color
	RingWidth, RingRadius float32
}

type TextEntryLook struct {
	// Fill and Border are the colors of the surface. Over an opaque theme they are
	// flattened onto the page background, as RoundedBorder needs, so a translucent token
	// does not tint the control's interior; over a translucent theme they keep their alpha.
	Fill, Border gfx.Color
	// BorderWidth and Radius are the line and the corners.
	BorderWidth, Radius float32
	// Shadow is the elevation under the control.
	Shadow gfx.ShadowLevel
}

type SelectLook struct {
	// Surface is the trigger's box: the text-entry recipe, which the select shares with the
	// input, in the state the trigger is in (a hovered or pressed trigger takes the dark
	// theme's hover fill).
	Surface TextEntryLook
	// Foreground is the color of the value, or of the placeholder while there is no value.
	Foreground gfx.Color
	// Chevron is the color of the arrow at the trigger's end.
	Chevron gfx.Color
}

type SegmentOption struct {
	ID       gfx.ID
	Label    string
	Disabled bool
}
