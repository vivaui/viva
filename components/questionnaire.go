package components

import "github.com/vivaui/viva/gfx"

// QuestionnaireChoiceType selects radio or checkbox indicator semantics.
type QuestionnaireChoiceType uint8

const (
	QuestionnaireRadio QuestionnaireChoiceType = iota
	QuestionnaireCheckbox
)

// QuestionnaireData contains root/item metadata. Progress is a caller-formatted
// tabular label such as "2 of 5"; no question index is stored internally.
type QuestionnaireData struct {
	Progress    string
	Title       string
	Description string
	ChoiceType  QuestionnaireChoiceType
	// Invalid marks the item as failing validation: its choices and input get
	// the destructive border (the official item invalid state).
	Invalid bool
}

// QuestionnaireChoice is one explicit answer state.
type QuestionnaireChoice struct {
	ID          gfx.ID
	Label       string
	Description string
	Shortcut    string
	Checked     bool
	Disabled    bool
}

// QuestionnaireActions contains the official previous, skip, and next/submit
// slots. Empty labels omit a slot; IDs of zero paint preview-only buttons.
type QuestionnaireActions struct {
	PreviousID    gfx.ID
	PreviousLabel string
	SkipID        gfx.ID
	SkipLabel     string
	NextID        gfx.ID
	NextLabel     string
	NextDisabled  bool
}

// Questionnaire incrementally paints one item without owning navigation,
// validation, checked values, or input text.
type Questionnaire struct {
	canvas  *gfx.Canvas
	theme   gfx.Theme
	bounds  gfx.Rect
	typeOf  QuestionnaireChoiceType
	invalid bool
	y       float32
	scale   float32
	prev    questionnairePart
}

// questionnairePart is what was painted last, which decides the gap before the next part:
// the official root, item and choice list each space their children differently.
type questionnairePart uint8

const (
	questionnaireIntro questionnairePart = iota
	questionnaireChoice
	questionnaireInput
	questionnaireError
	questionnaireActionRow
)

// gap is the space before a part of the given kind: gap-3 between choices, gap-5 between
// the parts of an item and gap-6 before the action row.
func (questionnaire Questionnaire) gap(next questionnairePart) float32 {
	switch {
	case next == questionnaireActionRow:
		return 24 * questionnaire.scale
	case next == questionnaireChoice && questionnaire.prev == questionnaireChoice:
		return 12 * questionnaire.scale
	}
	return 20 * questionnaire.scale
}

// place reserves a part height tall below what is painted so far and returns its top.
// It reports false, reserving nothing, when the part would not fit in the bounds.
func (questionnaire *Questionnaire) place(part questionnairePart, height float32) (float32, bool) {
	y := questionnaire.y + questionnaire.gap(part)
	if height <= 0 || y+height > questionnaire.bounds.Y+questionnaire.bounds.Height {
		return 0, false
	}
	questionnaire.y = y + height
	questionnaire.prev = part
	return y, true
}

// BeginQuestionnaire paints progress, title, and description slots.
func BeginQuestionnaire(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, data QuestionnaireData) Questionnaire {
	metrics := theme.Metrics
	scale := metrics.ControlHeight / 32
	y := bounds.Y
	if data.Progress != "" {
		canvas.Label(data.Progress, gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: metrics.FontSizeS + 5}, gfx.TextStyle{
			Size: metrics.FontSizeS, Weight: gfx.WeightMedium, Color: theme.MutedForeground,
		})
		y += metrics.FontSizeS + 5
		if data.Title != "" || data.Description != "" {
			y += 24 * scale // the root's gap-6
		}
	}
	if data.Title != "" {
		height := (metrics.FontSizeM + 2) * 1.5
		canvas.Label(data.Title, gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: height}, gfx.TextStyle{
			Size: metrics.FontSizeM + 2, Weight: gfx.WeightSemibold, Color: theme.Foreground,
		})
		y += height
		if data.Description != "" {
			y += 20 * scale // the item's gap-5
		}
	}
	if data.Description != "" {
		height := metrics.FontSizeM * 1.5
		canvas.Label(data.Description, gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: height}, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: theme.MutedForeground,
		})
		y += height
	}
	return Questionnaire{
		canvas: canvas, theme: theme, bounds: bounds, typeOf: data.ChoiceType,
		invalid: data.Invalid, y: y, scale: scale, prev: questionnaireIntro,
	}
}

