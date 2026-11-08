package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var colorPickerComponent = Component{
	ID: "color-picker", Title: "Color Picker", Category: "Forms", Summary: "应用持有 HSV 草稿，支持拖动、方向键、HEX 输入和确认取消。",
	paint: (*State).paintColorPicker, pointer: (*State).pointerColorPicker, key: (*State).keyColorPicker,
	initialize: func(s *State) {
		s.picker = &colorPickerDemoState{hex: "#3B82F6"}
		c, _ := components.ParseColorHex(s.picker.hex, false)
		s.picker.saved = components.ColorPickerValue{}.WithColor(c)
		s.picker.draft = s.picker.saved
	},
	editable: func(string) (editableField, bool) {
		return editableField{ID: demoPickerIDs.Hex, Initial: "#3B82F6"}, true
	},
	Examples: []Example{{ID: "default", Title: "选择颜色", Description: "拖动色相与饱和度/亮度，或输入 HEX。确认后应用，取消保留原色。", Height: 460, Snippet: `layout := components.BeginColorPicker(canvas, theme, trigger, components.ColorPickerOptions{
    IDs: ids, Value: draft, Hex: inputSnapshot, Open: open,
    Viewport: viewport, Layer: 3, Scope: scope,
}).Paint()
// Route pointer/key events to layout.Pointer / layout.Key.
// Resolve ids.Hex to your text.Editor and commit draft on ids.Apply.`},
		{ID: "alpha", Title: "透明度", Description: "可选透明度条和 #RRGGBBAA 输入；棋盘格显示透明部分。", Height: 480, Snippet: `options.ShowAlpha = true
// Use #RRGGBBAA in the caller-owned text editor.
color, valid := components.ParseColorHex(hex, true)
if valid { draft = draft.WithColor(color) }
layout := components.BeginColorPicker(canvas, theme, bounds, options).Paint()`}},
}

var demoPickerFeature = tree.Feature("color-picker")
var demoPickerScope = tree.ID(demoPickerFeature, "scope")
var demoPickerIDs = components.ColorPickerIDs{
	Trigger: tree.ID(demoPickerFeature, "trigger"), Dismiss: tree.ID(demoPickerFeature, "dismiss"),
	Saturation: tree.ID(demoPickerFeature, "sv"), Hue: tree.ID(demoPickerFeature, "hue"), Alpha: tree.ID(demoPickerFeature, "alpha"),
	Hex: tree.ID(demoPickerFeature, "hex"), Clear: tree.ID(demoPickerFeature, "clear"), Cancel: tree.ID(demoPickerFeature, "cancel"), Apply: tree.ID(demoPickerFeature, "apply"),
}

type colorPickerDemoState struct {
	saved, draft         components.ColorPickerValue
	layout               components.ColorPickerLayout
	hex, seen            string
	open, empty, invalid bool
}

func (s *State) pickerSetValue(v components.ColorPickerValue) {
	p := s.picker
	p.draft = v
	var buf [9]byte
	p.hex = string(v.AppendHex(buf[:0], s.exampleID == "alpha"))
	p.seen = p.hex
	p.invalid = false
	if s.editor != nil {
		*s.editor = text.Editor{}
		s.editor.Insert(p.hex)
	}
}

func (s *State) paintColorPicker(c *ui.Canvas, theme ui.Theme, size ui.Size) {
	p := s.picker
	if c.Clicked(demoPickerIDs.Trigger) {
		p.open = !p.open
		if p.open {
			s.pickerSetValue(p.saved)
		}
	}
	value := s.fieldValue(p.hex)
	if value != p.seen {
		p.seen = value
		parsed, ok := components.ParseColorHex(value, s.exampleID == "alpha")
		p.invalid = !ok
		if ok {
			p.draft = p.draft.WithColor(parsed)
		}
	}
	if c.Clicked(demoPickerIDs.Cancel) || c.Clicked(demoPickerIDs.Dismiss) {
		p.open = false
		c.RequestFocus(demoPickerIDs.Trigger)
	}
	if c.Clicked(demoPickerIDs.Clear) {
		p.empty = true
		p.open = false
		c.RequestFocus(demoPickerIDs.Trigger)
	}
	if c.Clicked(demoPickerIDs.Apply) && !p.invalid {
		p.saved = p.draft
		p.empty = false
		p.open = false
		c.RequestFocus(demoPickerIDs.Trigger)
	}
	c.Label(s.Text("自定义颜色", "Custom color"), ui.Rect{X: 24, Y: 20, Width: 300, Height: 24}, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
	hex := components.InputSnapshot{Value: value, Focused: c.Focused() == demoPickerIDs.Hex, Invalid: p.invalid}
	s.paintEditable(c, demoPickerIDs.Hex, value, &hex)
	o := components.ColorPickerOptions{IDs: demoPickerIDs, Value: p.saved, Open: p.open, Empty: p.empty, ShowAlpha: s.exampleID == "alpha", Viewport: ui.Rect{Width: size.Width, Height: size.Height}, Layer: 3, Scope: demoPickerScope,
		Hex: hex}
	if p.open {
		o.Value = p.draft
	}
	if p.invalid {
		o.Error = s.Text("请输入有效的十六进制色值", "Enter a valid hex color")
	}
	o.Labels = components.ColorPickerLabels{Color: s.Text("选择颜色", "Choose color"), Saturation: s.Text("饱和度与亮度", "Saturation and brightness"), Hue: s.Text("色相", "Hue"), Alpha: s.Text("透明度", "Opacity"), Hex: "HEX", Clear: s.Text("清空", "Clear"), Cancel: s.Text("取消", "Cancel"), Apply: s.Text("确定", "OK")}
	p.layout = components.BeginColorPicker(c, theme, ui.Rect{X: 24, Y: 52, Width: 32, Height: 32}, o).Paint()
}

func (s *State) pointerColorPicker(e ui.PointerEvent) ui.EventResult {
	if !s.picker.open {
		return ui.EventResult{}
	}
	v, r := s.picker.layout.Pointer(e, s.picker.draft)
	if r.Repaint {
		s.pickerSetValue(v)
	}
	return r
}
func (s *State) keyColorPicker(e ui.KeyEvent) ui.EventResult {
	if !s.picker.open {
		return ui.EventResult{}
	}
	v, r := s.picker.layout.Key(e, s.picker.draft)
	if r.Repaint {
		s.pickerSetValue(v)
	}
	return r
}
