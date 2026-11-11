package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var commandComponent = Component{
	ID: "command", Title: "Command", Category: "Overlays", Summary: "受控查询、分组结果与键盘提示的命令面板。",
	paint: (*State).paintCommand,
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
}`},
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
})`},
		{ID: "empty", Title: "空结果", Description: "使用生产空结果行而不伪造列表。", Height: 210, Snippet: `command := components.BeginCommand(canvas, theme, bounds, components.CommandOptions{
    InputID: inputID, Query: query, Placeholder: "Type a command…", Layer: 1,
})
command.AddEmpty("No commands found")
command.End()`},
	},
}

var (
	commandUI    = tree.Feature("command")
	commandInput = tree.ID(commandUI, "query")
	// commandItem's item i is the i-th result.
	commandItem = tree.Sub(commandUI, tree.Name("item"))
)

func (state *State) paintCommand(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	variant := components.CommandDefault
	// Input row, group label, three result rows, separator, and bottom padding.
	height := float32(36 + 28 + 3*32 + 9 + 4)
	query := ""
	switch state.exampleID {
	case "dialog":
		variant = components.CommandDialog
		height = 48 + 28 + 3*44 + 9 + 4
		query = "open"
	case "empty":
		height = 36 + 68 + 4
	}
	var panel ui.Box
	state.drawPage(canvas, theme, size, 520, state.Text("Command · 快速操作", "Command · Quick actions"), func(c ui.C) {
		panel = ui.Slot(c).FillWidth().Height(min(height, max(size.Height-84, 120)))
	})
	command := components.BeginCommand(canvas, theme, panel.Rect(), components.CommandOptions{InputID: commandInput, Label: "Command query", Query: query, Placeholder: "Type a command…", Hint: "Search commands", Variant: variant, Layer: 1})
	if state.exampleID == "empty" {
		command.AddEmpty("No commands found")
		return
	}
	// Selected is the row Enter runs (cmdk's data-selected): the pointer moves it
	// here, and a real panel also moves it from its arrow keys.
	current := int(state.values[0])
	for i := 0; i < 3; i++ {
		if canvas.Hovered() == commandItem.At(i) {
			current = i
		}
	}
	command.AddGroup("Suggestions")
	command.AddItem(components.CommandItem{ID: commandItem.At(0), Label: "Open workspace", Shortcut: "⌘O", Selected: current == 0})
	command.AddItem(components.CommandItem{ID: commandItem.At(1), Label: "Create window", Shortcut: "⌘N", Selected: current == 1})
	command.AddSeparator()
	command.AddItem(components.CommandItem{ID: commandItem.At(2), Label: "Pin current view", Checked: current == 2, Selected: current == 2, Disabled: state.exampleID == "dialog"})
	if index, ok := canvas.ClickedIn(commandItem); ok {
		state.values[0] = int32(index)
	}
}
