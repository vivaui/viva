package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestPageBindsAndCounts(t *testing.T) {
	var done bool
	mode, clicks := "one", 0
	modes := []string{"one", "two", "three"}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
			ui.Checkbox(c, &done, "Done")
			ui.Choice(c, &mode, modes, func(s string) string { return s })
			if ui.Button(c, "Count").Clicked() {
				clicks++
			}
		})
	})
	d.click("Done")
	d.click("three")
	d.click("Count")
	if !done || mode != "three" || clicks != 1 {
		t.Fatalf("done=%v mode=%q clicks=%d", done, mode, clicks)
	}
}

func TestPageFieldAppearingAboveKeepsTheTyping(t *testing.T) {
	var otp, password ui.Field
	password.Secure = true
	needOTP := false
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			if needOTP {
				ui.TextField(c, &otp, "Code", "")
			}
			ui.TextField(c, &password, "Password", "")
		})
	})
	d.click("Password")
	d.typeText("ab")
	needOTP = true
	d.repaint()
	d.typeText("cd")
	if password.Text() != "abcd" || otp.Text() != "" || d.focused() != "Password" {
		t.Fatalf("password %q code %q focus %q", password.Text(), otp.Text(), d.focused())
	}
}

// A copy bound in every paint works as its place does: its click arrives, and
// Changed writes it back.
func TestPageCopyBoundWorksByPlace(t *testing.T) {
	viewed := map[string]bool{}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			for _, name := range []string{"a", "b"} {
				v := viewed[name]
				if ui.Checkbox(c, &v, name).Changed() {
					viewed[name] = v
				}
			}
		})
	})
	d.repaint()
	d.click("b")
	d.click("b")
	d.click("a")
	if !viewed["a"] || viewed["b"] {
		t.Fatalf("viewed = %v, want a on and b toggled twice back off", viewed)
	}
}

// Places are counted in each parent: a row added in one container does not move
// the buttons of another.
func TestPagePlacesAreCountedPerParent(t *testing.T) {
	extra := false
	clicked := ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				if extra {
					ui.Button(c, "Extra")
				}
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Save").Clicked() {
					clicked = "Save"
				}
			})
		})
	})
	save := d.hit("Save").ID
	extra = true
	d.repaint()
	if d.hit("Save").ID != save {
		t.Fatal("a button added in another row renamed Save")
	}
	d.click("Save")
	if clicked != "Save" {
		t.Fatalf("clicked %q", clicked)
	}
}

func TestPageKeyedRowsFollowTheirItems(t *testing.T) {
	names := []string{"a", "b", "c"}
	opened := ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(4).Children(func() {
			for _, name := range names {
				ui.Row(c.Key(name)).Gap(8).Children(func() {
					if ui.Button(c, "Open "+name).Clicked() {
						opened = name
					}
				})
			}
		})
	})
	d.click("Open b")
	names = []string{"c", "b", "a"}
	d.repaint()
	d.key(gfx.KeyEnter)
	if opened != "b" || d.focused() != "Open b" {
		t.Fatalf("opened %q focus %q", opened, d.focused())
	}
}

// A control of the application's own, named with ui.ID inside keyed rows, keeps
// its identity and focus when a row is inserted above it.
func TestPageCustomControlFollowsItsRow(t *testing.T) {
	names := []string{"a", "b"}
	opened := ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(4).Children(func() {
			for _, name := range names {
				ui.Row(c.Key(name)).Children(func() {
					if ui.Text(c, "Row "+name).Pad(4).Hit(ui.ID(c), "Row "+name).Clicked() {
						opened = name
					}
				})
			}
		})
	})
	before := d.hit("Row b").ID
	names = []string{"z", "a", "b"}
	d.repaint()
	if d.hit("Row b").ID != before {
		t.Fatal("inserting a row above renamed a custom control")
	}
	if d.hit("Row z").ID == before {
		t.Fatal("two rows share a custom control's identity")
	}
	d.click("Row b")
	if opened != "b" {
		t.Fatalf("opened %q", opened)
	}
}

