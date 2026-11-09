package input

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

// Target is a stable application-defined interaction identity.
type Target = gfx.ID

// Rect is an axis-aligned logical-pixel hit region.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Region describes one interaction target in back-to-front paint order.
type Region struct {
	Target    Target
	Bounds    Rect
	Scope     gfx.ID
	Focusable bool
	Disabled  bool
	// FocusableDisabled keeps a disabled region focusable (but inert), as a
	// disabled accordion or tab trigger is.
	FocusableDisabled bool
	// Role is the semantic role the region declared. The router uses it to
	// decide which keys activate the focused control: a button takes Enter and
	// Space, a checkbox only Space, a slider neither.
	Role gfx.Role
	// SkipTab keeps a focusable region out of Tab traversal while leaving it
	// reachable by arrows and explicit Focus.
	SkipTab bool
	// Blocks makes the region opaque: presses inside it reach no target
	// behind it. Target is zero.
	Blocks bool
	// Label makes the region a label of Target: a press on it focuses the
	// target's own focusable region. It is not focusable itself.
	Label bool
	// Backdrop marks a region that is not a control (see gfx.HitOptions): it
	// takes presses like any other, but PressAt does not count it as one.
	Backdrop bool
	// MovesWindow marks a region a custom-titlebar window is moved from (see
	// gfx.Canvas.MoveRegion). Target is zero; it is never a hit target.
	MovesWindow bool
	// PassSecondary marks a Backdrop that hands a secondary press on to what lies
	// beneath once it is dismissed (see gfx.HitOptions).
	PassSecondary bool
	// PassPrimary marks a Backdrop that hands a primary press on to what lies
	// beneath after dismissing its overlay.
	PassPrimary bool
	// Layer is the compositing layer the region was declared on. SetRegions
	// expects regions ordered back to front, so a caller sorts them by it.
	Layer uint16
}

// Key is the internal alias of the public normalized keyboard identity.
type Key = gfx.Key

const (
	KeyUnknown   = gfx.KeyUnknown
	KeyTab       = gfx.KeyTab
	KeyEnter     = gfx.KeyEnter
	KeySpace     = gfx.KeySpace
	KeyEscape    = gfx.KeyEscape
	KeyLeft      = gfx.KeyLeft
	KeyRight     = gfx.KeyRight
	KeyUp        = gfx.KeyUp
	KeyDown      = gfx.KeyDown
	KeyHome      = gfx.KeyHome
	KeyEnd       = gfx.KeyEnd
	KeyBackspace = gfx.KeyBackspace
	KeyDelete    = gfx.KeyDelete
	KeyPageUp    = gfx.KeyPageUp
	KeyPageDown  = gfx.KeyPageDown
	KeyInsert    = gfx.KeyInsert
	KeyF1        = gfx.KeyF1
	KeyF2        = gfx.KeyF2
	KeyF3        = gfx.KeyF3
	KeyF4        = gfx.KeyF4
	KeyF5        = gfx.KeyF5
	KeyF6        = gfx.KeyF6
	KeyF7        = gfx.KeyF7
	KeyF8        = gfx.KeyF8
	KeyF9        = gfx.KeyF9
	KeyF10       = gfx.KeyF10
	KeyF11       = gfx.KeyF11
	KeyF12       = gfx.KeyF12

	KeyShiftLeft    = gfx.KeyShiftLeft
	KeyShiftRight   = gfx.KeyShiftRight
	KeyControlLeft  = gfx.KeyControlLeft
	KeyControlRight = gfx.KeyControlRight
	KeyAltLeft      = gfx.KeyAltLeft
	KeyAltRight     = gfx.KeyAltRight
	KeyMetaLeft     = gfx.KeyMetaLeft
	KeyMetaRight    = gfx.KeyMetaRight
)

// Modifiers is a bitset of active keyboard modifiers.
type Modifiers = gfx.Modifiers

const (
	ModifierShift   = gfx.ModifierShift
	ModifierControl = gfx.ModifierControl
	ModifierAlt     = gfx.ModifierAlt
	ModifierMeta    = gfx.ModifierMeta
)

// KeyEvent is a normalized keyboard event.
type KeyEvent struct {
	Key       Key
	Modifiers Modifiers
	Repeat    bool
}

// Result reports state changes and semantic activation caused by an event.
type Result struct {
	Target       Target
	FocusChanged bool
	Activated    bool
}

// Router retains hit regions and keyboard focus independently from platform
// event objects.
type Router struct {
	regions     []Region
	focused     Target
	focusedSlot uint32
	activeScope gfx.ID
	// activeInitial is the initial target announced with the active scope, to
	// tell a dialog that replaces another in the same scope from one that is
	// merely announced again.
	activeInitial Target
	returnFocus   Target
}

