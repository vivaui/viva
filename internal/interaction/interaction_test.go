package interaction

import (
	"math"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

func TestFocusRegionsSkipContainersAndOrderByLayer(t *testing.T) {
	var canvas ui.Canvas
	canvas.BeginFrame()
	canvas.Hit(ui.Item(1), ui.Rect{Width: 40, Height: 40})
	canvas.Container(ui.Rect{X: 10, Y: 10, Width: 40, Height: 40}, ui.RoleGroup, "")
	canvas.PushLayer(2)
	canvas.Hit(ui.Item(3), ui.Rect{X: 20, Y: 20, Width: 40, Height: 40})
	canvas.PopLayer()
	canvas.EndFrame()
	var focus Focus
	focus.Sync(&canvas)
	regions := focus.Router().Regions()
	if len(regions) != 2 || regions[0].Target != ui.Item(1) || regions[1].Target != ui.Item(3) {
		t.Fatalf("regions = %+v, want item 1 then item 3 on the higher layer", regions)
	}
	if target, ok := focus.Router().HitTest(25, 25); !ok || target != ui.Item(3) {
		t.Fatalf("front-most hit = %v/%v, want item 3", target, ok)
	}
}

func TestFocusKeepsAFocusedTargetAcrossRepaints(t *testing.T) {
	var canvas ui.Canvas
	canvas.BeginFrame()
	canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 40, Height: 40}, "", ui.HitOptions{Role: ui.RoleButton})
	canvas.EndFrame()
	var focus Focus
	focus.Sync(&canvas)
	if !focus.Apply(ui.EventResult{Focus: ui.Item(1), FocusVisible: true}) {
		t.Fatal("focusing an existing hit should report a change")
	}
	if focus.Focused() != ui.Item(1) || !focus.KeyboardFocus() {
		t.Fatalf("focus = %v/%v, want item 1 with a ring", focus.Focused(), focus.KeyboardFocus())
	}
	focus.Sync(&canvas)
	if focus.Focused() != ui.Item(1) || !focus.KeyboardFocus() {
		t.Fatalf("repaint lost focus: %v/%v", focus.Focused(), focus.KeyboardFocus())
	}
}

func TestPendingFocusUsesTheRevealedPaint(t *testing.T) {
	var canvas ui.Canvas
	var focus Focus
	focus.Sync(&canvas)
	if !focus.Apply(ui.EventResult{Focus: ui.Item(7), FocusVisible: true}) {
		t.Fatal("focusing a not-yet-painted hit should be pending")
	}
	if focus.PendingFocus() != ui.Item(7) {
		t.Fatalf("pending focus = %v", focus.PendingFocus())
	}
	if id, visible := focus.PaintTarget(); id != ui.Item(7) || !visible {
		t.Fatalf("paint target = %v/%v, want the pending item", id, visible)
	}
	// The next paint reveals the target because it was allowed to lay out
	// around the pending focus rather than the old one.
	canvas.Reset()
	canvas.BeginFrame()
	canvas.HitWithOptions(ui.Item(7), ui.Rect{X: 100, Width: 40, Height: 40}, "", ui.HitOptions{Role: ui.RoleButton})
	canvas.EndFrame()
	focus.Sync(&canvas)
	if focus.Focused() != ui.Item(7) || !focus.KeyboardFocus() || !focus.PendingFocus().IsZero() {
		t.Fatalf("resolved focus = %v/%v pending=%v", focus.Focused(), focus.KeyboardFocus(), focus.PendingFocus())
	}
}

