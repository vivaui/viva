package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var separatorComponent = Component{
	ID: "separator", Title: "Separator", Category: "Layout", Summary: "水平和垂直内容分组分隔线。",
	paint: (*State).paintSeparator,
	Examples: []Example{
		{ID: "horizontal", Title: "水平分组", Description: "在设置行之间绘制一像素生产分隔线。", Snippet: `rows := [...]string{"Profile", "Notifications", "Appearance"}
for index, label := range rows {
    ui.Text(c, label).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
    if index < len(rows)-1 {
        ui.Divider(c)
    }
}`},
		{ID: "vertical", Title: "垂直分组", Description: "在紧凑指标之间使用垂直分隔线。", Snippet: `labels := [...]string{"12 projects", "4 members", "99.9% uptime"}
ui.Row(c).Height(96).Background(theme.Card).Border(1, theme.Border).
    Radius(theme.Metrics.RadiusSurface).Children(func() {
        for index, label := range labels {
            ui.Column(c).Grow(1).Basis(0).Center().Children(func() {
                ui.Text(c, label).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
            })
            if index < len(labels)-1 {
                ui.Divider(c).MarginY(20)
            }
        }
    })`},
	},
}

func (state *State) paintSeparator(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 560, "Workspace settings", func(c ui.C) {
		if state.exampleID == "vertical" {
			labels := [...]string{"12 projects", "4 members", "99.9% uptime"}
			ui.Row(c).Height(96).Background(theme.Card).Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface).Children(func() {
				for index, label := range labels {
					ui.Column(c).Grow(1).Basis(0).Center().Children(func() {
						ui.Text(c, label).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
					})
					if index < len(labels)-1 {
						ui.Divider(c).MarginY(20)
					}
				}
			})
		} else {
			rows := [...]string{"Profile", "Notifications", "Appearance"}
			for index, label := range rows {
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
				if index < len(rows)-1 {
					ui.Divider(c)
				}
			}
		}
		caption(c, state.Text("仅绘制结构边界。", "Draws structural boundaries only."))
	})
}
