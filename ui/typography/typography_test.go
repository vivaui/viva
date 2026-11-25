//go:build !js

package typography_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/typography"
)

var long = strings.Repeat("Keeps scene nodes and repaints only when the content changes. ", 8)

func open(t *testing.T, tx *typography.Text, value string, spec typography.Spec) *headless.Session {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { tx.View(c, value, spec).FillWidth() })
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Text", Width: 420, Height: 360}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	session.Frame() // the second paint knows the width and so whether the text is cut
	return session
}

func lines(session *headless.Session) int {
	n := 0
	for _, text := range session.Driver().Snapshot().Texts {
		if len(text.Value) > 20 {
			n++
		}
	}
	return n
}

// A text longer than Rows is cut to Rows lines with a button to read the rest, and
// the button takes the rest away again.
func TestLongTextIsCutAndExpands(t *testing.T) {
	tx := typography.New()
	session := open(t, tx, long, typography.Spec{Rows: 2, Expandable: true})
	if got := lines(session); got != 2 {
		t.Fatalf("lines shown = %d, want 2", got)
	}
	if err := session.ClickLabel("More"); err != nil {
		t.Fatal(err)
	}
	if !tx.Expanded() || lines(session) <= 2 {
		t.Fatalf("expanded %v, lines %d after More; want all the lines", tx.Expanded(), lines(session))
	}
	if err := session.ClickLabel("Less"); err != nil {
		t.Fatal(err)
	}
	if tx.Expanded() || lines(session) != 2 {
		t.Fatalf("expanded %v, lines %d after Less; want cut to 2", tx.Expanded(), lines(session))
	}
}

// A text that fits in Rows offers nothing to expand.
func TestShortTextHasNoMoreButton(t *testing.T) {
	session := open(t, typography.New(), "A short line.", typography.Spec{Rows: 2, Expandable: true})
	if _, err := session.Locate("More"); err == nil {
		t.Fatal("a text that fits offers More")
	}
}

// The copy button puts the whole text, not the cut one, on the clipboard, and
// Copied is reported in the paint that delivers the press.
func TestCopyPutsTheWholeTextOnTheClipboard(t *testing.T) {
	tx := typography.New()
	copied := 0
	spec := typography.Spec{Rows: 2, Copyable: true}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			tx.View(c, long, spec).FillWidth()
			if tx.Copied() {
				copied++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Text", Width: 420, Height: 360}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.ClickLabel("Copy"); err != nil {
		t.Fatal(err)
	}
	if copied != 1 {
		t.Fatalf("Copied was reported in %d paints after one press, want 1", copied)
	}
	if got := session.Driver().Clipboard(); got != long {
		t.Fatalf("clipboard holds %d characters, want the whole text (%d)", len(got), len(long))
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	tx := typography.New()
	spec := typography.Spec{Rows: 2, Expandable: true, Copyable: true}
	view := func(c ui.C) { tx.View(c, long, spec).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 360}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}
