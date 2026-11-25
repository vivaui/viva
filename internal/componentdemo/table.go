package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
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
table.Caption("Click any data row to see its interactive feedback.")`, PageSnippet: `bills := ui.TableRows{
    {"Design system", "Paid", "¥ 2,400"},
    {"Desktop build", "Processing", "¥ 860"},
    {"Performance audit", "Unpaid", "¥ 1,280"},
}
columns := []style.TableColumn{
    {Title: "Item", Width: 112, Alignment: style.TableAlignLeft},
    {Title: "Status", Width: 88, Alignment: style.TableAlignCenter},
    {Title: "Amount", Width: 0, Alignment: style.TableAlignRight},
}
ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Source:  &bills,
})
ui.Text(c, "Total · 3 items · ¥ 4,540").Muted()
ui.Text(c, "Click any data row to see its interactive feedback.").Muted()`, PageSnippetZh: `bills := ui.TableRows{
    {"设计系统", "已支付", "¥ 2,400"},
    {"桌面构建", "处理中", "¥ 860"},
    {"性能审计", "待付款", "¥ 1,280"},
}
columns := []style.TableColumn{
    {Title: "项目", Width: 112, Alignment: style.TableAlignLeft},
    {Title: "状态", Width: 88, Alignment: style.TableAlignCenter},
    {Title: "金额", Width: 0, Alignment: style.TableAlignRight},
}
ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Source:  &bills,
})
ui.Text(c, "合计 · 3 项 · ¥ 4,540").Muted()
ui.Text(c, "点击任意数据行以查看交互反馈。").Muted()`},
		{ID: "selection", Title: "选择状态", Description: "点击稳定行 ID 切换调用方持有的选择位。", Snippet: `rows := tree.Sub(page, tree.Name("row"))
