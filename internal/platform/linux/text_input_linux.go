package linux

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

const linuxTextCaretWidth = 2

type linuxTextInput struct {
	config *platform.Config

	// engine places the caret, the selection and the composition underline, and finds
	// the character a click is on, from the same shaped line the glyphs are drawn from.
	engine ui.TextEngine

	input   platform.TextInput
	state   platform.TextState
	enabled bool

	caretVisible    bool
	caretGeneration uint32
	caretTimer      *time.Timer
	postCaret       func(uint32)
	caret           render.Rectangle
	// settings is the desktop's caret blink setting, as it stands; nil blinks at the
	// default rate.
	settings func() xsettings
	// compose follows a dead key or Compose sequence under way in this window.
	compose composer

	baseRectangles []render.Rectangle
	baseLabels     []platform.TextLabel
	rectangles     []render.Rectangle
	labels         []platform.TextLabel
}

func newLinuxTextInput(config *platform.Config) (*linuxTextInput, error) {
	state := &linuxTextInput{config: config}
	state.baseRectangles = append(state.baseRectangles, config.Rectangles...)
	state.baseLabels = copyTextLabels(state.baseLabels, config.TextLabels)
	if config.DynamicTextInput || config.TextInput != nil {
		// The caret, the selection and the composition underline stand where the glyphs are
		// drawn: on the character boundaries of the engine's shaped line.
		state.engine = TextEngine()
		if state.engine == nil {
			state.engine = ui.ReferenceTextEngine()
		}
	}
	if config.TextInput != nil {
		if err := validateLinuxTextInput(config.TextInput); err != nil {
			state.close()
			return nil, err
		}
		state.input = *config.TextInput
		state.state = config.TextInput.State
		state.enabled = true
		state.caretVisible = true
	}
	if err := state.rebuild(); err != nil {
		state.close()
		return nil, err
	}
	return state, nil
}

func (state *linuxTextInput) close() {
	state.stopCaretBlink()
	state.engine = nil
}

func (state *linuxTextInput) bindCaretWaker(post func(uint32)) {
	state.postCaret = post
	state.restartCaretBlink()
}

// bindSettings tells the caret where to read the desktop's blink setting. The
// setting applies from the next time the caret starts blinking.
func (state *linuxTextInput) bindSettings(settings func() xsettings) {
	state.settings = settings
	state.restartCaretBlink()
}

// blinkSetting is whether the caret blinks and how long each state lasts.
func (state *linuxTextInput) blinkSetting() (bool, time.Duration) {
	if state.settings == nil {
		return true, defaultBlinkInterval
	}
	return caretBlink(state.settings())
}

func copyTextLabels(destination []platform.TextLabel, source []platform.TextLabel) []platform.TextLabel {
	if cap(destination) < len(source) {
		destination = make([]platform.TextLabel, len(source))
	} else {
		destination = destination[:len(source)]
	}
	for index, label := range source {
		destination[index] = label
		destination[index].Runs = append([]ui.TextRun(nil), label.Runs...)
	}
	return destination
}

func validateLinuxTextInput(inputState *platform.TextInput) error {
	if inputState == nil {
		return nil
	}
	if inputState.Width <= 0 || inputState.Height <= 0 || !validFontSize(inputState.FontSize) {
		return fmt.Errorf("linux: text input dimensions and font size must be positive")
	}
	if inputState.OnEvent == nil {
		return fmt.Errorf("linux: text input handler is required")
	}
	return validateLinuxTextState(inputState.State)
}

func validateLinuxTextState(state platform.TextState) error {
	runeCount := len([]rune(state.Value))
	if !validLinuxTextRange(state.Selection, runeCount) {
		return fmt.Errorf("linux: invalid text selection %+v for %d runes", state.Selection, runeCount)
	}
	if state.Composing && !validLinuxTextRange(state.Composition, runeCount) {
		return fmt.Errorf("linux: invalid text composition %+v for %d runes", state.Composition, runeCount)
	}
	return nil
}

