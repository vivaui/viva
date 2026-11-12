package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var questionnaireComponent = Component{
	ID: "questionnaire", Title: "Questionnaire", Category: "Forms", Summary: "由调用方持有答案与导航状态的问卷题目。",
	paint: (*State).paintQuestionnaire, initialize: (*State).initializeQuestionnaire,
	Examples: []Example{
		{ID: "single-choice", Title: "单选题", Description: "选择一个桌面渲染后端并继续。", Snippet: `choice := tree.Sub(page, tree.Name("choice"))
q := components.BeginQuestionnaire(canvas, theme, bounds, components.QuestionnaireData{
    Progress: "Question 2 of 5", Title: "What should Viva optimize first?",
    Description: "Answers stay in application state.",
})
for index, label := range []string{"Low memory", "Fast startup", "Small binaries"} {
    q.AddChoice(components.QuestionnaireChoice{
        ID: choice.At(index), Label: label, Checked: selected == index,
    })
}
q.PaintActions(components.QuestionnaireActions{
    PreviousID: tree.ID(page, "previous"), PreviousLabel: "Back",
    SkipID: tree.ID(page, "skip"), SkipLabel: "Skip",
    NextID: tree.ID(page, "next"), NextLabel: "Continue",
})
if index, ok := canvas.ClickedIn(choice); ok {
    selected = index
}`},
		{ID: "multiple-choice", Title: "多选题", Description: "切换多个性能目标，组件不隐藏答案状态。", Snippet: `choice := tree.Sub(page, tree.Name("choice"))
q := components.BeginQuestionnaire(canvas, theme, bounds, components.QuestionnaireData{
    Progress: "Question 2 of 5", Title: "Which performance goals matter?",
    Description: "Answers stay in application state.",
    ChoiceType: components.QuestionnaireCheckbox,
})
for index, label := range []string{"Low memory", "Fast startup", "Small binaries"} {
    q.AddChoice(components.QuestionnaireChoice{
        ID: choice.At(index), Label: label, Checked: checked[index],
    })
}
if index, ok := canvas.ClickedIn(choice); ok {
    checked[index] = !checked[index]
}`},
	},
}

var (
	questionnaireUI = tree.Feature("questionnaire")
	// questionnaireChoice's item i is the i-th answer.
	questionnaireChoice   = tree.Sub(questionnaireUI, tree.Name("choice"))
	questionnairePrevious = tree.ID(questionnaireUI, "previous")
	questionnaireSkip     = tree.ID(questionnaireUI, "skip")
	questionnaireNext     = tree.ID(questionnaireUI, "next")
)

func (state *State) initializeQuestionnaire() {
	if state.exampleID == "single-choice" {
		state.values[0] = 0
	}
}

func (state *State) paintQuestionnaire(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	multiple := state.exampleID == "multiple-choice"
	data := components.QuestionnaireData{
		Progress: "Question 2 of 5", Title: "What should Viva optimize first?",
		Description: "Answers stay in application state.", ChoiceType: components.QuestionnaireRadio,
	}
	if multiple {
		data.Title = "Which performance goals matter?"
		data.ChoiceType = components.QuestionnaireCheckbox
	}
	var card ui.Box
	state.drawPage(canvas, theme, size, 560, "Questionnaire", func(c ui.C) {
		// The question fills what the page leaves: its own choices and actions
		// decide what fits (a narrow card drops its last choice).
		card = ui.Slot(c).Height(max(size.Height-48-36, 0))
	})
	bounds := card.Rect()
	if bounds.Height < 280 {
		data.Description = "" // a short card drops its description to keep the actions
	}
	questionnaire := components.BeginQuestionnaire(canvas, theme, bounds, data)
	labels := [...]string{"Low memory", "Fast startup", "Small binaries"}
	for index, label := range labels {
		checked := state.values[0] == int32(index)
		if multiple {
			checked = state.flags&(uint64(1)<<index) != 0
		}
		questionnaire.AddChoice(components.QuestionnaireChoice{
			ID: questionnaireChoice.At(index), Label: label, Shortcut: questionnaireChoiceShortcut(index), Checked: checked,
		})
		if bounds.Width < 360 && index == 1 {
			break
		}
	}
	questionnaire.PaintActions(components.QuestionnaireActions{
		PreviousID: questionnairePrevious, PreviousLabel: "Back",
		SkipID: questionnaireSkip, SkipLabel: "Skip",
		NextID: questionnaireNext, NextLabel: "Continue",
	})

	if index, ok := canvas.ClickedIn(questionnaireChoice); ok {
		if multiple {
			state.flags ^= uint64(1) << index
		} else {
			state.values[0] = int32(index)
		}
	}
	switch {
	case canvas.Clicked(questionnairePrevious):
		state.values[1]--
	case canvas.Clicked(questionnaireSkip), canvas.Clicked(questionnaireNext):
		state.values[1]++
	}
}

func questionnaireChoiceShortcut(index int) string {
	switch index {
	case 0:
		return "1"
	case 1:
		return "2"
	default:
		return "3"
	}
}
