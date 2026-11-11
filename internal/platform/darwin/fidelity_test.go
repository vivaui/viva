//go:build darwin && vivahead

package darwin

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textsharp"
)

// The reference: AppKit itself draws the same text, and the label rasterizer
// (Viva's label path into the Metal offscreen) is measured against it. The helper is
// testdata/appkit_text.m; it is compiled by the test and never shipped.

var (
	appkitOnce   sync.Once
	appkitBinary string
	appkitError  error
)

func appkitHelper(t testing.TB) string {
	appkitOnce.Do(func() {
		dir, err := os.MkdirTemp("", "viva-appkit-text")
		if err != nil {
			appkitError = err
			return
		}
		appkitBinary = filepath.Join(dir, "appkit_text")
		output, err := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Wno-deprecated-declarations",
			"testdata/appkit_text.m", "-o", appkitBinary, "-framework", "Cocoa").CombinedOutput()
		if err != nil {
			appkitError = fmt.Errorf("compile appkit_text: %v\n%s", err, output)
		}
	})
	if appkitError != nil {
		t.Fatal(appkitError)
	}
	return appkitBinary
}

// fidelityCase is one line of text drawn both ways.
type fidelityCase struct {
	Text           string
	Size, Weight   float64
	Scale          float64
	Foreground, BG float64 // sRGB grays
	Width, Height  int     // points: the canvas
	X, Y           float64 // where the label sits in it, points
	LabelHeight    int     // the label box; 0 for the canvas
	Alpha          float64 // opacity of the text color; 0 means 1
}

func (c fidelityCase) alpha() float64 {
	if c.Alpha == 0 {
		return 1
	}
	return c.Alpha
}

func (c fidelityCase) labelHeight() int {
	if c.LabelHeight > 0 {
		return c.LabelHeight
	}
	return c.Height
}

func (c fidelityCase) name() string {
	return fmt.Sprintf("%.12q %gpx w%g @%gx fg%.2f", c.Text, c.Size, c.Weight, c.Scale, c.Foreground)
}

