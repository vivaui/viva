//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// stillApp paints one control on a static page, so only the input overlay can
// make a recorded frame differ from another.
func stillApp() app.Config {
	return app.Config{
		Title: "still", Width: 240, Height: 140,
		ClearColor: app.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1},
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			canvas.Fill(ui.Rect{X: 20, Y: 20, Width: 80, Height: 30}, ui.Color{Red: 0.3, Green: 0.3, Blue: 0.3, Alpha: 1})
			canvas.Hit(ui.Item(1), ui.Rect{X: 20, Y: 20, Width: 80, Height: 30})
		},
	}
}

func openStill(t *testing.T) *Session {
	t.Helper()
	if !rendererAvailable {
		t.Skip("no offscreen renderer in this build")
	}
	session, err := Open(stillApp(), nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	if _, err := session.Image(); err != nil {
		t.Skipf("no offscreen renderer on this machine: %v", err)
	}
	return session
}

func recordedFrames(t *testing.T, session *Session, show bool, duration time.Duration) []recordFrame {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "frames")
	if _, err := session.Record(dir, RecordOptions{Duration: duration, FPS: 60, ShowInput: show}); err != nil {
		t.Fatal(err)
	}
	return readRecordManifest(t, dir).Frames
}

func pixelsOf(t *testing.T, session *Session, show bool) *image.RGBA {
	t.Helper()
	session.showInput = show
	defer func() { session.showInput = false }()
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	return picture
}

func TestRecordShowsAPressWhileItFades(t *testing.T) {
	session := openStill(t)
	baseline := pixelsOf(t, session, false)
	if err := session.Click(60, 35); err != nil {
		t.Fatal(err)
	}
	if got := len(recordedFrames(t, session, false, 300*time.Millisecond)); got != 1 {
		t.Fatalf("a still page recorded as %d frames without the overlay", got)
	}
	if err := session.Click(150, 100); err != nil {
		t.Fatal(err)
	}
	if got := len(recordedFrames(t, session, true, 300*time.Millisecond)); got < 2 {
		t.Fatalf("the ring of a press did not move: %d frame", got)
	}
	if err := session.Wait(time.Second); err != nil {
		t.Fatal(err)
	}
	if after := pixelsOf(t, session, true); !bytes.Equal(after.Pix, baseline.Pix) {
		t.Fatal("the ring of a press was still drawn a second later")
	}
}

func TestRecordShowsTheKeysTyped(t *testing.T) {
	session := openStill(t)
	baseline := pixelsOf(t, session, false)
	if err := session.Keys("tab"); err != nil {
		t.Fatal(err)
	}
	withChip := pixelsOf(t, session, true)
	if bytes.Equal(withChip.Pix, baseline.Pix) {
		t.Fatal("the keys that were pressed are not drawn")
	}
	if err := session.Wait(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if after := pixelsOf(t, session, true); !bytes.Equal(after.Pix, baseline.Pix) {
		t.Fatal("the chip of the keys stayed past its time")
	}
}

func TestRecordWithoutShowInputDrawsNothingOfTheInput(t *testing.T) {
	session := openStill(t)
	baseline := pixelsOf(t, session, false)
	if err := session.Click(60, 35); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("tab"); err != nil {
		t.Fatal(err)
	}
	if after := pixelsOf(t, session, false); !bytes.Equal(after.Pix, baseline.Pix) {
		t.Fatal("input was drawn into a frame that did not ask for it")
	}
}

func TestMarksOfALongSessionAreBounded(t *testing.T) {
	session := openStill(t)
	for range 500 {
		if err := session.Click(60, 35); err != nil {
			t.Fatal(err)
		}
		if err := session.Wait(100 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	if len(session.marks) > int(keysLife/(100*time.Millisecond))+1 {
		t.Fatalf("a long session keeps %d marks", len(session.marks))
	}
}

func TestKeysLabelReadsAsAChord(t *testing.T) {
	for spec, want := range map[string]string{
		"cmd+c": "Cmd+C", "shift+tab": "Shift+Tab", "tab": "Tab", "x": "x", "cmd++": "Cmd++", "+": "+",
	} {
		if got := keysLabel(spec); got != want {
			t.Errorf("keysLabel(%q) = %q, want %q", spec, got, want)
		}
	}
}

func TestProtocolRecordTakesInputAfterTheFramesPerSecond(t *testing.T) {
	session := openStill(t)
	dir := filepath.Join(t.TempDir(), "frames")
	var out strings.Builder
	script := "click 150 100\nrecord " + dir + " 300 60 input\nrecord " + dir + " 300 loud\nquit\n"
	if err := Serve(session, strings.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[1], "ok ") || strings.HasPrefix(lines[1], "ok 1 frame") {
		t.Fatalf("a recording with input of a press came back as %q", lines)
	}
	if !strings.HasPrefix(lines[2], "error") || !strings.Contains(lines[2], "neither fps nor input") {
		t.Fatalf("an unknown option was answered with %q", lines[2])
	}
}
