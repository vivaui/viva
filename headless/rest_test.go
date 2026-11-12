//go:build darwin || windows || linux

package headless

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// A dialog's entrance plays on the animation clock; Rest plays it out, so a
// screenshot after an input shows the interface at rest, and Wait steps it.
func TestRestPlaysTransitionsToTheirEnd(t *testing.T) {
	var canvas gfx.Canvas
	session, err := Open(app.Config{
		Title: "rest", Width: 200, Height: 100,
		Paint: func(c *gfx.Canvas, size gfx.Size) {
			c.BeginPresence(gfx.Item(1), 0, gfx.Presence{Motion: gfx.Motion{Duration: 100 * time.Millisecond}, Fade: true}, gfx.Point{})
			c.Fill(gfx.Rect{Width: 50, Height: 50}, gfx.Color{Alpha: 1})
			c.EndPresence()
		},
	}, &canvas, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	alpha := func() float32 { return canvas.Rectangles()[0].Color.Alpha }
	if alpha() != 0 || !session.Driver().Transitioning() {
		t.Fatalf("opening frame: alpha %v transitioning %v", alpha(), session.Driver().Transitioning())
	}
	if err := session.Wait(50 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if a := alpha(); a <= 0 || a >= 1 {
		t.Fatalf("halfway alpha = %v", a)
	}
	if err := session.Rest(); err != nil {
		t.Fatal(err)
	}
	if alpha() != 1 || session.Driver().Transitioning() {
		t.Fatalf("at rest: alpha %v transitioning %v", alpha(), session.Driver().Transitioning())
	}
}