func validLinuxTextRange(value text.Range, length int) bool {
	return value.Start >= 0 && value.End >= value.Start && value.End <= length
}

func (state *linuxTextInput) initial() ([]render.Rectangle, []platform.TextLabel) {
	return state.rectangles, state.labels
}

func (state *linuxTextInput) prepare(frame platform.Frame) (platform.Frame, bool, error) {
	rectanglesChanged := !frame.Update.Empty()
	labelsChanged := frame.TextLabels != nil
	if rectanglesChanged {
		if frame.Update.First < 0 || frame.Update.Count < 0 || frame.Update.First > len(frame.Rectangles) || frame.Update.Count > len(frame.Rectangles)-frame.Update.First {
			return platform.Frame{}, false, fmt.Errorf("linux: invalid canvas rectangle update")
		}
		if frame.Update.Rebuild {
			state.baseRectangles = append(state.baseRectangles[:0], frame.Rectangles...)
		} else if len(frame.Rectangles) > len(state.baseRectangles) {
			// The scene appended instances. Keep the old ones and grow the base;
			// the text decoration forces a merged rebuild below either way.
			if frame.Update.First > len(state.baseRectangles) || frame.Update.First+frame.Update.Count != len(frame.Rectangles) {
				return platform.Frame{}, false, fmt.Errorf("linux: canvas rectangle count grew outside a suffix update")
			}
			state.baseRectangles = append(state.baseRectangles, frame.Rectangles[len(state.baseRectangles):]...)
			copy(state.baseRectangles[frame.Update.First:frame.Update.First+frame.Update.Count], frame.Rectangles[frame.Update.First:frame.Update.First+frame.Update.Count])
		} else {
			if len(state.baseRectangles) != len(frame.Rectangles) {
				return platform.Frame{}, false, fmt.Errorf("linux: canvas rectangle count changed without rebuild")
			}
			copy(state.baseRectangles[frame.Update.First:frame.Update.First+frame.Update.Count], frame.Rectangles[frame.Update.First:frame.Update.First+frame.Update.Count])
		}
	}
	if labelsChanged {
		state.baseLabels = copyTextLabels(state.baseLabels, frame.TextLabels)
	}
	inputChanged, err := state.applyUpdate(frame.TextInput)
	if err != nil {
		return platform.Frame{}, false, err
	}
	if !rectanglesChanged && !labelsChanged && !inputChanged {
		return platform.Frame{Accessibility: frame.Accessibility}, false, nil
	}
	if rectanglesChanged && !frame.Update.Rebuild && !labelsChanged && !inputChanged && len(state.rectangles) == len(frame.Rectangles) {
		copy(state.rectangles[frame.Update.First:frame.Update.First+frame.Update.Count], state.baseRectangles[frame.Update.First:frame.Update.First+frame.Update.Count])
		return platform.Frame{Update: frame.Update, Rectangles: state.rectangles, Accessibility: frame.Accessibility}, true, nil
	}
	oldRectangleCount := len(state.rectangles)
	if err := state.rebuild(); err != nil {
		return platform.Frame{}, false, err
	}
	merged := platform.Frame{Accessibility: frame.Accessibility}
	if rectanglesChanged || inputChanged {
		if inputChanged || frame.Update.Rebuild || oldRectangleCount != len(state.rectangles) {
			merged.Update = render.Update{Count: len(state.rectangles), Rebuild: true}
		} else {
			merged.Update = frame.Update
		}
		merged.Rectangles = state.rectangles
	}
	if labelsChanged || inputChanged {
		merged.TextLabels = state.labels
	}
	return merged, !merged.Update.Empty() || merged.TextLabels != nil, nil
}

