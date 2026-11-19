package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func accessibilityLabel(t *testing.T, painter *painter, id gfx.ID) string {
	t.Helper()
	for _, node := range painter.accessibility {
		if node.ID == id {
			return node.Label
		}
	}
	t.Fatalf("no accessibility node for %v", id)
	return ""
}

func labelledPainter(t *testing.T, paint func(*gfx.Canvas)) *painter {
	t.Helper()
	var canvas gfx.Canvas
	config := Config{Title: "labels", Width: 200, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) { paint(canvas) }
	return newTestPainter(t, &canvas, config)
}

func TestLabelHitsNameTheirControlInEitherDeclarationOrder(t *testing.T) {
	bounds := gfx.Rect{Width: 80, Height: 20}
	for _, test := range []struct {
		name  string
		paint func(*gfx.Canvas)
	}{
		{"label after its control", func(canvas *gfx.Canvas) {
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
		}},
		{"label before its control", func(canvas *gfx.Canvas) {
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
		}},
		{"label before its control with another control between", func(canvas *gfx.Canvas) {
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
		}},
		{"label after its control with another control between", func(canvas *gfx.Canvas) {
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			painter := labelledPainter(t, test.paint)
			if got := accessibilityLabel(t, painter, gfx.Item(2)); got != "Real label" {
				t.Fatalf("control 2 label = %q", got)
			}
			if got := accessibilityLabel(t, painter, gfx.Item(1)); got != "Other" {
				t.Fatalf("control 1 label = %q", got)
			}
		})
	}
}

func TestTheFirstOfTwoLabelsForOneControlNamesIt(t *testing.T) {
	bounds := gfx.Rect{Width: 80, Height: 20}
	painter := labelledPainter(t, func(canvas *gfx.Canvas) {
		canvas.HitWithLabel(gfx.Item(1), bounds, "Button")
		canvas.HitLabel(gfx.Item(1), bounds, "first", false)
		canvas.HitLabel(gfx.Item(1), bounds, "second", false)
	})
	if got := accessibilityLabel(t, painter, gfx.Item(1)); got != "first" {
		t.Fatalf("label = %q, want the first label", got)
	}
}

// A label without words names nothing: the control keeps its own label, as an
// empty <label for> falls through to it in HTML. An icon declared LabelFor is
// the case that reaches here.
func TestALabelWithoutWordsLeavesTheControlsOwnName(t *testing.T) {
	bounds := gfx.Rect{Width: 80, Height: 20}
	painter := labelledPainter(t, func(canvas *gfx.Canvas) {
		canvas.HitWithLabel(gfx.Item(1), bounds, "Button")
		canvas.HitLabel(gfx.Item(1), bounds, "", false)
	})
	if got := accessibilityLabel(t, painter, gfx.Item(1)); got != "Button" {
		t.Fatalf("label = %q, want the control's own", got)
	}
}

// A large page with one changed label keeps every other accessible name and
// updates the one that changed. This is the incremental-mirror path: the hit
// list keeps its count, ids and label/control order.
func TestOneChangedLabelOnALargePageUpdatesOnlyItsControl(t *testing.T) {
	const controls = 128
	changed := "first"
	paint := func(canvas *gfx.Canvas) {
		for index := 0; index < controls; index++ {
			id := gfx.Item(index + 1)
			at := gfx.Rect{X: float32(index%16) * 10, Y: float32(index/16) * 10, Width: 8, Height: 8}
			canvas.HitWithLabel(id, at, "control")
			words := "label"
			if index == 3 {
				words = changed
			}
			canvas.HitLabel(id, gfx.Rect{X: at.X, Y: at.Y + 2, Width: 4, Height: 4}, words, false)
		}
	}
	painter := labelledPainter(t, paint)
	if got := accessibilityLabel(t, painter, gfx.Item(4)); got != "first" {
		t.Fatalf("initial label = %q", got)
	}
	changed = "second"
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if got := accessibilityLabel(t, painter, gfx.Item(4)); got != "second" {
		t.Fatalf("changed label = %q", got)
	}
	if got := accessibilityLabel(t, painter, gfx.Item(5)); got != "label" {
		t.Fatalf("untouched label = %q", got)
	}
}

// A large page with one changed value updates that control's accessible value
// without touching the other controls.
func TestOneChangedValueOnALargePageUpdatesOnlyItsControl(t *testing.T) {
	const controls = 128
	changed := "before"
	paint := func(canvas *gfx.Canvas) {
		for index := 0; index < controls; index++ {
			id := gfx.Item(index + 1)
			at := gfx.Rect{X: float32(index%16) * 10, Y: float32(index/16) * 10, Width: 8, Height: 8}
			value := "same"
			if index == 7 {
				value = changed
			}
			canvas.HitWithOptions(id, at, "field", gfx.HitOptions{Role: gfx.RoleTextField, Value: value})
		}
	}
	painter := labelledPainter(t, paint)
	if got := accessibilityValue(t, painter, gfx.Item(8)); got != "before" {
		t.Fatalf("initial value = %q", got)
	}
	changed = "after"
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if got := accessibilityValue(t, painter, gfx.Item(8)); got != "after" {
		t.Fatalf("changed value = %q", got)
	}
	if got := accessibilityValue(t, painter, gfx.Item(9)); got != "same" {
		t.Fatalf("untouched value = %q", got)
	}
}

func accessibilityValue(t *testing.T, painter *painter, id gfx.ID) string {
	t.Helper()
	for _, node := range painter.accessibility {
		if node.ID == id {
			return node.Value
		}
	}
	t.Fatalf("no accessibility node for %v", id)
	return ""
}

// Changing every label on a large page falls back to the full mirror and still
// updates every control; the incremental path is bounded, not lossy.
func TestAllLabelsChangedOnALargePageUpdatesEveryControl(t *testing.T) {
	const controls = 128
	words := "first"
	paint := func(canvas *gfx.Canvas) {
		for index := 0; index < controls; index++ {
			id := gfx.Item(index + 1)
			at := gfx.Rect{X: float32(index%16) * 10, Y: float32(index/16) * 10, Width: 8, Height: 8}
			canvas.HitWithLabel(id, at, "control")
			canvas.HitLabel(id, gfx.Rect{X: at.X, Y: at.Y + 2, Width: 4, Height: 4}, words, false)
		}
	}
	painter := labelledPainter(t, paint)
	words = "second"
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []gfx.ID{gfx.Item(1), gfx.Item(64), gfx.Item(controls)} {
		if got := accessibilityLabel(t, painter, id); got != "second" {
			t.Fatalf("control %v label = %q", id, got)
		}
	}
}
