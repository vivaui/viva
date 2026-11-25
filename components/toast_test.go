package components

import (
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestToastPaintsOnlyCallerSnapshot(t *testing.T) {
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 1000, Height: 700}
	layout := PaintToast(&canvas, gfx.LightTheme(), viewport, ToastSnapshot{
		Title: "Deployment complete", Description: "Production is now up to date.", Type: ToastSuccess, Index: 1, Expanded: true,
	}, ToastOptions{Layer: 12, Scope: gfx.Item(63), DismissID: gfx.Item(7), ActionID: gfx.Item(8), ActionLabel: "View"})
	if layout.Bounds.Width <= 0 || layout.Bounds.Width > toastMaxWidth ||
		layout.Bounds.X+layout.Bounds.Width != viewport.Width-toastInset {
		t.Fatalf("toast bounds = %+v", layout.Bounds)
	}
	if canvas.EscapeTarget() != gfx.Item(7) || !canvas.TrappedFocusScope().IsZero() {
		t.Fatalf("toast escape=%d trap=%d", canvas.EscapeTarget(), canvas.TrappedFocusScope())
	}
	if len(canvas.Hits()) != 2 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	if action := canvas.Hits()[0]; action.ID != gfx.Item(8) || action.Role != gfx.RoleButton || action.Scope != gfx.Item(63) {
		t.Fatalf("action = %+v", action)
	}
	if close := canvas.Hits()[1]; close.ID != gfx.Item(7) || close.Scope != gfx.Item(63) {
		t.Fatalf("close = %+v", close)
	}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 12 {
			t.Fatalf("rectangle layer = %+v", rectangle)
		}
	}
	for _, text := range canvas.Texts() {
		if text.Layer != 12 {
			t.Fatalf("text layer = %+v", text)
		}
	}

	canvas.Reset()
	if hidden := PaintToast(&canvas, gfx.LightTheme(), gfx.Rect{Width: 1000, Height: 700}, ToastSnapshot{Limited: true}, ToastOptions{Layer: 12}); hidden != (ToastLayout{}) || len(canvas.Rectangles()) != 0 {
		t.Fatalf("limited toast painted: layout=%+v rectangles=%+v", hidden, canvas.Rectangles())
	}
}

func TestToastPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		PaintToast(&canvas, gfx.DarkTheme(), gfx.Rect{X: 2, Y: 3, Width: 800, Height: 600}, ToastSnapshot{
			Title: "Saving", Description: "Your changes are being written.", Type: ToastLoading, Index: 2, OffsetY: -4,
		}, ToastOptions{Layer: 10, Scope: gfx.Item(11), DismissID: gfx.Item(1), ActionID: gfx.Item(2), ActionLabel: "Cancel"})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("toast stream changed")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("toast paint allocations = %v", allocations)
	}
}

func TestToastDescriptionWrapsAndTheToastGrows(t *testing.T) {
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 1000, Height: 700}
	paint := func(description string) ToastLayout {
		return PaintToast(&canvas, gfx.LightTheme(), viewport, ToastSnapshot{Title: "Upload failed", Description: description}, ToastOptions{Layer: 3, DismissID: gfx.Item(1)})
	}
	short := paint("Try again.")
	long := paint("The server closed the connection before the whole file had been written, so nothing was kept on the remote side.")
	if long.Bounds.Height <= short.Bounds.Height || long.Bounds.Y+long.Bounds.Height != short.Bounds.Y+short.Bounds.Height {
		t.Fatalf("short %+v, long %+v: want a taller toast that keeps its bottom edge", short.Bounds, long.Bounds)
	}
	if short.Bounds.Width >= long.Bounds.Width {
		t.Fatalf("short %+v, long %+v: want the width to follow the text", short.Bounds, long.Bounds)
	}
}

func TestToastPositionPicksTheCornerAndLoadingAsksForFramesOnlyWhileShown(t *testing.T) {
	viewport := gfx.Rect{Width: 1000, Height: 700}
	var canvas gfx.Canvas
	topLeft := PaintToast(&canvas, gfx.LightTheme(), viewport, ToastSnapshot{Title: "Hi"}, ToastOptions{Layer: 1, Position: ToastTopLeft})
	center := PaintToast(&canvas, gfx.LightTheme(), viewport, ToastSnapshot{Title: "Hi"}, ToastOptions{Layer: 1, Position: ToastBottomCenter})
	if topLeft.Bounds.X != 16 || topLeft.Bounds.Y != 16 || center.Bounds.X+center.Bounds.Width/2 != 500 {
		t.Fatalf("top left %+v, bottom center %+v", topLeft.Bounds, center.Bounds)
	}
	if _, wake := canvas.NextFrame(); wake {
		t.Fatal("toasts without a spinner asked for a frame")
	}
	PaintToast(&canvas, gfx.LightTheme(), viewport, ToastSnapshot{Title: "Saving", Type: ToastLoading}, ToastOptions{Layer: 1})
	if _, wake := canvas.NextFrame(); !wake {
		t.Fatal("a loading toast did not ask for the next spinner step")
	}
	canvas = gfx.Canvas{}
	PaintToast(&canvas, gfx.LightTheme(), viewport, ToastSnapshot{Title: "Saving", Type: ToastLoading, Limited: true}, ToastOptions{Layer: 1})
	if _, wake := canvas.NextFrame(); wake {
		t.Fatal("a toast that is not shown kept the frame loop running")
	}
}

