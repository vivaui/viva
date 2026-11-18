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
func ColorSwatch(c C, Color Color, selected bool, label string) Box {
	return colorSwatch(c, c.page.controlID(pkRadio), Color, selected, label)
}

func colorSwatch(c C, ID gfx.ID, Color Color, selected bool, label string) Box {
	b := Slot(c).Size(28, 28).Shrink(0)
	n := b.n()
	n.ID, n.label = ID, label
	// The swatch paints from what the element carries rather than from a Draw
	// callback: a closure made in each paint allocates in a hot path.
	extra := b.f.extra(b.i)
	extra.swatch, extra.swatchValue, extra.swatchChosen = true, Color, selected
	b.f.paintSwatchFn = (*frame).paintSwatch
	return b
}

// paintSwatch draws a ColorSwatch element.
func (f *frame) paintSwatch(n *node, r Rect) {
	extra := &f.exts[n.Ext]
	swatch := components.BeginColorSwatch(f.canvas, f.theme, n.ID, r, extra.swatchValue).Label(n.label).Selected(extra.swatchChosen).
		TabStop(!extra.swatchSkipTab)
	if extra.swatchFlush {
		swatch = swatch.Flush()
	}
	swatch.Paint()
}

// Flush makes a ColorSwatch fill its hairline border with its color instead of
// leaving a background-colored ring inside it, for a small swatch whose color
// would otherwise read as faded. Other elements ignore it.
func (b Box) Flush() Box {
	b.n()
	b.f.extra(b.i).swatchFlush = true
	return b
}

// TabStop says whether Tab may enter a ColorSwatch: a radio set gives its chosen
// swatch the stop and the others false, as components.ColorSwatch's TabStop. Other
// elements ignore it.
func (b Box) TabStop(on bool) Box {
	b.n()
	b.f.extra(b.i).swatchSkipTab = !on
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

// Draft is the color being staged while the panel is open, which follows the plane, the
// hue bar and the hex field as they change and is not yet the application's: a preview
// that tracks the drag reads it. It reports false when the panel is closed.
func (s *ColorPickerState) Draft() (Color, bool) {
	if !s.open {
		return Color{}, false
	}
	return s.draft.Color(), true
}

// Applied reports whether the user applied (or cleared) a color since the last time it was
// asked, so the application can save it; it reads true once.
func (s *ColorPickerState) Applied() bool {
	applied := s.applied
	s.applied = false
	return applied
}

// colorPickerLook is how a ColorPicker looks, set by its Box methods.
type colorPickerLook struct {
	label  string
	round  bool
	icon   *gfx.Icon
	empty  bool
	labels components.ColorPickerLabels
}

// ColorPicker adds a color trigger over *color that opens a panel to pick one: a plane for
// saturation and brightness, a hue bar, a hex field and Clear, Cancel and Apply. The
// color is staged while the panel is open and written to *color on Apply (Enter in the
// hex field); Cancel, Escape and a press outside leave it as it was. A transparent
// *color is no color, which Clear sets. The box is the 32 pixel trigger; Applied
// reports a change. RoundTrigger, TriggerIcon, NoColor and PanelLabels change how it looks.
//
//	ui.ColorPicker(c, &m.picker, &m.accent, "Accent")
//	if m.picker.Applied() { m.save() }
func ColorPicker(c C, state *ColorPickerState, Color *Color, label string) Box {
	return colorPicker(c, gfx.ID{}, state, Color, label)
}

// ColorPickerID is ColorPicker for a trigger the application already names: the picker
// keeps its state by identity, so it takes the id when it is made. A row's words can then
// be its LabelFor, as for any control the application names. A ColorSwatch, which keeps
// none, is named with Hit instead.
func ColorPickerID(c C, ID gfx.ID, state *ColorPickerState, Color *Color, label string) Box {
	return colorPicker(c, ID, state, Color, label)
}

func colorPicker(c C, ID gfx.ID, s *ColorPickerState, Color *Color, label string) Box {
	p := c.page
	p.forbidInMemo("ColorPicker")
	if ID.IsZero() {
		ID = p.controlID(pkControl)
	}
	s.target = Color
	if !s.bound {
		s.hex.Bind(&s.hexEditor)
		s.handlers.Key, s.handlers.Pointer = s.key, s.pointer
		s.bound = true
	}
	if s.open {
		p.overlay = true
		control := Control(c, label, &s.handlers).Absolute().Left(0).Top(0).Size(0, 0)
		control.HitWith(s.handlers.parts.At(8), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true, SkipFocus: true})
		p.fields = append(p.fields, fieldEntry{ID: s.handlers.parts.At(3), field: &s.hex})
		s.hex.ID = s.handlers.parts.At(3)
		s.syncHex()
	}
	b := Slot(c).Size(32, 32).Shrink(0)
	n := b.n()
	n.ID, n.label = ID, label
	// Like the swatch, the picker paints from what the element carries: a Draw
	// callback made for each paint would allocate one closure a paint.
	extra := b.f.extra(b.i)
	extra.picker, extra.pickerLook = s, colorPickerLook{label: label}
	b.f.paintPickerFn = (*frame).paintColorPicker
	return b
}

// RoundTrigger makes a ColorPicker's trigger a disc. Other elements ignore it.
func (b Box) RoundTrigger() Box {
	if extra := b.pickerExtra(); extra != nil {
		extra.pickerLook.round = true
	}
	return b
}

// TriggerIcon draws a glyph over a ColorPicker's trigger, such as a pipette for a custom
// color. Other elements ignore it.
func (b Box) TriggerIcon(icon *gfx.Icon) Box {
	if extra := b.pickerExtra(); extra != nil {
		extra.pickerLook.icon = icon
	}
	return b
}

// NoColor shows a ColorPicker's trigger as no color whatever its value, when empty. Other
// elements ignore it.
func (b Box) NoColor(empty bool) Box {
	if extra := b.pickerExtra(); extra != nil {
		extra.pickerLook.empty = empty
	}
	return b
}

// PanelLabels overrides the words of a ColorPicker's panel: its title, the plane, the hue
// bar, the hex field and the buttons. A label left empty keeps its default. Other
// elements ignore it.
func (b Box) PanelLabels(labels components.ColorPickerLabels) Box {
	if extra := b.pickerExtra(); extra != nil {
		extra.pickerLook.labels = labels
	}
	return b
}

func (b Box) pickerExtra() *extra {
	n := b.n()
	if n.Ext == 0 || b.f.exts[n.Ext].picker == nil {
		return nil
	}
	return &b.f.exts[n.Ext]
}

// paintColorPicker draws a ColorPicker's trigger, and its panel while it is open.
func (f *frame) paintColorPicker(n *node) {
	extra := &f.exts[n.Ext]
	ctx := Ctx{page: f.page, f: f}
	d := DrawContext{Canvas: f.canvas, Theme: &f.theme, Rect: n.rect, Label: n.label}
	extra.picker.paint(&d, &ctx, n.ID, extra.pickerLook)
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
		labels.Color = look.label
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

func (s *ColorPickerState) openPanel() {
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
