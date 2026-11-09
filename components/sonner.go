package components

import "github.com/vivaui/viva/gfx"

// SonnerToast is one caller-owned toast snapshot plus stable interaction IDs.
// The caller owns insertion, expiry, swipe state, and removal.
type SonnerToast struct {
	Snapshot    ToastSnapshot
	Scope       gfx.ID
	DismissID   gfx.ID
	ActionID    gfx.ID
	ActionLabel string
	Disabled    bool
}

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
	var stack SonnerLayout
	// Where the next toast of an expanded stack goes: after the heights of the
	// toasts in front of it, each with its own wrapped text.
	var reach float32
	for index := len(toasts) - 1; index >= 0 && len(toasts)-1-index < limit; index-- {
		toast := toasts[index]
		snapshot := toast.Snapshot
		snapshot.Index = sonnerIndex(len(toasts) - 1 - index)
		snapshot.Expanded = false
		toastOptions := ToastOptions{
			Layer:       sonnerLayer(options.Layer, len(toasts)-index-1),
			Scope:       toast.Scope,
			DismissID:   toast.DismissID,
			ActionID:    toast.ActionID,
			ActionLabel: toast.ActionLabel,
			Disabled:    toast.Disabled,
			Position:    options.Position,
		}
		if options.Expanded && !snapshot.Limited {
			if top {
				snapshot.OffsetY += reach
			} else {
				snapshot.OffsetY -= reach
			}
			snapshot.Index = 0
			reach += ToastHeight(canvas, theme, viewport, snapshot, toastOptions) + toastStackGap
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
