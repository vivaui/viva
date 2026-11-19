package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

// paintRangeSlider installs the painter that owns the two thumb declarations,
// then paints with it. The state keeps the painter, so the slice the component
// borrows is allocated once rather than on every frame.
func paintRangeSlider(d *base.DrawContext, s *base.RangeSliderState) {
	paint := rangeSliderPainter(s)
	s.Paint = paint
	paint(d)
}

// rangeSliderPainter returns a painter whose scratch holds the two thumb
// declarations. The caller's thumb labels and value texts reach the component,
// so a translated application announces the same text as the canvas-level
// preview.
func rangeSliderPainter(s *base.RangeSliderState) func(*base.DrawContext) {
	thumbs := new([2]components.SliderThumbValue)
	return func(d *base.DrawContext) {
		parts := s.Handlers.Parts()
		lowID, highID := parts.At(0), parts.At(1)
		s.SetTrack(d.Rect, lowID, highID)
		thumbs[0] = components.SliderThumbValue{ID: lowID, Label: s.ThumbLabel(0), ValueText: s.ThumbValueText(0), Value: s.Low}
		thumbs[1] = components.SliderThumbValue{ID: highID, Label: s.ThumbLabel(1), ValueText: s.ThumbValueText(1), Value: s.High}
		components.BeginRangeSlider(d.Canvas, *d.Theme, d.Rect, thumbs[:]).
			Range(s.Min, s.Max).Step(s.Step).Paint("Range")
	}
}
