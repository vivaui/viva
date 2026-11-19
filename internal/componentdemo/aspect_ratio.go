package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/aspectratio"
)

var aspectRatioComponent = Component{
	ID: "aspect-ratio", Title: "Aspect Ratio", Category: "Layout", Summary: "在不同容器宽度下保持稳定内容比例。",
	paint: (*State).paintAspectRatio,
	Examples: []Example{
		{ID: "default", Title: "16:9 预览", Description: "页面盒子按调用方给出的宽度推导高度，内容仍是应用的页面子节点。", Snippet: `ratio := float32(16.0 / 9.0)
width := min(size.Width-48, float32(480))
height := components.AspectRatio(gfx.Rect{Width: width}, ratio).Height
media := ui.Slot(c).Size(width, height)
bounds := components.AspectRatio(media.Rect(), ratio)
canvas.Rounded(bounds, theme.Metrics.RadiusSurface, theme.Muted)`, PageSnippet: `m.media.Ratio(16, 9)
aspectratio.View(c, "16:9 release preview", m.media, func() {
    ui.Column(c).Center().Radius(c.Theme().Metrics.RadiusSurface).
        Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "16:9 release preview").Muted()
    })
}).FillWidth().MaxWidth(480)`, PageSnippetZh: `m.media.Ratio(16, 9)
aspectratio.View(c, "16:9 发布预览", m.media, func() {
    ui.Column(c).Center().Radius(c.Theme().Metrics.RadiusSurface).
        Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "16:9 发布预览").Muted()
    })
}).FillWidth().MaxWidth(480)`},
		{ID: "square", Title: "方形封面", Description: "同一 API 改用 1:1 比例；没有为某种比例单独设构造函数。", Snippet: `width := min(size.Width-48, float32(240))
height := components.AspectRatio(gfx.Rect{Width: width}, 1).Height
media := ui.Slot(c).Size(width, height)
bounds := components.AspectRatio(media.Rect(), 1)
canvas.Rounded(bounds, theme.Metrics.RadiusSurface, theme.Muted)`, PageSnippet: `m.media.Ratio(1, 1)
aspectratio.View(c, "1:1 cover", m.media, func() {
    ui.Column(c).Center().Radius(c.Theme().Metrics.RadiusSurface).
        Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "1:1 cover").Muted()
    })
}).FillWidth().MaxWidth(240)`, PageSnippetZh: `m.media.Ratio(1, 1)
aspectratio.View(c, "1:1 封面", m.media, func() {
    ui.Column(c).Center().Radius(c.Theme().Metrics.RadiusSurface).
        Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "1:1 封面").Muted()
    })
}).FillWidth().MaxWidth(240)`},
	},
}

// aspectRatioDemo is one example's ratio box. It is kept beside the State
// rather than rebuilt in each paint, so the layout is reused and a warm paint
// allocates nothing.
type aspectRatioDemo struct {
	box aspectratio.Box
}

// aspectRatioDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var aspectRatioDemos = map[*State]*aspectRatioDemo{}

func aspectRatioDemoFor(state *State) *aspectRatioDemo {
	if demo := aspectRatioDemos[state]; demo != nil {
		return demo
	}
	demo := &aspectRatioDemo{box: *aspectratio.New()}
	aspectRatioDemos[state] = demo
	return demo
}

func (state *State) paintAspectRatio(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := aspectRatioDemoFor(state)
	ratioWidth, ratioHeight, maxWidth := float32(16), float32(9), float32(480)
	title := state.Text("16:9 发布预览", "16:9 release preview")
	if state.exampleID == "square" {
		ratioWidth, ratioHeight, maxWidth = 1, 1, 240
		title = state.Text("1:1 封面", "1:1 cover")
	}
	demo.box.Ratio(ratioWidth, ratioHeight)
	state.drawPage(canvas, theme, size, 520, title, func(c ui.C) {
		aspectratio.View(c, title, &demo.box, func() {
			ui.Column(c).Center().
				Radius(theme.Metrics.RadiusSurface).
				Background(theme.Muted).
				Children(func() {
					ui.Text(c, title).Muted()
				})
		}).FillWidth().MaxWidth(maxWidth)
	})
}
