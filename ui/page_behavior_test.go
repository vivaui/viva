package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestCaptionClicksTheSwitch(t *testing.T) {
	var sync bool
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.Switch(c, &sync, "Sync over SSH") })
	})
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.IsLabel && hit.Label == "Sync over SSH" {
			if err := d.driver.Click(center(hit.Bounds)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !sync {
		t.Fatal("a click on the caption did not flip the switch")
	}
}

type host struct {
	name    string
	handled bool
}

// Filtering the list keeps each checkbox's name on its host: the focused one stays
// with it, and the key that follows toggles the same host.
func TestFilterKeepsFocusOnTheSameRow(t *testing.T) {
	hosts := []host{{name: "a"}, {name: "b"}, {name: "c"}}
	hide := ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(4).Children(func() {
			for i := range hosts {
				if h := &hosts[i]; h.name != hide {
					ui.Checkbox(c, &h.handled, h.name)
				}
			}
		})
	})
	d.click("c")
	hide = "a"
	d.repaint()
	if d.focused() != "c" {
		t.Fatalf("after the filter the focus is on %q, want c", d.focused())
	}
	d.key(gfx.KeySpace)
	if hosts[2].handled || hosts[1].handled {
		t.Fatalf("Space toggled the wrong row: %+v", hosts)
	}
}

// A press held on a button whose place moves to another button (one was added
// above it) is not delivered: neither button runs.
func TestHeldButtonThatMovedIsNotClicked(t *testing.T) {
	labels := []string{"Alpha", "Beta"}
	clicked := map[string]int{}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(4).Children(func() {
			for _, label := range labels {
				if ui.Button(c, label).Clicked() {
					clicked[label]++
				}
			}
		})
	})
	x, y := center(d.hit("Beta").Bounds)
	if err := d.driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	labels = []string{"New", "Alpha", "Beta"}
	d.repaint()
	if err := d.driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 0 {
		t.Fatalf("a press that moved to another button clicked %v", clicked)
	}
}

// A button whose label changes with each click (a counter) keeps taking the keyboard.
func TestFocusedButtonWithChangingLabelKeepsClicking(t *testing.T) {
	count := 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			if ui.Button(c, strings.Repeat("|", count+1)).Clicked() {
				count++
			}
		})
	})
	d.click("|")
	d.key(gfx.KeyEnter)
	d.key(gfx.KeyEnter)
	if count != 3 {
		t.Fatalf("count = %d after a click and two Enters, want 3", count)
	}
}

func TestDuplicateKeyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("two rows with one key did not panic")
		}
	}()
	drawPage(ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas(), func(c ui.C) {
		ui.Row(c.Key("x"))
		ui.Row(c.Key("x"))
	})
}

func TestReservedNamesPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a part named with '~' did not panic")
		}
	}()
	drawPage(ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas(), func(c ui.C) {
		ui.Row(c.Part("~k0"))
	})
}

func TestDisabledAfterDeclarationIsNotWritten(t *testing.T) {
	var on bool
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.Checkbox(c, &on, "On").Disabled(true) })
	})
	if err := d.driver.Click(center(d.hit("On").Bounds)); err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("a disabled checkbox was written")
	}
}

func TestChangedReportsTheClickOnce(t *testing.T) {
	var on bool
	theme, changes := "light", 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			if ui.Checkbox(c, &on, "On").Changed() {
				changes++
			}
			if ui.Choice(c, &theme, []string{"light", "dark"}, nil).Changed() {
				changes += 10
			}
		})
	})
	d.click("On")
	d.click("dark")
	d.click("dark") // already chosen: no change
	d.repaint()
	if !on || theme != "dark" || changes != 11 {
		t.Fatalf("on=%v theme=%q changes=%d, want true, dark and 11", on, theme, changes)
	}
}

func TestFieldChangedReportsAnEditNotSetText(t *testing.T) {
	var query ui.Field
	edits := 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			if ui.TextField(c, &query, "Search", "").Changed() {
				edits++
			}
		})
	})
	d.click("Search")
	d.typeText("x")
	query.SetText("reset")
	d.repaint()
	if edits != 1 {
		t.Fatalf("edits = %d, want 1 (typing, not SetText)", edits)
	}
}

func TestConfigChainsAndKeepsTheRest(t *testing.T) {
	var tree gfx.Tree
	terminal := &app.TextInput{}
	baseScrolls := 0
	base := app.Config{
		Title: "kept", Tree: &tree,
		TextInput: func(app.TextInputContext) *app.TextInput { return terminal },
		OnScroll: func(gfx.ScrollEvent) gfx.EventResult {
			baseScrolls++
			return gfx.EventResult{Handled: true}
		},
	}
	var field ui.Field
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.TextField(c, &field, "Name", "") })
	}})
	config := page.Config(base)
	if config.Tree != &tree || config.Title != "kept" {
		t.Fatal("Config did not adopt the base's tree and keep the title")
	}
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var fieldID gfx.ID
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Name" {
			fieldID = hit.ID
		}
	}
	if input := config.TextInput(app.TextInputContext{ID: fieldID}); input == nil || input == terminal {
		t.Fatal("the page did not answer for its own field")
	}
	if input := config.TextInput(app.TextInputContext{ID: tree.ID(tree.Feature("terminal"), "pty")}); input != terminal {
		t.Fatal("a field the page does not own did not reach the base provider")
	}
	if config.OnScroll(gfx.ScrollEvent{DeltaY: 10}); baseScrolls != 1 {
		t.Fatalf("a scroll the page does not take reached the base %d times, want 1", baseScrolls)
	}
	other := ui.NewPage(ui.PageConfig{Tree: new(gfx.Tree), View: func(ui.C) {}})
	defer func() {
		if recover() == nil {
			t.Fatal("a Config with another Tree did not panic")
		}
	}()
	other.Config(base)
}

