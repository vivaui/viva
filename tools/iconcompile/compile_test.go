package iconcompile

import (
	"embed"
	"go/format"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

//go:embed testdata/*.svg
var testIcons embed.FS

// TestLucideSuiteCompiles walks every testdata SVG (a snapshot of the
// Lucide icon set) and checks the compiled vocabulary draws real geometry.
func TestLucideSuiteCompiles(t *testing.T) {
	entries, err := testIcons.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	compiled := 0
	for _, entry := range entries {
		source, err := testIcons.ReadFile("testdata/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		icon, err := Compile(entry.Name(), source)
		if err != nil {
			t.Errorf("%s: %v", entry.Name(), err)
			continue
		}
		compiled++

		// Every icon must draw a sane number of triangles at icon size.
		var canvas ui.Canvas
		canvas.DrawIcon(icon, ui.Rect{X: 0, Y: 0, Width: 16, Height: 16}, ui.Color{Alpha: 1})
		if len(canvas.Triangles()) < 2 {
			t.Errorf("%s: drew %d triangles; geometry lost", entry.Name(), len(canvas.Triangles()))
		}
		// Geometry must stay inside a padded icon box.
		for _, triangle := range canvas.Triangles() {
			for _, point := range triangle.Points {
				if point.X < -1 || point.X > 17 || point.Y < -1 || point.Y > 17 {
					t.Errorf("%s: point %+v escapes the icon box", entry.Name(), point)
					break
				}
			}
		}
	}
	if compiled < 8 {
		t.Fatalf("compiled %d icons; testdata shrank", compiled)
	}
}

// TestKnownIconGeometry pins one icon's compiled vocabulary exactly: any
// parser regression changes these numbers.
func TestKnownIconGeometry(t *testing.T) {
	source, err := testIcons.ReadFile("testdata/download.svg")
	if err != nil {
		t.Fatal(err)
	}
	icon, err := Compile("download", source)
	if err != nil {
		t.Fatal(err)
	}
	if icon.ViewBox != 24 || icon.Stroke != 2 || icon.Cap != ui.CapRound || icon.Join != ui.JoinRound {
		t.Fatalf("metadata = %+v", icon)
	}
	if len(icon.Commands) != 11 {
		t.Fatalf("commands = %d, want 11", len(icon.Commands))
	}
	if icon.Commands[0] != (ui.PathCommand{Kind: ui.PathCommandMove, Point: ui.Point{X: 21, Y: 15}}) {
		t.Fatalf("first command = %+v", icon.Commands[0])
	}
	// The arc pair (a 2 2 0 0 1 ...) must have become cubic commands.
	kinds := map[ui.PathCommandKind]int{}
	for _, command := range icon.Commands {
		kinds[command.Kind]++
	}
	if kinds[ui.PathCommandCubic] == 0 {
		t.Fatal("arc commands did not compile to cubics")
	}
}

// TestRelativeMoveContinuation verifies m19 12-7 7-7-7 style data: one
// move followed by implicit relative lineto pairs.
func TestRelativeMoveContinuation(t *testing.T) {
	source, err := testIcons.ReadFile("testdata/arrow-down.svg")
	if err != nil {
		t.Fatal(err)
	}
	icon, err := Compile("arrow-down", source)
	if err != nil {
		t.Fatal(err)
	}
	var last ui.PathCommand
	moves, lines := 0, 0
	for _, command := range icon.Commands {
		switch command.Kind {
		case ui.PathCommandMove:
			moves++
		case ui.PathCommandLine:
			lines++
		}
		last = command
	}
	if moves != 2 || lines != 3 {
		t.Fatalf("moves=%d lines=%d commands=%+v", moves, lines, icon.Commands)
	}
	if last.Point != (ui.Point{X: 5, Y: 12}) {
		t.Fatalf("final point = %+v, want (5,12)", last.Point)
	}
}

// TestGeneratedSourceCompiles runs the full CLI path over testdata and
// checks the emitted Go parses back into equivalent icons.
func TestGeneratedSourceCompiles(t *testing.T) {
	entries, err := testIcons.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	icons := map[string]ui.Icon{}
	for _, entry := range entries {
		source, err := testIcons.ReadFile("testdata/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		icon, err := Compile(entry.Name(), source)
		if err != nil {
			t.Fatal(err)
		}
		icons[nameWithoutExt(entry.Name())] = icon
	}
	generated, err := Generate("icons_test", icons, "test license line")
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) == 0 {
		t.Fatal("empty generation")
	}
	// Deterministic output: generating twice yields identical bytes.
	again, err := Generate("icons_test", icons, "test license line")
	if err != nil {
		t.Fatal(err)
	}
	if string(generated) != string(again) {
		t.Fatal("generation is not deterministic")
	}
}

func nameWithoutExt(name string) string {
	if extension := filepath.Ext(name); extension != "" {
		return name[:len(name)-len(extension)]
	}
	return name
}

// TestCircleIconDrawsDisc checks element coverage beyond <path>: circle
// sources must draw a filled-looking ring of stroke triangles.
func TestCircleIconDrawsDisc(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/></svg>`
	icon, err := Compile("circle", []byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	var canvas ui.Canvas
	canvas.DrawIcon(icon, ui.Rect{Width: 24, Height: 24}, ui.Color{Alpha: 1})
	triangles := canvas.Triangles()
	if len(triangles) < 8 {
		t.Fatalf("triangles = %d; circle did not tessellate", len(triangles))
	}
	// Sample coverage: ink on the ring (r≈10±1), no ink at the center or
	// outside the stroke. Overlapping opaque decomposition may exceed the
	// exact ring area, but its extent must match the ring.
	sample := func(x, y float32) bool { return inked(triangles, x, y) }
	if !sample(22, 12) || !sample(12, 22) || !sample(2, 12) {
		t.Fatal("ring has no ink at cardinal points")
	}
	if sample(12, 12) {
		t.Fatal("circle center is covered; stroke leaked into the disc")
	}
	if sample(1, 1) {
		t.Fatal("ink outside the circle")
	}
}

// inked reports whether any triangle covers the point.
func inked(triangles []ui.Triangle, x, y float32) bool {
	for _, triangle := range triangles {
		a, b, c := triangle.Points[0], triangle.Points[1], triangle.Points[2]
		d1 := (b.X-a.X)*(y-a.Y) - (b.Y-a.Y)*(x-a.X)
		d2 := (c.X-b.X)*(y-b.Y) - (c.Y-b.Y)*(x-b.X)
		d3 := (a.X-c.X)*(y-c.Y) - (a.Y-c.Y)*(x-c.X)
		hasNegative := d1 < -1e-4 || d2 < -1e-4 || d3 < -1e-4
		hasPositive := d1 > 1e-4 || d2 > 1e-4 || d3 > 1e-4
		if !(hasNegative && hasPositive) {
			return true
		}
	}
	return false
}

// TestRoundedRectIconTracesTheRoundedOutline checks the geometry, not just
// that something was drawn: the stroke must follow the rounded outline, so the
// middle of every corner arc is inked while the sharp corner beyond it, the
// inside, and the outside are not.
func TestRoundedRectIconTracesTheRoundedOutline(t *testing.T) {
	// A 3..21 square with radius 4 and a stroke of 2 drawn at 24 px.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="4"/></svg>`
	icon, err := Compile("rounded", []byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	var canvas ui.Canvas
	canvas.DrawIcon(icon, ui.Rect{Width: 24, Height: 24}, ui.Color{Alpha: 1})
	triangles := canvas.Triangles()

	const radius, edge = 4.0, 3.0
	arc := func(centerX, centerY, dx, dy float64) (float32, float32) {
		const quarter = 0.7071067811865476 // the arc's midpoint is 45 degrees round
		return float32(centerX + dx*radius*quarter), float32(centerY + dy*radius*quarter)
	}
	corners := []struct {
		name           string
		cx, cy         float64 // arc center
		dx, dy         float64 // direction toward the corner
		sharpX, sharpY float32
	}{
		{"top-right", 21 - radius, edge + radius, 1, -1, 21, 3},
		{"bottom-right", 21 - radius, 21 - radius, 1, 1, 21, 21},
		{"bottom-left", edge + radius, 21 - radius, -1, 1, 3, 21},
		{"top-left", edge + radius, edge + radius, -1, -1, 3, 3},
	}
	for _, corner := range corners {
		x, y := arc(corner.cx, corner.cy, corner.dx, corner.dy)
		if !inked(triangles, x, y) {
			t.Errorf("%s: no ink on the corner arc at (%.2f, %.2f)", corner.name, x, y)
		}
		if inked(triangles, corner.sharpX, corner.sharpY) {
			t.Errorf("%s: ink at the sharp corner (%v, %v); the corner is not rounded", corner.name, corner.sharpX, corner.sharpY)
		}
	}
	// The straight edges are inked at their midpoints...
	for _, point := range [][2]float32{{12, 3}, {21, 12}, {12, 21}, {3, 12}} {
		if !inked(triangles, point[0], point[1]) {
			t.Errorf("no ink on the edge at %v", point)
		}
	}
	// ...and nothing leaks inside or outside the outline.
	for _, point := range [][2]float32{{12, 12}, {8, 8}, {16, 16}, {1, 12}, {23, 12}, {12, 1}, {12, 23}, {0.5, 0.5}, {23.5, 23.5}} {
		if inked(triangles, point[0], point[1]) {
			t.Errorf("unexpected ink at %v", point)
		}
	}
}

// TestRectWithoutRadiusIsASharpSquare pins the plain rect path too.
func TestRectWithoutRadiusIsASharpSquare(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="6" width="16" height="12"/></svg>`
	icon, err := Compile("square", []byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	var canvas ui.Canvas
	canvas.DrawIcon(icon, ui.Rect{Width: 24, Height: 24}, ui.Color{Alpha: 1})
	for _, point := range [][2]float32{{4, 6}, {20, 6}, {20, 18}, {4, 18}, {12, 6}, {12, 18}, {4, 12}, {20, 12}} {
		if !inked(canvas.Triangles(), point[0], point[1]) {
			t.Errorf("no ink at %v", point)
		}
	}
	if inked(canvas.Triangles(), 12, 12) || inked(canvas.Triangles(), 2, 2) {
		t.Error("ink inside or outside the square")
	}
}

// TestRectCommandsAreExact pins the compiled outline of a rounded and an
// elliptical rect, so a regression in any corner's center or tangent shows as
// a changed number rather than a subtle skew.
func TestRectCommandsAreExact(t *testing.T) {
	const kappa = 0.5522847498307936
	near := func(a, b float32) bool { d := a - b; return d > -1e-4 && d < 1e-4 }
	for _, test := range []struct {
		name   string
		attrs  string
		rx, ry float64
	}{
		{"circular corners", `x="3" y="3" width="18" height="18" rx="2"`, 2, 2},
		{"elliptical corners", `x="2" y="4" width="20" height="14" rx="6" ry="3"`, 6, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect ` + test.attrs + `/></svg>`
			icon, err := Compile("rect", []byte(svg))
			if err != nil {
				t.Fatal(err)
			}
			var x, y, w, h float64
			switch test.name {
			case "circular corners":
				x, y, w, h = 3, 3, 18, 18
			default:
				x, y, w, h = 2, 4, 20, 14
			}
			rx, ry := test.rx, test.ry
			k := func(v float64) float32 { return float32(v) }
			want := []ui.PathCommand{
				ui.MoveTo(k(x+w-rx), k(y)),
				ui.CubicTo(k(x+w-rx+kappa*rx), k(y), k(x+w), k(y+ry-kappa*ry), k(x+w), k(y+ry)),
				ui.LineTo(k(x+w), k(y+h-ry)),
				ui.CubicTo(k(x+w), k(y+h-ry+kappa*ry), k(x+w-rx+kappa*rx), k(y+h), k(x+w-rx), k(y+h)),
				ui.LineTo(k(x+rx), k(y+h)),
				ui.CubicTo(k(x+rx-kappa*rx), k(y+h), k(x), k(y+h-ry+kappa*ry), k(x), k(y+h-ry)),
				ui.LineTo(k(x), k(y+ry)),
				ui.CubicTo(k(x), k(y+ry-kappa*ry), k(x+rx-kappa*rx), k(y), k(x+rx), k(y)),
				ui.ClosePath(),
			}
			if len(icon.Commands) != len(want) {
				t.Fatalf("%d commands, want %d: %+v", len(icon.Commands), len(want), icon.Commands)
			}
			for index, command := range icon.Commands {
				expected := want[index]
				if command.Kind != expected.Kind || !near(command.Point.X, expected.Point.X) || !near(command.Point.Y, expected.Point.Y) ||
					!near(command.Control1.X, expected.Control1.X) || !near(command.Control1.Y, expected.Control1.Y) ||
					!near(command.Control2.X, expected.Control2.X) || !near(command.Control2.Y, expected.Control2.Y) {
					t.Fatalf("command %d = %+v, want %+v", index, command, expected)
				}
			}
		})
	}
}

