package gfx

import "testing"

func rectsEqual(a, b Rect) bool { return a == b }

func TestRectCutsPartitionTheOriginal(t *testing.T) {
	whole := Rect{X: 10, Y: 20, Width: 300, Height: 200}
	for _, amount := range []float32{-5, 0, 1, 48, 199, 200, 5000} {
		top, below := whole.CutTop(amount)
		bottom, above := whole.CutBottom(amount)
		left, right := whole.CutLeft(amount)
		rightStrip, rest := whole.CutRight(amount)

		if top.Height+below.Height != whole.Height || top.Bottom() != below.Y || below.Bottom() != whole.Bottom() {
			t.Errorf("CutTop(%v) = %+v | %+v", amount, top, below)
		}
		if bottom.Height+above.Height != whole.Height || above.Bottom() != bottom.Y || bottom.Bottom() != whole.Bottom() {
			t.Errorf("CutBottom(%v) = %+v | %+v", amount, bottom, above)
		}
		if left.Width+right.Width != whole.Width || left.Right() != right.X || right.Right() != whole.Right() {
			t.Errorf("CutLeft(%v) = %+v | %+v", amount, left, right)
		}
		if rightStrip.Width+rest.Width != whole.Width || rest.Right() != rightStrip.X || rightStrip.Right() != whole.Right() {
			t.Errorf("CutRight(%v) = %+v | %+v", amount, rightStrip, rest)
		}
		for _, piece := range []Rect{top, below, bottom, above, left, right, rightStrip, rest} {
			if piece.Width < 0 || piece.Height < 0 {
				t.Errorf("amount %v produced a negative size: %+v", amount, piece)
			}
		}
	}
}

func TestRectCutKnownValues(t *testing.T) {
	r := Rect{X: 0, Y: 0, Width: 100, Height: 60}
	header, body := r.CutTop(20)
	if !rectsEqual(header, Rect{Width: 100, Height: 20}) || !rectsEqual(body, Rect{Y: 20, Width: 100, Height: 40}) {
		t.Fatalf("CutTop = %+v | %+v", header, body)
	}
	footer, rest := r.CutBottom(10)
	if !rectsEqual(footer, Rect{Y: 50, Width: 100, Height: 10}) || !rectsEqual(rest, Rect{Width: 100, Height: 50}) {
		t.Fatalf("CutBottom = %+v | %+v", footer, rest)
	}
	trailing, leading := r.CutRight(30)
	if !rectsEqual(trailing, Rect{X: 70, Width: 30, Height: 60}) || !rectsEqual(leading, Rect{Width: 70, Height: 60}) {
		t.Fatalf("CutRight = %+v | %+v", trailing, leading)
	}
	if got := r.Centered(40, 20); !rectsEqual(got, Rect{X: 30, Y: 20, Width: 40, Height: 20}) {
		t.Fatalf("Centered = %+v", got)
	}
	if got := r.Centered(120, 60); got.X != -10 {
		t.Fatalf("an oversized Centered overflows evenly, got X=%v", got.X)
	}
	if got := r.Offset(5, -3); !rectsEqual(got, Rect{X: 5, Y: -3, Width: 100, Height: 60}) {
		t.Fatalf("Offset = %+v", got)
	}
}

func BenchmarkRectCuts(b *testing.B) {
	bounds := Rect{Width: 800, Height: 600}
	b.ReportAllocs()
	var sink float32
	for i := 0; i < b.N; i++ {
		header, body := bounds.CutTop(48)
		sidebar, content := body.CutLeft(240)
		footer, content := content.CutBottom(32)
		sink += header.Height + sidebar.Width + footer.Height + content.Width
	}
	_ = sink
}
