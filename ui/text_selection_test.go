package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
	"github.com/vivaui/viva/ui"
)

// selectable is a headless window with one TextSelectionArea, declared by body.
type selectable struct {
	t      *testing.T
	driver *app.Driver
	canvas *gfx.Canvas
	area   ui.TextSelectionState
	body   func(c ui.C)
}

func openSelectable(t *testing.T, body func(c ui.C)) *selectable {
	t.Helper()
	s := &selectable{t: t, body: body}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
			ui.TextSelectionArea(c, &s.area).FillWidth().Gap(4).Children(func() { s.body(c) })
		})
	}})
	s.open(page)
	return s
}

func (s *selectable) open(page *ui.Page) {
	s.t.Helper()
	s.canvas = &gfx.Canvas{}
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 640, Height: 360}), s.canvas)
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Cleanup(driver.Close)
	s.driver = driver
	s.must(driver.Paint())
}

func (s *selectable) must(err error) {
	s.t.Helper()
	if err != nil {
		s.t.Fatal(err)
	}
}

// painted is the line value as the last paint drew it.
func (s *selectable) painted(value string) gfx.Text {
	s.t.Helper()
	for _, text := range s.driver.Snapshot().Texts {
		if text.Value == value {
			return text
		}
	}
	s.t.Fatalf("%q is not painted", value)
	return gfx.Text{}
}

func (s *selectable) text(value string) gfx.Rect { return s.painted(value).Bounds }

// width is how wide the words of the line holding value are drawn: part of it, when
// part is a prefix of value.
func (s *selectable) width(value, part string) float32 {
	return s.canvas.TextWidthStyle(part, s.painted(value).Style)
}

// start and end are points just inside the first and the last character of a line.
func (s *selectable) start(value string) (float32, float32) {
	r := s.text(value)
	return r.X + 1, r.Y + r.Height/2
}

func (s *selectable) end(value string) (float32, float32) {
	r := s.text(value)
	return r.X + s.width(value, value) + 4, r.Y + r.Height/2
}

func (s *selectable) drag(fromX, fromY, toX, toY float32) {
	s.t.Helper()
	s.must(s.driver.Press(fromX, fromY, gfx.PointerButtonPrimary))
	s.must(s.driver.Move((fromX+toX)/2, (fromY+toY)/2))
	s.must(s.driver.Move(toX, toY))
	s.must(s.driver.Release(toX, toY, gfx.PointerButtonPrimary))
	s.must(s.driver.Paint())
}

// copied presses Primary+C and returns what reached the clipboard.
func (s *selectable) copied() string {
	s.t.Helper()
	s.driver.SetClipboard("")
	_, err := s.driver.ShortcutChord(keymap.Primary, 'c')
	s.must(err)
	s.must(s.driver.Paint())
	return s.driver.Clipboard()
}

// A drag from one text into another selects the end of the first and the start of the
// second, and copies them as their source, one per line.
func TestTextSelectionAreaDragsAcrossTexts(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "host key fingerprint")
		ui.Text(c, "SHA256:abcdef")
	})
	fx, fy := s.start("host key fingerprint")
	tx, ty := s.end("SHA256:abcdef")
	s.drag(fx, fy, tx, ty)
	if got := s.copied(); got != "host key fingerprint\nSHA256:abcdef" {
		t.Fatalf("copied %q", got)
	}
	if got := s.area.Text(); got != "host key fingerprint\nSHA256:abcdef" {
		t.Fatalf("Text is %q", got)
	}
}

// A label and its value side by side in a row copy as one line, joined with a tab, so
// rows of them paste into a table.
func TestTextSelectionAreaJoinsARowWithATab(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		for _, row := range [][2]string{{"Address", "10.0.0.2"}, {"Type", "ed25519"}} {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, row[0]).Width(90)
				ui.Text(c, row[1])
			})
		}
	})
	_, err := s.driver.ShortcutChord(keymap.Primary, 'a')
	s.must(err)
	s.must(s.driver.Paint()) // nothing has the focus yet: the area does not take a page-wide Select All
	fx, fy := s.start("Address")
	tx, ty := s.end("ed25519")
	s.drag(fx, fy, tx, ty)
	if got := s.copied(); got != "Address\t10.0.0.2\nType\ted25519" {
		t.Fatalf("copied %q", got)
	}
}

