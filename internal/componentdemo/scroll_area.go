package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var scrollAreaComponent = Component{
	ID: "scroll-area", Title: "Scroll Area", Category: "Layout", Summary: "调用方提供偏移量的裁剪内容与原生滚动条几何。",
	paint: (*State).paintScrollArea, scroll: (*State).scrollScrollArea,
	Examples: []Example{
		{ID: "vertical", Title: "纵向内容", Description: "目标区域滚动更新调用方偏移，点击滚动条仍可循环。", Snippet: `contentSize := gfx.Size{Width: viewport.Width, Height: 420}
area := components.BeginScrollArea(canvas, theme, viewport, contentSize).
    Offset(0, offset).
    VerticalScrollbar(tree.ID(page, "vertical"), "Scroll activity", "Scrolled")
geometry := area.BeginContent()
canvas.Label("Window created", geometry.Content,
    gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
area.EndContent()`},
		{ID: "both-axes", Title: "双轴内容", Description: "typed 滚动同时更新水平、垂直偏移和角落几何。", Snippet: `contentSize := gfx.Size{Width: viewport.Width + 180, Height: 420}
area := components.BeginScrollArea(canvas, theme, viewport, contentSize).
    Offset(offsetX, offsetY).
    VerticalScrollbar(tree.ID(page, "vertical"), "Scroll activity", "Scrolled").
    HorizontalScrollbar(tree.ID(page, "horizontal"), "Scroll columns", "Scrolled")
geometry := area.BeginContent()
canvas.Label("Window created", geometry.Content,
    gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
area.EndContent()`},
	},
}

var (
	scrollAreaUI         = tree.Feature("scroll-area")
	scrollAreaVertical   = tree.ID(scrollAreaUI, "vertical")
	scrollAreaHorizontal = tree.ID(scrollAreaUI, "horizontal")
	scrollAreaViewport   = tree.ID(scrollAreaUI, "viewport")
)

func (state *State) scrollScrollArea(event gfx.ScrollEvent) gfx.EventResult {
	if event.Target != scrollAreaViewport && event.Target != scrollAreaVertical && event.Target != scrollAreaHorizontal {
		return gfx.EventResult{}
	}
	previousX, previousY := state.values[1], state.values[0]
	state.values[0] = clampInt32(state.values[0]+int32(event.DeltaY), 0, 180)
	if state.exampleID == "both-axes" {
		state.values[1] = clampInt32(state.values[1]+int32(event.DeltaX), 0, 180)
	}
	return gfx.EventResult{Handled: true, Repaint: previousX != state.values[1] || previousY != state.values[0]}
}

func clampInt32(value, minimum, maximum int32) int32 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func (state *State) paintScrollArea(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var stage ui.Box
	state.page(canvas, theme, size, 560, "Recent activity", func(f *ui.Frame) {
		stage = f.Slot().Height(220)
		caption(f, state.Text("偏移量由应用持有。", "The offset is caller-owned."))
	})

	viewport := stage.Rect()
	contentWidth := viewport.Width
	if state.exampleID == "both-axes" {
		contentWidth += 180
	}
	contentHeight := float32(420)
	area := components.BeginScrollArea(canvas, theme, viewport, gfx.Size{Width: contentWidth, Height: contentHeight}).
		Offset(scrollAreaHorizontalOffset(state.values[1]), scrollAreaVerticalOffset(state.values[0])).
		VerticalScrollbar(scrollAreaVertical, "Scroll activity", scrollAreaVerticalValue(state.values[0]))
	if state.exampleID == "both-axes" {
		area = area.HorizontalScrollbar(scrollAreaHorizontal, "Scroll columns", scrollAreaHorizontalValue(state.values[1]))
	}
	geometry := area.BeginContent()
	canvas.PushFocusScope(scrollAreaViewport)
	canvas.HitWithOptions(scrollAreaViewport, geometry.Viewport, "Scrollable activity", gfx.HitOptions{SkipFocus: true})
	canvas.PopFocusScope()

	canvas.RoundedBorder(geometry.Bounds, 6, 1, theme.Border, theme.Card)
	for index := 0; index < 8; index++ {
		y := geometry.Content.Y + float32(index)*50 + 10
		if y+38 <= geometry.Viewport.Y || y >= geometry.Viewport.Y+geometry.Viewport.Height {
			continue
		}
		row := gfx.Rect{X: geometry.Viewport.X + 12, Y: max(y, geometry.Viewport.Y), Width: max(geometry.Viewport.Width-36, 0), Height: min(y+38, geometry.Viewport.Y+geometry.Viewport.Height) - max(y, geometry.Viewport.Y)}
		canvas.Rounded(row, 5, theme.CardElevated)
		canvas.Label(scrollAreaRowLabel(index), gfx.Rect{X: row.X + 12, Y: row.Y, Width: max(row.Width-24, 0), Height: row.Height}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground,
		})
	}
	area.EndContent()

	// A click on a scrollbar steps its offset.
	if canvas.Clicked(scrollAreaVertical) {
		state.values[0] += 72
		if state.values[0] > 216 {
			state.values[0] = 0
		}
	}
	if canvas.Clicked(scrollAreaHorizontal) {
		state.values[1] += 72
		if state.values[1] > 144 {
			state.values[1] = 0
		}
	}
}

func scrollAreaVerticalOffset(value int32) float32 {
	return float32(value)
}

func scrollAreaHorizontalOffset(value int32) float32 {
	return float32(value)
}

func scrollAreaVerticalValue(value int32) string {
	switch value {
	case 72:
		return "72 pixels"
	case 144:
		return "144 pixels"
	case 216:
		return "216 pixels"
	default:
		return "Scrolled"
	}
}

func scrollAreaHorizontalValue(value int32) string {
	switch value {
	case 72:
		return "72 pixels"
	case 144:
		return "144 pixels"
	default:
		return "Scrolled"
	}
}

func scrollAreaRowLabel(index int) string {
	switch index {
	case 0:
		return "Window created"
	case 1:
		return "Scene retained"
	case 2:
		return "Layout cache hit"
	case 3:
		return "Text shaped"
	case 4:
		return "Input routed"
	case 5:
		return "Frame submitted"
	case 6:
		return "GPU idle"
	default:
		return "Resources released"
	}
}
