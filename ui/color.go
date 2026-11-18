package ui

import (
	"strconv"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// ColorSwatch adds a circular color choice, a radio button in a group of them: color is
// what it shows (a transparent color is "no color", drawn as a slash), selected rings
// it, and label names it for assistive technology. A click is Clicked; the application
// owns the choice. It is as wide and tall as its Size, 28 when it has none.
//
//	if ui.ColorSwatch(c.Key(name), color, m.color == color, name).Clicked() { m.color = color }
func ColorSwatch(c C, color Color, selected bool, label string) Box {
	return colorSwatch(c, c.page.controlID(pkRadio), color, selected, label)
}

// ColorSwatchID is ColorSwatch for a choice the application already names.
func ColorSwatchID(c C, id gfx.ID, color Color, selected bool, label string) Box {
	return colorSwatch(c, id, color, selected, label)
}

func colorSwatch(c C, id gfx.ID, color Color, selected bool, label string) Box {
	b := Slot(c).Size(28, 28).Shrink(0)
	n := b.n()
	n.id, n.label = id, label
	b.Draw(func(d *DrawContext) {
		components.BeginColorSwatch(d.Canvas, *d.Theme, id, d.Rect, color).Label(label).Selected(selected).Paint()
	})
	return b
}

// ColorPickerState is what a ColorPicker keeps between paints: whether its panel is open
// and the color being staged in it. Keep one per picker in the model; the zero value is
// closed.
type ColorPickerState struct {
	open, restoreFocus, applied bool
	draft                       components.ColorPickerValue
	hexEditor                   text.Editor
	hex                         Field
	seen, summary               string
	invalid                     bool
	handlers                    Handlers
	layout                      components.ColorPickerLayout
	target                      *Color
	bound                       bool
}

// Open reports whether the picker's panel is open.
func (s *ColorPickerState) Open() bool { return s.open }

// Applied reports whether the user applied (or cleared) a color since the last time it was
// asked, so the application can save it; it reads true once.
func (s *ColorPickerState) Applied() bool {
	applied := s.applied
	s.applied = false
	return applied
}

// ColorPickerSpec is how a ColorPicker looks: Label names it for assistive technology
// (and heads its panel), Round makes the trigger a disc, Icon draws a glyph over it (a
// pipette for a custom color), Empty shows it as no color whatever its value, and
// Labels override the panel's words.
type ColorPickerSpec struct {
	Label  string
	Round  bool
	Icon   *gfx.Icon
	Empty  bool
	Labels components.ColorPickerLabels
}

// ColorPicker adds a color trigger over *color that opens a panel to pick one: a plane for
// saturation and brightness, a hue bar, a hex field and Clear, Cancel and Apply. The
// color is staged while the panel is open and written to *color on Apply (Enter in the
// hex field); Cancel, Escape and a press outside leave it as it was. A transparent
// *color is no color, which Clear sets. The box is the 32 pixel trigger; Applied
// reports a change.
//
//	ui.ColorPicker(c, &m.picker, &m.accent, "Accent")
//	if m.picker.Applied() { m.save() }
func ColorPicker(c C, state *ColorPickerState, color *Color, label string) Box {
	return ColorPickerWith(c, state, ColorPickerSpec{Label: label}, color)
}

// ColorPickerWith is ColorPicker with a spec.
func ColorPickerWith(c C, s *ColorPickerState, spec ColorPickerSpec, color *Color) Box {
	p := c.page
	p.forbidInMemo("ColorPicker")
	id := p.controlID(pkControl)
	s.target = color
	if !s.bound {
		s.hex.Bind(&s.hexEditor)
		s.handlers.Key, s.handlers.Pointer = s.key, s.pointer
		s.bound = true
	}
	if s.open {
		p.overlay = true
		control := Control(c, spec.Label, &s.handlers).Absolute().Left(0).Top(0).Size(0, 0)
		control.HitWith(s.handlers.parts.At(8), spec.Label, gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true, SkipFocus: true})
		p.fields = append(p.fields, fieldEntry{id: s.handlers.parts.At(3), field: &s.hex})
		s.hex.id = s.handlers.parts.At(3)
		s.syncHex()
	}
	b := Slot(c).Size(32, 32).Shrink(0)
	n := b.n()
	n.id, n.label = id, spec.Label
	b.DrawOver(func(d *DrawContext) { s.paint(d, c, id, spec) })
	return b
}

