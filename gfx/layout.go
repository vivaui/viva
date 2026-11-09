package gfx

// Insets describe uniform per-edge padding in logical pixels.
type Insets struct {
	Top    float32
	Right  float32
	Bottom float32
	Left   float32
}

// All returns insets with the same value on every edge.
func All(value float32) Insets {
	return Insets{Top: value, Right: value, Bottom: value, Left: value}
}

// Inset shrinks the rectangle by the insets on each edge.
func (r Rect) Inset(insets Insets) Rect {
	r.X += insets.Left
	r.Y += insets.Top
	r.Width -= insets.Left + insets.Right
	r.Height -= insets.Top + insets.Bottom
	if r.Width < 0 {
		r.Width = 0
	}
	if r.Height < 0 {
		r.Height = 0
	}
	return r
}

// Right returns the x coordinate of the rectangle's right edge.
func (r Rect) Right() float32 { return r.X + r.Width }

// Bottom returns the y coordinate of the rectangle's bottom edge.
func (r Rect) Bottom() float32 { return r.Y + r.Height }

// Offset moves the rectangle by dx and dy without changing its size.
func (r Rect) Offset(dx, dy float32) Rect {
	r.X += dx
	r.Y += dy
	return r
}

// Centered returns a rectangle of the given size centered in r. A size larger
// than r overflows evenly on both sides.
func (r Rect) Centered(width, height float32) Rect {
	return Rect{X: r.X + (r.Width-width)/2, Y: r.Y + (r.Height-height)/2, Width: width, Height: height}
}

// CutTop splits r into a strip of the given height along its top edge and the
// rest below it. A height past r's own is clamped, so the strip never leaves r
// and the rest is empty rather than negative.
//
//	header, body := bounds.CutTop(48)
//	body = body.Inset(gfx.All(16))
func (r Rect) CutTop(height float32) (strip, rest Rect) {
	height = min(max(height, 0), r.Height)
	return Rect{X: r.X, Y: r.Y, Width: r.Width, Height: height},
		Rect{X: r.X, Y: r.Y + height, Width: r.Width, Height: r.Height - height}
}

// CutBottom splits r into a strip of the given height along its bottom edge and
// the rest above it.
func (r Rect) CutBottom(height float32) (strip, rest Rect) {
	height = min(max(height, 0), r.Height)
	return Rect{X: r.X, Y: r.Y + r.Height - height, Width: r.Width, Height: height},
		Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height - height}
}

// CutLeft splits r into a strip of the given width along its left edge and the
// rest to its right.
func (r Rect) CutLeft(width float32) (strip, rest Rect) {
	width = min(max(width, 0), r.Width)
	return Rect{X: r.X, Y: r.Y, Width: width, Height: r.Height},
		Rect{X: r.X + width, Y: r.Y, Width: r.Width - width, Height: r.Height}
}

// CutRight splits r into a strip of the given width along its right edge and
// the rest to its left.
func (r Rect) CutRight(width float32) (strip, rest Rect) {
	width = min(max(width, 0), r.Width)
	return Rect{X: r.X + r.Width - width, Y: r.Y, Width: width, Height: r.Height},
		Rect{X: r.X, Y: r.Y, Width: r.Width - width, Height: r.Height}
}

// Row lays out slots left to right with a fixed gap. It is a value type:
// slot bookkeeping stays on the stack and layout allocates nothing.
type Row struct {
	bounds Rect
	gap    float32
	cursor float32
}

// BeginRow starts a horizontal stack inside bounds.
func BeginRow(bounds Rect, gap float32) Row {
	if gap < 0 {
		gap = 0
	}
	return Row{bounds: bounds, gap: gap}
}

// Next returns the next slot with the requested width. Slots are full height.
func (r *Row) Next(width float32) Rect {
	if width < 0 {
		width = 0
	}
	slot := Rect{X: r.bounds.X + r.cursor, Y: r.bounds.Y, Width: width, Height: r.bounds.Height}
	r.cursor += width + r.gap
	return slot
}

// NextFlex returns one slot filling all remaining width.
func (r *Row) NextFlex() Rect {
	width := r.bounds.Width - r.cursor
	if width < 0 {
		width = 0
	}
	slot := Rect{X: r.bounds.X + r.cursor, Y: r.bounds.Y, Width: width, Height: r.bounds.Height}
	r.cursor = r.bounds.Width
	return slot
}

// Even returns n equal-width slots (divided over the remaining width, gaps
// between them) in one call.
func (r *Row) Even(n int) []Rect {
	if n <= 0 {
		return nil
	}
	total := r.bounds.Width - r.cursor - r.gap*float32(n-1)
	if total < 0 {
		total = 0
	}
	width := total / float32(n)
	slots := make([]Rect, n)
	for index := range slots {
		slots[index] = r.Next(width)
	}
	return slots
}

