package flex_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// screenDriver opens a headless window that paints body with a Screen.
type screenDriver struct {
	t           *testing.T
	driver      *app.Driver
	screen      *flex.Screen
	invalidator *app.Invalidator
}

func openScreen(t *testing.T, body func(flex.Screen)) *screenDriver {
	t.Helper()
	return openScreenWith(t, flex.ScreenConfig{Body: body}, app.Config{})
}

func openScreenWith(t *testing.T, config flex.ScreenConfig, base app.Config) *screenDriver {
	t.Helper()
	screen := flex.NewScreen(config)
	invalidator := new(app.Invalidator)
	base.Invalidator = invalidator
	if base.Width == 0 {
		base.Width, base.Height = 480, 360
	}
	driver, err := app.NewDriver(screen.Config(base), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return &screenDriver{t: t, driver: driver, screen: screen, invalidator: invalidator}
}

// repaint paints again after the model changed outside an event, as background
// work published through an Invalidator does.
func (d *screenDriver) repaint() {
	d.t.Helper()
	d.invalidator.Invalidate()
	if err := d.driver.Pump(); err != nil {
		d.t.Fatal(err)
	}
}

// hit finds the control named label (its accessible name), and fails without one.
func (d *screenDriver) hit(label string) ui.Hit {
	d.t.Helper()
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == label && !hit.IsLabel && !hit.Backdrop && !hit.ID.IsZero() {
			return hit
		}
	}
	d.t.Fatalf("no control labeled %q", label)
	return ui.Hit{}
}

func (d *screenDriver) has(label string) bool {
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == label && !hit.IsLabel && !hit.Backdrop {
			return true
		}
	}
	return false
}

func center(r ui.Rect) (float32, float32) { return r.X + r.Width/2, r.Y + r.Height/2 }

func (d *screenDriver) click(label string) {
	d.t.Helper()
	if err := d.driver.Click(center(d.hit(label).Bounds)); err != nil {
		d.t.Fatal(err)
	}
}

func (d *screenDriver) key(key ui.Key) {
	d.t.Helper()
	if err := d.driver.Key(key, 0); err != nil {
		d.t.Fatal(err)
	}
}

func (d *screenDriver) typeText(value string) {
	d.t.Helper()
	if err := d.driver.Type(value); err != nil {
		d.t.Fatal(err)
	}
}

func (d *screenDriver) focused() string {
	snapshot := d.driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == snapshot.Focused && !hit.IsLabel && !hit.Backdrop {
			return hit.Label
		}
	}
	return ""
}

func TestScreenBindsControlsAndNamesThemWithoutATree(t *testing.T) {
	var done bool
	var mode, clicks int
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Fill().Pad(16).Gap(8).Children(func() {
			s.Checkbox(&done, "Done")
			s.Choice(&mode, []string{"One", "Two", "Three"})
			if s.Button("Count").Clicked() {
				clicks++
			}
		})
	})
	d.click("Done")
	d.click("Three")
	d.click("Count")
	d.click("Count")
	if !done || mode != 2 || clicks != 2 {
		t.Fatalf("done=%v mode=%d clicks=%d, want true, 2 and 2", done, mode, clicks)
	}
	if path := d.driver.Snapshot().Tree.Text(d.hit("Count").ID); path != "screen/~button/#0" {
		t.Errorf("the button is named %q, want screen/~button/#0", path)
	}
}

func TestScreenCaptionClicksTheCheckbox(t *testing.T) {
	var sync bool
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() { s.Switch(&sync, "Sync over SSH") })
	})
	var caption ui.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.IsLabel && hit.Label == "Sync over SSH" {
			caption = hit
		}
	}
	if caption.ID.IsZero() {
		t.Fatal("the caption is not a label of the switch")
	}
	if err := d.driver.Click(center(caption.Bounds)); err != nil {
		t.Fatal(err)
	}
	if !sync {
		t.Fatal("a click on the caption did not flip the switch")
	}
}

