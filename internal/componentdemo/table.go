package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var tableComponent = Component{
	ID: "table", Title: "Table", Category: "Data", Summary: "紧凑、可选择的原生数据表格。",
	paint: (*State).paintTable,
	Examples: []Example{
		{ID: "default", Title: "账单列表", Description: "表头、三行数据、页脚与说明文字。", Snippet: `rows := tree.Sub(page, tree.Name("row"))
table := components.BeginTable(canvas, theme, bounds, columns)
table.Header([]string{"Item", "Status", "Amount"})
for index, cells := range bills {
    table.Row(rows.At(index), cells[:], false)
}
table.Footer([]string{"Total", "3 items", "¥ 4,540"})
table.Caption("Click any data row to see its interactive feedback.")`},
		{ID: "selection", Title: "选择状态", Description: "点击稳定行 ID 切换调用方持有的选择位。", Snippet: `rows := tree.Sub(page, tree.Name("row"))
table := components.BeginTable(canvas, theme, bounds, columns)
table.Header([]string{"Item", "Status", "Amount"})
for index, cells := range bills {
    selected := flags&(1<<index) != 0
    if table.RowAt(rows, index, cells[:], selected) {
        flags ^= 1 << index
    }
}
table.Footer([]string{"Total", "3 items", "¥ 4,540"})`},
		{ID: "alignment", Title: "列对齐", Description: "左、中、右三种列内文字对齐。", Snippet: `columns := []components.TableColumn{
    {Width: 112, Alignment: components.TableAlignLeft},
    {Width: 88, Alignment: components.TableAlignCenter},
    {Alignment: components.TableAlignRight},
}
table := components.BeginTable(canvas, theme, bounds, columns)
table.Header([]string{"Item", "Status", "Amount"})
for _, cells := range bills {
    table.Row(gfx.ID{}, cells[:], false)
}
table.Caption("Item is left-aligned, status centered, amount right-aligned.")`},
		{ID: "overflow", Title: "溢出与表头", Description: "放不下的单元格以省略号结尾；FitTableHeadings 让定宽列至少放得下自己的表头。", Snippet: `columns := []components.TableColumn{{Width: 96}, {Width: 40}, {Alignment: components.TableAlignRight}}
headings := []string{"Item", "Status", "Amount"}
components.FitTableHeadings(canvas, theme, columns, headings)
table := components.BeginTable(canvas, theme, bounds, columns)
table.Header(headings)
for _, cells := range bills {
    table.Row(gfx.ID{}, cells[:], false)
}
table.Caption("Item is cut with an ellipsis; Status grew to hold its heading.")`},
	},
}

var (
	tableUI = tree.Feature("table")
	// tableRow's item i is the i-th data row.
	tableRow = tree.Sub(tableUI, tree.Name("row"))
)

var tableColumns = [...]components.TableColumn{
	{Width: 112, Alignment: components.TableAlignLeft},
	{Width: 88, Alignment: components.TableAlignCenter},
	{Width: 0, Alignment: components.TableAlignRight},
}

// The demo paints the component table (with footer and caption, and a selection
// bit per row the caller owns) in a slot: ui.Table is the selectable, sortable,
// scrolling one.
func (state *State) paintTable(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var slot ui.Box
	state.page(canvas, theme, size, 620, state.Text("Table · 账单", "Table · Bills"), func(f *ui.Frame) {
		slot = f.Slot().Height(220)
	})
	columns := tableColumns
	header := [...]string{
		state.Text("项目", "Item"),
		state.Text("状态", "Status"),
		state.Text("金额", "Amount"),
	}
	if state.exampleID == "overflow" {
		columns[0].Width, columns[1].Width = 96, 40
		components.FitTableHeadings(canvas, theme, columns[:], header[:])
	}
	table := components.BeginTable(canvas, theme, slot.Rect(), columns[:])
	table.Header(header[:])
	rows := [...][3]string{
		{state.Text("设计系统", "Design system"), state.Text("已支付", "Paid"), "¥ 2,400"},
		{state.Text("桌面构建", "Desktop build"), state.Text("处理中", "Processing"), "¥ 860"},
		{state.Text("性能审计", "Performance audit"), state.Text("待付款", "Unpaid"), "¥ 1,280"},
	}
	for index := range rows {
		selected := state.exampleID == "selection" && state.flags&(1<<index) != 0
		if table.RowAt(tableRow, index, rows[index][:], selected) {
			state.flags ^= 1 << index
		}
	}
	footer := [...]string{
		state.Text("合计", "Total"),
		state.Text("3 项", "3 items"),
		"¥ 4,540",
	}
	table.Footer(footer[:])
	caption := state.Text("点击任意数据行以查看交互反馈。", "Click any data row to see its interactive feedback.")
	switch state.exampleID {
	case "alignment":
		caption = state.Text("项目左对齐，状态居中，金额右对齐。", "Item is left-aligned, status centered, amount right-aligned.")
	case "overflow":
		caption = state.Text("项目被省略号截断；状态列已长到放得下表头。", "Item is cut with an ellipsis; Status grew to hold its heading.")
	}
	table.Caption(caption)
}