func (state *linuxTextInput) applyUpdate(update platform.TextInputUpdate) (bool, error) {
	if !update.Changed {
		return false, nil
	}
	if !state.config.DynamicTextInput {
		return false, fmt.Errorf("linux: dynamic text input update is disabled")
	}
	if err := validateLinuxTextInput(update.Input); err != nil {
		return false, err
	}
	if update.Input == nil {
		state.stopCaretBlink()
		state.compose.reset()
		state.input = platform.TextInput{}
		state.state = platform.TextState{}
		state.enabled = false
		state.caretVisible = false
		state.caret = render.Rectangle{}
		return true, nil
	}
	state.input = *update.Input
	state.state = update.Input.State
	state.enabled = true
	state.restartCaretBlink()
	return true, nil
}

func (state *linuxTextInput) rebuild() error {
	extraRectangles, extraLabels := 0, 0
	if state.enabled {
		extraRectangles, extraLabels = 2, 1
		if state.state.Composing {
			extraRectangles++
		}
	}
	state.rectangles = append(state.rectangles[:0], state.baseRectangles...)
	state.labels = copyTextLabels(state.labels[:0], state.baseLabels)
	if !state.enabled {
		return nil
	}
	state.rectangles = append(state.rectangles, make([]render.Rectangle, extraRectangles)...)
	state.labels = append(state.labels, make([]platform.TextLabel, extraLabels)...)
	inputState := &state.input
	shown := state.state.Value
	if inputState.Secure {
		shown = strings.Repeat("•", len([]rune(shown)))
	}
	state.labels[len(state.baseLabels)] = platform.TextLabel{
		Value: shown, X: inputState.X, Y: inputState.Y, Width: inputState.Width, Height: inputState.Height,
		FontSize: inputState.FontSize, Color: inputState.Color, Layer: inputState.Layer, Mono: inputState.Terminal, MonoFace: inputState.MonoFace,
	}
	if inputState.Terminal {
		return nil
	}
	values := []rune(shown)
	selection := state.state.Selection
	selectionStart, err := state.prefixWidth(shown, values, selection.Start, inputState.FontSize)
	if err != nil {
		return err
	}
	selectionEnd, err := state.prefixWidth(shown, values, selection.End, inputState.FontSize)
	if err != nil {
		return err
	}
	base := len(state.baseRectangles)
	state.rectangles[base] = render.Rectangle{
		Bounds: scene.Rect{X: inputState.X + selectionStart, Y: inputState.Y, Width: selectionEnd - selectionStart, Height: inputState.Height},
		Color:  scene.Color{Red: 0.22, Green: 0.48, Blue: 1, Alpha: 0.34},
		Layer:  inputState.Layer,
	}
	caretHeight := min(inputState.Height, inputState.FontSize*1.35)
	state.caret = render.Rectangle{
		Bounds: scene.Rect{X: inputState.X + selectionEnd, Y: inputState.Y + (inputState.Height-caretHeight)/2, Width: linuxTextCaretWidth, Height: caretHeight},
		Color:  scene.Color{Red: float32(inputState.Color.Red), Green: float32(inputState.Color.Green), Blue: float32(inputState.Color.Blue), Alpha: float32(inputState.Color.Alpha)},
		Layer:  inputState.Layer,
	}
	if state.caretVisible && state.caretShown() {
		state.rectangles[base+1] = state.caret
	}
	if state.state.Composing {
		composition := state.state.Composition
		compositionStart, err := state.prefixWidth(shown, values, composition.Start, inputState.FontSize)
		if err != nil {
			return err
		}
		compositionEnd, err := state.prefixWidth(shown, values, composition.End, inputState.FontSize)
		if err != nil {
			return err
		}
		state.rectangles[base+2] = render.Rectangle{
			Bounds: scene.Rect{X: inputState.X + compositionStart, Y: inputState.Y + min(inputState.Height-2, inputState.FontSize*1.3), Width: compositionEnd - compositionStart, Height: 1.5},
			Color:  scene.Color{Red: float32(inputState.Color.Red), Green: float32(inputState.Color.Green), Blue: float32(inputState.Color.Blue), Alpha: float32(inputState.Color.Alpha)},
			Layer:  inputState.Layer,
		}
	}
	return nil
}

