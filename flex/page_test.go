package flex_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

func TestPageBindsAndCounts(t *testing.T) {
	var done bool
	mode, clicks := "one", 0
	modes := []string{"one", "two", "three"}
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Fill().Pad(16).Gap(8).Children(func() {
			flex.Checkbox(c, &done, "Done")
			flex.Choice(c, &mode, modes, func(s string) string { return s })
			if flex.Button(c, "Count").Clicked() {
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
	var otp, password flex.Field
	password.Secure = true
	needOTP := false
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Pad(16).Gap(8).Children(func() {
			if needOTP {
				flex.TextField(c, &otp, "Code", "")
			}
			flex.TextField(c, &password, "Password", "")
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

// A copy bound in every paint works as its place does, as in mygo: its click
// arrives, and Changed writes it back.
func TestPageCopyBoundWorksByPlace(t *testing.T) {
	viewed := map[string]bool{}
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Pad(16).Children(func() {
			for _, name := range []string{"a", "b"} {
				v := viewed[name]
				if flex.Checkbox(c, &v, name).Changed() {
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
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Pad(16).Gap(8).Children(func() {
			flex.Row(c).Gap(8).Children(func() {
				if extra {
					flex.Button(c, "Extra")
				}
			})
			flex.Row(c).Gap(8).Children(func() {
				if flex.Button(c, "Save").Clicked() {
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
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Pad(16).Gap(4).Children(func() {
			for _, name := range names {
				flex.Row(c.Key(name)).Gap(8).Children(func() {
					if flex.Button(c, "Open "+name).Clicked() {
						opened = name
					}
				})
			}
		})
	})
	d.click("Open b")
	names = []string{"c", "b", "a"}
	d.repaint()
	d.key(ui.KeyEnter)
	if opened != "b" || d.focused() != "Open b" {
		t.Fatalf("opened %q focus %q", opened, d.focused())
	}
}

// A control of the application's own, named with flex.ID inside keyed rows, keeps
// its identity and focus when a row is inserted above it.
func TestPageCustomControlFollowsItsRow(t *testing.T) {
	names := []string{"a", "b"}
	opened := ""
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Pad(16).Gap(4).Children(func() {
			for _, name := range names {
				flex.Row(c.Key(name)).Children(func() {
					if c.Frame().Text("Row "+name).Pad(4).Hit(flex.ID(c), "Row "+name).Clicked() {
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
	d := openPage(t, func(c flex.C) {
		flex.Column(c).Pad(16).Children(func() {
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
	d := openPage(t, func(c flex.C) {
		flex.Row(c).Pad(16).Gap(8).Children(func() {
			more := flex.Button(c, "More")
			if more.Clicked() {
				menu = !menu
			}
			flex.Popover(c, more, &menu, func() {
				if flex.Button(c, "Delete…").Clicked() {
					menu, confirm = false, true
				}
			})
			flex.Modal(c, &confirm, "Delete host?", func() {
				flex.Text(c, "This cannot be undone.")
			}, func() {
				if flex.Button(c, "Delete").Destructive().Clicked() {
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
	d.key(ui.KeyEnter)
	if deleted != 1 || confirm {
		t.Fatalf("deleted=%d confirm=%v", deleted, confirm)
	}
	if !strings.Contains(d.focused(), "More") {
		t.Fatalf("focus after closing is %q", d.focused())
	}
}

func TestPageSteadyPaintAllocatesNothing(t *testing.T) {
	m := newFleetModel()
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, m.view)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, m.view) }); allocs != 0 {
		t.Fatalf("a steady page allocates %v times", allocs)
	}
}

func (m *fleetModel) view(c flex.C) {
	flex.Column(c).Fill().Pad(22).Gap(14).Children(func() {
		flex.Text(c, "Fleet check").FontSize(26).Bold()
		flex.Choice(c, &m.filter, fleetFilters, fleetFilterLabel)
		flex.Scroll(c).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				h := &m.hosts[i]
				flex.Row(c).FillWidth().Pad(12).Gap(12).Children(func() {
					flex.Text(c, h.name).Bold()
					flex.Text(c, h.addr).Muted()
					flex.Checkbox(c, &h.handled, "")
				})
			}
		})
		flex.Row(c).Gap(10).Children(func() {
			flex.Text(c, m.summary).Muted()
			if flex.Button(c, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

var fleetFilters = []int{0, 1, 2}

func fleetFilterLabel(i int) string { return [...]string{"All", "Open", "Handled"}[i] }

func BenchmarkPageFleet(b *testing.B) {
	m := newFleetModel()
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}

func BenchmarkPageKeyed1000(b *testing.B) {
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c flex.C) {
		flex.Column(c).Children(func() {
			for i := range 1000 {
				flex.Row(c.KeyInt(i)).Children(func() { flex.Button(c, "Open") })
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
	var view flex.View
	theme := ui.LightTheme()
	body := func(f *flex.Frame) {
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
		view.Draw(canvas, theme, ui.Rect{Width: 600, Height: 400}, body)
		canvas.EndFrame()
	}
}

func BenchmarkRows1000Page(b *testing.B) {
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c flex.C) {
		flex.Column(c).Children(func() {
			for i := range 1000 {
				flex.Row(c.KeyInt(i)).Children(func() { flex.Button(c, "Open") })
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
func combobox[T comparable](c flex.C, value *T, options []T, label func(T) string) flex.Box {
	var root flex.Box
	root = flex.Column(c.Part("combobox")).Children(func() {
		open := flex.State[bool](c)
		trigger := flex.Button(c.Part("trigger"), label(*value)).Outline()
		if trigger.Clicked() {
			*open = !*open
		}
		flex.Popover(c, trigger, open, func() {
			for i, option := range options {
				if flex.Button(c.KeyInt(i), label(option)).Ghost().Clicked() {
					*value, *open = option, false
				}
			}
		})
	})
	return root
}
