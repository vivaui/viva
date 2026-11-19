package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var itemComponent = Component{
	ID: "item", Title: "Item", Category: "Display", Summary: "可交互列表行、密度与表面变体。",
	paint: (*State).paintItem,
	Examples: []Example{
		{ID: "default", Title: "可选择行", Description: "点击生产 Item 行切换调用方持有的选中提示。", Snippet: `id := row.At(0)
trailing := "Open"
if selected {
    trailing = "Selected"
}
layout := components.BeginItem(canvas, theme, id, bounds).
    Variant(components.ItemOutline).LeadingSpace(28).
    Paint("Release notes", "Latest runtime changes.", trailing)
canvas.Rounded(layout.Media, 6, theme.Primary)
if canvas.Clicked(id) {
    selected = !selected
}`, PageSnippet: `look := c.Theme()
trailing := "Open"
if m.selected {
    trailing = "Selected"
}
row := ui.Row(c).FillWidth().Pad(12).Gap(12).Items(ui.Center).Hit(ui.ID(c), "Release notes")
row.Children(func() {
    ui.Row(c).Size(24, 24).Radius(6).Background(look.Primary)
    ui.Column(c).Grow(1).Gap(2).Children(func() {
        ui.Text(c, "Release notes").Bold()
        ui.Text(c, "Latest runtime changes.").Muted()
    })
    ui.Text(c, trailing)
})
if row.Clicked() {
    m.selected = !m.selected
}`, PageSnippetZh: `look := c.Theme()
trailing := "Open"
if m.selected {
    trailing = "Selected"
}
row := ui.Row(c).FillWidth().Pad(12).Gap(12).Items(ui.Center).Hit(ui.ID(c), "Release notes")
row.Children(func() {
    ui.Row(c).Size(24, 24).Radius(6).Background(look.Primary)
    ui.Column(c).Grow(1).Gap(2).Children(func() {
        ui.Text(c, "Release notes").Bold()
        ui.Text(c, "Latest runtime changes.").Muted()
    })
    ui.Text(c, trailing)
})
if row.Clicked() {
    m.selected = !m.selected
}`},
		{ID: "variants", Title: "表面与密度", Description: "描边、弱化与紧凑尺寸组合。", Snippet: `components.BeginItem(canvas, theme, row.At(0), rowBounds[0]).
    Variant(components.ItemOutline).Paint("Outline", "Default density", "⌘1")
components.BeginItem(canvas, theme, row.At(1), rowBounds[1]).
    Variant(components.ItemMuted).Size(components.ItemSizeSmall).
    Paint("Muted", "Small density", "⌘2")
components.BeginItem(canvas, theme, row.At(2), rowBounds[2]).
    Size(components.ItemSizeExtraSmall).Disabled().
    Paint("Disabled", "", "Unavailable")`, PageSnippet: `ui.Row(c).FillWidth().Pad(12).Gap(12).Items(ui.Center).Children(func() {
    ui.Column(c).Grow(1).Children(func() {
        ui.Text(c, "Outline")
        ui.Text(c, "Default density").Muted()
    })
    ui.Text(c, "⌘1")
})
ui.Row(c).FillWidth().Pad(8).Gap(8).Items(ui.Center).Children(func() {
    ui.Column(c).Grow(1).Children(func() {
        ui.Text(c, "Muted")
        ui.Text(c, "Small density").Muted()
    })
    ui.Text(c, "⌘2")
})
ui.Row(c).FillWidth().Pad(6).Gap(6).Items(ui.Center).Disabled(true).Children(func() {
    ui.Column(c).Grow(1).Children(func() {
        ui.Text(c, "Disabled")
    })
    ui.Text(c, "Unavailable").Muted()
})`, PageSnippetZh: `ui.Row(c).FillWidth().Pad(12).Gap(12).Items(ui.Center).Children(func() {
    ui.Column(c).Grow(1).Children(func() {
        ui.Text(c, "Outline")
        ui.Text(c, "Default density").Muted()
    })
    ui.Text(c, "⌘1")
})
ui.Row(c).FillWidth().Pad(8).Gap(8).Items(ui.Center).Children(func() {
    ui.Column(c).Grow(1).Children(func() {
        ui.Text(c, "Muted")
        ui.Text(c, "Small density").Muted()
    })
    ui.Text(c, "⌘2")
})
ui.Row(c).FillWidth().Pad(6).Gap(6).Items(ui.Center).Disabled(true).Children(func() {
    ui.Column(c).Grow(1).Children(func() {
        ui.Text(c, "Disabled")
    })
    ui.Text(c, "Unavailable").Muted()
})`},
	},
}

var (
	itemUI = tree.Feature("item")
	// itemRow's item i is the i-th row of the example.
	itemRow = tree.Sub(itemUI, tree.Name("row"))
)

func (state *State) paintItem(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	selected := state.flags&1 != 0
	trailing := "Open"
	if selected {
		trailing = "Selected"
	}
	state.drawPage(canvas, theme, size, 560, state.Text("Item · 生产列表行", "Item · Production list rows"), func(c ui.C) {
		if state.exampleID == "default" {
			row := ui.Row(c).FillWidth().Pad(12).Gap(12).Items(ui.Center).Hit(itemRow.At(0), "Release notes")
			row.Children(func() {
				ui.Row(c).Size(24, 24).Radius(6).Background(theme.Primary)
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Release notes").Bold()
					ui.Text(c, "Latest runtime changes.").Muted()
				})
				ui.Text(c, trailing)
			})
			if row.Clicked() {
				state.flags ^= 1
			}
			return
		}
		ui.Row(c).FillWidth().Pad(12).Gap(12).Items(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "Outline")
				ui.Text(c, "Default density").Muted()
			})
			ui.Text(c, "⌘1")
		})
		ui.Row(c).FillWidth().Pad(8).Gap(8).Items(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "Muted")
				ui.Text(c, "Small density").Muted()
			})
			ui.Text(c, "⌘2")
		})
		ui.Row(c).FillWidth().Pad(6).Gap(6).Items(ui.Center).Disabled(true).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "Disabled")
			})
			ui.Text(c, "Unavailable").Muted()
		})
	})
}