// parts are the identities the panel's regions take.
func (s *ColorPickerState) ids(trigger gfx.ID) components.ColorPickerIDs {
	if !s.open {
		return components.ColorPickerIDs{Trigger: trigger}
	}
	at := s.handlers.parts.At
	return components.ColorPickerIDs{Trigger: trigger, Saturation: at(0), Hue: at(1), Hex: at(3), Clear: at(4), Cancel: at(5), Apply: at(6), Dismiss: at(7)}
}

func (s *ColorPickerState) paint(d *DrawContext, c C, id gfx.ID, spec ColorPickerSpec) {
	canvas := d.Canvas
	ids := s.ids(id)
	if canvas.Clicked(id) {
		if s.open {
			s.close()
		} else {
			s.openPanel(spec)
		}
		return
	}
	if s.restoreFocus {
		canvas.RequestFocus(id)
		s.restoreFocus = false
	}
	labels := spec.Labels
	if labels.Color == "" {
		labels.Color = spec.Label
	}
	value, empty := s.draft, false
	if !s.open {
		value = value.WithColor(*s.target)
		empty = s.target.Alpha == 0
	}
	empty = empty || spec.Empty
	options := components.ColorPickerOptions{
		IDs: ids, Labels: labels, Value: value, Open: s.open, Empty: empty,
		RoundTrigger: spec.Round, TriggerIcon: spec.Icon, SaturationText: s.summary,
		Viewport: Rect{Width: c.Bounds().Width, Height: c.Bounds().Height}, Layer: c.f.widgetLayer() + 2,
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

func (s *ColorPickerState) openPanel(ColorPickerSpec) {
	color := *s.target
	if color.Alpha == 0 {
		color = Color{Red: 0.231, Green: 0.51, Blue: 0.965, Alpha: 1} // #3B82F6, as the field's placeholder
	}
	s.setValue(components.ColorPickerValue{}.WithColor(color))
	s.open = true
}

func (s *ColorPickerState) close() { s.open, s.restoreFocus = false, true }

func (s *ColorPickerState) apply() {
	s.syncHex()
	if s.invalid {
		return
	}
	*s.target, s.applied = s.draft.Color(), true
	s.close()
}

func (s *ColorPickerState) setValue(v components.ColorPickerValue) {
	s.draft = v
	var buf [9]byte
	value := string(v.AppendHex(buf[:0], false))
	if s.hexEditor.String() != value {
		s.hexEditor = text.Editor{}
		s.hexEditor.Insert(value)
	}
	s.seen, s.invalid = value, false
	s.summary = summary(v)
}

func summary(v components.ColorPickerValue) string {
	return strconv.Itoa(int(v.Saturation*100+.5)) + "% / " + strconv.Itoa(int(v.Brightness*100+.5)) + "%"
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

func (s *ColorPickerState) pointer(event gfx.PointerEvent) gfx.EventResult {
	if !s.open {
		return gfx.EventResult{}
	}
	value, result := s.layout.Pointer(event, s.draft)
	if value != s.draft {
		s.setValue(value)
	}
	return result
}

func (s *ColorPickerState) key(event gfx.KeyEvent) gfx.EventResult {
	if !s.open || event.Release {
		return gfx.EventResult{}
	}
	hex := s.handlers.parts.At(3)
	if event.Key == gfx.KeyEscape || event.Key == gfx.KeyEnter && event.Target == hex && event.Modifiers == 0 {
		if _, composing := s.hexEditor.Composition(); composing {
			return gfx.EventResult{}
		}
		if !event.Repeat {
			if event.Key == gfx.KeyEscape {
				s.close()
			} else {
				s.apply()
			}
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	value, result := s.layout.Key(event, s.draft)
	if value != s.draft {
		s.setValue(value)
	}
	return result
}