// AddChoice paints one selectable answer and its full-surface hit region.
func (questionnaire *Questionnaire) AddChoice(choice QuestionnaireChoice) {
	theme := questionnaire.theme
	metrics := theme.Metrics
	scale := questionnaire.scale
	canvas := questionnaire.canvas
	padX, padY := metrics.PadSurface, 14*scale
	lineHeight := metrics.FontSizeM * 1.375 // the label's leading-snug
	textHeight := lineHeight
	if choice.Description != "" {
		textHeight += 4*scale + lineHeight
	}
	height := max(textHeight+padY*2, 44*scale)
	y, ok := questionnaire.place(questionnaireChoice, height)
	if !ok {
		return
	}
	bounds := gfx.Rect{X: questionnaire.bounds.X, Y: y, Width: questionnaire.bounds.Width, Height: height}
	interactive := !choice.ID.IsZero() && !choice.Disabled
	dark := theme.Background.Red < 0.5
	focused := interactive && canvas.FocusVisible() == choice.ID

	// Fill and border are flattened onto the page: a translucent border would show
	// through the interior, since the border is painted as a backing shape.
	fill := theme.Background
	if dark {
		fill = theme.Input
		fill.Alpha *= 0.20
	} else {
		fill.Alpha = 0
	}
	border := theme.Input
	switch {
	case choice.Checked:
		fill = theme.Muted
		border = theme.Primary
		border.Alpha *= 0.40
	case interactive && canvas.Pressed() == choice.ID:
		fill = theme.Muted
	case interactive && canvas.Hovered() == choice.ID:
		fill = theme.Muted
		fill.Alpha *= 0.50
	}
	if questionnaire.invalid {
		border = theme.Danger
	}
	if focused {
		border = theme.Ring
	}
	foreground := theme.Foreground
	secondary := theme.MutedForeground
	dim := float32(1)
	if choice.Disabled {
		dim = 0.50
		foreground.Alpha *= dim
		secondary.Alpha *= dim
		fill.Alpha *= dim
		border.Alpha *= dim
	}
	radius := metrics.RadiusControl
	if focused {
		ring := theme.Ring
		ring.Alpha *= 0.50
		canvas.Rounded(expand(bounds, 3), radius+3, ring)
	}
	canvas.RoundedBorder(bounds, radius, 1, border.Over(theme.Background), fill.Over(theme.Background))

	// The indicator is a solid primary box once checked, with the primary-foreground dot
	// (radio) or check (checkbox) in it; unchecked it is an outline.
	indicatorSize := CheckboxExtent(theme)
	indicator := gfx.Rect{X: bounds.X + padX, Y: bounds.Y + padY + (lineHeight-indicatorSize)/2, Width: indicatorSize, Height: indicatorSize}
	indicatorRadius := 4 * scale
	if questionnaire.typeOf == QuestionnaireRadio {
		indicatorRadius = indicatorSize / 2
	}
	indicatorFill := theme.Background
	indicatorFill.Alpha = 0
	if dark {
		indicatorFill = theme.Input
		indicatorFill.Alpha *= 0.30
	}
	indicatorBorder := theme.Input
	if choice.Checked {
		indicatorFill, indicatorBorder = theme.Primary, theme.Primary
	}
	indicatorFill.Alpha *= dim
	indicatorBorder.Alpha *= dim
	canvas.RoundedBorder(indicator, indicatorRadius, 1, indicatorBorder.Over(theme.Background), indicatorFill.Over(theme.Background))
	if choice.Checked {
		mark := theme.PrimaryForeground
		mark.Alpha *= dim
		if questionnaire.typeOf == QuestionnaireRadio {
			dot := 8 * scale
			canvas.Rounded(gfx.Rect{X: indicator.X + (indicator.Width-dot)/2, Y: indicator.Y + (indicator.Height-dot)/2, Width: dot, Height: dot}, dot/2, mark)
		} else {
			paintCheck(canvas, indicator, 14*scale, mark)
		}
	}

	// The shortcut badge sits at the far end, level with the first line.
	textRight := bounds.X + bounds.Width - padX
	if choice.Shortcut != "" {
		badge := 20 * scale
		textRight -= badge + 12*scale
		questionnaire.paintShortcut(choice.Shortcut, gfx.Rect{X: bounds.X + bounds.Width - padX - badge, Y: bounds.Y + padY + (lineHeight-badge)/2, Width: badge, Height: badge}, dim)
	}
	textX := indicator.X + indicator.Width + 12*scale
	textWidth := max(textRight-textX, 0)
	canvas.Label(choice.Label, gfx.Rect{X: textX, Y: bounds.Y + padY, Width: textWidth, Height: lineHeight}, gfx.TextStyle{
		Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground,
	})
	if choice.Description != "" {
		canvas.Label(choice.Description, gfx.Rect{X: textX, Y: bounds.Y + padY + lineHeight + 4*scale, Width: textWidth, Height: lineHeight}, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: secondary,
		})
	}
	if interactive {
		role := gfx.RoleRadio
		if questionnaire.typeOf == QuestionnaireCheckbox {
			role = gfx.RoleCheckbox
		}
		canvas.HitWithOptions(choice.ID, bounds, choice.Label, gfx.HitOptions{Role: role, Checked: choice.Checked})
	}
}

// paintShortcut draws the keyboard-shortcut badge: a bordered box with a 10px
// monospaced key, the official size-5 rounded-md border font-mono.
func (questionnaire Questionnaire) paintShortcut(key string, bounds gfx.Rect, dim float32) {
	theme := questionnaire.theme
	border, text := theme.Input, theme.MutedForeground
	border.Alpha *= dim
	text.Alpha *= dim
	questionnaire.canvas.RoundedBorder(bounds, theme.Metrics.RadiusControl, 1, border.Over(theme.Background), theme.Background)
	questionnaire.canvas.LabelCenter(key, bounds, gfx.TextStyle{Size: theme.Metrics.FontSizeS - 1, Weight: gfx.WeightMedium, Color: text, Mono: true})
}

