package components

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// Pagination incrementally paints a compact page chooser. Each Add call emits
// commands immediately, so callers do not need a registry or collected model.
type Pagination struct {
	canvas *gfx.Canvas
	theme  gfx.Theme
	bounds gfx.Rect
	x      float32
}

// chevronLeftIcon is the previous-page arrow.
var chevronLeftIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(15, 18), gfx.LineTo(9, 12), gfx.LineTo(15, 6)}}

// BeginPagination starts a left-aligned pagination row. Its items are
// as tall as a default Button (ControlHeight, size-8) unless bounds is shorter.
func BeginPagination(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) Pagination {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	return Pagination{canvas: canvas, theme: theme, bounds: bounds, x: bounds.X}
}

// AddPrevious adds a previous-page control. Disabled controls paint but do not
// emit a hit region.
func (pagination *Pagination) AddPrevious(id gfx.ID, label string, disabled bool) {
	pagination.add(id, label, PaginationPrevious, false, disabled)
}

// AddPage adds one page. Current pages use the blue accent and are inert.
func (pagination *Pagination) AddPage(id gfx.ID, label string, current bool) {
	pagination.add(id, label, PaginationPage, current, false)
}

// AddEllipsis paints a non-interactive omission marker.
func (pagination *Pagination) AddEllipsis() {
	pagination.add(gfx.ID{}, "…", PaginationEllipsis, false, true)
}

// AddNext adds a next-page control. Disabled controls paint but do not emit a
// hit region.
func (pagination *Pagination) AddNext(id gfx.ID, label string, disabled bool) {
	pagination.add(id, label, PaginationNext, false, disabled)
}

// Add paints one item.
func (pagination *Pagination) Add(item PaginationItem) {
	pagination.add(item.ID, item.Label, item.Kind, item.Current, item.Disabled)
}

// PaginationExtent is how wide items are laid side by side, gaps included, and
// how tall they paint (a control's height). A caller lays a row out with it
// before painting the items into a rectangle at least that wide.
func PaginationExtent(canvas *gfx.Canvas, theme gfx.Theme, items []PaginationItem) (width, height float32) {
	metrics := theme.Metrics
	height = metrics.ControlHeight
	for _, item := range items {
		width += paginationItemWidth(canvas, metrics, item.Kind, item.Label, metrics.ControlHeight)
	}
	if len(items) > 1 {
		width += float32(len(items)-1) * metrics.GapItem / 2
	}
	return width, height
}

// paginationItemWidth is the width one item takes, before the gap after it. A
// page or the ellipsis is square, as tall as the row; a previous or next item is
// a chevron, a gap and its label.
func paginationItemWidth(canvas *gfx.Canvas, metrics gfx.Metrics, kind PaginationKind, label string, itemHeight float32) float32 {
	if kind != PaginationPrevious && kind != PaginationNext {
		return itemHeight
	}
	edge, inner := metrics.GapItem, metrics.GapItem*1.25
	gap := metrics.GapItem * 0.75
	textWidth := canvas.TextWidthWeight(label, metrics.FontSizeM, gfx.WeightMedium)
	return edge + 16 + gap + textWidth + inner
}

func (pagination *Pagination) add(id gfx.ID, label string, kind PaginationKind, current, disabled bool) {
	if pagination.x >= pagination.bounds.X+pagination.bounds.Width {
		return
	}
	metrics := pagination.theme.Metrics
	itemHeight := pagination.bounds.Height
	if itemHeight > metrics.ControlHeight {
		itemHeight = metrics.ControlHeight
	}
	if itemHeight < 0 {
		itemHeight = 0
	}
	textWidth := pagination.canvas.TextWidthWeight(label, metrics.FontSizeM, gfx.WeightMedium)
	// Previous and Next are a Button's px-2.5 (pl-2/pr-2 on the icon side) around
	// a 16px chevron, a gap-1.5 and the label; pages and the ellipsis are squares.
	edge, inner := metrics.GapItem, metrics.GapItem*1.25
	gap := metrics.GapItem * 0.75
	width := paginationItemWidth(pagination.canvas, metrics, kind, label, itemHeight)
	remaining := pagination.bounds.X + pagination.bounds.Width - pagination.x
	if width > remaining {
		width = remaining
	}
	if width <= 0 {
		return
	}
	bounds := gfx.Rect{X: pagination.x, Y: pagination.bounds.Y, Width: width, Height: itemHeight}
	foreground := pagination.theme.Foreground
	background := pagination.theme.Card
	paintSurface := false
	paintBorder := false
	interactive := !id.IsZero() && !disabled && kind != PaginationEllipsis
	if disabled {
		foreground = pagination.theme.MutedForeground
	}
	if current {
		background = pagination.theme.Background
		foreground = pagination.theme.Foreground
		paintSurface = true
		paintBorder = true
	}
	if interactive && pagination.canvas.Pressed() == id {
		background = pagination.theme.Muted
		paintSurface = true
	} else if interactive && pagination.canvas.Hovered() == id {
		background = pagination.theme.Hover
		paintSurface = true
	}
	radius := metrics.RadiusItem()
	if interactive && pagination.canvas.FocusVisible() == id {
		ring := pagination.theme.Primary
		ring.Alpha = 0.30
		pagination.canvas.Rounded(gfx.Rect{X: bounds.X - 1.5, Y: bounds.Y - 1.5, Width: bounds.Width + 3, Height: bounds.Height + 3}, radius+1.5, ring)
	}
	if paintSurface {
		if paintBorder {
			pagination.canvas.RoundedBorder(bounds, radius, 1, pagination.theme.Border, background)
		} else {
			pagination.canvas.Rounded(bounds, radius, background)
		}
	}
	style := gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground}
	switch kind {
	case PaginationPrevious:
		paintChevron(pagination.canvas, &chevronLeftIcon, gfx.Rect{X: bounds.X + edge, Y: bounds.Y, Width: 16, Height: bounds.Height}, foreground)
		pagination.canvas.LabelCenter(label, gfx.Rect{X: bounds.X + edge + 16 + gap, Y: bounds.Y, Width: textWidth, Height: bounds.Height}, style)
	case PaginationNext:
		pagination.canvas.LabelCenter(label, gfx.Rect{X: bounds.X + inner, Y: bounds.Y, Width: textWidth, Height: bounds.Height}, style)
		paintChevron(pagination.canvas, &chevronRightIcon, gfx.Rect{X: bounds.X + bounds.Width - edge - 16, Y: bounds.Y, Width: 16, Height: bounds.Height}, foreground)
	case PaginationEllipsis:
		paintChevron(pagination.canvas, &moreHorizontalIcon, bounds, foreground)
	default:
		pagination.canvas.LabelCenter(label, bounds, style)
	}
	if interactive {
		pagination.canvas.HitWithLabel(id, bounds, label)
	}
	pagination.x += width + metrics.GapItem/2
}

// Aliases into the style vocabulary, kept for callers of this component.
type PaginationItem = style.PaginationItem
type PaginationKind = style.PaginationKind

const (
	PaginationEllipsis = style.PaginationEllipsis
	PaginationNext     = style.PaginationNext
	PaginationPage     = style.PaginationPage
	PaginationPrevious = style.PaginationPrevious
)