func TestHiddenFieldGivesNoEditor(t *testing.T) {
	var field ui.Field
	shown := true
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			if shown {
				ui.TextField(c, &field, "Name", "")
			}
		})
	})
	id := d.hit("Name").ID
	shown = false
	d.repaint()
	if input := d.page.TextInput(app.TextInputContext{ID: id}); input != nil {
		t.Fatal("a field the last paint did not declare still gives its editor")
	}
}

func TestSameFieldInTwoWindows(t *testing.T) {
	var shared ui.Field
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.TextField(c, &shared, "Shared", "") })
	}
	first, second := openPage(t, view), openPage(t, view)
	first.click("Shared")
	first.typeText("a")
	second.click("Shared")
	second.typeText("b")
	if shared.Text() != "ab" {
		t.Fatalf("text = %q, want what both windows typed", shared.Text())
	}
}

func TestPopoverClosesByEscapeAnchorAndOutside(t *testing.T) {
	var open bool
	other := 0
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Pad(16).Gap(8).Children(func() {
			more := ui.Button(c, "More")
			if more.Clicked() {
				open = !open
			}
			if ui.Button(c, "Other").Clicked() {
				other++
			}
			ui.Popover(c, more, &open, func() { ui.Button(c, "Rename") })
		})
	})
	d.click("More")
	d.click("Rename")
	d.key(gfx.KeyEscape)
	if open || d.focused() != "More" {
		t.Fatalf("after Escape open=%v focus=%q, want closed with the trigger focused", open, d.focused())
	}
	d.click("More")
	d.click("More")
	if open {
		t.Fatal("a press on the anchor closed the popover and opened it again")
	}
	d.click("More")
	d.click("Other")
	if open || other != 1 {
		t.Fatalf("a press outside: open=%v other=%d, want closed and the press passed on", open, other)
	}
}

func TestModalNeedsATitle(t *testing.T) {
	open := true
	defer func() {
		if recover() == nil {
			t.Fatal("a Modal without a title did not panic")
		}
	}()
	drawPage(ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas(), func(c ui.C) {
		ui.Modal(c, &open, "", func() {}, nil)
	})
}

func TestTabsAndSelectWriteTheirValue(t *testing.T) {
	tab, size := "General", ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Tabs(c, &tab, []string{"General", "Keys"}, nil)
			ui.Select(c, &size, []string{"S", "M", "L"}, nil).Placeholder("Size")
		})
	})
	d.click("Keys")
	if tab != "Keys" {
		t.Fatalf("tab = %q", tab)
	}
	var trigger gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			trigger = hit
		}
	}
	if err := d.driver.Click(center(trigger.Bounds)); err != nil {
		t.Fatal(err)
	}
	d.click("L")
	if size != "L" {
		t.Fatalf("size = %q", size)
	}
}

// Rows keyed by key come and go, and the Tree grows with the rows on screen at
// once, not with every key the page has seen.
func TestKeysKeepTheTreeBounded(t *testing.T) {
	var tree gfx.Tree
	canvas := benchCanvas()
	canvas.SetTree(&tree)
	page := ui.NewPage(ui.PageConfig{Tree: &tree, Theme: &pageTheme})
	n := 0
	view := func(c ui.C) {
		for i := range 10 {
			ui.Row(c.KeyInt(n*10 + i)).Children(func() { ui.Button(c, "Open") })
		}
	}
	for n = range 50 {
		drawPage(page, canvas, view)
	}
	settled := tree.Len()
	for n = 50; n < 5_000; n++ {
		drawPage(page, canvas, view)
	}
	if tree.Len() != settled || settled > 100 {
		t.Fatalf("the tree has %d places, %d after 50 pages", tree.Len(), settled)
	}
}

// A native view is declared where its element is painted: inside a Modal it is on
// the dialog's layer, inside a scroll area it is cut to the area, and the page's
// popovers go over it.
func TestNativeViewTakesItsPlaceInThePage(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	open, menu := true, true
	drawPage(page, canvas, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			anchor := ui.Button(c, "Menu")
			ui.Scroll(c).Height(100).Children(func() {
				ui.Slot(c).Height(300).FillWidth().NativeView(1)
			})
			ui.Popover(c, anchor, &menu, func() { ui.Text(c, "over the page") })
			ui.Modal(c, &open, "Preview", func() { ui.Slot(c).Size(100, 80).NativeView(2) }, nil)
		})
	})
	var inPage, inDialog gfx.NativeView
	for _, view := range canvas.NativeViews() {
		switch view.ID {
		case 1:
			inPage = view
		case 2:
			inDialog = view
		}
	}
	if !inPage.HasClip || inPage.ClipOf.Height > 100.5 {
		t.Errorf("the view in a scroll area is clipped to %+v", inPage.ClipOf)
	}
	if inDialog.Layer <= inPage.Layer {
		t.Errorf("the view in the dialog is on layer %d, the page's on %d", inDialog.Layer, inPage.Layer)
	}
	for _, text := range canvas.Texts() {
		if text.Value == "over the page" && text.Layer <= inPage.Layer {
			t.Errorf("the popover is on layer %d, not over the view on %d", text.Layer, inPage.Layer)
		}
	}
}

func TestDeclaringAfterEndPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "after the page was laid out") {
			t.Fatalf("declaring in a Draw callback: recover() = %v", r)
		}
	}()
	drawPage(ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas(), func(c ui.C) {
		ui.Slot(c).Size(10, 10).Draw(func(*ui.DrawContext) { ui.Text(c, "late") })
	})
}