func TestAutofocusTakesANewHitOnlyOnce(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		canvas.BeginFrame()
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 40, Height: 40}, "", ui.HitOptions{Role: ui.RoleButton, Autofocus: true})
		canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 50, Width: 40, Height: 40}, "", ui.HitOptions{Role: ui.RoleButton, Autofocus: true})
		canvas.EndFrame()
	}
	paint()
	var focus Focus
	focus.Sync(&canvas)
	if focus.Focused() != ui.Item(1) {
		t.Fatalf("autofocus chose %v, want item 1", focus.Focused())
	}
	focus.Apply(ui.EventResult{Focus: ui.Item(2), FocusVisible: true})
	focus.Sync(&canvas)
	if focus.Focused() != ui.Item(2) {
		t.Fatalf("an old autofocus hit stole focus back: %v", focus.Focused())
	}
	paint()
	focus.Sync(&canvas)
	if focus.Focused() != ui.Item(2) {
		t.Fatalf("a repaint of the same hits moved focus to %v", focus.Focused())
	}
}

func TestPointerDownClearsFocusOutsideATrappedScope(t *testing.T) {
	var canvas ui.Canvas
	canvas.BeginFrame()
	canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 40, Height: 40}, "", ui.HitOptions{Role: ui.RoleButton})
	canvas.EndFrame()
	var focus Focus
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: ui.Item(1), FocusVisible: true})
	focus.PointerDown(200, 200, ui.ID{})
	if !focus.Focused().IsZero() || focus.KeyboardFocus() {
		t.Fatalf("empty press left focus %v/%v", focus.Focused(), focus.KeyboardFocus())
	}
}

func TestKeysPairReleasesByCodeAndNoteATargetChange(t *testing.T) {
	var keys Keys
	repeat, tracked := keys.Press(input.HeldKey{Code: 3, Key: ui.KeyUnknown, Target: ui.Item(1), Text: "c"})
	if repeat || !tracked || keys.Len() != 1 {
		t.Fatalf("press = repeat %v tracked %v len %d", repeat, tracked, keys.Len())
	}
	if repeat, _ := keys.Press(input.HeldKey{Code: 3, Key: ui.KeyUnknown, Target: ui.Item(1), Text: "c"}); !repeat {
		t.Fatal("a second press of the same code is not a repeat")
	}
	if _, ok := keys.Release(9); ok {
		t.Fatal("an orphan release was delivered")
	}
	held, ok := keys.Release(3)
	if !ok || held.Target != ui.Item(1) || held.Text != "c" {
		t.Fatalf("release = %+v/%v", held, ok)
	}
	if keys.Len() != 0 {
		t.Fatalf("keys left down = %d", keys.Len())
	}
	keys.Press(input.HeldKey{Code: 4, Key: ui.KeyUnknown, Target: ui.Item(1)})
	keys.NoteTarget(ui.Item(1), true)
	if keys.TakeDue() {
		t.Fatal("the same target asked for a release")
	}
	keys.NoteTarget(ui.Item(2), true)
	if !keys.TakeDue() || keys.TakeDue() {
		t.Fatal("a moved keyboard target must ask for one release")
	}
}

func TestPointerCapturesPressAndClicksOnlyItsOwnTarget(t *testing.T) {
	var pointer Pointer
	pointer.SetHovered(ui.Item(1))
	pointer.Down(ui.PointerButtonPrimary)
	pointer.SetHovered(ui.Item(2))
	if target, captured := pointer.Delivery(ui.PointerMove); target != ui.Item(1) || !captured {
		t.Fatalf("move delivery = %v/%v, want the captured item", target, captured)
	}
	if _, ok := pointer.Click(); ok {
		t.Fatal("releasing away from the press target clicked")
	}
	pointer.SetHovered(ui.Item(1))
	if id, ok := pointer.Click(); !ok || id != ui.Item(1) {
		t.Fatalf("click = %v/%v, want item 1", id, ok)
	}
	pointer.NoteHandled(ui.PointerDown, true)
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
	pointer.SetHovered(ui.Item(1))
	pointer.Down(ui.PointerButtonPrimary)
	// A window that keeps a held press while the pointer leaves sets only hover.
	pointer.SetHovered(ui.ID{})
	if pointer.Pressed() != ui.Item(1) || pointer.PressTarget() != ui.Item(1) {
		t.Fatalf("exit ended the press: %v/%v", pointer.Pressed(), pointer.PressTarget())
	}
	pointer.SetHovered(ui.Item(1))
	if _, ok := pointer.Click(); !ok {
		t.Fatal("a release back on the press target should click")
	}
	pointer.Exit()
	if !pointer.Hovered().IsZero() || !pointer.Pressed().IsZero() {
		t.Fatal("exit did not clear hover and press")
	}
}

