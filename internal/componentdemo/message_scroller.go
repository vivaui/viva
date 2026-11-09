package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var messageScrollerComponent = Component{
	ID: "message-scroller", Title: "Message Scroller", Category: "Communication", Summary: "外部控制偏移的消息视口、滚动条与跳转按钮。",
	paint: (*State).paintMessageScroller,
	Examples: []Example{
		{ID: "default", Title: "消息流", Description: "点击上下按钮改变调用方持有的滚动偏移。", Snippet: `earlier, later := tree.ID(page, "earlier"), tree.ID(page, "later")
scroller := components.BeginMessageScroller(canvas, theme, viewport, 380, offset)
scroller = scroller.Buttons(earlier, "Earlier messages", later, "Later messages")
scroller.BeginContent()
for index := range 5 {
    item := scroller.ItemGeometry(index, float32(index)*74, 62)
    if item.Visible { canvas.Fill(item.Bounds, theme.Muted) }
}
scroller.EndContent()
if canvas.Clicked(earlier) { offset = max(0, offset-72) }
if canvas.Clicked(later) { offset = min(144, offset+72) }`},
		{ID: "end", Title: "接近末尾", Description: "展示偏移、滑块和可用方向变化。", Snippet: `bar := tree.ID(page, "scrollbar")
earlier, later := tree.ID(page, "earlier"), tree.ID(page, "later")
scroller := components.BeginMessageScroller(canvas, theme, viewport, 380, offset)
scroller = scroller.Scrollbar(bar, "Message position", "Conversation")
scroller = scroller.Buttons(earlier, "Earlier messages", later, "Later messages")
geometry := scroller.BeginContent()
item := scroller.ItemGeometry(4, 296, 62)
if item.Visible { canvas.Fill(item.Bounds, theme.Muted) }
scroller.EndContent()
if geometry.CanStart && canvas.Clicked(earlier) { offset = max(0, offset-72) }
if geometry.CanEnd && canvas.Clicked(later) { offset = min(144, offset+72) }`},
	},
}

var (
	messageScrollerUI    = tree.Feature("message-scroller")
	messageScrollerBar   = tree.ID(messageScrollerUI, "scrollbar")
	messageScrollerStart = tree.ID(messageScrollerUI, "earlier")
	messageScrollerEnd   = tree.ID(messageScrollerUI, "later")
)

func (state *State) paintMessageScroller(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var viewport ui.Box
	state.page(canvas, theme, size, 560, state.Text("Message Scroller · 受控偏移", "Message Scroller · Controlled offset"), func(f *ui.Frame) {
		viewport = f.Slot().FillWidth().Height(min(238, max(size.Height-84, 120)))
	})
	offset := float32(state.values[0])
	if state.exampleID == "end" && state.values[0] == 0 {
		offset = 132
	}
	scroller := components.BeginMessageScroller(canvas, theme, viewport.Rect(), 380, offset).Scrollbar(messageScrollerBar, "Message position", "Conversation").Buttons(messageScrollerStart, "Earlier messages", messageScrollerEnd, "Later messages")
	geometry := scroller.BeginContent()
	for index := range 5 {
		item := scroller.ItemGeometry(index, float32(index)*74, 62)
		if !item.Visible || item.Bounds.Y < geometry.Viewport.Y || item.Bounds.Y+item.Bounds.Height > geometry.Viewport.Y+geometry.Viewport.Height {
			continue
		}
		canvas.Rounded(item.Bounds, theme.Metrics.RadiusSurface, theme.Muted)
		canvas.Label([...]string{"Project started", "Build complete", "Review requested", "Changes approved", "Release ready"}[index], gfx.Rect{X: item.Bounds.X + 12, Y: item.Bounds.Y, Width: max(0, item.Bounds.Width-24), Height: item.Bounds.Height}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
	}
	scroller.EndContent()
	if canvas.Clicked(messageScrollerStart) {
		state.values[0] = max(0, state.values[0]-72)
	}
	if canvas.Clicked(messageScrollerEnd) {
		state.values[0] = min(144, state.values[0]+72)
	}
}
