package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type handledCaretPlacement struct{ calls int }

func (placement *handledCaretPlacement) OffsetAt(point gfx.Point) (int, bool) {
	placement.calls++
	return int(point.X / 10), true
}

func (*handledCaretPlacement) CaretBounds() (gfx.Rect, bool) {
	return gfx.Rect{Width: 1, Height: 20}, true
}

type handledCaretFixture struct {
	painter   *painter
	editor    *text.Editor
	placement handledCaretPlacement
	handled   gfx.PointerEventKind
	paints    int
	writes    int
}

func newHandledCaretFixture(t testing.TB, events bool) *handledCaretFixture {
	t.Helper()
	fixture := &handledCaretFixture{editor: text.NewEditor("ab cd\nef")}
	field := gfx.Item(7)
	config := Config{Title: "caret ownership", Width: 120, Height: 40}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		fixture.paints++
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.HitWithOptions(field, bounds, "Text", gfx.HitOptions{
			Role: gfx.RoleTextField, TextBounds: bounds, Autofocus: true, Placement: &fixture.placement,
		})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		if !events {
			return &TextInput{Editor: fixture.editor}
		}
		return &TextInput{State: TextStateOf(fixture.editor), OnEvent: func(event TextEvent) (TextState, error) {
			fixture.writes++
			event.Apply(fixture.editor)
			return TextStateOf(fixture.editor), nil
		}}
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		if event.Kind != fixture.handled {
			return gfx.EventResult{}
		}
		fixture.editor.SetSelection(text.Range{Start: 6, End: 8})
		return gfx.EventResult{Handled: true, Repaint: true, Focus: field}
	}
	var canvas gfx.Canvas
	fixture.painter = newTestPainter(t, &canvas, config)
	return fixture
}

func (fixture *handledCaretFixture) pointer(t testing.TB, kind platform.PointerEventKind, x float32, clicks int) {
	t.Helper()
	_, err := fixture.painter.pointerEvent(platform.PointerEvent{
		Kind: kind, X: x, Y: 10, Button: gfx.PointerButtonPrimary, ClickCount: clicks,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandledPointerOwnsCaretPlacement(t *testing.T) {
	for _, events := range []bool{false, true} {
		provider := "editor"
		if events {
			provider = "events"
		}
		t.Run(provider, func(t *testing.T) {
			for _, phase := range []struct {
				name string
				kind platform.PointerEventKind
			}{{"press", platform.PointerDown}, {"move", platform.PointerMove}, {"release", platform.PointerUp}} {
				t.Run(phase.name, func(t *testing.T) {
					fixture := newHandledCaretFixture(t, events)
					if phase.kind != platform.PointerDown {
						fixture.pointer(t, platform.PointerDown, 10, 1)
					}
					if phase.kind == platform.PointerUp {
						fixture.pointer(t, platform.PointerMove, 30, 1)
					}
					fixture.handled = gfx.PointerEventKind(phase.kind)
					calls, writes, paints := fixture.placement.calls, fixture.writes, fixture.paints
					fixture.pointer(t, phase.kind, 40, 1)
					want := text.Range{Start: 6, End: 8}
					if got := fixture.editor.Selection(); got != want {
						t.Fatalf("handled %s replaced custom selection: got %+v, want %+v", phase.name, got, want)
					}
					if fixture.placement.calls != calls || fixture.writes != writes {
						t.Fatalf("handled %s invoked default placement: calls %d -> %d, writes %d -> %d", phase.name, calls, fixture.placement.calls, writes, fixture.writes)
					}
					if fixture.paints <= paints || fixture.painter.focus.Focused() != gfx.Item(7) {
						t.Fatal("handled pointer lost repaint or focus")
					}
					fixture.handled = 0
					fixture.pointer(t, platform.PointerMove, 50, 1)
					fixture.pointer(t, platform.PointerUp, 50, 1)
					if fixture.editor.Selection() != want || fixture.placement.calls != calls || fixture.writes != writes {
						t.Fatal("retired default drag resumed after takeover")
					}
					fixture.pointer(t, platform.PointerDown, 20, 1)
					if got := fixture.editor.Selection(); got != (text.Range{Start: 2, End: 2}) {
						t.Fatalf("new unhandled press = %+v, want caret 2", got)
					}
					fixture.pointer(t, platform.PointerMove, 40, 1)
					fixture.pointer(t, platform.PointerUp, 40, 1)
					if got := fixture.editor.Selection(); got != (text.Range{Start: 2, End: 4}) {
						t.Fatalf("new unhandled drag = %+v, want 2..4", got)
					}
					if fixture.editor.String() != "ab cd\nef" || fixture.editor.CanUndo() {
						t.Fatal("pointer selection changed text or history")
					}
				})
			}
		})
	}
}

func TestUnhandledPointerCaretPlacementControls(t *testing.T) {
	for _, events := range []bool{false, true} {
		provider := "editor"
		if events {
			provider = "events"
		}
		t.Run(provider, func(t *testing.T) {
			t.Run("drag", func(t *testing.T) {
				fixture := newHandledCaretFixture(t, events)
				fixture.pointer(t, platform.PointerDown, 10, 1)
				if got := fixture.editor.Selection(); got != (text.Range{Start: 1, End: 1}) {
					t.Fatalf("ordinary press = %+v, want caret 1", got)
				}
				fixture.pointer(t, platform.PointerMove, 30, 1)
				fixture.pointer(t, platform.PointerUp, 30, 1)
				if got := fixture.editor.Selection(); got != (text.Range{Start: 1, End: 3}) {
					t.Fatalf("ordinary drag = %+v, want 1..3", got)
				}
			})
			for _, click := range []struct {
				name  string
				count int
				want  text.Range
			}{{"word", 2, text.Range{Start: 0, End: 2}}, {"line", 3, text.Range{Start: 0, End: 6}}} {
				t.Run(click.name, func(t *testing.T) {
					fixture := newHandledCaretFixture(t, events)
					fixture.pointer(t, platform.PointerDown, 10, click.count)
					fixture.pointer(t, platform.PointerUp, 10, click.count)
					if got := fixture.editor.Selection(); got != click.want {
						t.Fatalf("%d-click selection = %+v, want %+v", click.count, got, click.want)
					}
				})
			}
		})
	}
}
