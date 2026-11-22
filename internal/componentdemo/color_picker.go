package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var colorPickerComponent = Component{
	ID: "color-picker", Title: "Color Picker", Category: "Forms", Summary: "应用持有 HSV 草稿，支持拖动、方向键、HEX 输入和确认取消。",
	paint: (*State).paintColorPicker,
	initialize: func(s *State) {
		p := &colorPickerDemoState{color: gfx.RGB(59, 130, 246)}
		p.saved = p.color
		s.picker = p
	},
	Examples: []Example{{ID: "default", Title: "选择颜色", Description: "拖动色相与饱和度/亮度，或输入 HEX。确认后应用，取消保留原色。", Height: 460, Snippet: `layout := components.BeginColorPicker(canvas, theme, trigger, components.ColorPickerOptions{
    IDs: ids, Value: draft, Hex: inputSnapshot, Open: open,
    Viewport: viewport, Layer: 3, Scope: scope,
}).Paint()
// Route pointer/key events to layout.Pointer / layout.Key.
// Resolve ids.Hex to your text.Editor and commit draft on ids.Apply.`,
		PageSnippet: `// m.color starts as #3B82F6.
ui.ColorPicker(c, &m.picker, &m.color, "Custom color")
if m.picker.Applied() {
    m.saved = m.color
}`, PageSnippetZh: `// m.color starts as #3B82F6.
ui.ColorPicker(c, &m.picker, &m.color, "自定义颜色")
if m.picker.Applied() {
    m.saved = m.color
}`},
		{ID: "alpha", Title: "透明度", Description: "可选透明度条和 #RRGGBBAA 输入；棋盘格显示透明部分。", Height: 480, Snippet: `options.ShowAlpha = true
// Use #RRGGBBAA in the caller-owned text editor.
color, valid := components.ParseColorHex(hex, true)
if valid { draft = draft.WithColor(color) }
layout := components.BeginColorPicker(canvas, theme, bounds, options).Paint()`, PageSnippet: `// m.color starts as #3B82F6; ShowAlpha stages and applies #RRGGBBAA.
ui.ColorPicker(c, &m.picker, &m.color, "Custom color").ShowAlpha(true)
if m.picker.Applied() {
    m.saved = m.color
}`, PageSnippetZh: `// m.color 初始为 #3B82F6；ShowAlpha 用 #RRGGBBAA 暂存并应用颜色。
ui.ColorPicker(c, &m.picker, &m.color, "自定义颜色").ShowAlpha(true)
if m.picker.Applied() {
    m.saved = m.color
}`}},
}

// colorPickerDemoState is the page-first picker and the colors it writes.
type colorPickerDemoState struct {
	picker ui.ColorPickerState
	color  ui.Color
	saved  ui.Color
}

func (s *State) paintColorPicker(c *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	alpha := s.exampleID == "alpha"
	title := s.Text("Color Picker · 自定义颜色", "Color Picker · Custom color")
	caption := s.Text("拖动色相与饱和度/亮度，输入 HEX；确认后应用，取消保留原色。", "Drag the hue and saturation/brightness, or type a HEX value. Apply commits it; Cancel keeps the original.")
	if alpha {
		caption = s.Text("拖动透明度条，或输入 #RRGGBBAA；棋盘格显示透明部分。", "Drag the opacity bar, or type #RRGGBBAA. The checkerboard shows the transparent part.")
	}
	p := s.picker
	s.drawPage(c, theme, size, 460, title, func(page ui.C) {
		ui.Column(page).Gap(8).Children(func() {
			ui.ColorPicker(page, &p.picker, &p.color, s.Text("自定义颜色", "Custom color")).ShowAlpha(alpha)
			ui.Text(page, caption).FontSize(theme.Metrics.FontSizeS).Muted()
		})
		if p.picker.Applied() {
			p.saved = p.color
		}
	})
}
