package app

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// presented collects the frames a request hands to the platform.
type presented struct{ frames []platform.Frame }

func (p *presented) present(frame platform.Frame) error {
	p.frames = append(p.frames, frame)
	return nil
}

func TestAccessibilityNodesCarryWhatAssistiveTechnologyNeeds(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "a11y", Width: 200, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.ContainerWithOptions(ui.Rect{Width: 100, Height: 20}, ui.RoleHeading, "Settings", ui.HitOptions{Level: 2})
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Y: 30, Width: 100, Height: 16}, "Volume", ui.HitOptions{
			Role: ui.RoleSlider, Value: "30 percent", Range: ui.ValueRange{Now: 30, Min: 0, Max: 100, Step: 5, Set: true},
			Description: "Output level", PosInSet: 2, SetSize: 4,
		})
		canvas.HitWithOptions(ui.Item(2), ui.Rect{Y: 60, Width: 100, Height: 20}, "Details", ui.HitOptions{Expandable: true})
	}
	painter := newTestPainter(t, &canvas, config)
	nodes := painter.accessibility
	if len(nodes) != 3 {
		t.Fatalf("nodes = %d", len(nodes))
	}
	if nodes[0].Level != 2 || nodes[0].ID != (ui.ID{}) {
		t.Errorf("heading = %+v", nodes[0])
	}
	slider := nodes[1]
	if slider.ID != ui.Item(1) || !slider.Range.Set || slider.Range.Now != 30 || slider.Range.Step != 5 ||
		slider.Description != "Output level" || slider.PosInSet != 2 || slider.SetSize != 4 || !slider.Focusable {
		t.Errorf("slider = %+v", slider)
	}
	if !nodes[2].Expandable || nodes[2].Expanded {
		t.Errorf("collapsed disclosure = %+v", nodes[2])
	}
}

func TestAccessibilityPressClicksOnceAndIgnoresDisabledControls(t *testing.T) {
	var canvas ui.Canvas
	var clicks []ui.ID
	config := Config{Title: "press", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{Width: 80, Height: 20}, "Save")
		canvas.HitWithOptions(ui.Item(2), ui.Rect{Y: 30, Width: 80, Height: 20}, "Delete", ui.HitOptions{Disabled: true})
	}
	onClick(&config, func(id ui.ID) { clicks = append(clicks, id) })
	painter := newTestPainter(t, &canvas, config)
	var out presented

	if ok, err := painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(1), Action: platform.AccessibilityPress}, out.present); !ok || err != nil {
		t.Fatalf("press = %v, %v", ok, err)
	}
	if len(clicks) != 1 || clicks[0] != ui.Item(1) {
		t.Fatalf("clicks = %v", clicks)
	}
	if ok, _ := painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(2), Action: platform.AccessibilityPress}, out.present); ok {
		t.Error("a disabled control was pressed")
	}
	if ok, _ := painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(9), Action: platform.AccessibilityPress}, out.present); ok {
		t.Error("a node that is not there was pressed")
	}
	if len(clicks) != 1 {
		t.Fatalf("clicks = %v", clicks)
	}
}

func TestAccessibilityFocusMovesTheKeyboard(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "focus", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{Width: 80, Height: 20}, "A")
		canvas.HitWithLabel(ui.Item(2), ui.Rect{Y: 30, Width: 80, Height: 20}, "B")
	}
	painter := newTestPainter(t, &canvas, config)
	var out presented
	if ok, err := painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(2), Action: platform.AccessibilityFocus}, out.present); !ok || err != nil {
		t.Fatalf("focus = %v, %v", ok, err)
	}
	if painter.focus.Focused() != ui.Item(2) {
		t.Fatalf("focused = %v", painter.focus.Focused())
	}
	last := out.frames[len(out.frames)-1]
	if last.Accessibility == nil {
		t.Fatal("the focus change did not reach the accessibility tree")
	}
}