// A double press selects the word under the pointer and a triple press its line.
func TestTextSelectionAreaSelectsAWordAndALine(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "connection refused by peer")
	})
	r := s.text("connection refused by peer")
	line := "connection refused by peer"
	x := r.X + s.width(line, "connection ") + (s.width(line, "connection refused")-s.width(line, "connection "))/2
	y := r.Y + r.Height/2
	s.must(s.driver.ClickCount(x, y, 2))
	s.must(s.driver.Paint())
	if got := s.copied(); got != "refused" {
		t.Fatalf("a double press copied %q", got)
	}
	s.must(s.driver.ClickCount(x, y, 3))
	s.must(s.driver.Paint())
	if got := s.copied(); got != "connection refused by peer" {
		t.Fatalf("a triple press copied %q", got)
	}
}

// Shift with a press extends the selection from where it began.
func TestTextSelectionAreaShiftPressExtends(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "first line")
		ui.Text(c, "second line")
	})
	r := s.text("first line")
	x, y := r.X+s.width("first line", "first "), r.Y+r.Height/2
	s.must(s.driver.Click(x, y))
	tx, ty := s.end("second line")
	s.must(s.driver.KeyDown(gfx.KeyShiftLeft, ""))
	s.must(s.driver.Click(tx, ty))
	s.must(s.driver.KeyUp(gfx.KeyShiftLeft, ""))
	s.must(s.driver.Paint())
	if got := s.copied(); got != "line\nsecond line" {
		t.Fatalf("copied %q", got)
	}
}

// Text that belongs to a control is not selected: a press on a button inside the area
// clicks it, and Select All leaves its words out; nor is what Unselectable takes out.
func TestTextSelectionAreaLeavesControlsAlone(t *testing.T) {
	clicks := 0
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "error: timeout")
		if ui.Button(c, "Retry").Clicked() {
			clicks++
		}
		ui.Text(c, "decoration").Unselectable()
		ui.Text(c, "after")
	})
	b := s.text("Retry")
	s.must(s.driver.Click(b.X+b.Width/2, b.Y+b.Height/2))
	s.must(s.driver.Paint())
	if clicks != 1 {
		t.Fatalf("the button inside the area was clicked %d times", clicks)
	}
	x, y := s.start("error: timeout")
	s.must(s.driver.Click(x, y))
	s.must(s.driver.Paint())
	_, err := s.driver.ShortcutChord(keymap.Primary, 'a')
	s.must(err)
	s.must(s.driver.Paint())
	if got := s.copied(); got != "error: timeout\nafter" {
		t.Fatalf("Select All copied %q", got)
	}
}

// A text cut short by an ellipsis copies whole.
func TestTextSelectionAreaCopiesTheTextAnEllipsisCut(t *testing.T) {
	long := strings.Repeat("/very/long/remote/path", 12)
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, long).Ellipsis()
	})
	var shown gfx.Rect
	for _, text := range s.driver.Snapshot().Texts {
		if strings.HasSuffix(text.Value, "…") {
			shown = text.Bounds
		}
	}
	if shown.Width == 0 {
		t.Fatal("the text was not cut short")
	}
	y := shown.Y + shown.Height/2
	s.drag(shown.X+1, y, shown.X+shown.Width+40, y)
	if got := s.copied(); got != long {
		t.Fatalf("copied %d bytes of %d", len(got), len(long))
	}
}

// Escape drops the selection; so does another control taking the keyboard, while a
// press on nothing leaves it.
func TestTextSelectionAreaDropsTheSelection(t *testing.T) {
	var field string
	s := &selectable{t: t}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
			ui.TextSelectionArea(c, &s.area).FillWidth().Children(func() {
				ui.Text(c, "selected words")
			})
			ui.Spacer(c).Height(80)
			ui.TextField(c, &field, "Name", "type here").Width(200)
		})
	}})
	s.open(page)
	driver := s.driver
	selectAll := func() {
		x, y := s.start("selected words")
		s.must(driver.Click(x, y))
		_, err := driver.ShortcutChord(keymap.Primary, 'a')
		s.must(err)
		s.must(driver.Paint())
		if !s.area.Has() {
			t.Fatal("nothing was selected")
		}
	}
	selectAll()
	s.must(driver.Key(gfx.KeyEscape, 0))
	s.must(driver.Paint())
	if s.area.Has() {
		t.Fatal("Escape left the selection")
	}
	selectAll()
	s.must(driver.Click(600, 340))
	s.must(driver.Paint())
	if !s.area.Has() {
		t.Fatal("a press on nothing dropped the selection")
	}
	r := s.text("type here")
	s.must(driver.Click(r.X+r.Width/2, r.Y+r.Height/2))
	s.must(driver.Paint())
	if s.area.Has() {
		t.Fatal("the selection stayed while a field had the keyboard")
	}
}

