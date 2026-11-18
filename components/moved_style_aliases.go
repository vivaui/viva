package components

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// The tabs and metric vocabulary is style's; these aliases keep the components
// API they were published under.

type TabOption = style.TabOption
type TabsOrientation = style.TabsOrientation
type TabsVariant = style.TabsVariant

const (
	TabsHorizontal = style.TabsHorizontal
	TabsVertical   = style.TabsVertical
	TabsDefault    = style.TabsDefault
	TabsLine       = style.TabsLine
)

func TabsKeyDown(current gfx.ID, options []TabOption, orientation TabsOrientation, direction Direction, key gfx.Key) (gfx.ID, bool) {
	return style.TabsKeyDown(current, options, orientation, style.Direction(direction), key)
}

func SegmentedControlKeyDown(current gfx.ID, options []SegmentOption, direction Direction, key gfx.Key) (gfx.ID, bool) {
	return style.SegmentedControlKeyDown(current, options, style.Direction(direction), key)
}

func SwitchExtent(theme gfx.Theme, on bool) (width, height float32) {
	return style.SwitchExtent(theme, on)
}

func CheckboxExtent(theme gfx.Theme) float32 { return style.CheckboxExtent(theme) }

func TableRowHeight(theme gfx.Theme) float32 { return style.TableRowHeight(theme) }

func TableHeaderHeight(theme gfx.Theme) float32 { return style.TableHeaderHeight(theme) }

func TableCellBounds(columns []TableColumn, row gfx.Rect, dst []gfx.Rect) []gfx.Rect {
	return style.TableCellBounds(columns, row, dst)
}

func ClampOverlay(point gfx.Point, size gfx.Size, viewport gfx.Rect, margin float32) gfx.Rect {
	return style.ClampOverlay(point, size, viewport, margin)
}

func overlayClamp(bounds, viewport gfx.Rect, margin float32) gfx.Rect {
	return style.OverlayClamp(bounds, viewport, margin)
}

func SliderKeyDown(value, minimum, maximum, step float32, direction Direction, key gfx.Key, modifiers gfx.Modifiers) (float32, bool) {
	return style.SliderKeyDown(value, minimum, maximum, step, style.Direction(direction), key, modifiers)
}

func tableColumnWidth(column TableColumn, last bool, remaining float32) float32 {
	return style.TableColumnWidth(column, last, remaining)
}

func TextareaMovementForKey(key gfx.Key, modifiers gfx.Modifiers) (TextareaMovement, bool) {
	return style.TextareaMovementForKey(key, modifiers)
}
