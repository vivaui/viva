package componentdemo

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/text"
)

func TestButtonCountsAndDisabledVariants(t *testing.T) {
	for _, example := range buttonComponent.Examples {
		state := newTestState(t, "button", example.ID)
		var canvas gfx.Canvas
		paintFrame(&canvas, state, testSize, time.Second, true)
		hits := slices.Clone(canvas.Hits())
		wantCount := 6
		if example.ID == "default" {
			wantCount = 1
		}
		if len(hits) != wantCount {
			t.Fatalf("%s has %d hits, want %d", example.ID, len(hits), wantCount)
		}
		for _, hit := range hits {
			if hit.Role != gfx.RoleButton || hit.Disabled != (example.ID == "disabled") || hit.Focusable == hit.Disabled {
				t.Fatalf("incorrect button semantics: %+v", hit)
			}
			if !hit.Disabled { // the runtime never delivers a click to a disabled control
				click(&canvas, state, hit.ID)
			}
		}
		paintFrame(&canvas, state, testSize, time.Second, true)
		if example.ID == "disabled" {
			wantCount = 0
			for _, label := range canvas.Texts()[1:7] {
				if label.Style.Color.Alpha != 0.5 {
					t.Fatalf("disabled variant not visibly dimmed: %+v", label)
				}
			}
		}
		if !hasText(&canvas, state.Text("点击：", "Clicks: ")+strconv.Itoa(wantCount)) {
			t.Fatal("button activation did not update the visible count")
		}
		before := state.button
		click(&canvas, state, buttonPress.At(6))
		if example.ID == "default" {
			click(&canvas, state, buttonPress.At(1))
		}
		if state.button != before {
			t.Fatal("invisible button changed the count")
		}
	}
}

func TestButtonUsesCanvasInteractionAndFocusVisible(t *testing.T) {
	state := newTestState(t, "button", "default")
	var canvas gfx.Canvas
	paintFrame(&canvas, state, testSize, time.Second, true)
	idle := snapshot(&canvas)
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, buttonPress.At(0), false)
	paintFrame(&canvas, state, testSize, time.Second, true)
	assertSamePaint(t, idle, &canvas)
	canvas.SetInteraction(buttonPress.At(0), buttonPress.At(0), buttonPress.At(0), true)
	paintFrame(&canvas, state, testSize, time.Second, true)
	if slices.Equal(idle.rectangles, canvas.Rectangles()) || len(canvas.Rectangles()) <= len(idle.rectangles) {
		t.Fatal("live button ignored pressed state or keyboard focus ring")
	}
}

func TestBadgesFitContentAndNeverDeclareHits(t *testing.T) {
	for _, example := range badgeComponent.Examples {
		for _, measured := range []bool{false, true} {
			state := newTestState(t, "badge", example.ID)
			var canvas gfx.Canvas
			if measured {
				canvas.SetMeasurer(measureTestText)
			}
			paintFrame(&canvas, state, testSize, time.Second, true)
			assertLayout(t, &canvas, testSize)
			if len(canvas.Hits()) != 0 {
				t.Fatal("badge declared an interactive hit")
			}
			if example.ID == "default" {
				first := canvas.Rectangles()[1].Bounds
				if first.Width != canvas.TextWidthWeight("New", gfx.LightTheme().Metrics.FontSizeS, gfx.WeightMedium)+18 {
					t.Fatalf("badge ignored natural width: %+v", first)
				}
			}
			if example.ID == "long-text" {
				if !hasText(&canvas, "这是一条长中文状态标签") {
					t.Fatal("long Chinese label was shortened")
				}
				if textBounds(t, &canvas, "Viva 原生组件 · 已发布").Y <= textBounds(t, &canvas, "审核中").Y {
					t.Fatal("long labels should wrap to multiple rows at 320px")
				}
			}
		}
	}
}

func TestCheckboxAndSwitchAreControlled(t *testing.T) {
	for _, componentID := range []string{"checkbox", "switch"} {
		component := checkboxComponent
		role := gfx.RoleCheckbox
		if componentID == "switch" {
			component, role = switchComponent, gfx.RoleSwitch
		}
		for _, example := range component.Examples {
			t.Run(componentID+"/"+example.ID, func(t *testing.T) {
				state := newTestState(t, componentID, example.ID)
				var canvas gfx.Canvas
				paintFrame(&canvas, state, testSize, time.Second, true)
				initial := snapshot(&canvas)
				for _, hit := range initial.hits {
					if hit.Role != role || hit.Focusable == hit.Disabled {
						t.Fatalf("incorrect toggle semantics: %+v", hit)
					}
					click(&canvas, state, hit.ID)
					paintFrame(&canvas, state, testSize, time.Second, true)
					got := findHit(t, &canvas, hit.ID)
					wantChecked := hit.Checked
					if !hit.Disabled {
						wantChecked = !wantChecked
					}
					if got.Checked != wantChecked {
						t.Fatalf("controlled state did not toggle correctly: before=%+v after=%+v", hit, got)
					}
					click(&canvas, state, hit.ID)
					paintFrame(&canvas, state, testSize, time.Second, true)
					assertSamePaint(t, initial, &canvas)
				}
				click(&canvas, state, checkboxBox.At(3))
				if componentID == "switch" {
					click(&canvas, state, switchToggle.At(3))
				}
				if example.ID == "default" {
					paintFrame(&canvas, state, testSize, time.Second, true)
					assertSamePaint(t, initial, &canvas)
				}
			})
		}
	}
}

