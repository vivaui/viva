package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var reorderDemoHandles = tree.Sub(tree.Feature("reorder-list"), tree.Name("handle"))

// reorderGripIcon is Lucide's grip-vertical: six dots.
var reorderGripIcon = lucide(
	gfx.MoveTo(9, 5), gfx.LineTo(9.01, 5), gfx.MoveTo(9, 12), gfx.LineTo(9.01, 12), gfx.MoveTo(9, 19), gfx.LineTo(9.01, 19),
	gfx.MoveTo(15, 5), gfx.LineTo(15.01, 5), gfx.MoveTo(15, 12), gfx.LineTo(15.01, 12), gfx.MoveTo(15, 19), gfx.LineTo(15.01, 19))

type reorderDemoState struct {
	list      ui.ListState
	order     [4]int
	labels    [4]string
	hitLabels [4]string
	rowFn     func(ui.ListRow)
}

// key names a row by the item it shows, so focus and state follow the item as
// the rows are reordered.
func (demo *reorderDemoState) key(row int) int { return demo.order[row] }

// label is row i's text, from the item the row shows.
func (demo *reorderDemoState) label(row int) string { return demo.labels[demo.order[row]] }

// move puts rows, ascending, before row to, as the List's Reorder reports them.
func (demo *reorderDemoState) move(rows []int, to int) {
	if len(rows) == 0 {
		return
	}
	var moving, kept [4]int
	nm, nk := 0, 0
	for index, item := range demo.order {
		moved := false
		for _, row := range rows {
			if row == index {
				moved = true
				break
			}
		}
		if moved {
			moving[nm], nm = item, nm+1
		} else {
			kept[nk], nk = item, nk+1
		}
	}
	to = min(max(to, 0), nk)
	var next [4]int
	copy(next[:to], kept[:to])
	copy(next[to:], moving[:nm])
	copy(next[to+nm:], kept[to:nk])
	demo.order = next
}

// row declares one item: its grip starts the drag and its name fills the row.
func (demo *reorderDemoState) row(row ui.ListRow) {
	item := demo.order[row.Index]
	label := demo.labels[item]
	row.Element.Height(48).PadX(12).Gap(12).Items(ui.Center).Children(func() {
		ui.Icon(row.C, reorderGripIcon).Size(16, 16).
			Hit(reorderDemoHandles.At(item), demo.hitLabels[item])
		ui.Text(row.C, label).Grow(1)
	})
}

var reorderListComponent = Component{
	ID: "reorder-list", Title: "Reorder List", Category: "Layout", Summary: "拖动手柄或键盘排序，列表数据由调用方持有。",
	initialize: func(state *State) {
		demo := &reorderDemoState{order: [4]int{0, 1, 2, 3}}
		demo.labels = [4]string{
			state.Text("主机", "Hosts"),
			state.Text("文件传输", "File transfer"),
			state.Text("AI 助手", "AI assistant"),
			state.Text("设置", "Settings"),
		}
		for index, label := range demo.labels {
			demo.hitLabels[index] = state.Text("重新排序 ", "Reorder ") + label
		}
		demo.list.Key = demo.key
		demo.list.Label = demo.label
		demo.list.Reorder = demo.move
		demo.list.Handle = reorderDemoHandles
		demo.rowFn = demo.row
		state.reorder = demo
	},
	paint: func(state *State, canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
		state.drawPage(canvas, theme, size, 440, state.Text("Reorder List · 入口顺序", "Reorder List · Shortcut order"), func(c ui.C) {
			ui.List(c, &state.reorder.list, len(state.reorder.order), state.reorder.rowFn).
				Label(state.Text("快捷入口", "Shortcuts")).Height(192)
		})
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
_ = result`, PageSnippet: `m.list.Key = func(i int) int { return i }
m.list.Label = func(i int) string { return m.items[i] }
m.list.Reorder = m.move
m.list.Handle = m.handles
ui.List(c, &m.list, len(m.items), func(row ui.ListRow) {
    row.Element.Height(48).PadX(12).Gap(12).Items(ui.Center).Children(func() {
        ui.Icon(row.C, m.gripIcon).Size(16, 16).
            Hit(m.handles.At(row.Index), "Reorder "+m.items[row.Index])
        ui.Text(row.C, m.items[row.Index]).Grow(1)
    })
}).Label("Shortcuts")`, PageSnippetZh: `m.list.Key = func(i int) int { return i }
m.list.Label = func(i int) string { return m.items[i] }
m.list.Reorder = m.move
m.list.Handle = m.handles
ui.List(c, &m.list, len(m.items), func(row ui.ListRow) {
    row.Element.Height(48).PadX(12).Gap(12).Items(ui.Center).Children(func() {
        ui.Icon(row.C, m.gripIcon).Size(16, 16).
            Hit(m.handles.At(row.Index), "重新排序 "+m.items[row.Index])
        ui.Text(row.C, m.items[row.Index]).Grow(1)
    })
}).Label("快捷入口")`}},
}