// Remaining reports the unclaimed width, ignoring a trailing gap.
func (r Row) Remaining() float32 {
	remaining := r.bounds.Width - r.cursor
	if r.cursor > 0 && remaining < 0 {
		// The trailing gap of the last slot does not consume layout space.
		remaining += r.gap
		if remaining < 0 {
			remaining = 0
		}
	}
	return remaining
}

// Column lays out slots top to bottom with a fixed gap. It mirrors Row.
type Column struct {
	bounds Rect
	gap    float32
	cursor float32
}

// BeginColumn starts a vertical stack inside bounds.
func BeginColumn(bounds Rect, gap float32) Column {
	if gap < 0 {
		gap = 0
	}
	return Column{bounds: bounds, gap: gap}
}

// Next returns the next slot with the requested height. Slots are full width.
func (c *Column) Next(height float32) Rect {
	if height < 0 {
		height = 0
	}
	slot := Rect{X: c.bounds.X, Y: c.bounds.Y + c.cursor, Width: c.bounds.Width, Height: height}
	c.cursor += height + c.gap
	return slot
}

// NextFlex returns one slot filling all remaining height.
func (c *Column) NextFlex() Rect {
	height := c.bounds.Height - c.cursor
	if height < 0 {
		height = 0
	}
	slot := Rect{X: c.bounds.X, Y: c.bounds.Y + c.cursor, Width: c.bounds.Width, Height: height}
	c.cursor = c.bounds.Height
	return slot
}

// Even returns n equal-height slots in one call.
func (c *Column) Even(n int) []Rect {
	if n <= 0 {
		return nil
	}
	total := c.bounds.Height - c.cursor - c.gap*float32(n-1)
	if total < 0 {
		total = 0
	}
	height := total / float32(n)
	slots := make([]Rect, n)
	for index := range slots {
		slots[index] = c.Next(height)
	}
	return slots
}

// Remaining reports the unclaimed height, ignoring a trailing gap.
func (c Column) Remaining() float32 {
	remaining := c.bounds.Height - c.cursor
	if c.cursor > 0 && remaining < 0 {
		remaining += c.gap
		if remaining < 0 {
			remaining = 0
		}
	}
	return remaining
}

// GridSpec describes a wrapping grid of equal cells. MinCellWidth is the
// narrowest a cell may become, not its size: the column count is the most
// cells of at least that width that fit, and the cells then stretch to fill
// the container exactly, so the grid follows its container instead of
// sitting at a fixed size in one corner of it.
type GridSpec struct {
	MinCellWidth float32
	CellHeight   float32
	Gap          float32
}

// Columns returns the column count for a container width. It is never below
// one: a container narrower than MinCellWidth gets a single, narrower column
// rather than overflowing.
func (spec GridSpec) Columns(width float32) int {
	gap := max(spec.Gap, 0)
	unit := max(spec.MinCellWidth, 1) + gap
	if width <= 0 {
		return 1
	}
	return max(int((width+gap)/unit), 1)
}

// Rows returns the row count needed for count cells at a container width.
func (spec GridSpec) Rows(width float32, count int) int {
	if count <= 0 {
		return 0
	}
	columns := spec.Columns(width)
	return (count + columns - 1) / columns
}

// Height returns the height of count cells at a container width, without a
// trailing gap.
func (spec GridSpec) Height(width float32, count int) float32 {
	rows := spec.Rows(width, count)
	if rows == 0 {
		return 0
	}
	return float32(rows)*spec.CellHeight + float32(rows-1)*max(spec.Gap, 0)
}

// Grid hands out the cells of a GridSpec left to right, wrapping at the
// container edge. It is a value type: layout allocates nothing.
type Grid struct {
	bounds    Rect
	spec      GridSpec
	columns   int
	cellWidth float32
	next      int
}

// BeginGrid starts a grid inside bounds.
func BeginGrid(bounds Rect, spec GridSpec) Grid {
	spec.Gap = max(spec.Gap, 0)
	columns := spec.Columns(bounds.Width)
	cellWidth := max((bounds.Width-spec.Gap*float32(columns-1))/float32(columns), 0)
	return Grid{bounds: bounds, spec: spec, columns: columns, cellWidth: cellWidth}
}

// Columns reports how many cells share one row.
func (g Grid) Columns() int { return g.columns }

// Next returns the next cell. Cells keep coming past the container height;
// the caller owns scrolling and clipping.
func (g *Grid) Next() Rect {
	row, column := g.next/g.columns, g.next%g.columns
	g.next++
	return Rect{
		X:      g.bounds.X + float32(column)*(g.cellWidth+g.spec.Gap),
		Y:      g.bounds.Y + float32(row)*(g.spec.CellHeight+g.spec.Gap),
		Width:  g.cellWidth,
		Height: g.spec.CellHeight,
	}
}
