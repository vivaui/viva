package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

func TestInputGroupLaysOutExplicitSlotsAndSemantics(t *testing.T) {
	var canvas ui.Canvas
	layout := PaintInputGroup(&canvas, ui.LightTheme(), ui.Rect{X: 10, Y: 20, Width: 260}, InputGroupSnapshot{
		ID: ui.Item(1), Label: "Website", Value: "viva.dev", Hint: "Domain", Focused: true,
		InlineStart: InputGroupSlot{Kind: InputGroupSlotText, Text: "https://", Width: 58},
		InlineEnd:   InputGroupSlot{Kind: InputGroupSlotButton, ID: ui.Item(2), Text: "Copy", Label: "Copy URL", Width: 52},
		BlockStart:  InputGroupSlot{Kind: InputGroupSlotText, Text: "Public address", Separated: true},
	})
	if layout.Bounds.Height != 64 || layout.Control != (ui.Rect{X: 68, Y: 52, Width: 150, Height: 32}) {
		t.Fatalf("layout = %+v", layout)
	}
	if len(canvas.Hits()) != 2 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	if button := canvas.Hits()[0]; button.Role != ui.RoleButton || button.ID != ui.Item(2) {
		t.Fatalf("button = %+v", button)
	}
	if control := canvas.Hits()[1]; control.Role != ui.RoleTextField || control.Value != "viva.dev" || control.Bounds != layout.Control {
		t.Fatalf("control = %+v", control)
	}
}

func TestInputGroupTextareaState(t *testing.T) {
	var canvas ui.Canvas
	layout := PaintInputGroup(&canvas, ui.DarkTheme(), ui.Rect{Width: 200}, InputGroupSnapshot{
		ID: ui.Item(4), Label: "Message", Value: "hello\nworld", Multiline: true, ReadOnly: true,
		SelectionStart: 0, SelectionEnd: 5, CompositionStart: 6, CompositionEnd: 11,
		BlockEnd: InputGroupSlot{Kind: InputGroupSlotKbd, Text: "⌘↵", Separated: true},
	})
	if layout.Bounds.Height != 96 || layout.Control.Height != 64 {
		t.Fatalf("layout = %+v", layout)
	}
	if hit := canvas.Hits()[0]; !hit.Multiline || !hit.ReadOnly || hit.Role != ui.RoleTextField {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestInputGroupCanHideThePlatformEditorAndPaintItsOwnText(t *testing.T) {
	var canvas ui.Canvas
	PaintInputGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 220}, InputGroupSnapshot{
		ID: ui.Item(5), Label: "Domain", Value: "viva.dev", NativeEditing: true,
		HideEditorText: true, ShowCaret: true, Caret: 2,
	})
	hit := canvas.Hits()[0]
	if hit.TextColor.Alpha != 0 {
		t.Errorf("HideEditorText left the platform's text visible: %+v", hit.TextColor)
	}
	if !hit.HideNativeOverlays {
		t.Error("HideEditorText did not hide the platform caret and selection")
	}
	if len(canvas.Texts()) == 0 {
		t.Error("the component did not paint the text it hid from the platform")
	}
}

func TestInputGroupTypedSemanticsAndCommands(t *testing.T) {
	snapshot := InputGroupSnapshot{
		ID: ui.Item(6), Label: "Message", Value: "A中B", DisplayValue: "A中B", Hint: "Body",
		Focused: true, Invalid: true, Multiline: true,
		SelectionStart: 4, SelectionEnd: 2, CompositionStart: 1, CompositionEnd: 99,
	}
	want := InputGroupSemantics{
		ID: ui.Item(6), Label: "Message", Value: "A中B", Hint: "Body", Focused: true,
		Invalid: true, Multiline: true, SelectionStart: 1, SelectionEnd: 4,
		CompositionStart: 1, CompositionEnd: 5,
	}
	if got := snapshot.Semantics(); got != want {
		t.Fatalf("semantics = %+v, want %+v", got, want)
	}
	if command, handled := InputGroupCommandForKey(snapshot, input.KeyEnter); command != InputGroupCommandInsertNewline || !handled {
		t.Fatalf("multiline enter = (%v, %v)", command, handled)
	}
	for _, inert := range []InputGroupSnapshot{{}, {Multiline: true, Disabled: true}, {Multiline: true, ReadOnly: true}} {
		if command, handled := InputGroupCommandForKey(inert, input.KeyEnter); command != InputGroupCommandNone || handled {
			t.Fatalf("inert enter = (%v, %v)", command, handled)
		}
	}
	if allocations := testing.AllocsPerRun(100, func() {
		_ = snapshot.Semantics()
		InputGroupCommandForKey(snapshot, input.KeyEnter)
	}); allocations != 0 {
		t.Fatalf("input-group helpers allocations = %v", allocations)
	}
}

