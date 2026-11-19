package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// AccordionItem is one disclosure row of an Accordion. Content is declared only
// while the row is open.
type AccordionItem struct {
	Title   string
	Content func()
}

// AccordionState is the caller-owned expansion of an Accordion. Open has one
// entry per item and keeps its length for as long as the state is used; indexing
// it by item is what keeps each header's identity stable between paints.
//
// Multiple allows more than one row open at once. By default opening a row closes
// the others. Disabled makes every header inert.
type AccordionState struct {
	Open     []bool
	Multiple bool
	Disabled bool
}

// NewAccordionState makes state for n items, all closed.
func NewAccordionState(n int) *AccordionState {
	if n < 0 {
		n = 0
	}
	return &AccordionState{Open: make([]bool, n)}
}

// IsOpen reports whether item i is expanded. An index outside Open is closed.
func (s *AccordionState) IsOpen(i int) bool {
	return s != nil && i >= 0 && i < len(s.Open) && s.Open[i]
}

// Accordion declares a vertical set of disclosure rows with the theme's look.
// The caller owns which rows are open; a click on a header flips its entry.
//
//	ui.Accordion(c, m.help, []ui.AccordionItem{{Title: "Shipping", Content: body}})
func Accordion(c C, state *AccordionState, items []AccordionItem) Box {
	if state == nil {
		panic("ui: Accordion needs a state")
	}
	if len(state.Open) != len(items) {
		panic("ui: AccordionState.Open needs one entry per item and must keep its length")
	}
	root := base.Column(c).FillWidth()
	root.Children(func() {
		for i := range items {
			if i > 0 {
				base.Divider(c)
			}
			index := i
			base.Column(c).FillWidth().Children(func() {
				header := disclosureHeader(c, &state.Open[index], items[index].Title, state.Disabled)
				if header.Clicked() && !state.Multiple {
					for j := range state.Open {
						if j != index {
							state.Open[j] = false
						}
					}
				}
				content := base.Column(c)
				if state.Open[index] && items[index].Content != nil {
					content.PadBottom(16).Children(items[index].Content)
				}
			})
		}
	})
	return root
}

// CollapsibleState is the caller-owned state of one Collapsible. Disabled keeps
// the trigger painted and inert.
type CollapsibleState struct {
	Open     bool
	Disabled bool
}

// Collapsible declares one disclosure row with the theme's look: a click on the
// header flips state.Open, and content is declared only while state.Open is true.
func Collapsible(c C, state *CollapsibleState, title string, content func()) Box {
	if state == nil {
		panic("ui: Collapsible needs a state")
	}
	root := base.Column(c).FillWidth()
	root.Children(func() {
		disclosureHeader(c, &state.Open, title, state.Disabled)
		body := base.Column(c)
		if state.Open && content != nil {
			body.PadBottom(16).Children(content)
		}
	})
	return root
}

// disclosureHeader is the styled trigger shared by Accordion and Collapsible.
func disclosureHeader(c C, open *bool, title string, disabled bool) Box {
	theme := *c.Theme()
	foreground := theme.Foreground
	if disabled {
		foreground.Alpha *= 0.50
	}
	header := base.DisclosureBase(c, open, title).
		FillWidth().
		Height(components.AccordionHeaderExtent(theme)).
		PadX(8).
		Radius(theme.Metrics.RadiusControl).
		Disabled(disabled)
	if !disabled {
		header = header.
			Hover(Look{}.Background(theme.Hover)).
			Focus(Look{}.Ring(3, fadedRing(theme)))
	}
	header.Children(func() {
		base.Text(c, title).
			FontSize(theme.Metrics.FontSizeM).
			Weight(gfx.WeightMedium).
			Color(foreground)
		base.Spacer(c)
		base.Slot(c).Size(16, 16)
	})
	header.Draw(paintDisclosureChevron)
	return header
}

func fadedRing(theme base.Theme) gfx.Color {
	ring := theme.Ring
	ring.Alpha *= 0.50
	return ring
}

func paintDisclosureChevron(d *DrawContext) {
	icon := disclosureChevronDown
	if d.State.Open {
		icon = disclosureChevronUp
	}
	color := d.Theme.MutedForeground
	if d.State.Disabled {
		color.Alpha *= 0.50
	}
	const size = 16
	bounds := gfx.Rect{
		X:      d.Rect.X + d.Rect.Width - size - 8,
		Y:      d.Rect.Y + (d.Rect.Height-size)/2,
		Width:  size,
		Height: size,
	}
	d.Canvas.DrawIcon(icon, bounds, d.Fade(color))
}

var (
	disclosureChevronDown = gfx.Icon{
		ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(6, 9), gfx.LineTo(12, 15), gfx.LineTo(18, 9)},
	}
	disclosureChevronUp = gfx.Icon{
		ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(6, 15), gfx.LineTo(12, 9), gfx.LineTo(18, 15)},
	}
)
