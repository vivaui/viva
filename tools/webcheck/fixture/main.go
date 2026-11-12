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
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/text"
)

var (
	counterID  = gfx.Item(1)
	menuID     = gfx.Item(2)
	sliderID   = gfx.Item(3)
	backdropID = gfx.Item(4)
	editorID   = gfx.Item(5)
	statusID   = gfx.Item(6)
)

var (
	counterRect = gfx.Rect{X: 20, Y: 20, Width: 180, Height: 40}
	menuRect    = gfx.Rect{X: 220, Y: 20, Width: 180, Height: 40}
	sliderRect  = gfx.Rect{X: 20, Y: 100, Width: 300, Height: 32}
	editorRect  = gfx.Rect{X: 20, Y: 180, Width: 300, Height: 36}
	statusRect  = gfx.Rect{X: 20, Y: 240, Width: 300, Height: 32}
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
		TextInput: web.EditorInputs(func(id gfx.ID) (*text.Editor, bool) {
			return page.editor, id == editorID
		}),
	})
}

func (f *fixture) paint(canvas *gfx.Canvas, size gfx.Size) {
	theme := gfx.LightTheme()
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
	// ui slider does: the press, the captured moves and the release arrive in
	// a paint where PointerOn names this control.
	if event, ok := canvas.PointerOn(sliderID); ok {
		switch {
		case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
			f.level = slider(f.level).ValueAt(event.X)
			canvas.Consume()
		case event.Kind == gfx.PointerMove && event.Captured:
			f.level = slider(f.level).ValueAt(event.X)
			canvas.Consume()
		case event.Kind == gfx.PointerUp:
			canvas.Consume()
		}
	}
	slider(f.level).ValueText(fmt.Sprintf("%.0f", f.level)).Paint("Level")

	// A real editable field, wired to the browser text service the same way
	// ui.Fields is: the browser edits the Go-owned editor and publishes its
	// value through the same ARIA mirror a screen reader reads, while the
	// component paints the line (HideEditorText) so both states share one line.
	value := f.editor.String()
	focused := canvas.Focused() == editorID
	selection := f.editor.Selection()
	snapshot := components.InputSnapshot{
		ID: editorID, Label: "Name", Value: value, Placeholder: "Name",
		Focused: focused, HideEditorText: true, ShowCaret: focused && selection.Start == selection.End,
	}
	snapshot.Caret = f.editor.ByteOffset(f.editor.Caret())
	snapshot.SelectionStart, snapshot.SelectionEnd = f.editor.ByteOffset(selection.Start), f.editor.ByteOffset(selection.End)
	if marked, composing := f.editor.Composition(); composing {
		snapshot.CompositionStart, snapshot.CompositionEnd = f.editor.ByteOffset(marked.Start), f.editor.ByteOffset(marked.End)
	}
	components.PaintInput(canvas, theme, editorRect, snapshot)
	canvas.HitWithOptions(statusID, statusRect, "Editor status", gfx.HitOptions{
		Role: gfx.RoleStatus, SkipFocus: true, Live: gfx.LivePolite,
		Value: "Text: " + value,
	})

	if f.menuOpen {
		if canvas.Clicked(backdropID) {
			f.menuOpen = false
		}
		canvas.HitWithOptions(backdropID, gfx.Rect{X: 0, Y: 0, Width: size.Width, Height: size.Height}, "Dismiss",
			gfx.HitOptions{Backdrop: true, PassPrimary: true, SkipFocus: true, Role: gfx.RoleGroup})
	}
}

func (f *fixture) pointer(gfx.PointerEvent) gfx.EventResult {
	return gfx.EventResult{}
}

func (f *fixture) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Target != sliderID {
		return gfx.EventResult{}
	}
	value := f.level
	switch event.Key {
	case gfx.KeyLeft, gfx.KeyDown:
		value = max(0, value-1)
	case gfx.KeyRight, gfx.KeyUp:
		value = min(100, value+1)
	case gfx.KeyHome:
		value = 0
	case gfx.KeyEnd:
		value = 100
	default:
		return gfx.EventResult{}
	}
	if value == f.level {
		return gfx.EventResult{Handled: true}
	}
	f.level = value
	return gfx.EventResult{Handled: true, Repaint: true}
}
