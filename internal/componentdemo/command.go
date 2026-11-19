package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/command"
)

var commandComponent = Component{
	ID: "command", Title: "Command", Category: "Overlays", Summary: "受控查询、分组结果与键盘提示的命令面板。",
	paint: (*State).paintCommand, initialize: (*State).initializeCommand,
	editable: func(exampleID string) (editableField, bool) {
		initial := ""
		if exampleID == "dialog" {
			initial = "open"
		}
		return editableField{ID: commandInput, Initial: initial}, true
	},
	editor: func(state *State) *text.Editor {
		initial := ""
		if state.exampleID == "dialog" {
			initial = "open"
		}
		return text.NewEditor(initial)
	},
	Examples: []Example{
		{ID: "default", Title: "命令搜索", Description: "生产 Command 组合输入快照与调用方筛选的结果。", Height: 270, Snippet: `results := tree.Sub(page, tree.Name("item"))
command := components.BeginCommand(canvas, theme, bounds, components.CommandOptions{
    InputID: inputID, Query: query, Placeholder: "Type a command…", Layer: 1,
})
command.AddGroup("Suggestions")
command.AddItem(components.CommandItem{ID: results.At(0), Label: "Open workspace",
    Shortcut: "⌘O", Selected: selected == 0})
command.AddItem(components.CommandItem{ID: results.At(1), Label: "Create window",
    Shortcut: "⌘N", Selected: selected == 1})
// Selected is the row Enter runs: the pointer and the arrow keys move it.
if hovered := canvas.Hovered(); !hovered.IsZero() {
    for i := 0; i < 2; i++ {
        if hovered == results.At(i) {
            selected = i
        }
    }
}
if index, ok := canvas.ClickedIn(results); ok {
    selected = int(index)
}`,
			PageSnippet: `m.command.Filter = func(query string) {
    // Re-filter m.results and rebuild m.command.Results here.
}
m.command.Select = func(index int) {
    m.selected = index
}
m.command.Choose = func(index int) {
    m.chosen = index
}
m.command.Clear()
m.command.Group("Suggestions")
m.command.Item("Open workspace", "⌘O")
m.command.Item("Create window", "⌘N")
m.command.Separator()
m.command.Item("Pin current view", "")
command.View(c, "Command", &m.command).FillWidth().Height(180)`,
			PageSnippetZh: `m.command.Filter = func(query string) {
    // 在这里按 query 重新过滤 m.results，并重建 m.command.Results。
}
m.command.Select = func(index int) {
    m.selected = index
}
m.command.Choose = func(index int) {
    m.chosen = index
}
m.command.Clear()
m.command.Group("建议")
m.command.Item("打开工作区", "⌘O")
m.command.Item("新建窗口", "⌘N")
m.command.Separator()
m.command.Item("固定当前视图", "")
command.View(c, "命令", &m.command).FillWidth().Height(180)`},
		{ID: "dialog", Title: "对话框尺寸", Description: "更宽松的命令面板行高和已选状态。", Height: 320, Snippet: `results := tree.Sub(page, tree.Name("item"))
command := components.BeginCommand(canvas, theme, bounds, components.CommandOptions{
    InputID: inputID, Query: "open", Placeholder: "Type a command…",
    Variant: components.CommandDialog, Layer: 1,
})
command.AddItem(components.CommandItem{
    ID: results.At(0), Label: "Open workspace", Shortcut: "⌘O", Selected: true,
})
command.AddSeparator()
command.AddItem(components.CommandItem{
    ID: results.At(2), Label: "Pin current view", Checked: checked, Disabled: true,
})`,
			PageSnippet: `m.command.Options = m.command.Options.Dialog()
m.command.Query = "open"
m.command.Selected = 0
m.command.Clear()
m.command.Group("Suggestions")
m.command.Item("Open workspace", "⌘O")
m.command.Item("Create window", "⌘N")
m.command.Separator()
m.command.Item("Pin current view", "")
// A Result appended with Disabled: true is painted inert.
command.View(c, "Command", &m.command).FillWidth().Height(220)`,
			PageSnippetZh: `m.command.Options = m.command.Options.Dialog()
m.command.Query = "open"
m.command.Selected = 0
m.command.Clear()
m.command.Group("建议")
m.command.Item("打开工作区", "⌘O")
m.command.Item("新建窗口", "⌘N")
m.command.Separator()
m.command.Item("固定当前视图", "")
// 追加 Disabled: true 的 Result 会绘制为不可用行。
command.View(c, "命令", &m.command).FillWidth().Height(220)`},
		{ID: "empty", Title: "空结果", Description: "使用生产空结果行而不伪造列表。", Height: 210, Snippet: `command := components.BeginCommand(canvas, theme, bounds, components.CommandOptions{
    InputID: inputID, Query: query, Placeholder: "Type a command…", Layer: 1,
})
command.AddEmpty("No commands found")
command.End()`,
			PageSnippet: `m.command.Query = ""
m.command.Selected = 0
m.command.Clear()
m.command.Empty("No commands found")
command.View(c, "Command", &m.command).FillWidth().Height(112)`,
			PageSnippetZh: `m.command.Query = ""
m.command.Selected = 0
m.command.Clear()
m.command.Empty("没有找到命令")
command.View(c, "命令", &m.command).FillWidth().Height(112)`},
	},
}

