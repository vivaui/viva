package interaction

import (
	"cmp"
	"slices"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

// Focus owns the hit-region topology and the keyboard state that follows from
// it. A painter fills it from the canvas after each paint and asks it to apply
// focus results from event handlers. The zero value is ready to use.
type Focus struct {
	router         input.Router
	regions        []input.Region
	focused        ui.ID
	keyboardFocus  bool
	pendingFocus   ui.ID
	pendingVisible bool
	// focusClaim is the non-modal focus request already honored, so a popover
	// painted every frame does not pull focus back after the user leaves it.
	focusClaim ui.ID
	escape     ui.ID
	// painted is the hit list of the previous paint, so a newly appearing
	// Autofocus hit takes the keyboard once. scratch is the next list.
	painted []ui.ID
	scratch []ui.ID
}

// Router exposes the retained input router for hit testing, press routing and
// key traversal.
func (f *Focus) Router() *input.Router { return &f.router }

// Focused returns the target that owns the keyboard, or zero.
func (f *Focus) Focused() ui.ID { return f.focused }

// KeyboardFocus reports whether the focus ring is visible because the keyboard
// moved focus, as opposed to a pointer click.
func (f *Focus) KeyboardFocus() bool { return f.keyboardFocus }

// SetKeyboardFocus changes how focus is presented without moving it.
func (f *Focus) SetKeyboardFocus(visible bool) { f.keyboardFocus = visible }

// PendingFocus returns a target whose paint-time reveal is not resolved yet.
func (f *Focus) PendingFocus() ui.ID { return f.pendingFocus }

// PaintTarget is the focus a paint should announce: a pending target wins,
// because a virtualized list has to lay out around it before the hit exists.
func (f *Focus) PaintTarget() (ui.ID, bool) {
	if !f.pendingFocus.IsZero() {
		return f.pendingFocus, f.pendingVisible
	}
	return f.focused, f.keyboardFocus
}

// EscapeTarget returns the control Escape dismisses, or zero.
func (f *Focus) EscapeTarget() ui.ID { return f.escape }

// WasPainted reports whether the previous paint declared a hit with this ID.
func (f *Focus) WasPainted(id ui.ID) bool {
	for _, previous := range f.painted {
		if previous == id {
			return true
		}
	}
	return false
}

// Sync rebuilds the routing regions from canvas, moves focus for a newly
// appeared Autofocus hit, honors a canvas focus request, and resolves a pending
// target. Callers paint again when the focused target changed.
func (f *Focus) Sync(canvas *ui.Canvas) {
	f.regions = f.regions[:0]
	hits := canvas.Hits()
	for index := range hits {
		hit := &hits[index]
		if hit.Container {
			continue // semantic regions are not interaction targets
		}
		f.regions = append(f.regions, input.Region{
			Target:            input.Target(hit.ID),
			Bounds:            input.Rect{X: hit.Bounds.X, Y: hit.Bounds.Y, Width: hit.Bounds.Width, Height: hit.Bounds.Height},
			Scope:             hit.Scope,
			Focusable:         hit.Focusable,
			Role:              hit.Role,
			SkipTab:           hit.SkipTab,
			Disabled:          hit.Disabled,
			FocusableDisabled: hit.FocusableDisabled,
			Blocks:            hit.Block,
			Label:             hit.IsLabel,
			Backdrop:          hit.Backdrop,
			MovesWindow:       hit.MovesWindow,
			PassSecondary:     hit.PassSecondary, PassPrimary: hit.PassPrimary,
			Layer: hit.Layer,
		})
	}
	// Hits are declared in the order the code paints, which is not the order
	// the layers composite in: a popup on a higher layer is often painted
	// before the field it covers. The front-most region must be the one on the
	// highest layer, and within a layer the last declared.
	if !slices.IsSortedFunc(f.regions, compareRegionLayers) {
		slices.SortStableFunc(f.regions, compareRegionLayers)
	}
	f.router.SetRegions(f.regions)
	f.router.SetActiveScope(canvas.TrappedFocusScope(), input.Target(canvas.InitialFocus()))
	f.applyAutofocus(hits)
	if request := canvas.FocusRequest(); !request.IsZero() {
		if request != f.focusClaim {
			if focused, ok := f.router.Focused(); ok && focused == input.Target(request) {
				f.focusClaim = request
			} else if f.router.Focus(input.Target(request)) {
				f.focusClaim = request
			}
		}
	} else {
		f.focusClaim = ui.ID{}
	}
	if !f.pendingFocus.IsZero() {
		target := input.Target(f.pendingFocus)
		f.router.Focus(target)
		if focused, ok := f.router.Focused(); ok && focused == target {
			f.keyboardFocus = f.pendingVisible
		}
		f.pendingFocus = ui.ID{}
		f.pendingVisible = false
	}
	f.escape = canvas.EscapeTarget()
	f.refresh()
}

// Apply carries out the focus part of an event result. It reports whether the
// focused target or the presentation of focus changed.
func (f *Focus) Apply(result ui.EventResult) bool {
	before, beforeVisible := f.focused, f.keyboardFocus
	if result.Blur {
		f.pendingFocus = ui.ID{}
		f.pendingVisible = false
		f.router.Blur()
	} else if !result.Focus.IsZero() {
		if f.router.Focus(input.Target(result.Focus)) {
			f.pendingFocus = ui.ID{}
			f.pendingVisible = false
		} else if focused, ok := f.router.Focused(); !ok || focused != result.Focus {
			// The event may have revealed this hit. Resolve it against the hit
			// list produced by the repaint instead of making the application
			// stage focus through a second event.
			f.pendingFocus = result.Focus
			f.pendingVisible = result.FocusVisible
		}
	}
	f.refresh()
	if f.focused != before {
		f.keyboardFocus = result.FocusVisible && !f.focused.IsZero()
	} else if !result.Focus.IsZero() || result.Blur {
		f.keyboardFocus = result.FocusVisible && !f.focused.IsZero()
	}
	return f.focused != before || f.keyboardFocus != beforeVisible || !f.pendingFocus.IsZero()
}

// PointerDown applies the focus rule of a press: the hit under the pointer takes
// focus, a press on empty space clears it unless a focus scope is active, and a
// pointer never shows the keyboard ring.
func (f *Focus) PointerDown(x, y float32, target ui.ID) {
	f.router.PointerDown(x, y)
	if target.IsZero() && f.router.ActiveScope().IsZero() {
		f.router.Blur()
	}
	f.refresh()
	f.keyboardFocus = false
}

// Refresh copies the router's current focus into the cached value. Callers use
// it after a key traversal, which moves focus inside the router.
func (f *Focus) Refresh() { f.refresh() }

// Trim releases capacity left behind by a previous, much larger page.
func (f *Focus) Trim() {
	f.regions = trimCapacity(f.regions)
	f.painted = trimCapacity(f.painted)
	f.scratch = trimCapacity(f.scratch)
}

func (f *Focus) refresh() {
	if focused, ok := f.router.Focused(); ok {
		f.focused = focused
	} else {
		f.focused = ui.ID{}
	}
}

// applyAutofocus moves the keyboard to the first hit that asks for it and was
// not painted last time. A trapped focus scope keeps its own say.
func (f *Focus) applyAutofocus(hits []ui.Hit) {
	next := f.scratch[:0]
	focused := false
	for index := range hits {
		hit := &hits[index]
		if !focused && hit.Autofocus && !hit.ID.IsZero() && !f.WasPainted(hit.ID) {
			if f.router.Focus(input.Target(hit.ID)) {
				focused = true
			}
		}
		if !hit.ID.IsZero() {
			next = append(next, hit.ID)
		}
	}
	f.scratch = next
	f.painted, f.scratch = next, f.painted[:0]
}

func compareRegionLayers(left, right input.Region) int {
	return cmp.Compare(left.Layer, right.Layer)
}

// trimCapacity is the shared retention rule: a buffer that stayed near its
// capacity keeps it, so a page that hovers around a size never reallocates.
func trimCapacity[T any](items []T) []T {
	const minCapacity = 4096
	const ratio = 4
	if cap(items) <= minCapacity || len(items)*ratio >= cap(items) {
		return items
	}
	next := make([]T, len(items), max(len(items)*2, minCapacity/64))
	copy(next, items)
	return next
}
