package componentdemo

import (
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

// dropdownMenuDemo is the caller-owned state of the page-first Menu.
type dropdownMenuDemo struct {
	open, compact, ready bool
}

func (state *State) paintDropdownMenu(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	label := "Account"
	if state.exampleID != "default" {
		label = "Team"
	}
	state.drawPage(canvas, theme, size, 260, "Dropdown Menu · Account", func(c ui.C) {
		demo := ui.State[dropdownMenuDemo](c, "dropdown-menu-demo")
		if !demo.ready {
			demo.ready, demo.open = true, true
		}
		trigger := ui.Button(c, label).Outline()
		if trigger.Clicked() {
			demo.open = !demo.open
		}
		ui.Menu(c, trigger, &demo.open, func(menu *ui.MenuItems) {
			if state.exampleID == "default" {
				menu.Label("skyler@example.com")
				menu.Item("Profile").Keys(ui.Primary|ui.Shift, "P")
				if menu.Item("Compact mode").Checked(demo.compact).Clicked() {
					demo.compact = !demo.compact
				}
				menu.Separator()
				menu.Item("Sign out").Destructive()
				return
			}
			menu.Sub("Team", func(sub *ui.MenuItems) {
				sub.Item("Personal").Chosen(true)
				sub.Item("Organization").Chosen(false)
			})
			menu.Item("Unavailable").Disabled(true)
		})
		state.flags &^= 1
		if demo.compact {
			state.flags |= 1
		}
	})
}
