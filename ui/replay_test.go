package ui

import (
	"reflect"
	"testing"
)

// paintPart paints a part with every kind of primitive whose indexes refer across
// lists: a fill, a shadow, a triangle path, text with runs, an image, a hit and a
// native view.
func paintPart(canvas *Canvas, y float32) { paintPartNamed(canvas, y, int(y)+1) }

// paintPartNamed is paintPart with identities that do not follow its place.
func paintPartNamed(canvas *Canvas, y float32, name int) {
	canvas.Elevate(Rect{X: 10, Y: y, Width: 80, Height: 30}, 6, ShadowSM)
	canvas.Rounded(Rect{X: 10, Y: y, Width: 80, Height: 30}, 6, RGB(240, 240, 240))
	canvas.FillLinearGradient(Rect{X: 100, Y: y, Width: 20, Height: 20}, LinearGradient{Start: Point{X: 100, Y: y}, End: Point{X: 120, Y: y}, From: RGB(10, 20, 30), To: RGB(30, 40, 50)})
	canvas.Label("Hello, 世界", Rect{X: 14, Y: y + 4, Width: 72, Height: 20}, TextStyle{Size: 13})
	canvas.LabelRuns("ab", Rect{X: 14, Y: y + 16, Width: 72, Height: 14}, TextStyle{Size: 12, Mono: true}, 7,
		[]TextRun{{Column: 0, Bytes: 1, Cells: 1, Color: RGB(200, 0, 0)}, {Column: 1, Bytes: 1, Cells: 1, Color: RGB(0, 0, 200)}})
	canvas.DrawBitmap(Bitmap{}, Rect{X: 130, Y: y, Width: 16, Height: 16})
	canvas.HitWithOptions(Item(name), Rect{X: 10, Y: y, Width: 80, Height: 30}, "row", HitOptions{Role: RoleButton})
	canvas.NativeView(NativeViewID(name), Rect{X: 150, Y: y, Width: 40, Height: 30})
}

// paintPage paints a part before and after the cached one, so the copy lands at
// different offsets from where it was recorded.
func paintPage(canvas *Canvas, before int, cached func(*Canvas)) {
	canvas.Reset()
	canvas.BeginFrame()
	for i := range before {
		paintPart(canvas, float32(200+40*i))
	}
	canvas.PushLayer(1)
	cached(canvas)
	canvas.PopLayer()
	paintPart(canvas, 400)
	canvas.EndFrame()
}

type paintedLists struct {
	Rectangles  []Rectangle
	Triangles   []Triangle
	Shadows     []Shadow
	Images      []Image
	Texts       []Text
	Runs        []TextRun
	Hits        []Hit
	NativeViews []NativeView
}

func listsOf(canvas *Canvas) paintedLists {
	return paintedLists{
		append([]Rectangle(nil), canvas.Rectangles()...), append([]Triangle(nil), canvas.Triangles()...),
		append([]Shadow(nil), canvas.Shadows()...), append([]Image(nil), canvas.Images()...),
		append([]Text(nil), canvas.Texts()...), append([]TextRun(nil), canvas.AllTextRuns()...),
		append([]Hit(nil), canvas.Hits()...), append([]NativeView(nil), canvas.NativeViews()...),
	}
}

// A replayed part is exactly what painting it again gives, wherever in the paint
// order it lands.
func TestReplayEqualsRepaint(t *testing.T) {
	part := func(canvas *Canvas) { paintPart(canvas, 20) }

	var direct Canvas
	paintPage(&direct, 3, part)
	want := listsOf(&direct)

	var cached Canvas
	cached.KeepPrevious(true)
	var span Span
	paintPage(&cached, 1, func(canvas *Canvas) {
		from := canvas.Mark()
		part(canvas)
		span = canvas.SpanSince(from)
	})
	paintPage(&cached, 3, func(canvas *Canvas) {
		var ok bool
		if span, ok = canvas.Replay(span); !ok {
			t.Fatal("a span of the previous paint could not be replayed")
		}
	})
	if got := listsOf(&cached); !reflect.DeepEqual(got, want) {
		t.Fatalf("replay differs from a repaint:\n got %+v\nwant %+v", got, want)
	}
	// And again from the copy: a part unchanged for many paints is replayed each time.
	paintPage(&cached, 2, func(canvas *Canvas) {
		var ok bool
		if span, ok = canvas.Replay(span); !ok {
			t.Fatal("the copy could not be replayed in the next paint")
		}
	})
	var again Canvas
	paintPage(&again, 2, part)
	if got := listsOf(&cached); !reflect.DeepEqual(got, listsOf(&again)) {
		t.Fatal("a replay of a replay differs from a repaint")
	}
}

func TestReplayOnlyFromThePreviousPaint(t *testing.T) {
	var canvas Canvas
	canvas.KeepPrevious(true)
	var span Span
	paintPage(&canvas, 0, func(c *Canvas) {
		from := c.Mark()
		paintPart(c, 20)
		span = c.SpanSince(from)
	})
	paintPage(&canvas, 0, func(*Canvas) {})
	paintPage(&canvas, 0, func(c *Canvas) {
		if _, ok := c.Replay(span); ok {
			t.Fatal("a span two paints old was replayed")
		}
	})

	var plain Canvas // does not keep the previous paint
	paintPage(&plain, 0, func(c *Canvas) {
		from := c.Mark()
		paintPart(c, 20)
		span = c.SpanSince(from)
	})
	paintPage(&plain, 0, func(c *Canvas) {
		if _, ok := c.Replay(span); ok {
			t.Fatal("a canvas that keeps nothing replayed")
		}
	})
}

