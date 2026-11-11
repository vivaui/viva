package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var emptyCreate = tree.ID(tree.Feature("empty"), "create")

var emptyComponent = Component{
	ID: "empty", Title: "Empty", Category: "Feedback", Summary: "无数据时的居中图形、标题与说明。",
	paint: (*State).paintEmpty,
	Examples: []Example{
		{ID: "default", Title: "无项目", Description: "图标直接绘制在标题上方，下方留出放按钮的内容区。", Snippet: `data := components.EmptyData{
    Icon: icon, Title: "No projects yet",
    Description: "Create a project to get started.", ContentHeight: 32,
}
content := components.Empty(canvas, theme, bounds, data)
components.BeginButton(canvas, theme, tree.ID(page, "create"), gfx.Rect{Height: content.Height}).
    SizeToFit("Create project").CenterIn(content).Default("Create project")`},
		{ID: "icon", Title: "图标容器", Description: "带弱化图标背景的空状态。", Snippet: `data := components.EmptyData{
    Media: components.EmptyMediaIcon, Icon: icon,
    Title: "No projects yet", Description: "Create a project to get started.",
    ContentHeight: 32,
}
content := components.Empty(canvas, theme, bounds, data)
components.BeginButton(canvas, theme, tree.ID(page, "create"), gfx.Rect{Height: content.Height}).
    SizeToFit("Create project").CenterIn(content).Default("Create project")`},
	},
}

func (state *State) paintEmpty(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var stage ui.Box
	state.drawPage(canvas, theme, size, 560, state.Text("Empty · 空状态", "Empty · No data"), func(c ui.C) {
		stage = ui.Slot(c).FillWidth().Height(max(size.Height-110, 180))
	})
	data := components.EmptyData{
		Icon: folderIcon, Title: "No projects yet", Description: "Create a project to get started.", ContentHeight: 32,
	}
	if state.exampleID == "icon" {
		data.Media = components.EmptyMediaIcon
	}
	// The centered text needs more than the page column on a narrow stage, so the
	// recipe is given the page's side margin too.
	bounds := stage.Rect()
	bounds.X, bounds.Width = bounds.X-16, bounds.Width+32
	content := components.Empty(canvas, theme, bounds, data)
	const label = "Create project"
	components.BeginButton(canvas, theme, emptyCreate, gfx.Rect{Height: content.Height}).
		SizeToFit(label).CenterIn(content).Default(label)
}
