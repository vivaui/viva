package app

import (
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// hitGatePage is a small page whose one label and bounds come from variables,
// so a test can change what the paint declares without changing its shape.
type hitGatePage struct {
	label  string
	bounds ui.Rect
}

func (page *hitGatePage) paint(canvas *ui.Canvas, _ ui.Size) {
	canvas.HitWithLabel(ui.Item(1), page.bounds, page.label)
}

func TestRepaintReusesRegionsAndSemanticsWhenTheHitsAreUnchanged(t *testing.T) {
	page := &hitGatePage{label: "Save", bounds: ui.Rect{Width: 80, Height: 20}}
	config := Config{Title: "gate", Width: 200, Height: 80, Paint: page.paint}
	var canvas ui.Canvas
	painter := newTestPainter(t, &canvas, config)
	reused := painter.reusedHits

	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if painter.reusedHits != reused+1 {
		t.Fatalf("reused hits = %d, want %d", painter.reusedHits, reused+1)
	}
	if frame.Accessibility != nil {
		t.Error("an unchanged repaint republished the accessibility payload")
	}
	if got := painter.accessibility[0].Label; got != "Save" {
		t.Fatalf("label = %q", got)
	}
}

func TestRepaintPublishesASemanticChangeAfterTheGateOpens(t *testing.T) {
	page := &hitGatePage{label: "Save", bounds: ui.Rect{Width: 80, Height: 20}}
	config := Config{Title: "gate", Width: 200, Height: 80, Paint: page.paint}
	var canvas ui.Canvas
	painter := newTestPainter(t, &canvas, config)
	reused := painter.reusedHits

	page.label = "Save and close"
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if painter.reusedHits != reused {
		t.Fatal("a changed label was treated as an unchanged hit list")
	}
	if got := painter.accessibility[0].Label; got != "Save and close" {
		t.Fatalf("label = %q", got)
	}
	if frame.Accessibility == nil {
		t.Fatal("the changed label did not reach the accessibility payload")
	}
}

func TestRepaintPublishesABoundsChangeAfterTheGateOpens(t *testing.T) {
	page := &hitGatePage{label: "Save", bounds: ui.Rect{Width: 80, Height: 20}}
	config := Config{Title: "gate", Width: 200, Height: 80, Paint: page.paint}
	var canvas ui.Canvas
	painter := newTestPainter(t, &canvas, config)

	page.bounds = ui.Rect{X: 100, Width: 80, Height: 20}
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if target, ok := painter.focus.Router().HitTest(140, 10); !ok || target != ui.Item(1) {
		t.Fatalf("new bounds not routed: %v, %v", target, ok)
	}
	if _, ok := painter.focus.Router().HitTest(10, 10); ok {
		t.Fatal("old bounds still routed")
	}
}

func TestRepaintPublishesAFocusChangeAfterTheGateOpens(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "gate", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{Width: 80, Height: 20}, "A")
		canvas.HitWithLabel(ui.Item(2), ui.Rect{Y: 30, Width: 80, Height: 20}, "B")
	}
	painter := newTestPainter(t, &canvas, config)
	if painter.accessibility[0].Focused || painter.accessibility[1].Focused {
		t.Fatalf("unexpected initial focus: %+v", painter.accessibility)
	}

	frame, err := painter.key(platform.KeyEvent{Key: input.KeyTab})
	if err != nil {
		t.Fatal(err)
	}
	if painter.focus.Focused().IsZero() {
		t.Fatal("Tab did not focus anything")
	}
	if frame.Accessibility == nil {
		t.Fatal("a focus change did not republish accessibility")
	}
	focused := false
	for _, node := range painter.accessibility {
		if node.Focused && node.ID == painter.focus.Focused() {
			focused = true
		}
	}
	if !focused {
		t.Fatal("no node is focused after Tab")
	}
}

func TestRepaintPublishesATextSelectionChangeAfterTheGateOpens(t *testing.T) {
	editor := text.NewEditor("hello")
	var canvas ui.Canvas
	config := Config{Title: "gate", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 150, Height: 24}, "Name", ui.HitOptions{
			Role: ui.RoleTextField, Value: editor.String(), Autofocus: true,
		})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{Editor: editor, Bounds: ui.Rect{Width: 150, Height: 24}, FontSize: 13}
	}
	painter := newTestPainter(t, &canvas, config)
	if painter.activeText == nil {
		t.Fatal("the text field did not take the keyboard")
	}
	if selection := painter.accessibility[0].Selection; !selection.Valid || selection.Start != 5 || selection.End != 5 {
		t.Fatalf("initial selection = %+v", selection)
	}
	var out presented
	if ok, err := painter.accessibilityRequest(platform.AccessibilityRequest{
		Node: ui.Item(1), Action: platform.AccessibilitySetSelection, Start: 1, End: 3,
	}, out.present); !ok || err != nil {
		t.Fatalf("set selection = %v, %v", ok, err)
	}
	if selection := painter.accessibility[0].Selection; !selection.Valid || selection.Start != 1 || selection.End != 3 {
		t.Fatalf("selection = %+v", selection)
	}
	last := out.frames[len(out.frames)-1]
	if last.Accessibility == nil {
		t.Fatal("a selection change did not republish accessibility")
	}
}
