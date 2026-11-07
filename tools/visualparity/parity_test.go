//go:build darwin || windows || linux

package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var update = flag.Bool("update", false, "rewrite the golden layout from the machine the test runs on")

const (
	goldenPath = "testdata/layout.json"
)

// The data is embedded, so a test binary built on one machine and run on another
// (tools/testenv/go-test.sh) finds it.
//
//go:embed testdata/layout.json
var goldenData []byte

// round keeps the golden small: hundredths of a pixel are far below the tolerance.
func round(v float32) float32 { return float32(math.Round(float64(v)*100) / 100) }

func roundFacts(f *Facts) {
	r := func(b *Box) { b.X, b.Y, b.W, b.H = round(b.X), round(b.Y), round(b.W), round(b.H) }
	for i := range f.Texts {
		r(&f.Texts[i].Bounds)
		f.Texts[i].Canvas = round(f.Texts[i].Canvas)
	}
	for i := range f.Rectangles {
		r(&f.Rectangles[i].Bounds)
		f.Rectangles[i].Radius = round(f.Rectangles[i].Radius)
	}
	for i := range f.Hits {
		r(&f.Hits[i].Bounds)
	}
	for i := range f.Triangles {
		for p := range f.Triangles[i].Points {
			for c := range f.Triangles[i].Points[p] {
				f.Triangles[i].Points[p][c] = round(f.Triangles[i].Points[p][c])
			}
		}
	}
}

// layoutFacts lays every scene out in the given theme.
func layoutFacts(t *testing.T, dark bool) map[string]*Facts {
	t.Helper()
	facts := map[string]*Facts{}
	for _, sc := range catalogue() {
		f, _, err := renderScene(sc, dark, 1, false, true, nil)
		if err != nil {
			t.Fatalf("%s: %v", sc.name, err)
		}
		roundFacts(f)
		f.Platform = ""
		facts[sc.name] = f
	}
	return facts
}

// The layout of every scene, text included, is the same on macOS, Windows and Linux:
// positions, sizes and corner radii of texts, rectangles, icons and hit regions, the
// width of every button, tab and table column that is as wide as its label and every
// line a paragraph breaks into, within half a pixel of the golden file. Viva measures
// text with its own reference metrics, not with the platform's fonts, so
// the golden does not depend on the machine that wrote it. It fails when a platform
// rounds, snaps or measures something differently; regenerate with -update when the
// layout is meant to change.
func TestLayoutIsTheSameOnEveryPlatform(t *testing.T) {
	got := layoutFacts(t, false)
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	var want map[string]*Facts
	if err := json.Unmarshal(goldenData, &want); err != nil {
		t.Fatalf("%v (regenerate with: go test ./tools/visualparity -run LayoutIsTheSame -update)", err)
	}
	if len(want) != len(got) {
		t.Fatalf("the golden has %d scenes, the catalogue %d", len(want), len(got))
	}
	for name, facts := range got {
		golden := want[name]
		if golden == nil {
			t.Errorf("%s: no golden", name)
			continue
		}
		for kind, res := range compareKinds(golden, facts) {
			if len(res.leftOnly)+len(res.rightOnly) > 0 {
				t.Errorf("%s: %d %ss in the golden only, %d here only (first: %s)", name, len(res.leftOnly), kind, len(res.rightOnly), firstUnpaired(golden, facts, kind, res))
			}
			shown := 0
			for _, pair := range res.pairs {
				if axes, drifting := res.drifting[pair[0]]; drifting && shown < 3 {
					shown++
					dx, dy, dw, dh := itemDelta(golden, facts, kind, pair)
					t.Errorf("%s: %s drifts in %s by (%.2f, %.2f, %.2f, %.2f) px", name, describe(golden, kind, pair[0]), axes, dx, dy, dw, dh)
				}
			}
		}
	}
}

func firstUnpaired(golden, got *Facts, kind string, res *kindResult) string {
	if len(res.leftOnly) > 0 {
		return describe(golden, kind, res.leftOnly[0])
	}
	return describe(got, kind, res.rightOnly[0])
}

// Dark and light are the same page in other colors: nothing moves.
func TestDarkThemeLaysOutLikeLight(t *testing.T) {
	light, dark := layoutFacts(t, false), layoutFacts(t, true)
	uncolor := func(f *Facts) {
		for i := range f.Rectangles {
			f.Rectangles[i].Color = Hue{}
		}
		for i := range f.Triangles {
			f.Triangles[i].Color = Hue{}
		}
	}
	for name, l := range light {
		d := dark[name]
		uncolor(l)
		uncolor(d)
		for kind, res := range compareKinds(l, d) {
			if len(res.leftOnly)+len(res.rightOnly) > 0 || len(res.drifting) > 0 {
				t.Errorf("%s: the dark theme moves %d of the %d %ss and has %d unpaired", name, len(res.drifting), len(res.pairs), kind, len(res.leftOnly)+len(res.rightOnly))
			}
		}
	}
}

// A label is as wide as the platform's text engine says it is: layout, caret and hit-testing
// come from the same shaped line that is drawn, so the ink of a line starts where
// the engine says it starts and ends inside the advance box of its last glyph: it may stop
// short of the engine's width by that glyph's side bearing, which is the font's, but not by
// more, and it may not stop before the glyph begins. A font that does not scale with the
// size, a semibold emboldened from regular, or a draw path that shapes the line differently
// from the measuring one would leave the band.
func TestLabelInkEndsWhereTheEngineSaysTheLineEnds(t *testing.T) {
	engine := app.PlatformTextEngine()
	if engine == nil {
		t.Skip("this platform has no text engine yet")
	}
	sc := &scene{name: "fit", width: 640, height: 1400, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(16).Gap(4).Children(func() {
			for _, size := range []float32{11, 13, 14, 16, 20} {
				for _, w := range weights {
					f.Text("Settings and Preferences").FontSize(size).Weight(w.weight)
					f.Text("2213123中文 192.168.1.1").FontSize(size).Weight(w.weight)
				}
			}
		})
	}}
	facts, picture, err := renderScene(sc, false, 1, true, false, nil)
	if noRenderer(err) {
		t.Skip("no offscreen renderer on this machine or in this build (macOS needs -tags vivahead, Linux needs a Vulkan driver)")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range facts.Texts {
		width := float64(engine.Width(text.Value, text.Size, text.Weight))
		box := text.Bounds
		box.W = float32(width) + 8 // room to see ink that runs past the engine's width
		_, _, left, right, ok := inkRows(picture, 1, box)
		if !ok {
			t.Errorf("%q at %g: nothing was drawn", text.Value, text.Size)
			continue
		}
		slack := float64(text.Size) * 0.2 // the side bearing of the first glyph
		last := lastClusterOrigin(engine, text)
		if left > slack || float64(right) < last || right > width+slack {
			t.Errorf("%q at %g weight %g: ink from %.1f to %.1f, engine width %.1f, last glyph at %.1f", text.Value, text.Size, text.Weight, left, right, width, last)
		}
	}
}

// lastClusterOrigin is the x where the last cluster of text begins, as the engine's own
// caret reports it. The ink has to reach it; how far past it the ink stops short of the
// line's width is the last glyph's side bearing, and that is the font's to decide.
func lastClusterOrigin(engine ui.TextEngine, text TextFact) float64 {
	width := float64(engine.Width(text.Value, text.Size, text.Weight))
	for offset := len(text.Value) - 1; offset > 0; offset-- {
		if x := float64(engine.Caret(text.Value, text.Size, text.Weight, offset)); x < width {
			return x
		}
	}
	return 0
}
