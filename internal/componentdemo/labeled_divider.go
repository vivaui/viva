package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/divider"
)

var labeledDividerComponent = Component{
	ID: "labeled-divider", Title: "Labeled Divider", Category: "Layout", Summary: "带一个词的分隔线：居中，或靠近一端。",
	Source: "ui/divider/divider.go", paint: (*State).paintLabeledDivider,
	Examples: []Example{
		{ID: "default", Title: "带文字的分隔线", Description: "分隔线中间或靠近一端带一个词，如两种登录方式之间的“或”。", Snippet: `divider.View(c, "or", divider.Spec{})
divider.View(c, "Advanced", divider.Spec{Align: divider.Start})`, PageSnippet: `divider.View(c, "or", divider.Spec{})
divider.View(c, "Advanced", divider.Spec{Align: divider.Start})`, PageSnippetZh: `divider.View(c, "or", divider.Spec{})
divider.View(c, "Advanced", divider.Spec{Align: divider.Start})`},
	},
}

func (state *State) paintLabeledDivider(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 480, state.Text("Divider · 带文字", "Divider · Labeled"), func(c ui.C) {
		ui.Text(c, state.Text("使用密钥登录", "Sign in with a key")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
		divider.View(c, state.Text("或", "or"), divider.Spec{})
		ui.Text(c, state.Text("使用密码登录", "Sign in with a password")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
		divider.View(c, state.Text("高级", "Advanced"), divider.Spec{Align: divider.Start})
		ui.Text(c, state.Text("端口与代理", "Port and proxy")).FontSize(theme.Metrics.FontSizeM)
	})
}
