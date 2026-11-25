package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/components"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func openSelect(t *testing.T, Size *string, disabled []bool, options ...func(ui.Box) ui.Box) *pageDriver {
	t.Helper()
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			b := ui.Select(c, Size, []string{"S", "M", "L"}, nil).Placeholder("Size").DisabledOptions(disabled)
			for _, option := range options {
				b = option(b)
			}
		})
	})
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			if err := d.driver.Click(center(hit.Bounds)); err != nil {
				t.Fatal(err)
			}
			return d
		}
	}
	t.Fatal("no select trigger")
	return nil
}

// A disabled option is listed but cannot be chosen: a click on it picks nothing
// and leaves the list open, while its neighbours still pick.
func TestSelectDisabledOptionTakesNoClick(t *testing.T) {
	size := ""
	d := openSelect(t, &size, []bool{false, true, false})
	if err := d.driver.Click(center(d.hit("M").Bounds)); err != nil {
		t.Fatal(err)
	}
	if size != "" {
		t.Fatalf("a disabled option was chosen: %q", size)
	}
	if !d.has("L") {
		t.Fatal("the list closed on a click on a disabled option")
	}
	d.click("L")
	if size != "L" {
		t.Fatalf("an enabled option beside a disabled one was not chosen: %q", size)
	}
}

// The accessibility tree says which options are unavailable.
func TestSelectDisabledOptionIsReportedDisabled(t *testing.T) {
	size := ""
	d := openSelect(t, &size, []bool{false, true, false})
	if hit := d.hit("M"); !hit.Disabled {
		t.Fatal("the disabled option is not reported disabled")
	}
	if hit := d.hit("S"); hit.Disabled {
		t.Fatal("an enabled option is reported disabled")
	}
}

// A Select's Hint reaches assistive technology as the trigger's description.
func TestSelectHintIsTheTriggersHint(t *testing.T) {
	size := "M"
	d := openPage(t, page(func(c ui.C) { ui.Select(c, &size, []string{"S", "M"}, nil).Hint("Medium is the default") }))
	for _, h := range d.driver.Snapshot().Hits {
		if h.Role == gfx.RoleCombobox {
			if h.Hint != "Medium is the default" {
				t.Fatalf("hint = %q", h.Hint)
			}
			return
		}
	}
	t.Fatal("no select trigger")
}

// XSmall is a shorter trigger than the default, as a button's is.
func TestSelectXSmallIsShorterThanDefault(t *testing.T) {
	height := func(xsmall bool) float32 {
		size := ""
		d := openPage(t, func(c ui.C) {
			b := ui.Select(c, &size, []string{"S", "M"}, nil).Placeholder("Size")
			if xsmall {
				b.XSmall()
			}
		})
		for _, hit := range d.driver.Snapshot().Hits {
			if hit.Role == gfx.RoleCombobox {
				return hit.Bounds.Height
			}
		}
		t.Fatal("no select trigger")
		return 0
	}
	if normal, small := height(false), height(true); small >= normal {
		t.Fatalf("XSmall trigger is %v tall, the default %v", small, normal)
	}
}

// Declaring a Select with DisabledOptions allocates nothing once the page is warm.
func TestSelectDisabledOptionsDeclareWithoutAllocating(t *testing.T) {
	size := "M"
	disabled := []bool{false, true, false}
	choices := []string{"S", "M", "L"}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Select(c, &size, choices, nil).DisabledOptions(disabled).XSmall()
		})
	}
	drawPage(page, canvas, view)
	if got := testing.AllocsPerRun(50, func() { drawPage(page, canvas, view) }); got != 0 {
		t.Fatalf("a warm paint of a Select with DisabledOptions allocates %v times", got)
	}
}

// An Invalid Field is painted differently from a valid one.
func TestFieldInvalidChangesTheLook(t *testing.T) {
	look := func(invalid bool) string {
		var f ui.Field
		f.Invalid = invalid
		d := openPage(t, page(func(c ui.C) { ui.TextField(c, &f, "Name", "", ui.WithID(gfx.Item(5))) }))
		return fmt.Sprint(d.driver.Snapshot().Primitives)
	}
	if look(false) == look(true) {
		t.Fatal("an invalid field is painted like a valid one")
	}
}

