package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// SonnerOptions configures one stateless paint of a caller-owned stack. Toasts
// are ordered newest first. Layer is the oldest toast's layer; each newer toast
// receives the next layer so collapsed cards composite correctly.
type SonnerOptions struct {
	Layer    uint16
	Expanded bool
	// Limit is how many of the newest toasts show; the zero value is Base UI's 3.
	// Older ones are not painted.
	Limit int
	// Position is where the stack grows from; the zero value is the bottom right.
	Position ToastPosition
}

// SonnerLayout summarizes the painted stack without allocating per-toast
// layout storage.
type SonnerLayout struct {
	Bounds  gfx.Rect
	Painted int
}

// PaintSonner paints a complete caller-owned stack using the official themed
// Toast recipe. It does not retain entries, run timers, schedule frames, or
// interpret swipe gestures.
func PaintSonner(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, toasts []SonnerToast, options SonnerOptions) SonnerLayout {
	if canvas == nil || viewport.Width <= 0 || viewport.Height <= 0 || len(toasts) == 0 {
		return SonnerLayout{}
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 3
	}
	top := options.Position >= ToastTopRight
	visible := min(limit, len(toasts))
	// A stack shares one width, the widest card in it, so the cards that peek
	// behind the front one line up instead of showing ragged edges.
	var stackWidth float32
	for index := 0; index < visible; index++ {
		toast := toasts[index]
		if toast.Snapshot.Limited {
			continue
		}
		var scratch overlayLines
		geometry, _ := toastMeasure(canvas, theme, viewport, toast.Snapshot, sonnerToastOptions(options, toast, 0), &scratch)
		stackWidth = max(stackWidth, geometry.width)
	}
	var stack SonnerLayout
	// Paint from the back of the visible stack to its front, which is the
	// first toast: the newest one sits at the edge and the older ones behind it.
	for index := visible - 1; index >= 0; index-- {
		toast := toasts[index]
		snapshot := toast.Snapshot
		snapshot.Index = sonnerIndex(index)
		snapshot.Expanded = false
		toastOptions := sonnerToastOptions(options, toast, visible-1-index)
		toastOptions.width = stackWidth
		if options.Expanded && !snapshot.Limited {
			// The toast sits after the ones in front of it, each with its own
			// wrapped text at the stack's width.
			reach := sonnerReach(canvas, theme, viewport, toasts, options, index, stackWidth)
			if top {
				snapshot.OffsetY += reach
			} else {
				snapshot.OffsetY -= reach
			}
			snapshot.Index = 0
		}
		layout := PaintToast(canvas, theme, viewport, snapshot, toastOptions)
		if layout == (ToastLayout{}) {
			continue
		}
		if stack.Painted == 0 {
			stack.Bounds = layout.Bounds
		} else {
			stack.Bounds = sonnerUnion(stack.Bounds, layout.Bounds)
		}
		stack.Painted++
	}
	return stack
}

// sonnerReach is how far an expanded toast at index sits from the edge: after
// the heights of the toasts in front of it, which are the smaller indexes.
func sonnerReach(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, toasts []SonnerToast, options SonnerOptions, index int, width float32) float32 {
	var reach float32
	for front := 0; front < index; front++ {
		if toasts[front].Snapshot.Limited {
			continue
		}
		toastOptions := sonnerToastOptions(options, toasts[front], 0)
		toastOptions.width = width
		reach += ToastHeight(canvas, theme, viewport, toasts[front].Snapshot, toastOptions) + toastStackGap
	}
	return reach
}

// sonnerToastOptions is the options of one toast of a stack, before the stack's
// shared width is known.
func sonnerToastOptions(options SonnerOptions, toast SonnerToast, layerOffset int) ToastOptions {
	return ToastOptions{
		Layer:       sonnerLayer(options.Layer, layerOffset),
		Scope:       toast.Scope,
		DismissID:   toast.DismissID,
		ActionID:    toast.ActionID,
		ActionLabel: toast.ActionLabel,
		Disabled:    toast.Disabled,
		Position:    options.Position,
	}
}

func sonnerIndex(index int) uint16 {
	if index > int(^uint16(0)) {
		return ^uint16(0)
	}
	return uint16(index)
}

func sonnerLayer(base uint16, offset int) uint16 {
	remaining := int(^uint16(0) - base)
	if offset > remaining {
		return ^uint16(0)
	}
	return base + uint16(offset)
}

func sonnerUnion(a, b gfx.Rect) gfx.Rect {
	left := min(a.X, b.X)
	top := min(a.Y, b.Y)
	right := max(a.X+a.Width, b.X+b.Width)
	bottom := max(a.Y+a.Height, b.Y+b.Height)
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

// Aliases into the style vocabulary.
type SonnerToast = base.SonnerToast
