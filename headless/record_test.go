//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// fadingApp paints a rectangle that fades in over duration on the animation
// clock, so every sampled frame moves.
func fadingApp(duration time.Duration) app.Config {
	return app.Config{
		Title: "fade", Width: 64, Height: 48,
		ClearColor: app.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.BeginPresence(gfx.Item(1), 0, gfx.Presence{Motion: gfx.Motion{Duration: duration}, Fade: true}, gfx.Point{})
			canvas.Fill(gfx.Rect{X: 8, Y: 8, Width: 32, Height: 32}, gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
			canvas.EndPresence()
		},
	}
}

func readRecordManifest(t *testing.T, dir string) recordManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, recordManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest recordManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	return manifest
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRecordWritesTheAnimationAsFramesAndAManifest(t *testing.T) {
	if !rendererAvailable {
		t.Skip("no offscreen renderer in this build")
	}
	session, err := Open(fadingApp(100*time.Millisecond), nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err := session.Image(); err != nil {
		t.Skipf("no offscreen renderer on this machine: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "frames")
	frames, err := session.Record(dir, RecordOptions{Duration: 100 * time.Millisecond, FPS: 60})
	if err != nil {
		t.Fatal(err)
	}
	manifest := readRecordManifest(t, dir)
	if manifest.Version != 1 || manifest.FPS != 60 || manifest.Width != 64 || manifest.Height != 48 || manifest.Scale != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if manifest.DurationNS != int64(100*time.Millisecond) {
		t.Fatalf("manifest duration = %d, want %d", manifest.DurationNS, 100*time.Millisecond)
	}
	// 100 ms at 60 fps is six 1/60 s intervals; the fade moves on every one.
	if frames != 6 || len(manifest.Frames) != 6 {
		t.Fatalf("frames = %d, manifest lists %d, want 6", frames, len(manifest.Frames))
	}
	sum := int64(0)
	seen := make(map[string]bool)
	for _, frame := range manifest.Frames {
		if seen[frame.File] {
			t.Fatalf("%q is listed twice", frame.File)
		}
		seen[frame.File] = true
		if frame.DurationNS <= 0 {
			t.Errorf("%q lasts %d ns", frame.File, frame.DurationNS)
		}
		sum += frame.DurationNS
	}
	if sum != manifest.DurationNS {
		t.Fatalf("frame durations sum to %d ns, want %d", sum, manifest.DurationNS)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != frames+1 {
		t.Fatalf("%d files for %d frames, want %d (frames and manifest.json)", len(entries), frames, frames+1)
	}
	first := readFile(t, filepath.Join(dir, manifest.Frames[0].File))
	last := readFile(t, filepath.Join(dir, manifest.Frames[len(manifest.Frames)-1].File))
	if bytes.Equal(first, last) {
		t.Error("every recorded frame is the same: the animation did not move")
	}
}

func TestRecordIsDeterministic(t *testing.T) {
	if !rendererAvailable {
		t.Skip("no offscreen renderer in this build")
	}
	record := func(dir string) (map[string][]byte, []byte) {
		session, err := Open(fadingApp(80*time.Millisecond), nil, Options{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		if _, err := session.Image(); err != nil {
			t.Skipf("no offscreen renderer on this machine: %v", err)
		}
		if _, err := session.Record(dir, RecordOptions{Duration: 120 * time.Millisecond, FPS: 60}); err != nil {
			t.Fatal(err)
		}
		manifest := readRecordManifest(t, dir)
		frames := make(map[string][]byte, len(manifest.Frames))
		for _, frame := range manifest.Frames {
			frames[frame.File] = readFile(t, filepath.Join(dir, frame.File))
		}
		return frames, readFile(t, filepath.Join(dir, recordManifestName))
	}
	firstFrames, firstManifest := record(filepath.Join(t.TempDir(), "one"))
	secondFrames, secondManifest := record(filepath.Join(t.TempDir(), "two"))
	if !bytes.Equal(firstManifest, secondManifest) {
		t.Errorf("two recordings wrote different manifests:\n%s\n%s", firstManifest, secondManifest)
	}
	if len(firstFrames) != len(secondFrames) {
		t.Fatalf("two recordings wrote %d and %d frames", len(firstFrames), len(secondFrames))
	}
	for name, pixels := range firstFrames {
		if !bytes.Equal(pixels, secondFrames[name]) {
			t.Errorf("%s differs between two recordings of the same animation", name)
		}
	}
}

func TestRecordWritesOneFrameForAnApplicationAtRest(t *testing.T) {
	if !rendererAvailable {
		t.Skip("no offscreen renderer in this build")
	}
	session, err := Open(app.Config{
		Title: "still", Width: 40, Height: 30,
		ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Fill(gfx.Rect{Width: 20, Height: 15}, gfx.Color{Red: 1, Alpha: 1})
		},
	}, nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err := session.Image(); err != nil {
		t.Skipf("no offscreen renderer on this machine: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "frames")
	frames, err := session.Record(dir, RecordOptions{Duration: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if frames != 1 {
		t.Fatalf("frames = %d, want 1 for a still application", frames)
	}
	manifest := readRecordManifest(t, dir)
	if manifest.FPS != defaultRecordFPS || len(manifest.Frames) != 1 || manifest.Frames[0].DurationNS != int64(500*time.Millisecond) {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestRecordRequiresARendererAndLeavesNothingBehindWithoutOne(t *testing.T) {
	if rendererAvailable {
		t.Skip("this build has an offscreen renderer")
	}
	session, err := Open(fadingApp(time.Second), nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	dir := filepath.Join(t.TempDir(), "frames")
	if _, err := session.Record(dir, RecordOptions{Duration: time.Second}); !errors.Is(err, ErrNoRenderer) {
		t.Fatalf("Record without a renderer = %v, want ErrNoRenderer", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("a recording without a renderer made %q: %v", dir, err)
	}
}

func TestRecordRejectsBadOptions(t *testing.T) {
	session, err := Open(app.Config{}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	dir := filepath.Join(t.TempDir(), "frames")
	if _, err := session.Record(dir, RecordOptions{}); err == nil || !strings.Contains(err.Error(), "duration must be positive") {
		t.Fatalf("zero duration = %v", err)
	}
	if _, err := session.Record(dir, RecordOptions{Duration: time.Second, FPS: maxRecordFPS + 1}); err == nil || !strings.Contains(err.Error(), "fps must be between") {
		t.Fatalf("fps %d = %v", maxRecordFPS+1, err)
	}
}

func TestControlRecordsAFrameSequence(t *testing.T) {
	if !rendererAvailable {
		t.Skip("no offscreen renderer in this build")
	}
	session, _ := openTestApp(t)
	defer session.Close()
	if _, err := session.Image(); err != nil {
		t.Skipf("no offscreen renderer on this machine: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "frames")
	reply, _ := execute(session, "record "+strconv.Quote(dir)+" 100 30")
	manifest := readRecordManifest(t, dir)
	if manifest.FPS != 30 {
		t.Fatalf("fps = %d, want 30", manifest.FPS)
	}
	want := "ok " + strconv.Itoa(len(manifest.Frames)) + " frames\n"
	if len(manifest.Frames) == 1 {
		want = "ok 1 frame\n"
	}
	if reply != want {
		t.Fatalf("record reply = %q, want %q", reply, want)
	}
}

// Recording one frame is the cost of a screenshot's render plus a PNG encode and
// a file; repeated frames cost the render alone.
func BenchmarkRecordOneFrame(b *testing.B) {
	if !rendererAvailable {
		b.Skip("no offscreen renderer in this build")
	}
	session, err := Open(app.Config{
		Title: "bench", Width: 320, Height: 160,
		ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Fill(gfx.Rect{Width: 120, Height: 40}, gfx.Color{Red: 0.2, Green: 0.4, Blue: 0.9, Alpha: 1})
		},
	}, nil, Options{Scale: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer session.Close()
	if _, err := session.Image(); err != nil {
		b.Skipf("no renderer: %v", err)
	}
	dir := b.TempDir()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := session.Record(dir, RecordOptions{Duration: time.Millisecond}); err != nil {
			b.Fatal(err)
		}
	}
}
