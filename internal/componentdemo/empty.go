package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/empty"
)

var emptyComponent = Component{
	ID: "empty", Title: "Empty", Category: "Feedback", Summary: "无数据时的居中图形、标题与说明。",
	PageSource: "ui/empty/empty.go", paint: (*State).paintEmpty,
	Examples: []Example{
		{ID: "default", Title: "无项目", Description: "图标直接绘制在标题上方，下方留出放按钮的内容区。", Snippet: `data := components.EmptyData{
    Icon: icon, Title: "No projects yet",
    Description: "Create a project to get started.", ContentHeight: 32,
}
content := components.Empty(canvas, theme, bounds, data)
components.BeginButton(canvas, theme, tree.ID(page, "create"), gfx.Rect{Height: content.Height}).
    SizeToFit("Create project").CenterIn(content).Default("Create project")`, PageSnippet: `empty.View(c, empty.Spec{
    Icon:        m.folderIcon,
    Title:       "No projects yet",
    Description: "Create a project to get started.",
}, func() {
    ui.Button(c, "Create project")
}).Fill()`, PageSnippetZh: `empty.View(c, empty.Spec{
    Icon:        m.folderIcon,
    Title:       "No projects yet",
    Description: "Create a project to get started.",
}, func() {
    ui.Button(c, "Create project")
}).Fill()`},
		{ID: "icon", Title: "图标容器", Description: "Boxed 把图标放进弱化的圆角方块。", Snippet: `data := components.EmptyData{
    Media: components.EmptyMediaIcon, Icon: icon,
    Title: "No projects yet", Description: "Create a project to get started.",
    ContentHeight: 32,
}
content := components.Empty(canvas, theme, bounds, data)
components.BeginButton(canvas, theme, tree.ID(page, "create"), gfx.Rect{Height: content.Height}).
    SizeToFit("Create project").CenterIn(content).Default("Create project")`, PageSnippet: `empty.View(c, empty.Spec{
    Icon:        m.folderIcon,
    Boxed:       true,
    Title:       "No projects yet",
    Description: "Create a project to get started.",
}, func() {
    ui.Button(c, "Create project")
}).Fill()`, PageSnippetZh: `empty.View(c, empty.Spec{
    Icon:        m.folderIcon,
    Boxed:       true,
    Title:       "No projects yet",
    Description: "Create a project to get started.",
}, func() {
    ui.Button(c, "Create project")
}).Fill()`},
	},
}

// emptyDemo keeps the closure the state calls for its actions, made once.
type emptyDemo struct {
	c       ui.C
	actions func()
}

var emptyDemos = map[*State]*emptyDemo{}

func emptyDemoFor(state *State) *emptyDemo {
	if demo := emptyDemos[state]; demo != nil {
		return demo
	}
	demo := &emptyDemo{}
	demo.actions = func() { ui.Button(demo.c, state.Text("创建项目", "Create project")) }
	emptyDemos[state] = demo
	return demo
}

func (state *State) paintEmpty(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := emptyDemoFor(state)
	spec := empty.Spec{
		Icon: folderIcon, Boxed: state.exampleID == "icon",
		Title:       state.Text("还没有项目", "No projects yet"),
		Description: state.Text("创建一个项目即可开始。", "Create a project to get started."),
	}
	state.drawStage(canvas, theme, size, func(c ui.C) {
		demo.c = c
		empty.View(c, spec, demo.actions).Fill()
	})
}
