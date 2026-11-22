// Package avatareditor is the opt-in page control for the avatar crop
// surface of package components. It wraps components.BeginAvatarEditor in a
// ui.Control, so a page declares the editor without calling the component:
//
//	editor := &avatareditor.Editor{
//		Bitmap: m.avatar, IDs: m.avatarIDs, State: m.crop,
//		AspectRatio: 1, Circular: true, MaxStageSize: 320,
//	}
//	...
//	avatareditor.View(c, "Avatar crop", editor).FillWidth().MaxWidth(360)
//	m.crop = editor.State
//	if layout := editor.Layout(); layout.Ready {
//		m.avatar = export(layout.Crop)
//	}
//
// The caller owns the image, the crop state and the identities; the wrapper
// owns the control's identity and input routing. It paints through the
// component with identities inside the control, so pointer, key and wheel
// events aimed at the preview or its controls reach the wrapper and update
// State. Clicked translates a click on one of the caller's identities back, and
// the optional callbacks receive the caller's targets rather than the control's
// internal part identities.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Editor in the model, beside its image: its handlers
// and draw function are made once, so a warm paint allocates nothing. Do not
// copy an Editor after it has been painted.
package avatareditor

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// IDs, Labels, State and Layout are the component's types, named here so a page
// that declares an editor reads through this package alone.
type (
	IDs    = components.AvatarEditorIDs
	Labels = components.AvatarEditorLabels
	State  = components.AvatarEditorState
	Layout = components.AvatarEditorLayout
)

// Editor is one avatar editor's caller-owned image and crop state and its page
// control. Fill the fields from the model every paint; State is updated in
// place by View, by input and by the zoom and reset controls.
type Editor struct {
	// Bitmap is the caller-owned image. Its dimensions define Crop's coordinates.
	Bitmap gfx.Bitmap
	// IDs are the caller's identities for the preview and controls. A zero ID
	// omits that control.
	IDs IDs
	// State is the current zoom and center. Use the zero value for a new image or
	// a reset. View writes it back after every paint and input event.
	State State
	// Labels localizes the accessible names and empty states.
	Labels Labels

	// AspectRatio is width / height, defaulting to 1.
	AspectRatio float32
	// MinZoom defaults to 1. MaxZoom defaults to 8 (or MinZoom, if larger).
	MinZoom, MaxZoom float32
	// MaxStageSize caps the preview's longest edge, defaulting to 320.
	MaxStageSize float32
	// Circular masks a square preview.
	Circular bool
	// Backdrop must match the containing surface's solid color when Circular.
	Backdrop gfx.Color
	// Loading or Error disables editing. Error takes precedence.
	Loading bool
	Error   string

	// Pointer receives every pointer event aimed at the control, with a target
	// that is one of IDs when the event came from one of the editor's parts.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while the control has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the control.
	Scroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	layout   Layout
	ids      IDs
	clicked  gfx.ID
	hadClick bool
}

// View declares the editor at c. label names the control for assistive
// technology. Give it a size; it is a ui.Control painted through the component.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, editor *Editor) ui.Box {
	if editor == nil {
		panic("avatareditor: View needs its Editor")
	}
	editor.prepare()
	return ui.Control(c, label, &editor.handlers).Draw(editor.drawFn)
}

// View declares the editor at c without naming it separately, for a page that
// keeps the Editor in its model.
func (e *Editor) View(c ui.C, label string) ui.Box {
	return View(c, label, e)
}

// Layout is the frame painted last: the stage and the selected crop in bitmap
// pixels. It is zero until View has painted at least once.
func (e *Editor) Layout() Layout { return e.layout }

// Clicked reports whether the paint just made was an event pass delivering a
// click on id, one of the caller's own identities. It is false in ordinary
// paints. Ask it after View, where the component's clicks have always been
// read.
func (e *Editor) Clicked(id gfx.ID) bool {
	return e.hadClick && !id.IsZero() && e.caller(e.clicked) == id
}

func (e *Editor) prepare() {
	if e.prepared {
		return
	}
	e.prepared = true
	e.handlers.Pointer = e.pointer
	e.handlers.Key = e.key
	e.handlers.Scroll = e.scroll
	e.drawFn = e.draw
}

func (e *Editor) draw(d *ui.DrawContext) {
	e.clicked, e.hadClick = d.Canvas.ClickTarget()
	e.ids = e.remap()
	options := components.AvatarEditorOptions{
		IDs: e.ids, Labels: e.Labels, Bitmap: e.Bitmap, State: e.State,
		AspectRatio: e.AspectRatio, MinZoom: e.MinZoom, MaxZoom: e.MaxZoom,
		MaxStageSize: e.MaxStageSize, Circular: e.Circular, Backdrop: e.Backdrop,
		Loading: e.Loading, Error: e.Error,
	}
	e.layout = components.BeginAvatarEditor(d.Canvas, *d.Theme, d.Rect, options).Paint()
	e.State = e.layout.State
}

// remap gives each caller identity a stable identity inside the control, so the
// page routes its events to this control's handlers. A zero caller ID stays
// zero, which keeps that control omitted as the component expects.
func (e *Editor) remap() IDs {
	parts := e.handlers.Parts()
	part := func(caller gfx.ID, index int) gfx.ID {
		if caller.IsZero() {
			return gfx.ID{}
		}
		return parts.At(index)
	}
	return IDs{
		Canvas:  part(e.IDs.Canvas, 0),
		Zoom:    part(e.IDs.Zoom, 1),
		ZoomOut: part(e.IDs.ZoomOut, 2),
		ZoomIn:  part(e.IDs.ZoomIn, 3),
		Reset:   part(e.IDs.Reset, 4),
	}
}

// caller translates an identity inside the control back to the caller's own.
func (e *Editor) caller(part gfx.ID) gfx.ID {
	if part.IsZero() {
		return part
	}
	parts := e.handlers.Parts()
	callers := [...]gfx.ID{e.IDs.Canvas, e.IDs.Zoom, e.IDs.ZoomOut, e.IDs.ZoomIn, e.IDs.Reset}
	for index, caller := range callers {
		if !caller.IsZero() && part == parts.At(index) {
			return caller
		}
	}
	return part
}

func (e *Editor) pointer(event gfx.PointerEvent) gfx.EventResult {
	var result gfx.EventResult
	e.State, result = e.layout.Pointer(event, e.State)
	if e.Pointer != nil {
		event.Target = e.caller(event.Target)
		result = mergeResult(result, e.Pointer(event))
	}
	return result
}

func (e *Editor) key(event gfx.KeyEvent) gfx.EventResult {
	var result gfx.EventResult
	e.State, result = e.layout.Key(event, e.State)
	if e.Key != nil {
		event.Target = e.caller(event.Target)
		result = mergeResult(result, e.Key(event))
	}
	return result
}

func (e *Editor) scroll(event gfx.ScrollEvent) gfx.EventResult {
	var result gfx.EventResult
	e.State, result = e.layout.Scroll(event, e.State)
	if e.Scroll != nil {
		event.Target = e.caller(event.Target)
		result = mergeResult(result, e.Scroll(event))
	}
	return result
}

// mergeResult keeps the wrapper's handling and repaint requests and adds the
// caller callback's. Focus is the control's: it stays on the identity inside
// the control so the page can deliver later keys.
func mergeResult(wrapper, callback gfx.EventResult) gfx.EventResult {
	wrapper.Handled = wrapper.Handled || callback.Handled
	wrapper.Repaint = wrapper.Repaint || callback.Repaint
	return wrapper
}
