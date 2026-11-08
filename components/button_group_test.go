package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

var buttonGroupTestOptions = [...]ButtonGroupOption{
	{ID: ui.Item(61), Label: "Save", Variant: ButtonDefault},
	{ID: ui.Item(62), Label: "Preview", Variant: ButtonOutline},
	{ID: ui.Item(63), Label: "More", Variant: ButtonGhost, Disabled: true},
}

func TestButtonGroupPaintsVariantsAndDisabledAction(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	BeginButtonGroup(&canvas, theme, ui.Rect{Width: 270}, buttonGroupTestOptions[:]).Paint()
	if !hasRectangleColor(&canvas, theme.Primary) {
		t.Fatalf("no primary action fill in %+v", canvas.Rectangles())
	}
	if len(canvas.Hits()) != 3 || canvas.Hits()[0].Role != ui.RoleButton || canvas.Hits()[1].Label != "Preview" || !canvas.Hits()[2].Disabled {
		t.Fatalf("button group hits = %+v", canvas.Hits())
	}
}

func TestButtonGroupStableAndAllocationFree(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		BeginButtonGroup(&canvas, theme, ui.Rect{Width: 270}, buttonGroupTestOptions[:]).Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical button groups produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("button group paint allocations = %v", allocations)
	}
}

func TestButtonGroupVerticalStacksActions(t *testing.T) {
	var canvas ui.Canvas
	BeginButtonGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 120}, buttonGroupTestOptions[:]).Vertical().Paint()
	if len(canvas.Hits()) != 3 || canvas.Hits()[1].Bounds.Y <= canvas.Hits()[0].Bounds.Y {
		t.Fatalf("vertical button hits = %+v", canvas.Hits())
	}
	if canvas.Hits()[0].Bounds.Height != 32 {
		t.Fatalf("vertical button height = %v", canvas.Hits()[0].Bounds.Height)
	}
}

// A group takes every variant's colors from the same recipe as a Button, so a
// destructive or link action is not quietly drawn as a plain outline one.
func TestButtonGroupKeepsEveryVariantsOwnColors(t *testing.T) {
	theme := ui.LightTheme()
	options := [...]ButtonGroupOption{
		{ID: ui.Item(81), Label: "Delete", Variant: ButtonDestructive},
		{ID: ui.Item(82), Label: "Docs", Variant: ButtonLink},
		{ID: ui.Item(83), Label: "Undo", Variant: ButtonDestructiveOutline},
	}
	var canvas ui.Canvas
	BeginButtonGroup(&canvas, theme, ui.Rect{Width: 270}, options[:]).Paint()
	for index, option := range options {
		want := buttonStyleFor(&theme, option.Variant, false, false).foreground
		if got := canvas.Texts()[index].Style.Color; got != want {
			t.Fatalf("%s label color = %+v, want %+v", option.Label, got, want)
		}
	}
	if want := buttonStyleFor(&theme, ButtonDestructive, false, false).background; !hasRectangleColor(&canvas, want) {
		t.Fatalf("no destructive action fill %+v in %+v", want, canvas.Rectangles())
	}
}

// The end items are filled rounded and clipped back to their own square side, so
// a solid action at the end of a group does not show square corners past the
// group's rounded outline.
func TestButtonGroupEndItemsAreRoundedOnlyOutside(t *testing.T) {
	options := [...]ButtonGroupOption{
		{ID: ui.Item(61), Label: "Save", Variant: ButtonDefault},
		{ID: ui.Item(62), Label: "Preview", Variant: ButtonSecondary},
		{ID: ui.Item(63), Label: "More", Variant: ButtonSecondary},
	}
	var canvas ui.Canvas
	BeginButtonGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 270}, options[:]).Paint()
	rectangles := canvas.Rectangles()
	// Fills and the hairlines between them interleave: fill, fill, line, fill, line.
	first, middle, last := rectangles[0], rectangles[1], rectangles[3]
	if first.Radius == 0 || !first.HasClip || first.ClipOf != canvas.Hits()[0].Bounds {
		t.Fatalf("first item = %+v, want a rounded fill clipped to its own bounds", first)
	}
	if middle.Radius != 0 || middle.HasClip {
		t.Fatalf("middle item = %+v, want a square fill", middle)
	}
	if last.Radius == 0 || !last.HasClip || last.ClipOf != canvas.Hits()[2].Bounds {
		t.Fatalf("last item = %+v, want a rounded fill clipped to its own bounds", last)
	}
}

func TestButtonGroupClickedReportsTheClickedAction(t *testing.T) {
	var canvas ui.Canvas
	canvas.BeginEventPass(ui.Item(62))
	defer canvas.EndEventPass()
	id, ok := BeginButtonGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 270}, buttonGroupTestOptions[:]).Paint().Clicked()
	if !ok || id != ui.Item(62) {
		t.Fatalf("Clicked() = (%v, %v), want the Preview action", id, ok)
	}
}

func hasRectangleColor(canvas *ui.Canvas, color ui.Color) bool {
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color {
			return true
		}
	}
	return false
}