// A secondary press opens Copy and Select All; Copy puts the selection on the clipboard.
func TestTextSelectionAreaMenuCopies(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "ssh: handshake failed")
	})
	fx, fy := s.start("ssh: handshake failed")
	tx, ty := s.end("ssh: handshake failed")
	s.drag(fx, fy, tx, ty)
	s.must(s.driver.Press(fx+4, fy, gfx.PointerButtonSecondary))
	s.must(s.driver.Release(fx+4, fy, gfx.PointerButtonSecondary))
	s.must(s.driver.Paint())
	copyItem := s.text("Copy")
	s.driver.SetClipboard("")
	s.must(s.driver.Click(copyItem.X+copyItem.Width/2, copyItem.Y+copyItem.Height/2))
	s.must(s.driver.Paint())
	if got := s.driver.Clipboard(); got != "ssh: handshake failed" {
		t.Fatalf("Copy put %q on the clipboard", got)
	}
}

// Shift with an arrow key moves the end of the selection by a character, into the next
// text past the end of one.
func TestTextSelectionAreaShiftArrowExtends(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "ab")
		ui.Text(c, "cd")
	})
	fx, fy := s.start("ab")
	tx, ty := s.end("ab")
	s.drag(fx, fy, tx, ty)
	for range 2 {
		s.must(s.driver.Key(gfx.KeyRight, gfx.ModifierShift))
	}
	s.must(s.driver.Paint())
	if got := s.copied(); got != "ab\nc" {
		t.Fatalf("copied %q", got)
	}
}

// A drag held below the scrolling area the texts are in scrolls it, and the selection
// follows into the texts it brings up.
func TestTextSelectionAreaDragScrollsItsArea(t *testing.T) {
	s := &selectable{t: t}
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = "line " + string(rune('A'+i%26)) + strings.Repeat("x", i)
	}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Scroll(c).Height(200).FillWidth().Children(func() {
			ui.TextSelectionArea(c, &s.area).FillWidth().Children(func() {
				for _, line := range lines {
					ui.Text(c, line)
				}
			})
		})
	}})
	s.open(page)
	x, y := s.start(lines[0])
	s.must(s.driver.Press(x, y, gfx.PointerButtonPrimary))
	s.must(s.driver.Move(x, 150))
	s.must(s.driver.Move(x, 260)) // below the area's 200 pixels
	for range 20 {
		s.must(s.driver.Advance(50 * time.Millisecond))
	}
	s.must(s.driver.Release(x, 260, gfx.PointerButtonPrimary))
	s.must(s.driver.Paint())
	got := s.area.Text()
	if !strings.HasPrefix(got, lines[0]) || strings.Count(got, "\n") < 12 {
		t.Fatalf("the drag selected %d lines: %q", strings.Count(got, "\n")+1, got)
	}
}

// Selecting fills the primary selection where the desktop has one.
func TestTextSelectionAreaFillsThePrimarySelection(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "middle click pastes this")
	})
	if _, err := app.PrimarySelection(); err == app.ErrUnsupported {
		t.Skip("only an X11 desktop has a primary selection")
	}
	fx, fy := s.start("middle click pastes this")
	tx, ty := s.end("middle click pastes this")
	s.drag(fx, fy, tx, ty)
	if got, err := app.PrimarySelection(); err != nil || got != "middle click pastes this" {
		t.Fatalf("the primary selection is %q (%v)", got, err)
	}
}

func selectionPage(area *ui.TextSelectionState) func(ui.C) {
	return func(c ui.C) {
		ui.TextSelectionArea(c, area).FillWidth().Gap(4).Children(func() {
			for i := range 40 {
				ui.Row(c).Gap(6).Children(func() {
					ui.Text(c, detailLabels[i%len(detailLabels)]).Width(120)
					ui.Paragraph(c, "a value long enough to wrap onto a second line at this width", 2).Width(180)
				})
			}
		})
	}
}

var detailLabels = []string{"Address", "Fingerprint", "Type", "Saved"}

