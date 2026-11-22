package base

// Badge adds a presentational badge label, as components.PaintBadge paints it: a
// small chip dressed by its variant, chosen with the Button variants' methods.
// It takes no room beyond its text and responds to nothing unless given an
// identity with Hit, which makes a click on it report that identity (a removable
// tag's remove button, a link).
//
//	ui.Badge(c, "beta").Secondary()
//	ui.Badge(c, tag+" ×").Secondary().Hit(tagID, tag)
func (f *Frame) Badge(Label string) Box {
	b := f.Box(kindBadge)
	n := &f.Nodes[b.i]
	n.Text, n.Label = Label, Label
	n.Flags |= flagThemed
	// The chip's content is centered: the default label, or what Children
	// declares; the recipe gives its padding, pill radius and label size.
	n.Dir, n.Items, n.Justify = row, Center, Center
	return b
}

// Badge adds a badge to a page.
func Badge(c C, Label string) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Badge is for a page")
	}
	return c.f.Badge(Label)
}
