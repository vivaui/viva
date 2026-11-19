package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

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
    SizeToFit("Create project").CenterIn(content).Default("Create project")`, PageSnippet: `ui.Column(c).Fill().Center().Gap(6).Children(func() {
    ui.Icon(c, m.folderIcon).Size(40, 40).Muted()
    ui.Text(c, "No projects yet").FontSize(18).Bold()
    ui.Text(c, "Create a project to get started.").FontSize(13).Muted()
    ui.Button(c, "Create project").Self(ui.Start)
})`, PageSnippetZh: `ui.Column(c).Fill().Center().Gap(6).Children(func() {
    ui.Icon(c, m.folderIcon).Size(40, 40).Muted()
    ui.Text(c, "No projects yet").FontSize(18).Bold()
    ui.Text(c, "Create a project to get started.").FontSize(13).Muted()
    ui.Button(c, "Create project").Self(ui.Start)
})`},
		{ID: "icon", Title: "图标容器", Description: "带弱化图标背景的空状态。", Snippet: `data := components.EmptyData{
    Media: components.EmptyMediaIcon, Icon: icon,
    Title: "No projects yet", Description: "Create a project to get started.",
    ContentHeight: 32,
}
content := components.Empty(canvas, theme, bounds, data)
components.BeginButton(canvas, theme, tree.ID(page, "create"), gfx.Rect{Height: content.Height}).
    SizeToFit("Create project").CenterIn(content).Default("Create project")`, PageSnippet: `ui.Column(c).Fill().Center().Gap(6).Children(func() {
    ui.Column(c).Size(56, 56).Center().Radius(28).Background(c.Theme().Muted).Children(func() {
        ui.Icon(c, m.folderIcon).Size(24, 24).Muted()
    })
    ui.Text(c, "No projects yet").FontSize(18).Bold()
    ui.Text(c, "Create a project to get started.").FontSize(13).Muted()
    ui.Button(c, "Create project").Self(ui.Start)
})`, PageSnippetZh: `ui.Column(c).Fill().Center().Gap(6).Children(func() {
    ui.Column(c).Size(56, 56).Center().Radius(28).Background(c.Theme().Muted).Children(func() {
        ui.Icon(c, m.folderIcon).Size(24, 24).Muted()
    })
    ui.Text(c, "No projects yet").FontSize(18).Bold()
    ui.Text(c, "Create a project to get started.").FontSize(13).Muted()
    ui.Button(c, "Create project").Self(ui.Start)
})`},
	},
}

func (state *State) paintEmpty(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	icon := state.exampleID == "icon"
	state.drawStage(canvas, theme, size, func(c ui.C) {
		ui.Column(c).Fill().Center().Gap(6).Children(func() {
			if icon {
				ui.Column(c).Size(56, 56).Center().Radius(28).Background(theme.Muted).Children(func() {
					ui.Icon(c, folderIcon).Size(24, 24).Muted()
				})
			} else {
				ui.Icon(c, folderIcon).Size(40, 40).Muted()
			}
			ui.Text(c, "No projects yet").FontSize(18).Bold()
			ui.Text(c, "Create a project to get started.").FontSize(13).Muted()
			ui.Button(c, "Create project").Self(ui.Start)
		})
	})
}
