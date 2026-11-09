//go:build darwin || windows || linux

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func TestFileDropReachesTheHitUnderThePointAndAnswersWhetherItTakesFiles(t *testing.T) {
	var events []gfx.FileDropEvent
	repaints := 0
	config := Config{Title: "drop", Width: 300, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		repaints++
		theme := gfx.LightTheme()
		components.BeginButton(canvas, theme, gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 90, Height: 32}).Default("Folder")
		components.BeginButton(canvas, theme, gfx.Item(2), gfx.Rect{X: 120, Y: 10, Width: 90, Height: 32}).Secondary("Other")
	}
	config.OnFileDrop = func(event gfx.FileDropEvent) gfx.EventResult {
		events = append(events, event)
		// Only the first button takes files; a hover over it asks for a repaint, to
		// show where they would land.
		return gfx.EventResult{Handled: event.Target == gfx.Item(1), Repaint: event.Phase == gfx.FileDropOver && event.Target == gfx.Item(1)}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	before := repaints
	accepted, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropOver, X: 20, Y: 20})
	if err != nil || !accepted {
		t.Fatalf("hover over the folder = %v, %v", accepted, err)
	}
	if repaints == before {
		t.Fatal("the hover asked for a repaint and got none")
	}
	if accepted, _ := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropOver, X: 150, Y: 20}); accepted {
		t.Fatal("a hover over a button that takes nothing was accepted")
	}
	if accepted, _ := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropOver, X: 280, Y: 90}); accepted {
		t.Fatal("a hover over empty space was accepted")
	}
	if _, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropLeave}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dropped := filepath.Join(dir, "dropped.txt")
	if err := os.WriteFile(dropped, []byte("dropped"), 0o600); err != nil {
		t.Fatal(err)
	}
	accepted, err = driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 20, Y: 20, Paths: []string{dropped, dir}})
	if err != nil || !accepted {
		t.Fatalf("drop on the folder = %v, %v", accepted, err)
	}
	last := events[len(events)-1]
	if last.Phase != gfx.FileDropDrop || last.Target != gfx.Item(1) || len(last.Paths) != 2 || last.Paths[1] != dir {
		t.Fatalf("drop event = %+v", last)
	}
	if len(last.Files) != 2 || last.Files[0].Name() != "dropped.txt" || !last.Files[1].IsDir() {
		t.Fatalf("drop files = %+v", last.Files)
	}
	for _, event := range events {
		if event.Phase == gfx.FileDropLeave && (!event.Target.IsZero() || len(event.Paths) != 0) {
			t.Fatalf("leave event = %+v", event)
		}
		if event.Phase == gfx.FileDropOver && (len(event.Paths) != 0 || len(event.Files) != 0) {
			t.Fatalf("hover carried files: %+v", event)
		}
	}
}

func TestAWindowWithoutAFileDropHandlerAcceptsNothing(t *testing.T) {
	config := Config{Title: "drop", Width: 200, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		components.BeginButton(canvas, gfx.LightTheme(), gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 90, Height: 32}).Default("A")
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if accepted, err := driver.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: 20, Y: 20, Paths: []string{"/x"}}); err != nil || accepted {
		t.Fatalf("accepted = %v, %v", accepted, err)
	}
}
