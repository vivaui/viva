package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestQuestionnairePaintsChoiceTypesInputErrorAndActions(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	questionnaire := BeginQuestionnaire(&canvas, theme, gfx.Rect{Width: 340, Height: 440}, QuestionnaireData{Progress: "2 of 4", Title: "Choose features", Description: "Select all that apply", ChoiceType: QuestionnaireCheckbox})
	questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(1), Label: "Analytics", Description: "Usage dashboards", Shortcut: "A", Checked: true})
	questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(2), Label: "Exports", Disabled: true})
	questionnaire.Input(gfx.Item(3), "", "Something else", false)
	questionnaire.Error("Choose at least one")
	questionnaire.PaintActions(QuestionnaireActions{PreviousID: gfx.Item(4), PreviousLabel: "Previous", SkipID: gfx.Item(5), SkipLabel: "Skip", NextID: gfx.Item(6), NextLabel: "Next"})
	if len(canvas.Hits()) != 5 {
		t.Fatalf("questionnaire hits = %+v", canvas.Hits())
	}
	if canvas.Texts()[0].Value != "2 of 4" || canvas.Texts()[len(canvas.Texts())-4].Style.Color != theme.Danger {
		t.Fatalf("questionnaire metadata = %+v", canvas.Texts())
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("checked checkbox omitted its check mark")
	}
}

func TestQuestionnaireCheckMarkIsAnIconNotRectangles(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(typeOf QuestionnaireChoiceType, checked bool) *gfx.Canvas {
		var canvas gfx.Canvas
		questionnaire := BeginQuestionnaire(&canvas, theme, gfx.Rect{Width: 300, Height: 200}, QuestionnaireData{ChoiceType: typeOf})
		questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(1), Label: "One", Checked: checked})
		return &canvas
	}
	if got := len(paint(QuestionnaireCheckbox, false).Triangles()); got != 0 {
		t.Fatalf("an unchecked checkbox drew %d triangles", got)
	}
	checked := paint(QuestionnaireCheckbox, true)
	if len(checked.Triangles()) == 0 {
		t.Fatal("a checked checkbox has no check mark")
	}
	solid := false
	for _, rectangle := range checked.Rectangles() {
		if rectangle.Color == theme.Primary {
			solid = true
		}
	}
	if !solid {
		t.Fatal("a checked indicator is not solid primary")
	}
}

func TestQuestionnaireChoicesAreSpacedAndStopAtTheBounds(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	questionnaire := BeginQuestionnaire(&canvas, theme, gfx.Rect{Width: 300, Height: 130}, QuestionnaireData{ChoiceType: QuestionnaireRadio})
	for index := range 4 {
		questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(index + 1), Label: "Choice"})
	}
	hits := canvas.Hits()
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want the two choices that fit in 130px", len(hits))
	}
	if hits[1].Bounds.Y <= hits[0].Bounds.Y+hits[0].Bounds.Height {
		t.Fatalf("choices touch: %+v then %+v", hits[0].Bounds, hits[1].Bounds)
	}
	if hits[0].Bounds.Height < 44 {
		t.Fatalf("a choice is %v tall, under the official 44px minimum", hits[0].Bounds.Height)
	}
}

func TestQuestionnaireInvalidItemMarksChoicesDestructive(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	questionnaire := BeginQuestionnaire(&canvas, theme, gfx.Rect{Width: 300, Height: 200}, QuestionnaireData{ChoiceType: QuestionnaireRadio, Invalid: true})
	questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(1), Label: "One"})
	found := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == theme.Danger {
			found = true
		}
	}
	if !found {
		t.Fatal("an invalid item has no destructive choice border")
	}
}

func TestQuestionnaireCommandStreamIsStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		questionnaire := BeginQuestionnaire(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 300, Height: 210}, QuestionnaireData{Title: "Question", ChoiceType: QuestionnaireRadio})
		questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(1), Label: "One"})
		questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(2), Label: "Two", Checked: true})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical questionnaires produced different command streams")
	}
}