// SetRegions replaces paint-order hit regions while preserving a still-valid
// focused target. Destination storage is reused across calls.
func (router *Router) SetRegions(regions []Region) {
	// A repaint that declares the same regions (the common steady state) needs
	// no copy and no focus rescan: the retained list already routes input and
	// the focused slot is still valid.
	if len(router.regions) == len(regions) && slices.Equal(router.regions, regions) {
		return
	}
	router.regions = append(router.regions[:0], regions...)
	router.focusedSlot = 0
	if !router.focused.IsZero() {
		for index, region := range router.regions {
			if region.Target == router.focused && router.focusAvailableRegion(region) {
				router.focusedSlot = uint32(index + 1)
				break
			}
		}
	}
	if !router.focused.IsZero() && router.focusedSlot == 0 {
		router.focused = gfx.ID{}
	}
}

// Regions returns the current back-to-front regions. The result is invalidated
// by the next SetRegions call.
func (router *Router) Regions() []Region {
	return router.regions
}

// HitTest returns the front-most enabled target containing the point.
func (router *Router) HitTest(x float32, y float32) (Target, bool) {
	index, ok := router.hitTestIndex(x, y)
	if !ok {
		return gfx.ID{}, false
	}
	return router.regions[index].Target, true
}

// PressKind is what a press at a point does in a custom-titlebar window.
type PressKind uint8

const (
	// PressClient is a press the application takes and that moves nothing: the
	// zero value, and what a window with no move region answers everywhere.
	PressClient PressKind = iota
	// PressControl is a press on a control, which a window edge under it yields
	// to.
	PressControl
	// PressMove is a press on a move region with nothing of the application's
	// above it: the platform moves the window at once.
	PressMove
	// PressMoveOnDrag is a press on a move region under an overlay's backdrop:
	// the application takes it, to dismiss the overlay, and the platform moves
	// the window if the pointer drags.
	PressMoveOnDrag
)

// PressAt says what a press at the point does. It walks the regions front to
// back. A control or an opaque block above a move region ends the walk, since
// the press is theirs. A backdrop does not end it but is remembered, because it
// covers the window without being a place the window cannot be moved from. A move
// region ends it, as PressMove or, under a backdrop, PressMoveOnDrag. Move regions
// are found in every focus scope, so a modal does not stop the window from being
// moved by its titlebar.
func (router *Router) PressAt(x float32, y float32) PressKind {
	backdrop := false
	for index := len(router.regions) - 1; index >= 0; index-- {
		region := router.regions[index]
		if !contains(region.Bounds, x, y) {
			continue
		}
		switch {
		case region.MovesWindow:
			if backdrop {
				return PressMoveOnDrag
			}
			return PressMove
		case region.Blocks:
			if router.activeScope.IsZero() || region.Scope == router.activeScope {
				return PressClient
			}
		case region.Backdrop:
			if router.hitAvailable(region) {
				backdrop = true
			}
		case router.hitAvailable(region):
			return PressControl
		}
	}
	return PressClient
}

// PassesSecondary reports whether the front-most hit at the point is a backdrop
// that hands a secondary press on once it is dismissed.
func (router *Router) PassesSecondary(x float32, y float32) bool {
	index, ok := router.hitTestIndex(x, y)
	return ok && router.regions[index].PassSecondary
}

// PassesPrimary reports whether the front-most hit at the point is a backdrop
// that hands a primary press on once it is dismissed.
func (router *Router) PassesPrimary(x float32, y float32) bool {
	index, ok := router.hitTestIndex(x, y)
	return ok && router.regions[index].PassPrimary
}

func (router *Router) hitTestIndex(x float32, y float32) (int, bool) {
	for index := len(router.regions) - 1; index >= 0; index-- {
		region := router.regions[index]
		if !contains(region.Bounds, x, y) {
			continue
		}
		if region.Blocks {
			if router.activeScope.IsZero() || region.Scope == router.activeScope {
				return 0, false
			}
			continue
		}
		if router.hitAvailable(region) {
			return index, true
		}
	}
	return 0, false
}

// PointerDown hit-tests and focuses a focusable target.
func (router *Router) PointerDown(x float32, y float32) Result {
	index, ok := router.hitTestIndex(x, y)
	if !ok {
		return Result{}
	}
	region := router.regions[index]
	target := region.Target
	result := Result{Target: target}
	if region.Focusable && !target.IsZero() && router.focused != target {
		router.focused = target
		router.focusedSlot = uint32(index + 1)
		result.FocusChanged = true
	} else if region.Label && router.focused != target {
		for own, candidate := range router.regions {
			if candidate.Target == target && !candidate.Label && router.focusAvailableRegion(candidate) {
				router.focused = target
				router.focusedSlot = uint32(own + 1)
				result.FocusChanged = true
				break
			}
		}
	}
	return result
}

// KeyDown applies focus traversal and activation behavior.
func (router *Router) KeyDown(event KeyEvent) Result {
	if event.Key == KeyTab {
		return router.moveFocus(event.Modifiers&ModifierShift != 0)
	}
	if !event.Repeat && (event.Key == KeyEnter || event.Key == KeySpace) && !router.focused.IsZero() {
		if region, ok := router.focusedRegion(); ok {
			if region.Disabled || !activatesOn(region.Role, event.Key) {
				return Result{Target: router.focused}
			}
		}
		return Result{Target: router.focused, Activated: true}
	}
	return Result{Target: router.focused}
}

