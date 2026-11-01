//go:build linux && vivagtk

package linux

import (
	"math"
)

// When GTK draws Viva's content (X11), the drawing area lies under the web pages, so what Viva
// draws over a page cannot be in it. Each such part is a piece instead: a drawing area of its own
// over the page, in a window of the display server's own (GDK composes its own windows by what
// covers what and would let the page hide the piece), showing the same canvas at the same place.
// Pieces are opaque; the translucent edge of a shadow over a page is not blended with the page.

// gtkPiece is one such window.
type gtkPiece struct {
	widget, window uintptr
	at             viewPlace
	shown, input   bool
}

// box is a rectangle by its edges.
type box struct{ left, top, right, bottom int32 }

func (b box) touches(o box) bool {
	return b.left <= o.right && o.left <= b.right && b.top <= o.bottom && o.top <= b.bottom
}

func (b box) union(o box) box {
	return box{min(b.left, o.left), min(b.top, o.top), max(b.right, o.right), max(b.bottom, o.bottom)}
}

// addBox adds a rectangle to a list of disjoint ones, joining those it touches.
func addBox(boxes []box, b box) []box {
	for merged := true; merged; {
		merged = false
		for i, other := range boxes {
			if other.touches(b) {
				b = b.union(other)
				boxes = append(boxes[:i], boxes[i+1:]...)
				merged = true
				break
			}
		}
	}
	return append(boxes, b)
}

// layoutPieces puts the pieces where Viva draws over a page, and the input-only ones where it
// handles the pointer without drawing.
func (w *gtkWindow) layoutPieces() {
	if w.overlay == 0 {
		return
	}
	unit, ratio := float64(w.unit), float64(w.ratio)
	boxes, covers := w.pieceBoxes[:0], w.coverBoxes[:0]
	for i, view := range w.snapped {
		for _, o := range w.scene.overlays(i) {
			boxes = addBox(boxes, box{
				int32(math.Floor(float64(o.Left) / unit)), int32(math.Floor(float64(o.Top) / unit)),
				int32(math.Ceil(float64(o.Right) / unit)), int32(math.Ceil(float64(o.Bottom) / unit)),
			})
		}
		for _, c := range view.Covers {
			covers = addBox(covers, box{
				int32(math.Floor(float64(c.X) * ratio)), int32(math.Floor(float64(c.Y) * ratio)),
				int32(math.Ceil(float64(c.X+c.Width) * ratio)), int32(math.Ceil(float64(c.Y+c.Height) * ratio)),
			})
		}
	}
	w.pieceBoxes, w.coverBoxes = boxes, covers
	w.pieces = w.placePieces(w.pieces, boxes, false)
	w.inputPieces = w.placePieces(w.inputPieces, covers, true)
}

func (w *gtkWindow) placePieces(pieces []*gtkPiece, boxes []box, input bool) []*gtkPiece {
	for i, b := range boxes {
		if i == len(pieces) {
			pieces = append(pieces, w.newPiece(input))
		}
		pieces[i].place(w, viewPlace{b.left, b.top, b.right - b.left, b.bottom - b.top})
	}
	for _, p := range pieces[min(len(boxes), len(pieces)):] {
		p.hide(w)
	}
	return pieces
}

func (w *gtkWindow) newPiece(input bool) *gtkPiece {
	g := w.g
	cb := callbacks()
	p := &gtkPiece{input: input}
	events := int32(gdkPointerMotionMask | gdkButtonPressMask | gdkButtonReleaseMask | gdkEnterNotifyMask | gdkLeaveNotifyMask | gdkScrollMask | gdkSmoothScrollMask)
	if input {
		// A window with no picture of its own, that only catches the pointer.
		p.widget = g.eventBoxNew()
		g.eventBoxVisible(p.widget, 0)
	} else {
		p.widget = g.drawingAreaNew()
		events |= gdkExposureMask
		g.connect(p.widget, "draw", cb.draw, w.id)
	}
	g.widgetSetCanFocus(p.widget, 0)
	g.widgetSetEvents(p.widget, events)
	g.widgetSetHAlign(p.widget, 1)
	g.widgetSetVAlign(p.widget, 1)
	for _, signal := range []string{"button-press-event", "button-release-event", "motion-notify-event", "enter-notify-event", "leave-notify-event", "scroll-event"} {
		g.connect(p.widget, signal, cb.event, w.id)
	}
	g.overlayAdd(w.overlay, p.widget)
	return p
}

func (p *gtkPiece) place(w *gtkWindow, at viewPlace) {
	g := w.g
	if !p.shown || p.at != at {
		p.at = at
		g.widgetSetMarginX(p.widget, at.x)
		g.widgetSetMarginY(p.widget, at.y)
		g.widgetSetSize(p.widget, max(at.width, 1), max(at.height, 1))
	}
	if !p.shown {
		g.widgetShow(p.widget)
		p.shown = true
	}
	if p.window == 0 && !p.input {
		if window := g.widgetGetWindow(p.widget); window != 0 {
			g.gdkWindowNative(window)
			p.window = window
			w.applyCursorTo(window)
		}
	}
	if !p.input {
		g.widgetQueueDraw(p.widget)
	} else if window := g.widgetGetWindow(p.widget); window != 0 {
		w.applyCursorTo(window)
	}
}

func (p *gtkPiece) hide(w *gtkWindow) {
	if p.shown {
		w.g.widgetHide(p.widget)
		p.shown = false
	}
}

// pieceOf finds the piece that is the widget.
func (w *gtkWindow) pieceOf(widget uintptr) *gtkPiece {
	for _, p := range w.pieces {
		if p.widget == widget {
			return p
		}
	}
	for _, p := range w.inputPieces {
		if p.widget == widget {
			return p
		}
	}
	return nil
}

// eventOffset is where a widget's own coordinates start in the content's, in widget units.
func (w *gtkWindow) eventOffset(widget uintptr) (float64, float64) {
	if widget == w.area {
		return 0, 0
	}
	if p := w.pieceOf(widget); p != nil {
		return float64(p.at.x), float64(p.at.y)
	}
	return 0, 0
}

// invalidatePieces asks for the part of the canvas that changed, in device pixels, to be drawn in
// the pieces it falls in.
func (w *gtkWindow) invalidatePieces(left, top, right, bottom int32) {
	for _, p := range w.pieces {
		if !p.shown {
			continue
		}
		l, t := max(left, p.at.x), max(top, p.at.y)
		r, b := min(right, p.at.x+p.at.width), min(bottom, p.at.y+p.at.height)
		if r > l && b > t {
			w.g.widgetQueueDrawArea(p.widget, l-p.at.x, t-p.at.y, r-l, b-t)
		}
	}
}

// applyCursorTo shows the window's cursor on a window of a piece.
func (w *gtkWindow) applyCursorTo(window uintptr) {
	if w.cursorShape != 0 {
		w.g.gdkWindowSetCursor(window, w.cursors[w.cursorShape-1])
	}
}
