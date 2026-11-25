package components

import (
	"fmt"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSonnerPaintsCallerOwnedStackNewestOnTop(t *testing.T) {
	var canvas gfx.Canvas
	toasts := []SonnerToast{
		{Snapshot: ToastSnapshot{Title: "Newest", Index: 99}, Scope: gfx.Item(10), DismissID: gfx.Item(1)},
		{Snapshot: ToastSnapshot{Title: "Middle", Index: 99}, Scope: gfx.Item(11), DismissID: gfx.Item(2)},
		{Snapshot: ToastSnapshot{Title: "Oldest", Index: 99}, Scope: gfx.Item(12), DismissID: gfx.Item(3)},
	}
	viewport := gfx.Rect{Width: 1000, Height: 700}
	layout := PaintSonner(&canvas, gfx.LightTheme(), viewport, toasts, SonnerOptions{Layer: 40})
	if layout.Painted != 3 {
		t.Fatalf("painted = %d", layout.Painted)
	}
	// The stack shares one width, so its cards line up: the union's right and
	// bottom edges are the viewport's inset edges, not a sum of card widths.
	if layout.Bounds.Width <= 0 || layout.Bounds.Width > toastMaxWidth ||
		layout.Bounds.X+layout.Bounds.Width != viewport.Width-toastInset ||
		layout.Bounds.Y+layout.Bounds.Height != viewport.Height-toastInset {
		t.Fatalf("stack layout = %+v", layout)
	}
	if canvas.EscapeTarget() != gfx.Item(1) {
		t.Fatalf("escape target = %d", canvas.EscapeTarget())
	}
	if len(canvas.Hits()) != 3 || canvas.Hits()[0].ID != gfx.Item(3) || canvas.Hits()[2].ID != gfx.Item(1) {
		t.Fatalf("stack hits = %+v", canvas.Hits())
	}
	// The newest toast is at the edge and the older ones are lifted behind it.
	row := map[string]float32{}
	for _, text := range canvas.Texts() {
		row[text.Value] = text.Bounds.Y
	}
	if !(row["Newest"] > row["Middle"] && row["Middle"] > row["Oldest"]) {
		t.Fatalf("stack rows = %v, want the newest nearest the bottom", row)
	}
	seenLayer := [3]bool{}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer < 40 || rectangle.Layer > 42 {
			t.Fatalf("rectangle layer = %d", rectangle.Layer)
		}
		seenLayer[rectangle.Layer-40] = true
	}
	if seenLayer != [3]bool{true, true, true} {
		t.Fatalf("layers = %v", seenLayer)
	}
	for _, toast := range toasts {
		if toast.Snapshot.Index != 99 {
			t.Fatal("caller snapshot was mutated")
		}
	}
}

func TestSonnerExpandedGeometryAndLimitedEntry(t *testing.T) {
	var canvas gfx.Canvas
	viewport := gfx.Rect{X: 2, Y: 3, Width: 800, Height: 600}
	layout := PaintSonner(&canvas, gfx.DarkTheme(), viewport, []SonnerToast{
		{Snapshot: ToastSnapshot{Title: "One", Description: "Details"}, DismissID: gfx.Item(1)},
		{Snapshot: ToastSnapshot{Title: "Hidden", Limited: true}, DismissID: gfx.Item(2)},
		{Snapshot: ToastSnapshot{Title: "Three", Description: "Details"}, DismissID: gfx.Item(3)},
	}, SonnerOptions{Layer: 8, Expanded: true})
	if layout.Painted != 2 {
		t.Fatalf("painted = %d", layout.Painted)
	}
	if layout.Bounds.Width <= 0 || layout.Bounds.Width > toastMaxWidth ||
		layout.Bounds.X+layout.Bounds.Width != viewport.X+viewport.Width-toastInset ||
		layout.Bounds.Y+layout.Bounds.Height != viewport.Y+viewport.Height-toastInset {
		t.Fatalf("expanded bounds = %+v", layout.Bounds)
	}
}

func TestSonnerPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	toasts := []SonnerToast{
		{Snapshot: ToastSnapshot{Title: "Saved", Type: ToastSuccess}, Scope: gfx.Item(20), DismissID: gfx.Item(1)},
		{Snapshot: ToastSnapshot{Title: "Saving", Type: ToastLoading, SwipeX: 4}, Scope: gfx.Item(21), DismissID: gfx.Item(2), ActionID: gfx.Item(3), ActionLabel: "Cancel"},
	}
	paint := func() {
		canvas.Reset()
		PaintSonner(&canvas, gfx.LightTheme(), gfx.Rect{Width: 900, Height: 640}, toasts, SonnerOptions{Layer: 16})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("sonner stream changed")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("sonner allocations = %v", allocations)
	}
}

func TestSonnerShowsOnlyTheNewestLimitToasts(t *testing.T) {
	var canvas gfx.Canvas
	toasts := make([]SonnerToast, 5)
	for index := range toasts {
		toasts[index] = SonnerToast{Snapshot: ToastSnapshot{Title: fmt.Sprintf("Toast %d", index)}, DismissID: gfx.Item(index + 1)}
	}
	viewport := gfx.Rect{Width: 1000, Height: 700}
	if layout := PaintSonner(&canvas, gfx.LightTheme(), viewport, toasts, SonnerOptions{Layer: 1}); layout.Painted != 3 {
		t.Fatalf("painted = %d, want Base UI's default limit of 3", layout.Painted)
	}
	// The slice is newest first, so the visible three are 0, 1 and 2.
	seen := map[string]bool{}
	for _, text := range canvas.Texts() {
		seen[text.Value] = true
	}
	for index := 0; index < 3; index++ {
		if title := fmt.Sprintf("Toast %d", index); !seen[title] {
			t.Fatalf("newest toast %q is not painted: %v", title, seen)
		}
	}
	for index := 3; index < len(toasts); index++ {
		if title := fmt.Sprintf("Toast %d", index); seen[title] {
			t.Fatalf("toast %q beyond the limit is painted: %v", title, seen)
		}
	}
	canvas.Reset()
	if layout := PaintSonner(&canvas, gfx.LightTheme(), viewport, toasts, SonnerOptions{Layer: 1, Limit: 5}); layout.Painted != 5 {
		t.Fatalf("painted = %d with Limit 5", layout.Painted)
	}
}