// The password keeps its identity, its focus and its typing, when a field appears
// before it: a numbered field would hand them to the new one, and the rest of the
// password would be typed into a field that shows it.
func TestScreenFieldAppearingAboveKeepsTheTyping(t *testing.T) {
	var otp, password flex.Field
	password.Secure = true
	var needOTP bool
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Gap(8).Children(func() {
			if needOTP {
				s.Field(&otp, "Code", "")
			}
			s.Field(&password, "Password", "")
		})
	})
	d.click("Password")
	d.typeText("ab")
	needOTP = true
	d.repaint()
	d.typeText("cd")
	if password.Text() != "abcd" || otp.Text() != "" {
		t.Fatalf("password %q and code %q, want abcd and nothing", password.Text(), otp.Text())
	}
	if d.focused() != "Password" {
		t.Fatalf("focus is on %q, want the password", d.focused())
	}
}

type host struct {
	name    string
	handled bool
}

// Filtering the list keeps each checkbox's identity on its host: the focused one
// stays with it, and the key that follows toggles the same host.
func TestScreenFilterKeepsFocusOnTheSameRow(t *testing.T) {
	hosts := []host{{name: "a"}, {name: "b"}, {name: "c"}}
	hide := ""
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Gap(4).Children(func() {
			for i := range hosts {
				if h := &hosts[i]; h.name != hide {
					s.Checkbox(&h.handled, h.name)
				}
			}
		})
	})
	d.click("c")
	if !hosts[2].handled || d.focused() != "c" {
		t.Fatalf("clicking c: handled=%v focus=%q", hosts[2].handled, d.focused())
	}
	hide = "a"
	d.repaint()
	if d.focused() != "c" {
		t.Fatalf("after the filter the focus is on %q, want c", d.focused())
	}
	d.key(ui.KeySpace)
	if hosts[2].handled || hosts[1].handled {
		t.Fatalf("Space toggled the wrong row: %+v", hosts)
	}
}

// A press held on a numbered button whose number moves to another button (one was
// added above it) is not delivered: neither button runs.
func TestScreenHeldButtonThatMovedIsNotClicked(t *testing.T) {
	labels := []string{"Alpha", "Beta"}
	clicked := map[string]int{}
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Gap(4).Children(func() {
			for _, label := range labels {
				if s.Button(label).Clicked() {
					clicked[label]++
				}
			}
		})
	})
	x, y := center(d.hit("Beta").Bounds)
	if err := d.driver.Press(x, y, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	labels = []string{"New", "Alpha", "Beta"}
	d.repaint()
	if err := d.driver.Release(x, y, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 0 {
		t.Fatalf("a press that moved to another button clicked %v", clicked)
	}
	d.click("Beta")
	if clicked["Beta"] != 1 || len(clicked) != 1 {
		t.Fatalf("a fresh click on Beta clicked %v", clicked)
	}
}

// A button whose label changes with each click (a counter) keeps taking the keyboard.
func TestScreenFocusedButtonWithChangingLabelKeepsClicking(t *testing.T) {
	count := 0
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() {
			if s.Button(strings.Repeat("|", count+1)).Clicked() {
				count++
			}
		})
	})
	d.click("|")
	d.key(ui.KeyEnter)
	d.key(ui.KeyEnter)
	if count != 3 {
		t.Fatalf("count = %d after a click and two Enters, want 3", count)
	}
}

func TestScreenKeyRowsKeepFocusAcrossReorder(t *testing.T) {
	names := []string{"a", "b", "c"}
	opened := ""
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Gap(4).Children(func() {
			for _, name := range names {
				r := s.Key(name)
				r.Row().Gap(8).Children(func() {
					r.Text(name)
					if r.Button("Open " + name).Clicked() {
						opened = name
					}
				})
			}
		})
	})
	d.click("Open b")
	names = []string{"c", "b", "a"}
	d.repaint()
	if d.focused() != "Open b" {
		t.Fatalf("focus moved to %q after reordering, want Open b", d.focused())
	}
	d.key(ui.KeyEnter)
	if opened != "b" {
		t.Fatalf("Enter opened %q, want b", opened)
	}
}

func TestScreenDuplicateKeyPanics(t *testing.T) {
	screen := flex.NewScreen(flex.ScreenConfig{})
	defer func() {
		if recover() == nil {
			t.Fatal("two rows with one key did not panic")
		}
	}()
	screen.Draw(&ui.Canvas{}, ui.Rect{Width: 100, Height: 100}, func(s flex.Screen) {
		s.Key("x").Text("one")
		s.Key("x").Text("two")
	})
}

