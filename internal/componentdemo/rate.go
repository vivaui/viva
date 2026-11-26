package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/rate"
)

var rateComponent = Component{
	ID: "rate", Title: "Rate", Category: "Forms", Summary: "星级评分：点击选择，可半星、可清除，只读时仅展示。",
	Source: "ui/rate/rate.go", paint: (*State).paintRate,
	Examples: []Example{
		{ID: "default", Title: "星级评分", Description: "点击选择，再点已选的星清除；悬停预览，方向键调整。", Snippet: `m.rate = rate.New()
// in the page:
m.rate.View(c, "Rating", &m.score)
if m.rate.Changed() {
    // m.score already holds the new score
}`, PageSnippet: `m.rate = rate.New()
// in the page:
m.rate.View(c, "Rating", &m.score)
if m.rate.Changed() {
    // m.score already holds the new score
}`, PageSnippetZh: `m.rate = rate.New()
// 在页面中：
m.rate.View(c, "Rating", &m.score)
if m.rate.Changed() {
    // m.score 已经是新的分数
}`},
		{ID: "half", Title: "半星与只读", Description: "AllowHalf 支持半星；ReadOnly 只展示分数。", Snippet: `m.rate = rate.New().AllowHalf().Size(28)
m.average = rate.New().ReadOnly().Count(10).Size(16)`, PageSnippet: `m.rate = rate.New().AllowHalf().Size(28)
m.average = rate.New().ReadOnly().Count(10).Size(16)`, PageSnippetZh: `m.rate = rate.New().AllowHalf().Size(28)
m.average = rate.New().ReadOnly().Count(10).Size(16)`},
	},
}

type rateDemo struct {
	basic, half, readOnly *rate.Rate
	a, b, c               float32
}

var rateDemos = map[*State]*rateDemo{}

func rateDemoFor(state *State) *rateDemo {
	if demo := rateDemos[state]; demo != nil {
		return demo
	}
	demo := &rateDemo{
		basic: rate.New(), half: rate.New().AllowHalf().Size(28), readOnly: rate.New().ReadOnly().Count(10).Size(16),
		a: 3, b: 3.5, c: 8,
	}
	rateDemos[state] = demo
	return demo
}

func (state *State) paintRate(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := rateDemoFor(state)
	state.drawPage(canvas, theme, size, 420, state.Text("Rate · 评分", "Rate · Rating"), func(c ui.C) {
		if state.exampleID == "half" {
			demo.half.View(c, state.Text("半星评分", "Half star rating"), &demo.b)
			demo.readOnly.View(c, state.Text("平均分", "Average"), &demo.c)
			return
		}
		demo.basic.View(c, state.Text("评分", "Rating"), &demo.a)
	})
}