// A steady page of selectable texts, with part of them selected, allocates nothing.
func TestTextSelectionAreaPaintAllocatesNothing(t *testing.T) {
	var area ui.TextSelectionState
	view := selectionPage(&area)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, view)
	area.Selection.Select(ui.TextPoint{Surface: 3, Offset: 4}, ui.TextPoint{Surface: 30, Offset: 20})
	for range 3 {
		drawPage(page, canvas, view)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, view) }); allocs != 0 {
		t.Fatalf("a steady page of selectable texts allocates %v times", allocs)
	}
}

// BenchmarkTextSelectionArea is the cost of the same texts in an area and out of one.
func BenchmarkTextSelectionArea(b *testing.B) {
	plain := func(c ui.C) {
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			for i := range 40 {
				ui.Row(c).Gap(6).Children(func() {
					ui.Text(c, detailLabels[i%len(detailLabels)]).Width(120)
					ui.Paragraph(c, "a value long enough to wrap onto a second line at this width", 2).Width(180)
				})
			}
		})
	}
	var area ui.TextSelectionState
	for _, bench := range []struct {
		name string
		view func(ui.C)
	}{{"plain", plain}, {"area", selectionPage(&area)}} {
		b.Run(bench.name, func(b *testing.B) {
			page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
			drawPage(page, canvas, bench.view)
			area.Selection.Select(ui.TextPoint{Surface: 3, Offset: 4}, ui.TextPoint{Surface: 30, Offset: 20})
			b.ReportAllocs()
			for b.Loop() {
				drawPage(page, canvas, bench.view)
			}
		})
	}
}

// A group region that names its content for assistive technology does not take the
// text inside it out of the area, as a button does.
func TestTextSelectionAreaKeepsTheTextOfAGroupRegion(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Row(c).HitWith(ui.ID(c.Part("path")), "Path", gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true, SkipTab: true}).
			Children(func() { ui.Text(c, "/srv/backups/2026-11-26.tar.gz") })
	})
	fx, fy := s.start("/srv/backups/2026-11-26.tar.gz")
	tx, ty := s.end("/srv/backups/2026-11-26.tar.gz")
	s.drag(fx, fy, tx, ty)
	if got := s.copied(); got != "/srv/backups/2026-11-26.tar.gz" {
		t.Fatalf("copied %q", got)
	}
}

// A state copied into another field selects for that field, not for the one it was
// copied from.
func TestTextSelectionAreaStateCopiedByValue(t *testing.T) {
	var first, second ui.TextSelectionState
	showSecond := false
	s := &selectable{t: t}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		area := &first
		if showSecond {
			area = &second
		}
		ui.TextSelectionArea(c, area).FillWidth().Children(func() { ui.Text(c, "copied state") })
	}})
	s.open(page)
	fx, fy := s.start("copied state")
	tx, ty := s.end("copied state")
	s.drag(fx, fy, tx, ty)
	second, showSecond = first, true
	first.Clear()
	s.must(s.driver.Paint())
	s.drag(fx, fy, fx+20, fy)
	if first.Has() || !second.Has() {
		t.Fatalf("first has %q, second %q", first.Text(), second.Text())
	}
}

// Select All from the menu selects everything and gives the area the keyboard back, so
// Primary+C copies what it selected.
func TestTextSelectionAreaMenuGivesTheKeyboardBack(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Text(c, "first")
		ui.Text(c, "second")
	})
	x, y := s.start("first")
	s.must(s.driver.Press(x, y, gfx.PointerButtonSecondary))
	s.must(s.driver.Release(x, y, gfx.PointerButtonSecondary))
	s.must(s.driver.Paint())
	item := s.text("Select All")
	s.must(s.driver.Click(item.X+item.Width/2, item.Y+item.Height/2))
	s.must(s.driver.Paint())
	s.must(s.driver.Paint())
	if got := s.copied(); got != "first\nsecond" {
		t.Fatalf("copied %q after Select All", got)
	}
}

// A drag that begins in the blank part of the area, beside the words, selects from the
// nearest of them.
func TestTextSelectionAreaDragFromTheBlankBeside(t *testing.T) {
	s := openSelectable(t, func(c ui.C) {
		ui.Row(c).Gap(24).Children(func() {
			ui.Spacer(c).Width(30)
			ui.Text(c, "build 2026.11.26")
		})
	})
	r := s.text("build 2026.11.26")
	tx, ty := s.end("build 2026.11.26")
	s.drag(r.X-12, ty, tx, ty)
	if got := s.copied(); got != "build 2026.11.26" {
		t.Fatalf("copied %q", got)
	}
}