func TestReplayIsFreeOfAllocationsOnceWarm(t *testing.T) {
	var canvas Canvas
	canvas.KeepPrevious(true)
	var span Span
	paintPage(&canvas, 2, func(c *Canvas) {
		from := c.Mark()
		for i := range 20 {
			paintPart(c, float32(i*10))
		}
		span = c.SpanSince(from)
	})
	replay := func() {
		paintPage(&canvas, 2, func(c *Canvas) { span, _ = c.Replay(span) })
	}
	replay()
	if allocs := testing.AllocsPerRun(20, replay); allocs != 0 {
		t.Fatalf("a warm replay allocates %v times", allocs)
	}
}

func BenchmarkPaintPart20(b *testing.B) {
	var canvas Canvas
	b.ReportAllocs()
	for b.Loop() {
		paintPage(&canvas, 0, func(c *Canvas) {
			for i := range 20 {
				paintPart(c, float32(i*10))
			}
		})
	}
}

func BenchmarkReplayPart20(b *testing.B) {
	var canvas Canvas
	canvas.KeepPrevious(true)
	var span Span
	paintPage(&canvas, 0, func(c *Canvas) {
		from := c.Mark()
		for i := range 20 {
			paintPart(c, float32(i*10))
		}
		span = c.SpanSince(from)
	})
	b.ReportAllocs()
	for b.Loop() {
		paintPage(&canvas, 0, func(c *Canvas) { span, _ = c.Replay(span) })
	}
}

func TestReplayNeedsTheSameClipAndLayer(t *testing.T) {
	var canvas Canvas
	canvas.KeepPrevious(true)
	var span Span
	paintPage(&canvas, 0, func(c *Canvas) {
		from := c.Mark()
		paintPart(c, 20)
		span = c.SpanSince(from)
	})
	paintPage(&canvas, 0, func(c *Canvas) {
		c.Clip(Rect{Width: 50, Height: 50})
		if c.CanReplay(span) {
			t.Error("a span was replayable under another clip")
		}
		c.Unclip()
		c.PushLayer(5)
		if c.CanReplay(span) {
			t.Error("a span was replayable on another layer")
		}
		c.PopLayer()
		if !c.CanReplay(span) {
			t.Error("a span was not replayable where it was recorded")
		}
	})
}

func TestReplayAtMovesAndRewindTakesBack(t *testing.T) {
	var direct Canvas
	paintPage(&direct, 1, func(c *Canvas) { paintPartNamed(c, 60, 7) })
	want := listsOf(&direct)

	var canvas Canvas
	canvas.KeepPrevious(true)
	var span Span
	paintPage(&canvas, 1, func(c *Canvas) {
		from := c.Mark()
		paintPartNamed(c, 20, 7)
		span = c.SpanSince(from)
	})
	paintPage(&canvas, 1, func(c *Canvas) {
		mark := c.Mark()
		paintPart(c, 999) // a guess that turns out wrong
		if !c.Rewind(mark) {
			t.Fatal("rewind refused a mark of this paint")
		}
		var ok bool
		if span, ok = c.ReplayAt(span, 0, 40); !ok {
			t.Fatal("an unclipped span could not be moved")
		}
	})
	if got := listsOf(&canvas); !reflect.DeepEqual(got, want) {
		t.Fatalf("moved replay differs from painting there:\n got %+v\nwant %+v", got, want)
	}
}

func TestReplayAtRefusesToMoveAcrossAClip(t *testing.T) {
	var canvas Canvas
	canvas.KeepPrevious(true)
	var span Span
	clipped := func(c *Canvas, body func(*Canvas)) {
		c.Clip(Rect{Width: 300, Height: 100})
		body(c)
		c.Unclip()
	}
	paintPage(&canvas, 0, func(c *Canvas) {
		clipped(c, func(c *Canvas) {
			from := c.Mark()
			paintPart(c, 20)
			span = c.SpanSince(from)
		})
	})
	paintPage(&canvas, 0, func(c *Canvas) {
		clipped(c, func(c *Canvas) {
			if _, ok := c.ReplayAt(span, 0, 60); ok {
				t.Error("a part moved partly out of its clip was replayed")
			}
			if _, ok := c.ReplayAt(span, 0, 10); !ok {
				t.Error("a part moved within its clip was not replayed")
			}
		})
	})
}

func TestSameAsPreviousSeesADifference(t *testing.T) {
	var canvas Canvas
	canvas.KeepPrevious(true)
	var prev Span
	paintPage(&canvas, 0, func(c *Canvas) {
		from := c.Mark()
		paintPart(c, 20)
		prev = c.SpanSince(from)
	})
	paintPage(&canvas, 0, func(c *Canvas) {
		from := c.Mark()
		paintPart(c, 20)
		same := c.SpanSince(from)
		if !c.SameAsPrevious(same, prev, 0, 0) {
			t.Error("the same part painted again is reported different")
		}
		from = c.Mark()
		paintPartNamed(c, 21, 21)
		if c.SameAsPrevious(c.SpanSince(from), prev, 0, 0) {
			t.Error("a part painted a pixel lower is reported the same")
		}
		from = c.Mark()
		paintPartNamed(c, 21, 21)
		if !c.SameAsPrevious(c.SpanSince(from), prev, 0, 1) {
			t.Error("a part painted a pixel lower is not the previous one moved by a pixel")
		}
	})
}