func TestCursorFollowsThePressTargetAndHit(t *testing.T) {
	image, err := ui.NewCursorImage(1, 1, 0, 0, 1, []byte{0, 0, 0, 255})
	if err != nil {
		t.Fatal(err)
	}
	hits := []ui.Hit{
		{ID: ui.Item(1), Cursor: ui.PointerCursorText},
		{ID: ui.Item(2), Cursor: ui.PointerCursorImage, CursorImage: image},
		{ID: ui.Item(3), Cursor: ui.PointerCursorHand, Disabled: true},
	}
	if cursor, got := CursorForTarget(hits, ui.Item(1), ui.ID{}); cursor != ui.PointerCursorText || got != nil {
		t.Fatalf("hover cursor = %v %v, want text", cursor, got)
	}
	if cursor, got := CursorForTarget(hits, ui.Item(1), ui.Item(2)); cursor != ui.PointerCursorImage || got != image {
		t.Fatalf("captured cursor = %v %v, want the image", cursor, got)
	}
	if cursor, got := CursorForTarget(hits, ui.Item(3), ui.ID{}); cursor != ui.PointerCursorDefault || got != nil {
		t.Fatalf("disabled cursor = %v %v, want the arrow", cursor, got)
	}
}

func TestTextInputHitRequiresAFieldTerminalOrPlacedCombobox(t *testing.T) {
	hits := []ui.Hit{
		{ID: ui.Item(1), Role: ui.RoleTextField},
		{ID: ui.Item(2), Role: ui.RoleCombobox},
		{ID: ui.Item(3), Role: ui.RoleCombobox, TextBounds: ui.Rect{Width: 40, Height: 20}},
		{ID: ui.Item(4), Role: ui.RoleButton},
	}
	if hit, ok := TextInputHit(hits, ui.Item(1)); !ok || hit.ID != ui.Item(1) {
		t.Fatalf("text field = %v/%v", hit.ID, ok)
	}
	if _, ok := TextInputHit(hits, ui.Item(2)); ok {
		t.Fatal("a combobox that paints its own query took the editor")
	}
	if hit, ok := TextInputHit(hits, ui.Item(3)); !ok || hit.ID != ui.Item(3) {
		t.Fatalf("placed combobox = %v/%v", hit.ID, ok)
	}
	if _, ok := TextInputHit(hits, ui.Item(4)); ok {
		t.Fatal("a button took the editor")
	}
}

func TestSliderStepsUseHomeEndAndArrowBounds(t *testing.T) {
	value := ui.ValueRange{Set: true, Min: 0, Max: 100, Now: 40, Step: 5}
	if key, steps := SliderSteps(value, 0); key != ui.KeyHome || steps != 1 {
		t.Fatalf("minimum = %v/%d, want Home/1", key, steps)
	}
	if key, steps := SliderSteps(value, 100); key != ui.KeyEnd || steps != 1 {
		t.Fatalf("maximum = %v/%d, want End/1", key, steps)
	}
	if key, steps := SliderSteps(value, 60); key != ui.KeyUp || steps != 4 {
		t.Fatalf("up = %v/%d, want Up/4", key, steps)
	}
	if key, steps := SliderSteps(value, 20); key != ui.KeyDown || steps != 4 {
		t.Fatalf("down = %v/%d, want Down/4", key, steps)
	}
	if key, steps := SliderSteps(value, math.NaN()); key != ui.KeyUnknown || steps != 0 {
		t.Fatalf("NaN = %v/%d, want no press", key, steps)
	}
	if key, steps := SliderSteps(value, 100000); key != ui.KeyEnd || steps != 1 {
		t.Fatalf("far target = %v/%d, want End/1", key, steps)
	}
}
