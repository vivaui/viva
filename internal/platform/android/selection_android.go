//go:build android

package android

/*
#include "jni_android.h"
*/
import "C"

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// What the host's selection handles and menu need to follow: where the selection's two
// ends are and what it covers, in the surface's pixels, or that there is no selection to
// show. The host draws the handles, the magnifier and the menu from the system's own
// resources; the program owns the selection, and every move of a handle is an edit of it.
type selectionUI struct {
	shown bool
	// fixed is the end of the selection a handle drag leaves where it is, in runes.
	fixed int
	// last is what the host was last told, so an unchanged selection costs no call.
	last [9]float32
}

// Selection menu items, as the host numbers them.
const (
	selectionCut = iota + 1
	selectionCopy
	selectionPaste
	selectionSelectAll
)

// Handle drag phases.
const (
	handleDown = iota
	handleMove
	handleUp
)

// syncSelection tells the host where the selection is, or that there is none, after
// each frame: handles and the menu are shown over a selection of the focused text input
// that is not being composed, and only where the input can say where its text is.
func (win *window) syncSelection() {
	geometry := win.selectionGeometry()
	if geometry == nil {
		win.hideSelection()
		return
	}
	start, end := win.text.session.SelectionBounds()
	first, ok := geometry.CaretRect(start)
	last, okLast := geometry.CaretRect(end)
	if !ok || !okLast {
		win.hideSelection()
		return
	}
	density := win.density
	bounds := first
	for _, rect := range geometry.RangeRects(nil, start, end) {
		bounds = unionRect(bounds, rect)
	}
	bounds = unionRect(bounds, last)
	values := [9]float32{
		first.X * density, (first.Y + first.Height) * density,
		last.X * density, (last.Y + last.Height) * density,
		first.Height * density,
		bounds.X * density, bounds.Y * density, (bounds.X + bounds.Width) * density, (bounds.Y + bounds.Height) * density,
	}
	if win.selection.shown && values == win.selection.last {
		return
	}
	win.selection.shown, win.selection.last = true, values
	C.viva_host_selection(1, C.float(values[0]), C.float(values[1]), C.float(values[2]), C.float(values[3]),
		C.float(values[4]), C.float(values[5]), C.float(values[6]), C.float(values[7]), C.float(values[8]))
}

func (win *window) hideSelection() {
	if win.selection.shown {
		win.selection.shown = false
		C.viva_host_selection(0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	}
}

// selectionGeometry is the text geometry of the focused input when a selection is to be
// shown over it: not a caret, not a composition, and the input says where its text is.
func (win *window) selectionGeometry() platform.TextGeometry {
	if win.text == nil || win.config.TextGeometry == nil || win.text.session.State.Composing {
		return nil
	}
	if start, end := win.text.session.SelectionBounds(); start == end {
		return nil
	}
	return win.config.TextGeometry()
}

// selectionHandle follows a finger on a handle: the end it holds goes where the finger
// points, the other stays.
func (application *application) selectionHandle(ev event) {
	win := application.window
	if win == nil || win.closed || win.text == nil || win.density <= 0 {
		return
	}
	defer application.guard()
	session := &win.text.session
	start, end := session.SelectionBounds()
	switch int(ev.pointer) {
	case handleDown:
		// Dragging the start handle leaves the end where it is, and the other way round.
		if ev.action == 0 {
			win.selection.fixed = end
		} else {
			win.selection.fixed = start
		}
		return
	case handleUp:
		return
	}
	geometry := win.config.TextGeometry()
	if geometry == nil {
		return
	}
	index, ok := geometry.IndexAt(ev.x/win.density, ev.y/win.density)
	if !ok {
		return
	}
	if err := session.SelectRunes(win.selection.fixed, index); err != nil {
		application.fail(err)
		return
	}
	application.textEdited()
}

// selectionAction is an item of the menu over a selection.
func (application *application) selectionAction(ev event) {
	win := application.window
	if win == nil || win.closed || win.text == nil {
		return
	}
	defer application.guard()
	session := &win.text.session
	var err error
	switch ev.action {
	case selectionCopy, selectionCut:
		value := session.SelectedText()
		if value == "" {
			return
		}
		SetClipboardText(value)
		if ev.action == selectionCut {
			_, err = session.Cut()
		} else {
			_, end := session.SelectionBounds()
			err = session.SelectRunes(end, end) // copying ends the selection, as in every text field
		}
	case selectionPaste:
		if value, ok := ClipboardText(); ok {
			err = session.Paste(value, win.text.input.Multiline)
		}
	case selectionSelectAll:
		err = session.Command(platform.TextSelectAll)
	}
	if err != nil {
		application.fail(err)
		return
	}
	application.textEdited()
}

func unionRect(a, b gfx.Rect) gfx.Rect {
	left, top := min(a.X, b.X), min(a.Y, b.Y)
	right, bottom := max(a.X+a.Width, b.X+b.Width), max(a.Y+a.Height, b.Y+b.Height)
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}
