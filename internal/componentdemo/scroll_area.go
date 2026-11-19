package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var scrollAreaComponent = Component{
	ID: "scroll-area", Title: "Scroll Area", Category: "Layout", Summary: "调用方提供偏移量的裁剪内容与原生滚动条几何。",
	paint: (*State).paintScrollArea,
	Examples: []Example{
		{ID: "vertical", Title: "纵向内容", Description: "目标区域滚动更新调用方偏移，点击滚动条仍可循环。", Snippet: `contentSize := gfx.Size{Width: viewport.Width, Height: 420}
area := components.BeginScrollArea(canvas, theme, viewport, contentSize).
    Offset(0, offset).
    VerticalScrollbar(tree.ID(page, "vertical"), "Scroll activity", "Scrolled")
geometry := area.BeginContent()
canvas.Label("Window created", geometry.Content,
    gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
area.EndContent()`, PageSnippet: `ui.Scroll(c).Height(220).Scrollbars().Label("Scrollable activity").Children(func() {
    for _, label := range [...]string{
        "Window created", "Scene retained", "Layout cache hit", "Text shaped",
        "Input routed", "Frame submitted", "GPU idle", "Resources released",
    } {
        ui.Row(c).Height(50).PadX(12).Items(ui.Center).Children(func() {
            ui.Text(c, label)
        })
    }
})`, PageSnippetZh: `ui.Scroll(c).Height(220).Scrollbars().Label("Scrollable activity").Children(func() {
    for _, label := range [...]string{
        "Window created", "Scene retained", "Layout cache hit", "Text shaped",
        "Input routed", "Frame submitted", "GPU idle", "Resources released",
    } {
        ui.Row(c).Height(50).PadX(12).Items(ui.Center).Children(func() {
            ui.Text(c, label)
        })
    }
})`},
		{ID: "both-axes", Title: "双轴内容", Description: "typed 滚动同时更新水平、垂直偏移和角落几何。", Snippet: `contentSize := gfx.Size{Width: viewport.Width + 180, Height: 420}
area := components.BeginScrollArea(canvas, theme, viewport, contentSize).
    Offset(offsetX, offsetY).
    VerticalScrollbar(tree.ID(page, "vertical"), "Scroll activity", "Scrolled").
    HorizontalScrollbar(tree.ID(page, "horizontal"), "Scroll columns", "Scrolled")
geometry := area.BeginContent()
canvas.Label("Window created", geometry.Content,
    gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
area.EndContent()`, PageSnippet: `ui.Scroll(c).Height(220).Sideways().Scrollbars().Label("Scrollable activity").Children(func() {
    for _, label := range [...]string{
        "Window created", "Scene retained", "Layout cache hit", "Text shaped",
        "Input routed", "Frame submitted", "GPU idle", "Resources released",
    } {
        ui.Row(c).Width(c.Bounds().Width + 180).Height(50).PadX(12).Items(ui.Center).Children(func() {
            ui.Text(c, label)
        })
    }
})`, PageSnippetZh: `ui.Scroll(c).Height(220).Sideways().Scrollbars().Label("Scrollable activity").Children(func() {
    for _, label := range [...]string{
        "Window created", "Scene retained", "Layout cache hit", "Text shaped",
        "Input routed", "Frame submitted", "GPU idle", "Resources released",
    } {
        ui.Row(c).Width(c.Bounds().Width + 180).Height(50).PadX(12).Items(ui.Center).Children(func() {
            ui.Text(c, label)
        })
    }
})`},
	},
}

var (
	scrollAreaUI         = tree.Feature("scroll-area")
	scrollAreaVertical   = tree.ID(scrollAreaUI, "vertical")
	scrollAreaHorizontal = tree.ID(scrollAreaUI, "horizontal")
	scrollAreaViewport   = tree.ID(scrollAreaUI, "viewport")
)

// scrollAreaDemo remembers the page scroll area's state and its last bounds for
// the keyboard/wheel tests; the page owns the offsets themselves.
type scrollAreaDemo struct {
	scroller *ui.ScrollState
	bounds   gfx.Rect
}

var scrollAreaDemos = map[*State]*scrollAreaDemo{}

func scrollAreaDemoFor(state *State) *scrollAreaDemo {
	if demo := scrollAreaDemos[state]; demo != nil {
		return demo
	}
	demo := &scrollAreaDemo{}
	scrollAreaDemos[state] = demo
	return demo
}

func (state *State) paintScrollArea(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	both := state.exampleID == "both-axes"
	labels := [8]string{
		"Window created", "Scene retained", "Layout cache hit", "Text shaped",
		"Input routed", "Frame submitted", "GPU idle", "Resources released",
	}
	demo := scrollAreaDemoFor(state)
	var area ui.Box
	state.drawPage(canvas, theme, size, 560, "Recent activity", func(c ui.C) {
		scroll := ui.Scroll(c).Height(220).Scrollbars().Hit(scrollAreaViewport, "Scrollable activity").ScrollbarIDs(scrollAreaVertical, scrollAreaHorizontal)
		if both {
			scroll = scroll.Sideways()
		}
		area = scroll
		demo.scroller = scroll.ScrollState()
		scroll.Children(func() {
			for _, label := range labels {
				row := ui.Row(c).Height(50).PadX(12).Items(ui.Center)
				if both {
					row = row.Width(c.Bounds().Width + 180)
				}
				row.Children(func() {
					ui.Text(c, label)
				})
			}
		})
	})
	demo.bounds = area.Rect()
	if demo.scroller != nil {
		state.values[0] = int32(demo.scroller.Offset)
		state.values[1] = int32(demo.scroller.OffsetX)
	}
}
