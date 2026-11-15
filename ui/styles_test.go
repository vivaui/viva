package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// A page's Styles replace the recipe each styled control paints with, and a
// nil field keeps the theme's: the application keeps using Button, Checkbox,
// Switch and TextField instead of a Base at every call site.
func TestStylesReplaceThePageRecipes(t *testing.T) {
	theme := gfx.LightTheme()
	buttonColor := gfx.RGB(1, 2, 3)
	checkboxColor := gfx.RGB(4, 5, 6)
	switchColor := gfx.RGB(7, 8, 9)
	entryColor := gfx.RGB(10, 11, 12)
	styles := &Styles{
		Button: func(gfx.Theme, components.ButtonVariant, gfx.State) components.ButtonLook {
			return components.ButtonLook{Background: buttonColor, Foreground: gfx.RGB(255, 255, 255), Filled: true}
		},
		Checkbox: func(gfx.Theme, gfx.State) components.CheckboxLook {
			return components.CheckboxLook{Fill: checkboxColor, Border: checkboxColor, Mark: checkboxColor}
		},
		Switch: func(gfx.Theme, gfx.State, float32) components.SwitchLook {
			return components.SwitchLook{Track: switchColor, Thumb: switchColor}
		},
		TextEntry: func(gfx.Theme, gfx.State) components.TextEntryLook {
			return components.TextEntryLook{Fill: entryColor, Border: entryColor, BorderWidth: 1, Radius: 3}
		},
	}
	page := NewPage(PageConfig{Theme: &theme, Styles: styles})
	canvas := testCanvas()
	var on bool
	var field Field
	page.Draw(canvas, gfx.Rect{Width: 320, Height: 200}, func(c C) {
		Column(c).Pad(8).Gap(8).Children(func() {
			Button(c, "Save")
			Checkbox(c, &on, "Remember")
			Switch(c, &on, "Sync")
			TextFieldID(c, tree.ID(tree.Feature("styles"), "field"), &field, "Name", "")
		})
	})
	painted := make(map[gfx.Color]bool)
	for _, rectangle := range canvas.Rectangles() {
		painted[rectangle.Color] = true
	}
	for name, color := range map[string]gfx.Color{
		"button": buttonColor, "checkbox": checkboxColor, "switch": switchColor, "text entry": entryColor,
	} {
		if !painted[color] {
			t.Errorf("the %s did not paint its recipe's color; painted %d rectangles", name, len(canvas.Rectangles()))
		}
	}
}

// Without Styles the page paints the theme's recipe, not a zero look.
func TestAPageWithoutStylesPaintsTheThemeRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	page.Draw(canvas, gfx.Rect{Width: 200, Height: 100}, func(c C) {
		Column(c).Pad(8).Children(func() {
			Button(c, "Save")
		})
	})
	found := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == theme.Primary {
			found = true
		}
	}
	if !found {
		t.Fatalf("the default button did not paint the theme's primary color: %d rectangles", len(canvas.Rectangles()))
	}
}