func TestSwitchMotionSettlesWithoutIdleFrames(t *testing.T) {
	state := newTestState(t, "switch", "default")
	var canvas gfx.Canvas
	paintFrame(&canvas, state, testSize, time.Second, false)
	if canvas.Animating() {
		t.Fatal("initial switch requested animation")
	}
	clickAt(&canvas, state, switchToggle.At(0), 2*time.Second, false)
	paintFrame(&canvas, state, testSize, 2*time.Second, false)
	if !canvas.Animating() || !findHit(t, &canvas, switchToggle.At(0)).Checked {
		t.Fatal("switch did not animate toward its immediate semantic destination")
	}
	paintFrame(&canvas, state, testSize, 2*time.Second+gfx.DefaultMotion().Duration, false)
	if canvas.Animating() {
		t.Fatal("switch transition did not terminate")
	}
	expected := snapshot(&canvas)
	paintFrame(&canvas, state, testSize, 10*time.Second, false)
	assertSamePaint(t, expected, &canvas)
	if canvas.Animating() {
		t.Fatal("settled switch requested an idle frame")
	}
}

func TestEditorOnlyResolvesEditablePlatformInput(t *testing.T) {
	for _, component := range Catalog() {
		for _, example := range component.Examples {
			state := newTestState(t, component.ID, example.ID)
			state.EnableTextEditing()
			var canvas gfx.Canvas
			paintFrame(&canvas, state, testSize, time.Second, true)
			for _, hit := range canvas.Hits() {
				if hit.IsLabel { // a label has its control's identity, not its editor
					continue
				}
				editor, ok := state.Editor(hit.ID)
				want := (hit.Role == gfx.RoleTextField || component.editable != nil && hit.Role == gfx.RoleCombobox) && !hit.Disabled && !hit.ReadOnly
				if ok != want || (editor != nil) != want {
					t.Fatalf("%s/%s: incorrect editor resolution for %+v", component.ID, example.ID, hit)
				}
			}
			for _, id := range []gfx.ID{{}, gfx.Item(999), inputDisabled, inputReadOnly, buttonPress.At(0)} {
				if editor, ok := state.Editor(id); ok || editor != nil {
					t.Fatalf("unexpected editor for ID %v", id)
				}
			}
		}
	}
}

func TestPlatformInputEditingCompositionAndPlaceholder(t *testing.T) {
	state := newTestState(t, "input", "placeholder")
	state.EnableTextEditing()
	editor, ok := state.Editor(inputEditable)
	if !ok {
		t.Fatal("editable case has no platform editor")
	}
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, inputEditable, true)
	paintFrame(&canvas, state, testSize, time.Second, true)
	if !hasText(&canvas, "输入中文或 English…") {
		t.Fatal("focused empty platform input lost its placeholder")
	}
	editor.Insert("Go 中😀")
	editor.SetSelection(text.Range{Start: 3, End: 4})
	editor.SetMarkedText("中文", text.Range{Start: 1, End: 2})
	paintFrame(&canvas, state, testSize, time.Second, true)
	if value := findHit(t, &canvas, inputEditable).Value; value != "Go 中文😀" {
		t.Fatalf("provider changes were not reflected in semantics: %q", value)
	}
	if !hasText(&canvas, "Go 中文😀") {
		t.Fatal("the hidden platform editor's text was not painted on the canvas")
	}
	hit := findHit(t, &canvas, inputEditable)
	textInfo, _ := canvas.HitText(&hit)
	if textInfo.Color.Alpha != 0 || !hit.HideNativeOverlays {
		t.Fatalf("the platform text was not hidden: %+v", hit)
	}
	if selection := editor.Selection(); selection != (text.Range{Start: 4, End: 5}) {
		t.Fatalf("painting changed the editor selection: %+v", selection)
	}
	if composition, composing := editor.Composition(); !composing || composition != (text.Range{Start: 3, End: 5}) {
		t.Fatal("painting changed IME composition")
	}
	editor.Unmark()
	state.EnableTextEditing()
	if same, _ := state.Editor(inputEditable); same != editor || same.String() != "Go 中文😀" {
		t.Fatal("re-enabling platform input reset its editor")
	}
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.ID{}, false)
	paintFrame(&canvas, state, testSize, time.Second, true)
	if !hasText(&canvas, "Go 中文😀") {
		t.Fatal("blurred input did not paint its updated value")
	}
}