// appKit draws the case with AppKit: mode "string" (NSAttributedString) or "field"
// (NSTextField). It returns the gray (green channel) of every device pixel.
func (c fidelityCase) appKit(t testing.TB, mode string, defaults ...string) (gray []float64, w, h int) {
	out := filepath.Join(t.TempDir(), "out.raw")
	pixelHeight := int(math.Ceil(float64(c.Height) * c.Scale))
	baseline := c.Y + float64(platform.LabelBaseline(int(math.Ceil(float64(c.labelHeight())*c.Scale)), c.Size*c.Scale))/c.Scale
	args := []string{out, c.Text, mode, fmt.Sprint(c.Size), fmt.Sprint(c.Weight), fmt.Sprint(c.Scale),
		fmt.Sprint(c.Foreground), fmt.Sprint(c.BG), strconv.Itoa(c.Width), strconv.Itoa(c.Height), fmt.Sprint(baseline), fmt.Sprint(c.X), fmt.Sprint(c.alpha())}
	args = append(args, defaults...)
	if output, err := exec.Command(appkitHelper(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("appkit_text: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	w, h = int(math.Ceil(float64(c.Width)*c.Scale)), pixelHeight
	return grayOf(raw, w, h), w, h
}

// viva draws the case with the label path of the Metal offscreen.
func (c fidelityCase) viva(t testing.TB) (gray []float64, w, h int) {
	offscreen, err := NewOffscreen(c.Width, c.Height, c.Scale, platform.Color{Red: c.BG, Green: c.BG, Blue: c.BG, Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	// As the painter hands a label over: its frame on whole logical pixels, the rest of its
	// position as the origin of the text.
	x, y := math.Floor(c.X+0.5), math.Floor(c.Y+0.5)
	label := TextLabel{Value: c.Text, X: float32(x), Y: float32(y), OriginX: float32(c.X - x), OriginY: float32(c.Y - y),
		Width: float32(c.Width) - float32(x), Height: float32(c.labelHeight()), FontSize: float32(c.Size), Weight: float32(c.Weight),
		Color: platform.Color{Red: c.Foreground, Green: c.Foreground, Blue: c.Foreground, Alpha: c.alpha()}}
	if err := offscreen.Set(nil, []TextLabel{label}, nil); err != nil {
		t.Fatal(err)
	}
	pixels, w, h, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	return grayOf(pixels, w, h), w, h
}

func grayOf(bgra []byte, w, h int) []float64 {
	gray := make([]float64, w*h)
	for i := range gray {
		gray[i] = (float64(bgra[i*4]) + float64(bgra[i*4+1]) + float64(bgra[i*4+2])) / (3 * 255)
	}
	return gray
}

// coverage turns gray pixels into the coverage of the foreground over the background.
func (c fidelityCase) coverage(gray []float64) []float64 {
	out := make([]float64, len(gray))
	for i, g := range gray {
		out[i] = (g - c.BG) / (c.alpha() * (c.Foreground - c.BG))
	}
	return out
}

// inkStats are the numbers the comparison reports.
type inkStats struct {
	Ink    float64 // summed coverage
	CX, CY float64 // ink centroid, device pixels
	Peak   float64 // 99th percentile of the coverage of inked pixels
	Bottom int     // last row holding at least a quarter of the ink of the heaviest row
	Top    int
	Sharp  textsharp.Sharpness
}

func stats(cov []float64, w, h int) inkStats {
	var s inkStats
	var sx, sy float64
	rows := make([]float64, h)
	var inked []float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := cov[y*w+x]
			if v < 0 {
				v = 0
			}
			s.Ink += v
			sx += v * float64(x)
			sy += v * float64(y)
			rows[y] += v
			if v > 0.05 {
				inked = append(inked, v)
			}
		}
	}
	if s.Ink > 0 {
		s.CX, s.CY = sx/s.Ink+0.5, sy/s.Ink+0.5
	}
	var heaviest float64
	for _, r := range rows {
		heaviest = math.Max(heaviest, r)
	}
	s.Top = -1
	for y, r := range rows {
		if r >= heaviest/4 {
			if s.Top < 0 {
				s.Top = y
			}
			s.Bottom = y
		}
	}
	if len(inked) > 0 {
		sortFloats(inked)
		s.Peak = inked[len(inked)*99/100]
	}
	bytes := make([]byte, w*h)
	for i, v := range cov {
		bytes[i] = byte(math.Max(0, math.Min(1, v))*255 + 0.5)
	}
	s.Sharp = textsharp.Measure(bytes, w, h, 1, w)
	return s
}

func sortFloats(v []float64) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func rms(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(a)))
}

// bestRowShift is the whole number of rows (within 2) by which moving gray up makes it closest to
// reference. NSAttributedString places a line by its own rounding of the font's descent, which
// is not the label baseline under test here (TestLabelBaselineIsWhereAppKitLabelsPutIt is), so
// the glyph comparison first lines the two up.
func bestRowShift(reference, gray []float64, w, h int) int {
	best, bestError := 0, math.Inf(1)
	for shift := -2; shift <= 2; shift++ {
		var sum float64
		for y := 2; y < h-2; y++ {
			for x := 0; x < w; x++ {
				d := reference[y*w+x] - gray[(y+shift)*w+x]
				sum += d * d
			}
		}
		if sum < bestError {
			best, bestError = shift, sum
		}
	}
	return best
}

func shiftedRows(gray []float64, w, h, shift int) []float64 {
	out := make([]float64, len(gray))
	for y := 0; y < h; y++ {
		if y+shift >= 0 && y+shift < h {
			copy(out[y*w:(y+1)*w], gray[(y+shift)*w:(y+shift+1)*w])
		}
	}
	return out
}

// A line of Latin digits and letters, and of CJK, which are placed by a different rule in the font's own
// cascade: a label is one CoreText line, as AppKit draws it, so every glyph lands where AppKit's does.
var fidelityWords = []string{"Hamburgefonstiv", "0123456789", "界面一致性测试", "ネットワーク接続"}

func (c fidelityCase) sized() fidelityCase {
	c.Width = int(c.Size*float64(len([]rune(c.Text)))*1.1) + 12
	c.Height = int(c.Size) + 10
	c.LabelHeight = c.Height
	return c
}

// A label is what AppKit draws: the same string, size, weight and color rasterize to the
// same coverage, light text on dark and dark text on light, at 1x and 2x. The comparison is a
// sRGB bitmap context AppKit draws an NSAttributedString into; it is the measurement that
// makes a change of the label rasterizer visible.
func TestLabelGlyphsAreWhatAppKitDraws(t *testing.T) {
	var worstRMS, worstInk, worstEdge float64
	worstInk, worstEdge = 1, 1
	var cases int
	for _, scale := range []float64{1, 2} {
		for _, fg := range []float64{0.1, 0.95} {
			bg := 1.0
			if fg > 0.5 {
				bg = 0.12
			}
			for _, size := range []float64{11, 13, 16, 24} {
				for _, weight := range []float64{0, 0.4} {
					for _, word := range fidelityWords {
						c := fidelityCase{Text: word, Size: size, Weight: weight, Scale: scale, Foreground: fg, BG: bg}.sized()
						reference, w, h := c.appKit(t, "string")
						got, _, _ := c.viva(t)
						wantCoverage, gotCoverage := c.coverage(reference), c.coverage(got)
						wantCoverage = shiftedRows(wantCoverage, w, h, bestRowShift(gotCoverage, wantCoverage, w, h))
						rs, gs := stats(wantCoverage, w, h), stats(gotCoverage, w, h)
						cases++
						difference := rms(wantCoverage, gotCoverage)
						worstRMS = math.Max(worstRMS, difference)
						worstInk = math.Min(worstInk, math.Min(gs.Ink/rs.Ink, rs.Ink/gs.Ink))
						worstEdge = math.Min(worstEdge, math.Min(gs.Sharp.Edge/rs.Sharp.Edge, rs.Sharp.Edge/gs.Sharp.Edge))
						if difference > 0.003 || gs.Ink/rs.Ink < 0.985 || gs.Ink/rs.Ink > 1.015 || gs.Sharp.Edge/rs.Sharp.Edge < 0.97 || gs.Sharp.Edge/rs.Sharp.Edge > 1.03 {
							t.Errorf("%s: label differs from AppKit: rms %.4f ink %.4f edge %.4f", c.name(), difference, gs.Ink/rs.Ink, gs.Sharp.Edge/rs.Sharp.Edge)
						}
					}
				}
			}
		}
	}
	t.Logf("%d cases against AppKit: worst rms of coverage %.4f, ink ratio %.4f, edge ratio %.4f", cases, worstRMS, worstInk, worstEdge)
}

// Font smoothing is what macOS calls thickening: with it on AppKit draws heavier text, with it
// off (`defaults write -g AppleFontSmoothing -int 0`) the font's own outlines, and it draws the
// same for every value but zero. A Core Graphics bitmap context ignores the setting, so a label
// decides its own weight: off is what AppKit draws with smoothing off, on with it on, whichever
// the user set, since the application decides.
func TestLabelThickening(t *testing.T) {
	c := fidelityCase{Text: "Hamburgefonstiv", Size: 13, Scale: 2, Foreground: 0.95, BG: 0.12}.sized()
	inkOf := func(gray []float64, w, h int) float64 { return stats(c.coverage(gray), w, h).Ink }

	reference, w, h := c.appKit(t, "string")
	smoothed := inkOf(reference, w, h)
	referenceOff, _, _ := c.appKit(t, "string", "-AppleFontSmoothing", "0")
	flat := inkOf(referenceOff, w, h)
	if flat > 0.85*smoothed {
		t.Fatalf("AppKit with AppleFontSmoothing 0 draws ink %.0f against %.0f: the setting should thin white text by about a quarter", flat, smoothed)
	}

	engine := sharedTextEngine()
	defer engine.SetFontThickening(true)
	// The user's own setting is smoothing on; the label still draws what it was told.
	OverrideDefaultInteger("AppleFontSmoothing", 1, false)
	defer OverrideDefaultInteger("AppleFontSmoothing", 0, true)
	engine.SetFontThickening(false)
	off, _, _ := c.viva(t)
	if difference := rms(c.coverage(referenceOff), c.coverage(off)); difference > 0.003 {
		t.Errorf("thickening off: label glyphs differ from AppKit's smoothing-off text: rms %.4f", difference)
	}
	engine.SetFontThickening(true)
	on, _, _ := c.viva(t)
	if ratio := inkOf(on, w, h) / smoothed; ratio < 0.995 || ratio > 1.005 {
		t.Errorf("thickening on: label ink is %.3f of AppKit's", ratio)
	}

	// A label picture is kept per generation, so the generation has to move for the new
	// weight to be drawn, and only then: asking for the weight it already has redraws
	// nothing.
	generation := engine.FontGeneration()
	engine.SetFontThickening(false)
	engine.SetFontThickening(false)
	if moved := engine.FontGeneration() - generation; moved != 1 {
		t.Errorf("the font generation moved %d times for one change", moved)
	}
}

// Text with an alpha over a surface is blended as AppKit blends it: the label texture is
// premultiplied and the pipeline blends one / one-minus-source-alpha, so the result is the
// coverage times the alpha of the color, not a lighter or heavier version of it.
func TestTranslucentLabelsBlendAsAppKitBlends(t *testing.T) {
	for _, scale := range []float64{1, 2} {
		for _, alpha := range []float64{0.6, 0.3} {
			for _, fg := range []float64{0.1, 0.95} {
				c := fidelityCase{Text: "Hamburgefonstiv", Size: 13, Scale: scale, Foreground: fg, BG: 0.5, Alpha: alpha}.sized()
				reference, w, h := c.appKit(t, "string")
				got, _, _ := c.viva(t)
				wantCoverage, gotCoverage := c.coverage(reference), c.coverage(got)
				wantCoverage = shiftedRows(wantCoverage, w, h, bestRowShift(gotCoverage, wantCoverage, w, h))
				if difference := rms(wantCoverage, gotCoverage); difference > 0.003 {
					t.Errorf("%gx alpha %.1f text %.2f on 0.5: rms %.4f", scale, alpha, fg, difference)
				}
			}
		}
	}
}

// The row the baseline lands on, against an NSTextField label of the same font at the height
// AppKit gives it (its intrinsic content size): the glyphs are AppKit's, and the baseline is
// the one every backend uses (platform.LabelBaseline, calibrated once on macOS). The two agree
// to a device pixel; the cases where they are a row apart are listed by the log (closing them
// takes a rule that follows the font's metrics, and would move the baseline of every platform).
func TestLabelBaselineIsWhereAppKitLabelsPutIt(t *testing.T) {
	var cases, exact int
	for _, scale := range []float64{1, 2} {
		for _, size := range []float64{11, 12, 13, 14, 15, 16, 18, 20, 24} {
			for _, weight := range []float64{0, 0.4} {
				out := filepath.Join(t.TempDir(), "o.raw")
				printed, err := exec.Command(appkitHelper(t), out, "HHHH", "field", fmt.Sprint(size), fmt.Sprint(weight), fmt.Sprint(scale), "0.1", "1", "80", "0").Output()
				if err != nil {
					t.Fatal(err)
				}
				var pw, ph int
				fmt.Sscan(string(printed), &pw, &ph)
				c := fidelityCase{Text: "HHHH", Size: size, Weight: weight, Scale: scale, Foreground: 0.1, BG: 1, Width: 80, Height: int(float64(ph) / scale)}
				reference, w, h := c.appKit(t, "field")
				got, _, _ := c.viva(t)
				rs, gs := stats(c.coverage(reference), w, h), stats(c.coverage(got), w, h)
				cases++
				switch delta := gs.Bottom - rs.Bottom; {
				case delta == 0:
					exact++
				case delta < -1 || delta > 1:
					t.Errorf("%gx %gpx w%g box %d: the baseline is %+d device pixels from the NSTextField's", scale, size, weight, c.Height, delta)
				default:
					t.Logf("%gx %gpx w%g box %d: baseline %+d device pixel from the NSTextField's", scale, size, weight, c.Height, delta)
				}
				if ratio := gs.Ink / rs.Ink; ratio < 0.99 || ratio > 1.01 {
					t.Errorf("%gx %gpx w%g: ink %.3f of the NSTextField's", scale, size, weight, ratio)
				}
			}
		}
	}
	t.Logf("%d of %d baselines on the NSTextField's row", exact, cases)
}

// Text that layout put at a fraction of a pixel is drawn there, as AppKit draws it: the glyph
// at Core Graphics' quantized x, the baseline on the device row below. Rounding the frame to
// whole logical pixels, as the painter does, would put it up to half a point (a whole device
// pixel at 2x) from where AppKit puts the same string.
func TestLabelTextIsPlacedAtItsSubpixelPosition(t *testing.T) {
	for _, scale := range []float64{1, 2} {
		for _, at := range [][2]float64{{0.25, 0}, {0.5, 0.5}, {0.7, 0.4}, {3.5, 2.5}, {10.3, 5.2}, {20.75, 7.6}, {6.9, 0.9}} {
			for _, fg := range []float64{0.1, 0.95} {
				bg := 1.0
				if fg > 0.5 {
					bg = 0.12
				}
				c := fidelityCase{Text: "Hamburgefonstiv", Size: 13, Scale: scale, Foreground: fg, BG: bg, Width: 140, Height: 40, X: at[0], Y: at[1], LabelHeight: 20}
				reference, w, h := c.appKit(t, "string")
				got, _, _ := c.viva(t)
				// A glyph whose position falls within float rounding of one of Core Graphics'
				// quantization steps lands on the next step in one drawing and not the other
				// (a fifth of a pixel for that glyph): the rms of those is 0.02, the centroid does
				// not move. Ignoring the origin moves the text by the whole rounding, up to 0.5.
				rs, gs := stats(c.coverage(reference), w, h), stats(c.coverage(got), w, h)
				difference := rms(c.coverage(reference), c.coverage(got))
				if difference > 0.035 || math.Abs(gs.CX-rs.CX) > 0.1 || math.Abs(gs.CY-rs.CY) > 0.05 {
					t.Errorf("%gx at %v fg%.2f: rms %.4f, centroid %+.3f, %+.3f px from AppKit's", scale, at, fg, difference, gs.CX-rs.CX, gs.CY-rs.CY)
				}
			}
		}
	}
}
