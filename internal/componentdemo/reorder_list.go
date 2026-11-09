package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var reorderDemoHandles = tree.Sub(tree.Feature("reorder-list"), tree.Name("handle"))

type reorderDemoState struct {
	list  components.ReorderListState
	order [4]int
}

func (state *reorderDemoState) move(move components.ReorderMove) {
	if !move.Valid() || move.From >= len(state.order) || move.To >= len(state.order) {
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

var reorderListComponent = Component{
	ID: "reorder-list", Title: "Reorder List", Category: "Layout", Summary: "拖动手柄或键盘排序，列表数据由调用方持有。",
	initialize: func(state *State) { state.reorder = &reorderDemoState{order: [4]int{0, 1, 2, 3}} },
	paint: func(state *State, canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
		var slot ui.Box
		state.page(canvas, theme, size, 440, state.Text("Reorder List · 入口顺序", "Reorder List · Shortcut order"), func(f *ui.Frame) { slot = f.Slot().FillWidth().Height(188) })
		list := components.BeginReorderList(canvas, theme, slot.Rect(), &state.reorder.list)
		labels := [4]string{state.Text("主机", "Hosts"), state.Text("文件传输", "File transfer"), state.Text("AI 助手", "AI assistant"), state.Text("设置", "Settings")}
		for _, index := range state.reorder.order {
			row := list.Row(reorderDemoHandles.At(index), labels[index], false)
			canvas.Label(labels[index], row.Content, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
		}
		list.End()
	},
	pointer: func(state *State, event gfx.PointerEvent) gfx.EventResult {
		move, result := state.reorder.list.Pointer(event)
		state.reorder.move(move)
		return result
	},
	key: func(state *State, event gfx.KeyEvent) gfx.EventResult {
		move, result := state.reorder.list.Key(event)
		state.reorder.move(move)
		return result
	},
	Examples: []Example{{ID: "default", Title: "入口排序", Description: "只有左侧手柄启动拖动；方向键也可移动条目，Escape 取消尚未放下的拖动。", Height: 280, Snippet: `list := components.BeginReorderList(canvas, theme, bounds, &dragState)
for _, item := range items {
    row := list.Row(item.HandleID, item.Name, false)
    canvas.Label(item.Name, row.Content, textStyle)
}
list.End()

// Route pointer events outside paint. Apply each completed move once.
move, result := dragState.Pointer(event)
if move.Valid() {
    applyMove(move.From, move.To)
}
// Keyboard events can use dragState.Key(event) in the same way.
_ = result`}},
}
