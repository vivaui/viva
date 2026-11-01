package components

import (
	"github.com/vivaui/viva/ui"
	"strings"
	"testing"
)

func TestCodeEditorPreservesTextPlacementSelectionAndComposition(t *testing.T) {
	var plain, highlighted ui.Canvas
	theme := ui.LightTheme()
	bounds := ui.Rect{Width: 240, Height: 120}
	value := "key: 中文\n# comment\nnext: true"
	var layout TextareaLayout
	snapshot := TextareaSnapshot{ID: ui.Item(1), Value: value, Focused: true, ShowCaret: true, Caret: len("key: 中"), SelectionStart: 0, SelectionEnd: 3, CompositionStart: 5, CompositionEnd: len("key: 中文"), Layout: &layout}
	PaintTextarea(&plain, theme, bounds, snapshot)
	editor := CodeEditor{}
	calls := 0
	editor.Paint(&highlighted, theme, bounds, CodeEditorSnapshot{TextareaSnapshot: snapshot, Language: "yaml", Highlight: func(language, line string, dst []CodeEditorRun) []CodeEditorRun {
		calls++
		if language != "yaml" {
			t.Fatal(language)
		}
		return append(dst, CodeEditorRun{Start: -2, End: 3, Color: theme.Danger}, CodeEditorRun{Start: len(line) + 10, End: len(line) + 20, Color: theme.Primary})
	}})
	if calls == 0 {
		t.Fatal("no highlighted lines")
	}
	if plain.Hits()[0].Value != highlighted.Hits()[0].Value || highlighted.Hits()[0].Value != value {
		t.Fatal("highlight changed source")
	}
	a, ok := layout.CaretBounds()
	b, ok2 := editor.Layout.CaretBounds()
	if !ok || !ok2 || a != b {
		t.Fatal("highlight moved native caret", a, b)
	}
	for _, point := range []ui.Point{{X: 18, Y: 15}, {X: 150, Y: 38}} {
		a, ok := layout.OffsetAt(point)
		b, ok2 := editor.Layout.OffsetAt(point)
		if a != b || ok != ok2 {
			t.Fatal("highlight changed pointer placement")
		}
	}
}

func TestCodeEditorHidesPlatformOverlaysWhenItPaintsText(t *testing.T) {
	theme := ui.LightTheme()
	value := "first\nsecond\n"
	var layout TextareaLayout
	var canvas ui.Canvas
	var editor CodeEditor
	editor.Paint(&canvas, theme, ui.Rect{Width: 320, Height: 160}, CodeEditorSnapshot{
		TextareaSnapshot: TextareaSnapshot{
			ID: ui.Item(1), Value: value, Focused: true, ShowCaret: true,
			HideEditorText: true, Caret: 2, SelectionStart: 0, SelectionEnd: 5,
			Layout: &layout,
		},
	})
	if hits := canvas.Hits(); len(hits) != 1 || !hits[0].HideNativeOverlays {
		t.Fatalf("self-painted editor hit = %+v, want native overlays hidden", hits)
	}
}

func BenchmarkCodeEditorVisibleHighlighting(b *testing.B) {
	var canvas ui.Canvas
	var editor CodeEditor
	snapshot := CodeEditorSnapshot{TextareaSnapshot: TextareaSnapshot{ID: ui.Item(1), Value: strings.Repeat("key: value\n", 10000)}, Highlight: func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{End: 3, Color: ui.RGB(200, 20, 20)})
	}}
	editor.Paint(&canvas, ui.LightTheme(), ui.Rect{Width: 800, Height: 600}, snapshot)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		editor.Paint(&canvas, ui.LightTheme(), ui.Rect{Width: 800, Height: 600}, snapshot)
	}
}

func TestCodeEditorScrollbarDragKeepsSelectionAndAllocatesNothingWhenSteady(t *testing.T) {
	var canvas ui.Canvas
	var editor CodeEditor
	snapshot := CodeEditorSnapshot{TextareaSnapshot: TextareaSnapshot{ID: ui.Item(1), Value: strings.Repeat("key: 中文\n", 100), Caret: 0}, ScrollbarID: ui.Item(2), Highlight: func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{End: 3, Color: ui.RGB(200, 20, 20)})
	}}
	paint := func() {
		canvas.Reset()
		editor.Paint(&canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 200}, snapshot)
	}
	paint()
	if editor.bar.MaxOffsetY <= 0 {
		t.Fatal("missing scrollbar")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("steady editor allocations: %g", allocations)
	}
	result := editor.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Target: ui.Item(2), Y: editor.bar.VerticalTrack.Y + editor.bar.VerticalTrack.Height + 100})
	if !result.Handled || editor.Layout.scroll != editor.bar.MaxOffsetY {
		t.Fatal("scrollbar did not reach bottom")
	}
	paint()
	if editor.Layout.scroll == 0 {
		t.Fatal("paint undid user scroll")
	}
	editor.Pointer(ui.PointerEvent{Kind: ui.PointerUp})
	if editor.dragging {
		t.Fatal("scrollbar retained pointer")
	}
}
