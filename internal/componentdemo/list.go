package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// listDemoItem is one row of the list demo: a section heading, or a file with the
// columns a file browser shows.
type listDemoItem struct {
	id     int
	name   string
	size   string
	date   string
	header bool
}

// listDemo is a page-layer List: sections whose headings stay at the top, a
// multi-selection, rows that can be dragged to another place, and rows wider than
// the list that scroll sideways under a column heading.
type listDemo struct {
	list      ui.ListState
	selection ui.Selection
	rows      []listDemoItem
	english   bool
	page      *ui.Page
	theme     gfx.Theme
	dark      bool
	density   gfx.Density
	viewFn    func(ui.C)
	rowFn     func(ui.ListRow)
	pageDraw  func(*ui.DrawContext)
}

func newListDemo(english bool) *listDemo {
	d := &listDemo{english: english}
	recent, documents, images := "Recent", "Documents", "Images"
	if !english {
		recent, documents, images = "最近", "文档", "图片"
	}
	d.rows = []listDemoItem{
		{header: true, name: recent},
		{name: "Report.pdf", size: "2.4 MB", date: "2026-11-02 14:20"},
		{name: "Notes.md", size: "12 KB", date: "2026-11-02 09:05"},
		{name: "Budget.xlsx", size: "88 KB", date: "2026-11-01 18:44"},
		{name: "Screenshot.png", size: "1.1 MB", date: "2026-11-01 11:30"},
		{header: true, name: documents},
		{name: "Contract.pdf", size: "640 KB", date: "2026-10-31 16:12"},
		{name: "Roadmap.md", size: "8 KB", date: "2026-10-30 10:02"},
		{name: "Style guide.pdf", size: "3.8 MB", date: "2026-10-29 15:48"},
		{name: "Meeting notes.md", size: "24 KB", date: "2026-10-28 08:19"},
		{header: true, name: images},
		{name: "Logo.svg", size: "36 KB", date: "2026-10-27 13:07"},
		{name: "Hero.jpg", size: "4.2 MB", date: "2026-10-26 19:26"},
		{name: "Icon set.zip", size: "2.9 MB", date: "2026-10-25 07:41"},
		{name: "Palette.png", size: "310 KB", date: "2026-10-24 12:55"},
	}
	for i := range d.rows {
		d.rows[i].id = i + 1
	}
	d.list.Key = func(i int) int { return d.rows[i].id }
	d.list.Label = func(i int) string { return d.rows[i].name }
	d.list.Header = func(i int) bool { return d.rows[i].header }
	d.list.Selection = &d.selection
	d.list.Reorder = d.reorder
	d.list.RowWidth = 640
	d.viewFn, d.rowFn = d.view, d.row
	d.pageDraw = d.drawPage
	return d
}

func (d *listDemo) text(chinese, english string) string {
	if d.english {
		return english
	}
	return chinese
}

// page gives the demo's page its theme and density, made once for each.
func (d *listDemo) pageFor(state *State, theme gfx.Theme) *ui.Page {
	if d.page == nil || d.dark != state.dark || d.density != state.density {
		d.dark, d.density, d.theme = state.dark, state.density, theme
		d.page = ui.NewPage(ui.PageConfig{View: d.viewFn, Theme: &d.theme, Root: "list-demo"})
	}
	return d.page
}

// view is the page: a column heading that follows the rows sideways, the list, and
// what the list does under it.
func (d *listDemo) view(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Clip().Height(24).Children(func() {
			ui.Row(c).Width(640).PadX(8).Gap(8).Items(ui.Center).ScrollWith(&d.list).Children(func() {
				ui.Row(c).Width(16)
				ui.Text(c, d.text("名称", "Name")).Grow(1).Weight(gfx.WeightSemibold)
				ui.Text(c, d.text("大小", "Size")).Width(90).TextEnd().Weight(gfx.WeightSemibold)
				ui.Text(c, d.text("修改时间", "Modified")).Width(150).TextEnd().Weight(gfx.WeightSemibold)
			})
		})
		ui.List(c, &d.list, len(d.rows), d.rowFn).Label(d.text("文件", "Files")).Grow(1)
		ui.Text(c, d.text(
			"拖动行排序；Shift 或 Cmd/Ctrl 点击多选；Shift+滚轮横向滚动。",
			"Drag rows to reorder; Shift- or Cmd/Ctrl-click to choose several; Shift-wheel scrolls sideways.")).
			FontSize(12).Color(theme.MutedForeground).PadY(6)
	})
}