// Regenerating must not churn diffs: the output is already gofmt-clean.
func TestGeneratedSourceIsGofmtClean(t *testing.T) {
	icon, err := Compile("tick", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6 9 17l-5-5"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	source, err := Generate("icons", map[string]ui.Icon{"tick": icon}, "Lucide, ISC license.")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != string(source) {
		t.Fatal("generated source is not gofmt-clean")
	}
	if !strings.Contains(string(source), "// Lucide, ISC license.") {
		t.Fatal("the license text was not carried into the header")
	}
}

// A shape that names a fill color becomes a colored layer, in document order,
// with its own opacity and fill rule; a plain stroked shape stays an ordinary
// command of the icon.
func TestColoredFillsBecomeLayers(t *testing.T) {
	icon, err := Compile("html", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
<path fill="#e65100" d="M4 4h24v24H4z"/>
<g fill="#00f" fill-opacity=".5" opacity=".5"><path fill-rule="evenodd" d="M8 8h8v8H8z"/><rect x="18" y="18" width="4" height="4"/></g>
</svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(icon.Commands) != 0 || len(icon.Layers) != 3 {
		t.Fatalf("commands=%d layers=%d, want 0 and 3", len(icon.Commands), len(icon.Layers))
	}
	orange := icon.Layers[0].Fill
	if orange.Red < 0.89 || orange.Red > 0.91 || orange.Green < 0.31 || orange.Green > 0.33 || orange.Blue != 0 || orange.Alpha != 1 {
		t.Fatalf("first fill = %+v, want #e65100", orange)
	}
	if blue := icon.Layers[1].Fill; blue.Blue != 1 || blue.Alpha != 0.25 || icon.Layers[1].FillRule != ui.PathFillEvenOdd {
		t.Fatalf("second layer = %+v, want opaque-blue at a quarter alpha, even-odd", icon.Layers[1])
	}
	if icon.Layers[2].FillRule != ui.PathFillNonZero || len(icon.Layers[2].Commands) == 0 {
		t.Fatalf("rect layer = %+v", icon.Layers[2])
	}
}

func TestFillsThatCannotBeColoredAreRejectedNotGuessed(t *testing.T) {
	for _, source := range []string{
		`<svg viewBox="0 0 24 24"><path fill="url(#g)" d="M0 0h8v8z"/></svg>`,
		`<svg viewBox="0 0 24 24"><path fill="rebeccapurple" d="M0 0h8v8z"/></svg>`,
		`<svg viewBox="0 0 24 24"><path fill="#12" d="M0 0h8v8z"/></svg>`,
	} {
		if _, err := Compile("bad", []byte(source)); err == nil {
			t.Errorf("%s compiled", source)
		}
	}
}

func TestColoredLayersSurviveGenerationAndCompile(t *testing.T) {
	icon, err := Compile("dot", []byte(`<svg viewBox="0 0 16 16"><path fill="#8d6e63" d="M2 2h12v12H2z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	source, err := Generate("icons", map[string]ui.Icon{"dot": icon}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Layers: []ui.IconLayer{", "ui.Color{Red: 0.5529", "ui.MoveTo(2.0, 2.0)"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("generated source lacks %q:\n%s", want, source)
		}
	}
	if strings.Contains(string(source), "Commands: []ui.PathCommand{\n\t\tui.MoveTo(2.0, 2.0)") {
		t.Error("a colored icon also emitted stroked commands")
	}
}

// A transparent bounding box (fill none, no stroke) draws nothing, while the
// Lucide convention of fill none with an inherited stroke still strokes.
func TestUnpaintedShapesAreSkipped(t *testing.T) {
	icon, err := Compile("document", []byte(`<svg viewBox="0 0 24 24"><g fill="none"><path d="M0 0h24v24H0z"/><path fill="#42a5f5" d="M8 16h8v2H8z"/></g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(icon.Commands) != 0 || len(icon.Layers) != 1 {
		t.Fatalf("commands=%d layers=%d, want the bounding box dropped and one layer", len(icon.Commands), len(icon.Layers))
	}
	stroked, err := Compile("line", []byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M4 12h16"/></svg>`))
	if err != nil || len(stroked.Commands) == 0 || len(stroked.Layers) != 0 {
		t.Fatalf("a stroked Lucide shape = %+v, %v", stroked, err)
	}
}

// A view box that starts away from 0,0 is shifted to the origin, so the glyph
// lands inside the square the icon is drawn into.
func TestViewBoxOriginIsHonored(t *testing.T) {
	icon, err := Compile("brace", []byte(`<svg viewBox="0 -960 960 960"><path fill="#f9a825" d="M560-160h80v-80z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	first := icon.Layers[0].Commands[0]
	if icon.ViewBox != 960 || first.Point.X != 560 || first.Point.Y != 800 {
		t.Fatalf("view box %v, first point %+v; want 960 and (560, 800)", icon.ViewBox, first.Point)
	}
}

// A root stroke-width of 0 makes the tinted shapes of the icon filled, which is how
// a glyph drawn as outlines takes the theme's text color.
func TestZeroStrokeWidthMakesTintedShapesFilled(t *testing.T) {
	icon, err := Compile("glyph", []byte(`<svg viewBox="0 0 24 24" stroke-width="0"><path fill="currentColor" d="M2 2h20v20z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if icon.Stroke != 0 || len(icon.Commands) == 0 {
		t.Fatalf("stroke %v with %d commands, want an unstroked icon with commands", icon.Stroke, len(icon.Commands))
	}
}

// A view box that is not square keeps both its sides, so a horizontal logo is
// not reduced to its height.
func TestNonSquareViewBoxKeepsBothSides(t *testing.T) {
	wide, err := Compile("wide", []byte(`<svg viewBox="0 0 40 10"><path fill="#00aa00" d="M0 0h40v10z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if wide.ViewBox != 40 || wide.ViewHeight != 10 {
		t.Fatalf("wide view box %v x %v, want 40 x 10", wide.ViewBox, wide.ViewHeight)
	}
	square, err := Compile("square", []byte(`<svg viewBox="0 0 24 24"><path fill="#00aa00" d="M0 0h24v24z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if square.ViewBox != 24 || square.ViewHeight != 0 {
		t.Fatalf("square view box %v x %v, want 24 and an unset height", square.ViewBox, square.ViewHeight)
	}
	source, err := Generate("icons", map[string]ui.Icon{"wide": wide}, "")
	if err != nil || !strings.Contains(string(source), "ViewHeight: 10.0") {
		t.Fatalf("generated source lacks the view height: %v\n%s", err, source)
	}
}

const gradientSource = `<svg viewBox="10 10 24 24"><defs>
<linearGradient id="g" gradientUnits="userSpaceOnUse" x1="14" y1="10" x2="30" y2="10">
<stop stop-color="#ff0000"/><stop offset="100%" style="stop-color:#0000ff;stop-opacity:.5"/></linearGradient></defs>
<g transform="scale(2)"><path fill="url(#g)" d="M10 10h20v20z"/></g></svg>`

func TestLinearGradientFillBecomesAGradientLayer(t *testing.T) {
	icon, err := Compile("grad", []byte(gradientSource))
	if err != nil {
		t.Fatal(err)
	}
	if len(icon.Layers) != 1 || icon.Layers[0].Gradient == nil || len(icon.Commands) != 0 {
		t.Fatalf("compiled to %+v", icon)
	}
	gradient := icon.Layers[0].Gradient
	// The view box origin (10,10) and the scale(2) of the group both apply to
	// the endpoints, as they do to the path.
	if gradient.Start != (ui.Point{X: 18, Y: 10}) || gradient.End != (ui.Point{X: 50, Y: 10}) {
		t.Fatalf("endpoints = %+v → %+v", gradient.Start, gradient.End)
	}
	if gradient.From != (ui.Color{Red: 1, Alpha: 1}) || gradient.To != (ui.Color{Blue: 1, Alpha: 0.5}) {
		t.Fatalf("colors = %+v → %+v", gradient.From, gradient.To)
	}
	source, err := Generate("icons", map[string]ui.Icon{"grad": icon}, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Gradient: &ui.LinearGradient{Start: ui.Point{X: 18.0, Y: 10.0}"; !strings.Contains(string(source), want) {
		t.Fatalf("generated source lacks %q:\n%s", want, source)
	}
}

func TestGradientsTheCanvasCannotPaintAreRejected(t *testing.T) {
	wrap := func(gradient, attributes string) string {
		return `<svg viewBox="0 0 24 24"><defs>` + gradient + `</defs><path fill="url(#g)"` + attributes + ` d="M0 0h8v8z"/></svg>`
	}
	stops := `<stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#000"/>`
	linear := `<linearGradient id="g" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="8" y2="0"`
	for name, source := range map[string]string{
		"slanted bounding-box":  wrap(`<linearGradient id="g" x1="0" y1="0" x2="1" y2="1">`+stops+`</linearGradient>`, ""),
		"rotated bounding-box":  wrap(`<linearGradient id="g">`+stops+`</linearGradient>`, ` transform="rotate(30)"`),
		"unknown units":         wrap(`<linearGradient id="g" gradientUnits="viewport" x1="0" y1="0" x2="8" y2="0">`+stops+`</linearGradient>`, ""),
		"percent in user space": wrap(`<linearGradient id="g" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="50%" y2="0">`+stops+`</linearGradient>`, ""),
		"three stops":           wrap(linear+`>`+stops+`<stop offset=".5" stop-color="#888"/></linearGradient>`, ""),
		"radial":                wrap(`<radialGradient id="g" gradientUnits="userSpaceOnUse">`+stops+`</radialGradient>`, ""),
		"transform":             wrap(linear+` gradientTransform="rotate(10)">`+stops+`</linearGradient>`, ""),
		"spread":                wrap(linear+` spreadMethod="reflect">`+stops+`</linearGradient>`, ""),
		"skewed element":        wrap(linear+`>`+stops+`</linearGradient>`, ` transform="matrix(1 0 1 1 0 0)"`),
		"missing":               wrap(``, ""),
	} {
		if _, err := Compile("bad", []byte(source)); err == nil {
			t.Errorf("%s: compiled", name)
		}
	}
}

// Transform arguments may be separated by spaces as well as commas, and a list may
// hold several transforms; both used to be silently dropped.
func TestTransformListsWithSpacedArgumentsApply(t *testing.T) {
	for raw, want := range map[string]transform{
		"translate(3 4)":                  {1, 0, 0, 1, 3, 4},
		"matrix(1 0 0 1 5 6)":             {1, 0, 0, 1, 5, 6},
		"translate(3,4) scale(2)":         {2, 0, 0, 2, 3, 4},
		"translate(3, 4)  translate(1 1)": {1, 0, 0, 1, 4, 5},
		"scale(2 3) translate(1,1)":       {2, 0, 0, 3, 2, 3},
	} {
		if got := parseTransform(raw); got != want {
			t.Errorf("%q = %+v, want %+v", raw, got, want)
		}
	}
}

// A brand mark often puts its color on the root element, not on each path. That
// used to be ignored, leaving the shapes unpainted, so they were stroked as
// outlines in the theme's text color.
func TestPaintOnTheRootElementIsInherited(t *testing.T) {
	icon, err := Compile("brand", []byte(`<svg viewBox="0 0 24 24" fill="#5786FE"><path d="M2 2h20v20H2z"/><path fill="#ff0000" d="M4 4h4v4H4z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(icon.Commands) != 0 || len(icon.Layers) != 2 {
		t.Fatalf("compiled to %d stroked commands and %d layers, want 0 and 2", len(icon.Commands), len(icon.Layers))
	}
	if got := icon.Layers[0].Fill; got != (ui.Color{Red: 0x57 / 255.0, Green: 0x86 / 255.0, Blue: 0xFE / 255.0, Alpha: 1}) {
		t.Fatalf("the path took %+v, not the root's color", got)
	}
	if icon.Layers[1].Fill.Red != 1 {
		t.Fatalf("a path's own fill must win over the root's: %+v", icon.Layers[1].Fill)
	}

	// The stroked Lucide convention keeps working: fill none and a stroke on the root.
	lucide, err := Compile("lucide", []byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 12h16"/></svg>`))
	if err != nil || len(lucide.Commands) == 0 || len(lucide.Layers) != 0 {
		t.Fatalf("a stroked icon compiled to %+v (%v)", lucide, err)
	}
	// A root fill of none with nothing to stroke paints nothing, as in SVG.
	if _, err := Compile("empty", []byte(`<svg viewBox="0 0 24 24" fill="none"><path d="M4 12h16"/></svg>`)); err == nil {
		t.Fatal("an icon that paints nothing compiled")
	}
}

// Arc flags are one character each and need no separator after them, which icon
// minifiers rely on; they used to be read as numbers, so "0 110 3.846" became a
// rotation of 0, a large-arc flag of 110 and an endpoint that swallowed the rest.
func TestArcFlagsMayBeCompact(t *testing.T) {
	for input, want := range map[string][7]float64{
		"1.923 1.923 0 110 3.846":     {1.923, 1.923, 0, 1, 1, 0, 3.846},
		"1.697 1.697 0 011.099 2.132": {1.697, 1.697, 0, 0, 1, 1.099, 2.132},
		".24.24 0 00-.244.248":        {.24, .24, 0, 0, 0, -.244, .248},
		"2 2 0 0 1 2 2":               {2, 2, 0, 0, 1, 2, 2},
		"5,5,0,1,0,3,4":               {5, 5, 0, 1, 0, 3, 4},
	} {
		parser := &pathParser{input: input}
		got, ok := parser.seven()
		if !ok || got != want {
			t.Errorf("%q parsed to %v (ok %v), want %v", input, got, ok, want)
		}
	}
}

// The first path of the Kimi mark is a dot drawn as one arc with compact flags.
func TestCompactArcDotKeepsItsSize(t *testing.T) {
	icon, err := Compile("dot", []byte(`<svg viewBox="0 0 24 24" fill="#000"><path d="M21.846 0a1.923 1.923 0 110 3.846H20.15a.226.226 0 01-.227-.226V1.923C19.923.861 20.784 0 21.846 0z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	var minX, maxX, minY, maxY float32 = 1e9, -1e9, 1e9, -1e9
	for _, command := range icon.Layers[0].Commands {
		for _, point := range []ui.Point{command.Point, command.Control1, command.Control2} {
			if command.Kind == ui.PathCommandClose {
				continue
			}
			minX, maxX = min(minX, point.X), max(maxX, point.X)
			minY, maxY = min(minY, point.Y), max(maxY, point.Y)
		}
	}
	if maxY-minY < 3.5 || maxY-minY > 4 || maxX > 24 {
		t.Fatalf("the dot spans x %v..%v y %v..%v, want a disc about 3.8 tall inside the icon", minX, maxX, minY, maxY)
	}
}

// Gradients in SVG's default units are fractions of the shape's own box. MiniMax's
// and Qwen's color marks are written that way, with percentages.
func TestBoundingBoxGradientIsPlacedOnTheShapesBox(t *testing.T) {
	icon, err := Compile("box", []byte(`<svg viewBox="0 0 24 24"><defs>
<linearGradient id="g" x1="0%" x2="100.5%" y1="50%" y2="50%"><stop offset="0%" stop-color="#e2167e"/><stop offset="100%" stop-color="#fe603c"/></linearGradient>
</defs><path fill="url(#g)" d="M4 6h12v10H4z"/><path fill="url(#g)" transform="translate(2 0) scale(2)" d="M10 4h4v4h-4z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(icon.Layers) != 2 {
		t.Fatalf("%d layers", len(icon.Layers))
	}
	first, second := icon.Layers[0].Gradient, icon.Layers[1].Gradient
	if first.Start != (ui.Point{X: 4, Y: 11}) || math.Abs(float64(first.End.X)-16.06) > 1e-4 || first.End.Y != 11 {
		t.Fatalf("first gradient runs %+v to %+v, want the box 4..16 across, at its middle height", first.Start, first.End)
	}
	// The same gradient on another shape lands on that shape's box, after its transform.
	if second.Start != (ui.Point{X: 22, Y: 12}) || math.Abs(float64(second.End.X)-30.04) > 1e-4 {
		t.Fatalf("second gradient runs %+v to %+v, want the box 22..30 across at height 12", second.Start, second.End)
	}
	if first.From != (ui.Color{Red: 0xe2 / 255.0, Green: 0x16 / 255.0, Blue: 0x7e / 255.0, Alpha: 1}) {
		t.Fatalf("first stop %+v", first.From)
	}
}

// The box of a curved shape is its true extent, not its control points'.
func TestCommandBoundsFollowsCurves(t *testing.T) {
	minX, minY, maxX, maxY, ok := commandBounds([]ui.PathCommand{ui.MoveTo(0, 0), ui.CubicTo(0, 10, 10, 10, 10, 0), ui.ClosePath()})
	if !ok || minX != 0 || maxX != 10 || minY != 0 || maxY < 7.4 || maxY > 7.6 {
		t.Fatalf("bounds = %v %v %v %v, want x 0..10 and y 0..7.5 (the curve's peak, not its control points at 10)", minX, minY, maxX, maxY)
	}
}

// TestEllipseCompilesToItsRing: Lucide's cylinder draws its top with <ellipse>,
// which the compiler used to skip silently — half the icon went missing.
func TestEllipseCompilesToItsRing(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"/></svg>`
	icon, err := Compile("ellipse", []byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	// A ring drawn as one move and four cubics, closed.
	if len(icon.Commands) != 6 {
		t.Fatalf("%d commands, want move + four cubics + close", len(icon.Commands))
	}
	var canvas ui.Canvas
	canvas.DrawIcon(icon, ui.Rect{Width: 24, Height: 24}, ui.Color{Alpha: 1})
	triangles := canvas.Triangles()
	if !inked(triangles, 21, 5) || !inked(triangles, 3, 5) || !inked(triangles, 12, 2) || !inked(triangles, 12, 8) {
		t.Fatal("the ellipse ring has no ink at its cardinal points")
	}
	if inked(triangles, 12, 5) {
		t.Fatal("the ellipse is filled: its centre has ink")
	}
}

// TestTheLucideCylinderKeepsAllThreeParts: the icon that showed the gap in the
// owner's window — its ellipse top, body and middle ring must all be there.
func TestTheLucideCylinderKeepsAllThreeParts(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/></svg>`
	icon, err := Compile("cylinder", []byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	var canvas ui.Canvas
	canvas.DrawIcon(icon, ui.Rect{Width: 24, Height: 24}, ui.Color{Alpha: 1})
	triangles := canvas.Triangles()
	for _, probe := range []struct {
		name string
		x, y float32
	}{
		{"top ellipse", 12, 2.5},
		{"left wall", 3, 12},
		{"right wall", 21, 12},
		{"bottom curve", 12, 21.5},
		{"middle ring", 12, 15},
	} {
		if !inked(triangles, probe.x, probe.y) {
			t.Errorf("%s has no ink at (%.1f, %.1f)", probe.name, probe.x, probe.y)
		}
	}
}

// TestAnUnknownElementIsAnError: a drawable element the compiler does not know
// must fail the compile, not leave a partial icon on screen.
func TestAnUnknownElementIsAnError(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><image href="x.png" x="0" y="0" width="24" height="24"/></svg>`
	if _, err := Compile("image", []byte(svg)); err == nil || !strings.Contains(err.Error(), "unsupported element <image>") {
		t.Fatalf("compiling an <image> gave %v, want an unsupported-element error", err)
	}
}
