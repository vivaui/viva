package componentdemo

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/editabletabs"
)

var editableTabsComponent = Component{
	ID: "editable-tabs", Title: "Editable Tabs", Category: "Navigation", Summary: "可关闭、可新增、可滚动的标签条，如编辑器或终端的标签。",
	Source: "ui/editabletabs/editabletabs.go", paint: (*State).paintEditableTabs,
	Examples: []Example{
		{ID: "default", Title: "关闭与新增", Description: "标签宽度随标题；点 × 或中键、Delete 关闭，+ 新增。控件只报告，标签由页面持有；固定的标签没有关闭按钮。", Snippet: `m.strip = editabletabs.New()
// m.tabs is a []editabletabs.Tab kept in the model.
m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
if i, ok := m.strip.Closed(); ok {
    m.closeTab(i)
}
if m.strip.Added() {
    m.openTab()
}`, PageSnippet: `m.strip = editabletabs.New()
// m.tabs is a []editabletabs.Tab kept in the model.
m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
if i, ok := m.strip.Closed(); ok {
    m.closeTab(i)
}
if m.strip.Added() {
    m.openTab()
}`, PageSnippetZh: `m.strip = editabletabs.New()
// m.tabs 是保存在模型里的 []editabletabs.Tab。
m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
if i, ok := m.strip.Closed(); ok {
    m.closeTab(i)
}
if m.strip.Added() {
    m.openTab()
}`},
		{ID: "overflow", Title: "溢出滚动", Description: "标签条比空间长时用滚轮滚动，并保持当前标签可见。", Snippet: `editabletabs.Spec{Tabs: m.tabs} // wheel scrolls; the active tab stays in view`, PageSnippet: `editabletabs.Spec{Tabs: m.tabs} // wheel scrolls; the active tab stays in view`, PageSnippetZh: `editabletabs.Spec{Tabs: m.tabs} // wheel scrolls; the active tab stays in view`},
	},
}

type editableTabsDemo struct {
	strip  *editabletabs.EditableTabs
	tabs   []editabletabs.Tab
	active int
	serial int
}

var editableTabsDemos = map[*State]*editableTabsDemo{}

func editableTabsDemoFor(state *State) *editableTabsDemo {
	if demo := editableTabsDemos[state]; demo != nil {
		return demo
	}
	demo := &editableTabsDemo{strip: editabletabs.New(), active: 1, serial: 3, tabs: []editabletabs.Tab{
		{Title: "build-01", Pinned: true}, {Title: "db-primary"}, {Title: "staging"},
	}}
	if state.exampleID == "overflow" {
		demo.tabs = append(demo.tabs, editabletabs.Tab{Title: "bastion"}, editabletabs.Tab{Title: "monitor"}, editabletabs.Tab{Title: "cache-01"}, editabletabs.Tab{Title: "queue"})
		demo.active = 6
	}
	editableTabsDemos[state] = demo
	return demo
}

func (state *State) paintEditableTabs(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := editableTabsDemoFor(state)
	spec := editabletabs.Spec{Tabs: demo.tabs, Addable: state.exampleID != "overflow", CloseLabel: state.Text("关闭", "Close"), AddLabel: state.Text("新增标签", "Add tab")}
	state.drawPage(canvas, theme, size, 560, state.Text("Editable Tabs · 标签条", "Editable Tabs"), func(c ui.C) {
		strip := demo.strip.View(c, state.Text("会话", "Sessions"), &demo.active, spec)
		if state.exampleID == "overflow" {
			strip.Width(320)
		} else {
			strip.FillWidth()
		}
		if i, ok := demo.strip.Closed(); ok {
			demo.tabs = append(demo.tabs[:i], demo.tabs[i+1:]...)
			if demo.active >= i {
				demo.active = max(demo.active-1, 0)
			}
		}
		if demo.strip.Added() {
			demo.serial++
			demo.tabs = append(demo.tabs, editabletabs.Tab{Title: state.Text("会话 ", "session ") + strconv.Itoa(demo.serial)})
			demo.active = len(demo.tabs) - 1
		}
		if len(demo.tabs) > 0 {
			ui.Text(c, demo.tabs[min(demo.active, len(demo.tabs)-1)].Title).Muted()
		}
	})
}