// row is one file, or a section heading.
func (d *listDemo) row(row ui.ListRow) {
	theme := row.C.Theme()
	item := &d.rows[row.Index]
	if row.Header {
		row.Element.Height(32).PadX(8).Items(ui.Center).Background(theme.Muted)
		ui.Text(row.C, item.name).FontSize(12).Weight(gfx.WeightSemibold).Color(theme.MutedForeground)
		return
	}
	row.Element.Height(32).PadX(8).Gap(8).Items(ui.Center)
	if row.Selected() {
		row.Element.Background(theme.Accent)
	}
	ui.Row(row.C).Size(16, 16).Radius(4).Background(theme.Primary)
	ui.Text(row.C, item.name).Grow(1).Ellipsis()
	ui.Text(row.C, item.size).Width(90).TextEnd().FontSize(12).Color(theme.MutedForeground)
	ui.Text(row.C, item.date).Width(150).TextEnd().FontSize(12).Color(theme.MutedForeground)
}

// reorder moves the dragged rows, which the list passes ascending, before row to.
func (d *listDemo) reorder(rows []int, to int) {
	if len(rows) == 0 {
		return
	}
	moving := make([]listDemoItem, 0, len(rows))
	for _, at := range rows {
		if at >= 0 && at < len(d.rows) {
			moving = append(moving, d.rows[at])
		}
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if at := rows[i]; at >= 0 && at < len(d.rows) {
			d.rows = append(d.rows[:at], d.rows[at+1:]...)
		}
	}
	for _, removed := range rows {
		if removed < to {
			to--
		}
	}
	to = min(max(to, 0), len(d.rows))
	result := make([]listDemoItem, 0, len(d.rows)+len(moving))
	result = append(result, d.rows[:to]...)
	result = append(result, moving...)
	result = append(result, d.rows[to:]...)
	d.rows = result
}

// drawPage paints the demo's page into the slot the Frame laid out for it.
func (d *listDemo) drawPage(dc *ui.DrawContext) {
	d.page.Draw(dc.Canvas, dc.Rect, d.viewFn)
}

var listComponent = Component{
	ID: "list", Title: "List", Category: "Data", Summary: "虚拟化列表：多选、吸顶分组、拖动排序与横向滚动。",
	Source:     "ui/list.go",
	initialize: func(state *State) { state.list = newListDemo(state.english) },
	paint: func(state *State, canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
		d := state.list
		d.pageFor(state, theme)
		state.drawPage(canvas, theme, size, 560, d.text(
			"List · 虚拟化、选择与排序", "List · virtualization, selection and reorder"),
			func(c ui.C) {
				ui.Slot(c).FillWidth().Height(310).Draw(d.pageDraw)
			})
	},
	scroll: func(state *State, event gfx.ScrollEvent) gfx.EventResult {
		if state.list.page == nil {
			return gfx.EventResult{}
		}
		return state.list.page.Wheel(event)
	},
	Examples: []Example{{ID: "selection", Title: "文件列表", Description: "文件、大小与修改时间并排；顶部那一组的标题随滚动留在顶部，多选和拖动排序都由列表本身处理。", Height: 400, Snippet: `files := ui.List(c, &m.files, len(m.files), func(row ui.ListRow) {
    row.Element.Height(28).PadX(8).Gap(8).Children(func() {
        ui.Text(row.C, m.files[row.Index].Name)
    })
}).Label("Files").Grow(1)
m.files.Header = func(i int) bool { return m.files[i].Header }
m.files.Selection = &m.chosen
m.files.Reorder = m.move
m.files.RowWidth = 640`}},
}
