//go:build js && wasm

// Command fixture is the browser verification page driven by
// tools/webcheck/check.mjs. It is not a product example: every control has a
// fixed position and publishes its state through the same ARIA mirror a real
// application uses, so a headless browser can assert the shared interaction
// path end to end.
package main

import (
	"fmt"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var (
	counterID  = ui.Item(1)
	menuID     = ui.Item(2)
	sliderID   = ui.Item(3)
	backdropID = ui.Item(4)
	editorID   = ui.Item(5)
	statusID   = ui.Item(6)
)

var (
	counterRect = ui.Rect{X: 20, Y: 20, Width: 180, Height: 40}
	menuRect    = ui.Rect{X: 220, Y: 20, Width: 180, Height: 40}
	sliderRect  = ui.Rect{X: 20, Y: 100, Width: 300, Height: 32}
	editorRect  = ui.Rect{X: 20, Y: 180, Width: 300, Height: 36}
	statusRect  = ui.Rect{X: 20, Y: 240, Width: 300, Height: 32}
)

type fixture struct {
	count    int
	level    float32
	menuOpen bool
	editor   *text.Editor
}

func main() {
	page := &fixture{level: 20, editor: text.NewEditor("")}
	_ = web.Run(web.Config{
		Title:     "Viva web check",
		Width:     640,
		Height:    480,
		Paint:     page.paint,
		OnPointer: page.pointer,
		OnKey:     page.key,
		TextInput: web.EditorInputs(func(id ui.ID) (*text.Editor, bool) {
			return page.editor, id == editorID
		}),
	})
}

func (f *fixture) paint(canvas *ui.Canvas, size ui.Size) {
	theme := ui.LightTheme()
	if components.BeginButton(canvas, theme, counterID, counterRect).Default(fmt.Sprintf("Count: %d", f.count)).Clicked() {
		f.count++
	}
	if components.BeginButton(canvas, theme, menuID, menuRect).Default(fmt.Sprintf("Menu: %v", f.menuOpen)).Clicked() {
		f.menuOpen = !f.menuOpen
	}

	slider := func(value float32) components.Slider {
		return components.BeginSlider(canvas, theme, sliderID, sliderRect, value).Range(0, 100).Step(1)
	}
	// The slider reads its pointer only through the event pass, exactly as the
	// flex slider does: the press, the captured moves and the release arrive in
	// a paint where PointerOn names this control.
	if event, ok := canvas.PointerOn(sliderID); ok {
		switch {
		case event.Kind == ui.PointerDown && event.Button <= ui.PointerButtonPrimary:
			f.level = slider(f.level).ValueAt(event.X)
			canvas.Consume()
		case event.Kind == ui.PointerMove && event.Captured:
			f.level = slider(f.level).ValueAt(event.X)
			canvas.Consume()
		case event.Kind == ui.PointerUp:
			canvas.Consume()
		}
	}
	slider(f.level).ValueText(fmt.Sprintf("%.0f", f.level)).Paint("Level")

	// A real editable field, wired to the browser text service the same way
	// flex.Fields is: the platform edits the Go-owned editor and publishes its
	// value through the same ARIA mirror a screen reader reads.
	value := f.editor.String()
	focused := canvas.Focused() == editorID
	components.PaintInput(canvas, theme, editorRect, components.InputSnapshot{
		ID: editorID, Label: "Name", Value: value, Placeholder: "Name",
		Focused: focused, PlatformEditing: focused,
	})
	canvas.HitWithOptions(statusID, statusRect, "Editor status", ui.HitOptions{
		Role: ui.RoleStatus, SkipFocus: true, Live: ui.LivePolite,
		Value: "Text: " + value,
	})

	if f.menuOpen {
		if canvas.Clicked(backdropID) {
			f.menuOpen = false
		}
		canvas.HitWithOptions(backdropID, ui.Rect{X: 0, Y: 0, Width: size.Width, Height: size.Height}, "Dismiss",
			ui.HitOptions{Backdrop: true, PassPrimary: true, SkipFocus: true, Role: ui.RoleGroup})
	}
}

func (f *fixture) pointer(ui.PointerEvent) ui.EventResult {
	return ui.EventResult{}
}

func (f *fixture) key(event ui.KeyEvent) ui.EventResult {
	if event.Target != sliderID {
		return ui.EventResult{}
	}
	value := f.level
	switch event.Key {
	case ui.KeyLeft, ui.KeyDown:
		value = max(0, value-1)
	case ui.KeyRight, ui.KeyUp:
		value = min(100, value+1)
	case ui.KeyHome:
		value = 0
	case ui.KeyEnd:
		value = 100
	default:
		return ui.EventResult{}
	}
	if value == f.level {
		return ui.EventResult{Handled: true}
	}
	f.level = value
	return ui.EventResult{Handled: true, Repaint: true}
}
