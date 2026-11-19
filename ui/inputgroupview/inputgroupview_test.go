package inputgroupview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputgroupview"
	"github.com/vivaui/viva/window"
)

var (
	clearID  = gfx.Item(91)
	testIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{
		gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
)

func testTheme() gfx.Theme { return gfx.LightTheme() }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 560, Height: 240}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := testTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, view)
	return page, canvas
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].Label == label {
			return hits[index]
		}
	}
	t.Fatalf("no hit labelled %q was painted", label)
	return gfx.Hit{}
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, label := range canvas.Texts() {
		if label.Value == value {
			return true
		}
	}
	return false
}

type model struct {
	editor  *text.Editor
	field   ui.Field
	group   inputgroupview.Group
	cleared int
}

func (m *model) init() {
	m.editor = text.NewEditor("viva.dev")
	m.field.Bind(m.editor)
	m.group = *inputgroupview.New(&m.field).
		Placeholder("project-name").
		Prefix("https://").
		Action(clearID, testIcon, "Clear domain", func() { m.cleared++ })
}

func (m *model) view(c ui.C) {
	m.group.View(c, "Project domain").FillWidth()
}

// The group paints the caller's field text and addons, and the action's click
// runs the caller's callback and is read after View.
func TestGroupPaintsFieldAndRunsAction(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)

	field := hit(t, canvas, "Project domain")
	if field.ID.IsZero() || field.Role != gfx.RoleTextField {
		t.Fatalf("field hit = %+v, want the control's text field", field)
	}
	if !hasText(canvas, "viva.dev") || !hasText(canvas, "https://") {
		t.Fatal("the field's text or prefix was not painted")
	}
	action := hit(t, canvas, "Clear domain")
	if action.ID != clearID || action.Role != gfx.RoleButton {
		t.Fatalf("action hit = %+v, want the caller's button", action)
	}

	input := page.TextInput(window.TextInputContext{ID: field.ID})
	if input == nil || input.Editor != m.editor {
		t.Fatalf("the platform text service got %+v, want the field's editor", input)
	}

	drawClick(page, canvas, m.view, clearID)
	if m.cleared != 1 {
		t.Fatalf("the action callback ran %d times, want 1", m.cleared)
	}
	if !m.group.ActionClicked() {
		t.Fatal("ActionClicked did not report the action")
	}
	draw(page, canvas, m.view)
	if m.group.ActionClicked() {
		t.Fatal("ActionClicked reported a click in an ordinary paint")
	}
}

// The chained options place every addon the component knows: a suffix, a
// button, block addons and a spinner.
func TestGroupAddons(t *testing.T) {
	var field ui.Field
	field.SetText("viva.dev")
	var group inputgroupview.Group
	group = *inputgroupview.New(&field).Suffix("@viva.dev")
	page, canvas := open(t, func(c ui.C) {
		group.View(c, "Project domain").FillWidth()
	})
	_ = page
	if !hasText(canvas, "@viva.dev") {
		t.Fatal("the suffix was not painted")
	}

	buttonID := gfx.Item(92)
	group = *inputgroupview.New(&field).
		BlockStart("Release description").
		BlockKbd("⌘↵").
		Button(buttonID, "Invite", nil).
		Small().
		DisableAction()
	_, canvas = open(t, func(c ui.C) {
		group.View(c, "Release description").FillWidth()
	})
	if !hasText(canvas, "Release description") || !hasText(canvas, "⌘↵") {
		t.Fatal("the block addons were not painted")
	}
	if invite := hit(t, canvas, "Invite"); invite.ID != buttonID || invite.Disabled == false {
		t.Fatalf("button hit = %+v, want the disabled caller's button", invite)
	}

	group = *inputgroupview.New(&field).Prefix("Search").Spinner(2)
	_, canvas = open(t, func(c ui.C) {
		group.View(c, "Search").FillWidth()
	})
	if !hasText(canvas, "Search") || hasText(canvas, "Clear domain") {
		t.Fatal("the spinner did not replace the action")
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, m.view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.init()
	page, canvas := open(b, m.view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, m.view)
	}
}
