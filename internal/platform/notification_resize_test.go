package platform

import "testing"

func TestResizeNotificationFrame(t *testing.T) {
	work := ScreenRect{X: 0, Y: 0, Width: 1920, Height: 1040}

	t.Run("bottom-right collapse and expand stay pinned", func(t *testing.T) {
		expanded := ScreenRect{X: 1920 - 12 - 368, Y: 1040 - 12 - 92, Width: 368, Height: 92}
		collapsed := ResizeNotificationFrame(expanded, work, 64, 48)
		if wantX, wantY := float32(1920-12-64), float32(1040-12-48); collapsed.X != wantX || collapsed.Y != wantY || collapsed.Width != 64 || collapsed.Height != 48 {
			t.Fatalf("collapsed = %+v, want origin (%v,%v) size 64x48", collapsed, wantX, wantY)
		}
		restored := ResizeNotificationFrame(collapsed, work, 368, 92)
		if restored != expanded {
			t.Fatalf("restored = %+v, want %+v", restored, expanded)
		}
	})

	t.Run("top-left stays pinned to top-left", func(t *testing.T) {
		current := ScreenRect{X: 12, Y: 12, Width: 368, Height: 92}
		collapsed := ResizeNotificationFrame(current, work, 64, 48)
		if collapsed.X != 12 || collapsed.Y != 12 {
			t.Fatalf("top-left collapsed = %+v, want origin (12,12)", collapsed)
		}
	})

	t.Run("negative monitor work area clamps cleanly", func(t *testing.T) {
		leftWork := ScreenRect{X: -1920, Y: 0, Width: 1920, Height: 1080}
		current := ScreenRect{X: -12 - 64, Y: 1080 - 12 - 48, Width: 64, Height: 48}
		expanded := ResizeNotificationFrame(current, leftWork, 368, 92)
		if wantX, wantY := float32(-12-368), float32(1080-12-92); expanded.X != wantX || expanded.Y != wantY {
			t.Fatalf("left monitor expanded = %+v, want origin (%v,%v)", expanded, wantX, wantY)
		}
	})
}