var (
	commandUI = tree.Feature("command")
	// commandInput is the demo's native text hit before the first paint; the
	// page control's own identity replaces it once View declares the control.
	commandInput = tree.ID(commandUI, "query")
	// commandItem's item i is the i-th result.
	commandItem = tree.Sub(commandUI, tree.Name("item"))
)

// commandDemo is one command example's page control. The wrapper's handlers
// and draw function point at this value, so it is kept beside the State rather
// than rebuilt in each paint.
type commandDemo struct {
	command command.Command
}

// commandDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var commandDemos = map[*State]*commandDemo{}

func (state *State) initializeCommand() {
	demo := &commandDemo{}
	if state.exampleID == "dialog" {
		demo.command.Query = "open"
		demo.command.Options = demo.command.Options.Dialog()
	}
	// The selection is the wrapper's; the test reads it back through the demo's
	// shared value slot.
	demo.command.Select = func(index int) { state.values[0] = int32(index) }
	commandDemos[state] = demo
}

func commandDemoFor(state *State) *commandDemo {
	if demo := commandDemos[state]; demo != nil {
		return demo
	}
	demo := &commandDemo{}
	commandDemos[state] = demo
	return demo
}

func (state *State) paintCommand(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := commandDemoFor(state)
	// Input row, group label, three result rows, separator, and bottom padding.
	height := float32(36 + 28 + 3*32 + 9 + 4)
	switch state.exampleID {
	case "dialog":
		height = 48 + 28 + 3*44 + 9 + 4
	case "empty":
		height = 36 + 68 + 4
	}
	state.drawPage(canvas, theme, size, 520, state.Text("Command · 快速操作", "Command · Quick actions"), func(c ui.C) {
		panel := &demo.command
		panel.Clear()
		if state.exampleID == "empty" {
			panel.Empty(state.Text("没有找到命令", "No commands found"))
		} else {
			panel.Group(state.Text("建议", "Suggestions"))
			panel.Results = append(panel.Results, command.Result{
				ID: commandItem.At(0), Label: state.Text("打开工作区", "Open workspace"), Shortcut: "⌘O",
			})
			panel.Results = append(panel.Results, command.Result{
				ID: commandItem.At(1), Label: state.Text("新建窗口", "Create window"), Shortcut: "⌘N",
			})
			panel.Results = append(panel.Results, command.Result{Kind: command.ResultSeparator})
			panel.Results = append(panel.Results, command.Result{
				ID:       commandItem.At(2),
				Label:    state.Text("固定当前视图", "Pin current view"),
				Checked:  panel.Selected == 2,
				Disabled: state.exampleID == "dialog",
			})
		}
		command.View(c, state.Text("命令", "Command"), panel).
			FillWidth().Height(min(height, max(size.Height-84, 120)))
		if state.editing {
			// The query row is a part of the control; the demo's fixed-field text
			// service edits the page control's own editor, so typing reaches Query
			// and Filter exactly as it does for a page that calls Page.TextInput.
			state.fieldID = panel.InputID()
			state.editor = panel.Editor()
		}
	})
}