// prefixWidth is the x of the caret before the end-th character of shown (values are its
// runes): the edge of the boundary in the shaped line, which a ligature or a combining mark
// may place differently from the width of the characters before it.
func (state *linuxTextInput) prefixWidth(shown string, values []rune, end int, size float32) (float32, error) {
	if end <= 0 {
		return 0, nil
	}
	return state.engine.Caret(shown, size, 0, len(string(values[:end]))), nil
}

func (state *linuxTextInput) key(event platform.KeyEvent) (platform.Frame, bool, error) {
	if !state.enabled {
		return platform.Frame{}, false, nil
	}
	if event.Modifiers&(input.ModifierControl|input.ModifierAlt|input.ModifierMeta) == 0 && event.Text != "" {
		frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: event.Text})
		return frame, true, err
	}
	if state.input.Terminal {
		return platform.Frame{}, false, nil
	}
	if command, ok := platform.TextHistoryCommand(event); ok {
		frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
		return frame, true, err
	}
	// Apply platform-independent editing chords before handling the unmodified
	// named keys below. This keeps Control/Command+Delete behavior consistent
	// with Windows and macOS, including word and line-boundary deletion.
	if command, ok := platform.TextEditingCommand(event); ok {
		frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
		return frame, true, err
	}
	command := platform.TextCommand(0)
	switch event.Key {
	case input.KeyBackspace:
		command = platform.TextDeleteBackward
	case input.KeyDelete:
		command = platform.TextDeleteForward
	case input.KeyLeft:
		command = platform.TextMoveLeft
		if event.Modifiers&input.ModifierShift != 0 {
			command = platform.TextMoveLeftAndModifySelection
		}
	case input.KeyRight:
		command = platform.TextMoveRight
		if event.Modifiers&input.ModifierShift != 0 {
			command = platform.TextMoveRightAndModifySelection
		}
	case input.KeyEnter:
		if state.input.Multiline {
			command = platform.TextInsertNewline
			if event.Modifiers&input.ModifierShift != 0 {
				command = platform.TextInsertLineBreak
			}
		}
	}
	if command == 0 {
		return platform.Frame{}, false, nil
	}
	frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
	return frame, true, err
}

func (state *linuxTextInput) pointer(event platform.PointerEvent) (platform.Frame, bool, error) {
	if !state.enabled || state.input.Terminal || state.input.Placed || event.Kind != platform.PointerDown ||
		(event.Button != ui.PointerButtonPrimary && event.Button != ui.PointerButtonMiddle) {
		return platform.Frame{}, false, nil
	}
	inputState := &state.input
	if event.X < inputState.X || event.X > inputState.X+inputState.Width || event.Y < inputState.Y || event.Y > inputState.Y+inputState.Height {
		return platform.Frame{}, false, nil
	}
	index, err := state.nearestTextIndex(event.X - inputState.X)
	if err != nil {
		return platform.Frame{}, true, err
	}
	frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextSetSelection, Selection: text.Range{Start: index, End: index}})
	if err != nil || event.Button != ui.PointerButtonMiddle {
		return frame, true, err
	}
	// A middle press pastes the PRIMARY selection where it lands.
	pasted, err := PrimaryText()
	if err != nil || pasted == "" {
		return frame, true, nil
	}
	if !state.input.Multiline {
		pasted = strings.Join(strings.FieldsFunc(pasted, func(r rune) bool { return r == '\n' || r == '\r' }), " ")
	}
	next, err := state.textEvent(platform.TextEvent{Kind: platform.TextPaste, Text: pasted})
	if err != nil {
		return frame, true, err
	}
	return next, true, nil
}

// publishPrimary makes the text the user just selected the PRIMARY selection, as
// every X11 toolkit does, so a middle click elsewhere pastes it.
func (state *linuxTextInput) publishPrimary(previous, next platform.TextState) {
	if selected, ok := primaryCandidate(state.input.Secure || state.input.Terminal, previous, next); ok {
		_ = SetPrimaryText(selected)
	}
}

