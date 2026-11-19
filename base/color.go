package base

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// SetHexText replaces the hex field's text, as typing the value does.
func (s *ColorPickerState) SetHexText(value string) {
	s.hexEditor = text.Editor{}
	s.hexEditor.Insert(value)
}

// ColorSwatch adds a circular color choice, a radio button in a group of them: color is
// what it shows (a transparent color is "no color", drawn as a slash), selected rings
// it, and label names it for assistive technology. A click is Clicked; the application
// owns the choice. It is as wide and tall as its Size, 28 when it has none. Disabled
// makes it dimmed and inert, as it does for the other controls.
//
//	if ui.ColorSwatch(c.Key(name), color, m.color == color, name).Clicked() { m.color = color }
func ColorSwatch(c C, Color Color, selected bool, Label string) Box {
	return colorSwatch(c, c.page.ControlID(pkRadio), Color, selected, Label)
}

func colorSwatch(c C, ID gfx.ID, Color Color, selected bool, Label string) Box {
	b := Slot(c).Size(28, 28).Shrink(0)
	n := b.N()
	n.ID, n.Label = ID, Label
	// The swatch paints from what the element carries rather than from a Draw
	// callback: a closure made in each paint allocates in a hot path.
	extra := b.f.Extra(b.i)
	extra.swatch, extra.swatchValue, extra.swatchChosen = true, Color, selected
	b.f.paintSwatchFn = (*Frame).paintSwatch
	return b
}

// Flush makes a ColorSwatch fill its hairline border with its color instead of
// leaving a background-colored ring inside it, for a small swatch whose color
// would otherwise read as faded. Other elements ignore it.
func (b Box) Flush() Box {
	b.N()
	b.f.Extra(b.i).swatchFlush = true
	return b
}

// TabStop says whether Tab may enter a ColorSwatch: a radio set gives its chosen
// swatch the stop and the others false, as components.ColorSwatch's TabStop. Other
// elements ignore it.
func (b Box) TabStop(on bool) Box {
	b.N()
	b.f.Extra(b.i).swatchSkipTab = !on
	return b
}

// ColorPickerState is what a ColorPicker keeps between paints: whether its panel is open
// and the color being staged in it. Keep one per picker in the model; the zero value is
// closed.
type ColorPickerState struct {
	open, restoreFocus, applied bool
	showAlpha                   bool
	draft                       ColorPickerValue
	hexEditor                   text.Editor
	hex                         Field
	seen, Summary               string
	invalid                     bool
	handlers                    Handlers
	layout                      ColorPickerLayout
	target                      *Color
	Bound                       bool
}

// Open reports whether the picker's panel is open.
func (s *ColorPickerState) Open() bool { return s.open }

// ShowsAlpha reports whether the picker offers an opacity bar and reads and writes
// #RRGGBBAA, as ShowAlpha set it.
func (s *ColorPickerState) ShowsAlpha() bool { return s.showAlpha }

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
type ColorPickerLook struct {
	Label  string
	round  bool
	icon   *gfx.Icon
	empty  bool
	labels ColorPickerLabels
}

// ColorPicker adds a color trigger over *color that opens a panel to pick one: a plane for
// saturation and brightness, a hue bar, a hex field and Clear, Cancel and Apply. The
// color is staged while the panel is open and written to *color on Apply (Enter in the
// hex field); Cancel, Escape and a press outside leave it as it was. A transparent
// *color is no color, which Clear sets. The box is the 32 pixel trigger; Applied
// reports a change. RoundTrigger, TriggerIcon, NoColor and PanelLabels change how it
// looks, and ShowAlpha adds an opacity bar and #RRGGBBAA input.
//
//	ui.ColorPicker(c, &m.picker, &m.accent, "Accent")
//	if m.picker.Applied() { m.save() }
func ColorPicker(c C, state *ColorPickerState, Color *Color, Label string) Box {
	return colorPicker(c, gfx.ID{}, state, Color, Label)
}

// ColorPickerID is ColorPicker for a trigger the application already names: the picker
// keeps its state by identity, so it takes the id when it is made. A row's words can then
// be its LabelFor, as for any control the application names. A ColorSwatch, which keeps
// none, is named with Hit instead.
func ColorPickerID(c C, ID gfx.ID, state *ColorPickerState, Color *Color, Label string) Box {
	return colorPicker(c, ID, state, Color, Label)
}

