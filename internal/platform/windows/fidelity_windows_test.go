package windows

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textsharp"
)

//go:embed testdata/gdiref.ps1
var gdiReferenceScript []byte

func gray(value float64) platform.Color {
	return platform.Color{Red: value / 255, Green: value / 255, Blue: value / 255, Alpha: 1}
}

// nativeImage draws text with DirectWrite through Direct2D as an application does, in one
// call with the real colors on an opaque bitmap, its first glyph at the whole pixel x and
// its baseline on the row a label puts it: what Explorer or Edge would show.
func nativeImage(t *testing.T, rasterizer *directRasterizer, c textsharp.Case, text string, x int) image.Image {
	t.Helper()
	back, front := float32(c.Background()/255), float32(c.Foreground()/255)
	return nativeColorImage(t, rasterizer, c, text, x, d2dColor{R: back, G: back, B: back, A: 1}, d2dColor{R: front, G: front, B: front, A: 1})
}

// nativeColorImage is nativeImage in any colors.
func nativeColorImage(t *testing.T, rasterizer *directRasterizer, c textsharp.Case, text string, x int, back, front d2dColor) image.Image {
	t.Helper()
	width, height := c.DeviceSize()
	size := c.Size * c.Scale
	format, err := rasterizer.services.formatFamily("Segoe UI", size, c.Weight())
	if err != nil {
		t.Fatal(err)
	}
	defer release(&format)
	units, _ := syscall.UTF16FromString(text)
	var layout uintptr
	if err := hresult(rasterizer.services.createLayout(rasterizer.services.write, &units[0], uint32(len(units)-1), format, 1<<20, 1<<20, &layout)); err != nil {
		t.Fatal(err)
	}
	defer release(&layout)
	var line lineMetrics
	var lines uint32
	comCall3(layout, writeGetLineMetrics, uintptr(unsafe.Pointer(&line)), 1, uintptr(unsafe.Pointer(&lines)))
	scratch, err := rasterizer.opaque.bind(width, height)
	if err != nil {
		t.Fatal(err)
	}
	fill(scratch, width, height, back)
	rasterizer.opaque.begin(front, d2dAntialiasDefault)
	rasterizer.opaque.draw(float32(x), float32(platform.LabelBaseline(height, float64(size)))-line.Baseline, layout, 0)
	if err := rasterizer.opaque.end(); err != nil {
		t.Fatal(err)
	}
	pixels := make([]byte, width*height*4)
	for row := 0; row < height; row++ {
		copy(pixels[row*width*4:(row+1)*width*4], scratch.Pixels[row*scratch.Width*4:])
	}
	return bgraImage(pixels, width, height)
}

func bgraImage(pixels []byte, width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = pixels[i*4+2], pixels[i*4+1], pixels[i*4], 255
	}
	return img
}

