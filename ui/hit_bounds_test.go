package ui

import "testing"

func TestHitBoundsFindsAControlDeclaredEarlierInThePaint(t *testing.T) {
	var canvas Canvas
	trigger := Rect{X: 10, Y: 20, Width: 80, Height: 28}
	canvas.Hit(Item(1), trigger)
	canvas.Hit(Item(2), Rect{X: 200, Y: 20, Width: 80, Height: 28})
	if got, ok := canvas.HitBounds(Item(1)); !ok || got != trigger {
		t.Fatalf("HitBounds = %+v, %v; want %+v", got, ok, trigger)
	}
}

func TestHitBoundsReportsTheLastDeclarationAndWhatIsReachable(t *testing.T) {
	var canvas Canvas
	canvas.Hit(Item(1), Rect{Width: 10, Height: 10})
	canvas.Clip(Rect{X: 20, Y: 30, Width: 50, Height: 40})
	canvas.Hit(Item(1), Rect{X: 0, Y: 0, Width: 40, Height: 50})
	want := Rect{X: 20, Y: 30, Width: 20, Height: 20}
	if got, ok := canvas.HitBounds(Item(1)); !ok || got != want {
		t.Fatalf("HitBounds = %+v, %v; want the clipped last declaration %+v", got, ok, want)
	}
}

func TestHitBoundsHasNothingForWhatThePaintHasNotDeclared(t *testing.T) {
	var canvas Canvas
	canvas.Hit(Item(1), Rect{Width: 10, Height: 10})
	canvas.Container(Rect{Width: 50, Height: 50}, RoleGroup, "region")
	canvas.Block(Rect{Width: 5, Height: 5})
	if _, ok := canvas.HitBounds(Item(2)); ok {
		t.Fatal("a control that was not declared has bounds")
	}
	if _, ok := canvas.HitBounds(ID{}); ok {
		t.Fatal("the zero ID has bounds")
	}
	canvas.Reset()
	if _, ok := canvas.HitBounds(Item(1)); ok {
		t.Fatal("a control of the previous paint has bounds after Reset")
	}
}

func BenchmarkHitBoundsInALargePaint(b *testing.B) {
	var canvas Canvas
	for index := range 4000 {
		canvas.Hit(Item(index+1), Rect{Y: float32(index) * 24, Width: 100, Height: 24})
	}
	first := Item(1)
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := canvas.HitBounds(first); !ok {
			b.Fatal("not found")
		}
	}
}

func TestHitBoundsDoesNotAllocate(t *testing.T) {
	var canvas Canvas
	for index := range 100 {
		canvas.Hit(Item(index+1), Rect{Width: 10, Height: 10})
	}
	allocs := testing.AllocsPerRun(100, func() { canvas.HitBounds(Item(1)) })
	if allocs != 0 {
		t.Fatalf("HitBounds allocates %v times", allocs)
	}
}
