//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

type handledCaretPlacement struct{ calls int }

func (placement *handledCaretPlacement) OffsetAt(point gfx.Point) (int, bool) {
	placement.calls++
	return int(point.X / 10), true
}

func (*handledCaretPlacement) CaretBounds() (gfx.Rect, bool) { return gfx.Rect{}, false }

// A handler that returns Handled owns the selection for that pointer event; the
// runtime must not place a default caret over it, on wasm as on the native
// backends.
func TestHandledPointerOwnsPlacedEditorCaret(t *testing.T) {
	phases := []struct {
		name string
		kind gfx.PointerEventKind
	}{
		{"press", gfx.PointerDown},
		{"move", gfx.PointerMove},
		{"release", gfx.PointerUp},
	}
	for _, phase := range phases {
		t.Run(phase.name, func(t *testing.T) {
			var canvas gfx.Canvas
			editor := text.NewEditor("ab cd")
			placement := &handledCaretPlacement{}
			field := gfx.Item(1)
			handled := phase.kind
			config := Config{
				Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
					canvas.HitWithOptions(field, gfx.Rect{Width: 100, Height: 40}, "source", gfx.HitOptions{
						Role: gfx.RoleTextField, Multiline: true,
						HideNativeOverlays: true, Placement: placement,
					})
				},
				TextInput: EditorInputs(func(id gfx.ID) (*text.Editor, bool) { return editor, id == field }),
				OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
					if event.Kind != handled {
						return gfx.EventResult{}
					}
					editor.SetSelection(text.Range{Start: 3, End: 5})
					return gfx.EventResult{Handled: true, Repaint: true}
				},
			}
			painter := newPainter(&canvas, &config)
			painter.width, painter.height = 100, 40
			painter.repaint()
			if phase.kind != gfx.PointerDown {
				painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 5, Y: 10})
			}
			if phase.kind == gfx.PointerUp {
				painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 20, Y: 10})
			}
			calls := placement.calls
			painter.pointerEvent(gfx.PointerEvent{
				Kind: phase.kind, Button: gfx.PointerButtonPrimary, X: 40, Y: 10,
			})
			want := text.Range{Start: 3, End: 5}
			if got := editor.Selection(); got != want {
				t.Fatalf("handled %s replaced the custom selection: got %+v, want %+v", phase.name, got, want)
			}
			if placement.calls != calls {
				t.Fatalf("handled %s invoked default placement: calls %d -> %d", phase.name, calls, placement.calls)
			}
			// The handled event ended any default drag, so a later unhandled
			// move has nothing to resume and must leave the app's selection.
			handled = 0
			painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 50, Y: 10})
			painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 50, Y: 10})
			if got := editor.Selection(); got != want {
				t.Fatalf("a later unhandled move resumed the default drag: got %+v, want %+v", got, want)
			}
		})
	}
}
