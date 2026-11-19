package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// menuShapeCase paints one of the three menus that share menuSurface. They all
// expose the same row height and shadow options, so the same checks apply to
// each one.
type menuShapeCase struct {
	name  string
	paint func(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, submenu bool, rowHeight float32, shadow *gfx.ShadowLevel) gfx.Rect
}

var menuShapeCases = []menuShapeCase{
	{"dropdown", func(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, submenu bool, rowHeight float32, shadow *gfx.ShadowLevel) gfx.Rect {
		menu := BeginDropdownMenu(canvas, theme, bounds, DropdownMenuOptions{Layer: 3, Submenu: submenu, RowHeight: rowHeight, Shadow: shadow})
		return menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})
	}},
	{"context menu", func(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, submenu bool, rowHeight float32, shadow *gfx.ShadowLevel) gfx.Rect {
		menu := BeginContextMenu(canvas, theme, bounds, ContextMenuOptions{Layer: 3, Submenu: submenu, RowHeight: rowHeight, Shadow: shadow})
		return menu.AddItem(ContextMenuItem{ID: gfx.Item(1), Label: "Open"})
	}},
	{"menubar popup", func(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, submenu bool, rowHeight float32, shadow *gfx.ShadowLevel) gfx.Rect {
		content := BeginMenubarContent(canvas, theme, bounds, MenubarOverlayOptions{Layer: 3, Submenu: submenu, RowHeight: rowHeight, Shadow: shadow})
		return content.AddItem(MenubarItem{ID: gfx.Item(1), Label: "Open"})
	}},
}

// The dropdown menu, the context menu and the menubar's popup share menuSurface,
// so the row height and shadow options must reach every one of them.
func TestMenuShapesShareRowHeightAndShadowOptions(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 20, Width: 160, Height: 120}
	lg := gfx.ShadowLG
	for _, shape := range menuShapeCases {
		t.Run(shape.name+"/options", func(t *testing.T) {
			var canvas gfx.Canvas
			row := shape.paint(&canvas, theme, bounds, false, 24, &lg)
			if row.Height != 24 {
				t.Fatalf("row = %+v, want 24 high", row)
			}
			if got, want := menuShadowStyles(canvas.Shadows()), elevatedShadowStyles(theme, bounds, lg); !slices.Equal(got, want) {
				t.Fatalf("shadow = %+v, want shadow-lg %+v", got, want)
			}
		})
		t.Run(shape.name+"/defaults", func(t *testing.T) {
			var canvas gfx.Canvas
			row := shape.paint(&canvas, theme, bounds, false, 0, nil)
			if row.Height != theme.Metrics.ControlHeight {
				t.Fatalf("row = %+v, want the theme's %v", row, theme.Metrics.ControlHeight)
			}
			if got, want := menuShadowStyles(canvas.Shadows()), elevatedShadowStyles(theme, bounds, gfx.ShadowMD); !slices.Equal(got, want) {
				t.Fatalf("default shadow = %+v, want shadow-md %+v", got, want)
			}
		})
		t.Run(shape.name+"/submenu defaults", func(t *testing.T) {
			var canvas gfx.Canvas
			row := shape.paint(&canvas, theme, bounds, true, 0, nil)
			if row.Height != theme.Metrics.ControlHeight {
				t.Fatalf("row = %+v, want the theme's %v", row, theme.Metrics.ControlHeight)
			}
			if got, want := menuShadowStyles(canvas.Shadows()), elevatedShadowStyles(theme, bounds, gfx.ShadowLG); !slices.Equal(got, want) {
				t.Fatalf("submenu default shadow = %+v, want shadow-lg %+v", got, want)
			}
		})
	}
}

// A shadow level given to the menu is used as is: no shadow at all, the default
// shadow-md, or the stronger shadow-lg. It also wins over the submenu's own
// shadow-lg, so a caller can flatten a submenu as well.
func TestDropdownMenuShadowOptionChoosesTheElevation(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 20, Width: 160, Height: 120}
	for _, test := range []struct {
		name  string
		level gfx.ShadowLevel
	}{
		{"none", gfx.ShadowNone},
		{"md", gfx.ShadowMD},
		{"lg", gfx.ShadowLG},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			level := test.level
			BeginDropdownMenu(&canvas, theme, bounds, DropdownMenuOptions{Layer: 3, Shadow: &level})
			got, want := menuShadowStyles(canvas.Shadows()), elevatedShadowStyles(theme, bounds, test.level)
			if !slices.Equal(got, want) {
				t.Fatalf("shadow = %+v, want %v %+v", got, test.level, want)
			}
		})
	}
	t.Run("submenu", func(t *testing.T) {
		var canvas gfx.Canvas
		none := gfx.ShadowNone
		BeginDropdownMenu(&canvas, theme, bounds, DropdownMenuOptions{Layer: 3, Submenu: true, Shadow: &none})
		if shadows := canvas.Shadows(); len(shadows) != 0 {
			t.Fatalf("a flat submenu cast %+v", shadows)
		}
	})
}

// A row height given to the menu sizes its rows, while a described row still
// grows to fit both of its lines.
func TestDropdownMenuRowHeightOptionSizesRows(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 160, Height: 200}, DropdownMenuOptions{Layer: 3, RowHeight: 24})
	if next := menu.NextItemBounds(); next.Height != 24 {
		t.Fatalf("next item = %+v, want 24 high", next)
	}
	plain := menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})
	if plain.Height != 24 {
		t.Fatalf("plain row = %+v, want 24 high", plain)
	}
	described := menu.AddItem(DropdownMenuItem{ID: gfx.Item(2), Label: "Model", Description: "What answers"})
	if described.Height <= 24 {
		t.Fatalf("a described row is %v tall, want more than the 24px row height", described.Height)
	}
	for _, hit := range nonBlockHits(canvas.Hits()) {
		if hit.ID == gfx.Item(1) && hit.Bounds.Height != 24 {
			t.Fatalf("hit = %+v, want a 24px row", hit)
		}
	}
}

// menuShadowStyles drops the geometry and order of the shadow primitives, which
// say where a surface was painted rather than how high it floats.
func menuShadowStyles(shadows []gfx.Shadow) []gfx.ShadowStyle {
	styles := make([]gfx.ShadowStyle, len(shadows))
	for index, shadow := range shadows {
		styles[index] = shadow.Style
	}
	return styles
}

// elevatedShadowStyles is the shadow a popup surface casts at level, painted on
// a canvas of its own. Comparing against it keeps the test free of the shadow
// layer numbers.
func elevatedShadowStyles(theme gfx.Theme, bounds gfx.Rect, level gfx.ShadowLevel) []gfx.ShadowStyle {
	var reference gfx.Canvas
	reference.Elevate(bounds, theme.Metrics.RadiusPopup(), level)
	return menuShadowStyles(reference.Shadows())
}