// focusedRegion returns the region that owns the keyboard focus.
func (router *Router) focusedRegion() (Region, bool) {
	if router.focused.IsZero() {
		return Region{}, false
	}
	for _, region := range router.regions {
		if region.Target == router.focused {
			return region, true
		}
	}
	return Region{}, false
}

// activatesOn reports whether role uses key to activate. Base UI buttons, tabs,
// menu items, options and the combobox trigger take Enter and Space; checkboxes,
// radios and switches take Space only; sliders and group/container roles take
// neither (a form would consume Enter).
func activatesOn(role gfx.Role, key Key) bool {
	switch role {
	case gfx.RoleCheckbox, gfx.RoleRadio, gfx.RoleSwitch:
		return key == KeySpace
	case gfx.RoleTextField:
		return key == KeyEnter // a form submits; a click on the field is its submit
	case gfx.RoleSlider, gfx.RoleGroup, gfx.RoleTerminal:
		return false
	case gfx.RoleButton, gfx.RoleLink, gfx.RoleTab, gfx.RoleMenuItem, gfx.RoleCombobox:
		return key == KeyEnter || key == KeySpace
	default:
		return key == KeyEnter || key == KeySpace
	}
}

// Focus moves keyboard focus to target if it is currently focusable.
func (router *Router) Focus(target Target) bool {
	if target == router.focused {
		return false
	}
	for index, region := range router.regions {
		if region.Target == target && router.focusAvailableRegion(region) {
			router.focused = target
			router.focusedSlot = uint32(index + 1)
			return true
		}
	}
	return false
}

// SetActiveScope restricts pointer and keyboard routing to one focus scope.
// Closing the outermost scope restores the previously focused target when it
// remains available in the replacement region set. When a scope opens,
// initial (if nonzero and focusable inside the scope) takes focus; announcing
// the scope that is already active never moves focus, except that a different
// nonzero initial target means another dialog has taken the scope's place and
// it takes focus as if the scope had just opened. It reports whether the
// focused target changed.
func (router *Router) SetActiveScope(scope gfx.ID, initial Target) bool {
	if scope == router.activeScope {
		if scope.IsZero() || initial.IsZero() || initial == router.activeInitial {
			return false
		}
		before := router.focused
		router.activeInitial = initial
		router.focusAvailable(initial)
		return before != router.focused
	}
	router.activeInitial = initial
	before := router.focused
	if router.activeScope.IsZero() && !scope.IsZero() {
		router.returnFocus = router.focused
	}
	router.activeScope = scope
	if scope.IsZero() {
		target := router.returnFocus
		router.returnFocus = gfx.ID{}
		router.focused = gfx.ID{}
		router.focusedSlot = 0
		if !target.IsZero() {
			router.focusAvailable(target)
		}
	} else {
		if !router.focusAvailable(router.focused) {
			router.focused = gfx.ID{}
			router.focusedSlot = 0
		}
		router.focusAvailable(initial)
	}
	return before != router.focused
}

// ActiveScope returns the current focus trap, or zero.
func (router *Router) ActiveScope() gfx.ID {
	return router.activeScope
}

// Blur clears keyboard focus.
func (router *Router) Blur() bool {
	if router.focused.IsZero() {
		return false
	}
	router.focused = gfx.ID{}
	router.focusedSlot = 0
	return true
}

// Focused returns the current focused target.
func (router *Router) Focused() (Target, bool) {
	return router.focused, !router.focused.IsZero()
}

func (router *Router) moveFocus(reverse bool) Result {
	count := len(router.regions)
	if count == 0 {
		return Result{}
	}

	start := int(router.focusedSlot) - 1
	for step := 1; step <= count; step++ {
		index := start + step
		if reverse {
			if start < 0 {
				index = count - step
			} else {
				index = start - step
				for index < 0 {
					index += count
				}
			}
		} else {
			index %= count
		}
		region := router.regions[index]
		if region.SkipTab {
			continue
		}
		if router.focusAvailableRegion(region) {
			changed := router.focused != region.Target
			router.focused = region.Target
			router.focusedSlot = uint32(index + 1)
			return Result{Target: region.Target, FocusChanged: changed}
		}
	}
	return Result{Target: router.focused}
}

func (router *Router) focusAvailable(target Target) bool {
	if target.IsZero() {
		return false
	}
	for index, region := range router.regions {
		if region.Target == target && router.focusAvailableRegion(region) {
			router.focused = target
			router.focusedSlot = uint32(index + 1)
			return true
		}
	}
	return false
}

func (router *Router) hitAvailable(region Region) bool {
	return !region.Target.IsZero() && !region.Disabled &&
		(router.activeScope.IsZero() || region.Scope == router.activeScope)
}

func (router *Router) focusAvailableRegion(region Region) bool {
	if !region.Focusable || region.Target.IsZero() {
		return false
	}
	if region.Disabled && !region.FocusableDisabled {
		return false
	}
	return router.activeScope.IsZero() || region.Scope == router.activeScope
}

func contains(bounds Rect, x float32, y float32) bool {
	return x >= bounds.X && y >= bounds.Y &&
		x < bounds.X+bounds.Width && y < bounds.Y+bounds.Height
}
