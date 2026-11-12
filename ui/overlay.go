package ui

// OverlaySpec is an application overlay: the band it paints on, and the page
// rectangle it covers. The rectangle is known when the page is declared; an
// overlay does not take part in the page's layout.
type OverlaySpec struct {
	Band   Band
	Bounds Rect
}

// Overlay declares an application's own overlay: a floating region over the
// page, painted on Band, with no dismissal, focus trap or backdrop of its own.
// It is for chrome the framework does not own, such as a sidebar that expands
// over the workspace, or a drag ghost:
//
//	ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: r}).Children(func() {
//		// a fill container, rows, and controls of the page's own
//	})
//
// The overlay is a Popup: it is painted after the page's tree, is not clipped by
// the container it is declared in, and is kept inside the page's bounds. Its
// children are page elements like any other: one with a Hit is hit and takes
// Tab normally, one without is only painted. A clean surface that stops presses
// from reaching the page uses Canvas.Block at the start of the children; Block
// has no identity, focus or accessibility node, and later children win over it.
// Block stops presses only: wheel and shortcuts are the application's to route.
//
// The bands are below every menu, dialog, popover, tooltip and toast Viva
// paints, so an overlay never paints over one of them. Hits and Tab follow the
// bands: a higher band is hit first and reached later by Tab, and within one
// band the last declared wins. The overlay itself adds no focus scope.
//
// Declare one overlay per purpose. Two overlays on the same band that overlap
// can interleave their shapes and text (a layer paints all its shapes, then its
// text); give them different bands, or keep them from overlapping. Declare it
// where its name should live, usually at the page root: it is a Popup, so the
// container it is declared in does not lay it out or clip it. Not inside a Memo.
func Overlay(c C, spec OverlaySpec) Box {
	p := c.page
	p.forbidInMemo("Overlay")
	b := c.f.Popup().
		At(spec.Bounds.X, spec.Bounds.Y).
		Size(max(spec.Bounds.Width, 0), max(spec.Bounds.Height, 0))
	b = p.container(b)
	b.n().band = uint16(spec.Band)
	return b
}
