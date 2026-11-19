package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var dropdownMenuComponent = Component{
	ID: "dropdown-menu", Title: "Dropdown Menu", Category: "Overlays", Summary: "触发器旁定位的操作菜单与受控选择状态。",
	paint: (*State).paintDropdownMenu,
	Examples: []Example{
		{ID: "default", Title: "账户菜单", Description: "生产 DropdownMenu 绘制动作、快捷键和分隔线。", Snippet: `menu := components.BeginDropdownMenu(canvas, theme, bounds, components.DropdownMenuOptions{Layer: 1})
menu.AddLabel("skyler@example.com", false)
menu.AddItem(components.DropdownMenuItem{ID: tree.ID(page, "profile"), Label: "Profile", Shortcut: "⇧⌘P"})
menu.AddItem(components.DropdownMenuItem{
    ID: toggleID, Label: "Compact mode", Kind: components.DropdownMenuCheckbox, Checked: compact,
})
menu.AddSeparator()
menu.AddItem(components.DropdownMenuItem{
    ID: tree.ID(page, "sign-out"), Label: "Sign out", Variant: components.DropdownMenuItemDestructive,
})
menu.End()
if canvas.Clicked(toggleID) { compact = !compact }`, PageSnippet: `open := false
trigger := ui.Button(c, "Account").Outline()
if trigger.Clicked() {
    open = !open
}
ui.Menu(c, trigger, &open, func(menu *ui.MenuItems) {
    menu.Label("skyler@example.com")
    menu.Item("Profile").Keys(ui.Primary|ui.Shift, "P")
    menu.Item("Compact mode").Checked(false)
    menu.Separator()
    menu.Item("Sign out").Destructive()
})`, PageSnippetZh: `open := false
trigger := ui.Button(c, "Account").Outline()
if trigger.Clicked() {
    open = !open
}
ui.Menu(c, trigger, &open, func(menu *ui.MenuItems) {
    menu.Label("skyler@example.com")
    menu.Item("Profile").Keys(ui.Primary|ui.Shift, "P")
    menu.Item("Compact mode").Checked(false)
    menu.Separator()
    menu.Item("Sign out").Destructive()
})`},
		{ID: "states", Title: "选择状态", Description: "复选、单选、子菜单与禁用项。", Snippet: `items := tree.Sub(page, tree.Name("item"))
menu := components.BeginDropdownMenu(canvas, theme, bounds, components.DropdownMenuOptions{Layer: 1})
menu.AddItem(components.DropdownMenuItem{
    ID: items.At(0), Label: "Team", Kind: components.DropdownMenuSubmenu, Expanded: true,
})
for index, label := range []string{"Personal", "Organization"} {
    menu.AddItem(components.DropdownMenuItem{
        ID: items.At(index + 1), Label: label, Kind: components.DropdownMenuRadio, Selected: scope == index+1,
    })
}
menu.AddItem(components.DropdownMenuItem{ID: items.At(3), Label: "Unavailable", Disabled: true})
menu.End()
if index, ok := canvas.ClickedIn(items); ok && index > 0 && index < 3 { scope = int(index) }`, PageSnippet: `open := false
trigger := ui.Button(c, "Team").Outline()
if trigger.Clicked() {
    open = !open
}
ui.Menu(c, trigger, &open, func(menu *ui.MenuItems) {
    menu.Sub("Team", func(sub *ui.MenuItems) {
        sub.Item("Personal").Chosen(true)
        sub.Item("Organization").Chosen(false)
    })
    menu.Item("Unavailable").Disabled(true)
})`, PageSnippetZh: `open := false
trigger := ui.Button(c, "Team").Outline()
if trigger.Clicked() {
    open = !open
}
ui.Menu(c, trigger, &open, func(menu *ui.MenuItems) {
    menu.Sub("Team", func(sub *ui.MenuItems) {
        sub.Item("Personal").Chosen(true)
        sub.Item("Organization").Chosen(false)
    })
    menu.Item("Unavailable").Disabled(true)
})`},
	},
}

var (
	dropdownMenuUI     = tree.Feature("dropdown-menu")
	dropdownMenuToggle = tree.ID(dropdownMenuUI, "compact")
	// dropdownMenuItem's item i is the i-th plain item of the example.
	dropdownMenuItem = tree.Sub(dropdownMenuUI, tree.Name("item"))
)

func (state *State) paintDropdownMenu(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	// Label (32), three rows (96), separator (9), and the surface's 4px padding.
	height := float32(32 + 3*32 + 9 + 2*4)
	if state.exampleID != "default" {
		height = 4*32 + 2*4
	}
	var panel ui.Box
	state.drawPage(canvas, theme, size, 260, "Dropdown Menu · Account", func(c ui.C) {
		panel = ui.Slot(c).FillWidth().Height(min(height, max(size.Height-84, 120)))
	})
	menu := components.BeginDropdownMenu(canvas, theme, panel.Rect(), components.DropdownMenuOptions{Layer: 1})
	if state.exampleID == "default" {
		menu.AddLabel("skyler@example.com", false)
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuItem.At(0), Label: "Profile", Shortcut: "⇧⌘P"})
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuToggle, Label: "Compact mode", Kind: components.DropdownMenuCheckbox, Checked: state.flags&1 != 0})
		menu.AddSeparator()
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuItem.At(1), Label: "Sign out", Variant: components.DropdownMenuItemDestructive})
	} else {
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuItem.At(2), Label: "Team", Kind: components.DropdownMenuSubmenu, Expanded: true})
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuItem.At(3), Label: "Personal", Kind: components.DropdownMenuRadio, Selected: true})
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuItem.At(4), Label: "Organization", Kind: components.DropdownMenuRadio})
		menu.AddItem(components.DropdownMenuItem{ID: dropdownMenuItem.At(5), Label: "Unavailable", Disabled: true})
	}
	menu.End()
	if canvas.Clicked(dropdownMenuToggle) {
		state.flags ^= 1
	}
}
