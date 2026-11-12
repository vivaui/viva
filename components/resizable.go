package components

import "github.com/vivaui/viva/gfx"

// ResizableOrientation selects side-by-side or stacked panels.
type ResizableOrientation uint8

const (
	ResizableHorizontal ResizableOrientation = iota
	ResizableVertical
)

// ResizableLayout is the exact panel and handle geometry for one split.
type ResizableLayout struct {
	First  gfx.Rect
	Handle gfx.Rect
	Hit    gfx.Rect
	Second gfx.Rect
}

// Resizable paints a split handle, returns panel geometry, and maps pointer
// coordinates back to a constrained fraction. Fraction and drag ownership stay
// in application state, so an idle split owns no timer, goroutine, or polling
// work.
type Resizable struct {
	canvas    *gfx.Canvas
	theme     gfx.Theme
	id        gfx.ID
	bounds    gfx.Rect
	fraction  float32
	valueText string
	minimum   float32
	maximum   float32
	hitSize   float32
	minFirst  float32 // pixel minimum of the first panel
	minSecond float32 // pixel minimum of the second panel
	step      float32 // arrow step in pixels; zero means 5% of the group
	collapsed float32 // fraction Enter collapses to; negative means the minimum
	// defaultFraction is where a double click returns the handle.
	defaultFraction float32
	orientation     ResizableOrientation
	withHandle      bool
	disabled        bool
}

// WithHandle adds the compact Vega grip pill. Without it the component paints
// only the one-pixel separator while retaining its wider hit strip.
func (resizable Resizable) WithHandle() Resizable {
	resizable.withHandle = true
	return resizable
}

// BeginResizable starts a horizontal split declaration.
func BeginResizable(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, fraction float32) Resizable {
	return Resizable{
		canvas: canvas, theme: theme, id: id, bounds: bounds, fraction: fraction,
		minimum: 0.1, maximum: 0.9, hitSize: 4, collapsed: -1, defaultFraction: fraction,
	}
}

// Vertical stacks the first panel above the second.
func (resizable Resizable) Vertical() Resizable {
	resizable.orientation = ResizableVertical
	return resizable
}

// Limits constrains the first panel's fraction.
func (resizable Resizable) Limits(minimum, maximum float32) Resizable {
	if minimum < 0 {
		minimum = 0
	}
	if maximum > 1 {
		maximum = 1
	}
	if maximum < minimum {
		maximum = minimum
	}
	resizable.minimum = minimum
	resizable.maximum = maximum
	return resizable
}

// LimitsPx adds pixel minimums for the two panels (a panel's minSize in pixels,
// which is what a desktop sidebar or inspector wants) on top of Limits. Where the
// two cannot both hold, the pixel minimums win up to the room there is.
func (resizable Resizable) LimitsPx(minFirst, minSecond float32) Resizable {
	resizable.minFirst = max(minFirst, 0)
	resizable.minSecond = max(minSecond, 0)
	return resizable
}

// Step sets how many pixels an arrow key moves the handle by. Without it an
// arrow moves 5% of the group, as Base UI's resizable panel does.
func (resizable Resizable) Step(pixels float32) Resizable {
	if pixels > 0 {
		resizable.step = pixels
	}
	return resizable
}

// FractionForKey is the keyboard behavior of a focused handle: the arrows along
// the split's axis move it by Step pixels, or 5% of the group without a Step;
// Home and End send it to the lower and upper limit. It returns the new
// first-panel fraction and whether the key was one the handle uses.
func (resizable Resizable) FractionForKey(key gfx.Key) (float32, bool) {
	fraction := resizable.clampedFraction()
	usable := resizable.bounds.Width - 1
	less, more := gfx.KeyLeft, gfx.KeyRight
	if resizable.orientation == ResizableVertical {
		usable = resizable.bounds.Height - 1
		less, more = gfx.KeyUp, gfx.KeyDown
	}
	switch key {
	case less, more:
		delta := float32(0.05)
		if resizable.step > 0 {
			if usable <= 0 {
				return fraction, true
			}
			delta = resizable.step / usable
		}
		if key == less {
			delta = -delta
		}
		return resizable.constrain(fraction + delta), true
	case gfx.KeyHome:
		return resizable.constrain(0), true
	case gfx.KeyEnd:
		return resizable.constrain(1), true
	}
	return fraction, false
}

