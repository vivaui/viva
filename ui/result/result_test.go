package result_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/result"
)

func measure(value string, size, weight float32) float32 { return float32(len(value)) * size * 0.55 }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 600, Height: 400}, view)
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

// The icon takes the color of the status, and the result is announced as a
// status named by its title.
func TestStatusColorsTheIconAndNamesTheResult(t *testing.T) {
	theme := gfx.LightTheme()
	for name, test := range map[string]struct {
		status ui.Status
		want   gfx.Color
	}{
		"success": {ui.StatusSuccess, theme.Success},
		"warning": {ui.StatusWarning, theme.Warning},
		"danger":  {ui.StatusDanger, theme.Danger},
		"info":    {ui.StatusDefault, theme.MutedForeground},
	} {
		view := func(c ui.C) { result.View(c, result.Spec{Status: test.status, Title: "Done"}, nil).Fill() }
		_, canvas := open(t, view)
		found := false
		for _, triangle := range canvas.Triangles() {
			if triangle.Color == test.want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: no part of the icon is painted in %+v", name, test.want)
		}
	}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		result.View(c, result.Spec{Status: ui.StatusSuccess, Title: "Key imported"}, nil).Fill()
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Result", Width: 480, Height: 320}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if node, ok := session.AccessByName("Key imported"); !ok || node.Role != gfx.RoleStatus {
		t.Fatalf("access node = %+v, %v; want a status named by the title", node, ok)
	}
}

// The actions sit under the words and are page controls of their own.
func TestActionsSitUnderTheWords(t *testing.T) {
	var c0 ui.C
	actions := func() { ui.Button(c0, "Done") }
	view := func(c ui.C) {
		c0 = c
		result.View(c, result.Spec{Title: "Key imported", Description: "In your vault."}, actions).Fill()
	}
	_, canvas := open(t, view)
	y := map[string]float32{}
	for _, text := range canvas.Texts() {
		y[text.Value] = text.Bounds.Y
	}
	if !(y["Key imported"] < y["In your vault."] && y["In your vault."] < y["Done"]) {
		t.Fatalf("title %v, description %v, action %v are not in order", y["Key imported"], y["In your vault."], y["Done"])
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	var c0 ui.C
	actions := func() { ui.Button(c0, "Done") }
	view := func(c ui.C) {
		c0 = c
		result.View(c, result.Spec{Status: ui.StatusWarning, Title: "Check", Description: "Look again."}, actions).Fill()
	}
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	view := func(c ui.C) { result.View(c, result.Spec{Status: ui.StatusSuccess, Title: "Done"}, nil).Fill() }
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
