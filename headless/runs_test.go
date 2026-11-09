//go:build darwin || windows || linux

package headless

import (
	"image"
	"os"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// gridApp paints one fixed-pitch line "ab世cd" whose runs are red, green, blue
// and white, so a screenshot shows which cell each one landed in.
type gridApp struct {
	cellWidth, cellHeight float32
	origin                gfx.Rect
	runs                  []gfx.TextRun
}

func (g *gridApp) config() app.Config {
	red, green := gfx.Color{Red: 1, Alpha: 1}, gfx.Color{Green: 1, Alpha: 1}
	blue, white := gfx.Color{Blue: 1, Alpha: 1}, gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	return app.Config{
		Title: "grid", Width: 320, Height: 100,
		ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			g.cellWidth, g.cellHeight = canvas.CellSize(20)
			g.origin = gfx.Rect{X: 10, Y: 10, Width: g.cellWidth * 10, Height: g.cellHeight}
			g.runs = []gfx.TextRun{
				{Column: 0, Bytes: 1, Cells: 1, Color: red},
				{Column: 1, Bytes: 1, Cells: 1, Color: green},
				{Column: 2, Bytes: 3, Cells: 2, Color: blue}, // a wide character
				{Column: 4, Bytes: 2, Cells: 2, Color: white, Underline: true},
			}
			canvas.LabelRuns("ab世cd", g.origin, gfx.TextStyle{Size: 20, Color: white}, g.cellWidth, g.runs)
		},
	}
}

func TestFixedPitchRunsLandInTheirCellsWithTheirColors(t *testing.T) {
	grid := &gridApp{}
	session, err := Open(grid.config(), nil, Options{Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	picture, err := session.Image()
	if err != nil {
		t.Skipf("no renderer: %v", err)
	}

	if path := os.Getenv("VIVA_TEST_SHOT"); path != "" {
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
	}

	// Which cell column holds how much of each primary?
	column := func(cell float32) image.Rectangle {
		left := int((grid.origin.X + cell*grid.cellWidth) * 2)
		right := int((grid.origin.X + (cell+1)*grid.cellWidth) * 2)
		return image.Rect(left, int(grid.origin.Y*2), right, int((grid.origin.Y+grid.cellHeight)*2))
	}
	dominant := func(area image.Rectangle) (red, green, blue int) {
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				r, g, b, _ := picture.At(x, y).RGBA()
				red, green, blue = red+int(r>>8), green+int(g>>8), blue+int(b>>8)
			}
		}
		return red, green, blue
	}
	r, g, b := dominant(column(0))
	if r < 200 || g > r/4 || b > r/4 {
		t.Errorf("cell 0 should be red ink: r=%d g=%d b=%d", r, g, b)
	}
	r, g, b = dominant(column(1))
	if g < 200 || r > g/4 || b > g/4 {
		t.Errorf("cell 1 should be green ink: r=%d g=%d b=%d", r, g, b)
	}
	// The wide character spans cells 2 and 3 and is blue; nothing red or green leaks in.
	r, g, b = dominant(image.Rect(column(2).Min.X, column(2).Min.Y, column(3).Max.X, column(3).Max.Y))
	if b < 200 || r > b/4 || g > b/4 {
		t.Errorf("cells 2-3 should be blue ink: r=%d g=%d b=%d", r, g, b)
	}
	// White text with an underline at the end.
	r, g, b = dominant(image.Rect(column(4).Min.X, column(4).Min.Y, column(5).Max.X, column(5).Max.Y))
	if r < 200 || g < 200 || b < 200 {
		t.Errorf("cells 4-5 should be white ink: r=%d g=%d b=%d", r, g, b)
	}
	// Nothing is drawn past the last run.
	if r, g, b = dominant(column(8)); r+g+b != 0 {
		t.Errorf("cell 8 should be empty: r=%d g=%d b=%d", r, g, b)
	}
}

func TestFixedPitchCellMetricsAreWholeLinesAndPositive(t *testing.T) {
	grid := &gridApp{}
	session, err := Open(grid.config(), nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if grid.cellWidth <= 0 || grid.cellHeight <= 0 || grid.cellHeight != float32(int(grid.cellHeight)) {
		t.Fatalf("cell = %vx%v", grid.cellWidth, grid.cellHeight)
	}
	if hit, ok := session.Hit("nothing"); ok {
		t.Fatalf("unexpected hit %+v", hit)
	}
	found := false
	for _, text := range session.Texts() {
		if text == "ab世cd" {
			found = true
		}
	}
	if !found {
		t.Errorf("the grid line is missing from the screen texts: %q", session.Texts())
	}
}