// A component written in the page's own API keeps its state to itself.
func TestPageComponentWithItsOwnState(t *testing.T) {
	size := "M"
	sizes := []string{"S", "M", "L"}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			combobox(c, &size, sizes, func(s string) string { return s })
		})
	})
	d.click("M")
	if !d.has("L") {
		t.Fatal("the combobox did not open")
	}
	d.click("L")
	if size != "L" || d.has("S") {
		t.Fatalf("size = %q, list still open: %v", size, d.has("S"))
	}
}

func TestPagePopoverAndModal(t *testing.T) {
	var menu, confirm bool
	deleted := 0
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Pad(16).Gap(8).Children(func() {
			more := ui.Button(c, "More")
			if more.Clicked() {
				menu = !menu
			}
			ui.Popover(c, more, &menu, func() {
				if ui.Button(c, "Delete…").Clicked() {
					menu, confirm = false, true
				}
			})
			ui.Modal(c, &confirm, "Delete host?", func() {
				ui.Text(c, "This cannot be undone.")
			}, func() {
				if ui.Button(c, "Delete").Destructive().Clicked() {
					deleted++
					confirm = false
				}
			})
		})
	})
	d.click("More")
	d.click("Delete…")
	if !confirm || d.focused() != "Delete" {
		t.Fatalf("confirm=%v focus=%q", confirm, d.focused())
	}
	d.key(gfx.KeyEnter)
	if deleted != 1 || confirm {
		t.Fatalf("deleted=%d confirm=%v", deleted, confirm)
	}
	if !strings.Contains(d.focused(), "More") {
		t.Fatalf("focus after closing is %q", d.focused())
	}
}

func TestPageSteadyPaintAllocatesNothing(t *testing.T) {
	m := newFleetModel()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, m.view)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, m.view) }); allocs != 0 {
		t.Fatalf("a steady page allocates %v times", allocs)
	}
}

