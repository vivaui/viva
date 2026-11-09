package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	dlgScope = tree.Sub(page, tree.Name("dialog"))
	dlgClose = dlgScope.ID(tree.Name("close"))
	dlgNo    = dlgScope.ID(tree.Name("cancel"))
	dlgYes   = dlgScope.ID(tree.Name("delete"))
)

// A Dialog looks exactly like the components.Dialog it is painted by: the same scrim,
// surface, title, close button, body text and footer buttons in the same places.
func TestDialogIsPaintedLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Rect{Width: 640, Height: 400}
	for _, description := range []string{"This cannot be undone.", ""} {
		flexed, direct := testCanvas(), testCanvas()
		var view View
		view.Draw(flexed, theme, viewport, func(f *Frame) {
			f.Column().Pad(12).Children(func() { f.Text("page behind") })
			d := f.Dialog(DialogSpec{Title: "Delete host?", Description: description, Width: 400, Scope: dlgScope.ID(tree.Name("scope")), DismissID: dlgNo, CloseID: dlgClose})
			d.Body(func() { f.Text("Body text") })
			d.Footer(func() {
				f.Button(dlgNo, "Cancel").Outline()
				f.Button(dlgYes, "Delete").Destructive()
			})
		})

		direct.Label("page behind", gfx.Rect{X: 12, Y: 12, Width: viewport.Width - 24, Height: theme.Metrics.FontSizeM + 7}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
		dialog := components.BeginDialog(direct, theme, viewport, components.DialogOptions{
			Layer: 2, Scope: dlgScope.ID(tree.Name("scope")), DismissID: dlgNo, CloseID: dlgClose,
			ContentSize: gfx.Size{Width: 400}, FitBody: true, BodyHeight: theme.Metrics.FontSizeM + 7,
		})
		layout := dialog.Begin("Delete host?", description)
		direct.Label("Body text", gfx.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: theme.Metrics.FontSizeM + 7}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
		dialog.Actions(layout,
			components.ButtonAction{ID: dlgNo, Label: "Cancel", Variant: components.ButtonOutline},
			components.ButtonAction{ID: dlgYes, Label: "Delete", Variant: components.ButtonDestructive})
		dialog.End()

		sameCanvas(t, "description "+description, flexed, direct)
		if len(flexed.Hits()) < 5 {
			t.Errorf("%d hits, want the scrim, the surface, the close button and two buttons", len(flexed.Hits()))
		}
	}
}

func TestDialogReportsDismissalAndTheButtonsAreClicked(t *testing.T) {
	theme := gfx.LightTheme()
	draw := func(canvas *gfx.Canvas) (dismissed, deleted bool) {
		var view View
		view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) {
			d := f.Dialog(DialogSpec{Title: "Delete host?", Width: 400, Scope: dlgScope.ID(tree.Name("scope")), DismissID: dlgNo, CloseID: dlgClose})
			d.Footer(func() {
				f.Button(dlgNo, "Cancel").Outline()
				deleted = f.Button(dlgYes, "Delete").Destructive().Clicked()
			})
			dismissed = d.Dismissed()
		})
		return
	}
	canvas := testCanvas()
	if dismissed, deleted := draw(canvas); dismissed || deleted {
		t.Fatal("a paint that is not an event pass reports a click")
	}
	if got := canvas.Hits()[0]; got.ID != dlgNo || got.Bounds != (gfx.Rect{Width: 640, Height: 400}) {
		t.Errorf("the first hit is %+v, want the scrim, which dismisses", got)
	}
	for _, test := range []struct {
		click              gfx.ID
		dismissed, deleted bool
	}{{dlgNo, true, false}, {dlgClose, true, false}, {dlgYes, false, true}} {
		canvas = testCanvas()
		canvas.BeginEventPass(test.click, gfx.ClickInfo{})
		dismissed, deleted := draw(canvas)
		if dismissed != test.dismissed || deleted != test.deleted {
			t.Errorf("click %v: dismissed=%v deleted=%v, want %v and %v", test.click, dismissed, deleted, test.dismissed, test.deleted)
		}
	}
}