// Collapsed sets the fraction Enter collapses the first panel to; the default
// is the lower limit.
func (resizable Resizable) Collapsed(fraction float32) Resizable {
	if fraction >= 0 && fraction <= 1 {
		resizable.collapsed = fraction
	}
	return resizable
}

// CollapseFraction is Enter on a focused handle: collapse to Collapsed (the
// lower limit by default) when the panel is open, else restore restore, the
// size it had before. It is the caller's job to remember that size.
func (resizable Resizable) CollapseFraction(current, restore float32, key gfx.Key) (float32, bool) {
	if key != gfx.KeyEnter {
		return current, false
	}
	collapsed := resizable.minimum
	if resizable.collapsed >= 0 {
		collapsed = resizable.collapsed
	}
	if current > resizable.constrain(collapsed)+0.0001 {
		return resizable.constrain(collapsed), true
	}
	return resizable.constrain(restore), true
}

// Default sets the fraction a double click restores; the initial fraction is
// the default.
func (resizable Resizable) Default(fraction float32) Resizable {
	if fraction >= 0 && fraction <= 1 {
		resizable.defaultFraction = fraction
	}
	return resizable
}

// FractionForClick is the double-click reset: a click count of two or more
// returns Default constrained by the current limits, and reports whether it
// applied.
func (resizable Resizable) FractionForClick(clickCount int) (float32, bool) {
	if clickCount < 2 {
		return resizable.clampedFraction(), false
	}
	return resizable.constrain(resizable.defaultFraction), true
}

// ResizableHandleForKey moves focus between resize handles with F6 and
// Shift+F6, wrapping and skipping zero identities. The application owns the
// handles' identities and the focus result.
func ResizableHandleForKey(key gfx.Key, handles []gfx.ID, current gfx.ID, reverse bool) (gfx.ID, bool) {
	if key != gfx.KeyF6 {
		return gfx.ID{}, false
	}
	index := -1
	for at, id := range handles {
		if id == current && !id.IsZero() {
			index = at
		}
	}
	step := 1
	if reverse {
		step = -1
	}
	for offset := 1; offset <= len(handles); offset++ {
		at := (index + step*offset) % len(handles)
		if at < 0 {
			at += len(handles)
		}
		if !handles[at].IsZero() {
			return handles[at], true
		}
	}
	return gfx.ID{}, false
}

// HandleSize overrides the logical-pixel interaction strip thickness. Panel
// geometry always reserves the official one-pixel visual separator.
func (resizable Resizable) HandleSize(size float32) Resizable {
	if size > 0 {
		resizable.hitSize = size
	}
	return resizable
}

// ValueText supplies caller-formatted accessibility value text without
// formatting or allocating in the paint path.
func (resizable Resizable) ValueText(value string) Resizable {
	resizable.valueText = value
	return resizable
}

// Disabled renders and routes the handle as inert.
func (resizable Resizable) Disabled() Resizable {
	resizable.disabled = true
	return resizable
}

// FractionAt maps a caller-supplied x or y coordinate to a constrained split.
// The coordinate is x for horizontal layouts and y for vertical layouts.
func (resizable Resizable) FractionAt(position float32) float32 {
	extent := resizable.bounds.Width
	origin := resizable.bounds.X
	if resizable.orientation == ResizableVertical {
		extent = resizable.bounds.Height
		origin = resizable.bounds.Y
	}
	usable := extent - 1
	if usable <= 0 {
		return resizable.clampedFraction()
	}
	return resizable.constrain((position - origin - 0.5) / usable)
}

// Paint draws the one-pixel separator and optional four-by-twenty-four grip. A
// hovered or pressed handle is drawn three pixels wide in the ring color
// (primary while pressed), the usual cue that a divider is draggable.
func (resizable Resizable) Paint(label string) ResizableLayout {
	layout := resizable.layout()
	if resizable.canvas == nil || layout.Handle.Width <= 0 || layout.Handle.Height <= 0 {
		return layout
	}
	enabled := !resizable.disabled && !resizable.id.IsZero()
	focused := enabled && resizable.canvas.FocusVisible() == resizable.id
	if focused {
		focus := resizable.theme.Ring
		focus.Alpha *= 0.50
		resizable.canvas.Rounded(expand(layout.Hit, 1), 2, focus)
	}
	separator := resizable.theme.Border
	if resizable.disabled {
		separator.Alpha *= 0.50
	}
	resizable.canvas.Fill(layout.Handle, separator)
	// The pointer resting on the handle, or dragging it, thickens the line in
	// the accent color: without it nothing says the divider can be moved.
	if enabled && !focused {
		pressed := resizable.canvas.Pressed() == resizable.id
		if pressed || resizable.canvas.Hovered() == resizable.id {
			line := resizable.theme.Ring
			if pressed {
				line = resizable.theme.Primary
			}
			resizable.canvas.Rounded(expand(layout.Handle, 1), 1, line)
		}
	}
	if resizable.withHandle {
		resizable.paintGrip(layout.Handle)
	}
	if !resizable.id.IsZero() {
		cursor := gfx.PointerCursorResizeHorizontal
		if resizable.orientation == ResizableVertical {
			cursor = gfx.PointerCursorResizeVertical
		}
		resizable.canvas.HitWithOptions(resizable.id, layout.Hit, label, gfx.HitOptions{
			Role: gfx.RoleSeparator, Cursor: cursor, Value: resizable.valueText,
			Disabled: resizable.disabled,
		})
	}
	return layout
}

