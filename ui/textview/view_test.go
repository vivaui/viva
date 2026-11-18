package textview_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/textview"
)

// output is a view of lines in a headless window, with what its links and trails did.
type output struct {
	t       *testing.T
	driver  *app.Driver
	view    textview.View
	lines   textview.Strings
	link    string // the text a span links, on every line that holds it
	trail   int    // the line a Button trail is drawn after, or -1
	opened  []string
	pressed []int
	copied  []string
	hook    bool // copy through Spec.Copy instead of the clipboard
}

func openOutput(t *testing.T, lines ...string) *output {
	t.Helper()
	o := &output{t: t, lines: textview.Strings(lines), trail: -1}
	page := ui.NewPage(ui.PageConfig{View: o.declare})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 640, Height: 320}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	o.driver = driver
	o.must(driver.Paint())
	return o
}

func (o *output) declare(c ui.C) {
	spec := textview.Spec{
		Label:      "Output",
		Source:     o.lines,
		Font:       gfx.TextStyle{Size: 13, Mono: true, Color: gfx.RGB(0x20, 0x20, 0x20)},
		LineHeight: 18,
		PadX:       4,
		Selection:  gfx.RGBA(0x40, 0x80, 0xff, 0.4),
		Scroll:     true,
		Spans: func(n int, text string, dst []textview.Span) []textview.Span {
			if at := strings.Index(text, o.link); o.link != "" && at >= 0 {
				dst = append(dst, textview.Span{From: at, To: at + len(o.link), Link: true, Color: gfx.RGB(0, 0, 0xc0)})
			}
			return dst
		},
		Open: func(n int, span textview.Span) {
			o.opened = append(o.opened, o.lines.Line(n)[span.From:span.To])
		},
		Trail: func(n int) textview.Trail {
			if n != o.trail {
				return textview.Trail{}
			}
			return textview.Trail{Text: "Show 3 more", Style: gfx.TextStyle{Size: 12}, Button: true}
		},
		PressTrail: func(n int) { o.pressed = append(o.pressed, n) },
	}
	if o.hook {
		spec.Copy = func(text string) { o.copied = append(o.copied, text) }
	}
	o.view.Declare(c, spec).Fill()
}

// at is the window point of byte b of line n.
func (o *output) at(n, b int) (float32, float32) {
	o.t.Helper()
	p, ok := o.view.PointOf(textview.Point{Line: n, Byte: b})
	if !ok {
		o.t.Fatalf("line %d byte %d is not on screen", n, b)
	}
	return p.X, p.Y
}

func (o *output) must(err error) {
	o.t.Helper()
	if err != nil {
		o.t.Fatal(err)
	}
}

func (o *output) drag(fromLine, fromByte, toLine, toByte int) {
	o.t.Helper()
	x, y := o.at(fromLine, fromByte)
	o.must(o.driver.Press(x, y, gfx.PointerButtonPrimary))
	tx, ty := o.at(toLine, toByte)
	o.must(o.driver.Move((x+tx)/2, (y+ty)/2))
	o.must(o.driver.Move(tx, ty))
	o.must(o.driver.Release(tx, ty, gfx.PointerButtonPrimary))
}

// copyShortcut presses Primary+C and returns what reached the clipboard.
func (o *output) copyShortcut() string {
	o.t.Helper()
	o.driver.SetClipboard("")
	_, err := o.driver.ShortcutChord(keymap.Primary, 'c')
	o.must(err)
	o.must(o.driver.Paint())
	return o.driver.Clipboard()
}

func (o *output) chord(character rune) {
	o.t.Helper()
	_, err := o.driver.ShortcutChord(keymap.Primary, character)
	o.must(err)
	o.must(o.driver.Paint())
}

const trace = "Exception in thread \"main\" java.lang.IllegalStateException: boom"

// A drag selects from where it was pressed to where it was released, across lines, and
// Primary+C copies exactly that text; a click selects nothing and Escape drops a selection.
func TestDragAcrossLinesCopiesTheTextBetween(t *testing.T) {
	o := openOutput(t, trace, "\tat demo.Main.run(Main.java:12)", "\tat demo.Main.main(Main.java:5)")
	x, y := o.at(0, 3)
	o.must(o.driver.Click(x, y))
	if o.view.Has() {
		t.Fatalf("a click selected %q", o.view.Text())
	}
	o.drag(0, len("Exception in "), 2, len("\tat demo"))
	want := "thread \"main\" java.lang.IllegalStateException: boom\n\tat demo.Main.run(Main.java:12)\n\tat demo"
	if got := o.copyShortcut(); got != want {
		t.Fatalf("the drag copied %q, want %q", got, want)
	}
	o.must(o.driver.Key(gfx.KeyEscape, 0))
	if o.view.Has() {
		t.Fatalf("Escape kept %q selected", o.view.Text())
	}
	if got := o.copyShortcut(); got != "" {
		t.Fatalf("Primary+C with nothing selected copied %q", got)
	}
}

