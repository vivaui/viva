package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var colorSwatchComponent = Component{
	ID: "color-swatch", Title: "Color Swatch", Category: "Forms", Summary: "带选中环、键盘焦点和禁用状态的颜色选择。",
	paint: (*State).paintColorSwatch, key: (*State).keyColorSwatch,
	Examples: []Example{{ID: "palette", Title: "颜色选择", Description: "选择一种颜色，或恢复默认颜色。最后一个选项展示禁用状态。", Snippet: `swatch := components.BeginColorSwatch(canvas, theme, id, bounds, color).
	    Label("Blue").Selected(selected == id).TabStop(selected == id)
if swatch.Paint().Clicked() {
    selected = id
}`}},
}

var colorSwatchItems = tree.Sub(tree.Feature("color-swatch"), tree.Name("color"))

var colorSwatchOptions = [...]components.RadioOption{
	{ID: colorSwatchItems.At(0)},
	{ID: colorSwatchItems.At(1)},
	{ID: colorSwatchItems.At(2)},
	{ID: colorSwatchItems.At(3), Disabled: true},
}

func (state *State) keyColorSwatch(event gfx.KeyEvent) gfx.EventResult {
	if listIndex(event.Target, colorSwatchItems) < 0 {
		return gfx.EventResult{}
	}
	next, handled := components.RadioGroupKeyDown(event.Target, colorSwatchOptions[:], components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	nextValue := int32(listIndex(next, colorSwatchItems))
	changed := state.values[0] != nextValue
	state.values[0] = nextValue
	return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: changed}
}

func (state *State) paintColorSwatch(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var slots [4]ui.Box
	state.drawPage(canvas, theme, size, 360, state.Text("连接颜色", "Connection color"), func(c ui.C) {
		ui.Row(c).Gap(12).Children(func() {
			for i := range slots {
				slots[i] = ui.Slot(c).Size(36, 36)
			}
		})
		ui.Wrap(c, state.Text("Tab 进入色板，方向键移动，Space 选择。", "Tab enters the palette, arrows move, and Space selects.")).
			FontSize(theme.Metrics.FontSizeS).Muted()
	})
	colors := [...]gfx.Color{{}, {Green: 0.77, Red: 0.13, Blue: 0.37, Alpha: 1}, {Red: 0.23, Green: 0.51, Blue: 0.96, Alpha: 1}, {Red: 0.94, Green: 0.27, Blue: 0.27, Alpha: 1}}
	labels := [...]string{"No color", "Green", "Blue", "Red (disabled)"}
	for i, color := range colors {
		selected := state.values[0] == int32(i)
		swatch := components.BeginColorSwatch(canvas, theme, colorSwatchItems.At(i), slots[i].Rect(), color).Label(labels[i]).Selected(selected).TabStop(selected)
		if i == 3 {
			swatch = swatch.Disabled()
		}
		if swatch.Paint().Clicked() {
			state.values[0] = int32(i)
		}
	}
}
