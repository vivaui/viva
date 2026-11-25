package steps_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/steps"
)

var items = []steps.Step{
	{Title: "Account", Description: "Create a login"},
	{Title: "Profile"},
	{Title: "Done"},
}

func measure(value string, size, weight float32) float32 {
	return float32(len(value)) * size * 0.55
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 240}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	canvas.SetReduceMotion(true)
	draw(page, canvas, view)
	return page, canvas
}

func hitByLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].IsLabel && !hits[i].Backdrop {
			return hits[i], true
		}
	}
	return gfx.Hit{}, false
}

func click(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
	draw(page, canvas, view)
}

// markerFills counts the circles of the markers by the color they are filled
// with, so a test can say which step stands how without knowing its position.
func markerFills(canvas *gfx.Canvas, color gfx.Color) int {
	count := 0
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color && rectangle.Radius >= 8 && rectangle.Bounds.Width == rectangle.Bounds.Height {
			count++
		}
	}
	return count
}

// The steps behind the current one are finished, the current one is under way
// and the rest wait, each painted in the tone of its status.
func TestStepsStandByTheirPlaceFromTheCurrentOne(t *testing.T) {
	theme := gfx.LightTheme()
	current := 1
	view := func(c ui.C) {
		steps.View(c, "Checkout", &current, steps.Spec{Steps: items}).FillWidth()
	}
	_, canvas := open(t, view)
	for name, want := range map[string]struct {
		color gfx.Color
		count int
	}{
		"finished": {theme.Secondary, 1},
		"current":  {theme.Primary, 1},
		"waiting":  {theme.Muted, 1},
		"failed":   {theme.Danger, 0},
	} {
		if got := markerFills(canvas, want.color); got != want.count {
			t.Fatalf("%s markers = %d, want %d", name, got, want.count)
		}
	}
}

// The status says how the current step stands: a failed one is painted in the
// danger color and the steps around it are unchanged.
func TestStepsShowAFailedCurrentStep(t *testing.T) {
	theme := gfx.LightTheme()
	current := 1
	view := func(c ui.C) {
		steps.View(c, "Checkout", &current, steps.Spec{Steps: items, Status: steps.Error}).FillWidth()
	}
	_, canvas := open(t, view)
	if got := markerFills(canvas, theme.Danger); got != 1 {
		t.Fatalf("failed markers = %d, want 1", got)
	}
	if got := markerFills(canvas, theme.Primary); got != 0 {
		t.Fatalf("under-way markers = %d, want 0 while the current step failed", got)
	}
}

// A clickable bar makes the clicked step the current one; a bar that is not
// clickable offers no step to click.
func TestStepsClickMovesTheCurrentStep(t *testing.T) {
	current := 0
	view := func(c ui.C) {
		steps.View(c, "Checkout", &current, steps.Spec{Steps: items, Clickable: true}).FillWidth()
	}
	page, canvas := open(t, view)
	hit, ok := hitByLabel(canvas, "Done")
	if !ok {
		t.Fatal("a clickable bar has no step named Done to click")
	}
	click(page, canvas, view, hit.ID)
	if current != 2 {
		t.Fatalf("current = %d after a click on the last step, want 2", current)
	}

	still := 0
	fixed := func(c ui.C) {
		steps.View(c, "Checkout", &still, steps.Spec{Steps: items}).FillWidth()
	}
	_, canvas = open(t, fixed)
	if _, ok := hitByLabel(canvas, "Done"); ok {
		t.Fatal("a bar that is not clickable offers a step to click")
	}
}

// A clickable step is a button for the keyboard too: Tab reaches it and Enter
// chooses it.
func TestStepsChooseWithTheKeyboard(t *testing.T) {
	current := 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		steps.View(c, "Checkout", &current, steps.Spec{Steps: items, Clickable: true}).FillWidth()
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Steps", Width: 480, Height: 200}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	session.Keys("tab")
	session.Keys("tab")
	session.Keys("enter")
	if current != 1 {
		t.Fatalf("current = %d after Tab, Tab and Enter, want the second step", current)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]steps.Spec{
		"horizontal": {Steps: items, Clickable: true},
		"vertical":   {Steps: items, Direction: steps.Vertical, Type: steps.Dot, Size: steps.Small},
	} {
		t.Run(name, func(t *testing.T) {
			current := 1
			view := func(c ui.C) { steps.View(c, "Checkout", &current, spec).FillWidth() }
			page, canvas := open(t, view)
			if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
				t.Fatalf("warm paint allocated %v times, want 0", allocations)
			}
		})
	}
}

func BenchmarkViewPaint(b *testing.B) {
	current := 1
	view := func(c ui.C) { steps.View(c, "Checkout", &current, steps.Spec{Steps: items}).FillWidth() }
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