func TestInputGroupNativeEditingKeepsSlotsAndSemantics(t *testing.T) {
	var canvas ui.Canvas
	PaintInputGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 220}, InputGroupSnapshot{
		ID: ui.Item(8), Label: "Search", Value: "native", NativeEditing: true,
		InlineStart: InputGroupSlot{Kind: InputGroupSlotText, Text: "⌕", Width: 32},
	})
	if texts := canvas.Texts(); len(texts) != 1 || texts[0].Value != "⌕" {
		t.Fatalf("native editing texts = %+v", texts)
	}
	if hits := canvas.Hits(); len(hits) != 1 || hits[0].Value != "native" {
		t.Fatalf("native editing semantics = %+v", hits)
	}
}

func TestInputGroupPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		PaintInputGroup(&canvas, ui.LightTheme(), ui.Rect{X: 3, Y: 4, Width: 220}, InputGroupSnapshot{
			ID: ui.Item(1), Label: "Query", Value: "native", Focused: true, ShowCaret: true, Caret: 6,
			SelectionStart: 0, SelectionEnd: 2, CompositionStart: 2, CompositionEnd: 6,
			InlineStart: InputGroupSlot{Kind: InputGroupSlotText, Text: "⌕", Width: 32},
			InlineEnd:   InputGroupSlot{Kind: InputGroupSlotButton, ID: ui.Item(2), Text: "Clear", Width: 52},
		})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical input-group snapshots produced different streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("input-group paint allocations = %v", allocations)
	}
}

// A search field's magnifier is an icon slot: drawn in the muted color inside
// its slot, no hit of its own, and the control (with its declared native text
// geometry) starts after it.
func TestInputGroupIconSlotDrawsAnIconAndShiftsTheControl(t *testing.T) {
	icon := ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound,
		Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 4), ui.LineTo(20, 20)}}
	theme := ui.DarkTheme()
	bounds := ui.Rect{X: 10, Y: 20, Width: 300, Height: 40}
	var plain, withIcon ui.Canvas
	PaintInputGroup(&plain, theme, bounds, InputGroupSnapshot{ID: ui.Item(1), Label: "Search", Value: "x"})
	layout := PaintInputGroup(&withIcon, theme, bounds, InputGroupSnapshot{
		ID: ui.Item(1), Label: "Search", Value: "x", InlineStart: InputGroupSlot{Kind: InputGroupSlotIcon, Icon: icon},
	})

	if len(withIcon.Triangles()) == 0 || len(plain.Triangles()) != 0 {
		t.Fatalf("triangles with icon %d, without %d", len(withIcon.Triangles()), len(plain.Triangles()))
	}
	slot := layout.InlineStart
	if slot.Width <= 16 || slot.Width > 32 || slot.X != bounds.X || slot.Height != bounds.Height {
		t.Fatalf("icon slot = %+v, want it to hug the 16px icon at the group's left edge, full height", slot)
	}
	if layout.Control.X != bounds.X+slot.Width {
		t.Fatalf("control starts at %v, want right after the slot", layout.Control.X)
	}
	for _, triangle := range withIcon.Triangles() {
		if triangle.Color != theme.MutedForeground {
			t.Fatalf("icon color = %+v, want the muted foreground", triangle.Color)
		}
		for _, point := range triangle.Points {
			if point.X < slot.X || point.X > slot.X+slot.Width || point.Y < slot.Y || point.Y > slot.Y+slot.Height {
				t.Fatalf("icon point %+v lies outside its slot %+v", point, slot)
			}
		}
	}
	if len(withIcon.Hits()) != 1 || withIcon.Hits()[0].Role != ui.RoleTextField {
		t.Fatalf("hits = %+v, want only the text field", withIcon.Hits())
	}
	hit := withIcon.Hits()[0]
	if hit.TextBounds.X <= layout.Control.X || hit.TextBounds.X > layout.Control.X+12 || hit.TextSize != theme.Metrics.FontSizeM {
		t.Fatalf("native text geometry %+v / %v must follow the shifted control", hit.TextBounds, hit.TextSize)
	}

	// Disabled groups dim the icon like their other addons.
	var disabled ui.Canvas
	PaintInputGroup(&disabled, theme, bounds, InputGroupSnapshot{
		ID: ui.Item(1), Label: "Search", Disabled: true, InlineStart: InputGroupSlot{Kind: InputGroupSlotIcon, Icon: icon},
	})
	if got := disabled.Triangles()[0].Color.Alpha; got != theme.MutedForeground.Alpha*0.5 {
		t.Fatalf("disabled icon alpha = %v", got)
	}

	paint := func() {
		withIcon.Reset()
		PaintInputGroup(&withIcon, theme, bounds, InputGroupSnapshot{
			ID: ui.Item(1), Label: "Search", Value: "x", InlineStart: InputGroupSlot{Kind: InputGroupSlotIcon, Icon: icon},
		})
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("icon-slot group allocated %v times", allocations)
	}
}

