package app

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A focus change the paint itself causes, by a newly appearing Autofocus hit, a
// RequestFocus, a ReleaseFocus or a ReturnFocus scope closing, must show in the
// frame that paint publishes: the runtime paints again with the new focus before
// publishing, instead of leaving the old focus on screen until another event.
func TestPaintTimeFocusChangesShowInTheSameFrame(t *testing.T) {
	const field, other, trigger, inside = 10, 11, 12, 13
	for _, test := range []struct {
		name   string
		before func(canvas *gfx.Canvas)
		after  func(canvas *gfx.Canvas)
		want   gfx.ID
	}{
		{
			name: "autofocus",
			after: func(canvas *gfx.Canvas) {
				canvas.HitWithOptions(gfx.Item(field), gfx.Rect{Y: 30, Width: 80, Height: 20}, "Field", gfx.HitOptions{Role: gfx.RoleTextField, Autofocus: true})
			},
			want: gfx.Item(field),
		},
		{
			name: "request",
			after: func(canvas *gfx.Canvas) {
				canvas.HitWithOptions(gfx.Item(field), gfx.Rect{Y: 30, Width: 80, Height: 20}, "Field", gfx.HitOptions{Role: gfx.RoleTextField})
				canvas.RequestFocus(gfx.Item(field))
			},
			want: gfx.Item(field),
		},
		{
			name:   "release",
			before: func(canvas *gfx.Canvas) { canvas.RequestFocus(gfx.Item(other)) },
			after:  func(canvas *gfx.Canvas) { canvas.ReleaseFocus(gfx.Item(other)) },
			want:   gfx.ID{},
		},
		{
			name: "return",
			before: func(canvas *gfx.Canvas) {
				canvas.PushFocusScope(gfx.Item(100))
				canvas.ReturnFocus(gfx.Item(100))
				canvas.HitWithOptions(gfx.Item(inside), gfx.Rect{Y: 60, Width: 80, Height: 20}, "Inside", gfx.HitOptions{Role: gfx.RoleButton})
				canvas.PopFocusScope()
				canvas.RequestFocus(gfx.Item(inside))
			},
			after: func(*gfx.Canvas) {}, // the scope closes
			want:  gfx.Item(trigger),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			stage, paints := 0, 0
			config := Config{Title: "converge", Width: 200, Height: 100}
			config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
				paints++
				canvas.HitWithOptions(gfx.Item(other), gfx.Rect{Width: 80, Height: 20}, "Other", gfx.HitOptions{Role: gfx.RoleButton})
				canvas.HitWithOptions(gfx.Item(trigger), gfx.Rect{X: 100, Width: 80, Height: 20}, "Trigger", gfx.HitOptions{Role: gfx.RoleButton})
				switch {
				case stage == 1 && test.before != nil:
					test.before(canvas)
				case stage == 2:
					test.after(canvas)
				}
				// What the frame shows depends on the focus the paint was told.
				canvas.Label(fmt.Sprint(canvas.Focused()), gfx.Rect{Y: 80, Width: 200, Height: 20}, gfx.TextStyle{Size: 12})
			}
			painter := newTestPainter(t, &canvas, config)
			if test.name == "return" {
				painter.focus.Apply(gfx.EventResult{Focus: gfx.Item(trigger)})
			}
			stage = 1
			if _, err := painter.resize(size(201, 100)); err != nil {
				t.Fatal(err)
			}
			stage, paints = 2, 0
			if _, err := painter.resize(size(202, 100)); err != nil {
				t.Fatal(err)
			}
			if painter.focus.Focused() != test.want {
				t.Fatalf("focus is %v, want %v", painter.focus.Focused(), test.want)
			}
			labels := painter.frame.Labels()
			if shown := labels[len(labels)-1].Value; shown != fmt.Sprint(test.want) {
				t.Fatalf("the published frame shows focus %q, want %q", shown, fmt.Sprint(test.want))
			}
			if paints > 3 {
				t.Fatalf("one repaint painted %d times", paints)
			}
		})
	}
}
