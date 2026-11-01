package componentdemo

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var testSize = ui.Size{Width: 320, Height: 340}

type paintSnapshot struct {
	rectangles []ui.Rectangle
	texts      []ui.Text
	hits       []ui.Hit
	scope      ui.ID
	escape     ui.ID
}

func newTestState(testingContext testing.TB, componentID, exampleID string) *State {
	testingContext.Helper()
	state, err := New(componentID, exampleID)
	if err != nil {
		testingContext.Fatal(err)
	}
	return state
}

func paintFrame(canvas *ui.Canvas, state *State, size ui.Size, now time.Duration, reduce bool) {
	canvas.SetReduceMotion(reduce)
	if !reduce {
		canvas.SetClock(func() time.Duration { return now })
	}
	canvas.BeginFrame()
	canvas.Reset()
	state.Paint(canvas, size)
	canvas.EndFrame()
}

// bareState is a State for a component painted without New, as the catalog
// tests do. It has the layout memory New gives, so painting a flex page does not
// look like a change of the State.
func bareState(componentID, exampleID string) *State {
	return &State{componentID: componentID, exampleID: exampleID, view: new(flex.View)}
}

// click delivers a click on id the way the runtime does: one paint, the event
// pass, in which the control that declared id reads Clicked as true. The state it
// changes shows in the next paint.
func click(canvas *ui.Canvas, state *State, id ui.ID) {
	clickAt(canvas, state, id, time.Second, true)
}

func clickAt(canvas *ui.Canvas, state *State, id ui.ID, now time.Duration, reduce bool) {
	canvas.BeginEventPass(id)
	paintFrame(canvas, state, testSize, now, reduce)
	canvas.EndEventPass()
}

// hitsWithoutPlacement copies hits without the row layout a Textarea hands the
// runtime: each State owns its own, which is not part of what is painted.
func hitsWithoutPlacement(hits []ui.Hit) []ui.Hit {
	hits = slices.Clone(hits)
	for index := range hits {
		hits[index].Placement = nil
	}
	return hits
}

func snapshot(canvas *ui.Canvas) paintSnapshot {
	return paintSnapshot{
		rectangles: slices.Clone(canvas.Rectangles()), texts: slices.Clone(canvas.Texts()),
		hits: hitsWithoutPlacement(canvas.Hits()), scope: canvas.TrappedFocusScope(), escape: canvas.EscapeTarget(),
	}
}

func assertSamePaint(testingContext testing.TB, expected paintSnapshot, canvas *ui.Canvas) {
	testingContext.Helper()
	if !slices.Equal(expected.rectangles, canvas.Rectangles()) || !slices.Equal(expected.texts, canvas.Texts()) ||
		!slices.Equal(expected.hits, hitsWithoutPlacement(canvas.Hits())) || expected.scope != canvas.TrappedFocusScope() || expected.escape != canvas.EscapeTarget() {
		testingContext.Fatal("identical state produced different paint or interaction streams")
	}
}

func findHit(testingContext testing.TB, canvas *ui.Canvas, id ui.ID) ui.Hit {
	testingContext.Helper()
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit
		}
	}
	testingContext.Fatalf("missing hit %v", id)
	return ui.Hit{}
}

func hasText(canvas *ui.Canvas, value string) bool {
	for _, label := range canvas.Texts() {
		if label.Value == value {
			return true
		}
	}
	return false
}

func textBounds(testingContext testing.TB, canvas *ui.Canvas, value string) ui.Rect {
	testingContext.Helper()
	for _, label := range canvas.Texts() {
		if label.Value == value {
			return label.Bounds
		}
	}
	testingContext.Fatalf("missing text %q", value)
	return ui.Rect{}
}

func measureTestText(value string, size, weight float32) float32 {
	width := float32(0)
	for _, character := range value {
		if character > 0x2e7f {
			width += size
		} else {
			width += size * 0.54
		}
	}
	return width
}

