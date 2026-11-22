//go:build !android

package linux

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textsharp"
)

//go:embed testdata/gtkref.py
var gtkReferenceScript []byte

// gtkSpec is a case as the reference script takes it: in device pixels.
type gtkSpec struct {
	Name   string  `json:"name"`
	Text   string  `json:"text"`
	Size   float32 `json:"size"`
	Weight string  `json:"weight"`
	Fg     string  `json:"fg"`
	Bg     string  `json:"bg"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	X      int     `json:"x"`
}

func specOf(c textsharp.Case, x int) gtkSpec {
	width, height := c.DeviceSize()
	return gtkSpec{Name: c.Name, Text: c.Text, Size: c.Size * c.Scale, Weight: c.CSSWeight(), Fg: c.HexForeground(), Bg: c.HexBackground(), Width: width, Height: height, X: x}
}

// gtkReferences renders the lines and the words of the cases with real GtkLabels (python3
// gi) and returns the images by name and the settings GTK reported.
func gtkReferences(t *testing.T, cases []textsharp.Case, words []textsharp.WordRequest) (map[string]image.Image, string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3 for the GTK reference")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "gtkref.py")
	if err := os.WriteFile(script, gtkReferenceScript, 0o600); err != nil {
		t.Fatal(err)
	}
	var specs []gtkSpec
	for _, c := range cases {
		specs = append(specs, specOf(c, 0))
	}
	for _, w := range words {
		specs = append(specs, specOf(w.Case, textsharp.Margin))
	}
	data, _ := json.Marshal(specs)
	spec := filepath.Join(dir, "cases.json")
	if err := os.WriteFile(spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(python, "-I", script, spec, dir).CombinedOutput(); err != nil {
		t.Skipf("the GTK reference is not available (%v): %s", err, output)
	}
	settings, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	images := make(map[string]image.Image, len(specs))
	for _, s := range specs {
		file, err := os.Open(filepath.Join(dir, s.Name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		images[s.Name] = img
	}
	return images, string(settings)
}

func gray(value float64) platform.Color {
	return platform.Color{Red: value / 255, Green: value / 255, Blue: value / 255, Alpha: 1}
}

// vivaImager draws labels through the window's own pipeline (Vulkan on a software
// rasterizer under test), one renderer per image size.
type vivaImager struct {
	t          *testing.T
	offscreens map[string]*Offscreen
}

func newVivaImager(t *testing.T) *vivaImager {
	imager := &vivaImager{t: t, offscreens: map[string]*Offscreen{}}
	t.Cleanup(func() {
		for _, offscreen := range imager.offscreens {
			offscreen.Close()
		}
	})
	return imager
}

// image draws the case's text with its label at (x, 0) logical pixels and returns the
// opaque result.
func (imager *vivaImager) image(c textsharp.Case, x float32) image.Image {
	t := imager.t
	t.Helper()
	key := fmt.Sprintf("%d %d %g %g", c.Width, c.Height, c.Scale, c.Background())
	offscreen := imager.offscreens[key]
	if offscreen == nil {
		var err error
		offscreen, err = NewOffscreen(c.Width, c.Height, float64(c.Scale), gray(c.Background()))
		if err != nil {
			t.Skipf("no Vulkan device: %v", err)
		}
		imager.offscreens[key] = offscreen
	}
	label := platform.TextLabel{Value: c.Text, X: x, Width: float32(c.Width) - x, Height: float32(c.Height), FontSize: c.Size, Weight: c.Weight(), Color: gray(c.Foreground())}
	if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
		t.Fatal(err)
	}
	pixels, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	return pixels
}

// Labels next to real GtkLabels (set to unhinted metrics, as Viva draws them: testdata/gtkref.py).
// A word is drawn alone by both at the same origin, so that the glyphs are compared; the ink,
// the stems and the sharpness of whole lines are compared with the line GTK draws.
func TestLabelsAgainstAGtkLabel(t *testing.T) {
	compareLabels(t, []float32{1}, textsharp.FidelitySizes, 0.01, 0.05, 0.97, 0.95)
}

// The same on a high-resolution display: a GTK application on a 2x screen draws the same text
// at twice the pixel size.
func TestScaledLabelsAgainstAGtkLabel(t *testing.T) {
	// One CJK character at 19.5 and 24 pixels (4 of 928 words) differs by 0.13: GTK picks another
	// hinted outline for it.
	compareLabels(t, []float32{1.5, 2}, []float32{11, 13, 16, 20}, 0.02, 0.15, 0.88, 0.91)
}

func compareLabels(t *testing.T, scales, sizes []float32, meanResidual, worstResidual, leastWordEdge, leastLineEdge float64) {
	cases := textsharp.Cases(scales, sizes)
	words := textsharp.WordRequests(cases)
	references, settings := gtkReferences(t, cases, words)
	t.Logf("GTK settings: %s", settings)
	imager := newVivaImager(t)
	var line, word textsharp.Summary
	for _, c := range cases {
		got := textsharp.Coverage(imager.image(c, 0), c.Foreground(), c.Background())
		ref := textsharp.Coverage(references[c.Name], c.Foreground(), c.Background())
		whole := textsharp.Compare(got, ref, 0, 0)
		line.Add(whole)
		_, gotBottom := got.Vertical()
		_, refBottom := ref.Vertical()
		t.Logf("%-30s line mass %.3f stem %.3f edge %.3f solid %.3f baseline %+.1f", c.Name, whole.Mass, whole.Stem, whole.Edge, whole.Solid, gotBottom-refBottom)
	}
	for _, request := range words {
		c := request.Case
		got := textsharp.Coverage(imager.image(c, float32(textsharp.Margin)/c.Scale), c.Foreground(), c.Background())
		ref := textsharp.Coverage(references[request.Name], c.Foreground(), c.Background())
		f := textsharp.Compare(got, ref, 1, 12)
		word.Add(f)
		if f.Residual > 0.1 {
			t.Logf("  word %q %gpx@%gx bold=%t dark=%t: mass %.3f stem %.3f dx %d dy %d residual %.3f", c.Text, c.Size, c.Scale, c.Bold, c.Dark, f.Mass, f.Stem, f.DX, f.DY, f.Residual)
		}
	}
	// Tolerances are from those measurements, under the session's settings and
	// under every combination of Xft antialiasing, hinting and subpixel order tried: the words
	// are the same pixels as GTK's (residual 0.000, at most 0.014).
	if word.Residual/float64(word.N) > meanResidual || word.WorstResidual > worstResidual {
		t.Errorf("words differ from GTK's: %s", word)
	}
	if mass := word.Mass / float64(word.N); mass < 0.99 || mass > 1.01 || word.LeastEdge < leastWordEdge {
		t.Errorf("words are lighter, heavier or blurrier than GTK's: %s", word)
	}
	if mass := line.Mass / float64(line.N); mass < 0.97 || mass > 1.02 || line.LeastMass < 0.9 || line.LeastEdge < leastLineEdge {
		t.Errorf("lines carry less or more ink than GTK's: %s", line)
	}
	t.Logf("lines: %s", line)
	t.Logf("words: %s", word)
}
