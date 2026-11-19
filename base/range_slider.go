package base

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
)

// RangeSliderState is a page range slider: two values on one track. The
// application owns Low and High; Min, Max and Step are the range and the snapping
// step (0..100 and 1 when zero). Handlers and Paint are the page control's bridge
// and are set by package ui.
type RangeSliderState struct {
	Low, High      float32
	Min, Max, Step float32

	// LowLabel and HighLabel name the thumbs for assistive technology. An empty
	// name takes "Minimum" or "Maximum"; a translated application supplies its
	// own.
	LowLabel, HighLabel string
	// LowValueText and HighValueText are the thumbs' values as assistive
	// technology announces them. An empty text is Low or High formatted as a
	// decimal number; a caller that formats them itself keeps them in step with
	// the values.
	LowValueText, HighValueText string

	Handlers Handlers
	// Paint paints the slider for the Control Draw; ui sets it.
	Paint func(*DrawContext)

	lowID, highID gfx.ID
	rect          Rect
	dragging      int // 1 low, 2 high, 0 none
	draw          func(*DrawContext)
	// valueTexts caches the default decimal text of each thumb, so a steady
	// paint formats no value again.
	valueTexts  [2]string
	valueTextOf [2]float32
}

// NewRangeSliderState returns a range slider holding low and high.
func NewRangeSliderState(low, high float32) *RangeSliderState {
	return &RangeSliderState{Low: low, High: high, Min: 0, Max: 100, Step: 1}
}

// SetTrack records the rectangle and thumb identities of the last paint, for
// the pointer handlers.
func (s *RangeSliderState) SetTrack(rect Rect, lowID, highID gfx.ID) {
	s.rect, s.lowID, s.highID = rect, lowID, highID
}

// ThumbLabel returns the accessible name of thumb 0 (Low) or thumb 1 (High):
// LowLabel or HighLabel when set, otherwise "Minimum" or "Maximum".
func (s *RangeSliderState) ThumbLabel(index int) string {
	switch index {
	case 0:
		if s.LowLabel != "" {
			return s.LowLabel
		}
		return "Minimum"
	case 1:
		if s.HighLabel != "" {
			return s.HighLabel
		}
		return "Maximum"
	}
	return ""
}

// ThumbValueText returns the accessible value text of thumb 0 (Low) or thumb 1
// (High): LowValueText or HighValueText when set, otherwise the value formatted
// as a decimal number. The default text is reused until the value changes, so a
// steady paint allocates nothing.
func (s *RangeSliderState) ThumbValueText(index int) string {
	var text string
	switch index {
	case 0:
		text = s.LowValueText
	case 1:
		text = s.HighValueText
	default:
		return ""
	}
	if text != "" {
		return text
	}
	value := s.value(index)
	if s.valueTexts[index] == "" || s.valueTextOf[index] != value {
		s.valueTexts[index] = strconv.FormatFloat(float64(value), 'f', -1, 32)
		s.valueTextOf[index] = value
	}
	return s.valueTexts[index]
}

// DrawFunc is Draw as a stable function value, so a warm paint allocates nothing.
func (s *RangeSliderState) DrawFunc() func(*DrawContext) {
	if s.draw == nil {
		s.draw = s.Draw
	}
	return s.draw
}

func (s *RangeSliderState) bounds() (minimum, maximum, step float32) {
	minimum, maximum, step = s.Min, s.Max, s.Step
	if maximum <= minimum {
		maximum = minimum + 100
	}
	if step <= 0 {
		step = 1
	}
	return
}

// Draw paints the slider when package ui installed the painter.
func (s *RangeSliderState) Draw(d *DrawContext) {
	if s.Paint != nil {
		s.Paint(d)
	}
}

// Key moves the focused thumb with the arrows, PageUp/PageDown, Home and End.
func (s *RangeSliderState) Key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	index := -1
	if event.Target == s.lowID {
		index = 0
	} else if event.Target == s.highID {
		index = 1
	}
	if index < 0 {
		return gfx.EventResult{}
	}
	minimum, maximum, step := s.bounds()
	value := s.value(index)
	page := max((maximum-minimum)*0.1, step)
	target := value
	switch event.Key {
	case gfx.KeyLeft, gfx.KeyDown:
		target -= step
	case gfx.KeyRight, gfx.KeyUp:
		target += step
	case gfx.KeyPageDown:
		target -= page
	case gfx.KeyPageUp:
		target += page
	case gfx.KeyHome:
		target = minimum
	case gfx.KeyEnd:
		target = maximum
	default:
		return gfx.EventResult{}
	}
	target = min(max(target, minimum), maximum)
	if index == 0 {
		target = min(target, s.High)
	} else {
		target = max(target, s.Low)
	}
	if target == value {
		return gfx.EventResult{Handled: true}
	}
	s.setValue(index, target)
	return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target, FocusVisible: true}
}

// Pointer drags the thumb under the pointer, or the clicked track position.
func (s *RangeSliderState) Pointer(event gfx.PointerEvent) gfx.EventResult {
	switch event.Kind {
	case gfx.PointerDown:
		switch event.Target {
		case s.lowID:
			s.dragging = 1
		case s.highID:
			s.dragging = 2
		default:
			return gfx.EventResult{}
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target, FocusVisible: true}
	case gfx.PointerMove:
		if s.dragging == 0 {
			return gfx.EventResult{}
		}
		target := s.valueAt(event.X)
		if s.dragging == 1 {
			target = min(target, s.High)
		} else {
			target = max(target, s.Low)
		}
		s.setValue(s.dragging-1, target)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerUp:
		if s.dragging == 0 {
			return gfx.EventResult{}
		}
		s.dragging = 0
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerExit:
		if s.dragging != 0 {
			s.dragging = 0
			return gfx.EventResult{Handled: true}
		}
	}
	return gfx.EventResult{}
}

func (s *RangeSliderState) value(index int) float32 {
	if index == 0 {
		return s.Low
	}
	return s.High
}

func (s *RangeSliderState) setValue(index int, value float32) {
	minimum, maximum, step := s.bounds()
	value = min(max(value, minimum), maximum)
	if step > 0 {
		value = minimum + float32(int((value-minimum)/step+0.5))*step
	}
	if index == 0 {
		s.Low = min(value, s.High)
	} else {
		s.High = max(value, s.Low)
	}
}

func (s *RangeSliderState) valueAt(x float32) float32 {
	minimum, maximum, _ := s.bounds()
	if s.rect.Width <= 0 {
		return minimum
	}
	fraction := (x - s.rect.X) / s.rect.Width
	return min(max(minimum+fraction*(maximum-minimum), minimum), maximum)
}