func colorPicker(c C, ID gfx.ID, s *ColorPickerState, Color *Color, Label string) Box {
	p := c.page
	p.ForbidInMemo("ColorPicker")
	if ID.IsZero() {
		ID = p.ControlID(pkControl)
	}
	s.target = Color
	if !s.Bound {
		s.hex.Bind(&s.hexEditor)
		s.handlers.Key, s.handlers.Pointer = s.key, s.pointer
		s.Bound = true
	}
	if s.open {
		p.overlay = true
		control := Control(c, Label, &s.handlers).Absolute().Left(0).Top(0).Size(0, 0)
		control.HitWith(s.handlers.parts.At(8), Label, gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true, SkipFocus: true})
		p.fields = append(p.fields, fieldEntry{ID: s.handlers.parts.At(3), field: &s.hex})
		s.hex.ID = s.handlers.parts.At(3)
		s.syncHex()
	}
	b := Slot(c).Size(32, 32).Shrink(0)
	n := b.N()
	n.ID, n.Label = ID, Label
	// Like the swatch, the picker paints from what the element carries: a Draw
	// callback made for each paint would allocate one closure a paint.
	extra := b.f.Extra(b.i)
	extra.picker, extra.pickerLook = s, ColorPickerLook{Label: Label}
	b.f.paintPickerFn = (*Frame).PaintColorPicker
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
// bar, the alpha bar, the hex field and the buttons. A label left empty keeps its
// default. Other elements ignore it.
func (b Box) PanelLabels(labels ColorPickerLabels) Box {
	if extra := b.pickerExtra(); extra != nil {
		extra.pickerLook.labels = labels
	}
	return b
}

// ShowAlpha makes a ColorPicker offer an opacity bar and read and write #RRGGBBAA,
// so the chosen color keeps its alpha; without it a picked color is opaque. Other
// elements ignore it.
func (b Box) ShowAlpha(show bool) Box {
	if extra := b.pickerExtra(); extra != nil {
		extra.picker.showAlpha = show
	}
	return b
}

func (b Box) pickerExtra() *Extra {
	n := b.N()
	if n.Ext == 0 || b.f.Exts[n.Ext].picker == nil {
		return nil
	}
	return &b.f.Exts[n.Ext]
}

// paintColorPicker draws a ColorPicker's trigger, and its panel while it is open.
func (f *Frame) PaintColorPicker(n *Node) {
	extra := &f.Exts[n.Ext]
	// The picker's painter is handed the frame's own scratch: a context and a
	// draw context made here would escape through the call and allocate once
	// per picker in each paint.
	f.pickerCtx.page, f.pickerCtx.f = f.page, f
	d := &f.drawContext
	*d = DrawContext{Canvas: f.Surface, Theme: &f.Palette, Rect: n.Rect, Label: n.Label}
	extra.picker.paint(d, &f.pickerCtx, n.ID, extra.pickerLook)
}

func (s *ColorPickerState) ClosePanel() { s.open, s.restoreFocus = false, true }

func (s *ColorPickerState) ApplyDraft() {
	s.syncHex()
	if s.invalid {
		return
	}
	*s.target, s.applied = s.draft.Color(), true
	s.ClosePanel()
}

func (s *ColorPickerState) SetDraftValue(v ColorPickerValue) {
	s.draft = v
	var buf [9]byte
	value := string(v.AppendHex(buf[:0], s.showAlpha))
	if s.hexEditor.String() != value {
		s.hexEditor = text.Editor{}
		s.hexEditor.Insert(value)
	}
	s.seen, s.invalid = value, false
	s.Summary = Summary(v)
}

func Summary(v ColorPickerValue) string {
	return strconv.Itoa(int(v.Saturation*100+.5)) + "% / " + strconv.Itoa(int(v.Brightness*100+.5)) + "%"
}

func (s *ColorPickerState) pointer(event gfx.PointerEvent) gfx.EventResult {
	if !s.open {
		return gfx.EventResult{}
	}
	value, result := s.layout.Pointer(event, s.draft)
	if value != s.draft {
		s.SetDraftValue(value)
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
				s.ClosePanel()
			} else {
				s.ApplyDraft()
			}
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	value, result := s.layout.Key(event, s.draft)
	if value != s.draft {
		s.SetDraftValue(value)
	}
	return result
}

// WidgetLayer is the layer a widget's children paint on.
func (f *Frame) WidgetLayer() uint16 { return max(WidgetsOf(f).layer, WidgetLayerBase) }

// syncHex takes what was typed in the hex field as the staged color, when it is one.
func (s *ColorPickerState) syncHex() {
	value := s.hexEditor.String()
	if value == s.seen {
		return
	}
	s.seen = value
	color, valid := ParseColorHex(value, s.showAlpha)
	s.invalid = !valid
	if valid {
		s.draft = s.draft.WithColor(color)
		s.Summary = Summary(s.draft)
	}
}
