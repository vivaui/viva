package interaction

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
)

func TestFocusRegionsSkipContainersAndOrderByLayer(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginFrame()
	canvas.Hit(gfx.Item(1), gfx.Rect{Width: 40, Height: 40})
	canvas.Container(gfx.Rect{X: 10, Y: 10, Width: 40, Height: 40}, gfx.RoleGroup, "")
	canvas.PushLayer(2)
	canvas.Hit(gfx.Item(3), gfx.Rect{X: 20, Y: 20, Width: 40, Height: 40})
	canvas.PopLayer()
	canvas.EndFrame()
	var focus Focus
	focus.Sync(&canvas)
	regions := focus.Router().Regions()
	if len(regions) != 2 || regions[0].Target != gfx.Item(1) || regions[1].Target != gfx.Item(3) {
		t.Fatalf("regions = %+v, want item 1 then item 3 on the higher layer", regions)
	}
	if target, ok := focus.Router().HitTest(25, 25); !ok || target != gfx.Item(3) {
		t.Fatalf("front-most hit = %v/%v, want item 3", target, ok)
	}
}

func TestFocusKeepsAFocusedTargetAcrossRepaints(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginFrame()
	canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 40, Height: 40}, "", gfx.HitOptions{Role: gfx.RoleButton})
	canvas.EndFrame()
	var focus Focus
	focus.Sync(&canvas)
	if !focus.Apply(gfx.EventResult{Focus: gfx.Item(1), FocusVisible: true}) {
		t.Fatal("focusing an existing hit should report a change")
	}
	if focus.Focused() != gfx.Item(1) || !focus.KeyboardFocus() {
		t.Fatalf("focus = %v/%v, want item 1 with a ring", focus.Focused(), focus.KeyboardFocus())
	}
	focus.Sync(&canvas)
	if focus.Focused() != gfx.Item(1) || !focus.KeyboardFocus() {
		t.Fatalf("repaint lost focus: %v/%v", focus.Focused(), focus.KeyboardFocus())
	}
}

func TestPendingFocusUsesTheRevealedPaint(t *testing.T) {
	var canvas gfx.Canvas
	var focus Focus
	focus.Sync(&canvas)
	if !focus.Apply(gfx.EventResult{Focus: gfx.Item(7), FocusVisible: true}) {
		t.Fatal("focusing a not-yet-painted hit should be pending")
	}
	if focus.PendingFocus() != gfx.Item(7) {
		t.Fatalf("pending focus = %v", focus.PendingFocus())
	}
	if id, visible := focus.PaintTarget(); id != gfx.Item(7) || !visible {
		t.Fatalf("paint target = %v/%v, want the pending item", id, visible)
	}
	// The next paint reveals the target because it was allowed to lay out
	// around the pending focus rather than the old one.
	canvas.Reset()
	canvas.BeginFrame()
	canvas.HitWithOptions(gfx.Item(7), gfx.Rect{X: 100, Width: 40, Height: 40}, "", gfx.HitOptions{Role: gfx.RoleButton})
	canvas.EndFrame()
	focus.Sync(&canvas)
	if focus.Focused() != gfx.Item(7) || !focus.KeyboardFocus() || !focus.PendingFocus().IsZero() {
		t.Fatalf("resolved focus = %v/%v pending=%v", focus.Focused(), focus.KeyboardFocus(), focus.PendingFocus())
	}
}

func TestAutofocusTakesANewHitOnlyOnce(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		canvas.BeginFrame()
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 40, Height: 40}, "", gfx.HitOptions{Role: gfx.RoleButton, Autofocus: true})
		canvas.HitWithOptions(gfx.Item(2), gfx.Rect{X: 50, Width: 40, Height: 40}, "", gfx.HitOptions{Role: gfx.RoleButton, Autofocus: true})
		canvas.EndFrame()
	}
	paint()
	var focus Focus
	focus.Sync(&canvas)
	if focus.Focused() != gfx.Item(1) {
		t.Fatalf("autofocus chose %v, want item 1", focus.Focused())
	}
	focus.Apply(gfx.EventResult{Focus: gfx.Item(2), FocusVisible: true})
	focus.Sync(&canvas)
	if focus.Focused() != gfx.Item(2) {
		t.Fatalf("an old autofocus hit stole focus back: %v", focus.Focused())
	}
	paint()
	focus.Sync(&canvas)
	if focus.Focused() != gfx.Item(2) {
		t.Fatalf("a repaint of the same hits moved focus to %v", focus.Focused())
	}
}