func TestCatalogAndSelection(t *testing.T) {
	coreExamples := map[string][]string{
		"button":   {"default", "variants", "disabled"},
		"badge":    {"default", "variants", "long-text"},
		"checkbox": {"default", "selection", "states"},
		"switch":   {"default", "sizes", "states"},
		"input":    {"default", "placeholder", "invalid", "states"},
		"dialog":   {"default", "confirm", "destructive"},
	}
	if len(Catalog()) == 0 || &Catalog()[0] != &Catalog()[0] {
		t.Fatal("catalog must return one fixed nonempty slice")
	}
	seen := make(map[string]bool, len(Catalog()))
	for index, component := range Catalog() {
		if seen[component.ID] || component.ID == "" || component.Title == "" || component.Category == "" || component.Summary == "" || component.paint == nil {
			t.Fatalf("incomplete component metadata: %+v", component)
		}
		seen[component.ID] = true
		if index > 0 && componentLess(component, Catalog()[index-1]) {
			t.Fatalf("catalog is not sorted: %s before %s", Catalog()[index-1].ID, component.ID)
		}
		var identifiers []string
		exampleSeen := make(map[string]bool, len(component.Examples))
		for _, example := range component.Examples {
			if exampleSeen[example.ID] || example.ID == "" || example.Title == "" || example.Description == "" {
				t.Fatalf("incomplete example metadata: %+v", example)
			}
			exampleSeen[example.ID] = true
			identifiers = append(identifiers, example.ID)
			state := newTestState(t, component.ID, example.ID)
			if state.componentID != component.ID || state.exampleID != example.ID {
				t.Fatal("New selected the wrong example")
			}
		}
		if expected, ok := coreExamples[component.ID]; ok && !slices.Equal(identifiers, expected) {
			t.Fatalf("%s examples = %v", component.ID, identifiers)
		}
		state := newTestState(t, component.ID, "")
		if state.exampleID != component.Examples[0].ID {
			t.Fatal("empty example did not select the first entry")
		}
	}
	for component := range coreExamples {
		if !seen[component] {
			t.Fatalf("core component %s is missing", component)
		}
	}
}

func TestUnknownIDsReturnErrors(t *testing.T) {
	for _, componentID := range []string{"", "unknown", "Button", " button"} {
		if state, err := New(componentID, ""); err == nil || state != nil {
			t.Fatalf("New(%q, empty) = %v, %v", componentID, state, err)
		}
	}
	for _, component := range Catalog() {
		for _, exampleID := range []string{"unknown", "Default", "default ", "../default"} {
			if state, err := New(component.ID, exampleID); err == nil || state != nil {
				t.Fatalf("New(%q, %q) = %v, %v", component.ID, exampleID, state, err)
			}
		}
	}
}

func TestPaintDeterministicResponsiveAndIdle(t *testing.T) {
	for _, component := range Catalog() {
		for _, example := range component.Examples {
			for _, dark := range []bool{false, true} {
				for _, width := range []float32{320, 479, 527, 528, 760} {
					t.Run(fmt.Sprintf("%s/%s/dark=%t/width=%.0f", component.ID, example.ID, dark, width), func(t *testing.T) {
						state := newTestState(t, component.ID, example.ID)
						state.SetDark(dark)
						before := *state
						var canvas ui.Canvas
						canvas.SetMeasurer(measureTestText)
						size := ui.Size{Width: width, Height: 340}
						paintFrame(&canvas, state, size, time.Second, true)
						assertLayout(t, &canvas, size)
						if canvas.Animating() || *state != before {
							t.Fatal("paint mutated application state or requested idle frames")
						}
						expected := snapshot(&canvas)
						paintFrame(&canvas, state, size, time.Second, true)
						assertSamePaint(t, expected, &canvas)
						// An overlay enters when it first appears with a clock; five
						// seconds later it is in, and paints what reduced motion did.
						paintFrame(&canvas, state, size, 5*time.Second, false)
						paintFrame(&canvas, state, size, 10*time.Second, false)
						assertSamePaint(t, expected, &canvas)
						if canvas.Animating() {
							t.Fatal("unchanged example requested another frame")
						}
						canvas.BeginEventPass(ui.Item(999)) // a click on nothing the example declared
						paintFrame(&canvas, state, size, 10*time.Second, true)
						canvas.EndEventPass()
						paintFrame(&canvas, state, size, 10*time.Second, true)
						assertSamePaint(t, expected, &canvas)
					})
				}
			}
		}
	}
}