func (resizable Resizable) layout() ResizableLayout {
	fraction := resizable.clampedFraction()
	if resizable.orientation == ResizableVertical {
		usable := resizable.bounds.Height - 1
		if usable < 0 {
			usable = 0
		}
		firstHeight := usable * fraction
		handle := gfx.Rect{X: resizable.bounds.X, Y: resizable.bounds.Y + firstHeight, Width: resizable.bounds.Width, Height: 1}
		return ResizableLayout{
			First:  gfx.Rect{X: resizable.bounds.X, Y: resizable.bounds.Y, Width: resizable.bounds.Width, Height: firstHeight},
			Handle: handle,
			Hit:    resizable.hitBounds(handle),
			Second: gfx.Rect{X: resizable.bounds.X, Y: handle.Y + handle.Height, Width: resizable.bounds.Width, Height: usable - firstHeight},
		}
	}
	usable := resizable.bounds.Width - 1
	if usable < 0 {
		usable = 0
	}
	firstWidth := usable * fraction
	handle := gfx.Rect{X: resizable.bounds.X + firstWidth, Y: resizable.bounds.Y, Width: 1, Height: resizable.bounds.Height}
	return ResizableLayout{
		First:  gfx.Rect{X: resizable.bounds.X, Y: resizable.bounds.Y, Width: firstWidth, Height: resizable.bounds.Height},
		Handle: handle,
		Hit:    resizable.hitBounds(handle),
		Second: gfx.Rect{X: handle.X + handle.Width, Y: resizable.bounds.Y, Width: usable - firstWidth, Height: resizable.bounds.Height},
	}
}

func (resizable Resizable) hitBounds(handle gfx.Rect) gfx.Rect {
	size := max(resizable.hitSize, float32(1))
	if resizable.orientation == ResizableVertical {
		return gfx.Rect{X: handle.X, Y: handle.Y - (size-1)/2, Width: handle.Width, Height: size}
	}
	return gfx.Rect{X: handle.X - (size-1)/2, Y: handle.Y, Width: size, Height: handle.Height}
}

func (resizable Resizable) clampedFraction() float32 {
	return resizable.constrain(resizable.fraction)
}

// constrain applies the fraction limits, then the pixel minimums.
func (resizable Resizable) constrain(fraction float32) float32 {
	fraction = min(max(fraction, resizable.minimum), resizable.maximum)
	usable := resizable.bounds.Width - 1
	if resizable.orientation == ResizableVertical {
		usable = resizable.bounds.Height - 1
	}
	if usable <= 0 || resizable.minFirst == 0 && resizable.minSecond == 0 {
		return fraction
	}
	low := min(resizable.minFirst/usable, 1)
	high := max(1-resizable.minSecond/usable, 0)
	if low > high {
		// Not both fit: split the shortfall by giving each its share of the room.
		return low / (low + (1 - high))
	}
	return min(max(fraction, low), high)
}

func (resizable Resizable) paintGrip(handle gfx.Rect) {
	color := resizable.theme.Border
	if resizable.disabled {
		color.Alpha *= 0.50
	}
	if resizable.orientation == ResizableHorizontal {
		resizable.canvas.Rounded(gfx.Rect{X: handle.X - 1.5, Y: handle.Y + handle.Height/2 - 12, Width: 4, Height: 24}, 2, color)
		return
	}
	resizable.canvas.Rounded(gfx.Rect{X: handle.X + handle.Width/2 - 12, Y: handle.Y - 1.5, Width: 24, Height: 4}, 2, color)
}
