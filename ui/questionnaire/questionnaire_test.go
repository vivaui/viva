package questionnaire_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/questionnaire"
)

var (
	choiceID   = gfx.Item(101)
	secondID   = gfx.Item(102)
	thirdID    = gfx.Item(103)
	previousID = gfx.Item(104)
	skipID     = gfx.Item(105)
	nextID     = gfx.Item(106)
)

func testTheme() gfx.Theme { return gfx.LightTheme() }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 560, Height: 520}, view)
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
	question                 questionnaire.Questionnaire
	selected                 int
	backs, skips, nexts      int
	choose                   [3]func()
	back, skip, continueNext func()
}

func (m *model) init() {
	for index := range m.choose {
		value := index
		m.choose[index] = func() { m.selected = value }
	}
	m.back = func() { m.backs++ }
	m.skip = func() { m.skips++ }
	m.continueNext = func() { m.nexts++ }
}

func (m *model) rebuild() {
	m.question.Reset().
		Progress("Question 2 of 5").
		Title("What should Viva optimize first?").
		Description("Answers stay in application state.").
		Single().
		Choice(choiceID, "Low memory", m.selected == 0, m.choose[0]).
		Choice(secondID, "Fast startup", m.selected == 1, m.choose[1]).
		Choice(thirdID, "Small binaries", m.selected == 2, m.choose[2]).
		Back(previousID, "Back", m.back).
		Skip(skipID, "Skip", m.skip).
		Continue(nextID, "Continue", m.continueNext)
}

func (m *model) view(c ui.C) {
	m.rebuild()
	m.question.View(c, "Questionnaire").Height(480)
}

// The question paints the caller's progress, title, description, labels and
// actions, and a choice's callback runs after its click is read.
func TestQuestionPaintsAndCallsBack(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	for _, value := range []string{
		"Question 2 of 5", "What should Viva optimize first?",
		"Answers stay in application state.", "Low memory", "Fast startup",
		"Small binaries", "Back", "Skip", "Continue",
	} {
		if !hasText(canvas, value) {
			t.Fatalf("%q was not painted", value)
		}
	}
	choice := hit(t, canvas, "Fast startup")
	if choice.ID != secondID || choice.Role != gfx.RoleRadio {
		t.Fatalf("choice = %+v, want the caller's radio identity", choice)
	}

	drawClick(page, canvas, m.view, secondID)
	if m.selected != 1 {
		t.Fatalf("selection = %d after the click, want 1", m.selected)
	}
	drawClick(page, canvas, m.view, nextID)
	if m.nexts != 1 {
		t.Fatalf("Continue ran %d times, want 1", m.nexts)
	}
}

// Multiple choices paint checkbox indicators, and every action has its own
// callback.
func TestMultipleChoicesAndActions(t *testing.T) {
	m := new(model)
	m.init()
	view := func(c ui.C) {
		m.question.Reset().
			Title("Which performance goals matter?").
			Multiple().
			Choice(choiceID, "Low memory", true, nil).
			Choice(secondID, "Fast startup", false, nil).
			Back(previousID, "Back", func() { m.backs++ }).
			Skip(skipID, "Skip", func() { m.skips++ }).
			Continue(nextID, "Continue", func() { m.nexts++ })
		m.question.View(c, "Questionnaire").Height(400)
	}
	page, canvas := open(t, view)
	choice := hit(t, canvas, "Low memory")
	if choice.Role != gfx.RoleCheckbox || !choice.Checked {
		t.Fatalf("choice = %+v, want a checked checkbox", choice)
	}
	drawClick(page, canvas, view, previousID)
	drawClick(page, canvas, view, skipID)
	if m.backs != 1 || m.skips != 1 {
		t.Fatalf("Back ran %d times and Skip %d, want 1 each", m.backs, m.skips)
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