table := components.BeginTable(canvas, theme, bounds, columns)
table.Header([]string{"Item", "Status", "Amount"})
for index, cells := range bills {
    selected := flags&(1<<index) != 0
    if table.RowAt(rows, index, cells[:], selected) {
        flags ^= 1 << index
    }
}
table.Footer([]string{"Total", "3 items", "¥ 4,540"})`, PageSnippet: `bills := ui.TableRows{
    {"Design system", "Paid", "¥ 2,400"},
    {"Desktop build", "Processing", "¥ 860"},
    {"Performance audit", "Unpaid", "¥ 1,280"},
}
columns := []style.TableColumn{
    {Width: 112, Alignment: style.TableAlignLeft},
    {Width: 88, Alignment: style.TableAlignCenter},
    {Width: 0, Alignment: style.TableAlignRight},
}
table := ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Headers: []string{"Item", "Status", "Amount"},
    Source:  &bills,
})
if row, ok := table.RowClicked(); ok {
    m.table.Select(row)
}
if row := m.table.Selected(); row >= 0 {
    ui.Text(c, "Selected: "+bills[row][0]).Muted()
}
ui.Text(c, "Total · 3 items · ¥ 4,540").Muted()`, PageSnippetZh: `bills := ui.TableRows{
    {"设计系统", "已支付", "¥ 2,400"},
    {"桌面构建", "处理中", "¥ 860"},
    {"性能审计", "待付款", "¥ 1,280"},
}
columns := []style.TableColumn{
    {Width: 112, Alignment: style.TableAlignLeft},
    {Width: 88, Alignment: style.TableAlignCenter},
    {Width: 0, Alignment: style.TableAlignRight},
}
table := ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Headers: []string{"项目", "状态", "金额"},
    Source:  &bills,
})
if row, ok := table.RowClicked(); ok {
    m.table.Select(row)
}
if row := m.table.Selected(); row >= 0 {
    ui.Text(c, "Selected: "+bills[row][0]).Muted()
}
ui.Text(c, "合计 · 3 项 · ¥ 4,540").Muted()`},
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
table.Caption("Item is left-aligned, status centered, amount right-aligned.")`, PageSnippet: `bills := ui.TableRows{
    {"Design system", "Paid", "¥ 2,400"},
    {"Desktop build", "Processing", "¥ 860"},
    {"Performance audit", "Unpaid", "¥ 1,280"},
}
columns := []style.TableColumn{
    {Title: "Item", Width: 112, Alignment: style.TableAlignLeft},
    {Title: "Status", Width: 88, Alignment: style.TableAlignCenter},
    {Title: "Amount", Alignment: style.TableAlignRight},
}
ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Source:  &bills,
})
ui.Text(c, "Item is left-aligned, status centered, amount right-aligned.").Muted()`, PageSnippetZh: `bills := ui.TableRows{
    {"设计系统", "已支付", "¥ 2,400"},
    {"桌面构建", "处理中", "¥ 860"},
    {"性能审计", "待付款", "¥ 1,280"},
}
columns := []style.TableColumn{
    {Title: "项目", Width: 112, Alignment: style.TableAlignLeft},
    {Title: "状态", Width: 88, Alignment: style.TableAlignCenter},
    {Title: "金额", Alignment: style.TableAlignRight},
}
ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Source:  &bills,
})
ui.Text(c, "项目左对齐，状态居中，金额右对齐。").Muted()`},
		{ID: "checked", Title: "勾选列", Description: "Checked 在最左加一列复选框，表头复选框全选或清空，部分勾选时为半选；勾选与选中行互不影响，列索引仍是 Columns 的。", Snippet: `// m.checked is a ui.Selection kept in the model: the rows ticked.
ui.Table(c, &m.table, ui.TableSpec{
    Columns: m.columns,
    Source:  &m.bills,
    Checked: &m.checked,
})
if m.table.CheckedChanged() {
    // the user ticked or unticked a row, or the header's checkbox
}
// m.checked.Has(row) says whether a row is ticked; m.checked.Len() how many.`, PageSnippet: `// m.checked is a ui.Selection kept in the model: the rows ticked.
ui.Table(c, &m.table, ui.TableSpec{
    Columns: m.columns,
    Source:  &m.bills,
    Checked: &m.checked,
})
if m.table.CheckedChanged() {
    // the user ticked or unticked a row, or the header's checkbox
}
// m.checked.Has(row) says whether a row is ticked; m.checked.Len() how many.`, PageSnippetZh: `// m.checked is a ui.Selection kept in the model: the rows ticked.
ui.Table(c, &m.table, ui.TableSpec{
    Columns: m.columns,
    Source:  &m.bills,
    Checked: &m.checked,
})
if m.table.CheckedChanged() {
    // the user ticked or unticked a row, or the header's checkbox
}
// m.checked.Has(row) says whether a row is ticked; m.checked.Len() how many.`},
		{ID: "overflow", Title: "溢出与表头", Description: "放不下的单元格以省略号结尾；FitTableHeadings 让定宽列至少放得下自己的表头。", Snippet: `columns := []components.TableColumn{{Width: 96}, {Width: 40}, {Alignment: components.TableAlignRight}}
headings := []string{"Item", "Status", "Amount"}
components.FitTableHeadings(canvas, theme, columns, headings)
table := components.BeginTable(canvas, theme, bounds, columns)
table.Header(headings)
for _, cells := range bills {
    table.Row(gfx.ID{}, cells[:], false)
}
table.Caption("Item is cut with an ellipsis; Status grew to hold its heading.")`, PageSnippet: `bills := ui.TableRows{
    {"Design system", "Paid", "¥ 2,400"},
    {"Desktop build", "Processing", "¥ 860"},
    {"Performance audit", "Unpaid", "¥ 1,280"},
}
columns := []style.TableColumn{{Width: 96}, {Width: 40}, {Alignment: style.TableAlignRight}}
ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Headers: []string{"Item", "Status", "Amount"},
    FitHeadings: true,
    Source:  &bills,
})
ui.Text(c, "Item is cut with an ellipsis; Status grew to hold its heading.").Muted()`, PageSnippetZh: `bills := ui.TableRows{
    {"设计系统", "已支付", "¥ 2,400"},
    {"桌面构建", "处理中", "¥ 860"},
    {"性能审计", "待付款", "¥ 1,280"},
}
columns := []style.TableColumn{{Width: 96}, {Width: 40}, {Alignment: style.TableAlignRight}}
ui.Table(c, &m.table, ui.TableSpec{
    Columns: columns,
    Headers: []string{"项目", "状态", "金额"},
    FitHeadings: true,
    Source:  &bills,
})
ui.Text(c, "项目被省略号截断；状态列已长到放得下表头。").Muted()`},
	},
}

