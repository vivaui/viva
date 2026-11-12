//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func distinctTextFrame(t testing.TB, distinct int) (*Session, *app.Invalidator, *gfx.Canvas, []string) {
	t.Helper()
	if app.PlatformTextEngine() == nil {
		t.Skip("platform text engine unavailable")
	}
	lines := make([]string, distinct)
	for index := range lines {
		lines[index] = fmt.Sprintf("row %04d 主机", index)
	}
	invalidator := &app.Invalidator{}
	var canvas *gfx.Canvas
	session, err := Open(app.Config{
		Title: "Distinct text", Width: 480, Height: 400, Invalidator: invalidator,
		Paint: func(target *gfx.Canvas, size gfx.Size) {
			canvas = target
			for _, line := range lines {
				target.TextWidth(line, 14)
			}
			for index, line := range lines[:min(len(lines), 16)] {
				target.Label(line, gfx.Rect{X: 16, Y: float32(index) * 24, Width: size.Width - 32, Height: 24}, gfx.TextStyle{Size: 14})
			}
		},
	}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	for frame := 0; frame < 20; frame++ {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session, invalidator, canvas, lines
}

func TestHeadlessDistinctTextStaysWarm(t *testing.T) {
	session, _, canvas, lines := distinctTextFrame(t, 1000)
	if !session.HasText(lines[0]) || !session.HasText(lines[15]) {
		t.Fatal("the distinct-text page did not paint its visible rows")
	}
	if allocs := testing.AllocsPerRun(20, func() {
		for _, line := range lines {
			canvas.TextWidth(line, 14)
		}
	}); allocs != 0 {
		t.Fatalf("warm distinct-text measurement allocated %v times", allocs)
	}
}

func BenchmarkDistinctTextFrames(b *testing.B) {
	for _, distinct := range []int{512, 600, 1000, 1500, 5000} {
		b.Run(fmt.Sprint(distinct), func(b *testing.B) {
			session, invalidator, _, _ := distinctTextFrame(b, distinct)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				invalidator.Invalidate()
				if err := session.Driver().Pump(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
