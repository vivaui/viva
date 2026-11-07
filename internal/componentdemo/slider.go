package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var sliderComponent = Component{
	ID: "slider", Title: "Slider", Category: "Forms", Summary: "调用方持有数值的单值与范围滑块。",
	paint: (*State).paintSlider, pointer: (*State).pointerSlider, key: (*State).keySlider, initialize: (*State).initializeSlider,
	Examples: []Example{
		{ID: "default", Title: "音量", Description: "主指针拖动按十为步长更新，激活仍可循环调用方数值。", Snippet: `components.BeginSlider(canvas, theme, tree.ID(page, "volume"), bounds, float32(volume)).
    Step(10).Paint("Volume")
if canvas.Clicked(tree.ID(page, "volume")) {
    volume = (volume + 10) % 110
}`},
		{ID: "range", Title: "价格区间", Description: "两个稳定 thumb ID 分别接收坐标并约束下限与上限。", Snippet: `thumbs := []components.SliderThumbValue{
    {ID: tree.ID(page, "lower"), Label: "Minimum price", ValueText: "25", Value: 25},
    {ID: tree.ID(page, "upper"), Label: "Maximum price", ValueText: "75", Value: 75},
}
components.BeginRangeSlider(canvas, theme, bounds, thumbs).Step(5).Paint("Price range")`},
		{ID: "states", Title: "方向与禁用", Description: "同时展示竖向滑块与禁用状态。", Snippet: `components.BeginSlider(canvas, theme, tree.ID(page, "volume"), volumeBounds,
    float32(volume)).Vertical().Step(10).Paint("Volume")
components.BeginSlider(canvas, theme, tree.ID(page, "disabled"), disabledBounds, 64).
    Disabled().Paint("Disabled slider")`},
	},
}

var (
	sliderUI       = tree.Feature("slider")
	sliderPrimary  = tree.ID(sliderUI, "primary")
	sliderLower    = tree.ID(sliderUI, "lower")
	sliderUpper    = tree.ID(sliderUI, "upper")
	sliderDisabled = tree.ID(sliderUI, "disabled")
)

var sliderRangePresets = [...][2]components.SliderThumbValue{
	{
		{ID: sliderLower, ValueText: "25", Value: 25},
		{ID: sliderUpper, ValueText: "75", Value: 75},
	},
	{
		{ID: sliderLower, ValueText: "40", Value: 40},
		{ID: sliderUpper, ValueText: "90", Value: 90},
	},
}

// sliderRangeThumbs returns a preset with its thumb labels in the preview's
// language; caller-owned thumb text is created here, not while painting.
func (state *State) sliderRangeThumbs(index int) [2]components.SliderThumbValue {
	thumbs := sliderRangePresets[index]
	thumbs[0].Label = state.Text("最低价格", "Minimum price")
	thumbs[1].Label = state.Text("最高价格", "Maximum price")
	return thumbs
}

func (state *State) initializeSlider() {
	bag := state.bag()
	if state.exampleID == "range" {
		state.values[0] = 25
		state.values[1] = 75
		bag.thumbs = state.sliderRangeThumbs(0)
		return
	}
	state.values[0] = 40
}

func (state *State) toggleSliderRangePreset() {
	state.flags ^= 1
	preset := state.sliderRangeThumbs(int(state.flags & 1))
	state.values[0] = int32(preset[0].Value)
	state.values[1] = int32(preset[1].Value)
	state.bag().thumbs = preset
}

func (state *State) pointerSlider(event ui.PointerEvent) ui.EventResult {
	if event.Target != sliderPrimary && event.Target != sliderLower && event.Target != sliderUpper {
		return ui.EventResult{}
	}
	if event.Kind != ui.PointerDown && !(event.Kind == ui.PointerMove && event.Captured) {
		return ui.EventResult{Handled: event.Kind == ui.PointerUp}
	}
	if event.Kind == ui.PointerDown && event.Button != ui.PointerButtonPrimary {
		return ui.EventResult{}
	}
	position := event.X
	if state.exampleID == "states" {
		position = event.Y
	}
	value := int32(state.sliderForExample().ValueAt(position) + 0.5)
	index := 0
	if event.Target == sliderUpper {
		index = 1
	}
	if state.exampleID == "range" {
		if index == 0 && value > state.values[1] {
			value = state.values[1]
		}
		if index == 1 && value < state.values[0] {
			value = state.values[0]
		}
	}
	changed := state.values[index] != value
	state.values[index] = value
	if state.exampleID == "range" {
		thumbs := &state.bag().thumbs
		thumbs[index].Value = float32(value)
		thumbs[index].ValueText = sliderDemoValueText(value)
	}
	return ui.EventResult{Focus: event.Target, FocusVisible: false, Handled: true, Repaint: changed}
}