// Without an addon the embedded control reads exactly like a standalone Input
// of the same width: the text starts at the same x, and a Secure group masks
// and hides its value the same way.
func TestInputGroupControlMatchesAStandaloneInput(t *testing.T) {
	theme := ui.LightTheme()
	bounds := ui.Rect{X: 5, Y: 6, Width: 200}
	var input, group ui.Canvas
	PaintInput(&input, theme, bounds, InputSnapshot{ID: ui.Item(1), Label: "Token", Value: "secret", Secure: true, Focused: true, ShowCaret: true, Caret: 3, SelectionStart: 1, SelectionEnd: 2})
	PaintInputGroup(&group, theme, bounds, InputGroupSnapshot{ID: ui.Item(1), Label: "Token", Value: "secret", Secure: true, Focused: true, ShowCaret: true, Caret: 3, SelectionStart: 1, SelectionEnd: 2})
	if input.Hits()[0].TextBounds != group.Hits()[0].TextBounds {
		t.Fatalf("group text bounds %+v, input %+v", group.Hits()[0].TextBounds, input.Hits()[0].TextBounds)
	}
	if !slices.Equal(input.Texts(), group.Texts()) || !slices.Equal(input.Rectangles(), group.Rectangles()) {
		t.Fatalf("group painted differently from the input:\ngroup %+v\ninput %+v", group.Rectangles(), input.Rectangles())
	}
	if hit := group.Hits()[0]; !hit.Secure || hit.Value != "" {
		t.Fatalf("secure group exposed %+v", hit)
	}
}

func TestInputGroupButtonsComeInTheShadcnSizes(t *testing.T) {
	theme := ui.LightTheme()
	icon := ui.Icon{ViewBox: 24, Stroke: 2, Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 20)}}
	hitOf := func(slot InputGroupSlot, bounds ui.Rect) ui.Hit {
		var canvas ui.Canvas
		slot.Kind, slot.ID = InputGroupSlotButton, ui.Item(2)
		PaintInputGroup(&canvas, theme, bounds, InputGroupSnapshot{ID: ui.Item(1), Label: "x", Value: "v", InlineEnd: slot})
		for _, hit := range canvas.Hits() {
			if hit.ID == ui.Item(2) {
				return hit
			}
		}
		t.Fatal("button slot declared no hit")
		return ui.Hit{}
	}
	tall := ui.Rect{Width: 240, Height: 40}
	iconXS := hitOf(InputGroupSlot{Icon: icon, Label: "Clear", ButtonSize: InputGroupButtonIconXS}, tall)
	if iconXS.Bounds.Width != 24 || iconXS.Bounds.Height != 24 || iconXS.Label != "Clear" {
		t.Fatalf("icon-xs button = %+v, want a 24px square named Clear", iconXS)
	}
	iconSM := hitOf(InputGroupSlot{Icon: icon, Label: "Reveal", ButtonSize: InputGroupButtonIconSM}, tall)
	if iconSM.Bounds.Width != 32 || iconSM.Bounds.Height != 32 {
		t.Fatalf("icon-sm button = %+v, want a 32px square", iconSM.Bounds)
	}
	// In a group of one control height there is not room for 32 inside the border.
	snug := hitOf(InputGroupSlot{Icon: icon, Label: "Reveal", ButtonSize: InputGroupButtonIconSM}, ui.Rect{Width: 240})
	if snug.Bounds.Height >= 32 || snug.Bounds.Height != snug.Bounds.Width {
		t.Fatalf("icon-sm button in a one-line group = %+v, want a smaller square inside the border", snug.Bounds)
	}
	xs := hitOf(InputGroupSlot{Text: "Copy"}, tall)
	sm := hitOf(InputGroupSlot{Text: "Copy", ButtonSize: InputGroupButtonSM}, tall)
	if xs.Bounds.Height != 24 || sm.Bounds.Height != 28 || sm.Bounds.Width <= xs.Bounds.Width {
		t.Fatalf("text buttons xs %+v, sm %+v", xs.Bounds, sm.Bounds)
	}
	both := hitOf(InputGroupSlot{Icon: icon, Text: "Copy"}, tall)
	if both.Bounds.Width <= xs.Bounds.Width {
		t.Fatalf("icon with text is %v wide, no wider than text alone (%v)", both.Bounds.Width, xs.Bounds.Width)
	}
}