// Input paints the questionnaire free-text slot. It exposes a hit only; text
// editing remains in the platform text subsystem rather than hidden here.
func (questionnaire *Questionnaire) Input(id gfx.ID, value, placeholder string, disabled bool) {
	metrics := questionnaire.theme.Metrics
	y, ok := questionnaire.place(questionnaireInput, metrics.ControlHeight)
	if !ok {
		return
	}
	bounds := gfx.Rect{X: questionnaire.bounds.X, Y: y, Width: questionnaire.bounds.Width, Height: metrics.ControlHeight}
	foreground := questionnaire.theme.Foreground
	text := value
	if text == "" {
		text = placeholder
		foreground = questionnaire.theme.MutedForeground
	}
	interactive := !id.IsZero() && !disabled
	if disabled {
		foreground.Alpha *= 0.50
	}
	focused := interactive && questionnaire.canvas.FocusVisible() == id
	paintTextEntrySurface(questionnaire.canvas, &questionnaire.theme, bounds, gfx.State{Focused: focused, Invalid: questionnaire.invalid, Disabled: disabled})
	questionnaire.canvas.Label(text, gfx.Rect{X: bounds.X + metrics.PadControlX, Y: bounds.Y, Width: max(bounds.Width-metrics.PadControlX*2, 0), Height: bounds.Height}, gfx.TextStyle{
		Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground,
	})
	if interactive {
		questionnaire.canvas.HitWithLabel(id, bounds, "Questionnaire input")
	}
}

// Error paints the official destructive error slot.
func (questionnaire *Questionnaire) Error(text string) {
	if text == "" {
		return
	}
	size := questionnaire.theme.Metrics.FontSizeM
	y, ok := questionnaire.place(questionnaireError, size*1.5)
	if !ok {
		return
	}
	questionnaire.canvas.Label(text, gfx.Rect{X: questionnaire.bounds.X, Y: y, Width: questionnaire.bounds.Width, Height: size * 1.5}, gfx.TextStyle{
		Size: size, Weight: gfx.WeightRegular, Color: questionnaire.theme.Danger,
	})
}

// PaintActions paints previous at start and skip/next at end, matching the
// official three-column action arrangement.
func (questionnaire *Questionnaire) PaintActions(actions QuestionnaireActions) {
	metrics := questionnaire.theme.Metrics
	height := metrics.ControlHeight
	y, ok := questionnaire.place(questionnaireActionRow, height)
	if !ok {
		return
	}
	row := gfx.Rect{X: questionnaire.bounds.X, Y: y, Width: questionnaire.bounds.Width, Height: height}
	if actions.PreviousLabel != "" {
		width := questionnaire.actionWidth(actions.PreviousLabel)
		questionnaire.paintAction(actions.PreviousID, gfx.Rect{X: row.X, Y: row.Y, Width: width, Height: height}, actions.PreviousLabel, false, false)
	}
	nextWidth := questionnaire.actionWidth(actions.NextLabel)
	nextX := row.X + row.Width - nextWidth
	if actions.NextLabel != "" {
		questionnaire.paintAction(actions.NextID, gfx.Rect{X: nextX, Y: row.Y, Width: nextWidth, Height: height}, actions.NextLabel, true, actions.NextDisabled)
	}
	if actions.SkipLabel != "" {
		skipWidth := questionnaire.actionWidth(actions.SkipLabel)
		questionnaire.paintAction(actions.SkipID, gfx.Rect{X: nextX - metrics.GapItem - skipWidth, Y: row.Y, Width: skipWidth, Height: height}, actions.SkipLabel, false, false)
	}
}

func (questionnaire Questionnaire) actionWidth(label string) float32 {
	return questionnaire.canvas.TextWidthWeight(label, questionnaire.theme.Metrics.FontSizeM, gfx.WeightMedium) + questionnaire.theme.Metrics.PadControlX*2
}

func (questionnaire Questionnaire) paintAction(id gfx.ID, bounds gfx.Rect, label string, primary, disabled bool) {
	fill := questionnaire.theme.Background
	border := questionnaire.theme.Border
	foreground := questionnaire.theme.Foreground
	interactive := !id.IsZero() && !disabled
	if primary {
		fill = questionnaire.theme.Primary
		foreground = questionnaire.theme.PrimaryForeground
		border = questionnaire.theme.Primary
	}
	if disabled {
		fill = questionnaire.theme.Muted
		foreground = questionnaire.theme.MutedForeground
	} else if interactive && questionnaire.canvas.Hovered() == id {
		fill = questionnaire.theme.Hover
		if primary {
			fill = blend(questionnaire.theme.Primary, gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, 0.08)
		}
	}
	questionnaire.canvas.RoundedBorder(bounds, questionnaire.theme.Metrics.RadiusItem(), 1, border, fill)
	questionnaire.canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: questionnaire.theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground})
	if interactive {
		questionnaire.canvas.HitWithLabel(id, bounds, label)
	}
}
