package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type drawerModel struct {
	open, nonModal bool
	title          string
	side           ui.DrawerSide
	saves, behind  int
	name           string
}

func (m *drawerModel) page(c ui.C) {
	if ui.Shortcut(c, ui.Primary, "s") {
		m.saves++
	}
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		if ui.Button(c, "Open").Clicked() {
			m.open = true
		}
		if ui.Button(c, "Behind").Clicked() {
			m.behind++
		}
	})
	ui.Drawer(c, &m.open, m.title, ui.DrawerSpec{Side: m.side, Size: 300, NonModal: m.nonModal}, func() {
		ui.TextField(c, &m.name, "Name", "")
	}, func() {
		if ui.Button(c, "Done").Clicked() {
			m.open = false
		}
	})
}

// A Drawer comes in from its edge with its body in the panel, takes the keyboard for
// its first control, and Escape, its close button, a press beside it and its own
// buttons close it.
func TestDrawerOpensAndCloses(t *testing.T) {
	for _, close := range []string{"escape", "close", "outside", "done"} {
		t.Run(close, func(t *testing.T) {
			m := &drawerModel{title: "Host"}
			d := openPage(t, m.page)
			d.click("Open")
			if err := d.driver.Advance(time.Second); err != nil {
				t.Fatal(err)
			}
			name := d.hit("Name").Bounds
			if !d.page.Overlay() || name.X < 480-300 || name.X+name.Width > 480 {
				t.Fatalf("overlay %v, the body at %v is not in the right 300 of the window", d.page.Overlay(), name)
			}
			if d.focused() != "Name" {
				t.Fatalf("focus is on %q, not the first control", d.focused())
			}
			if done := d.hit("Done").Bounds; done.Y < name.Y+name.Height {
				t.Fatalf("the footer %v is not under the body %v", done, name)
			}
			switch close {
			case "escape":
				d.key(gfx.KeyEscape)
			case "close":
				d.click("Close drawer")
			case "outside":
				if err := d.driver.Click(20, 400); err != nil {
					t.Fatal(err)
				}
			case "done":
				d.click("Done")
			}
			if m.open || m.behind != 0 {
				t.Fatalf("open %v, the page behind took %d presses", m.open, m.behind)
			}
		})
	}
}

// Without a title the body has the panel to itself; a drawer from the left is on
// the left.
func TestDrawerUntitledAndLeft(t *testing.T) {
	m := &drawerModel{side: ui.DrawerLeft}
	d := openPage(t, m.page)
	d.click("Open")
	if err := d.driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	name := d.hit("Name").Bounds
	if name.X > 1 || name.Y > 1 || d.has("Close drawer") {
		t.Fatalf("an untitled left drawer's body starts at %v, close shown %v", name, d.has("Close drawer"))
	}
}

// A NonModal drawer leaves the keyboard to the page: the page's shortcuts act.
func TestDrawerNonModal(t *testing.T) {
	m := &drawerModel{title: "Assistant", nonModal: true}
	d := openPage(t, m.page)
	d.click("Open")
	if d.page.Overlay() || d.focused() == "Name" {
		t.Fatalf("overlay %v, focus %q", d.page.Overlay(), d.focused())
	}
	if _, err := d.driver.ShortcutChord(ui.Primary, 's'); err != nil {
		t.Fatal(err)
	}
	if m.saves != 1 {
		t.Fatal("the page's Shortcut did not act with a NonModal drawer open")
	}
}

func BenchmarkDrawerOpen(b *testing.B) {
	m := &drawerModel{open: true, title: "Host"}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.page)
	}
}
