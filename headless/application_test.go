//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestApplicationSessionRoutesStateFocusAndClosePerWindow(t *testing.T) {
	application := newApplicationSession(Options{Scale: 1})
	t.Cleanup(application.closeAll)
	firstEditor := &text.Editor{}
	secondEditor := &text.Editor{}
	firstClose, err := application.OpenWindow(editingWindow("First", 11, firstEditor), nil, func() {})
	if err != nil {
		t.Fatal(err)
	}
	secondClose, err := application.OpenWindow(editingWindow("Second", 22, secondEditor), nil, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer firstClose.Close()
	defer secondClose.Close()

	commands := strings.NewReader("window 1\nclick \"First field\"\ntype alpha\nwindow 2\nclick \"Second field\"\ntype beta\nwindow 1\neditor\nclosewindow 2\nwindows\nclosewindow 1\n")
	var output bytes.Buffer
	if err := ServeApplication(application, commands, &output); err != nil {
		t.Fatal(err)
	}
	if firstEditor.String() != "alpha" || secondEditor.String() != "beta" {
		t.Fatalf("editors first=%q second=%q", firstEditor.String(), secondEditor.String())
	}
	text := output.String()
	for _, fragment := range []string{"ok active=1", "ok active=2", `value="alpha"`, "ok closed=2 windows=1", "ok windows=1 active=1", "ok closed=1 windows=0"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output missing %q:\n%s", fragment, text)
		}
	}
}

func editingWindow(title string, hit uint32, editor *text.Editor) app.Config {
	return app.Config{
		Title: title, Width: 240, Height: 100,
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			canvas.Label(title+" "+editor.String(), ui.Rect{X: 8, Y: 8, Width: 220, Height: 20}, ui.TextStyle{Size: 14})
			canvas.HitWithOptions(ui.Item(int(hit)), ui.Rect{X: 8, Y: 32, Width: 200, Height: 30}, title+" field", ui.HitOptions{Role: ui.RoleTextField})
		},
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if context.ID != ui.Item(int(hit)) {
				return nil
			}
			return &app.TextInput{Editor: editor}
		},
	}
}

func BenchmarkApplicationSessionIdleSnapshots(b *testing.B) {
	application := newApplicationSession(Options{Scale: 1})
	b.Cleanup(application.closeAll)
	firstClose, err := application.OpenWindow(app.Config{Width: 200, Height: 100}, nil, func() {})
	if err != nil {
		b.Fatal(err)
	}
	secondClose, err := application.OpenWindow(app.Config{Width: 200, Height: 100}, nil, func() {})
	if err != nil {
		b.Fatal(err)
	}
	defer firstClose.Close()
	defer secondClose.Close()
	_, windows := application.describe()
	b.ReportAllocs()
	for b.Loop() {
		for _, window := range windows {
			_ = application.windows[window.id].session.Snapshot()
		}
	}
}

func TestApplicationSessionIdleWindowsDoNotRepaint(t *testing.T) {
	application := newApplicationSession(Options{Scale: 1})
	t.Cleanup(application.closeAll)
	paints := [2]int{}
	closes := make([]app.WindowControl, 0, 2)
	for index := range paints {
		config := app.Config{Width: 200, Height: 100}
		config.Paint = func(*ui.Canvas, ui.Size) { paints[index]++ }
		closeWindow, err := application.OpenWindow(config, nil, func() {})
		if err != nil {
			t.Fatal(err)
		}
		closes = append(closes, closeWindow)
	}
	defer func() {
		for _, closeWindow := range closes {
			closeWindow.Close()
		}
	}()
	if paints != [2]int{1, 1} {
		t.Fatalf("initial paints = %v", paints)
	}
	for _, window := range application.windows {
		if err := window.session.Wait(time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if paints != [2]int{1, 1} {
		t.Fatalf("idle paints = %v", paints)
	}
}

func TestDockBadgeCommandReportsWhatTheDockWasAskedToShow(t *testing.T) {
	application := newApplicationSession(Options{Scale: 1})
	t.Cleanup(application.closeAll)
	closed, err := application.OpenWindow(app.Config{Title: "main", Width: 200, Height: 100}, nil, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer closed.Close()
	if text := run(t, application, "dockbadge\n"); !strings.Contains(text, `ok label=""`) {
		t.Fatalf("unset badge: %q", text)
	}
	application.SetDockBadge("DEV")
	if text := run(t, application, "dockbadge\n"); !strings.Contains(text, `ok label="DEV"`) {
		t.Fatalf("set badge: %q", text)
	}
	application.SetDockBadge("")
	if text := run(t, application, "dockbadge\n"); !strings.Contains(text, `ok label=""`) {
		t.Fatalf("cleared badge: %q", text)
	}
}

func TestProgressCommandReportsWhatTheIconWasAskedToShow(t *testing.T) {
	application := newApplicationSession(Options{Scale: 1})
	t.Cleanup(application.closeAll)
	closed, err := application.OpenWindow(app.Config{Title: "main", Width: 200, Height: 100}, nil, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer closed.Close()
	if text := run(t, application, "progress\n"); !strings.Contains(text, "ok fraction=-1") {
		t.Fatalf("no bar before any progress: %q", text)
	}
	application.SetProgress(0.5)
	if text := run(t, application, "progress\n"); !strings.Contains(text, "ok fraction=0.5") {
		t.Fatalf("half: %q", text)
	}
	application.SetProgress(-1)
	if text := run(t, application, "progress\n"); !strings.Contains(text, "ok fraction=-1") {
		t.Fatalf("cleared: %q", text)
	}
}
