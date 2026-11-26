package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/card"
)

var cardComponent = Component{
	ID: "card", Title: "Card", Category: "Layout", Summary: "调用方布局内容、生产组件提供统一表面与间距。",
	PageSource: "ui/card/card.go", paint: (*State).paintCard,
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
    })`, PageSnippet: `card.View(c, card.Spec{
    Title:       "Native preview",
    Description: "Keeps scene nodes and repaints only when content changes.",
    FooterBorder: true,
    Footer:      func() { ui.Text(c, "Measured locally").Muted() },
}, func() {
    ui.Text(c, "0").FontSize(28).Bold()
    ui.Text(c, "idle GPU submissions").Muted()
}).FillWidth()`, PageSnippetZh: `card.View(c, card.Spec{
    Title:       "Native preview",
    Description: "Keeps scene nodes and repaints only when content changes.",
    FooterBorder: true,
    Footer:      func() { ui.Text(c, "Measured locally").Muted() },
}, func() {
    ui.Text(c, "0").FontSize(28).Bold()
    ui.Text(c, "idle GPU submissions").Muted()
}).FillWidth()`},
		{ID: "compact", Title: "紧凑卡片", Description: "Size: Small 缩减内边距与间距。", Snippet: `size := components.CardSizeSmall
ring := theme.Foreground
ring.Alpha *= 0.10
ui.Column(c).FillWidth().Pad(components.CardSpacing(theme, size)).Gap(8).
    Background(theme.Card).Border(1, ring).
    Radius(theme.Metrics.RadiusSurface + 4).Shadow(gfx.ShadowXS).Children(func() {
        ui.Text(c, "Native preview").FontSize(theme.Metrics.FontSizeM).
            Weight(gfx.WeightSemibold)
        ui.Text(c, "Measured locally").FontSize(theme.Metrics.FontSizeS).Muted()
    })`, PageSnippet: `card.View(c, card.Spec{
    Title:       "Native preview",
    Description: "Keeps scene nodes and repaints only when content changes.",
    Size:        card.Small,
    FooterBorder: true,
    Footer:      func() { ui.Text(c, "Measured locally").Muted() },
}, func() {
    ui.Text(c, "0").FontSize(28).Bold()
    ui.Text(c, "idle GPU submissions").Muted()
}).FillWidth()`, PageSnippetZh: `card.View(c, card.Spec{
    Title:       "Native preview",
    Description: "Keeps scene nodes and repaints only when content changes.",
    Size:        card.Small,
    FooterBorder: true,
    Footer:      func() { ui.Text(c, "Measured locally").Muted() },
}, func() {
    ui.Text(c, "0").FontSize(28).Bold()
    ui.Text(c, "idle GPU submissions").Muted()
}).FillWidth()`},
	},
}

// cardDemo keeps the closures the card calls, made once: a closure made in each
// paint allocates in each paint.
type cardDemo struct {
	c      ui.C
	theme  *gfx.Theme
	footer func()
	body   func()
}

var cardDemos = map[*State]*cardDemo{}

func cardDemoFor(state *State) *cardDemo {
	if demo := cardDemos[state]; demo != nil {
		return demo
	}
	demo := &cardDemo{}
	demo.footer = func() { ui.Text(demo.c, "Measured locally").FontSize(demo.theme.Metrics.FontSizeS).Muted() }
	demo.body = func() {
		ui.Text(demo.c, "0").FontSize(28).Bold()
		ui.Text(demo.c, "idle GPU submissions").FontSize(demo.theme.Metrics.FontSizeS).Muted()
	}
	cardDemos[state] = demo
	return demo
}

func (state *State) paintCard(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := cardDemoFor(state)
	spec := card.Spec{
		Title:        "Native preview",
		Description:  state.Text("保留场景节点，只在内容变化时重绘。", "Keeps scene nodes and repaints only when content changes."),
		FooterBorder: true, Footer: demo.footer,
	}
	if state.exampleID == "compact" {
		spec.Size = card.Small
	}
	state.drawPage(canvas, theme, size, 520, state.Text("Card · 项目摘要", "Card · Project summary"), func(c ui.C) {
		demo.c, demo.theme = c, c.Theme()
		card.View(c, spec, demo.body).FillWidth()
	})
}
