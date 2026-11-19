package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

// paintRangeSlider paints a page range slider and records the thumb rectangles
// its pointer handlers use.
func paintRangeSlider(d *base.DrawContext, s *base.RangeSliderState) {
	parts := s.Handlers.Parts()
	lowID, highID := parts.At(0), parts.At(1)
	s.SetTrack(d.Rect, lowID, highID)
	thumbs := []components.SliderThumbValue{
		{ID: lowID, Label: "Minimum", Value: s.Low},
		{ID: highID, Label: "Maximum", Value: s.High},
	}
	components.BeginRangeSlider(d.Canvas, *d.Theme, d.Rect, thumbs).
		Range(s.Min, s.Max).Step(s.Step).Paint("Range")
}
