package base

// A control (an Input, a Select, a Button, a Textarea) takes the radius of its
// corners and the size of its text from the theme the frame was begun with.
// Radius and FontSize on its element give that one control its own, as
// Theme.WithRadius and WithFontSize do for a component painted by hand:
//
//	f.Input(snapshot).Radius(gfx.RadiusFull).FontSize(16)
//
// The element is measured and painted with that theme, so its size, baseline and
// text agree; no other element sees it. They have no effect on a container or a
// Text, which Radius and FontSize style directly, and none on a control that
// draws no corners or text of its own, such as a Slider.

// hasOwnTheme reports whether n is a control that asked for a radius or a font
// size other than the theme's.
func (n *Node) HasOwnTheme() bool {
	switch n.Kind {
	case kindContainer, kindText, kindSpacer, kindIcon, kindSlot, kindImage:
		return false
	case kindInput, kindTextarea:
		return n.FontSize > 0 // its corners are the element's own, and the recipe's
	}
	return n.FontSize > 0 || n.Flags&flagHasRadius != 0
}

// ownTheme is the frame's theme with the radius and font size n asked for.
func (f *Frame) OwnTheme(n *Node) Theme {
	theme := f.Palette
	if n.FontSize > 0 {
		theme = theme.WithFontSize(n.FontSize)
	}
	if n.Flags&flagHasRadius != 0 {
		theme = theme.WithRadius(n.Radius)
	}
	return theme
}
