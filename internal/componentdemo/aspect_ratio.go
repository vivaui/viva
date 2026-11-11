package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var aspectRatioComponent = Component{
	ID: "aspect-ratio", Title: "Aspect Ratio", Category: "Layout", Summary: "在不同容器宽度下保持稳定内容比例。",
	paint: (*State).paintAspectRatio,
	Examples: []Example{
		{ID: "default", Title: "16:9 预览", Description: "通过生产布局函数计算媒体区域高度。", Snippet: `ratio := float32(16.0 / 9.0)
width := min(size.Width-48, float32(520))
height := components.AspectRatio(gfx.Rect{Width: width}, ratio).Height
media := ui.Slot(c).Size(width, height)
bounds := components.AspectRatio(media.Rect(), ratio)
canvas.Rounded(bounds, theme.Metrics.RadiusSurface, theme.Muted)`},
		{ID: "square", Title: "方形封面", Description: "同一 API 使用 1:1 比例。", Snippet: `width := min(size.Width-48, float32(250))
height := components.AspectRatio(gfx.Rect{Width: width}, 1).Height
media := ui.Slot(c).Size(width, height)
bounds := components.AspectRatio(media.Rect(), 1)
canvas.Rounded(bounds, theme.Metrics.RadiusSurface, theme.Muted)`},
	},
}

func (state *State) paintAspectRatio(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	maxWidth := float32(520)
	ratio := float32(16.0 / 9.0)
	title, heading := state.Text("16:9 发布预览", "16:9 release preview"), state.Text("Aspect Ratio · 16:9 发布预览", "Aspect Ratio · 16:9 release preview")
	if state.exampleID == "square" {
		maxWidth, ratio = 250, 1
		title, heading = state.Text("1:1 封面", "1:1 cover"), state.Text("Aspect Ratio · 1:1 封面", "Aspect Ratio · 1:1 cover")
	}
	// The height follows from the width the page gives the slot: the width is
	// the page's (the viewport less its padding, up to maxWidth) and
	// components.AspectRatio turns it into the media rectangle.
	width := min(size.Width-48, maxWidth)
	height := min(components.AspectRatio(gfx.Rect{Width: width}, ratio).Height, size.Height-92)
	var media ui.Box
	state.drawPage(canvas, theme, size, maxWidth, heading, func(c ui.C) {
		media = ui.Slot(c).Size(width, height)
	})
	bounds := components.AspectRatio(media.Rect(), ratio)
	canvas.Rounded(bounds, theme.Metrics.RadiusSurface, theme.Muted)
	canvas.LabelCenter(title, bounds, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.MutedForeground})
}