// A caller can move the icon inside a button slot without moving the slot's hit
// region. This is what lets Motty reuse its icon-hover transform in a password
// field, whose reveal control lives inside InputGroup.
func TestInputGroupButtonIconTransformKeepsItsHitRegion(t *testing.T) {
	theme := ui.LightTheme()
	icon := ui.Icon{ViewBox: 24, Stroke: 2, Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 20)}}
	bounds := ui.Rect{Width: 240, Height: 40}
	paint := func(transform ui.IconTransform) (*ui.Canvas, InputGroupLayout) {
		var canvas ui.Canvas
		layout := PaintInputGroup(&canvas, theme, bounds, InputGroupSnapshot{
			ID: ui.Item(1), Label: "Password", Value: "secret", Secure: true, InlineEnd: InputGroupSlot{
				Kind: InputGroupSlotButton, ID: ui.Item(2), Icon: icon, Label: "Show password",
				ButtonSize: InputGroupButtonIconXS, IconTransform: transform,
			},
		})
		return &canvas, layout
	}
	stillCanvas, stillLayout := paint(ui.IconTransform{})
	movedCanvas, movedLayout := paint(ui.IconTransform{Translate: ui.Point{X: 2}})
	if stillLayout != movedLayout {
		t.Fatalf("the icon transform moved the layout from %+v to %+v", stillLayout, movedLayout)
	}
	var stillHit, movedHit ui.Hit
	for _, hit := range stillCanvas.Hits() {
		if hit.ID == ui.Item(2) {
			stillHit = hit
		}
	}
	for _, hit := range movedCanvas.Hits() {
		if hit.ID == ui.Item(2) {
			movedHit = hit
		}
	}
	if stillHit.Bounds != movedHit.Bounds {
		t.Fatalf("the icon transform moved the button hit from %+v to %+v", stillHit.Bounds, movedHit.Bounds)
	}
	if slices.Equal(stillCanvas.Triangles(), movedCanvas.Triangles()) {
		t.Fatal("the icon transform painted the icon in the same place")
	}
}

func TestInputGroupSpinnerSlotPaintsWithoutAHit(t *testing.T) {
	var canvas ui.Canvas
	layout := PaintInputGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 200}, InputGroupSnapshot{
		ID: ui.Item(1), Label: "Search", InlineEnd: InputGroupSlot{Kind: InputGroupSlotSpinner, Phase: 3},
	})
	if layout.InlineEnd.Width <= 16 {
		t.Fatalf("spinner slot = %+v", layout.InlineEnd)
	}
	if len(canvas.Hits()) != 1 {
		t.Fatalf("hits = %+v, want only the text field", canvas.Hits())
	}
	spokes := 0
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.X >= layout.InlineEnd.X && rectangle.Bounds.Width <= 16 {
			spokes++
		}
	}
	if spokes < 8 {
		t.Fatalf("only %d spinner shapes inside the slot", spokes)
	}
}
