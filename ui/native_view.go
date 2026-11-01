package ui

// A native view is a part of the window that the system draws and serves instead of Viva: a page of
// the system's web engine, for one. The application paints a hole where it goes with
// Canvas.NativeView; the platform puts the view there and keeps Viva's own pointer and keyboard
// handling away from it. What is painted on the same layer as the hole, or below, is hidden by the
// view; what is painted on a higher layer (a popover, a menu, a tooltip opened with PushLayer) is
// drawn over it and takes the pointer where it covers it.
//
//	canvas.NativeView(page.ID(), content)
//	canvas.PushLayer(1)
//	menu.Paint(canvas) // stands over the page
//	canvas.PopLayer()

// NativeViewID names a native view in a window. The values are chosen by whatever opens the view
// (see app.WebHost); zero is never a view.
type NativeViewID uint32

// NativeView is one place a native view fills.
type NativeView struct {
	ID     NativeViewID
	Bounds Rect
	Layer  uint16
	// ClipOf and HasClip capture the active clip at paint time: the view shows only inside it.
	ClipOf  Rect
	HasClip bool
}

// NativeView declares that the native view id fills bounds, on the current layer and inside the
// current clip. A view declared in no paint is hidden; one declared twice in a paint is placed at
// the last bounds.
func (canvas *Canvas) NativeView(id NativeViewID, bounds Rect) {
	if id == 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	view := NativeView{ID: id, Bounds: bounds, Layer: canvas.currentLayer()}
	view.ClipOf, view.HasClip = canvas.clipSnapshot()
	canvas.nativeViews = append(canvas.nativeViews, view)
}

// NativeViews returns the native views painted so far. The result is invalidated by Reset.
func (canvas *Canvas) NativeViews() []NativeView {
	return canvas.nativeViews
}
