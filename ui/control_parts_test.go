package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// stars is a rating of five stars made of the public API alone: a Control whose
// stars are parts it declares as it draws, with its own keys.
type stars struct {
	handlers ui.Handlers
	value    int
	pressed  []int // the parts a pointer event was aimed at
	typed    []string
	drawFn   func(*ui.DrawContext) // s.draw, made once: a method value made in each paint allocates
}

func newStars() *stars {
	s := &stars{}
	s.handlers.Key = s.key
	s.handlers.Pointer = s.pointer
	s.drawFn = s.draw
	return s
}

var starNames = [5]string{"Star 1", "Star 2", "Star 3", "Star 4", "Star 5"}

func (s *stars) part(target gfx.ID) (int, bool) { return target.In(s.handlers.Parts()) }

func (s *stars) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Text != "" {
		s.typed = append(s.typed, event.Text)
		return gfx.EventResult{Handled: true}
	}
	i, ok := s.part(event.Target)
	if !ok {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyRight:
		next := min(i+1, 4)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.handlers.Parts().At(next), FocusVisible: true}
	case gfx.KeyLeft:
		prev := max(i-1, 0)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.handlers.Parts().At(prev), FocusVisible: true}
	case gfx.KeyEnter, gfx.KeySpace:
		s.value = i + 1
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

func (s *stars) pointer(event gfx.PointerEvent) gfx.EventResult {
	if i, ok := s.part(event.Target); ok && event.Kind == gfx.PointerDown {
		s.pressed = append(s.pressed, i)
		s.value = i + 1
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// draw declares a part for each star: a hit of its own with its name, drawn filled up
// to the value.
func (s *stars) draw(d *ui.DrawContext) {
	parts := s.handlers.Parts()
	for i := range 5 {
		r := gfx.Rect{X: d.Rect.X + float32(i)*34, Y: d.Rect.Y, Width: 32, Height: 32}
		color := d.Theme.Muted
		if i < s.value {
			color = d.Theme.Primary
		}
		d.Canvas.Fill(r, color)
		d.Canvas.HitWithOptions(parts.At(i), r, starNames[i], gfx.HitOptions{Role: gfx.RoleButton})
	}
}

func (s *stars) page(c ui.C) {
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		ui.Control(c, "Rating", &s.handlers).Size(170, 32).Draw(s.drawFn)
		ui.Button(c, "after")
	})
}

// A control declares the regions inside it as it draws; the pointer, the keys and the
// focus reach its handlers with the region as the target, so a component with
// several interactive parts is one Control.
func TestControlPartsTakeThePointerAndTheKeys(t *testing.T) {
	s := newStars()
	d := openPage(t, s.page)
	d.click("Star 4")
	if s.value != 4 || len(s.pressed) == 0 || s.pressed[0] != 3 {
		t.Fatalf("a press on star 4: value %d, parts pressed %v", s.value, s.pressed)
	}
	if d.focused() != "Star 4" {
		t.Fatalf("the press focused %q", d.focused())
	}
	d.key(gfx.KeyRight)
	if d.focused() != "Star 5" {
		t.Fatalf("Right went to %q", d.focused())
	}
	d.key(gfx.KeyLeft)
	d.key(gfx.KeyLeft)
	d.key(gfx.KeyEnter)
	if d.focused() != "Star 3" || s.value != 3 {
		t.Fatalf("focused %q, value %d after Left, Left, Enter", d.focused(), s.value)
	}
}

// A printable key typed while a part has the focus reaches the control's Key with
// its text, as typeahead needs; with a modifier it is the window's.
func TestControlKeyGetsPrintableKeys(t *testing.T) {
	s := newStars()
	d := openPage(t, s.page)
	d.click("Star 2")
	if _, err := d.driver.ShortcutChord(0, 'g'); err != nil {
		t.Fatal(err)
	}
	if _, err := d.driver.ShortcutChord(gfx.ModifierShift, 'r'); err != nil {
		t.Fatal(err)
	}
	if len(s.typed) != 2 || s.typed[0] != "g" || s.typed[1] != "r" {
		t.Fatalf("typed %q", s.typed)
	}
	if _, err := d.driver.ShortcutChord(gfx.ModifierMeta, 'k'); err != nil {
		t.Fatal(err)
	}
	if len(s.typed) != 2 {
		t.Fatalf("Meta+k reached the control: %q", s.typed)
	}
}

// What is aimed at another control or at the page is not the control's.
func TestControlPartsAreItsOwn(t *testing.T) {
	s := newStars()
	d := openPage(t, s.page)
	d.click("after")
	d.key(gfx.KeyRight)
	if _, err := d.driver.ShortcutChord(0, 'g'); err != nil {
		t.Fatal(err)
	}
	if s.value != 0 || len(s.typed) != 0 {
		t.Fatalf("value %d, typed %q from a key on a button", s.value, s.typed)
	}
}

// Declaring parts as it draws allocates nothing.
func TestControlPartsRepaintAllocates(t *testing.T) {
	s := newStars()
	d := openPage(t, s.page)
	d.click("Star 2")
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
