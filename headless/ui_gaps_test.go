//go:build darwin || windows || linux

package headless

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// A hit that asks for the grabbing cursor shows it while the pointer is over
// it, end to end: HitOptions.Cursor through the painter into the driver the
// same gesture carries to a real window's backend.
func TestHeadlessGrabbingHitIsThePointer(t *testing.T) {
	session, err := Open(app.Config{Title: "cursor", Width: 80, Height: 60, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 60, Height: 40}, "drag", gfx.HitOptions{Cursor: gfx.PointerCursorGrabbing})
	}}, nil, Options{ReferenceText: true})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Move(30, 20); err != nil {
		t.Fatal(err)
	}
	if cursor := session.Snapshot().Cursor; cursor != app.CursorGrabbing {
		t.Fatalf("headless cursor = %v, want grabbing", cursor)
	}
}

// An InputSnapshot that aligns its text to the right carries that through the
// whole stack: the static line is painted right-aligned, the hit declares it,
// and the native editor the focused field is given resolves to AlignRight and
// keeps the line box the component declared, so the text does not move when
// the field gains focus.
func TestInputSnapshotRightAlignmentReachesTheHeadlessEditor(t *testing.T) {
	editor := text.NewEditor("1234")
	bounds := gfx.Rect{X: 20, Y: 20, Width: 220, Height: 32}
	hittest := func() (hits []gfx.Hit) { return }
	_ = hittest
	config := app.Config{
		Title: "aligned number", Width: 260, Height: 80,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			components.PaintInput(canvas, gfx.LightTheme(), bounds, components.InputSnapshot{ID: gfx.Item(1), Label: "Amount", Value: editor.String(), Alignment: gfx.AlignRight})
		},
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if context.ID != gfx.Item(1) {
				t.Fatalf("text input is for %v, want the field", context.ID)
			}
			if context.Alignment != gfx.AlignRight {
				t.Fatalf("the declaration reached the provider as %v, want right", context.Alignment)
			}
			return &app.TextInput{Editor: editor}
		},
	}
	session, err := Open(config, nil, Options{ReferenceText: true})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Static frame: the line is right-aligned in the content box, and says so.
	snapshot := session.Snapshot()
	var textBounds gfx.Rect
	for i, hit := range snapshot.Hits {
		if hit.ID == gfx.Item(1) {
			if declared, ok := snapshot.HitTextAt(i); ok {
				textBounds = declared.Bounds
				if declared.Alignment != gfx.AlignRight {
					t.Fatalf("the declared alignment is %v, want right", declared.Alignment)
				}
			}
		}
	}
	if textBounds == (gfx.Rect{}) {
		t.Fatal("the field declared no text geometry")
	}

	if err := session.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	snapshot = session.Snapshot()
	if snapshot.Editor == nil {
		t.Fatal("the click did not focus the field")
	}
	if snapshot.Editor.Alignment != gfx.AlignRight {
		t.Fatalf("the native editor alignment is %v, want right", snapshot.Editor.Alignment)
	}
	// The editor sits where the component said the text is: the line box inside
	// the content rectangle, so the glyphs platform rendering draws are exactly
	// where the painted ones were.
	if snapshot.Editor.Bounds != textBounds {
		t.Fatalf("editor bounds %+v, want the declared text box %+v", snapshot.Editor.Bounds, textBounds)
	}
}
