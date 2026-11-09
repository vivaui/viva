//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/textmetrics"
)

func layoutSession(t *testing.T, paint func(canvas *gfx.Canvas)) *Session {
	t.Helper()
	session, err := Open(app.Config{
		Title: "layout", Width: 320, Height: 160,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) { paint(canvas) },
	}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func layoutKinds(problems []LayoutProblem) map[LayoutKind]int {
	kinds := map[LayoutKind]int{}
	for _, problem := range problems {
		kinds[problem.Kind]++
	}
	return kinds
}

func TestCheckLayoutAcceptsAFrameWhoseTextFitsItsControls(t *testing.T) {
	session, _ := openTestApp(t)
	if problems := CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestCheckLayoutFindsTextThatDoesNotFit(t *testing.T) {
	style := gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}}
	session := layoutSession(t, func(canvas *gfx.Canvas) {
		// Wider than the button it is in.
		canvas.Hit(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 100, Height: 24})
		canvas.Label("A very long caption", gfx.Rect{X: 14, Y: 12, Width: 200, Height: 20}, style)
		// Cut by a clip.
		canvas.Clip(gfx.Rect{X: 10, Y: 50, Width: 40, Height: 20})
		canvas.Label("Cut off by the clip", gfx.Rect{X: 10, Y: 50, Width: 200, Height: 20}, style)
		canvas.Unclip()
		// Two lines on the same spot.
		canvas.Label("First", gfx.Rect{X: 150, Y: 90, Width: 80, Height: 20}, style)
		canvas.Label("Second", gfx.Rect{X: 160, Y: 95, Width: 80, Height: 20}, style)
		// Past the right edge of the window.
		canvas.Label("Runs off the window", gfx.Rect{X: 250, Y: 130, Width: 200, Height: 20}, style)
		// Two buttons that cross.
		canvas.Hit(gfx.Item(2), gfx.Rect{X: 10, Y: 100, Width: 60, Height: 30})
		canvas.Hit(gfx.Item(3), gfx.Rect{X: 50, Y: 110, Width: 60, Height: 30})
	})
	kinds := layoutKinds(CheckLayout(session.Snapshot()))
	for _, kind := range []LayoutKind{LayoutSpill, LayoutClipped, LayoutOverlap, LayoutOffscreen, LayoutCollide} {
		if kinds[kind] == 0 {
			t.Errorf("no %s problem found; kinds = %v", kind, kinds)
		}
	}
}

func TestCheckLayoutIgnoresTextOnAnotherLayerAndCenteredText(t *testing.T) {
	style := gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}}
	session := layoutSession(t, func(canvas *gfx.Canvas) {
		canvas.Label("Below", gfx.Rect{X: 20, Y: 20, Width: 100, Height: 20}, style)
		canvas.PushLayer(1)
		canvas.Label("Above", gfx.Rect{X: 20, Y: 20, Width: 100, Height: 20}, style)
		canvas.PopLayer()
		// The box is wide and the line sits in its middle: the box is not the text.
		canvas.Hit(gfx.Item(1), gfx.Rect{X: 20, Y: 60, Width: 120, Height: 24})
		canvas.LabelCenter("OK", gfx.Rect{X: 20, Y: 62, Width: 120, Height: 20}, style)
	})
	if problems := CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestServeLayoutListsProblemsAndNeverFails(t *testing.T) {
	session := layoutSession(t, func(canvas *gfx.Canvas) {
		canvas.Label("Runs off the window", gfx.Rect{X: 250, Y: 20, Width: 200, Height: 20}, gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}})
	})
	var out bytes.Buffer
	if err := Serve(session, strings.NewReader("layout\nquit\n"), &out); err != nil {
		t.Fatal(err)
	}
	reply := out.String()
	if !strings.HasPrefix(reply, "offscreen ") || !strings.Contains(reply, `"Runs off the window"`) || !strings.Contains(reply, "ok 1 problems\n") {
		t.Fatalf("reply = %q", reply)
	}
}

// A snapshot's painted widths come from the platform font engine, which can be
// slightly narrower than the reference metrics table for CJK runs. Sequentially
// laid-out spans measured by the engine must not be reported as overlapping.
func TestCheckLayoutPrefersPaintedTextWidths(t *testing.T) {
	style := gfx.TextStyle{Size: 16, Color: gfx.Color{Alpha: 1}}
	const first = "如果还需要详细的介绍，请查看"
	approximate := textmetrics.Width(first, style.Size, style.Weight)
	painted := approximate - 1.4
	snapshot := app.Snapshot{
		Width: 640, Height: 100,
		Texts: []gfx.Text{
			{Value: first, Bounds: gfx.Rect{X: 10, Y: 20, Width: 400, Height: 21}, Style: style},
			{Value: " 芒小智帮助文档", Bounds: gfx.Rect{X: 10 + painted, Y: 20, Width: 400, Height: 21}, Style: style},
		},
	}
	if kinds := layoutKinds(CheckLayout(snapshot)); kinds[LayoutOverlap] == 0 {
		t.Fatal("a hand-built snapshot without painted widths should keep using reference metrics")
	}
	snapshot.TextWidths = []float32{painted, 120}
	if kinds := layoutKinds(CheckLayout(snapshot)); kinds[LayoutOverlap] != 0 {
		t.Fatalf("painted widths still report a false overlap: %+v", kinds)
	}
}

func TestCheckLayoutIgnoresALineThatAScrolledAreaShowsHalfOf(t *testing.T) {
	session := layoutSession(t, func(canvas *gfx.Canvas) {
		canvas.Clip(gfx.Rect{X: 0, Y: 0, Width: 320, Height: 100})
		canvas.Label("Half way out", gfx.Rect{X: 10, Y: 90, Width: 200, Height: 20}, gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}})
		canvas.Unclip()
	})
	if problems := CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("problems = %+v", problems)
	}
}
