package components

import "github.com/vivaui/viva/gfx"

// ColumnResizeLayout is the geometry of a row of columns and the dividers
// between them. The caller passes the slices it wants filled, so a paint can
// reuse them.
type ColumnResizeLayout struct {
	Columns  []gfx.Rect
	Dividers []gfx.Rect
	Hits     []gfx.Rect
}

// ColumnResizeEdit is what a drag or a double click does to two neighbouring
// columns: their new widths. Their sum does not change, so the rest of the row
// stays where it is.
type ColumnResizeEdit struct {
	Left, Right float32
}

// ColumnResize describes a row of columns whose dividers the user may drag.
// Widths stay in application state, as with Resizable: the component gives the
// geometry, the pointer mapping and the clamped arithmetic; the page owns the
// press and writes the widths it gets back.
//
// The dividers lie in the gaps between columns. A caller that wants a column to
// take the room left over should give every column an explicit width and update
// that column when its bounds change; that keeps a drag and a resize from
// fighting over it.
type ColumnResize struct {
	canvas   *gfx.Canvas
	theme    gfx.Theme
	bounds   gfx.Rect
	widths   []float32
	minimums []float32
	fits     []float32
	ids      []gfx.ID
	gap      float32
	hitSize  float32
}

// BeginColumnResize starts the description of a row of columns. widths is the
// application's current widths, one per column.
func BeginColumnResize(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, widths []float32) ColumnResize {
	return ColumnResize{canvas: canvas, theme: theme, bounds: bounds, widths: widths, hitSize: 4}
}

// Dividers gives the identity of each divider's hit region, left to right.
func (resize ColumnResize) Dividers(ids []gfx.ID) ColumnResize {
	resize.ids = ids
	return resize
}

// Minimums gives the least width of each column.
func (resize ColumnResize) Minimums(minimums []float32) ColumnResize {
	resize.minimums = minimums
	return resize
}

// Fits gives the width each column needs for its content, which a double click
// applies.
func (resize ColumnResize) Fits(fits []float32) ColumnResize {
	resize.fits = fits
	return resize
}

// Gap sets the room between two columns; the dividers sit in the middle of it.
func (resize ColumnResize) Gap(gap float32) ColumnResize {
	resize.gap = max(gap, 0)
	return resize
}

// HandleSize overrides the logical-pixel interaction strip centered on a
// divider. The visible line stays one pixel wide.
func (resize ColumnResize) HandleSize(size float32) ColumnResize {
	resize.hitSize = max(size, 1)
	return resize
}

// Layout fills columns, dividers and hits with this frame's geometry.
func (resize ColumnResize) Layout(columns, dividers, hits []gfx.Rect) ColumnResizeLayout {
	count := len(resize.widths)
	if cap(columns) >= count {
		columns = columns[:count]
	} else {
		columns = make([]gfx.Rect, count)
	}
	for index := range columns {
		columns[index] = gfx.Rect{
			X: resize.columnLeft(index), Y: resize.bounds.Y,
			Width: resize.width(index), Height: resize.bounds.Height,
		}
	}
	dividerCount := max(count-1, 0)
	if cap(dividers) >= dividerCount {
		dividers = dividers[:dividerCount]
	} else {
		dividers = make([]gfx.Rect, dividerCount)
	}
	if cap(hits) >= dividerCount {
		hits = hits[:dividerCount]
	} else {
		hits = make([]gfx.Rect, dividerCount)
	}
	size := max(resize.hitSize, 1)
	for index := range dividers {
		x := resize.dividerX(index)
		dividers[index] = gfx.Rect{X: x, Y: resize.bounds.Y, Width: 1, Height: resize.bounds.Height}
		hits[index] = gfx.Rect{X: x - (size-1)/2, Y: resize.bounds.Y, Width: size, Height: resize.bounds.Height}
	}
	return ColumnResizeLayout{Columns: columns, Dividers: dividers, Hits: hits}
}

// Paint draws the dividers and registers their hit regions. A hovered or pressed
// divider is drawn thicker in the ring or primary color, as Resizable's handle
// is. labels names each divider for assistive technology.
func (resize ColumnResize) Paint(columns, dividers, hits []gfx.Rect, labels []string) ColumnResizeLayout {
	layout := resize.Layout(columns, dividers, hits)
	if resize.canvas == nil {
		return layout
	}
	for index := range layout.Dividers {
		divider := layout.Dividers[index]
		if divider.Width <= 0 || divider.Height <= 0 {
			continue
		}
		resize.canvas.Fill(divider, resize.theme.Border)
		var id gfx.ID
		if index < len(resize.ids) {
			id = resize.ids[index]
		}
		if id.IsZero() {
			continue
		}
		pressed := resize.canvas.Pressed() == id
		if pressed || resize.canvas.Hovered() == id {
			line := resize.theme.Ring
			if pressed {
				line = resize.theme.Primary
			}
			resize.canvas.Rounded(expand(divider, 1), 1, line)
		}
		label := ""
		if index < len(labels) {
			label = labels[index]
		}
		resize.canvas.HitWithOptions(id, layout.Hits[index], label, gfx.HitOptions{
			Role: gfx.RoleSeparator, Cursor: gfx.PointerCursorResizeHorizontal,
		})
	}
	return layout
}

// Edit maps a pointer coordinate to the new widths of the columns either side of
// divider. The pair keeps its total, and neither column goes below its minimum.
func (resize ColumnResize) Edit(divider int, x float32) (ColumnResizeEdit, bool) {
	if divider < 0 || divider+1 >= len(resize.widths) {
		return ColumnResizeEdit{}, false
	}
	total := resize.width(divider) + resize.width(divider+1)
	left := x - resize.columnLeft(divider) - resize.gap/2
	left = resize.clampPair(divider, left, total)
	return ColumnResizeEdit{Left: left, Right: total - left}, true
}

// Fit is the double-click behavior: the left column takes the width its content
// needs, and its neighbour gives up or gains the difference.
func (resize ColumnResize) Fit(divider int) (ColumnResizeEdit, bool) {
	if divider < 0 || divider+1 >= len(resize.widths) {
		return ColumnResizeEdit{}, false
	}
	total := resize.width(divider) + resize.width(divider+1)
	left := resize.width(divider)
	if divider < len(resize.fits) && resize.fits[divider] > 0 {
		left = resize.fits[divider]
	}
	left = resize.clampPair(divider, left, total)
	return ColumnResizeEdit{Left: left, Right: total - left}, true
}

func (resize ColumnResize) width(index int) float32 {
	width := resize.widths[index]
	if index < len(resize.minimums) {
		width = max(width, max(resize.minimums[index], 0))
	}
	return max(width, 0)
}

func (resize ColumnResize) minimum(index int) float32 {
	if index < len(resize.minimums) {
		return max(resize.minimums[index], 0)
	}
	return 0
}

func (resize ColumnResize) columnLeft(index int) float32 {
	x := resize.bounds.X
	for at := 0; at < index && at < len(resize.widths); at++ {
		x += resize.width(at) + resize.gap
	}
	return x
}

func (resize ColumnResize) dividerX(index int) float32 {
	return resize.columnLeft(index) + resize.width(index) + resize.gap/2
}

// clampPair keeps both widths at their minimums. When the pair cannot hold both,
// the shortfall is shared in proportion to the minimums, as Resizable does.
func (resize ColumnResize) clampPair(divider int, left, total float32) float32 {
	low := resize.minimum(divider)
	high := resize.minimum(divider + 1)
	if low+high > total {
		if low+high <= 0 {
			return total / 2
		}
		return total * low / (low + high)
	}
	return min(max(left, low), total-high)
}
