package flex

import "github.com/vivaui/viva/ui"

// A control (an Input, a Select, a Button, a Textarea) takes the radius of its
// corners and the size of its text from the theme the frame was begun with.
// Radius and FontSize on its element give that one control its own, as
// ui.Theme.WithRadius and WithFontSize do for a component painted by hand:
//
//	f.Input(snapshot).Radius(ui.RadiusFull).FontSize(16)
//
// The element is measured and painted with that theme, so its size, baseline and
// text agree; no other element sees it. They have no effect on a container or a
// Text, which Radius and FontSize style directly, and none on a control that
// draws no corners or text of its own, such as a Slider.

// hasOwnTheme reports whether n is a control that asked for a radius or a font
// size other than the theme's.
func (n *node) hasOwnTheme() bool {
	switch n.kind {
	case kindContainer, kindText, kindSpacer, kindIcon, kindSlot, kindImage:
		return false
	}
	return n.fontSize > 0 || n.flags&flagHasRadius != 0
}

// ownTheme is the frame's theme with the radius and font size n asked for.
func (f *Frame) ownTheme(n *node) ui.Theme {
	theme := f.theme
	if n.fontSize > 0 {
		theme = theme.WithFontSize(n.fontSize)
	}
	if n.flags&flagHasRadius != 0 {
		theme = theme.WithRadius(n.radius)
	}
	return theme
}