func TestInputInvalidStateReactsToRealEdits(t *testing.T) {
	state := newTestState(t, "input", "invalid")
	state.EnableTextEditing()
	editor, _ := state.Editor(inputEditable)
	var canvas gfx.Canvas
	paintFrame(&canvas, state, testSize, time.Second, true)
	dangerRectangles := func() (count int) {
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == gfx.DarkTheme().Danger || rectangle.Color == gfx.LightTheme().Danger {
				count++
			}
		}
		return count
	}
	if dangerRectangles() == 0 {
		t.Fatal("an empty required value did not paint the danger border")
	}
	editor.SetSelection(text.Range{End: editor.RuneCount()})
	editor.Insert("hello@viva.dev")
	paintFrame(&canvas, state, testSize, time.Second, true)
	if !hasText(&canvas, "演示校验通过。") || dangerRectangles() != 0 {
		t.Fatal("editing a valid example value did not clear the danger border")
	}
}

func syncRouter(router *input.Router, canvas *gfx.Canvas) {
	regions := make([]input.Region, len(canvas.Hits()))
	for index, hit := range canvas.Hits() {
		regions[index] = input.Region{
			Target: input.Target(hit.ID), Scope: hit.Scope, Focusable: hit.Focusable, Disabled: hit.Disabled,
			Bounds: input.Rect{X: hit.Bounds.X, Y: hit.Bounds.Y, Width: hit.Bounds.Width, Height: hit.Bounds.Height},
		}
	}
	router.SetRegions(regions)
	router.SetActiveScope(canvas.TrappedFocusScope(), input.Target(canvas.InitialFocus()))
}

func TestDialogActionsFocusTrapEscapeAndRestoration(t *testing.T) {
	for _, example := range dialogComponent.Examples {
		for _, action := range []gfx.ID{dialogConfirm, dialogCancel, dialogClose} {
			state := newTestState(t, "dialog", example.ID)
			var canvas gfx.Canvas
			click(&canvas, state, dialogCancel)
			var router input.Router
			paintFrame(&canvas, state, testSize, time.Second, true)
			syncRouter(&router, &canvas)
			if !router.Focus(input.Target(dialogOpen)) {
				t.Fatal("trigger cannot receive focus")
			}
			click(&canvas, state, dialogOpen)
			paintFrame(&canvas, state, testSize, time.Second, true)
			syncRouter(&router, &canvas)
			if canvas.TrappedFocusScope() != dialogFocus || canvas.EscapeTarget() != dialogClose {
				t.Fatal("dialog did not install focus and Escape routing")
			}
			if router.Focus(input.Target(dialogOpen)) {
				t.Fatal("modal allowed focus to escape to its trigger")
			}
			for _, reverse := range []bool{false, true} {
				modifiers := input.Modifiers(0)
				if reverse {
					modifiers = input.ModifierShift
				}
				seen := make(map[gfx.ID]bool)
				for range 6 {
					result := router.KeyDown(input.KeyEvent{Key: input.KeyTab, Modifiers: modifiers})
					hit := findHit(t, &canvas, result.Target)
					if hit.Scope != dialogFocus || !hit.Focusable {
						t.Fatalf("Tab escaped modal: %+v", hit)
					}
					seen[hit.ID] = true
				}
				if len(seen) != 3 {
					t.Fatalf("Tab did not visit all dialog actions: %v", seen)
				}
			}
			if target, ok := router.HitTest(2, 2); ok || !target.IsZero() {
				t.Fatal("modal backdrop should block, not dismiss or activate background content")
			}
			if action == dialogClose {
				click(&canvas, state, canvas.EscapeTarget())
			} else {
				router.Focus(input.Target(action))
				result := router.KeyDown(input.KeyEvent{Key: input.KeyEnter})
				if !result.Activated {
					t.Fatal("dialog action could not be keyboard activated")
				}
				click(&canvas, state, gfx.ID(result.Target))
			}
			paintFrame(&canvas, state, testSize, time.Second, true)
			syncRouter(&router, &canvas)
			focused, _ := router.Focused()
			if state.dialog.open || !canvas.TrappedFocusScope().IsZero() || !canvas.EscapeTarget().IsZero() || focused != input.Target(dialogOpen) {
				t.Fatal("closing dialog did not clear the modal and restore trigger focus")
			}
			expected := "已取消 · 未应用更改"
			if action == dialogConfirm {
				expected = "已确认 · 仅更新示例状态"
			}
			if !hasText(&canvas, expected) {
				t.Fatal("dialog result was not visible")
			}
			click(&canvas, state, dialogConfirm)
			if state.dialog.result != expected {
				t.Fatal("stale hidden dialog action changed state")
			}
		}
	}
}
