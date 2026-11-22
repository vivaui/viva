package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// TabsKeyDown applies orientation-aware roving focus to current. Arrow keys on
// the tab list's layout axis wrap through enabled non-zero options; Home and
// End choose the first and last. Horizontal Left and Right reverse in RTL.
func TabsKeyDown(current gfx.ID, options []TabOption, orientation TabsOrientation, direction WritingDirection, key gfx.Key) (gfx.ID, bool) {
	step := 0
	switch key {
	case gfx.KeyLeft:
		if orientation != TabsHorizontal {
			return current, false
		}
		step = -1
		if direction.IsRTL() {
			step = 1
		}
	case gfx.KeyRight:
		if orientation != TabsHorizontal {
			return current, false
		}
		step = 1
		if direction.IsRTL() {
			step = -1
		}
	case gfx.KeyUp:
		if orientation != TabsVertical {
			return current, false
		}
		step = -1
	case gfx.KeyDown:
		if orientation != TabsVertical {
			return current, false
		}
		step = 1
	case gfx.KeyHome:
		return firstEnabledTab(current, options, 1), true
	case gfx.KeyEnd:
		return firstEnabledTab(current, options, -1), true
	default:
		return current, false
	}
	return nextEnabledTab(current, options, step), true
}

func firstEnabledTab(current gfx.ID, options []TabOption, step int) gfx.ID {
	for index := 0; index < len(options); index++ {
		candidate := index
		if step < 0 {
			candidate = len(options) - 1 - index
		}
		option := options[candidate]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

func nextEnabledTab(current gfx.ID, options []TabOption, step int) gfx.ID {
	if len(options) == 0 {
		return current
	}
	currentIndex := -1
	for index := range options {
		if options[index].ID == current {
			currentIndex = index
			break
		}
	}
	if currentIndex < 0 {
		return firstEnabledTab(current, options, step)
	}
	for offset := 1; offset <= len(options); offset++ {
		index := (currentIndex + step*offset) % len(options)
		if index < 0 {
			index += len(options)
		}
		option := options[index]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

// SwitchExtent is the width and height of a switch at the theme's density.
func SwitchExtent(theme gfx.Theme, small bool) (width, height float32) {
	scale := theme.Metrics.ControlHeight / 32
	if small {
		return 24 * scale, 14 * scale
	}
	return 32 * scale, 18.4 * scale
}

// SliderKeyDown applies Base UI's slider keyboard pattern to value with Base
// UI's default large step of 10 value units: the arrows move it by one step
// (Right and Up up, Left and Down down; Left and Right swap in RTL), Shift with
// an arrow and PageUp/PageDown by 10 units, Home and End to the minimum and
// maximum. A zero or negative step moves by one unit. The result is clamped and
// snapped like Slider.Step does; the returned bool reports whether key is owned
// by the slider pattern, so the caller passes anything else on.
//
//	next, handled := components.SliderKeyDown(m.volume, 0, 100, 5, components.LTR, event.Key, event.Modifiers)
func SliderKeyDown(value, minimum, maximum, step float32, direction WritingDirection, key gfx.Key, modifiers gfx.Modifiers) (float32, bool) {
	return SliderKeyDownLarge(value, minimum, maximum, step, 10, direction, key, modifiers)
}

// CheckboxExtent is the side of a checkbox's square at the theme's density.
func CheckboxExtent(theme gfx.Theme) float32 { return 16 * theme.Metrics.ControlHeight / 32 }

// ClampOverlay keeps an overlay opened at point inside viewport, except for its
// margin. Unlike PlaceOverlay, a context menu that fits above the pointer but
// would clamp below it stays at the pointer's y; that is how a native context
// menu opens at the mouse rather than choosing a side.
func ClampOverlay(point gfx.Point, size gfx.Size, viewport gfx.Rect, margin float32) gfx.Rect {
	return OverlayClamp(gfx.Rect{X: point.X, Y: point.Y, Width: size.Width, Height: size.Height}, viewport, margin)
}

// SegmentedControlKeyDown applies horizontal roving focus to current. Left and
// Right wrap through enabled non-zero options and reverse in RTL; Home and End
// choose the first and last enabled options. Vertical arrows are not owned by
// a horizontal segmented control.
func SegmentedControlKeyDown(current gfx.ID, options []style.SegmentOption, direction WritingDirection, key gfx.Key) (gfx.ID, bool) {
	step := 0
	switch key {
	case gfx.KeyLeft:
		step = -1
		if direction.IsRTL() {
			step = 1
		}
	case gfx.KeyRight:
		step = 1
		if direction.IsRTL() {
			step = -1
		}
	case gfx.KeyHome:
		return firstEnabledSegment(current, options, 1), true
	case gfx.KeyEnd:
		return firstEnabledSegment(current, options, -1), true
	default:
		return current, false
	}
	return nextEnabledSegment(current, options, step), true
}

func firstEnabledSegment(current gfx.ID, options []style.SegmentOption, step int) gfx.ID {
	for index := 0; index < len(options); index++ {
		candidate := index
		if step < 0 {
			candidate = len(options) - 1 - index
		}
		option := options[candidate]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

func nextEnabledSegment(current gfx.ID, options []style.SegmentOption, step int) gfx.ID {
	if len(options) == 0 {
		return current
	}
	currentIndex := -1
	for index := range options {
		if options[index].ID == current {
			currentIndex = index
			break
		}
	}
	if currentIndex < 0 {
		return firstEnabledSegment(current, options, step)
	}
	for offset := 1; offset <= len(options); offset++ {
		index := (currentIndex + step*offset) % len(options)
		if index < 0 {
			index += len(options)
		}
		option := options[index]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

// TableRowHeight is the default height of a body or footer row: TableCell's p-2
// around one 20px line, scaled to the theme's control height.
func TableRowHeight(theme gfx.Theme) float32 { return theme.Metrics.ControlHeight + 4 }

// TableHeaderHeight is the default height of the header row, TableHead's h-10.
func TableHeaderHeight(theme gfx.Theme) float32 { return theme.Metrics.ControlHeight + 8 }

// TableCellBounds appends to dst the cells of a row that fills row, as Header and
// Row paint them: fixed widths in order, a column without one taking what is left,
// cells that no longer fit left out. It lets a caller put something on a cell, a
// sort arrow or a hit region, without painting the row first.
func TableCellBounds(columns []style.TableColumn, row gfx.Rect, dst []gfx.Rect) []gfx.Rect {
	x, remaining := row.X, row.Width
	for index, column := range columns {
		width := TableColumnWidth(column, index == len(columns)-1, remaining)
		if width <= 0 {
			break
		}
		dst = append(dst, gfx.Rect{X: x, Y: row.Y, Width: width, Height: row.Height})
		x += width
		remaining -= width
	}
	return dst
}

// tableColumnWidth is the width a column gets of the room remaining: its own, or
// all that is left for a column with no width (and for a last one too wide).
func TableColumnWidth(column style.TableColumn, last bool, remaining float32) float32 {
	width := column.Width
	if width <= 0 || last && width > remaining {
		width = remaining
	}
	return min(width, remaining)
}

// TabOption is one stable tab trigger.

// TabsOrientation selects a row or column list.

// TabsVariant selects a contained list or an indicator-only list.

const (
	TabsHorizontal TabsOrientation = iota
	TabsVertical
)

const (
	TabsDefault TabsVariant = iota
	TabsLine
)

// overlayClamp shrinks bounds to fit viewport less margin and shifts them back
// inside it.
func OverlayClamp(bounds, viewport gfx.Rect, margin float32) gfx.Rect {
	left, top := viewport.X+margin, viewport.Y+margin
	right, bottom := viewport.X+viewport.Width-margin, viewport.Y+viewport.Height-margin
	bounds.Width = min(bounds.Width, max(right-left, 0))
	bounds.Height = min(bounds.Height, max(bottom-top, 0))
	bounds.X = max(min(bounds.X, right-bounds.Width), left)
	bounds.Y = max(min(bounds.Y, bottom-bounds.Height), top)
	return bounds
}

type TabsVariant uint8

type TabsOrientation uint8

type TabOption struct {
	ID       gfx.ID
	Label    string
	Disabled bool
	// Icon is an optional leading icon. Icon and label form one centered group.
	Icon gfx.Icon
	// IconOnly hides the label while keeping it as the tab's accessible name.
	// Set Label even when only the icon is painted.
	IconOnly bool
}