func TestPointerDownClearsFocusOutsideATrappedScope(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginFrame()
	canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 40, Height: 40}, "", gfx.HitOptions{Role: gfx.RoleButton})
	canvas.EndFrame()
	var focus Focus
	focus.Sync(&canvas)
	focus.Apply(gfx.EventResult{Focus: gfx.Item(1), FocusVisible: true})
	focus.PointerDown(200, 200, gfx.ID{})
	if !focus.Focused().IsZero() || focus.KeyboardFocus() {
		t.Fatalf("empty press left focus %v/%v", focus.Focused(), focus.KeyboardFocus())
	}
}

func TestKeysPairReleasesByCodeAndNoteATargetChange(t *testing.T) {
	var keys Keys
	repeat, tracked := keys.Press(input.HeldKey{Code: 3, Key: gfx.KeyUnknown, Target: gfx.Item(1), Text: "c"})
	if repeat || !tracked || keys.Len() != 1 {
		t.Fatalf("press = repeat %v tracked %v len %d", repeat, tracked, keys.Len())
	}
	if repeat, _ := keys.Press(input.HeldKey{Code: 3, Key: gfx.KeyUnknown, Target: gfx.Item(1), Text: "c"}); !repeat {
		t.Fatal("a second press of the same code is not a repeat")
	}
	if _, ok := keys.Release(9); ok {
		t.Fatal("an orphan release was delivered")
	}
	held, ok := keys.Release(3)
	if !ok || held.Target != gfx.Item(1) || held.Text != "c" {
		t.Fatalf("release = %+v/%v", held, ok)
	}
	if keys.Len() != 0 {
		t.Fatalf("keys left down = %d", keys.Len())
	}
	keys.Press(input.HeldKey{Code: 4, Key: gfx.KeyUnknown, Target: gfx.Item(1)})
	keys.NoteTarget(gfx.Item(1), true)
	if keys.TakeDue() {
		t.Fatal("the same target asked for a release")
	}
	keys.NoteTarget(gfx.Item(2), true)
	if !keys.TakeDue() || keys.TakeDue() {
		t.Fatal("a moved keyboard target must ask for one release")
	}
}

func TestPointerCapturesPressAndClicksOnlyItsOwnTarget(t *testing.T) {
	var pointer Pointer
	pointer.SetHovered(gfx.Item(1))
	pointer.Down(gfx.PointerButtonPrimary)
	pointer.SetHovered(gfx.Item(2))
	if target, captured := pointer.Delivery(gfx.PointerMove); target != gfx.Item(1) || !captured {
		t.Fatalf("move delivery = %v/%v, want the captured item", target, captured)
	}
	if _, ok := pointer.Click(); ok {
		t.Fatal("releasing away from the press target clicked")
	}
	pointer.SetHovered(gfx.Item(1))
	if id, ok := pointer.Click(); !ok || id != gfx.Item(1) {
		t.Fatalf("click = %v/%v, want item 1", id, ok)
	}
	pointer.NoteHandled(gfx.PointerDown, true)
	if _, ok := pointer.Click(); ok {
		t.Fatal("a handled press still clicked")
	}
	pointer.Up()
	if !pointer.Pressed().IsZero() || !pointer.PressTarget().IsZero() {
		t.Fatal("release left the press active")
	}
}

func TestPointerExitKeepsAPressForItsRelease(t *testing.T) {
	var pointer Pointer
	pointer.SetHovered(gfx.Item(1))
	pointer.Down(gfx.PointerButtonPrimary)
	// A window that keeps a held press while the pointer leaves sets only hover.
	pointer.SetHovered(gfx.ID{})
	if pointer.Pressed() != gfx.Item(1) || pointer.PressTarget() != gfx.Item(1) {
		t.Fatalf("exit ended the press: %v/%v", pointer.Pressed(), pointer.PressTarget())
	}
	pointer.SetHovered(gfx.Item(1))
	if _, ok := pointer.Click(); !ok {
		t.Fatal("a release back on the press target should click")
	}
	pointer.Exit()
	if !pointer.Hovered().IsZero() || !pointer.Pressed().IsZero() {
		t.Fatal("exit did not clear hover and press")
	}
}