func (m *fleetModel) view(c ui.C) {
	ui.Column(c).Fill().Pad(22).Gap(14).Children(func() {
		ui.Text(c, "Fleet check").FontSize(26).Bold()
		ui.Choice(c, &m.filter, fleetFilters, fleetFilterLabel)
		ui.Scroll(c).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				h := &m.hosts[i]
				ui.Row(c).FillWidth().Pad(12).Gap(12).Children(func() {
					ui.Text(c, h.name).Bold()
					ui.Text(c, h.addr).Muted()
					ui.Checkbox(c, &h.handled, "")
				})
			}
		})
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, m.summary).Muted()
			if ui.Button(c, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

var fleetFilters = []int{0, 1, 2}

func fleetFilterLabel(i int) string { return [...]string{"All", "Open", "Handled"}[i] }

func BenchmarkPageFleet(b *testing.B) {
	m := newFleetModel()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}

func BenchmarkPageKeyed1000(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Children(func() {
			for i := range 1000 {
				ui.Row(c.KeyInt(i)).Children(func() { ui.Button(c, "Open") })
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}

// The same page in the three styles: a thousand keyed rows, each a row holding a button.
func BenchmarkRows1000Frame(b *testing.B) {
	canvas := benchCanvas()
	canvas.SetTree(&benchTree)
	var view ui.View
	theme := gfx.LightTheme()
	body := func(f *ui.Frame) {
		f.Column().Children(func() {
			for i := range 1000 {
				f.Row().Children(func() { f.Button(benchRows.At(i), "Open") })
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.BeginFrame()
		view.Draw(canvas, theme, gfx.Rect{Width: 600, Height: 400}, body)
		canvas.EndFrame()
	}
}

func BenchmarkRows1000Page(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Children(func() {
			for i := range 1000 {
				ui.Row(c.KeyInt(i)).Children(func() { ui.Button(c, "Open") })
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}

// combobox is a component of an application's own, written in the API Viva's
// elements are: a button
// showing the chosen option and a popover listing them, with whether it is open
// kept as the component's own state rather than the caller's.
func combobox[T comparable](c ui.C, value *T, options []T, label func(T) string) ui.Box {
	var root ui.Box
	root = ui.Column(c.Part("combobox")).Children(func() {
		open := ui.State[bool](c, "open")
		trigger := ui.Button(c.Part("trigger"), label(*value)).Outline()
		if trigger.Clicked() {
			*open = !*open
		}
		ui.Popover(c, trigger, open, func() {
			for i, option := range options {
				if ui.Button(c.KeyInt(i), label(option)).Ghost().Clicked() {
					*value, *open = option, false
				}
			}
		})
	})
	return root
}

// AutoFocus takes the focus as its element appears, not again while it stays, and
// again after it was gone; Enter in a page's TextField is Submitted.
func TestPageAutoFocusAndSubmit(t *testing.T) {
	var query ui.Field
	shown, submitted := true, ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Button(c, "Other")
			if shown && ui.TextField(c, &query, "Search", "").AutoFocus().Submitted() {
				submitted = query.Text()
			}
		})
	})
	if d.focused() != "Search" {
		t.Fatalf("focus on open = %q", d.focused())
	}
	d.typeText("go")
	d.key(gfx.KeyEnter)
	if submitted != "go" {
		t.Fatalf("submitted %q", submitted)
	}
	d.click("Other")
	d.repaint()
	if d.focused() != "Other" {
		t.Fatalf("AutoFocus took the focus back: %q", d.focused())
	}
	shown = false
	d.repaint()
	shown = true
	d.repaint()
	if d.focused() != "Search" {
		t.Fatalf("a field shown again did not take the focus: %q", d.focused())
	}
}

func TestPageAutoFocusInAMemoPanics(t *testing.T) {
	var query ui.Field
	defer func() {
		if recover() == nil {
			t.Fatal("AutoFocus in a Memo did not panic")
		}
	}()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, func(c ui.C) {
		ui.Memo(c, 1, func() { ui.TextField(c, &query, "Search", "").AutoFocus() })
	})
}

// A Popover enters and leaves with the theme's motion, and the leaving picture takes
// no press; a theme without transitions shows and hides it at once.
func TestPagePopoverMoves(t *testing.T) {
	still := pageTheme
	still.Transition.Duration = 0
	for _, test := range []struct {
		name  string
		theme *gfx.Theme
		moves bool
	}{{"moving", &pageTheme, true}, {"still", &still, false}} {
		t.Run(test.name, func(t *testing.T) {
			open := false
			d := openPageWith(t, ui.PageConfig{Theme: test.theme, View: func(c ui.C) {
				ui.Column(c).Pad(16).Children(func() {
					more := ui.Button(c, "More")
					if more.Clicked() {
						open = !open
					}
					ui.Popover(c, more, &open, func() { ui.Button(c, "Rename") })
				})
			}}, app.Config{})
			d.click("More")
			if !open || d.driver.Transitioning() != test.moves {
				t.Fatalf("open=%v transitioning=%v", open, d.driver.Transitioning())
			}
			if err := d.driver.Advance(300 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if d.driver.Transitioning() || !d.has("Rename") {
				t.Fatal("the popover did not come to rest open")
			}
			d.key(gfx.KeyEscape)
			if open || d.has("Rename") || d.driver.Transitioning() != test.moves {
				t.Fatalf("closing: open=%v pressable=%v transitioning=%v", open, d.has("Rename"), d.driver.Transitioning())
			}
			if err := d.driver.Advance(300 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if d.driver.Transitioning() {
				t.Fatal("the popover's exit never ended")
			}
		})
	}
}

func BenchmarkPagePopoverOpen(b *testing.B) {
	open := true
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	var now time.Duration // the presence runs on a clock, as in a window
	canvas.SetClock(func() time.Duration { return now })
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			more := ui.Button(c, "More")
			ui.Popover(c, more, &open, func() {
				ui.Button(c, "Rename")
				ui.Button(c, "Delete")
			})
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		now += 16 * time.Millisecond
		drawPage(page, canvas, view)
	}
}

// A TextArea edits its Field in rows: Enter is a new line, not a submission.
func TestPageTextArea(t *testing.T) {
	var notes ui.Field
	changes := 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			if ui.TextArea(c, &notes, "Notes", "Anything to add").Width(300).Height(120).Changed() {
				changes++
			}
			ui.Button(c, "After")
		})
	})
	d.click("Notes")
	if d.focused() != "Notes" {
		t.Fatalf("focus = %q", d.focused())
	}
	d.typeText("one")
	d.key(gfx.KeyEnter)
	d.typeText("two")
	if notes.Text() != "one\ntwo" || changes == 0 {
		t.Fatalf("text %q, changes %d", notes.Text(), changes)
	}
	d.key(gfx.KeyUp)
	d.typeText("!")
	if notes.Text() != "one!\ntwo" {
		t.Fatalf("Up did not move to the row above: %q", notes.Text())
	}
}

// A Field declared as a TextArea and then as a one-line TextField moves by its line
// again: Up goes to the start, not to a row it no longer shows.
func TestPageFieldLeavesItsRows(t *testing.T) {
	var notes ui.Field
	area := true
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			if area {
				ui.TextArea(c, &notes, "Notes", "").Width(300).Height(120)
			} else {
				ui.TextField(c, &notes, "Notes", "").Width(300)
			}
		})
	})
	d.click("Notes")
	d.typeText("one")
	d.key(gfx.KeyEnter)
	d.typeText("two")
	area = false
	d.repaint()
	if result := notes.Key(gfx.KeyEvent{Key: gfx.KeyUp, Target: d.hit("Notes").ID}); result.Handled {
		t.Fatal("a one-line field still moves by the rows of the TextArea it was")
	}
}

