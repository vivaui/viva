package style

import (
	"github.com/vivaui/viva/gfx"
)

// MenuItemPaint is one row of a menu declared Unpainted: the caller draws it,
// and the menu keeps the row's hit, focus and keyboard behavior. Separator and
// GroupLabel say the row is not an action. Active is the row the pointer or
// the keyboard is on, and Foreground is the color the menu's own row would
// give its label.
type MenuItemPaint struct {
	Canvas      *gfx.Canvas
	Theme       gfx.Theme
	Bounds      gfx.Rect
	Label       string
	Description string
	Shortcut    string
	Checked     bool
	Selected    bool
	Expanded    bool
	Disabled    bool
	Active      bool
	Destructive bool
	Separator   bool
	GroupLabel  bool
	Foreground  gfx.Color
}

// PaginationKind is what one item of a pagination row is: a page number, the
// previous or next control, or an omission marker.
type PaginationKind uint8

const (
	PaginationPage PaginationKind = iota
	PaginationPrevious
	PaginationNext
	PaginationEllipsis
)

// PaginationItem is one item a Pagination paints. An item with no identity and
// not an ellipsis paints inert, as a current page does.
type PaginationItem struct {
	ID       gfx.ID
	Label    string
	Kind     PaginationKind
	Current  bool
	Disabled bool
}

// PopoverSide selects the edge of the anchor used by a popover.
type PopoverSide uint8

const (
	PopoverBottom PopoverSide = iota
	PopoverTop
	PopoverRight
	PopoverLeft
)

// TableAlignment selects text placement within a table column.
type TableAlignment uint8

const (
	TableAlignLeft TableAlignment = iota
	TableAlignCenter
	TableAlignRight
)

// TableColumn is a statically typed column declaration. Width is measured in
// logical pixels; the final non-positive width consumes the remaining space.
type TableColumn struct {
	Width     float32
	Alignment TableAlignment
	// Inset reserves leading content space inside a left-aligned cell, for
	// example an application-painted file icon.
	Inset float32
	// EndInset reserves trailing content space inside a left-aligned cell, for
	// example an application-painted expander.
	EndInset float32
}

// TooltipSide selects the anchor edge used by a tooltip.
type TooltipSide uint8

const (
	TooltipTop TooltipSide = iota
	TooltipRight
	TooltipBottom
	TooltipLeft
)

// The layout helpers the moved value types use, with the meaning they have in
// components.