// A toast with no dismiss ID paints no close button and does not keep its room.
func TestToastWithoutDismissIDHasNoCloseButton(t *testing.T) {
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 1000, Height: 700}
	snapshot := ToastSnapshot{Title: "Copied", Type: ToastSuccess}
	withClose := PaintToast(&canvas, gfx.LightTheme(), viewport, snapshot, ToastOptions{Layer: 1, DismissID: gfx.Item(1)})
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].Label != "Close toast" {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	canvas.Reset()
	withoutClose := PaintToast(&canvas, gfx.LightTheme(), viewport, snapshot, ToastOptions{Layer: 1})
	if len(canvas.Hits()) != 0 {
		t.Fatalf("a toast without a dismiss ID has hits: %+v", canvas.Hits())
	}
	if withoutClose.Bounds.Width >= withClose.Bounds.Width {
		t.Fatalf("with close %+v, without %+v: want the close room gone", withClose.Bounds, withoutClose.Bounds)
	}
}

// The queue's default timeout is one policy; AddFor lets one toast carry its
// own, and a loading toast still never expires.
func TestToastQueueAddForUsesTheToastsOwnTimeout(t *testing.T) {
	now := time.Unix(0, 0)
	var queue ToastQueue
	if evicted, ok := queue.AddFor(gfx.Item(1), now, false, 2*time.Second); !ok || !evicted.IsZero() {
		t.Fatalf("AddFor = (%v, %v)", evicted, ok)
	}
	if evicted, ok := queue.Add(gfx.Item(2), now, false); !ok || !evicted.IsZero() {
		t.Fatalf("Add = (%v, %v)", evicted, ok)
	}
	if _, expired := queue.Expire(now.Add(1999 * time.Millisecond)); expired {
		t.Fatal("a two-second toast expired early")
	}
	if id, expired := queue.Expire(now.Add(2 * time.Second)); !expired || id != gfx.Item(1) {
		t.Fatalf("first expiry = (%v, %v), want item 1", id, expired)
	}
	if _, expired := queue.Expire(now.Add(4 * time.Second)); expired {
		t.Fatal("the default five-second toast expired early")
	}
	if id, expired := queue.Expire(now.Add(5 * time.Second)); !expired || id != gfx.Item(2) {
		t.Fatalf("second expiry = (%v, %v), want item 2", id, expired)
	}
	if _, ok := queue.AddFor(gfx.Item(3), now, true, time.Second); !ok {
		t.Fatal("a loading toast did not add")
	}
	if _, expired := queue.Expire(now.Add(time.Hour)); expired {
		t.Fatal("a loading toast expired")
	}
}

// The queue carries Base UI's lifetime: five seconds, a visible limit, loading
// toasts that never expire, and pause/resume without losing the remaining time.
func TestToastQueueExpiresPausesAndLimits(t *testing.T) {
	now := time.Unix(0, 0)
	var queue ToastQueue
	for index := 1; index <= 3; index++ {
		if evicted, ok := queue.Add(gfx.Item(index), now, false); !ok || !evicted.IsZero() {
			t.Fatalf("add %d = (%v, %v)", index, evicted, ok)
		}
	}
	if evicted, ok := queue.Add(gfx.Item(4), now, false); !ok || evicted != gfx.Item(1) || queue.Len() != 3 {
		t.Fatalf("limit add = (%v, %v), len %d", evicted, ok, queue.Len())
	}
	if id, expired := queue.Expire(now.Add(ToastQueueTimeout - time.Millisecond)); expired {
		t.Fatalf("expired early: %v", id)
	}
	if id, expired := queue.Expire(now.Add(ToastQueueTimeout)); !expired || id != gfx.Item(2) {
		t.Fatalf("first expiry = (%v, %v), want item 2", id, expired)
	}
	if !queue.Pause(gfx.Item(3), now.Add(ToastQueueTimeout)) {
		t.Fatal("pause failed")
	}
	if _, expired := queue.Expire(now.Add(time.Hour)); expired {
		// Item 4 was not paused; only item 3 remains after item 2 expired, so
		// this first call must expire item 4.
	}
	if queue.Len() != 1 {
		t.Fatalf("len after expiry = %d, want the paused toast only", queue.Len())
	}
	if !queue.Resume(gfx.Item(3), now.Add(time.Hour)) {
		t.Fatal("resume failed")
	}
	if id, expired := queue.Expire(now.Add(time.Hour + time.Millisecond)); !expired || id != gfx.Item(3) {
		t.Fatalf("resumed expiry = (%v, %v)", id, expired)
	}
	if _, ok := queue.Add(gfx.Item(5), now, true); !ok {
		t.Fatal("loading add failed")
	}
	if _, expired := queue.Expire(now.Add(time.Hour)); expired {
		t.Fatal("a loading toast expired")
	}
}
