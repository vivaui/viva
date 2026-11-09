package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// console is a control of the application's own, as a terminal is: it paints its
// lines itself and takes keys, presses, the wheel and typing (with input methods)
// through its Handlers, with no identity declared and no window-level dispatch.
type console struct {
	input   ui.Handlers
	draw    func(*ui.DrawContext)
	typed   string
	keys    []gfx.Key
	presses int
	drags   int
	scroll  float32
	focused bool
	text    app.TextInput
	asked   gfx.ID
}

func newConsole() *console {
	t := &console{}
	t.input = ui.Handlers{
		Role: gfx.RoleTerminal,
		Key: func(event gfx.KeyEvent) gfx.EventResult {
			if event.Key == gfx.KeyTab {
				return gfx.EventResult{} // leave Tab to the page: the focus moves on
			}
			t.keys = append(t.keys, event.Key)
			return gfx.EventResult{Handled: true, Repaint: true}
		},
		Pointer: func(event gfx.PointerEvent) gfx.EventResult {
			switch event.Kind {
			case gfx.PointerDown:
				t.presses++
			case gfx.PointerMove:
				if event.Captured {
					t.drags++
				}
			}
			return gfx.EventResult{Handled: true}
		},
		Scroll: func(event gfx.ScrollEvent) gfx.EventResult {
			t.scroll += event.DeltaY
			return gfx.EventResult{Handled: true}
		},
		Text: func(context app.TextInputContext) *app.TextInput {
			t.asked = context.ID
			t.text.Bounds = gfx.Rect{X: context.Bounds.X + 4, Y: context.Bounds.Y + 4, Width: 1, Height: 16}
			t.text.OnEvent = t.onText
			return &t.text
		},
	}
	t.draw = func(d *ui.DrawContext) {
		t.focused = d.State.Focused
		d.Canvas.Fill(d.Rect, d.Theme.Card)
	}
	return t
}

func (t *console) onText(event app.TextEvent) (app.TextState, error) {
	if event.Kind == app.TextInsert {
		t.typed += event.Text
	}
	return app.TextState{}, nil
}

func (t *console) view(c ui.C) {
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		ui.Control(c, "Console", &t.input).Size(300, 200).Draw(t.draw)
		ui.Button(c, "After")
	})
}

func TestControlTakesItsOwnInput(t *testing.T) {
	term := newConsole()
	d := openPage(t, term.view)
	box := d.hit("Console").Bounds
	x, y := center(box)
	if err := d.driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(x+20, y+10); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(x+20, y+10, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if term.presses != 1 || term.drags == 0 {
		t.Fatalf("presses=%d drags=%d", term.presses, term.drags)
	}
	if d.focused() != "Console" || !term.focused {
		t.Fatalf("a press on the control did not focus it: %q, drawn focused %v", d.focused(), term.focused)
	}
	d.key(gfx.KeyUp)
	d.key(gfx.KeyEnter)
	if len(term.keys) != 2 || term.keys[0] != gfx.KeyUp || term.keys[1] != gfx.KeyEnter {
		t.Fatalf("keys = %v", term.keys)
	}
	d.typeText("ls")
	if term.typed != "ls" || term.asked != d.hit("Console").ID {
		t.Fatalf("typed %q, text service asked for %v", term.typed, term.asked)
	}
	if err := d.driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: -3}); err != nil {
		t.Fatal(err)
	}
	if term.scroll != -3 {
		t.Fatalf("scroll = %v", term.scroll)
	}
	d.key(gfx.KeyTab)
	if d.focused() != "After" {
		t.Fatalf("Tab left unhandled did not move the focus on: %q", d.focused())
	}
}

// A Control in a pane of a Frame takes its input through the host's routing.
func TestControlInAFramePane(t *testing.T) {
	term := newConsole()
	var tree gfx.Tree
	side := tree.Feature("side")
	page := ui.NewPage(ui.PageConfig{Root: "pane", Theme: &pageTheme})
	var view ui.View
	sideClicks := 0
	config := app.Config{Width: 640, Height: 400, Tree: &tree, TextInput: page.TextInput, OnScroll: page.Wheel,
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			if r, ok := page.Key(event); ok {
				return r
			}
			return gfx.EventResult{}
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			if r, ok := page.Pointer(event); ok {
				return r
			}
			return gfx.EventResult{}
		}}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		view.Draw(canvas, pageTheme, gfx.Rect{Width: size.Width, Height: size.Height}, func(f *ui.Frame) {
			f.Row().Fill().Children(func() {
				if f.Button(side.ID(gfx.Index(0)), "Side").Clicked() {
					sideClicks++
				}
				f.Page(page, term.view).Grow(1).FillHeight()
			})
		})
	}
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	d := &pageDriver{t: t, driver: driver, page: page}
	d.click("Console")
	d.key(gfx.KeyDown)
	d.typeText("pwd")
	if term.presses != 1 || len(term.keys) != 1 || term.typed != "pwd" {
		t.Fatalf("presses=%d keys=%v typed=%q", term.presses, term.keys, term.typed)
	}
	d.click("Side")
	if sideClicks != 1 {
		t.Fatal("the window's own control lost its click")
	}
}

func TestControlInAMemoPanics(t *testing.T) {
	term := newConsole()
	defer func() {
		if recover() == nil {
			t.Fatal("a Control in a Memo did not panic")
		}
	}()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, func(c ui.C) {
		ui.Memo(c, 1, func() { ui.Control(c, "Console", &term.input).Size(10, 10) })
	})
}

func BenchmarkPageControl(b *testing.B) {
	term := newConsole()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, term.view)
	}
}
