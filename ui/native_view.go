package ui

import "github.com/vivaui/viva/gfx"

// NativeView makes a native view (a page of the system's web engine, app.WebView.ID)
// fill the element's rectangle, on the layer and inside the clips the element is
// painted in: in a ScrollArea it is cut to the area, in a Dialog's body it stands on
// the dialog's layer. It is declared where the element is painted, so nothing is kept
// past the frame and no callback is needed. What the page paints on the same layer
// under or over the element is hidden by the view; the page's Popups, Popovers and
// Dialogs go on a higher layer and stand over it.
//
//	f.Slot().Grow(1).NativeView(m.docs.ID())
func (b Box) NativeView(id gfx.NativeViewID) Box {
	b.n()
	b.f.extra(b.i).nativeView = id
	return b
}

// raiseOverNativeViews puts the Popups over the native views the page declared,
// which hide what is painted on their layer and below.
func (f *Frame) raiseOverNativeViews() {
	if top, ok := f.nativeTop(); ok && f.popupLayer <= top {
		f.popupLayer = top + 1
	}
}

// nativeTop is the highest layer a native view of this page is on.
func (f *Frame) nativeTop() (uint16, bool) {
	if f.canvas == nil {
		return 0, false
	}
	views := f.canvas.NativeViews()
	if len(views) <= f.nativeMark {
		return 0, false
	}
	var top uint16
	for _, view := range views[f.nativeMark:] {
		top = max(top, view.Layer)
	}
	return top, true
}