func assertLayout(t *testing.T, canvas *ui.Canvas, size ui.Size) {
	t.Helper()
	inside := func(bounds ui.Rect) bool {
		return bounds.X >= 0 && bounds.Y >= 0 && bounds.Width >= 0 && bounds.Height >= 0 &&
			bounds.X+bounds.Width <= size.Width+0.01 && bounds.Y+bounds.Height <= size.Height+0.01
	}
	for _, rectangle := range canvas.Rectangles() {
		if !inside(rectangle.Bounds) {
			t.Fatalf("rectangle outside viewport: %+v", rectangle)
		}
	}
	for _, label := range canvas.Texts() {
		ink := label.Bounds
		ink.Width = measureTestText(label.Value, label.Style.Size, label.Style.Weight)
		if label.HasClip {
			clip := label.ClipOf
			left := max(ink.X, clip.X)
			top := max(ink.Y, clip.Y)
			right := min(ink.X+ink.Width, clip.X+clip.Width)
			bottom := min(ink.Y+ink.Height, clip.Y+clip.Height)
			ink = ui.Rect{X: left, Y: top, Width: max(right-left, 0), Height: max(bottom-top, 0)}
		}
		if !inside(ink) {
			t.Fatalf("text outside viewport: %+v; measured %+v", label, ink)
		}
	}
	seen := make(map[ui.ID]bool)
	for index, hit := range canvas.Hits() {
		if hit.Block || hit.Container {
			// An occluder or a semantic container has no ID and may sit under
			// later hits by design.
			if !inside(hit.Bounds) || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
				t.Fatalf("invalid or clipped block/container: %+v", hit)
			}
			continue
		}
		// A label shares the identity of the control it labels, by design.
		if hit.ID.IsZero() || seen[hit.ID] && !hit.IsLabel || !inside(hit.Bounds) || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			t.Fatalf("invalid, duplicate, or clipped hit: %+v", hit)
		}
		seen[hit.ID] = seen[hit.ID] || !hit.IsLabel
		for _, previous := range canvas.Hits()[:index] {
			// A hit declared on top of another and wholly inside it is an overlay by design
			// (an attachment's action over its trigger): the later hit wins its own area.
			if !previous.Block && !previous.Container && hit.Scope == previous.Scope && overlap(hit.Bounds, previous.Bounds) && !contains(previous.Bounds, hit.Bounds) {
				t.Fatalf("same-scope hit regions overlap: %+v and %+v", previous, hit)
			}
		}
	}
}

func overlap(left, right ui.Rect) bool {
	return left.X < right.X+right.Width && right.X < left.X+left.Width &&
		left.Y < right.Y+right.Height && right.Y < left.Y+left.Height
}

func TestThemesAndNewResetState(t *testing.T) {
	for _, component := range Catalog() {
		for _, example := range component.Examples {
			t.Run(component.ID+"/"+example.ID, func(t *testing.T) {
				state := newTestState(t, component.ID, example.ID)
				var canvas ui.Canvas
				paintFrame(&canvas, state, testSize, time.Second, true)
				expected := snapshot(&canvas)
				state.SetDark(true)
				paintFrame(&canvas, state, testSize, time.Second, true)
				if canvas.Rectangles()[0].Color != ui.DarkTheme().Background {
					t.Fatal("SetDark did not change the background")
				}
				state.SetDark(false)
				paintFrame(&canvas, state, testSize, time.Second, true)
				assertSamePaint(t, expected, &canvas)
				for _, hit := range expected.hits {
					canvas.BeginEventPass(hit.ID)
					paintFrame(&canvas, state, testSize, time.Second, true)
					canvas.EndEventPass()
				}
				state.EnableTextEditing()
				if editor, ok := state.Editor(inputEditable); ok {
					editor.Insert(" changed")
				}
				fresh := newTestState(t, component.ID, example.ID)
				paintFrame(&canvas, fresh, testSize, time.Second, true)
				assertSamePaint(t, expected, &canvas)
			})
		}
	}
}

