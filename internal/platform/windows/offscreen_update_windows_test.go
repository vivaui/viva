package windows

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// A renderer that is sent only the store's updates (a range that may grow the
// list, be cut short with Shrink, or a rebuild) draws exactly what a renderer
// given the whole list draws, through batches that delete, insert in the
// middle and recolor shapes together.
func TestOffscreenIncrementalUpdatesDrawWhatARebuildDraws(t *testing.T) {
	incremental := newOffscreen(t, 160, 120, 1)
	random := rand.New(rand.NewSource(3))
	graph := scene.New()
	var store render.Store
	var shapes []scene.ID
	place := func() (scene.Rect, scene.Color) {
		bounds := scene.Rect{X: float32(random.Intn(140)), Y: float32(random.Intn(100)), Width: float32(8 + random.Intn(20)), Height: float32(8 + random.Intn(20))}
		return bounds, scene.Color{Red: random.Float32(), Green: random.Float32(), Blue: random.Float32(), Alpha: 1}
	}
	for range 40 {
		bounds, color := place()
		id, _ := graph.AddRectangle(graph.Root(), bounds, color)
		shapes = append(shapes, id)
	}
	shrinks, grows := 0, 0
	for batch := 0; batch < 120; batch++ {
		if batch > 0 {
			for operations := 1 + random.Intn(6); operations > 0; operations-- {
				switch operation := random.Intn(3); {
				case operation == 0 && len(shapes) > 1:
					at := random.Intn(len(shapes))
					graph.Remove(shapes[at])
					shapes = append(shapes[:at], shapes[at+1:]...)
				case operation == 1:
					at := random.Intn(len(shapes) + 1)
					previous := scene.ID(0)
					if at > 0 {
						previous = shapes[at-1]
					}
					bounds, color := place()
					id, ok := graph.InsertRoundedRectangle(graph.Root(), previous, bounds, color, 0)
					if !ok {
						t.Fatal("insert failed")
					}
					shapes = append(shapes[:at], append([]scene.ID{id}, shapes[at:]...)...)
				case len(shapes) > 0:
					bounds, color := place()
					graph.SetRectangle(shapes[random.Intn(len(shapes))], bounds, color)
				}
			}
		}
		update, err := store.Apply(graph.DrainChanges(nil))
		if err != nil {
			t.Fatalf("batch %d: %v", batch, err)
		}
		rectangles := store.Rectangles()
		if !update.Rebuild && update.Shrink {
			shrinks++
		}
		if !update.Rebuild && !update.Shrink && update.First+update.Count > len(incremental.renderer.instances) {
			grows++
		}
		if err := incremental.renderer.apply(platform.Frame{Update: update, Rectangles: rectangles}); err != nil {
			t.Fatalf("batch %d: update %+v: %v", batch, update, err)
		}
		got, err := incremental.Image()
		if err != nil {
			t.Fatal(err)
		}
		want := renderScene(t, newOffscreen(t, 160, 120, 1), append([]render.Rectangle(nil), rectangles...), nil)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("batch %d: update %+v drew other pixels than a rebuild of %d instances", batch, update, len(rectangles))
		}
	}
	if shrinks == 0 || grows == 0 {
		t.Fatalf("the batches sent %d shrinking and %d growing ranges; the test needs both", shrinks, grows)
	}
}
