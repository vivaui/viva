package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/reordergrid"
)

var reorderGridDemoHandles = tree.Sub(tree.Feature("reorder-grid"), tree.Name("handle"))

type reorderGridDemoState struct {
	state *State
	view  reordergrid.Grid
	order [6]int
}

func (demo *reorderGridDemoState) move(move reordergrid.Move) {
	if !move.Valid() || move.From < 0 || move.From >= len(demo.order) || move.To < 0 || move.To >= len(demo.order) {
		return
	}
	value := demo.order[move.From]
	if move.From < move.To {
		copy(demo.order[move.From:move.To], demo.order[move.From+1:move.To+1])
	} else {
		copy(demo.order[move.To+1:move.From+1], demo.order[move.To:move.From])
	}
	demo.order[move.To] = value
}

var reorderGridComponent = Component{
	ID: "reorder-grid", Title: "Reorder Grid", Category: "Layout", Summary: "在调用方定位的卡片或行中，用拖动手柄和键盘调整顺序。",
	initialize: func(state *State) {
		demo := &reorderGridDemoState{state: state, order: [6]int{0, 1, 2, 3, 4, 5}}
		demo.view.HandleHitSlop = 8
		demo.view.Draw = demo.draw
		demo.view.Move = demo.move
		state.reorderGrid = demo
	},
	paint: (*State).paintReorderGrid,
	Examples: []Example{{
		ID: "default", Title: "卡片排序",
		Description: "调用方提供每个项目的位置与逻辑顺序；手柄负责拖动，方向键、Home / End 也可排序。",
		Height:      320,
		Snippet: "grid := components.BeginReorderGrid(canvas, theme, viewport, &dragState)\n" +
			"for order, item := range items {\n" +
			"    grid.Order(item.HandleID, item.Lane, order, item.Disabled)\n" +
			"}\n" +
			"for visibleIndex, item := range visibleItems {\n" +
			"    order := visibleStart + visibleIndex\n" +
			"    grid.Item(item.HandleID, item.Name, item.Lane, order, item.Bounds, item.Handle, item.Disabled)\n" +
			"}\n" +
			"grid.End()\n\n" +
			"move, result := dragState.Pointer(event)\n" +
			"if move.Valid() {\n" +
			"    applyMove(move.From, move.To)\n" +
			"}\n" +
			"// Keyboard events use dragState.Key(event) in the same way.\n" +
			"_ = result",
		PageSnippet: `// m.reorderGrid is a reordergrid.Grid.
grid := m.reorderGrid
grid.HandleHitSlop = 8
grid.Draw = m.drawReorderGrid
grid.Move = m.applyReorderMove
grid.View(c, "Host order").FillWidth().Height(220)
// Pointer, key and wheel callbacks are optional observers; the wrapper routes
// reorder input to State either way.`,
		PageSnippetZh: `// m.reorderGrid 是 reordergrid.Grid。
grid := m.reorderGrid
grid.HandleHitSlop = 8
grid.Draw = m.drawReorderGrid
grid.Move = m.applyReorderMove
grid.View(c, "主机排序").FillWidth().Height(220)
// Pointer、按键和滚轮回调是可选的观察者；重排输入始终由包装路由到 State。`,
	}},
}

func (state *State) paintReorderGrid(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.reorderGrid
	state.drawPage(canvas, theme, size, 620, state.Text("Reorder Grid · 卡片顺序", "Reorder Grid · Card order"), func(c ui.C) {
		reordergrid.View(c, state.Text("主机排序", "Host order"), &demo.view).FillWidth().Height(220)
	})
}

// draw paints the caller's cards and declares each one through the layout. The
// layout reports the dragged and target items with the caller's own identities.
func (demo *reorderGridDemoState) draw(ctx *ui.DrawContext, layout reordergrid.Layout) {
	labels := [6]string{
		demo.state.Text("主机一", "Host one"),
		demo.state.Text("主机二", "Host two"),
		demo.state.Text("主机三", "Host three"),
		demo.state.Text("主机四", "Host four"),
		demo.state.Text("主机五", "Host five"),
		demo.state.Text("主机六", "Host six"),
	}
	const (
		columns = 3
		gap     = float32(10)
		height  = float32(92)
	)
	for order, index := range demo.order {
		layout.Order(reorderGridDemoHandles.At(index), 0, order, false)
	}
	for order, index := range demo.order {
		id := reorderGridDemoHandles.At(index)
		card := layout.ItemBounds(columns, order, gap, height)
		edge := ctx.Theme.Border
		if layout.Target() == id && layout.Active() != id {
			edge = ctx.Theme.Primary
		}
		if layout.Dragging() && layout.Active() == id {
			ctx.Canvas.Elevate(card, ctx.Theme.Metrics.RadiusControl, gfx.ShadowMD)
			edge = ctx.Theme.Primary
		}
		ctx.Canvas.RoundedBorder(card, ctx.Theme.Metrics.RadiusControl, 1, edge, ctx.Theme.Background)
		handle := gfx.Rect{X: card.X + 8, Y: card.Y + 12, Width: 28, Height: card.Height - 24}
		content := gfx.Rect{X: card.X + 44, Y: card.Y, Width: max(card.Width-52, 0), Height: card.Height}
		layout.Item(id, labels[index], 0, order, card, handle, false)
		ctx.Canvas.Label(labels[index], content, gfx.TextStyle{Size: ctx.Theme.Metrics.FontSizeM, Color: ctx.Theme.Foreground})
	}
}
