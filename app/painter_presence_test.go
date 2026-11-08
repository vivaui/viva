package app

import (
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

// A leaving overlay keeps moving past the window edge. Its labels must take the
// same negative position as its panel; clamping them to zero leaves the words
// behind on the page the overlay is uncovering.
func TestPresenceExitPublishesLabelsPastTheWindowEdge(t *testing.T) {
	var canvas ui.Canvas
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	show := true
	config := Config{Title: "presence", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		if !show {
			return
		}
		canvas.BeginPresence(ui.Item(1), 0, ui.Presence{
			Motion:   ui.Motion{Duration: 100 * time.Millisecond, Easing: ui.EaseLinear},
			SlideIn:  ui.Point{X: -101},
			SlideOut: ui.Point{X: -101},
		}, ui.Point{})
		canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 100, Height: 100}, ui.Color{Alpha: 1})
		canvas.Label("drawer", ui.Rect{X: 10, Y: 10, Width: 60, Height: 20}, ui.TextStyle{Size: 12})
		canvas.EndPresence()
	}
	normalized, err := config.normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainterWithClock(&canvas, &normalized, func() time.Duration { return now })
	if err != nil {
		t.Fatal(err)
	}

	now += 200 * time.Millisecond
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	startPanelX := painter.clipped[0].Bounds.X
	startLabelX := painter.labels[0].X
	show = false
	now += 16 * time.Millisecond
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	now += 50 * time.Millisecond
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 1 {
		t.Fatalf("leaving label count = %d, want 1", len(frame.TextLabels))
	}
	if frame.TextLabels[0].OriginX != 0 || frame.TextLabels[0].OriginY != 0 {
		t.Fatalf("moving label kept sub-pixel origin %v,%v", frame.TextLabels[0].OriginX, frame.TextLabels[0].OriginY)
	}
	movedPanelX, movedLabelX := painter.clipped[0].Bounds.X, frame.TextLabels[0].X
	if movedPanelX >= startPanelX || movedLabelX >= startLabelX {
		t.Fatalf("leaving panel %v->%v and label %v->%v did not move left", startPanelX, movedPanelX, startLabelX, movedLabelX)
	}
	if delta := (movedLabelX - startLabelX) - (movedPanelX - startPanelX); delta < -1 || delta > 1 {
		t.Fatalf("label moved %v while its panel moved %v", movedLabelX-startLabelX, movedPanelX-startPanelX)
	}

	now += 100 * time.Millisecond
	frame, err = painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 0 {
		t.Fatalf("settled frame still published leaving labels: %+v", frame.TextLabels)
	}
}
