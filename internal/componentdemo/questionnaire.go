package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/questionnaireview"
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
}`, PageSnippet: `question := questionnaireview.New().
    Progress("Question 2 of 5").
    Title("What should Viva optimize first?").
    Description("Answers stay in application state.").
    Single().
    Choice(m.choiceIDs[0], "Low memory", m.answer == 0, m.choose[0]).
    Choice(m.choiceIDs[1], "Fast startup", m.answer == 1, m.choose[1]).
    Choice(m.choiceIDs[2], "Small binaries", m.answer == 2, m.choose[2]).
    Back(m.previousID, "Back", m.back).
    Skip(m.skipID, "Skip", m.skip).
    Continue(m.nextID, "Continue", m.next)
question.View(c, "Questionnaire").FillWidth()`, PageSnippetZh: `question := questionnaireview.New().
    Progress("第 2 题，共 5 题").
    Title("Viva 应优先优化什么？").
    Description("答案保留在应用状态中。").
    Single().
    Choice(m.choiceIDs[0], "低内存", m.answer == 0, m.choose[0]).
    Choice(m.choiceIDs[1], "快速启动", m.answer == 1, m.choose[1]).
    Choice(m.choiceIDs[2], "小体积", m.answer == 2, m.choose[2]).
    Back(m.previousID, "返回", m.back).
    Skip(m.skipID, "跳过", m.skip).
    Continue(m.nextID, "继续", m.next)
question.View(c, "问卷").FillWidth()`},
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
}`, PageSnippet: `question := questionnaireview.New().
    Progress("Question 2 of 5").
    Title("Which performance goals matter?").
    Description("Answers stay in application state.").
    Multiple().
    Choice(m.choiceIDs[0], "Low memory", m.memory, m.toggle[0]).
    Choice(m.choiceIDs[1], "Fast startup", m.startup, m.toggle[1]).
    Choice(m.choiceIDs[2], "Small binaries", m.binaries, m.toggle[2]).
    Back(m.previousID, "Back", m.back).
    Skip(m.skipID, "Skip", m.skip).
    Continue(m.nextID, "Continue", m.next)
question.View(c, "Questionnaire").FillWidth()`, PageSnippetZh: `question := questionnaireview.New().
    Progress("第 2 题，共 5 题").
    Title("哪些性能目标重要？").
    Description("答案保留在应用状态中。").
    Multiple().
    Choice(m.choiceIDs[0], "低内存", m.memory, m.toggle[0]).
    Choice(m.choiceIDs[1], "快速启动", m.startup, m.toggle[1]).
    Choice(m.choiceIDs[2], "小体积", m.binaries, m.toggle[2]).
    Back(m.previousID, "返回", m.back).
    Skip(m.skipID, "跳过", m.skip).
    Continue(m.nextID, "继续", m.next)
question.View(c, "问卷").FillWidth()`},
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

// questionnaireDemo is one example's page control and its stable callbacks. It
// is kept beside the State rather than rebuilt in each paint, so the handlers
// are stable and a warm re-declaration allocates nothing.
type questionnaireDemo struct {
	view   questionnaireview.Questionnaire
	choose [3]func()
	toggle [3]func()
	back   func()
	skip   func()
	next   func()
}

// questionnaireDemos is keyed by State because the constrained demo model has
// no field for this control; a State is painted on one thread at a time.
var questionnaireDemos = map[*State]*questionnaireDemo{}

func (state *State) initializeQuestionnaire() {
	if state.exampleID == "single-choice" {
		state.values[0] = 0
	}
	demo := &questionnaireDemo{}
	for index := range demo.choose {
		value := index
		demo.choose[index] = func() { state.values[0] = int32(value) }
		demo.toggle[index] = func() { state.flags ^= uint64(1) << value }
	}
	demo.back = func() { state.values[1]-- }
	demo.skip = func() { state.values[1]++ }
	demo.next = func() { state.values[1]++ }
	questionnaireDemos[state] = demo
}

func questionnaireDemoFor(state *State) *questionnaireDemo {
	if demo := questionnaireDemos[state]; demo != nil {
		return demo
	}
	demo := &questionnaireDemo{}
	for index := range demo.choose {
		value := index
		demo.choose[index] = func() { state.values[0] = int32(value) }
		demo.toggle[index] = func() { state.flags ^= uint64(1) << value }
	}
	demo.back = func() { state.values[1]-- }
	demo.skip = func() { state.values[1]++ }
	demo.next = func() { state.values[1]++ }
	questionnaireDemos[state] = demo
	return demo
}

func (state *State) paintQuestionnaire(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	multiple := state.exampleID == "multiple-choice"
	demo := questionnaireDemoFor(state)
	height := max(size.Height-48-36, 0)
	title := "What should Viva optimize first?"
	description := "Answers stay in application state."
	if multiple {
		title = "Which performance goals matter?"
	}
	if height < 280 {
		description = "" // a short card drops its description to keep the actions
	}
	labels := [...]string{"Low memory", "Fast startup", "Small binaries"}
	// A narrow card keeps its two first answers so the action row still fits;
	// the component drops what it cannot place, but the demo chooses the pair.
	count := len(labels)
	if min(size.Width-2*theme.Metrics.PadSurface, 560) < 360 {
		count = 2
	}
	state.drawPage(canvas, theme, size, 560, "Questionnaire", func(c ui.C) {
		question := &demo.view
		question.Reset().
			Progress("Question 2 of 5").
			Title(title).
			Description(description)
		if multiple {
			question.Multiple()
		} else {
			question.Single()
		}
		for index := 0; index < count; index++ {
			label := labels[index]
			checked := state.values[0] == int32(index)
			if multiple {
				checked = state.flags&(uint64(1)<<index) != 0
			}
			onSelect := demo.choose[index]
			if multiple {
				onSelect = demo.toggle[index]
			}
			question.Choice(questionnaireChoice.At(index), label, checked, onSelect).
				Detail("", questionnaireChoiceShortcut(index))
		}
		question.Back(questionnairePrevious, "Back", demo.back).
			Skip(questionnaireSkip, "Skip", demo.skip).
			Continue(questionnaireNext, "Continue", demo.next)
		question.View(c, "Questionnaire").FillWidth().Height(height)
	})
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