// primaryCandidate is the text to offer as the PRIMARY selection after an edit: the
// newly selected text, never a password's, and nothing when the selection did not
// change or is empty.
func primaryCandidate(private bool, previous, next platform.TextState) (string, bool) {
	if private || next.Selection == previous.Selection && next.Value == previous.Value {
		return "", false
	}
	selection := next.Selection
	if selection.Start > selection.End {
		selection.Start, selection.End = selection.End, selection.Start
	}
	runes := []rune(next.Value)
	if selection.Start == selection.End || selection.Start < 0 || selection.End > len(runes) {
		return "", false
	}
	return string(runes[selection.Start:selection.End]), true
}

func (state *linuxTextInput) nearestTextIndex(x float32) (int, error) {
	shown := state.state.Value
	if state.input.Secure {
		shown = strings.Repeat("•", len([]rune(shown)))
	}
	if shown == "" {
		return 0, nil
	}
	offset := state.engine.OffsetAt(shown, state.input.FontSize, 0, x)
	return utf8.RuneCountInString(shown[:max(0, min(offset, len(shown)))]), nil
}

func (state *linuxTextInput) textEvent(event platform.TextEvent) (platform.Frame, error) {
	if !state.enabled || state.input.OnEvent == nil {
		return platform.Frame{}, nil
	}
	previous := state.state
	next, err := state.input.OnEvent(event)
	if err == nil {
		err = validateLinuxTextState(next)
	}
	if err != nil {
		return platform.Frame{}, err
	}
	redraw := platform.TextInputPresentationChanged(state.input.Terminal, previous, next)
	next = next.WithoutRedraw() // a one-shot request, not editor state
	state.state = next
	state.input.State = next
	state.publishPrimary(previous, next)
	if !redraw {
		return platform.Frame{}, nil
	}
	state.restartCaretBlink()
	if state.config.OnRedraw != nil {
		frame, err := state.config.OnRedraw()
		if err == nil && !frame.TextInput.Changed {
			frame.TextInput = platform.TextInputUpdate{Changed: true, Input: &state.input}
		}
		return frame, err
	}
	return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &state.input}}, nil
}

// caretShown says whether the field has a caret to draw: not a terminal, not
// while composing, not over a selection.
func (state *linuxTextInput) caretShown() bool {
	return state.enabled && !state.input.Terminal && !state.state.Composing && state.state.Selection.Start == state.state.Selection.End
}

func (state *linuxTextInput) caretShouldBlink() bool {
	blinks, _ := state.blinkSetting()
	return blinks && state.caretShown()
}

func (state *linuxTextInput) restartCaretBlink() {
	state.stopCaretBlink()
	state.caretVisible = true
	state.caretGeneration++
	state.armCaretBlink()
}

func (state *linuxTextInput) armCaretBlink() {
	if !state.caretShouldBlink() || state.postCaret == nil {
		return
	}
	generation := state.caretGeneration
	_, interval := state.blinkSetting()
	state.caretTimer = time.AfterFunc(interval, func() {
		state.postCaret(generation)
	})
}

func (state *linuxTextInput) stopCaretBlink() {
	state.caretGeneration++
	if state.caretTimer != nil {
		state.caretTimer.Stop()
		state.caretTimer = nil
	}
}

func (state *linuxTextInput) blinkCaret(generation uint32) (platform.Frame, bool) {
	if generation != state.caretGeneration || !state.caretShown() {
		return platform.Frame{}, false
	}
	blinks := state.caretShouldBlink()
	if !blinks && state.caretVisible {
		return platform.Frame{}, false
	}
	// A desktop that stopped the blinking while the caret was hidden gets it back.
	state.caretVisible = !blinks || !state.caretVisible
	index := len(state.baseRectangles) + 1
	state.rectangles[index] = render.Rectangle{}
	if state.caretVisible {
		state.rectangles[index] = state.caret
	}
	if blinks {
		state.armCaretBlink()
	}
	return platform.Frame{
		Update:     render.Update{First: index, Count: 1},
		Rectangles: state.rectangles,
	}, true
}