func TestPaintPreservesHostCanvasAndFrameOwnership(t *testing.T) {
	state := newTestState(t, "dialog", "confirm")
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{Width: 1, Height: 1}, ui.Color{Red: 1, Alpha: 1})
	canvas.SetClock(func() time.Duration { return 37 * time.Second })
	canvas.BeginFrame()
	state.Paint(&canvas, testSize)
	canvas.EndFrame()
	if canvas.Rectangles()[0].Bounds.Width != 1 || canvas.Now() != 37*time.Second {
		t.Fatal("Paint reset the host canvas or frame clock")
	}
	canvas.HitWithLabel(ui.Item(999), ui.Rect{Width: 1, Height: 1}, "Host")
	canvas.Fill(ui.Rect{Width: 1, Height: 1}, ui.Color{})
	if !findHit(t, &canvas, ui.Item(999)).Scope.IsZero() {
		t.Fatal("Paint leaked a focus scope")
	}
	last := canvas.Rectangles()[len(canvas.Rectangles())-1]
	if last.Layer != 0 || last.HasClip {
		t.Fatal("Paint leaked a layer or clip")
	}
	state.Paint(nil, testSize)
	count := len(canvas.Rectangles())
	state.Paint(&canvas, ui.Size{})
	if len(canvas.Rectangles()) != count {
		t.Fatal("empty viewport painted content")
	}
}

func TestInputEditingConnectionIsExplicitAndShared(t *testing.T) {
	for _, example := range inputComponent.Examples {
		state := newTestState(t, "input", example.ID)
		var canvas ui.Canvas
		paintFrame(&canvas, state, testSize, time.Second, true)
		if state.editor != nil {
			t.Fatal("disconnected preview allocated an unused editor")
		}
		for _, hit := range canvas.Hits() {
			if !hit.Disabled && !hit.ReadOnly {
				t.Fatalf("disconnected hit pretends to be editable: %+v", hit)
			}
			if editor, ok := state.Editor(hit.ID); editor != nil || ok {
				t.Fatal("disconnected preview exposed an editor")
			}
		}
		state.EnableTextEditing()
		paintFrame(&canvas, state, testSize, time.Second, true)
		if example.ID == "states" {
			for _, hit := range canvas.Hits() {
				if editor, ok := state.Editor(hit.ID); editor != nil || ok {
					t.Fatal("disabled/read-only state exposed an editor")
				}
			}
			continue
		}
		if editor, ok := state.Editor(inputEditable); editor == nil || !ok {
			t.Fatal("connected platform preview did not expose the shared editor")
		}
		if hit := findHit(t, &canvas, inputEditable); hit.ReadOnly || hit.Disabled {
			t.Fatalf("connected input is not editable: %+v", hit)
		}
		if !hasText(&canvas, "平台文本服务 · 支持输入、选区与 IME") {
			t.Fatal("connected input does not describe the shared platform contract")
		}
	}
}

func TestSetDensityChangesControlMetrics(t *testing.T) {
	paint := func(density ui.Density) float32 {
		state, err := New("button", "default")
		if err != nil {
			t.Fatal(err)
		}
		state.SetDensity(density)
		var canvas ui.Canvas
		state.Paint(&canvas, ui.Size{Width: 420, Height: 340})
		for _, hit := range canvas.Hits() {
			if hit.ID == buttonPress.At(0) {
				return hit.Bounds.Height
			}
		}
		t.Fatal("button hit not found")
		return 0
	}
	regular, compact, large := paint(ui.DensityRegular), paint(ui.DensityCompact), paint(ui.DensityLarge)
	if !(compact < regular && regular < large) {
		t.Fatalf("button heights compact %v, regular %v, large %v do not follow density", compact, regular, large)
	}
}

func contains(outer, inner ui.Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}