// A State asked for only in some paints does not take the value of another in the
// same container: each is found by its name.
func TestStateIsFoundByName(t *testing.T) {
	var expanded, open *bool
	extra := false
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Children(func() {
			if extra {
				expanded = ui.State[bool](c, "expanded")
			}
			open = ui.State[bool](c, "open")
		})
	}
	drawPage(page, canvas, view)
	*open = true
	extra = true
	drawPage(page, canvas, view)
	if !*open || *expanded {
		t.Fatalf("open=%v expanded=%v after a State appeared before it", *open, *expanded)
	}
}

// A Select's open list follows its value: a Select that appears above it does not
// open in its place.
func TestSelectStaysOpenWhenOneAppearsAbove(t *testing.T) {
	size, color, first := "", "", false
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			if first {
				ui.Select(c, &color, []string{"Red", "Blue"}, nil).Placeholder("Color")
			}
			ui.Select(c, &size, []string{"S", "M", "L"}, nil).Placeholder("Size")
		})
	})
	var trigger gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			trigger = hit
		}
	}
	if err := d.driver.Click(center(trigger.Bounds)); err != nil {
		t.Fatal(err)
	}
	first = true
	d.repaint()
	if !d.has("L") || d.has("Red") {
		t.Fatalf("size open=%v, color open=%v", d.has("L"), d.has("Red"))
	}
}

// SelectIcon opens, chooses and writes back through the option type of the
// application's own, label and icon both named by functions of the option.
func TestSelectIconChoosesAnOption(t *testing.T) {
	type locale struct {
		name string
		flag int
	}
	flagOf := func(flag int) gfx.Icon {
		return gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
			Commands: []gfx.PathCommand{gfx.MoveTo(4+float32(flag)*4, 6), gfx.LineTo(14+float32(flag)*4, 18)}}
	}
	locales := []locale{{"简体中文", 0}, {"English", 1}, {"日本語", 2}}
	chosen := locales[1]
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.SelectIcon(c, &chosen, locales,
				func(l locale) string { return l.name },
				func(l locale) gfx.Icon { return flagOf(l.flag) }).Placeholder("Language")
		})
	})
	d.click("Language")
	if !d.has("日本語") {
		t.Fatal("the open list does not offer its options")
	}
	d.click("日本語")
	if chosen != locales[2] {
		t.Fatalf("chose %+v, want %+v", chosen, locales[2])
	}
}