func TestAccessibilitySliderSteps(t *testing.T) {
	var canvas ui.Canvas
	var keys []ui.Key
	value := 30.0
	config := Config{Title: "slider", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 100, Height: 16}, "Volume", ui.HitOptions{
			Role: ui.RoleSlider, Range: ui.ValueRange{Now: value, Min: 0, Max: 100, Step: 5, Set: true},
		})
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		keys = append(keys, event.Key)
		switch event.Key {
		case ui.KeyUp:
			value += 5
		case ui.KeyDown:
			value -= 5
		case ui.KeyEnd:
			value = 100
		}
		return ui.EventResult{Handled: true, Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	var out presented
	request := func(action platform.AccessibilityAction, number float64) {
		t.Helper()
		keys = nil
		ok, err := painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(1), Action: action, Number: number}, out.present)
		if !ok || err != nil {
			t.Fatalf("action %d = %v, %v", action, ok, err)
		}
	}
	request(platform.AccessibilityIncrement, 0)
	if len(keys) != 1 || keys[0] != ui.KeyUp || value != 35 {
		t.Fatalf("increment: keys %v value %v", keys, value)
	}
	request(platform.AccessibilitySetNumber, 50)
	if value != 50 || len(keys) != 3 {
		t.Fatalf("set 50: keys %v value %v", keys, value)
	}
	request(platform.AccessibilitySetNumber, 1000)
	if value != 100 || len(keys) != 1 || keys[0] != ui.KeyEnd {
		t.Fatalf("set past the maximum: keys %v value %v", keys, value)
	}
	if len(out.frames) < 3 {
		t.Fatalf("each step must be presented, got %d frames", len(out.frames))
	}
}

func TestAccessibilitySetTextReplacesTheFieldAndTellsThePlatformEditor(t *testing.T) {
	var canvas ui.Canvas
	editor := text.NewEditor("old name")
	config := Config{Title: "text", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 150, Height: 24}, "Name", ui.HitOptions{Role: ui.RoleTextField, Value: editor.String()})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{Editor: editor, Bounds: ui.Rect{Width: 150, Height: 24}, FontSize: 13}
	}
	painter := newTestPainter(t, &canvas, config)
	var out presented
	ok, err := painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(1), Action: platform.AccessibilitySetText, Text: "new name"}, out.present)
	if !ok || err != nil {
		t.Fatalf("set text = %v, %v", ok, err)
	}
	if editor.String() != "new name" {
		t.Fatalf("editor = %q", editor.String())
	}
	last := out.frames[len(out.frames)-1]
	if !last.TextInput.Changed || last.TextInput.Input == nil || last.TextInput.Input.State.Value != "new name" {
		t.Fatalf("the platform editor was not given the new text: %+v", last.TextInput)
	}

	ok, err = painter.accessibilityRequest(platform.AccessibilityRequest{Node: ui.Item(1), Action: platform.AccessibilitySetSelection, Start: 4, End: 8}, out.present)
	if !ok || err != nil {
		t.Fatalf("set selection = %v, %v", ok, err)
	}
	if selection := editor.Selection(); selection.Start != 4 || selection.End != 8 {
		t.Fatalf("selection = %+v", selection)
	}
	node := painter.accessibility[0]
	if !node.Selection.Valid || node.Selection.Start != 4 || node.Selection.End != 8 {
		t.Fatalf("the node does not report the selection: %+v", node.Selection)
	}
}

func TestAccessibilityDoesNotEditReadOnlyOrSecureFields(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "text", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 150, Height: 24}, "Id", ui.HitOptions{Role: ui.RoleTextField, ReadOnly: true})
		canvas.HitWithOptions(ui.Item(2), ui.Rect{Y: 30, Width: 150, Height: 24}, "Password", ui.HitOptions{Role: ui.RoleTextField, Secure: true})
	}
	painter := newTestPainter(t, &canvas, config)
	var out presented
	for _, id := range []ui.ID{ui.Item(1), ui.Item(2)} {
		if ok, _ := painter.accessibilityRequest(platform.AccessibilityRequest{Node: id, Action: platform.AccessibilitySetText, Text: "x"}, out.present); ok {
			t.Errorf("%v was edited", id)
		}
	}
}

func TestRegionsNeverReportTheKeyboard(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "regions", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.ContainerWithOptions(ui.Rect{Width: 100, Height: 20}, ui.RoleStatus, "Saved", ui.HitOptions{Live: ui.LivePolite})
		canvas.HitWithLabel(ui.Item(1), ui.Rect{Y: 30, Width: 80, Height: 20}, "Save")
	}
	painter := newTestPainter(t, &canvas, config)
	// Nothing has the keyboard, and a region has no identity to have it with.
	for _, node := range painter.accessibility {
		if node.Focused {
			t.Errorf("%+v claims the keyboard", node)
		}
	}
	if !painter.accessibility[0].ID.IsZero() || painter.accessibility[0].Live != ui.LivePolite {
		t.Errorf("region = %+v", painter.accessibility[0])
	}
}
