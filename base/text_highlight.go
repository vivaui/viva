package base

import "github.com/vivaui/viva/gfx"

type textHighlight struct {
	query string
	color Color
}

// Highlight paints case-insensitive, non-overlapping query matches behind Text
// or Paragraph, preserving the label's shaping, wrapping and text direction.
// Matches are found within each displayed line; a query spanning a wrapped
// line break is not highlighted. Calling it again replaces the earlier query;
// an empty query or transparent color removes the highlight.
func (b Box) Highlight(query string, color Color) Box {
	n := b.N()
	if query == "" || color.Alpha <= 0 {
		if n.Ext != 0 {
			b.f.Exts[n.Ext].highlight = 0
		}
		return b
	}
	extra := b.f.Extra(b.i)
	if extra.highlight == 0 {
		b.f.highlights = append(b.f.highlights, textHighlight{})
		extra.highlight = int32(len(b.f.highlights))
	}
	b.f.highlights[extra.highlight-1] = textHighlight{query: query, color: color}
	// Keep matching and selection geometry out of applications that never opt in.
	b.f.paintHighlightFn = paintHighlightedLabel
	return b
}

func paintHighlightedLabel(f *Frame, n *Node, value string, bounds Rect, style gfx.TextStyle, alignment gfx.Alignment) {
	highlight := f.highlights[f.Exts[n.Ext].highlight-1]
	f.Surface.LabelHighlighted(value, bounds, style, alignment, highlight.query, fade(highlight.color, f.Opacity))
}
