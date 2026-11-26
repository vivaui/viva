// Package descriptions is the opt-in page control for a list of labelled
// values: the details of a record, laid out in columns. A page keeps the items
// in its model and declares the list with a Spec:
//
//	descriptions.View(c, "Host details", descriptions.Spec{
//		Title:    "Host details",
//		Items:    m.details, // a []descriptions.Item kept in the model
//		Columns:  2,
//		Bordered: true,
//	}).FillWidth()
//
// An item has a Label and a Value, and a Span of the columns it takes, one by
// default. The items fill the lines in order; the last item of a line takes
// the columns the line has left, so a line is always full. A Value is text, or
// Content declares the page's own elements, such as a badge.
//
// By default a label is a muted word before its value. Bordered draws the list
// as a table, Vertical puts each label above its value, and Size: Small is the
// compact list. LabelWidth gives every label the same width, so the values line
// up down the list. It composes the page's own primitives, so it takes the
// page's theme and density.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package descriptions

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Size is how large the text and the padding are.
type Size uint8

const (
	// Default is the zero value.
	Default Size = iota
	// Small is the compact list.
	Small
)

// Item is one labelled value.
type Item struct {
	// Label names the value.
	Label string
	// Value is the value, as text.
	Value string
	// Content declares the value with the page's own elements instead of Value.
	// Its elements are the caller's: they take their own identities and
	// handlers. It is not called when nil.
	Content func(ui.C)
	// Span is how many columns the item takes; one when zero or less. A span
	// wider than the list is the whole line.
	Span int
}

// Spec declares the list. Its zero value is a three-column, borderless list.
type Spec struct {
	// Title heads the list. It is not drawn when empty.
	Title string
	// Items are the labelled values, in order. Keep the slice in the model: a
	// slice made in each paint allocates in each paint.
	Items []Item
	// Columns is how many columns a line has; three when zero or less.
	Columns int
	// Bordered draws the list as a table: lines around every label and value, and
	// a tinted label.
	Bordered bool
	// Vertical puts each label above its value, instead of before it.
	Vertical bool
	// Size is how large the text and the padding are.
	Size Size
	// LabelWidth is the width of every label, so the values line up; zero lets a
	// label be as wide as its words. A Vertical list ignores it.
	LabelWidth float32
}

func (s Spec) columns() int {
	if s.Columns <= 0 {
		return 3
	}
	return s.Columns
}

// span is how many columns item i takes, never more than the line has.
func (s Spec) span(i int) int {
	span := s.Items[i].Span
	if span <= 0 {
		span = 1
	}
	return min(span, s.columns())
}

// lineEnd is the index after the last item of the line that starts at start:
// the items that fit the columns in order.
func (s Spec) lineEnd(start int) int {
	used, end := 0, start
	for end < len(s.Items) {
		span := s.span(end)
		if used+span > s.columns() {
			break
		}
		used += span
		end++
	}
	return max(end, start+1)
}

// View declares the list at c. label names it for assistive technology. The list
// is as tall as its lines; give it FillWidth. Declare the view on its own, not
// inside a Memo.
func View(c ui.C, label string, spec Spec) ui.Box {
	if c == nil {
		panic("descriptions: View needs a page")
	}
	theme := c.Theme()
	metrics := theme.Metrics
	look := look{theme: theme, pad: metrics.GapItem, size: metrics.FontSizeM}
	if spec.Size == Small {
		look.pad, look.size = metrics.GapItem*0.5, metrics.FontSizeS
	}
	root := ui.Column(c).Gap(look.pad)
	root = root.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	root.Children(func() {
		if spec.Title != "" {
			ui.Text(c, spec.Title).FontSize(look.size + 2).Weight(gfx.WeightSemibold)
		}
		body := ui.Column(c).FillWidth().Gap(look.pad)
		if spec.Bordered {
			body = ui.Column(c).FillWidth().Border(1, theme.Border).Radius(metrics.RadiusControl).Clip()
		}
		body.Children(func() {
			for start := 0; start < len(spec.Items); {
				end := spec.lineEnd(start)
				if spec.Bordered && start > 0 {
					ui.Spacer(c).FillWidth().Height(1).Background(theme.Border)
				}
				// Cells are not spaced with the line's gap: a line of two cells would
				// share its width differently from a line of three, and the columns
				// would not line up. A cell keeps its own room inside itself.
				line := ui.Row(c).FillWidth().Items(ui.Stretch)
				line.Children(func() {
					used := 0
					for i := start; i < end; i++ {
						span := spec.span(i)
						if i == end-1 {
							span = max(span, spec.columns()-used)
						}
						used += span
						if spec.Bordered {
							look.tableCell(c, spec, spec.Items[i], span, i > start)
						} else {
							look.plainCell(c, spec, spec.Items[i], span)
						}
					}
				})
				start = end
			}
		})
	})
	return root
}

// look is what a list paints with: the page's theme and the spec's size.
type look struct {
	theme *gfx.Theme
	pad   float32
	size  float32
}

// plainCell declares one item of a borderless line: a muted label and its value.
func (l look) plainCell(c ui.C, spec Spec, item Item, span int) {
	var cell ui.Box
	if spec.Vertical {
		cell = ui.Column(c).Gap(l.pad * 0.25)
	} else {
		cell = ui.Row(c).Gap(l.pad).Items(ui.Start)
	}
	cell.Basis(0).Grow(float32(span))
	cell.Children(func() {
		label := ui.Text(c, item.Label).FontSize(l.size).Muted().Shrink(0)
		if spec.LabelWidth > 0 && !spec.Vertical {
			label.Width(spec.LabelWidth)
		}
		// The room to the next cell is the value's, so the cell itself is exactly
		// its share of the line.
		value := ui.Row(c).Items(ui.Center).PadSides(0, l.pad*2, 0, 0)
		if spec.Vertical {
			value.FillWidth()
		} else {
			value.Grow(1).Basis(0)
		}
		value.Children(func() { l.value(c, item) })
	})
}

// tableCell declares one item of a bordered line: a tinted label cell and a
// value cell, side by side or, for a vertical list, one above the other. The line
// that parts it from the cell before it is inside it, so the cell is exactly its
// share of the line and the columns of two lines line up.
func (l look) tableCell(c ui.C, spec Spec, item Item, span int, parted bool) {
	cell := ui.Row(c).Basis(0).Grow(float32(span)).Items(ui.Stretch)
	cell.Children(func() {
		if parted {
			ui.Spacer(c).Width(1).Grow(0).Shrink(0).Background(l.theme.Border)
		}
		var inner ui.Box
		if spec.Vertical {
			inner = ui.Column(c)
		} else {
			inner = ui.Row(c).Items(ui.Stretch)
		}
		inner.Grow(1).Basis(0)
		inner.Children(func() {
			label := ui.Row(c).Pad(l.pad).Background(l.theme.Muted).Items(ui.Center)
			if spec.Vertical {
				label.FillWidth()
			} else {
				label.Shrink(0)
				if spec.LabelWidth > 0 {
					label.Width(spec.LabelWidth)
				}
			}
			label.Children(func() {
				ui.Text(c, item.Label).FontSize(l.size).Weight(gfx.WeightMedium)
			})
			value := ui.Row(c).Grow(1).Basis(0).Pad(l.pad).Items(ui.Center)
			value.Children(func() { l.value(c, item) })
		})
	})
}

// value declares the value of an item.
func (l look) value(c ui.C, item Item) {
	if item.Content != nil {
		item.Content(c)
		return
	}
	ui.Text(c, item.Value).FontSize(l.size)
}
