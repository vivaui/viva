package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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
//
// The caller may name the low and high thumbs, the track and the filled value
// range with RangeSliderState.LowID, HighID, TrackID and ValueID. A part left
// zero takes its identity from the control's own scope, as it did before the
// caller could name it, and every named part is registered as an alias of the
// control so its keys and presses reach the same handlers.
func rangeSliderPainter(s *base.RangeSliderState) func(*base.DrawContext) {
	thumbs := new([2]components.SliderThumbValue)
	return func(d *base.DrawContext) {
		parts := s.Handlers.Parts()
		lowID, highID := s.LowID, s.HighID
		if lowID.IsZero() {
			lowID = parts.At(0)
		}
		if highID.IsZero() {
			highID = parts.At(1)
		}
		trackID, valueID := s.TrackID, s.ValueID
		if trackID.IsZero() {
			trackID = parts.At(2)
		}
		if valueID.IsZero() {
			valueID = parts.At(3)
		}
		base.AddControlAliases(d.Frame, &s.Handlers, lowID, highID, trackID, valueID)
		// A thumb's hit reaches six pixels around its 16 pixel circle. The
		// control's own hit covers its whole rectangle, so the travel is inset by
		// that reach: every thumb stays inside the control at the ends of the
		// range, and no thumb hit reaches outside the control's group hit.
		track := d.Rect
		if track.Width > 12 {
			track.X += 6
			track.Width -= 12
		}
		s.SetTrack(track, lowID, highID, trackID, valueID)
		thumbs[0] = components.SliderThumbValue{ID: lowID, Label: s.ThumbLabel(0), ValueText: s.ThumbValueText(0), Value: s.Low}
		thumbs[1] = components.SliderThumbValue{ID: highID, Label: s.ThumbLabel(1), ValueText: s.ThumbValueText(1), Value: s.High}
		slider := components.BeginRangeSlider(d.Canvas, *d.Theme, track, thumbs[:]).
			Range(s.Min, s.Max).Step(s.Step)
		// Only parts the caller named get a region of their own: a zero identity
		// keeps the thumbs' behavior the page had before it could name a track or
		// a value, without a second group over the control.
		rangeSliderPartHits(d, slider, s.TrackID, s.ValueID)
		slider.Paint("Range")
	}
}

// rangeSliderPartHits declares the track and value regions the caller named,
// before the component declares its thumbs: the slider's whole rectangle is the
// track's region, and the filled range spans the thumbs' hit regions. Both
// contain the thumbs' own hits, so a press still lands on the thumb under it,
// and both are groups rather than Tab stops.
func rangeSliderPartHits(d *base.DrawContext, slider components.Slider, trackID, valueID gfx.ID) {
	options := gfx.HitOptions{Role: gfx.RoleGroup, Disabled: d.State.Disabled, SkipFocus: true}
	if !trackID.IsZero() {
		d.Canvas.HitWithOptions(trackID, d.Rect, "Range track", options)
	}
	if valueID.IsZero() {
		return
	}
	low := thumbHitBounds(slider.ThumbBounds(0))
	high := thumbHitBounds(slider.ThumbBounds(1))
	if high.X < low.X {
		low, high = high, low
	}
	width := high.X + high.Width - low.X
	if width <= 0 || low.Height <= 0 {
		return
	}
	d.Canvas.HitWithOptions(valueID, base.Rect{X: low.X, Y: low.Y, Width: width, Height: low.Height}, "Range value", options)
}

// thumbHitBounds is the reach the component gives a thumb: its circle and six
// pixels around it.
func thumbHitBounds(thumb base.Rect) base.Rect {
	return base.Rect{X: thumb.X - 6, Y: thumb.Y - 6, Width: thumb.Width + 12, Height: thumb.Height + 12}
}
