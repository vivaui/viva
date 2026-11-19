package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var resizableComponent = Component{
	ID: "resizable", Title: "Resizable", Category: "Layout", Summary: "返回精确面板几何的横向与纵向分隔器。",
	paint: (*State).paintResizable,
	Examples: []Example{
		{ID: "horizontal", Title: "横向分栏", Description: "捕获主指针拖动分隔器，激活仍可循环调用方持有的分栏比例。", Snippet: `bounds := stage.Rect()
handle := tree.ID(page, "split")
resizable := components.BeginResizable(canvas, theme, handle, bounds, fraction).
    Limits(0.2, 0.8).HandleSize(12).WithHandle().ValueText(valueText)
layout := resizable.Paint("Resize editor panels")
canvas.Rounded(layout.First.Inset(gfx.All(8)), 6, theme.Muted)
canvas.Rounded(layout.Second.Inset(gfx.All(8)), 6, theme.CardElevated)`, PageSnippet: `split := ui.SplitterState{Pos: 260}
ui.Row(c).Height(200).Children(func() {
    ui.Column(c).Width(split.Pos).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "Navigator")
    })
    ui.Splitter(c, &split).Mins(120, 200).Label("Resize editor panels")
    ui.Column(c).Grow(1).Basis(0).Background(c.Theme().CardElevated).Children(func() {
        ui.Text(c, "Preview")
    })
})`, PageSnippetZh: `split := ui.SplitterState{Pos: 260}
ui.Row(c).Height(200).Children(func() {
    ui.Column(c).Width(split.Pos).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "Navigator")
    })
    ui.Splitter(c, &split).Mins(120, 200).Label("Resize editor panels")
    ui.Column(c).Grow(1).Basis(0).Background(c.Theme().CardElevated).Children(func() {
        ui.Text(c, "Preview")
    })
})`},
		{ID: "vertical", Title: "纵向分栏", Description: "同一生产组件切换为上下堆叠布局。", Snippet: `bounds := stage.Rect()
handle := tree.ID(page, "split")
layout := components.BeginResizable(canvas, theme, handle, bounds, fraction).
    Vertical().Limits(0.2, 0.8).HandleSize(12).WithHandle().
    Paint("Resize editor rows")
canvas.Rounded(layout.First.Inset(gfx.All(8)), 6, theme.Muted)
canvas.Rounded(layout.Second.Inset(gfx.All(8)), 6, theme.CardElevated)`, PageSnippet: `split := ui.SplitterState{Pos: 100}
ui.Column(c).Height(200).Children(func() {
    ui.Row(c).Height(split.Pos).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "Navigator")
    })
    ui.Splitter(c, &split).Mins(40, 40).Label("Resize editor rows")
    ui.Row(c).Grow(1).Basis(0).Background(c.Theme().CardElevated).Children(func() {
        ui.Text(c, "Preview")
    })
})`, PageSnippetZh: `split := ui.SplitterState{Pos: 100}
ui.Column(c).Height(200).Children(func() {
    ui.Row(c).Height(split.Pos).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "Navigator")
    })
    ui.Splitter(c, &split).Mins(40, 40).Label("Resize editor rows")
    ui.Row(c).Grow(1).Basis(0).Background(c.Theme().CardElevated).Children(func() {
        ui.Text(c, "Preview")
    })
})`},
		{ID: "disabled", Title: "禁用分隔器", Description: "保留布局与抓手，但不允许交互。", Snippet: `bounds := stage.Rect()
handle := tree.ID(page, "split")
layout := components.BeginResizable(canvas, theme, handle, bounds, fraction).
    Limits(0.2, 0.8).HandleSize(12).WithHandle().Disabled().
    Paint("Resize editor panels")
canvas.Rounded(layout.First.Inset(gfx.All(8)), 6, theme.Muted)
canvas.Rounded(layout.Second.Inset(gfx.All(8)), 6, theme.CardElevated)`, PageSnippet: `split := ui.SplitterState{Pos: 260}
ui.Row(c).Height(200).Children(func() {
    ui.Column(c).Width(split.Pos).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "Navigator")
    })
    ui.Splitter(c, &split).Mins(120, 200).Label("Resize editor panels").Disabled(true)
    ui.Column(c).Grow(1).Basis(0).Background(c.Theme().CardElevated).Children(func() {
        ui.Text(c, "Preview")
    })
})`, PageSnippetZh: `split := ui.SplitterState{Pos: 260}
ui.Row(c).Height(200).Children(func() {
    ui.Column(c).Width(split.Pos).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "Navigator")
    })
    ui.Splitter(c, &split).Mins(120, 200).Label("Resize editor panels").Disabled(true)
    ui.Column(c).Grow(1).Basis(0).Background(c.Theme().CardElevated).Children(func() {
        ui.Text(c, "Preview")
    })
})`},
	},
}

var (
	resizableUI     = tree.Feature("resizable")
	resizableHandle = tree.ID(resizableUI, "handle")
)

var resizableDemos = map[*State]*resizableDemo{}

type resizableDemo struct {
	state ui.SplitterState
}

func resizableDemoFor(state *State) *resizableDemo {
	if demo := resizableDemos[state]; demo != nil {
		return demo
	}
	demo := &resizableDemo{}
	if state.exampleID == "vertical" {
		demo.state.Pos = 100
	} else {
		demo.state.Pos = 260
	}
	resizableDemos[state] = demo
	return demo
}

func (state *State) paintResizable(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := resizableDemoFor(state)
	vertical := state.exampleID == "vertical"
	disabled := state.exampleID == "disabled"
	label := state.Text("调整编辑器面板", "Resize editor panels")
	state.drawPage(canvas, theme, size, 600, "Editor layout", func(c ui.C) {
		splitter := func(minsFirst, minsSecond float32) ui.Box {
			box := ui.Splitter(c, &demo.state).Mins(minsFirst, minsSecond).Hit(resizableHandle, label)
			if disabled {
				box = box.Disabled(true)
			}
			return box
		}
		if vertical {
			ui.Column(c).Height(200).Children(func() {
				ui.Row(c).Height(demo.state.Pos).Background(theme.Muted).Children(func() {
					ui.Text(c, "Navigator")
				})
				splitter(40, 40)
				ui.Row(c).Grow(1).Basis(0).Background(theme.CardElevated).Children(func() {
					ui.Text(c, "Preview")
				})
			})
			return
		}
		ui.Row(c).Height(200).Children(func() {
			ui.Column(c).Width(demo.state.Pos).Background(theme.Muted).Children(func() {
				ui.Text(c, "Navigator")
			})
			splitter(120, 200)
			ui.Column(c).Grow(1).Basis(0).Background(theme.CardElevated).Children(func() {
				ui.Text(c, "Preview")
			})
		})
	})
}