// A double click selects the word under it, a triple click its whole line.
func TestDoubleClickSelectsAWordAndTripleClickALine(t *testing.T) {
	o := openOutput(t, "first line", "\tat demo.Main.run(Main.java:12)")
	x, y := o.at(1, len("\tat demo.Ma"))
	o.must(o.driver.DoubleClick(x, y))
	if got := o.view.Text(); got != "Main" {
		t.Fatalf("a double click selected %q, want %q", got, "Main")
	}
	o.must(o.driver.ClickCount(x, y, 3))
	if got, want := o.view.Text(), "\tat demo.Main.run(Main.java:12)"; got != want {
		t.Fatalf("a triple click selected %q, want %q", got, want)
	}
}

// Shift with a click extends the selection from where the last click put it, and
// Primary+A selects every line.
func TestShiftClickExtendsAndPrimaryASelectsEverything(t *testing.T) {
	o := openOutput(t, "first line", "second line", "third line")
	x, y := o.at(0, len("first "))
	o.must(o.driver.Click(x, y))
	o.must(o.driver.KeyDown(gfx.KeyShiftLeft, ""))
	x, y = o.at(1, len("second"))
	o.must(o.driver.Click(x, y))
	o.must(o.driver.KeyUp(gfx.KeyShiftLeft, ""))
	if got, want := o.copyShortcut(), "line\nsecond"; got != want {
		t.Fatalf("Shift with a click copied %q, want %q", got, want)
	}
	o.chord('a')
	if got, want := o.copyShortcut(), "first line\nsecond line\nthird line"; got != want {
		t.Fatalf("Primary+A then Primary+C copied %q, want %q", got, want)
	}
}

// A click on a link opens it; a press on a link that drags selects instead and opens
// nothing.
func TestALinkOpensOnAClickAndSelectsOnADrag(t *testing.T) {
	o := openOutput(t, "\tat demo.Main.run(Main.java:12)")
	o.link = "Main.java:12"
	o.must(o.driver.Paint())
	from := strings.Index(o.lines[0], o.link)
	x, y := o.at(0, from+2)
	o.must(o.driver.Click(x, y))
	if len(o.opened) != 1 || o.opened[0] != o.link {
		t.Fatalf("a click on the link opened %q, want [%q]", o.opened, o.link)
	}
	o.drag(0, from+2, 0, from+len("Main.java"))
	if len(o.opened) != 1 {
		t.Fatalf("a drag from the link opened it again: %q", o.opened)
	}
	if got, want := o.view.Text(), "in.java"; got != want {
		t.Fatalf("a drag from the link selected %q, want %q", got, want)
	}
}

// A Button trail is pressed by a click, and is never part of what is selected or copied.
func TestATrailIsPressedButNeverCopied(t *testing.T) {
	o := openOutput(t, "Receiving objects", "done")
	o.trail = 0
	o.must(o.driver.Paint())
	hit := trailHit(t, o.driver, "Show 3 more")
	o.must(o.driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2))
	if len(o.pressed) != 1 || o.pressed[0] != 0 {
		t.Fatalf("a click on the trail pressed %v, want [0]", o.pressed)
	}
	o.chord('a')
	if got, want := o.copyShortcut(), "Receiving objects\ndone"; got != want {
		t.Fatalf("select all copied %q, want %q", got, want)
	}
}

// Spec.Copy takes what Primary+C copies, so the caller can record it or rewrite it before
// it reaches the clipboard.
func TestCopyGoesThroughTheSpecHook(t *testing.T) {
	o := openOutput(t, "alpha beta")
	o.hook = true
	o.must(o.driver.Paint())
	x, y := o.at(0, len("alpha b"))
	o.must(o.driver.DoubleClick(x, y))
	if got := o.copyShortcut(); got != "" {
		t.Fatalf("a copy through the hook still reached the clipboard: %q", got)
	}
	if len(o.copied) != 1 || o.copied[0] != "beta" {
		t.Fatalf("the hook got %q, want [\"beta\"]", o.copied)
	}
}

func trailHit(t *testing.T, driver *app.Driver, label string) gfx.Hit {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label && !hit.IsLabel && !hit.ID.IsZero() {
			return hit
		}
	}
	t.Fatalf("no control labeled %q", label)
	return gfx.Hit{}
}