func TestScreenReservedNamesPanic(t *testing.T) {
	for name, declare := range map[string]func(s flex.Screen){
		"ID":         func(s flex.Screen) { s.ID("~button") },
		"Scoped":     func(s flex.Screen) { s.Scoped("~x", func(flex.Screen) {}) },
		"ScrollArea": func(s flex.Screen) { s.ScrollArea("~list") },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s with a '~' name did not panic", name)
				}
			}()
			flex.NewScreen(flex.ScreenConfig{}).Draw(&ui.Canvas{}, ui.Rect{Width: 100, Height: 100}, declare)
		})
	}
}

func TestScreenDisabledAfterDeclarationIsNotWritten(t *testing.T) {
	var on bool
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() { s.Checkbox(&on, "On").Disabled(true) })
	})
	if err := d.driver.Click(center(d.hit("On").Bounds)); err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("a disabled checkbox was written")
	}
}

func TestScreenChangedReportsTheClickOnce(t *testing.T) {
	var on bool
	changes := 0
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() {
			if s.Checkbox(&on, "On").Changed() {
				changes++
			}
		})
	})
	d.click("On")
	d.repaint()
	if !on || changes != 1 {
		t.Fatalf("on=%v changes=%d, want true and 1", on, changes)
	}
}

func TestScreenFieldChangedReportsAnEditNotSetText(t *testing.T) {
	var query flex.Field
	edits := 0
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() {
			if s.Field(&query, "Search", "").Changed() {
				edits++
			}
		})
	})
	d.click("Search")
	d.typeText("x")
	if edits != 1 {
		t.Fatalf("edits = %d after typing, want 1", edits)
	}
	query.SetText("reset")
	d.repaint()
	if edits != 1 {
		t.Fatalf("SetText counted as an edit: %d", edits)
	}
}

func TestScreenConfigChainsAndKeepsTheRest(t *testing.T) {
	var tree ui.Tree
	terminal := &app.TextInput{}
	baseScrolls := 0
	base := app.Config{
		Title: "kept", Tree: &tree,
		TextInput: func(app.TextInputContext) *app.TextInput { return terminal },
		OnScroll: func(ui.ScrollEvent) ui.EventResult {
			baseScrolls++
			return ui.EventResult{Handled: true}
		},
	}
	var field flex.Field
	screen := flex.NewScreen(flex.ScreenConfig{Body: func(s flex.Screen) {
		s.Column().Pad(16).Children(func() { s.Field(&field, "Name", "") })
	}})
	config := screen.Config(base)
	if config.Tree != &tree || config.Title != "kept" {
		t.Fatalf("Config did not adopt the base's tree and keep the title: %+v", config)
	}
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var fieldID ui.ID
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Name" {
			fieldID = hit.ID
		}
	}
	if input := config.TextInput(app.TextInputContext{ID: fieldID}); input == nil || input == terminal {
		t.Fatal("the Screen did not answer for its own field")
	}
	if input := config.TextInput(app.TextInputContext{ID: tree.ID(tree.Feature("terminal"), "pty")}); input != terminal {
		t.Fatal("a field the Screen does not own did not reach the base provider")
	}
	if config.OnScroll(ui.ScrollEvent{DeltaY: 10}); baseScrolls != 1 {
		t.Fatalf("a scroll the page does not take reached the base %d times, want 1", baseScrolls)
	}

	other := flex.NewScreen(flex.ScreenConfig{Tree: new(ui.Tree), Body: func(flex.Screen) {}})
	defer func() {
		if recover() == nil {
			t.Fatal("a Config with another Tree did not panic")
		}
	}()
	other.Config(base)
}

func TestScreenHiddenFieldGivesNoEditor(t *testing.T) {
	var field flex.Field
	shown := true
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() {
			if shown {
				s.Field(&field, "Name", "")
			}
		})
	})
	id := d.hit("Name").ID
	shown = false
	d.repaint()
	if input := d.screen.TextInput(app.TextInputContext{ID: id}); input != nil {
		t.Fatal("a field the last paint did not declare still gives its editor")
	}
}

func TestScreenSameFieldInTwoWindows(t *testing.T) {
	var shared flex.Field
	body := func(s flex.Screen) {
		s.Column().Pad(16).Children(func() { s.Field(&shared, "Shared", "") })
	}
	first, second := openScreen(t, body), openScreen(t, body)
	first.click("Shared")
	first.typeText("a")
	second.click("Shared")
	second.typeText("b")
	if shared.Text() != "ab" {
		t.Fatalf("text = %q, want what both windows typed", shared.Text())
	}
}

