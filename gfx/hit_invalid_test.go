package gfx

import "testing"

// A hit carries the semantic invalid state a control declared, so a runtime can
// pass it to assistive technology beside Disabled and ReadOnly.
func TestHitCarriesInvalid(t *testing.T) {
	var canvas Canvas
	canvas.Reset()
	canvas.HitWithOptions(Item(1), Rect{Width: 10, Height: 10}, "Name", HitOptions{Invalid: true})
	canvas.HitWithOptions(Item(2), Rect{X: 10, Width: 10, Height: 10}, "Other", HitOptions{})
	hits := canvas.Hits()
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if !hits[0].Invalid {
		t.Fatal("a hit declared Invalid lost it")
	}
	if hits[1].Invalid {
		t.Fatal("a hit not declared Invalid became invalid")
	}
}
