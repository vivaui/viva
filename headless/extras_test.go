//go:build darwin || windows || linux

package headless

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestSessionClockMovesOnlyWhenAsked(t *testing.T) {
	session, target := openTestApp(t)
	if now := session.Now(); now != 0 {
		t.Fatalf("clock starts at %v", now)
	}
	if session.Transitioning() {
		t.Fatal("a still frame reports a transition")
	}
	if err := session.Advance(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if now := session.Now(); now != 100*time.Millisecond {
		t.Fatalf("clock = %v after advancing 100ms", now)
	}
	// A frame can be painted without moving the clock, for state changed from
	// outside the application.
	target.clicks = 3
	if err := session.Frame(); err != nil {
		t.Fatal(err)
	}
	if now := session.Now(); now != 100*time.Millisecond {
		t.Fatalf("Frame moved the clock to %v", now)
	}
	if !session.HasText("Clicks: 3") {
		t.Fatalf("frame did not show the change: %q", session.Texts())
	}
}

func TestSessionAccessReadsTheAccessibilityTree(t *testing.T) {
	session := layoutSession(t, func(canvas *gfx.Canvas) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 16, Y: 16, Width: 120, Height: 32}, "Auto update", gfx.HitOptions{
			Role: gfx.RoleCheckbox, Checked: true, Disabled: true,
			Description: "Applies updates at night", Level: 2, PosInSet: 2, SetSize: 3,
			Live: gfx.LivePolite,
		})
		canvas.HitWithOptions(gfx.Item(2), gfx.Rect{X: 16, Y: 64, Width: 200, Height: 24}, "Volume", gfx.HitOptions{
			Role: gfx.RoleSlider, Range: gfx.ValueRange{Now: 0.4, Min: 0, Max: 1, Set: true},
		})
	})
	nodes := session.Access()
	if len(nodes) != 2 {
		t.Fatalf("accessibility tree = %+v", nodes)
	}
	check, ok := session.AccessByRole(gfx.RoleCheckbox, "Auto update")
	if !ok {
		t.Fatalf("no checkbox: %+v", nodes)
	}
	if !check.Checked || !check.Disabled || check.Description != "Applies updates at night" ||
		check.Level != 2 || check.PosInSet != 2 || check.SetSize != 3 || check.Live != gfx.LivePolite {
		t.Errorf("checkbox = %+v", check)
	}
	if check.ID != gfx.Item(1) || check.Bounds.Width != 120 {
		t.Errorf("checkbox identity or bounds = %+v", check)
	}
	slider, ok := session.AccessByName("Volume")
	if !ok || slider.Role != gfx.RoleSlider || !slider.Range.Set || slider.Range.Now != 0.4 {
		t.Errorf("slider = %+v, %v", slider, ok)
	}
	if _, ok := session.AccessByRole(gfx.RoleButton, ""); ok {
		t.Error("AccessByRole found a button that is not there")
	}
}

func TestSessionRecordsNotifications(t *testing.T) {
	session, _ := openTestApp(t)
	if err := app.SendNotification(app.Notification{Title: "Build", Body: "done"}); err != nil {
		t.Fatal(err)
	}
	if err := app.SendNotification(app.Notification{Title: "Upload", Body: "queued"}); err != nil {
		t.Fatal(err)
	}
	got := session.Notifications()
	if len(got) != 2 || got[0].Title != "Build" || got[1].Body != "queued" {
		t.Fatalf("notifications = %+v", got)
	}
	if err := app.SendNotification(app.Notification{}); err == nil {
		t.Error("an empty notification was accepted")
	}
}
