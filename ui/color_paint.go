package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintSwatch draws a ColorSwatch element.
func (f *frame) paintSwatch(n *node, r Rect) {
	extra := &f.Exts[n.Ext]
	swatch := components.BeginColorSwatch(f.Surface, f.Palette, n.ID, r, extra.swatchValue).Label(n.Label).Selected(extra.swatchChosen).
		TabStop(!extra.swatchSkipTab)
	if extra.swatchFlush {
		swatch = swatch.Flush()
	}
	swatch.Paint()
}

// parts are the identities the panel's regions take.
func (s *ColorPickerState) ids(trigger gfx.ID) components.ColorPickerIDs {
	if !s.open {
		return components.ColorPickerIDs{Trigger: trigger}
	}
	at := s.handlers.parts.At
	return components.ColorPickerIDs{Trigger: trigger, Saturation: at(0), Hue: at(1), Hex: at(3), Clear: at(4), Cancel: at(5), Apply: at(6), Dismiss: at(7)}
}

func (s *ColorPickerState) paint(d *DrawContext, c C, ID gfx.ID, look colorPickerLook) {
	canvas := d.Canvas
	ids := s.ids(ID)
	if canvas.Clicked(ID) {
		if s.open {
			s.close()
		} else {
			s.openPanel()
		}
		return
	}
	if s.restoreFocus {
		canvas.RequestFocus(ID)
		s.restoreFocus = false
	}
	labels := look.labels
	if labels.Color == "" {
		labels.Color = look.Label
	}
	value, empty := s.draft, false
	if !s.open {
		value = value.WithColor(*s.target)
		empty = s.target.Alpha == 0
	}
	empty = empty || look.empty
	options := components.ColorPickerOptions{
		IDs: ids, Labels: labels, Value: value, Open: s.open, Empty: empty,
		RoundTrigger: look.round, TriggerIcon: look.icon, SaturationText: s.summary,
		Viewport: Rect{Width: c.Bounds().Width, Height: c.Bounds().Height}, Layer: c.f.WidgetLayer() + 2,
	}
	if s.open {
		options.Scope = s.handlers.parts.At(8)
		options.Hex = s.hex.snapshot(c.f, ids.Hex, labels.Hex, "#3B82F6")
		if s.invalid {
			options.Error = "Invalid color"
		}
	}
	picker := components.BeginColorPicker(canvas, *d.Theme, d.Rect, options)
	picker.PaintTrigger()
	if !s.open {
		return
	}
	s.layout = picker.PaintPanel()
	switch {
	case canvas.Clicked(ids.Dismiss), canvas.Clicked(ids.Cancel):
		s.close()
	case canvas.Clicked(ids.Clear):
		*s.target, s.applied = Color{}, true
		s.close()
	case canvas.Clicked(ids.Apply):
		s.apply()
	}
}

func (s *ColorPickerState) openPanel() {
	color := *s.target
	if color.Alpha == 0 {
		color = Color{Red: 0.231, Green: 0.51, Blue: 0.965, Alpha: 1} // #3B82F6, as the field's placeholder
	}
	s.setValue(components.ColorPickerValue{}.WithColor(color))
	s.open = true
}

// syncHex takes what was typed in the hex field as the staged color, when it is one.
func (s *ColorPickerState) syncHex() {
	value := s.hexEditor.String()
	if value == s.seen {
		return
	}
	s.seen = value
	color, valid := components.ParseColorHex(value, false)
	s.invalid = !valid
	if valid {
		s.draft = s.draft.WithColor(color)
		s.summary = summary(s.draft)
	}
}
