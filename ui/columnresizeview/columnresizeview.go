// Package columnresizeview is the opt-in page control for a row of columns
// whose dividers the user drags. It wraps components.BeginColumnResize in a
// ui.Control: the caller keeps the widths and paints each cell, the wrapper
// paints the dividers, declares their hit regions and maps pointer coordinates
// to the clamped width edit.
//
//	cols := &columnresizeview.Columns{
//		Widths: []float32{260, 90, 140}, Minimums: []float32{120, 54, 76},
//		Fits: []float32{180, 64, 100}, Gap: 8,
//		DividerIDs: []gfx.ID{first, second},
//		Cell: func(d *ui.DrawContext, index int, bounds gfx.Rect) { ... },
//	}
//	columnresizeview.View(c, "Files", cols).FillWidth().Height(220)
package columnresizeview

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Columns is one column row's caller-owned widths, geometry and page control.
// Fill Widths and Fits from the model every paint; keep Widths at one entry per
// column.
type Columns struct {
	// Widths are the current column widths. The wrapper does not write them;
	// Edit reports the pair to write.
	Widths []float32
	// Minimums and Fits are the least width and content width of each column.
	Minimums, Fits []float32
	// Gap is the room between two columns; dividers sit in its middle.
	Gap float32
	// DividerIDs name the dividers for input; zero IDs paint no divider hits.
	DividerIDs []gfx.ID
	// DividerLabels name the dividers for assistive technology.
	DividerLabels []string
	// Cell paints column index into bounds. The wrapper paints all cells after
	// the dividers, so a cell may cover its own column.
	Cell func(*ui.DrawContext, int, gfx.Rect)
	// Pointer receives the pointer events aimed at the control, with the
	// caller's divider IDs as Target.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives keys while the control has focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the control.
	Scroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	bounds   gfx.Rect
	columns  []gfx.Rect
	dividers []gfx.Rect
	hits     []gfx.Rect
}

// View declares the column row at c. Give it a size.
func View(c ui.C, label string, columns *Columns) ui.Box {
	if columns == nil {
		panic("columnresizeview: View needs its Columns")
	}
	columns.prepare()
	return ui.Control(c, label, &columns.handlers).Draw(columns.drawFn)
}

func (c *Columns) prepare() {
	if c.prepared {
		return
	}
	c.prepared = true
	c.handlers.Pointer = c.pointer
	c.handlers.Key = c.key
	c.handlers.Scroll = c.scroll
	c.drawFn = c.draw
}

// Edit maps pointer x to the new widths of the columns either side of divider.
// Write the pair back to Widths and repaint. The first return is the left
// width, the second the right.
func (c *Columns) Edit(divider int, x float32) (left, right float32, ok bool) {
	edit, ok := c.component(nil).Edit(divider, x)
	return edit.Left, edit.Right, ok
}

// Fit is Edit's double-click behavior: the left column takes its content width.
func (c *Columns) Fit(divider int) (left, right float32, ok bool) {
	edit, ok := c.component(nil).Fit(divider)
	return edit.Left, edit.Right, ok
}

// Columns returns the rectangles of the last paint, for a caller that needs
// the cell geometry outside Cell.
func (c *Columns) Columns() []gfx.Rect { return c.columns }

func (c *Columns) component(canvas *gfx.Canvas) components.ColumnResize {
	return components.BeginColumnResize(canvas, gfx.Theme{}, c.bounds, c.Widths).
		Dividers(c.DividerIDs).Minimums(c.Minimums).Fits(c.Fits).Gap(c.Gap)
}

func (c *Columns) draw(d *ui.DrawContext) {
	c.bounds = d.Rect
	layout := c.component(d.Canvas).Paint(c.columns, c.dividers, c.hits, c.DividerLabels)
	c.columns, c.dividers, c.hits = layout.Columns, layout.Dividers, layout.Hits
	if c.Cell == nil {
		return
	}
	for index, bounds := range c.columns {
		c.Cell(d, index, bounds)
	}
}

func (c *Columns) pointer(event gfx.PointerEvent) gfx.EventResult {
	if c.Pointer == nil {
		return gfx.EventResult{}
	}
	return c.Pointer(event)
}

func (c *Columns) key(event gfx.KeyEvent) gfx.EventResult {
	if c.Key == nil {
		return gfx.EventResult{}
	}
	return c.Key(event)
}

func (c *Columns) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if c.Scroll == nil {
		return gfx.EventResult{}
	}
	return c.Scroll(event)
}
