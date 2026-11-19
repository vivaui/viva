// Package questionnaire is the opt-in page control for the questionnaire
// item of package components: a progress line, a question title and
// description, single or multiple choice answers, and Back, Skip and Continue
// actions. The application owns the visible labels, the selection and what each
// action does:
//
//	question := questionnaire.New().
//		Progress("Question 2 of 5").
//		Title("What should Viva optimize first?").
//		Description("Answers stay in application state.").
//		Choice(m.choiceIDs[0], "Low memory", m.answer == 0, m.chooseLow).
//		Choice(m.choiceIDs[1], "Fast startup", m.answer == 1, m.chooseStartup).
//		Choice(m.choiceIDs[2], "Small binaries", m.answer == 2, m.chooseBinaries).
//		Back(m.previousID, "Back", m.back).
//		Skip(m.skipID, "Skip", m.skip).
//		Continue(m.nextID, "Continue", m.next)
//	question.View(c, "State").FillWidth()
//
// Choices and actions are chained; the view paints exactly the Checked values
// the caller supplies and calls the caller's callback after a click, so the
// application is the only owner of the answer state and the navigation. Single
// and Multiple choose the radio or checkbox indicator; there is no second
// constructor.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a State in the model: Reset keeps the choice
// storage, so re-declaring the same answers every paint allocates nothing.
// Do not copy a State after it has been declared.
package questionnaire

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Choice is one answer option: the caller's identity, visible labels and
// selected state, and the callback run after a click.
type Choice struct {
	ID          gfx.ID
	Label       string
	Description string
	Shortcut    string
	Checked     bool
	Disabled    bool
	OnSelect    func()
}

// action is one footer action.
type action struct {
	id       gfx.ID
	label    string
	onClick  func()
	disabled bool
}

// State is one questionnaire item and its page control.
type State struct {
	progress    string
	title       string
	description string
	multiple    bool
	invalid     bool
	choices     []Choice
	back        action
	skip        action
	next        action

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
}

// New returns an empty single-choice question.
func New() *State { return new(State) }

// Reset clears the text, choices and actions, keeping the choice storage so a
// page that rebuilds the same question every paint does not allocate.
func (q *State) Reset() *State {
	q.progress, q.title, q.description = "", "", ""
	q.multiple, q.invalid = false, false
	q.choices = q.choices[:0]
	q.back, q.skip, q.next = action{}, action{}, action{}
	return q
}

// Progress is the caller-formatted line above the question, such as
// "Question 2 of 5".
func (q *State) Progress(text string) *State {
	q.progress = text
	return q
}

// Title is the question.
func (q *State) Title(text string) *State {
	q.title = text
	return q
}

// Description is the muted line under the title.
func (q *State) Description(text string) *State {
	q.description = text
	return q
}

// Single selects the radio indicator, the default.
func (q *State) Single() *State {
	q.multiple = false
	return q
}

// Multiple selects the checkbox indicator.
func (q *State) Multiple() *State {
	q.multiple = true
	return q
}

// Invalid paints the question and its choices as failing validation.
func (q *State) Invalid() *State {
	q.invalid = true
	return q
}

// Choice adds one answer. The view paints Checked as given and runs onSelect,
// which may be nil, after the click is read.
func (q *State) Choice(id gfx.ID, label string, checked bool, onSelect func()) *State {
	if len(q.choices) == cap(q.choices) {
		q.choices = append(q.choices, Choice{})
	} else {
		q.choices = q.choices[:len(q.choices)+1]
		q.choices[len(q.choices)-1] = Choice{}
	}
	choice := &q.choices[len(q.choices)-1]
	choice.ID, choice.Label, choice.Checked, choice.OnSelect = id, label, checked, onSelect
	return q
}

// Detail sets the description and shortcut badge of the choice added last.
func (q *State) Detail(description, shortcut string) *State {
	if len(q.choices) == 0 {
		return q
	}
	choice := &q.choices[len(q.choices)-1]
	choice.Description, choice.Shortcut = description, shortcut
	return q
}

// DisableChoice paints the choice added last inert.
func (q *State) DisableChoice() *State {
	if len(q.choices) > 0 {
		q.choices[len(q.choices)-1].Disabled = true
	}
	return q
}

// Back adds the previous action at the start. An empty label omits it.
func (q *State) Back(id gfx.ID, label string, onClick func()) *State {
	q.back = action{id: id, label: label, onClick: onClick}
	return q
}

// Skip adds the skip action next to Continue. An empty label omits it.
func (q *State) Skip(id gfx.ID, label string, onClick func()) *State {
	q.skip = action{id: id, label: label, onClick: onClick}
	return q
}

// Continue adds the primary action at the end. An empty label omits it.
func (q *State) Continue(id gfx.ID, label string, onClick func()) *State {
	q.next = action{id: id, label: label, onClick: onClick}
	return q
}

// DisableContinue paints the Continue action inert.
func (q *State) DisableContinue() *State {
	q.next.disabled = true
	return q
}

// View declares the question at c. label names the group for assistive
// technology. Give it a width and the height the item may use; choices and
// actions that do not fit are not painted, as in the component.
//
// Declare the view on its own, not inside a Memo.
func (q *State) View(c ui.C, label string) ui.Box {
	if q == nil {
		panic("questionnaire: View needs its State")
	}
	q.prepare()
	return ui.Control(c, label, &q.handlers).Draw(q.drawFn)
}

func (q *State) prepare() {
	if q.prepared {
		return
	}
	q.prepared = true
	q.handlers.Role = gfx.RoleGroup
	q.drawFn = q.draw
}

func (q *State) draw(d *ui.DrawContext) {
	choiceType := components.QuestionnaireRadio
	if q.multiple {
		choiceType = components.QuestionnaireCheckbox
	}
	item := components.BeginQuestionnaire(d.Canvas, *d.Theme, d.Rect, components.QuestionnaireData{
		Progress: q.progress, Title: q.title, Description: q.description,
		ChoiceType: choiceType, Invalid: q.invalid,
	})
	for index := range q.choices {
		choice := &q.choices[index]
		item.AddChoice(components.QuestionnaireChoice{
			ID: choice.ID, Label: choice.Label, Description: choice.Description,
			Shortcut: choice.Shortcut, Checked: choice.Checked, Disabled: choice.Disabled,
		})
	}
	item.PaintActions(components.QuestionnaireActions{
		PreviousID: q.back.id, PreviousLabel: q.back.label,
		SkipID: q.skip.id, SkipLabel: q.skip.label,
		NextID: q.next.id, NextLabel: q.next.label, NextDisabled: q.next.disabled,
	})
	for index := range q.choices {
		choice := &q.choices[index]
		if !choice.ID.IsZero() && !choice.Disabled && d.Canvas.Clicked(choice.ID) && choice.OnSelect != nil {
			choice.OnSelect()
		}
	}
	q.notify(q.back, d)
	q.notify(q.skip, d)
	q.notify(q.next, d)
}

func (q *State) notify(item action, d *ui.DrawContext) {
	if item.label == "" || item.id.IsZero() || item.disabled {
		return
	}
	if d.Canvas.Clicked(item.id) && item.onClick != nil {
		item.onClick()
	}
}
