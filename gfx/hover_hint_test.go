package gfx

import (
	"fmt"
	"testing"
)

func TestHoverHintFollowsReplayAndRewind(t *testing.T) {
	var canvas Canvas
	owner := canvas.Tree().Feature("page")
	canvas.KeepPrevious(true)
	canvas.Reset()
	canvas.BeginFrame()
	start := canvas.Mark()
	canvas.HitWithLabel(Item(1), Rect{Width: 100, Height: 40}, "row")
	canvas.SetHoverHintSide(2)
	canvas.HoverHint(owner, Rect{X: 10, Y: 10, Width: 60, Height: 20}, "Complete filename.txt")
	span := canvas.SpanSince(start)
	canvas.EndFrame()

	canvas.Reset()
	canvas.BeginFrame()
	canvas.HitWithLabel(Item(2), Rect{X: 200, Width: 30, Height: 30}, "other")
	canvas.HoverHint(owner, Rect{X: 200, Width: 30, Height: 30}, "Other complete filename.txt")
	if hint := canvas.HoverHintAt(owner, 210, 10); hint.Side != 0 {
		t.Fatalf("Reset kept the side of the last frame: %+v", hint)
	}
	mark := canvas.Mark()
	if _, ok := canvas.ReplayAt(span, 0, 50); !ok {
		t.Fatal("the unchanged row could not be replayed")
	}
	if hint := canvas.HoverHintAt(owner, 20, 65); hint.Text != "Complete filename.txt" || hint.Bounds.Y != 60 || hint.Side != 2 {
		t.Fatalf("replayed row lost its tooltip or its side: %+v", hint)
	}
	if hint := canvas.HoverHintAt(owner, 20, 15); hint.Text != "" {
		t.Fatal("the tooltip stayed at the row's old position")
	}
	if !canvas.Rewind(mark) || canvas.HoverHintAt(owner, 20, 65).Text != "" {
		t.Fatal("rewinding the row left its tooltip behind")
	}
	canvas.EndFrame()
}

func TestHoverHintIsMountedAndClippedWithEmbeddedText(t *testing.T) {
	var child, page Canvas
	owner := child.Tree().Feature("page")
	child.SetHoverHintSide(3)
	child.HoverHint(owner, Rect{Width: 80, Height: 20}, "Complete filename.txt", Item(1))
	child.HitWithLabel(Item(1), Rect{Width: 80, Height: 20}, "row")
	mount := testMount(&page, 1)
	page.Clip(Rect{X: 100, Y: 100, Width: 40, Height: 50})
	page.Embed(&child, Point{X: 100, Y: 100}, mount)
	page.Unclip()
	mounted := page.Tree().Mount(mount, owner)
	if hint := page.HoverHintAt(mounted, 110, 110); hint.Text != "Complete filename.txt" || hint.Bounds != (Rect{X: 100, Y: 100, Width: 40, Height: 20}) || hint.Side != 3 {
		t.Fatalf("embedded text lost its full value, clip or side: %+v", hint)
	}
	if page.HoverHintAt(mounted, 150, 110).Text != "" || page.HoverHintAt(owner, 110, 110).Text != "" {
		t.Fatal("the embedded tooltip escaped its clip or page owner")
	}
	page.Block(Rect{X: 100, Y: 100, Width: 40, Height: 20})
	if page.HoverHintAt(mounted, 110, 110).Text != "" {
		t.Fatal("covered text still showed its tooltip")
	}
}

// BenchmarkHoverHintAt is a page asking for the tooltip under the pointer once
// a frame: a table whose every row is a hit holding shortened text. On the first
// row's text the query passes every hint and every hit painted after the row;
// beside the text it passes every hint alone; with the tooltips off the page
// declares no hints at all.
func BenchmarkHoverHintAt(b *testing.B) {
	for _, rows := range []int{100, 1000, 10000} {
		var canvas Canvas
		owner := canvas.Tree().Feature("page")
		canvas.Reset()
		canvas.BeginFrame()
		for index := range rows {
			y := float32(index) * 24
			canvas.HitWithLabel(Item(index+1), Rect{Y: y, Width: 400, Height: 24}, "row")
			canvas.HoverHint(owner, Rect{X: 8, Y: y + 4, Width: 160, Height: 16}, "Complete filename.txt")
		}
		var off Canvas
		offOwner := off.Tree().Feature("page")
		off.Reset()
		off.BeginFrame()
		for index := range rows {
			off.HitWithLabel(Item(index+1), Rect{Y: float32(index) * 24, Width: 400, Height: 24}, "row")
		}
		for _, run := range []struct {
			name   string
			canvas *Canvas
			owner  Scope
			x, y   float32
			want   string
		}{
			{"on the first row's text", &canvas, owner, 20, 10, "Complete filename.txt"},
			{"beside the text", &canvas, owner, 300, 10, ""},
			{"tooltips off", &off, offOwner, 20, 10, ""},
		} {
			b.Run(fmt.Sprintf("rows=%d/%s", rows, run.name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if hint := run.canvas.HoverHintAt(run.owner, run.x, run.y); hint.Text != run.want {
						b.Fatalf("hint %q, want %q", hint.Text, run.want)
					}
				}
			})
		}
	}
}