func TestCursorFollowsThePressTargetAndHit(t *testing.T) {
	image, err := gfx.NewCursorImage(1, 1, 0, 0, 1, []byte{0, 0, 0, 255})
	if err != nil {
		t.Fatal(err)
	}
	hits := []gfx.Hit{
		{ID: gfx.Item(1), Cursor: gfx.PointerCursorText},
		{ID: gfx.Item(2), Cursor: gfx.PointerCursorImage, CursorImage: image},
		{ID: gfx.Item(3), Cursor: gfx.PointerCursorHand, Disabled: true},
	}
	if cursor, got := CursorForTarget(hits, gfx.Item(1), gfx.ID{}); cursor != gfx.PointerCursorText || got != nil {
		t.Fatalf("hover cursor = %v %v, want text", cursor, got)
	}
	if cursor, got := CursorForTarget(hits, gfx.Item(1), gfx.Item(2)); cursor != gfx.PointerCursorImage || got != image {
		t.Fatalf("captured cursor = %v %v, want the image", cursor, got)
	}
	if cursor, got := CursorForTarget(hits, gfx.Item(3), gfx.ID{}); cursor != gfx.PointerCursorDefault || got != nil {
		t.Fatalf("disabled cursor = %v %v, want the arrow", cursor, got)
	}
}

func TestTextInputHitRequiresAFieldTerminalOrPlacedCombobox(t *testing.T) {
	var canvas gfx.Canvas
	canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 10, Height: 10}, "", gfx.HitOptions{Role: gfx.RoleTextField})
	canvas.HitWithOptions(gfx.Item(2), gfx.Rect{Width: 10, Height: 10}, "", gfx.HitOptions{Role: gfx.RoleCombobox})
	canvas.HitWithOptions(gfx.Item(3), gfx.Rect{Width: 10, Height: 10}, "", gfx.HitOptions{
		Role: gfx.RoleCombobox, TextBounds: gfx.Rect{Width: 40, Height: 20},
	})
	canvas.HitWithOptions(gfx.Item(4), gfx.Rect{Width: 10, Height: 10}, "", gfx.HitOptions{Role: gfx.RoleButton})
	if hit, index, ok := TextInputHit(&canvas, gfx.Item(1)); !ok || hit.ID != gfx.Item(1) || index != 0 {
		t.Fatalf("text field = %v/%d/%v", hit.ID, index, ok)
	}
	if _, _, ok := TextInputHit(&canvas, gfx.Item(2)); ok {
		t.Fatal("a combobox that paints its own query took the editor")
	}
	if hit, index, ok := TextInputHit(&canvas, gfx.Item(3)); !ok || hit.ID != gfx.Item(3) || index != 2 {
		t.Fatalf("placed combobox = %v/%d/%v", hit.ID, index, ok)
	}
	if _, _, ok := TextInputHit(&canvas, gfx.Item(4)); ok {
		t.Fatal("a button took the editor")
	}
}

func TestSliderStepsUseHomeEndAndArrowBounds(t *testing.T) {
	value := gfx.ValueRange{Set: true, Min: 0, Max: 100, Now: 40, Step: 5}
	if key, steps := SliderSteps(value, 0); key != gfx.KeyHome || steps != 1 {
		t.Fatalf("minimum = %v/%d, want Home/1", key, steps)
	}
	if key, steps := SliderSteps(value, 100); key != gfx.KeyEnd || steps != 1 {
		t.Fatalf("maximum = %v/%d, want End/1", key, steps)
	}
	if key, steps := SliderSteps(value, 60); key != gfx.KeyUp || steps != 4 {
		t.Fatalf("up = %v/%d, want Up/4", key, steps)
	}
	if key, steps := SliderSteps(value, 20); key != gfx.KeyDown || steps != 4 {
		t.Fatalf("down = %v/%d, want Down/4", key, steps)
	}
	if key, steps := SliderSteps(value, math.NaN()); key != gfx.KeyUnknown || steps != 0 {
		t.Fatalf("NaN = %v/%d, want no press", key, steps)
	}
	if key, steps := SliderSteps(value, 100000); key != gfx.KeyEnd || steps != 1 {
		t.Fatalf("far target = %v/%d, want End/1", key, steps)
	}
}
