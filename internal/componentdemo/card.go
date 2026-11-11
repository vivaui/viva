package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var cardComponent = Component{
	ID: "card", Title: "Card", Category: "Layout", Summary: "调用方布局内容、生产组件提供统一表面与间距。",
	paint: (*State).paintCard,
	Examples: []Example{
		{ID: "default", Title: "项目摘要", Description: "默认卡片承载标题、指标和页脚。", Snippet: `size := components.CardSizeDefault
ring := theme.Foreground
ring.Alpha *= 0.10
ui.Column(c).FillWidth().Pad(components.CardSpacing(theme, size)).Gap(8).
    Background(theme.Card).Border(1, ring).
    Radius(theme.Metrics.RadiusSurface + 4).Shadow(gfx.ShadowXS).Children(func() {
        ui.Text(c, "Native preview").FontSize(theme.Metrics.FontSizeM + 2).
            Weight(gfx.WeightSemibold)
        ui.Text(c, "0").FontSize(28).Bold()
        ui.Divider(c)
        ui.Text(c, "Measured locally").FontSize(theme.Metrics.FontSizeS).Muted()
    })`},
		{ID: "compact", Title: "紧凑卡片", Description: "小尺寸使用生产 CardSpacing 缩减内边距。", Snippet: `size := components.CardSizeSmall
ring := theme.Foreground
ring.Alpha *= 0.10
ui.Column(c).FillWidth().Pad(components.CardSpacing(theme, size)).Gap(8).
    Background(theme.Card).Border(1, ring).
    Radius(theme.Metrics.RadiusSurface + 4).Shadow(gfx.ShadowXS).Children(func() {
        ui.Text(c, "Native preview").FontSize(theme.Metrics.FontSizeM).
            Weight(gfx.WeightSemibold)
        ui.Text(c, "Measured locally").FontSize(theme.Metrics.FontSizeS).Muted()
    })`},
	},
}

func (state *State) paintCard(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	cardSize := components.CardSizeDefault
	if state.exampleID == "compact" {
		cardSize = components.CardSizeSmall
	}
	// The card is a ui column dressed as components.PaintCard draws a card: the
	// same padding, radius and hairline ring.
	ring := theme.Foreground
	ring.Alpha *= 0.10
	state.drawPage(canvas, theme, size, 520, state.Text("Card · 项目摘要", "Card · Project summary"), func(c ui.C) {
		ui.Column(c).FillWidth().Pad(components.CardSpacing(theme, cardSize)).Gap(8).
			Background(theme.Card).Border(1, ring).Radius(theme.Metrics.RadiusSurface + 4).Shadow(gfx.ShadowXS).Children(func() {
			ui.Text(c, "Native preview").FontSize(theme.Metrics.FontSizeM + 2).Weight(gfx.WeightSemibold).Color(theme.CardForeground)
			ui.Text(c, state.Text("保留场景节点，只在内容变化时重绘。", "Keeps scene nodes and repaints only when content changes.")).FontSize(theme.Metrics.FontSizeS).Muted()
			ui.Text(c, "0").FontSize(28).Bold().Color(theme.Foreground)
			ui.Text(c, "idle GPU submissions").FontSize(theme.Metrics.FontSizeS).Muted()
			ui.Divider(c)
			ui.Text(c, "Measured locally").FontSize(theme.Metrics.FontSizeS).Muted()
		})
	})
}
