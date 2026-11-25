package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tag"
)

var tagComponent = Component{
	ID: "tag", Title: "Tag", Category: "Data Display", Summary: "短标签：状态着色、描边、三种尺寸，可关闭，可作为开关。",
	Source: "ui/tag/tag.go", paint: (*State).paintTag,
	Examples: []Example{
		{ID: "default", Title: "状态与描边", Description: "默认、成功、警告、危险共用 ui.Status；Outlined 只画边框。", Snippet: `tag.View(c.KeyInt(0), "default", tag.Spec{})
tag.View(c.KeyInt(1), "success", tag.Spec{Status: ui.StatusSuccess})
tag.View(c.KeyInt(2), "outlined", tag.Spec{Outlined: true})`, PageSnippet: `tag.View(c.KeyInt(0), "default", tag.Spec{})
tag.View(c.KeyInt(1), "success", tag.Spec{Status: ui.StatusSuccess})
tag.View(c.KeyInt(2), "outlined", tag.Spec{Outlined: true})`, PageSnippetZh: `tag.View(c.KeyInt(0), "default", tag.Spec{})
tag.View(c.KeyInt(1), "success", tag.Spec{Status: ui.StatusSuccess})
tag.View(c.KeyInt(2), "outlined", tag.Spec{Outlined: true})`},
		{ID: "interactive", Title: "可关闭与可勾选", Description: "点关闭按钮移除标签；可勾选的标签是开关，由页面保存选中状态。", Snippet: `for i, name := range m.tags {
    if tag.View(c.Key(name), name, tag.Spec{Closable: true}).Closed {
        m.removeTag(i)
    }
}
result := tag.View(c, "production", tag.Spec{Checkable: true, Checked: m.production})
if result.Toggled {
    m.production = !m.production
}`, PageSnippet: `for i, name := range m.tags {
    if tag.View(c.Key(name), name, tag.Spec{Closable: true}).Closed {
        m.removeTag(i)
    }
}
result := tag.View(c, "production", tag.Spec{Checkable: true, Checked: m.production})
if result.Toggled {
    m.production = !m.production
}`, PageSnippetZh: `for i, name := range m.tags {
    if tag.View(c.Key(name), name, tag.Spec{Closable: true}).Closed {
        m.removeTag(i)
    }
}
result := tag.View(c, "production", tag.Spec{Checkable: true, Checked: m.production})
if result.Toggled {
    m.production = !m.production
}`},
		{ID: "sizes", Title: "尺寸", Description: "Small、Default、Large 三档，图标与关闭按钮随之缩放。", Snippet: `tag.View(c, "small", tag.Spec{Size: tag.Small, Closable: true})
tag.View(c, "large", tag.Spec{Size: tag.Large, Closable: true})`, PageSnippet: `tag.View(c, "small", tag.Spec{Size: tag.Small, Closable: true})
tag.View(c, "large", tag.Spec{Size: tag.Large, Closable: true})`, PageSnippetZh: `tag.View(c, "small", tag.Spec{Size: tag.Small, Closable: true})
tag.View(c, "large", tag.Spec{Size: tag.Large, Closable: true})`},
	},
}

type tagDemo struct {
	names      []string
	production bool
}

var tagDemos = map[*State]*tagDemo{}

func tagDemoFor(state *State) *tagDemo {
	if demo := tagDemos[state]; demo != nil {
		return demo
	}
	demo := &tagDemo{names: []string{"build-01", "db-primary", "staging", "bastion"}, production: true}
	tagDemos[state] = demo
	return demo
}

func (state *State) paintTag(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := tagDemoFor(state)
	state.drawPage(canvas, theme, size, 560, state.Text("Tag · 标签", "Tag"), func(c ui.C) {
		remove := state.Text("移除", "Remove")
		row := func(children func()) { ui.Row(c).Gap(8).Items(ui.Center).Children(children) }
		switch state.exampleID {
		case "interactive":
			row(func() {
				for i := 0; i < len(demo.names); i++ {
					if tag.View(c.Key(demo.names[i]), demo.names[i], tag.Spec{Closable: true, CloseLabel: remove}).Closed {
						demo.names = append(demo.names[:i], demo.names[i+1:]...)
						i--
					}
				}
				if len(demo.names) == 0 {
					ui.Text(c, state.Text("标签已全部移除", "Every tag was removed")).Muted()
				}
			})
			row(func() {
				if tag.View(c.KeyInt(100), state.Text("生产环境", "production"), tag.Spec{Checkable: true, Checked: demo.production}).Toggled {
					demo.production = !demo.production
				}
			})
		case "sizes":
			row(func() {
				tag.View(c.KeyInt(0), "small", tag.Spec{Size: tag.Small, Closable: true, CloseLabel: remove})
				tag.View(c.KeyInt(1), "default", tag.Spec{Closable: true, CloseLabel: remove})
				tag.View(c.KeyInt(2), "large", tag.Spec{Size: tag.Large, Closable: true, CloseLabel: remove})
			})
		default:
			row(func() {
				tag.View(c.KeyInt(0), state.Text("默认", "default"), tag.Spec{})
				tag.View(c.KeyInt(1), state.Text("成功", "success"), tag.Spec{Status: ui.StatusSuccess})
				tag.View(c.KeyInt(2), state.Text("警告", "warning"), tag.Spec{Status: ui.StatusWarning})
				tag.View(c.KeyInt(3), state.Text("危险", "danger"), tag.Spec{Status: ui.StatusDanger})
			})
			row(func() {
				tag.View(c.KeyInt(4), state.Text("描边", "outlined"), tag.Spec{Outlined: true})
				tag.View(c.KeyInt(5), state.Text("成功描边", "success"), tag.Spec{Outlined: true, Status: ui.StatusSuccess})
			})
		}
	})
}
