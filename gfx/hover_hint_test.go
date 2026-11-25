package gfx

import "testing"

func TestHoverHintFollowsReplayAndRewind(t *testing.T) {
	var canvas Canvas
	owner := canvas.Tree().Feature("page")
	canvas.KeepPrevious(true)
	canvas.Reset()
	canvas.BeginFrame()
	start := canvas.Mark()
	canvas.HitWithLabel(Item(1), Rect{Width: 100, Height: 40}, "row")
	canvas.HoverHint(owner, Rect{X: 10, Y: 10, Width: 60, Height: 20}, "Complete filename.txt")
	span := canvas.SpanSince(start)
	canvas.EndFrame()

	canvas.Reset()
	canvas.BeginFrame()
	canvas.HitWithLabel(Item(2), Rect{X: 200, Width: 30, Height: 30}, "other")
	mark := canvas.Mark()
	if _, ok := canvas.ReplayAt(span, 0, 50); !ok {
		t.Fatal("the unchanged row could not be replayed")
	}
	if hint := canvas.HoverHintAt(owner, 20, 65); hint.Text != "Complete filename.txt" || hint.Bounds.Y != 60 {
		t.Fatalf("replayed row lost its tooltip: %+v", hint)
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
	child.HoverHint(owner, Rect{Width: 80, Height: 20}, "Complete filename.txt", Item(1))
	child.HitWithLabel(Item(1), Rect{Width: 80, Height: 20}, "row")
	mount := testMount(&page, 1)
	page.Clip(Rect{X: 100, Y: 100, Width: 40, Height: 50})
	page.Embed(&child, Point{X: 100, Y: 100}, mount)
	page.Unclip()
	mounted := page.Tree().Mount(mount, owner)
	if hint := page.HoverHintAt(mounted, 110, 110); hint.Text != "Complete filename.txt" || hint.Bounds != (Rect{X: 100, Y: 100, Width: 40, Height: 20}) {
		t.Fatalf("embedded text lost its full value or clip: %+v", hint)
	}
	if page.HoverHintAt(mounted, 150, 110).Text != "" || page.HoverHintAt(owner, 110, 110).Text != "" {
		t.Fatal("the embedded tooltip escaped its clip or page owner")
	}
	page.Block(Rect{X: 100, Y: 100, Width: 40, Height: 20})
	if page.HoverHintAt(mounted, 110, 110).Text != "" {
		t.Fatal("covered text still showed its tooltip")
	}
}
