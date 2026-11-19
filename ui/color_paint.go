package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintSwatch draws a ColorSwatch element.
func paintSwatch(f *Frame, n *Node, r Rect) {
	extra := &f.Exts[n.Ext]
	disabled := n.Flags&FlagDisabled != 0
	swatch := components.BeginColorSwatch(f.Surface, f.Palette, n.ID, r, extra.GetSwatchValue()).Label(n.Label).Selected(extra.GetSwatchChosen()).
		TabStop(!disabled && !extra.GetSwatchSkipTab())
	if disabled {
		swatch = swatch.Disabled()
	}
	if extra.GetSwatchFlush() {
		swatch = swatch.Flush()
	}
	swatch.Paint()
}

// parts are the identities the panel's regions take.
func ids(s *ColorPickerState, trigger gfx.ID) components.ColorPickerIDs {
	if !s.GetOpen() {
		return components.ColorPickerIDs{Trigger: trigger}
	}
	at := s.GetHandlers().GetParts().At
	return components.ColorPickerIDs{Trigger: trigger, Saturation: at(0), Hue: at(1), Alpha: at(2), Hex: at(3), Clear: at(4), Cancel: at(5), Apply: at(6), Dismiss: at(7)}
}

func paintColorPickerPanel(s *ColorPickerState, d *DrawContext, c C, ID gfx.ID, look ColorPickerLook) {
	canvas := d.Canvas
	ids := ids(s, ID)
	if canvas.Clicked(ID) {
		if s.GetOpen() {
			s.ClosePanel()
		} else {
			openPanel(s)
		}
		return
	}
	if s.GetRestoreFocus() {
		canvas.RequestFocus(ID)
		s.SetRestoreFocusState(false)
	}
	labels := look.GetLabels()
	if labels.Color == "" {
		labels.Color = look.Label
	}
	value, empty := s.GetDraft(), false
	if !s.GetOpen() {
		value = value.WithColor(*s.GetTarget())
		empty = s.GetTarget().Alpha == 0
	}
	empty = empty || look.GetEmpty()
	options := components.ColorPickerOptions{
		IDs: ids, Labels: labels, Value: value, Open: s.GetOpen(), Empty: empty, ShowAlpha: s.ShowsAlpha(),
		RoundTrigger: look.GetRound(), TriggerIcon: look.GetIcon(), SaturationText: s.Summary,
		Viewport: Rect{Width: c.Bounds().Width, Height: c.Bounds().Height}, Layer: c.GetF().WidgetLayer() + 2,
	}
	if s.GetOpen() {
		options.Scope = s.GetHandlers().GetParts().At(8)
		options.Hex = s.HexPtr().Snapshot(c.GetF(), ids.Hex, labels.Hex, "#3B82F6")
		if s.GetInvalid() {
			options.Error = "Invalid color"
		}
	}
	picker := components.BeginColorPicker(canvas, *d.Theme, d.Rect, options)
	picker.PaintTrigger()
	if !s.GetOpen() {
		return
	}
	s.SetLayoutValue(picker.PaintPanel())
	switch {
	case canvas.Clicked(ids.Dismiss), canvas.Clicked(ids.Cancel):
		s.ClosePanel()
	case canvas.Clicked(ids.Clear):
		*s.GetTarget() = Color{}
		s.SetAppliedValue(true)
		s.ClosePanel()
	case canvas.Clicked(ids.Apply):
		s.ApplyDraft()
	}
}

func openPanel(s *ColorPickerState) {
	color := *s.GetTarget()
	if color.Alpha == 0 { // no color: start from the field's placeholder
		color = Color{Red: 0.231, Green: 0.51, Blue: 0.965, Alpha: 1} // #3B82F6
	} else if !s.ShowsAlpha() {
		color.Alpha = 1
	}
	s.SetDraftValue(components.ColorPickerValue{}.WithColor(color))
	s.SetOpenState(true)
}
