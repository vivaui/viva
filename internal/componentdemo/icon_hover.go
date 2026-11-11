package componentdemo

import (
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var iconHoverComponent = Component{
	ID: "icon-hover", Title: "Icon Hover", Category: "Actions", Summary: "图标停留后播放一次手势，不移动布局或命中区域。",
	paint: (*State).paintIconHover,
	Examples: []Example{
		{ID: "default", Title: "悬停手势", Description: "指针在图标上停留后上浮再归位；离开不会中断已经开始的播放。", Snippet: `icon := tree.ID(page, "icon")
ui.Icon(c, menuIcon).Size(24, 24).Hit(icon, "Menu").
    BindIconHover(ui.BindIconHover(&hover, icon))`, Height: 200},
	},
}

var (
	iconHoverUI     = tree.Feature("icon-hover")
	iconHoverTarget = tree.ID(iconHoverUI, "target")
)

func (state *State) paintIconHover(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 480, state.Text("Icon Hover · 停留后播放一次", "Icon Hover · Play once after a dwell"), func(c ui.C) {
		ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
			ui.Icon(c, moreIcon).Size(24, 24).Color(theme.Foreground).Hit(iconHoverTarget, state.Text("更多操作", "More actions")).
				BindIconHover(ui.BindIconHover(&state.iconHover, iconHoverTarget).Dwell(130 * time.Millisecond).Span(180 * time.Millisecond))
			ui.Text(c, state.Text("悬停 → 上浮 → 归位", "Hover → lift → settle")).FontSize(theme.Metrics.FontSizeM)
		})
		caption(c, state.Text("动画只移动图标；文字标签、布局和点击区域保持不动。", "Motion moves only the glyph; its label, layout and hit region stay put."))
	})
}
