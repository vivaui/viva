package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestPainterReentrantTextEventKeepsLatestAcknowledgment(t *testing.T) {
	for _, mode := range []string{"nested edit", "ignored newline"} {
		t.Run(mode, func(t *testing.T) {
			editor := text.NewEditor("a")
			editor.MoveTo(1, false)
			owner := &painter{nativeText: platform.TextInput{Multiline: mode == "nested edit"}}
			owner.nativeText.State = platform.EditorState(editor)
			owner.activeText = &TextInput{State: TextStateOf(editor)}
			owner.activeText.OnEvent = func(event TextEvent) (TextState, error) {
				event.Apply(editor)
				ack := TextStateOf(editor)
				if event.Text == "r" {
					nested := platform.TextEvent{Kind: platform.TextInsert, Text: "z"}
					if mode == "ignored newline" {
						nested = platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextInsertNewline}
					}
					if _, err := owner.applyTextEvent(nested); err != nil {
						t.Fatal(err)
					}
				}
				return ack, nil
			}
			ack, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "r"})
			want := TextStateOf(editor)
			wantValue := "arz"
			if mode == "ignored newline" {
				wantValue = "ar"
			}
			if err != nil || want.Value != wantValue || owner.activeText.State != want ||
				owner.nativeText.State != platform.EditorState(editor) || ack.WithoutRedraw() != platform.EditorState(editor) {
				t.Fatalf("nested text acknowledgment: editor=%+v active=%+v native=%+v ack=%+v err=%v", want, owner.activeText.State, owner.nativeText.State, ack, err)
			}
		})
	}
}

func TestPainterTextCallbackSyncPreservesInputOwnership(t *testing.T) {
	for _, mode := range []string{"same owner fresh provider", "same owner geometry", "replacement owner", "identical direct replacement", "removed owner"} {
		t.Run(mode, func(t *testing.T) {
			first, second := text.NewEditor("a"), text.NewEditor("other")
			first.MoveTo(1, false)
			if mode == "identical direct replacement" {
				second = text.NewEditor("a")
				second.MoveTo(1, false)
			}
			current := first
			extraWidth := float32(0)
			var canvas gfx.Canvas
			config := Config{Title: "reentrant provider", Width: 200, Height: 80}
			config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
				canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 24}, "Name", gfx.HitOptions{Role: gfx.RoleTextField})
			}
			var owner *painter
			config.TextInput = func(context TextInputContext) *TextInput {
				if current == nil {
					return nil
				}
				editor := current
				bounds := context.Bounds
				bounds.Width += extraWidth
				if mode == "identical direct replacement" && editor == second {
					return &TextInput{Bounds: bounds, FontSize: 13, Editor: editor}
				}
				return &TextInput{Bounds: bounds, FontSize: 13, State: TextStateOf(editor), OnEvent: func(event TextEvent) (TextState, error) {
					if mode == "replacement owner" || mode == "identical direct replacement" || mode == "removed owner" {
						event.Apply(editor)
						current = second
						if mode == "removed owner" {
							current = nil
						}
						owner.syncTextInput()
					} else {
						if mode == "same owner geometry" {
							extraWidth = 10
						}
						owner.syncTextInput()
						event.Apply(editor)
					}
					ack := TextStateOf(editor)
					ack.Redraw = true
					return ack, nil
				}}
			}
			owner = newTestPainter(t, &canvas, config)
			for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
				if _, err := owner.pointerEvent(platform.PointerEvent{Kind: kind, X: 10, Y: 10}); err != nil {
					t.Fatal(err)
				}
			}
			ack, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "r"})
			if mode == "removed owner" {
				if err != nil || first.String() != "ar" || owner.activeText != nil || owner.textEnabled ||
					ack.WithoutRedraw() != (platform.TextState{}) || !ack.Redraw {
					t.Fatalf("removed provider acknowledgment: active=%+v enabled=%t ack=%+v error=%v", owner.activeText, owner.textEnabled, ack, err)
				}
				return
			}
			want := TextStateOf(current)
			if err != nil || first.String() != "ar" || owner.activeText.State != want ||
				owner.nativeText.State != platform.EditorState(current) || ack.WithoutRedraw() != platform.EditorState(current) || !ack.Redraw {
				t.Fatalf("provider acknowledgment: editor=%+v active=%+v native=%+v ack=%+v err=%v", want, owner.activeText.State, owner.nativeText.State, ack, err)
			}
		})
	}
}
