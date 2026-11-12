package components

import "github.com/vivaui/viva/gfx"

type breadcrumbItemKind uint8

const (
	breadcrumbLink breadcrumbItemKind = iota
	breadcrumbDisabled
	breadcrumbCurrent
	breadcrumbEllipsis
)

// Breadcrumb incrementally paints one breadcrumb row without collecting items
// in a slice. Each Add call emits its commands immediately and advances the
// insertion point, keeping the paint path allocation-free after warm-up.
type Breadcrumb struct {
	canvas *gfx.Canvas
	theme  gfx.Theme
	bounds gfx.Rect
	x      float32
	first  bool
	size   float32 // font size; zero selects the theme's body size
}

// moreHorizontalIcon is Lucide's three dots, the breadcrumb ellipsis.
var moreHorizontalIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(5, 12), gfx.LineTo(5.01, 12), gfx.MoveTo(12, 12), gfx.LineTo(12.01, 12), gfx.MoveTo(19, 12), gfx.LineTo(19.01, 12)}}

const breadcrumbEllipsisBox = 20 // size-5

// BeginBreadcrumb starts a compact breadcrumb row.
func BeginBreadcrumb(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) Breadcrumb {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	return Breadcrumb{canvas: canvas, theme: theme, bounds: bounds, x: bounds.X, first: true}
}

// FontSize sets the size of every item and separator. The default is the
// theme's body size (text-sm); a denser path, say in a toolbar, can pass the
// small size.
func (breadcrumb Breadcrumb) FontSize(size float32) Breadcrumb {
	breadcrumb.size = size
	return breadcrumb
}

func (breadcrumb *Breadcrumb) fontSize() float32 {
	if breadcrumb.size > 0 {
		return breadcrumb.size
	}
	return breadcrumb.theme.Metrics.FontSizeM
}

// AddLink paints an enabled destination and declares its hit region. ID 0 is
// useful for a non-interactive preview and emits no hit.
func (breadcrumb *Breadcrumb) AddLink(id gfx.ID, label string) {
	breadcrumb.paintItem(id, label, breadcrumbLink)
}

// AddDisabled paints an unavailable destination without a hit region.
func (breadcrumb *Breadcrumb) AddDisabled(label string) {
	breadcrumb.paintItem(gfx.ID{}, label, breadcrumbDisabled)
}

// AddCurrent paints the current destination with stronger emphasis.
func (breadcrumb *Breadcrumb) AddCurrent(label string) {
	breadcrumb.paintItem(gfx.ID{}, label, breadcrumbCurrent)
}

// AddEllipsis paints the overflow item: three dots in a 20px box. A non-zero id
// makes it a button, typically opening a menu of the omitted levels; a zero id
// leaves it presentational.
func (breadcrumb *Breadcrumb) AddEllipsis(id gfx.ID) {
	breadcrumb.paintItem(id, "…", breadcrumbEllipsis)
}

func (breadcrumb *Breadcrumb) paintItem(id gfx.ID, label string, kind breadcrumbItemKind) {
	if label == "" || breadcrumb.x >= breadcrumb.bounds.X+breadcrumb.bounds.Width {
		return
	}
	if !breadcrumb.first {
		separatorAdvance := breadcrumb.separatorGap()*2 + breadcrumb.fontSize()
		if breadcrumb.x+separatorAdvance >= breadcrumb.bounds.X+breadcrumb.bounds.Width {
			return
		}
		breadcrumb.paintSeparator()
	}
	naturalWidth := breadcrumb.canvas.TextWidth(label, breadcrumb.fontSize())
	if kind == breadcrumbEllipsis {
		naturalWidth = breadcrumbEllipsisBox
	}
	width := breadcrumb.clampWidth(naturalWidth)
	if width <= 0 {
		return
	}
	breadcrumb.first = false
	itemBounds := gfx.Rect{
		X: breadcrumb.x, Y: breadcrumb.bounds.Y,
		Width: width, Height: breadcrumb.bounds.Height,
	}
	color := breadcrumb.theme.MutedForeground
	weight := gfx.WeightRegular
	switch kind {
	case breadcrumbLink:
		if !id.IsZero() && (breadcrumb.canvas.Pressed() == id || breadcrumb.canvas.Hovered() == id) {
			color = breadcrumb.theme.Foreground
		}
		if !id.IsZero() {
			breadcrumb.canvas.HitWithOptions(id, itemBounds, label, gfx.HitOptions{
				Role: gfx.RoleLink,
			})
		}
	case breadcrumbDisabled:
		color.Alpha *= 0.55
	case breadcrumbCurrent:
		color = breadcrumb.theme.Foreground
	case breadcrumbEllipsis:
		if !id.IsZero() {
			if breadcrumb.canvas.Pressed() == id || breadcrumb.canvas.Hovered() == id {
				color = breadcrumb.theme.Foreground
			}
			breadcrumb.canvas.HitWithOptions(id, gfx.Rect{
				X: itemBounds.X, Y: itemBounds.Y + (itemBounds.Height-breadcrumbEllipsisBox)/2, Width: width, Height: breadcrumbEllipsisBox,
			}, "More", gfx.HitOptions{Role: gfx.RoleButton})
		}
	}
	if kind == breadcrumbEllipsis {
		breadcrumb.canvas.DrawIcon(moreHorizontalIcon, gfx.Rect{
			X: itemBounds.X + (width-16)/2, Y: itemBounds.Y + (itemBounds.Height-16)/2, Width: 16, Height: 16,
		}, color)
	} else {
		breadcrumb.canvas.Label(label, itemBounds, gfx.TextStyle{
			Size: breadcrumb.fontSize(), Weight: weight, Color: color,
		})
	}
	breadcrumb.x += width
}

// separatorGap is the list's gap-2.5 on each side of a separator.
func (breadcrumb *Breadcrumb) separatorGap() float32 {
	return breadcrumb.theme.Metrics.GapItem * 1.25
}

// paintSeparator draws the chevron, as tall as the text is.
func (breadcrumb *Breadcrumb) paintSeparator() {
	gap := breadcrumb.separatorGap()
	breadcrumb.x += gap
	width := breadcrumb.clampWidth(breadcrumb.fontSize())
	if width > 0 {
		paintChevronSized(breadcrumb.canvas, &chevronRightIcon, gfx.Rect{
			X: breadcrumb.x, Y: breadcrumb.bounds.Y, Width: width, Height: breadcrumb.bounds.Height,
		}, width, breadcrumb.theme.MutedForeground)
		breadcrumb.x += width
	}
	breadcrumb.x += gap
}

func (breadcrumb *Breadcrumb) clampWidth(width float32) float32 {
	remaining := breadcrumb.bounds.X + breadcrumb.bounds.Width - breadcrumb.x
	if remaining <= 0 {
		return 0
	}
	if width > remaining {
		return remaining
	}
	return width
}