func (state *State) keySlider(event ui.KeyEvent) ui.EventResult {
	index := 0
	switch event.Target {
	case sliderPrimary:
	case sliderLower:
		index = 0
	case sliderUpper:
		index = 1
	default:
		return ui.EventResult{}
	}
	values := []float32{float32(state.values[0]), float32(state.values[1])}
	if state.exampleID == "range" {
		if !components.RangeSliderKeyDown(values, index, 0, 100, 5, 10, 1, components.LTR, event.Key, event.Modifiers) {
			return ui.EventResult{}
		}
	} else {
		next, handled := components.SliderKeyDown(values[0], 0, 100, 10, components.LTR, event.Key, event.Modifiers)
		if !handled {
			return ui.EventResult{}
		}
		values[0] = next
	}
	nextValue := int32(values[index] + 0.5)
	changed := state.values[index] != nextValue
	state.values[index] = nextValue
	if state.exampleID == "range" {
		thumbs := &state.bag().thumbs
		thumbs[index].Value = float32(nextValue)
		thumbs[index].ValueText = sliderDemoValueText(nextValue)
	}
	return ui.EventResult{Handled: true, Repaint: changed}
}

// sliderForExample is the interactive slider as the last paint placed it, for the
// pointer to turn a coordinate into a value.
func (state *State) sliderForExample() components.Slider {
	placed := state.bag().placed
	if state.exampleID == "range" {
		return components.BeginSlider(nil, ui.Theme{}, sliderLower, placed, float32(state.values[0])).Step(5)
	}
	slider := components.BeginSlider(nil, ui.Theme{}, sliderPrimary, placed, float32(state.values[0])).Step(10)
	if state.exampleID == "states" {
		slider = slider.Vertical()
	}
	return slider
}

func (state *State) paintSlider(canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
	var track, disabled flex.Box
	state.page(canvas, theme, size, 520, state.Text("Slider · 显式数值", "Slider · Explicit values"), func(f *flex.Frame) {
		switch state.exampleID {
		case "range":
			track = f.Slot().Height(16)
			f.Text(state.Text("下限 / 上限", "Lower / upper bound")).FontSize(theme.Metrics.FontSizeM)
			caption(f, state.Text("分别激活两个圆形滑块，演示调用方更新范围。", "Activate each round thumb to see the caller update the range."))
		case "states":
			f.Row().Gap(40).Children(func() {
				f.Column().Gap(10).Items(flex.Center).Children(func() {
					track = f.Slot().Size(24, 160)
					f.Text(state.Text("竖向", "Vertical")).FontSize(theme.Metrics.FontSizeM)
				})
				f.Column().Grow(1).Basis(0).Gap(10).Children(func() {
					disabled = f.Slot().Height(16)
					f.Text(state.Text("禁用", "Disabled")).FontSize(theme.Metrics.FontSizeM).Muted()
				})
			})
		default:
			track = f.Slot().Height(16)
			caption(f, state.Text("拖动滑块按十为步长更新；激活仍可循环。", "Dragging steps by ten; activating cycles the caller's value."))
		}
	})

	bounds := track.Rect()
	if bag := state.slider; bag != nil {
		bag.placed = bounds
	}
	switch state.exampleID {
	case "range":
		components.BeginRangeSlider(canvas, theme, bounds, state.bag().thumbs[:]).Step(5).Paint(state.Text("价格区间", "Price range"))
		if canvas.Clicked(sliderLower) || canvas.Clicked(sliderUpper) {
			state.toggleSliderRangePreset()
		}
		return
	case "states":
		components.BeginSlider(canvas, theme, sliderPrimary, bounds, float32(state.values[0])).Vertical().Step(10).Paint(state.Text("音量", "Volume"))
		components.BeginSlider(canvas, theme, sliderDisabled, disabled.Rect(), 64).Disabled().Paint(state.Text("禁用滑块", "Disabled slider"))
	default:
		components.BeginSlider(canvas, theme, sliderPrimary, bounds, float32(state.values[0])).Step(10).ValueText(sliderDemoValueText(state.values[0])).Paint(state.Text("音量", "Volume"))
	}
	if canvas.Clicked(sliderPrimary) {
		state.values[0] = (state.values[0] + 10) % 110
	}
}

func sliderDemoValueText(value int32) string {
	values := [...]string{
		"0", "5", "10", "15", "20", "25", "30", "35", "40", "45", "50",
		"55", "60", "65", "70", "75", "80", "85", "90", "95", "100",
	}
	if value < 0 || value > 100 || value%5 != 0 {
		return "custom value"
	}
	return values[value/5]
}
