package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var reorderGridDemoHandles = tree.Sub(tree.Feature("reorder-grid"), tree.Name("handle"))

type reorderGridDemoState struct {
	grid  components.ReorderGridState
	order [6]int
}

func (state *reorderGridDemoState) move(move components.ReorderGridMove) {
	if !move.Valid() || move.From < 0 || move.From >= len(state.order) || move.To < 0 || move.To >= len(state.order) {
		return
	}
	value := state.order[move.From]
	if move.From < move.To {
		copy(state.order[move.From:move.To], state.order[move.From+1:move.To+1])
	} else {
		copy(state.order[move.To+1:move.From+1], state.order[move.To:move.From])
	}
	state.order[move.To] = value
}

var reorderGridComponent = Component{
	ID: "reorder-grid", Title: "Reorder Grid", Category: "Layout", Summary: "在调用方定位的卡片或行中，用拖动手柄和键盘调整顺序。",
	initialize: func(state *State) {
		state.reorderGrid = &reorderGridDemoState{order: [6]int{0, 1, 2, 3, 4, 5}}
	},
	paint: func(state *State, canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
		var slot flex.Box
		state.page(canvas, theme, size, 620, state.Text("Reorder Grid · 卡片顺序", "Reorder Grid · Card order"), func(f *flex.Frame) {
			slot = f.Slot().FillWidth().Height(220)
		})
		bounds := slot.Rect()
		const (
			columns = 3
			gap     = float32(10)
			height  = float32(92)
		)
		width := max((bounds.Width-gap*(columns-1))/columns, 0)
		grid := components.BeginReorderGrid(canvas, theme, bounds, &state.reorderGrid.grid).HandleHitSlop(8)
		labels := [6]string{
			state.Text("主机一", "Host one"),
			state.Text("主机二", "Host two"),
			state.Text("主机三", "Host three"),
			state.Text("主机四", "Host four"),
			state.Text("主机五", "Host five"),
			state.Text("主机六", "Host six"),
		}
		for order, index := range state.reorderGrid.order {
			grid.Order(reorderGridDemoHandles.At(index), 0, order, false)
		}
		for order, index := range state.reorderGrid.order {
			id := reorderGridDemoHandles.At(index)
			column, row := order%columns, order/columns
			card := ui.Rect{
				X:     bounds.X + float32(column)*(width+gap),
				Y:     bounds.Y + float32(row)*(height+gap),
				Width: width, Height: height,
			}
			edge := theme.Border
			if state.reorderGrid.grid.Target() == id && state.reorderGrid.grid.Active() != id {
				edge = theme.Primary
			}
			if state.reorderGrid.grid.Dragging() && state.reorderGrid.grid.Active() == id {
				canvas.Elevate(card, theme.Metrics.RadiusControl, ui.ShadowMD)
				edge = theme.Primary
			}
			canvas.RoundedBorder(card, theme.Metrics.RadiusControl, 1, edge, theme.Background)
			handle := ui.Rect{X: card.X + 8, Y: card.Y + 12, Width: 28, Height: card.Height - 24}
			content := ui.Rect{X: card.X + 44, Y: card.Y, Width: max(card.Width-52, 0), Height: card.Height}
			grid.Item(id, labels[index], 0, order, card, handle, false)
			canvas.Label(labels[index], content, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
		}
		grid.End()
	},
	pointer: func(state *State, event ui.PointerEvent) ui.EventResult {
		move, result := state.reorderGrid.grid.Pointer(event)
		state.reorderGrid.move(move)
		return result
	},
	key: func(state *State, event ui.KeyEvent) ui.EventResult {
		move, result := state.reorderGrid.grid.Key(event)
		state.reorderGrid.move(move)
		return result
	},
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
	}},
}