type gdiSpec struct {
	Name   string  `json:"name"`
	Text   string  `json:"text"`
	Size   float32 `json:"size"`
	Weight string  `json:"weight"`
	Fg     string  `json:"fg"`
	Bg     string  `json:"bg"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
}

// gdiReferences renders lines and words with WinForms' TextRenderer (GDI, as a Win32
// application draws its labels); the origin is the top left of the image.
func gdiReferences(t *testing.T, cases []textsharp.Case, words []textsharp.WordRequest) map[string]image.Image {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "gdiref.ps1")
	if err := os.WriteFile(script, gdiReferenceScript, 0o600); err != nil {
		t.Fatal(err)
	}
	var specs []gdiSpec
	add := func(c textsharp.Case) {
		width, height := c.DeviceSize()
		specs = append(specs, gdiSpec{Name: c.Name, Text: c.Text, Size: c.Size * c.Scale, Weight: c.CSSWeight(), Fg: c.HexForeground(), Bg: c.HexBackground(), Width: width, Height: height})
	}
	for _, c := range cases {
		add(c)
	}
	for _, w := range words {
		add(w.Case)
	}
	data, _ := json.Marshal(specs)
	spec := filepath.Join(dir, "cases.json")
	if err := os.WriteFile(spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, spec, dir).CombinedOutput(); err != nil {
		t.Skipf("the GDI reference is not available (%v): %s", err, output)
	}
	images := map[string]image.Image{}
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
	return images
}

// vivaImager draws labels through the window's own Direct3D 11 pipeline (on WARP in the
// test machine), one renderer per image size.
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

func (imager *vivaImager) image(c textsharp.Case, x float32) image.Image {
	t := imager.t
	t.Helper()
	key := fmt.Sprintf("%d %d %g %g", c.Width, c.Height, c.Scale, c.Background())
	offscreen := imager.offscreens[key]
	if offscreen == nil {
		var err error
		offscreen, err = NewOffscreen(c.Width, c.Height, float64(c.Scale), gray(c.Background()))
		if errors.Is(err, platform.ErrNoGraphics) {
			t.Skipf("no Direct3D device: %v", err)
		}
		if err != nil {
			t.Fatal(err)
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
	// The image is premultiplied over an opaque clear color: opaque.
	return pixels
}

func plane(img image.Image, c textsharp.Case) textsharp.Plane {
	return textsharp.Coverage(img, c.Foreground(), c.Background())
}

// chroma is the share of the ink of img that is colored (a ClearType fringe).
func chromaOf(p textsharp.Plane) float64 { return p.Stats().Fringe }

var _ = color.RGBA{}
var _ = math.Pi

// Labels next to what Windows draws. A word is drawn alone by both at the same origin,
// so that the glyphs are compared and not where layout puts them; the ink, stems and
// sharpness of whole lines are compared with the line drawn in one call.
func TestLabelsAgainstDirectWriteAndGDI(t *testing.T) {
	compareLabels(t, []float32{1}, textsharp.FidelitySizes)
}

// The same at the scales of a high-resolution display, where the font is not an integer
// number of pixels at 1.5x.
func TestScaledLabelsAgainstDirectWriteAndGDI(t *testing.T) {
	compareLabels(t, []float32{1.5, 2}, []float32{11, 13, 16, 20})
}

func compareLabels(t *testing.T, scales, sizes []float32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cases := textsharp.Cases(scales, sizes)
	words := textsharp.WordRequests(cases)
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer rasterizer.close()
	imager := newVivaImager(t)
	gdi := gdiReferences(t, cases, words)
	var lineD2D, wordD2D, lineGDI, wordGDI textsharp.Summary
	for _, c := range cases {
		got := plane(imager.image(c, 0), c)
		native := plane(nativeImage(t, rasterizer, c, c.Text, 0), c)
		ref := plane(gdi[c.Name], c)
		d2d := textsharp.Compare(got, native, 0, 0)
		gdiF := textsharp.Compare(got, ref, 0, 0)
		lineD2D.Add(d2d)
		lineGDI.Add(gdiF)
		_, gotBottom := got.Vertical()
		_, nativeBottom := native.Vertical()
		if math.Abs(gotBottom-nativeBottom) > 0.25 {
			t.Errorf("%s: the baseline is at row %.2f, Direct2D's is at %.2f", c.Name, gotBottom, nativeBottom)
		}
		_, gdiBottom := ref.Vertical()
		t.Logf("%-30s D2D line mass %.3f stem %.3f edge %.3f solid %.3f baseline %+.1f fringe %.2f/%.2f | GDI mass %.3f stem %.3f edge %.3f baseline %+.1f fringe %.2f",
			c.Name, d2d.Mass, d2d.Stem, d2d.Edge, d2d.Solid, gotBottom-nativeBottom, d2d.Fringe, d2d.RefFringe, gdiF.Mass, gdiF.Stem, gdiF.Edge, gotBottom-gdiBottom, gdiF.RefFringe)
	}
	for _, request := range words {
		c := request.Case
		got := plane(imager.image(c, float32(textsharp.Margin)/c.Scale), c)
		native := plane(nativeImage(t, rasterizer, c, c.Text, textsharp.Margin), c)
		d2d := textsharp.Compare(got, native, 1, 2)
		wordD2D.Add(d2d)
		gdiF := textsharp.Compare(got, plane(gdi[request.Name], c), 1, 12)
		wordGDI.Add(gdiF)
		if d2d.Residual > 0.1 {
			t.Logf("  word %q %gpx bold=%t dark=%t: D2D mass %.3f stem %.3f dx %d dy %d residual %.3f", c.Text, c.Size, c.Bold, c.Dark, d2d.Mass, d2d.Stem, d2d.DX, d2d.DY, d2d.Residual)
		}
	}
	// Tolerances are from those measurements (scales 1, 1.5 and 2, sizes 11 to
	// 24): words are within 0.02 of Direct2D's own pixels on average and 0.11 at the worst, ink
	// within 0.5 percent, stems within 1.5. A line is narrower where a word was squeezed.
	if wordD2D.Residual/float64(wordD2D.N) > 0.04 || wordD2D.WorstResidual > 0.2 {
		t.Errorf("words differ from Direct2D's: %s", wordD2D)
	}
	if mass, stem := wordD2D.Mass/float64(wordD2D.N), wordD2D.Stem/float64(wordD2D.N); mass < 0.98 || mass > 1.02 || stem < 0.97 || stem > 1.05 || wordD2D.LeastEdge < 0.85 {
		t.Errorf("words are lighter, heavier or blurrier than Direct2D's: %s", wordD2D)
	}
	if mass := lineD2D.Mass / float64(lineD2D.N); mass < 0.97 || mass > 1.02 || lineD2D.LeastMass < 0.9 {
		t.Errorf("lines carry less or more ink than Direct2D's: %s", lineD2D)
	}
	t.Logf("lines vs Direct2D: %s", lineD2D)
	t.Logf("words vs Direct2D: %s", wordD2D)
	t.Logf("lines vs GDI:      %s", lineGDI)
	t.Logf("words vs GDI:      %s", wordGDI)
}

// The text of any color blends as Direct2D blends it: the coverage is read for the brightness of
// the text and the color is applied per channel, which differs from Direct2D's own
// blend of that color by the contrast it enhances for each channel.
func TestColoredTextMatchesDirect2DBlendingThatColor(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer rasterizer.close()
	for _, test := range []struct {
		name        string
		front, back [3]float32
	}{
		{"blue on white", [3]float32{0.15, 0.35, 0.85}, [3]float32{1, 1, 1}},
		{"orange on dark", [3]float32{0.95, 0.6, 0.2}, [3]float32{0.12, 0.12, 0.14}},
		{"green on gray", [3]float32{0.1, 0.5, 0.2}, [3]float32{0.8, 0.8, 0.82}},
		{"red on light", [3]float32{0.8, 0.1, 0.1}, [3]float32{0.97, 0.95, 0.9}},
	} {
		for _, size := range []float32{13, 16} {
			c := textsharp.Case{Text: "Connect", Size: size, Scale: 1, Width: 160, Height: 28}
			native := nativeColorImage(t, rasterizer, c, c.Text, 0, d2dColor{R: test.back[0], G: test.back[1], B: test.back[2], A: 1}, d2dColor{R: test.front[0], G: test.front[1], B: test.front[2], A: 1})
			offscreen, err := NewOffscreen(160, 28, 1, platform.Color{Red: float64(test.back[0]), Green: float64(test.back[1]), Blue: float64(test.back[2]), Alpha: 1})
			if err != nil {
				t.Skip(err)
			}
			label := platform.TextLabel{Value: c.Text, Width: 160, Height: 28, FontSize: size, Color: platform.Color{Red: float64(test.front[0]), Green: float64(test.front[1]), Blue: float64(test.front[2]), Alpha: 1}}
			if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
				t.Fatal(err)
			}
			got, err := offscreen.Image()
			offscreen.Close()
			if err != nil {
				t.Fatal(err)
			}
			var total float64
			var count int
			for y := 0; y < 28; y++ {
				for x := 0; x < 160; x++ {
					nr, ng, nb, _ := native.At(x, y).RGBA()
					gr, gg, gb, _ := got.At(x, y).RGBA()
					br, bg, bb := int(test.back[0]*255+0.5), int(test.back[1]*255+0.5), int(test.back[2]*255+0.5)
					if abs(int(nr>>8)-br)+abs(int(ng>>8)-bg)+abs(int(nb>>8)-bb) < 24 && abs(int(gr>>8)-br)+abs(int(gg>>8)-bg)+abs(int(gb>>8)-bb) < 24 {
						continue
					}
					total += float64(abs(int(nr>>8)-int(gr>>8))+abs(int(ng>>8)-int(gg>>8))+abs(int(nb>>8)-int(gb>>8))) / 3
					count++
				}
			}
			t.Logf("%-14s %gpx: mean channel error over %d ink pixels %.2f / 255", test.name, size, count, total/float64(max(count, 1)))
			if total/float64(max(count, 1)) > 8 {
				t.Errorf("%s at %gpx: the color of the text is blended %.1f / 255 away from Direct2D's", test.name, size, total/float64(count))
			}
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
