// Package typography is the opt-in page control for a piece of text that can be
// cut to a few lines with a way to read the rest, and copied: a description, a
// fingerprint, a command. A page keeps a Text in its model, because it remembers
// whether the user expanded it:
//
//	m.about = typography.New()
//	// in the page:
//	m.about.View(c, m.description, typography.Spec{Rows: 3, Expandable: true, Copyable: true}).FillWidth()
//	if m.about.Copied() {
//		// the text went to the clipboard; say so, for instance with a toast
//	}
//
// Rows is how many lines the text shows before it ends in an ellipsis; zero shows
// all of it. When the text needs more lines than Rows at the width it has, Expandable
// adds a button after it that shows the rest and takes it away again. Copyable adds a
// button that puts the whole text on the system clipboard; Copied reports a press on
// it. Whether the text is cut is known from the width it had in the paint before, so
// the button appears one paint after the text first does.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Text in the model: it holds the expanded state and the
// width the last paint measured, so a warm paint allocates nothing. Do not copy a
// Text after it has been declared.
package typography

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var copyIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(9, 9), gfx.LineTo(20, 9), gfx.LineTo(20, 20), gfx.LineTo(9, 20), gfx.ClosePath(),
	gfx.MoveTo(5, 15), gfx.LineTo(4, 15), gfx.LineTo(4, 4), gfx.LineTo(15, 4), gfx.LineTo(15, 5),
}}

// Labels are the words of the buttons.
type Labels struct {
	// More and Less are the expand button's two states; Copy names the copy button.
	More, Less, Copy string
}

// English is the default words.
var English = Labels{More: "More", Less: "Less", Copy: "Copy"}

// Spec declares the text.
type Spec struct {
	// Rows is how many lines show before the ellipsis; zero shows all. Up to seven are
	// told apart.
	Rows int
	// Expandable adds the button that shows the rest of a text cut to Rows.
	Expandable bool
	// Copyable adds the copy button.
	Copyable bool
	// Muted paints the text in the muted color.
	Muted bool
	// Labels are the words; English when zero.
	Labels Labels
}

// Text is one text and whether it is expanded.
type Text struct {
	expanded bool
	copied   bool
	width    float32 // the width the paragraph had in the last paint
	cut      bool    // whether the text needs more than Rows lines at that width
	measure  func(*ui.DrawContext)
}

// New returns a collapsed text.
func New() *Text {
	t := &Text{}
	t.measure = t.noteWidth
	return t
}

// noteWidth keeps the width the paragraph was laid out at. A change asks for one more
// paint, which decides again whether the text is cut; the width only changes when the
// window or the layout does, so an idle text asks for nothing.
func (t *Text) noteWidth(d *ui.DrawContext) {
	if d.Rect.Width != t.width {
		t.width = d.Rect.Width
		d.Canvas.RequestFrame()
	}
}

// Copied reports whether the copy button was pressed in the paint just made.
func (t *Text) Copied() bool { return t.copied }

// Expanded reports whether the text shows all of itself.
func (t *Text) Expanded() bool { return t.expanded }

// View declares the text at c. The text is as tall as its lines; give it FillWidth.
// Declare the view on its own, not inside a Memo.
func (t *Text) View(c ui.C, value string, spec Spec) ui.Box {
	if t == nil {
		panic("typography: View needs its Text")
	}
	t.copied = false
	labels := spec.Labels
	if labels == (Labels{}) {
		labels = English
	}
	rows := min(max(spec.Rows, 0), 7)
	theme := c.Theme()
	metrics := theme.Metrics
	// Whether the text is cut, from the width the paragraph had last paint.
	t.cut = false
	if rows > 0 && t.width > 0 {
		var scratch [8]string
		lines := c.Canvas().WrapLines(scratch[:0], value, metrics.FontSizeM, gfx.WeightRegular, t.width, rows+1)
		t.cut = len(lines) > rows
	}
	maxLines := rows
	if t.expanded {
		maxLines = 0
	}
	root := ui.Column(c).Gap(metrics.GapItem * 0.5)
	root.Children(func() {
		var paragraph ui.Box
		if spec.Copyable {
			ui.Row(c).FillWidth().Items(ui.Start).Gap(metrics.GapItem).Children(func() {
				paragraph = ui.Paragraph(c, value, maxLines).Grow(1).Basis(0).Draw(t.measure)
				if spec.Muted {
					paragraph.Muted()
				}
				if ui.Button(c, labels.Copy).Icon(copyIcon).IconOnly().Ghost().Shrink(0).Clicked() {
					_ = ui.SetClipboardText(value)
					t.copied = true
				}
			})
		} else {
			paragraph = ui.Paragraph(c, value, maxLines).Draw(t.measure)
			if spec.Muted {
				paragraph.Muted()
			}
		}
		if spec.Expandable && (t.cut || t.expanded) {
			label := labels.More
			if t.expanded {
				label = labels.Less
			}
			if ui.Button(c, label).Link().Small().Clicked() {
				t.expanded = !t.expanded
			}
		}
	})
	return root
}