// A FormField with an Error paints it, with For the title is the control's label, and the
// field keeps the height it is given.
func TestFormFieldErrorInvalidAndFor(t *testing.T) {
	var state ui.FormFieldState
	state.Error, state.Invalid, state.For = "Not valid.", true, gfx.Item(9)
	d := openPage(t, page(func(c ui.C) {
		ui.FormField(c, &state, "Name").FillWidth().Height(90).Children(func() {
			ui.TextField(c, new(ui.Field), "Name", "", ui.WithID(gfx.Item(9)))
		})
	}))
	snap := d.driver.Snapshot()
	var gotError, titleIsLabel bool
	for _, x := range snap.Texts {
		gotError = gotError || x.Value == "Not valid."
	}
	for _, h := range snap.Hits {
		titleIsLabel = titleIsLabel || (h.IsLabel && h.Label == "Name")
	}
	if !gotError {
		t.Fatal("the error line is not painted")
	}
	if !titleIsLabel {
		t.Fatal("For did not make the title the control's label")
	}
}

// A Disabled Field is painted dimmed, reported disabled and not focusable.
func TestFieldDisabledIsDimmedAndInert(t *testing.T) {
	look := func(disabled bool) (string, gfx.Hit) {
		var f ui.Field
		f.Disabled = disabled
		d := openPage(t, page(func(c ui.C) { ui.TextField(c, &f, "Name", "", ui.WithID(gfx.Item(5))) }))
		snap := d.driver.Snapshot()
		for _, h := range snap.Hits {
			if h.Role == gfx.RoleTextField {
				return fmt.Sprint(snap.Primitives), h
			}
		}
		t.Fatal("no field")
		return "", gfx.Hit{}
	}
	plain, enabled := look(false)
	dim, disabled := look(true)
	if plain == dim {
		t.Fatal("a disabled field is painted like an enabled one")
	}
	if enabled.Disabled || !disabled.Disabled || disabled.Focusable {
		t.Fatalf("hit: enabled.Disabled=%v disabled.Disabled=%v disabled.Focusable=%v", enabled.Disabled, disabled.Disabled, disabled.Focusable)
	}
}

var darkSaveRecipe = func(Palette gfx.Theme, Variant components.ButtonVariant, state gfx.State) components.ButtonLook {
	Palette.Primary, Palette.PrimaryForeground = gfx.RGB(24, 24, 27), gfx.RGB(250, 250, 250)
	return components.ButtonRecipe(Palette, Variant, state)
}

// A Button given its own recipe is painted with it; the others on the page keep the theme's.
func TestButtonRecipeDressesOnlyThatButton(t *testing.T) {
	d := openPage(t, page(func(c ui.C) {
		ui.Button(c, "Plain")
		ui.Button(c, "Dark").ButtonRecipe(darkSaveRecipe)
	}))
	fill := func(Label string) gfx.Color {
		var bounds gfx.Rect
		for _, h := range d.driver.Snapshot().Hits {
			if h.Label == Label && !h.IsLabel {
				bounds = h.Bounds
			}
		}
		for _, p := range d.driver.Snapshot().Primitives {
			if p.Bounds.X == bounds.X && p.Bounds.Y == bounds.Y && p.Bounds.Width == bounds.Width {
				return p.Color
			}
		}
		t.Fatalf("no fill for %s", Label)
		return gfx.Color{}
	}
	if fill("Plain") == fill("Dark") {
		t.Fatal("the recipe did not change the button's fill")
	}
	if got, want := fill("Dark"), gfx.RGB(24, 24, 27); got != want {
		t.Fatalf("dark button fill = %v, want %v", got, want)
	}
}

// Declaring a button with its own recipe allocates nothing once the page is warm.
func TestButtonRecipeDeclaresWithoutAllocating(t *testing.T) {
	page2, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.Button(c, "Save").ButtonRecipe(darkSaveRecipe) })
	}
	drawPage(page2, canvas, view)
	if got := testing.AllocsPerRun(50, func() { drawPage(page2, canvas, view) }); got != 0 {
		t.Fatalf("a warm paint of a button with its own recipe allocates %v times", got)
	}
}

// A FormField paints its Description, and a Field's Hint reaches assistive technology.
func TestFormFieldDescriptionAndFieldHint(t *testing.T) {
	var state ui.FormFieldState
	state.Description = "Locked while connected."
	var f ui.Field
	f.Hint = "Locked while connected."
	d := openPage(t, page(func(c ui.C) {
		ui.FormField(c, &state, "Port").FillWidth().Height(90).Children(func() {
			ui.TextField(c, &f, "Port", "", ui.WithID(gfx.Item(9)))
		})
	}))
	snap := d.driver.Snapshot()
	painted, hinted := false, false
	for _, x := range snap.Texts {
		painted = painted || x.Value == "Locked while connected."
	}
	for _, h := range snap.Hits {
		hinted = hinted || (h.Role == gfx.RoleTextField && h.Hint == "Locked while connected.")
	}
	if !painted {
		t.Fatal("the description is not painted")
	}
	if !hinted {
		t.Fatal("the field's hint did not reach its hit")
	}
}
