package ui_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type fileDropModel struct {
	outer, inner ui.FileDropState
	hidden       bool
}

func (m *fileDropModel) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		if m.hidden {
			return
		}
		ui.Column(c).Height(120).FillWidth().Hit(ui.ID(c), "Outer").DropFiles(&m.outer).Children(func() {
			ui.Row(c).Height(40).FillWidth().Hit(ui.ID(c), "Inner").DropFiles(&m.inner)
		})
		ui.Row(c).Height(30).FillWidth().Hit(ui.ID(c), "Elsewhere")
	})
}

func dropAt(t *testing.T, d *pageDriver, phase gfx.FileDropPhase, label string, paths ...string) bool {
	t.Helper()
	x, y := center(d.hit(label).Bounds)
	taken, err := d.driver.Drop(gfx.FileDropEvent{Phase: phase, X: x, Y: y, Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	return taken
}

func TestFileDropAreaTakesFilesOverIt(t *testing.T) {
	m := &fileDropModel{}
	d := openPage(t, m.page)
	if !dropAt(t, d, gfx.FileDropOver, "Outer") {
		t.Fatal("files over an area are not accepted")
	}
	if !m.outer.Over() {
		t.Fatal("no area reports the files over it")
	}
	if dropAt(t, d, gfx.FileDropOver, "Elsewhere") {
		t.Fatal("files over no area are accepted")
	}
	if m.outer.Over() || m.inner.Over() {
		t.Fatal("an area still reports files after they moved away")
	}
}

func TestFileDropGoesToTheInnermostLastDeclaredArea(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("a browser gives dropped files without paths, so a test cannot drop them by path")
	}
	m := &fileDropModel{}
	d := openPage(t, m.page)
	dropAt(t, d, gfx.FileDropOver, "Inner")
	if !m.inner.Over() || m.outer.Over() {
		t.Fatalf("over: inner %v outer %v, want only the area declared last", m.inner.Over(), m.outer.Over())
	}
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !dropAt(t, d, gfx.FileDropDrop, "Inner", path) {
		t.Fatal("the drop is not accepted")
	}
	files, paths := m.inner.Dropped()
	if len(files) != 1 || files[0].Name() != "a.txt" || len(paths) != 1 || paths[0] != path {
		t.Fatalf("Dropped = %v, %v", files, paths)
	}
	if got := ui.FilePath(files[0]); got != path {
		t.Fatalf("FilePath = %q, want %q", got, path)
	}
	if again, _ := m.inner.Dropped(); again != nil {
		t.Fatal("Dropped returned the same drop twice")
	}
	if m.inner.Over() {
		t.Fatal("an area still reports files after the drop")
	}
	if got, _ := m.outer.Dropped(); got != nil {
		t.Fatal("the area underneath took the drop too")
	}
}

func TestFileDropLeaveClearsOver(t *testing.T) {
	m := &fileDropModel{}
	d := openPage(t, m.page)
	dropAt(t, d, gfx.FileDropOver, "Outer")
	if _, err := d.driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropLeave}); err != nil {
		t.Fatal(err)
	}
	if m.outer.Over() || m.inner.Over() {
		t.Fatal("leaving the window left an area over")
	}
}

func TestFileDropFallsThroughToTheWindowsOwnHandler(t *testing.T) {
	m := &fileDropModel{}
	var seen int
	d := openPageWith(t, ui.PageConfig{View: m.page}, app.Config{OnFileDrop: func(e gfx.FileDropEvent) gfx.EventResult {
		seen++
		return gfx.EventResult{Handled: true}
	}})
	if !dropAt(t, d, gfx.FileDropOver, "Elsewhere") || seen != 1 {
		t.Fatalf("the window's handler saw %d events and the area-less point was not its own", seen)
	}
	dropAt(t, d, gfx.FileDropOver, "Outer")
	if seen != 1 {
		t.Fatal("the window's handler saw a drop an area took")
	}
}

func TestFileDropAreaNotDeclaredAnymoreTakesNothing(t *testing.T) {
	m := &fileDropModel{}
	d := openPage(t, m.page)
	m.hidden = true
	d.invalidator.Invalidate()
	if err := d.driver.Click(1, 1); err != nil { // paint again
		t.Fatal(err)
	}
	if taken, err := d.driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropOver, X: 60, Y: 60}); err != nil || taken {
		t.Fatalf("an undeclared area took files: %v %v", taken, err)
	}
}

func TestFileDropOverDoesNotAllocate(t *testing.T) {
	m := &fileDropModel{}
	d := openPage(t, m.page)
	event := gfx.FileDropEvent{Phase: gfx.FileDropOver, X: 60, Y: 60}
	if n := testing.AllocsPerRun(50, func() { d.driver.Drop(event) }); n != 0 {
		t.Fatalf("a hover with files allocates %v times", n)
	}
}