// tableDemo keeps the page Table's selection, row scope and the data the table
// reads each paint. The slices live here because the page retains the spec, so a
// warm paint allocates nothing.
type tableDemo struct {
	state   ui.TableState
	checked ui.Selection
	rows    [3][3]string
	source  ui.TableRows
	columns []style.TableColumn
	footer  string
	caption string
}

var tableDemos = map[*State]*tableDemo{}

func tableDemoFor(state *State) *tableDemo {
	if demo := tableDemos[state]; demo != nil {
		return demo
	}
	demo := &tableDemo{
		rows: [3][3]string{
			{state.Text("设计系统", "Design system"), state.Text("已支付", "Paid"), "¥ 2,400"},
			{state.Text("桌面构建", "Desktop build"), state.Text("处理中", "Processing"), "¥ 860"},
			{state.Text("性能审计", "Performance audit"), state.Text("待付款", "Unpaid"), "¥ 1,280"},
		},
		columns: []style.TableColumn{
			{Title: state.Text("项目", "Item"), Width: 112, Alignment: style.TableAlignLeft},
			{Title: state.Text("状态", "Status"), Width: 88, Alignment: style.TableAlignCenter},
			{Title: state.Text("金额", "Amount"), Width: 0, Alignment: style.TableAlignRight},
		},
		footer:  state.Text("合计 · 3 项 · ¥ 4,540", "Total · 3 items · ¥ 4,540"),
		caption: state.Text("点击任意数据行以查看交互反馈。", "Click any data row to see its interactive feedback."),
	}
	demo.source = ui.TableRows{demo.rows[0][:], demo.rows[1][:], demo.rows[2][:]}
	switch state.exampleID {
	case "alignment":
		demo.caption = state.Text("项目左对齐，状态居中，金额右对齐。", "Item is left-aligned, status centered, amount right-aligned.")
	case "overflow":
		// The narrow columns are what the heading fit widens: Status is 40
		// wide as declared and grows to hold its heading.
		demo.columns[0].Width, demo.columns[1].Width = 96, 40
		demo.columns[1].Alignment = style.TableAlignLeft
		demo.caption = state.Text("项目被省略号截断；状态列已长到放得下表头。", "Item is cut with an ellipsis; Status grew to hold its heading.")
	}
	tableDemos[state] = demo
	return demo
}

// paintTable paints the page Table with its footer, caption and the selection
// bit the caller owns. The overflow example asks the same table to fit its
// headings, so fixed columns grow to hold their localized titles.
func (state *State) paintTable(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := tableDemoFor(state)
	state.drawPage(canvas, theme, size, 620, state.Text("Table · 账单", "Table · Bills"), func(c ui.C) {
		spec := ui.TableSpec{
			Columns: demo.columns,
			Source:  &demo.source,
		}
		if state.exampleID == "overflow" {
			spec.FitHeadings = true
		}
		if state.exampleID == "checked" {
			spec.Checked = &demo.checked
			spec.SelectAllLabel, spec.SelectRowLabel = state.Text("全选", "Select all rows"), state.Text("选择此行", "Select row")
		}
		table := ui.Table(c, &demo.state, spec)
		if state.exampleID == "selection" {
			if row, ok := table.RowClicked(); ok {
				demo.state.Select(row)
			}
			state.flags = 0
			if selected := demo.state.Selected(); selected >= 0 {
				state.flags = 1 << selected
				ui.Text(c, "Selected: "+demo.rows[selected][0]).Muted()
			}
			ui.Text(c, demo.footer).Muted()
			return
		}
		ui.Text(c, demo.footer).Muted()
		ui.Text(c, demo.caption).Muted()
	})
}