func TestScreenPopoverOpensClosesAndGivesFocusBack(t *testing.T) {
	var open bool
	other := 0
	d := openScreen(t, func(s flex.Screen) {
		s.Row().Pad(16).Gap(8).Children(func() {
			more := s.Button("More")
			if more.Clicked() {
				open = !open
			}
			if s.Button("Other").Clicked() {
				other++
			}
			s.Popover(more, &open, func(p flex.Screen) { p.Button("Rename") })
		})
	})
	d.click("More")
	if !open || !d.has("Rename") {
		t.Fatal("the popover did not open")
	}
	d.click("Rename")
	d.key(ui.KeyEscape)
	if open {
		t.Fatal("Escape did not close the popover")
	}
	if d.focused() != "More" {
		t.Fatalf("focus after Escape is on %q, want the trigger", d.focused())
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

func TestScreenModalTrapsFocusAndCloses(t *testing.T) {
	var open bool
	confirmed := 0
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() {
			if s.Button("Delete…").Clicked() {
				open = true
			}
			s.Modal(&open, "Delete host?", func(m flex.ModalScreen) {
				m.Text("This cannot be undone.")
				m.Footer(func() {
					if m.Button("Confirm").Clicked() {
						confirmed++
						open = false
					}
				})
			})
		})
	})
	d.click("Delete…")
	if !open || d.focused() != "Confirm" {
		t.Fatalf("open=%v focus=%q, want the dialog open with its first control focused", open, d.focused())
	}
	d.key(ui.KeyEscape)
	if open {
		t.Fatal("Escape did not close the modal")
	}
	if d.focused() != "Delete…" {
		t.Fatalf("focus after closing is on %q, want the button that opened it", d.focused())
	}
	d.click("Delete…")
	d.key(ui.KeyEnter)
	if confirmed != 1 || open {
		t.Fatalf("confirmed=%d open=%v after Enter on the focused button", confirmed, open)
	}
}

func TestScreenModalNeedsATitle(t *testing.T) {
	open := true
	defer func() {
		if recover() == nil {
			t.Fatal("a Modal without a title did not panic")
		}
	}()
	flex.NewScreen(flex.ScreenConfig{}).Draw(&ui.Canvas{}, ui.Rect{Width: 300, Height: 300}, func(s flex.Screen) {
		s.Modal(&open, "", func(flex.ModalScreen) {})
	})
}

func TestScreenDescribeShowsKeys(t *testing.T) {
	screen := flex.NewScreen(flex.ScreenConfig{})
	var id ui.ID
	screen.Draw(&ui.Canvas{}, ui.Rect{Width: 300, Height: 100}, func(s flex.Screen) {
		s.Key("edge-00").Text("first")
		id = s.Key("edge-01").ID("go")
	})
	if got := screen.Describe(id); got != `screen/~key/"edge-01"/go` {
		t.Fatalf("Describe = %q", got)
	}
}

// Under a Driver, which is how an application is tested, a mistake the release
// build only reports fails the test where it happens, with the fix in the message.
func TestScreenMistakesFailUnderADriver(t *testing.T) {
	defer func() {
		r := recover()
		message, _ := r.(string)
		if !strings.Contains(message, "have no Key") || !strings.Contains(message, "s.Key(") {
			t.Fatalf("recover() = %v, want the unkeyed rows reported with the fix", r)
		}
	}()
	openScreen(t, func(s flex.Screen) {
		s.Column().Children(func() {
			for range 2 {
				s.Button("Delete")
			}
		})
	})
}

func TestScreenCopyBoundFailsWhenClickedUnderADriver(t *testing.T) {
	model := map[string]bool{"a": false}
	d := openScreen(t, func(s flex.Screen) {
		s.Column().Pad(16).Children(func() {
			done := model["a"]
			s.Checkbox(&done, "A")
		})
	})
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		return d.driver.Click(center(d.hit("A").Bounds))
	}()
	if err == nil || !strings.Contains(err.Error(), "new address in every paint") {
		t.Fatalf("clicking a copy: %v, want the copy reported", err)
	}
}
