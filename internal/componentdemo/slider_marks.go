package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var sliderMarksComponent = Component{
	ID: "slider-marks", Title: "Slider Marks", Category: "Forms", Summary: "滑块轨道上的刻度点与文字标记。",
	Source: "base/slider.go", paint: (*State).paintSliderMarks,
	Examples: []Example{
		{ID: "default", Title: "带标记的滑块", Description: "Marks 在轨道上给每个值画一个点，并把文字放在下面；带文字的横向滑块会相应增高。", Snippet: `// m.marks is a []ui.SliderMark kept in the model.
ui.Slider(c, &m.quality).Range(0, 100).Step(25).Marks(m.marks)`, PageSnippet: `// m.marks is a []ui.SliderMark kept in the model.
ui.Slider(c, &m.quality).Range(0, 100).Step(25).Marks(m.marks)`, PageSnippetZh: `// m.marks 是保存在模型里的 []ui.SliderMark。
ui.Slider(c, &m.quality).Range(0, 100).Step(25).Marks(m.marks)`},
	},
}

type sliderMarksDemo struct {
	marks []ui.SliderMark
}

var sliderMarksDemos = map[*State]*sliderMarksDemo{}

func sliderMarksDemoFor(state *State) *sliderMarksDemo {
	if demo := sliderMarksDemos[state]; demo != nil {
		return demo
	}
	demo := &sliderMarksDemo{marks: []ui.SliderMark{
		{Value: 0, Label: state.Text("低", "Low")}, {Value: 25}, {Value: 50, Label: state.Text("中", "Medium")}, {Value: 75},
		{Value: 100, Label: state.Text("高", "High")},
	}}
	sliderMarksDemos[state] = demo
	return demo
}

func (state *State) paintSliderMarks(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := sliderMarksDemoFor(state)
	state.drawPage(canvas, theme, size, 480, state.Text("Slider · 标记", "Slider · Marks"), func(c ui.C) {
		quality := ui.State[ui.SliderState](c, "quality")
		if quality.Value == 0 && state.flags&1 == 0 {
			quality.Value, state.flags = 50, state.flags|1
		}
		ui.Slider(c, quality).Range(0, 100).Step(25).Marks(demo.marks).Label(state.Text("画质", "Quality")).FillWidth()
	})
}