// A Select opened inside a dialog is above it, and a dialog with a height of its own
// gives the rest to its body.
func TestDialogRaisesTheLayerOfWhatIsInsideIt(t *testing.T) {
	theme := gfx.LightTheme()
	canvas := testCanvas()
	var state SelectState
	state.Open = true
	var body Box
	var view View
	view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) {
		d := f.Dialog(DialogSpec{Title: "Size", Width: 400, Height: 300, Scope: dlgScope.ID(tree.Name("scope")), CloseID: dlgClose})
		d.Body(func() {
			body = f.Column().Grow(1)
			f.Select(selScope, &state, selLabels, 0)
		})
	})
	var dialogLayer, listLayer uint16
	for _, hit := range canvas.Hits() {
		if hit.ID == dlgClose {
			dialogLayer = hit.Layer
		}
		if i, ok := hit.ID.In(selScope); ok && i == 0 {
			listLayer = hit.Layer
		}
	}
	if dialogLayer != 2 || listLayer != 3 {
		t.Errorf("dialog on layer %d and its select's list on %d, want 2 and 3", dialogLayer, listLayer)
	}
	if h := body.Rect().Height; h <= 0 {
		t.Errorf("the body is %v tall", h)
	}
}

func TestDialogAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) {
			d := f.Dialog(DialogSpec{Title: "Delete host?", Description: "Sure?", Width: 400, Scope: dlgScope.ID(tree.Name("scope")), DismissID: dlgNo, CloseID: dlgClose})
			d.Body(func() { f.Text("Body") })
			d.Footer(func() {
				f.Button(dlgNo, "Cancel").Outline()
				f.Button(dlgYes, "Delete").Destructive()
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm dialog allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkDialogPage(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) {
			d := f.Dialog(DialogSpec{Title: "Delete host?", Description: "Sure?", Width: 400, Scope: dlgScope.ID(tree.Name("scope")), DismissID: dlgNo, CloseID: dlgClose})
			d.Body(func() { f.Text("Body") })
			d.Footer(func() {
				f.Button(dlgNo, "Cancel").Outline()
				f.Button(dlgYes, "Delete").Destructive()
			})
		})
	}
}

// Escape closes a Dialog with nothing for the application to wire: the canvas routes it
// to the dismiss target (or to the close button when there is none), which Dismissed
// reports like a click on the scrim.
func TestDialogRoutesEscapeToItsDismissTarget(t *testing.T) {
	theme := gfx.LightTheme()
	for name, spec := range map[string]DialogSpec{
		"a dismiss target":    {Title: "T", Scope: dlgScope.ID(tree.Name("scope")), DismissID: dlgNo, CloseID: dlgClose},
		"only a close button": {Title: "T", Scope: dlgScope.ID(tree.Name("scope")), CloseID: dlgClose},
	} {
		want := spec.DismissID
		if want.IsZero() {
			want = spec.CloseID
		}
		canvas := testCanvas()
		var view View
		view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) { f.Dialog(spec) })
		if got := canvas.EscapeTarget(); got != want {
			t.Errorf("%s: Escape goes to %v, want %v", name, got, want)
		}
		canvas = testCanvas()
		canvas.BeginEventPass(want, gfx.ClickInfo{})
		dismissed := false
		view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) { dismissed = f.Dialog(spec).Dismissed() })
		if !dismissed {
			t.Errorf("%s: the click Escape becomes is not reported by Dismissed", name)
		}
	}
	// Without a Dialog nothing is routed.
	canvas := testCanvas()
	var view View
	view.Draw(canvas, theme, gfx.Rect{Width: 640, Height: 400}, func(f *Frame) { f.Column() })
	if !canvas.EscapeTarget().IsZero() {
		t.Error("a page without a dialog routes Escape")
	}
}
