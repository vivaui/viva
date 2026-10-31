package windows

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	inputpkg "github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

const (
	compositionString  = 0x0008
	compositionAttr    = 0x0010
	compositionCursor  = 0x0080
	resultString       = 0x0800
	compositionPoint   = 2
	compositionCancel  = 4
	notifyComposition  = 0x0015
	textRectangleCount = 4
	caretTimerID       = 1
	textCaretWidth     = 2
)

type compositionForm struct {
	Style uint32
	Point point
	Area  rect
}

var imm = systemDLL("imm32.dll")
var getInputContext = imm.NewProc("ImmGetContext")
var releaseInputContext = imm.NewProc("ImmReleaseContext")
var getCompositionString = imm.NewProc("ImmGetCompositionStringW")
var setCompositionWindow = imm.NewProc("ImmSetCompositionWindow")
var notifyIME = imm.NewProc("ImmNotifyIME")
var setCandidateWindow = imm.NewProc("ImmSetCandidateWindow")

// candidateForm is CANDIDATEFORM.
type candidateForm struct {
	Index, Style uint32
	Point        point
	Area         rect
}

const (
	candidateExclude = 0x0080 // CFS_EXCLUDE: place the list outside Area
)

func validateTextInput(input *platform.TextInput) error {
	if input == nil {
		return nil
	}
	if input.Width <= 0 || input.Height <= 0 || input.FontSize <= 0 {
		return fmt.Errorf("windows: text input dimensions and font size must be positive")
	}
	if input.OnEvent == nil {
		return fmt.Errorf("windows: text input handler is required")
	}
	return validateTextState(input.State)
}

func validateTextState(state platform.TextState) error {
	runeCount := utf8.RuneCountInString(state.Value)
	if !validTextRange(state.Selection, runeCount) {
		return fmt.Errorf("windows: invalid text selection %+v for %d runes", state.Selection, runeCount)
	}
	if state.Composing && !validTextRange(state.Composition, runeCount) {
		return fmt.Errorf("windows: invalid text composition %+v for %d runes", state.Composition, runeCount)
	}
	return nil
}

func validTextRange(value text.Range, length int) bool {
	return value.Start >= 0 && value.End >= value.Start && value.End <= length
}

func (state *windowState) initializeText() error {
	state.baseRectangleCount = len(state.config.Rectangles)
	state.baseTextLabelCount = len(state.config.TextLabels)
	state.textRectangles = append(state.textRectangles[:0], state.config.Rectangles...)
	state.textLabels = append(state.textLabels[:0], state.config.TextLabels...)
	if state.config.TextInput == nil && !state.config.DynamicTextInput {
		return nil
	}
	// The caret, the selection and the composition underline stand where the glyphs are
	// drawn: the shaped line of the text engine is both.
	engine, err := theTextEngine()
	if err != nil {
		return err
	}
	state.textEngine = engine
	if state.config.TextInput == nil {
		return nil
	}
	state.textInput = *state.config.TextInput
	state.textState = state.textInput.State
	state.textEnabled = true
	state.caretVisible = true
	state.resizeTextStorage(state.baseRectangleCount+textRectangleCount, state.baseTextLabelCount+1)
	return state.updateTextVisuals()
}

func (state *windowState) closeText() {
	state.stopCaretTimer()
	state.textEngine = nil
}

func (state *windowState) updateTextVisuals() error {
	if !state.textEnabled {
		return nil
	}
	input := &state.textInput
	caretX := state.textCaret(input.FontSize, state.textCaretIndex())
	textWidth := state.textWidth()
	visibleWidth := max(1, input.Width-textCaretWidth)
	state.textScroll = max(0, min(state.textScroll, caretX, max(0, textWidth-visibleWidth)))
	if caretX-state.textScroll > visibleWidth {
		state.textScroll = caretX - visibleWidth
	}
	originX := input.X - state.textScroll
	label := platform.TextLabel{
		Value: state.textDisplayValue(), X: originX, Y: input.Y,
		Width:  max(input.Width+state.textScroll, textWidth+textCaretWidth),
		Height: input.Height, FontSize: input.FontSize, Color: input.Color, Layer: input.Layer,
		Clipped: true, Clip: ui.Rect{X: input.X, Y: input.Y, Width: input.Width, Height: input.Height},
	}
	if input.Terminal {
		// A terminal's preedit stands on the terminal's cells, in its fixed-pitch font,
		// not on the advances of the system UI font.
		color := ui.Color{Red: float32(input.Color.Red), Green: float32(input.Color.Green), Blue: float32(input.Color.Blue), Alpha: float32(input.Color.Alpha)}
		state.textRuns = platform.TerminalPreeditRuns(state.textRuns[:0], label.Value, color)
		label.Mono, label.MonoFace, label.CellWidth, label.Runs = true, input.MonoFace, terminalCellWidth(input.FontSize), state.textRuns
	}
	state.textLabels[state.baseTextLabelCount] = label
	base := state.baseRectangleCount
	caretHeight := min(input.Height, input.FontSize*1.35)
	caretColor := scene.Color{Red: float32(input.Color.Red), Green: float32(input.Color.Green), Blue: float32(input.Color.Blue), Alpha: float32(input.Color.Alpha)}
	state.textRectangles[base] = render.Rectangle{}
	state.textRectangles[base+1] = render.Rectangle{}
	// A terminal is a preedit surface with a cursor of its own. Components that
	// paint a multi-line caret and selection also ask the native editor to keep
	// its single-line overlays invisible. In both cases the composition remains
	// native so the IME continues to own its underline.
	if !input.Terminal && !input.HideNativeOverlays {
		selection := state.textState.Selection
		selectionStart, selectionEnd := state.textExtent(input.FontSize, selection)
		state.textRectangles[base] = render.Rectangle{
			Bounds: scene.Rect{X: originX + selectionStart, Y: input.Y, Width: selectionEnd - selectionStart, Height: input.Height},
			Color:  scene.Color{Red: 0.22, Green: 0.48, Blue: 1, Alpha: 0.34},
			Layer:  input.Layer,
		}
		if state.caretVisible {
			state.textRectangles[base+1] = render.Rectangle{
				Bounds: scene.Rect{X: originX + caretX, Y: input.Y + (input.Height-caretHeight)/2, Width: textCaretWidth, Height: caretHeight},
				Color:  caretColor,
				Layer:  input.Layer,
			}
		}
	}
	state.textRectangles[base+2] = render.Rectangle{}
	state.textRectangles[base+3] = render.Rectangle{}
	if state.textState.Composing {
		composition := state.textState.Composition
		compositionStart, compositionEnd := state.textExtent(input.FontSize, composition)
		state.textRectangles[base+2] = render.Rectangle{
			Bounds: scene.Rect{X: originX + compositionStart, Y: input.Y + min(input.Height-2, input.FontSize*1.3), Width: compositionEnd - compositionStart, Height: 1.5},
			Color:  caretColor,
			Layer:  input.Layer,
		}
		if state.imeHasTarget {
			// The clause the input method is converting is underlined more thickly than
			// the rest of the composition, which is how it tells which part a candidate
			// list is for.
			target := text.Range{Start: composition.Start + state.imeTarget.Start, End: composition.Start + state.imeTarget.End}
			if validTextRange(target, utf8.RuneCountInString(state.textState.Value)) {
				targetStart, targetEnd := state.textExtent(input.FontSize, target)
				state.textRectangles[base+3] = render.Rectangle{
					Bounds: scene.Rect{X: originX + targetStart, Y: input.Y + min(input.Height-3, input.FontSize*1.3), Width: targetEnd - targetStart, Height: 3},
					Color:  caretColor,
					Layer:  input.Layer,
				}
			}
		}
	}
	for index := base; index < base+textRectangleCount; index++ {
		state.textRectangles[index].Clipped = true
		state.textRectangles[index].Clip = scene.Rect{X: input.X, Y: input.Y, Width: input.Width, Height: input.Height}
	}
	return nil
}

func (state *windowState) textDisplayValue() string {
	if state.textInput.Secure {
		count := utf8.RuneCountInString(state.textState.Value)
		if len(state.textMask) != count*len("•") {
			state.textMask = strings.Repeat("•", count)
		}
		return state.textMask
	}
	return state.textState.Value
}

func (state *windowState) textCaretIndex() int {
	if state.textState.SelectionReversed {
		return state.textState.Selection.Start
	}
	return state.textState.Selection.End
}

// textWidth is the width of the input's display value.
func (state *windowState) textWidth() float32 {
	if state.textInput.Terminal {
		return state.textCaret(state.textInput.FontSize, utf8.RuneCountInString(state.textDisplayValue()))
	}
	return state.textEngine.Width(state.textDisplayValue(), state.textInput.FontSize, 0)
}

// terminalCellWidth is the width of one terminal cell at a font size: the pitch every
// platform's fixed-pitch text is placed on (see ui.Canvas.MonoCellSize).
func terminalCellWidth(size float32) float32 {
	return platform.TerminalPreeditCellWidth(size)
}

// textByte is the byte offset of the rune index in the text input's display value.
func (state *windowState) textByte(runeIndex int) int {
	value := state.textDisplayValue()
	offset := 0
	for ; runeIndex > 0 && offset < len(value); runeIndex-- {
		_, size := utf8.DecodeRuneInString(value[offset:])
		offset += size
	}
	return offset
}

// textCaret is the x of the caret before the rune index, from the left of the input.
func (state *windowState) textCaret(size float32, runeIndex int) float32 {
	if state.textInput.Terminal {
		return float32(platform.TerminalPreeditColumn(state.textDisplayValue(), runeIndex)) * terminalCellWidth(size)
	}
	return state.textEngine.Caret(state.textDisplayValue(), size, 0, state.textByte(runeIndex))
}

// textExtent is the left and right x of the glyphs of the runes in the range: the one
// rectangle a selection or a composition underline is drawn as. A range with text of
// both directions is drawn from its leftmost to its rightmost glyph.
func (state *windowState) textExtent(size float32, runes text.Range) (left, right float32) {
	if state.textInput.Terminal {
		return state.textCaret(size, runes.Start), state.textCaret(size, runes.End)
	}
	state.textSpans = state.textEngine.Selection(state.textSpans[:0], state.textDisplayValue(), size, 0, state.textByte(runes.Start), state.textByte(runes.End))
	if len(state.textSpans) == 0 {
		return 0, 0
	}
	left, right = state.textSpans[0].X, state.textSpans[0].X+state.textSpans[0].Width
	for _, span := range state.textSpans[1:] {
		left, right = min(left, span.X), max(right, span.X+span.Width)
	}
	return left, right
}

func (state *windowState) textPointerEvent(event platform.PointerEvent) (platform.TextEvent, bool, error) {
	if !state.textEnabled || state.textInput.Terminal || state.textInput.Placed || state.imeComposing {
		state.textDragging = false
		return platform.TextEvent{}, false, nil
	}
	input := &state.textInput
	switch event.Kind {
	case platform.PointerDown:
		state.textDragging = false
		if event.Button != ui.PointerButtonPrimary || event.X < input.X || event.X > input.X+input.Width || event.Y < input.Y || event.Y > input.Y+input.Height {
			return platform.TextEvent{}, false, nil
		}
	case platform.PointerMove:
		if !state.textDragging {
			return platform.TextEvent{}, false, nil
		}
	case platform.PointerUp:
		state.textDragging = false
		return platform.TextEvent{}, false, nil
	case platform.PointerExit:
		state.textDragging = false
		return platform.TextEvent{}, false, nil
	default:
		return platform.TextEvent{}, false, nil
	}
	value := state.textDisplayValue()
	offset := state.textEngine.OffsetAt(value, input.FontSize, 0, event.X-input.X+state.textScroll)
	index := utf8.RuneCountInString(value[:max(0, min(offset, len(value)))])
	if event.Kind == platform.PointerDown {
		state.textDragAnchor = index
		state.textDragging = true
		if event.Modifiers&inputpkg.ModifierShift != 0 {
			state.textDragAnchor = state.textState.Selection.Start
			if state.textState.SelectionReversed {
				state.textDragAnchor = state.textState.Selection.End
			}
		} else if event.ClickCount >= 2 {
			selection := text.WordRange(state.textState.Value, index)
			if event.ClickCount >= 3 || state.textInput.Secure {
				selection = text.Range{End: utf8.RuneCountInString(state.textState.Value)}
			}
			state.textDragAnchor = selection.Start
			return platform.TextEvent{Kind: platform.TextSetSelection, Selection: selection}, true, nil
		}
	}
	selection := text.Range{Start: state.textDragAnchor, End: index}
	return platform.TextEvent{Kind: platform.TextSetSelection, Selection: selection}, true, nil
}

func (state *windowState) handleTextEvent(event platform.TextEvent) {
	frame, changed, err := state.textEventFrame(event)
	if err != nil {
		state.fail(err)
		return
	}
	if !changed {
		return
	}
	state.frameWithText(frame, nil, true)
}

func (state *windowState) textEventFrame(event platform.TextEvent) (platform.Frame, bool, error) {
	if !state.textEnabled || state.textInput.OnEvent == nil {
		return platform.Frame{}, false, nil
	}
	previous := state.textState
	next, err := state.textInput.OnEvent(event)
	if err == nil {
		err = validateTextState(next)
	}
	if err != nil {
		return platform.Frame{}, false, err
	}
	redraw := platform.TextInputPresentationChanged(state.textInput.Terminal, previous, next)
	next = next.WithoutRedraw() // a one-shot request, not editor state
	state.textState = next
	state.textInput.State = next
	if !redraw {
		return platform.Frame{}, false, nil
	}
	state.caretVisible = true
	if err := state.restartCaretTimer(); err != nil {
		return platform.Frame{}, false, err
	}
	if state.config.OnRedraw != nil {
		frame, err := state.config.OnRedraw()
		return frame, true, err
	}
	return platform.Frame{}, true, nil
}

func (state *windowState) resizeTextStorage(rectangleCount, labelCount int) {
	if rectangleCount < len(state.textRectangles) {
		clear(state.textRectangles[rectangleCount:])
	}
	if cap(state.textRectangles) < rectangleCount {
		next := make([]render.Rectangle, rectangleCount)
		copy(next, state.textRectangles)
		state.textRectangles = next
	} else {
		state.textRectangles = state.textRectangles[:rectangleCount]
	}
	if labelCount < len(state.textLabels) {
		clear(state.textLabels[labelCount:])
	}
	if cap(state.textLabels) < labelCount {
		next := make([]platform.TextLabel, labelCount)
		copy(next, state.textLabels)
		state.textLabels = next
	} else {
		state.textLabels = state.textLabels[:labelCount]
	}
}

func (state *windowState) applyTextInputUpdate(update platform.TextInputUpdate) (bool, bool, error) {
	if !update.Changed {
		return false, false, nil
	}
	if !state.config.DynamicTextInput {
		return false, false, fmt.Errorf("windows: dynamic text input update is disabled")
	}
	if err := validateTextInput(update.Input); err != nil {
		return false, false, err
	}
	wasEnabled := state.textEnabled
	// A placed/multiline editor updates its caret after every preedit. The
	// text event has already installed the acknowledged state before redraw;
	// moving that same editor must not send CPS_CANCEL to the input method.
	replaced := update.Input == nil || state.textInput.Target != update.Input.Target ||
		state.textInput.Multiline != update.Input.Multiline || state.textInput.Secure != update.Input.Secure ||
		state.textInput.Terminal != update.Input.Terminal || state.textInput.Placed != update.Input.Placed ||
		state.textState != update.Input.State.WithoutRedraw()
	if wasEnabled && replaced {
		state.cancelComposition()
	}
	if replaced {
		state.pendingHighSurrogate = 0
	}
	if update.Input == nil {
		state.textDragging = false
		state.textScroll = 0
		state.textEnabled = false
		state.textInput = platform.TextInput{}
		state.textState = platform.TextState{}
		state.caretVisible = false
		state.stopCaretTimer()
		return true, wasEnabled, nil
	}
	state.textInput = *update.Input
	state.textState = state.textInput.State
	state.textEnabled = true
	state.caretVisible = true
	if err := state.restartCaretTimer(); err != nil {
		return false, false, err
	}
	state.positionCompositionWindow()
	return true, !wasEnabled, nil
}

func (state *windowState) prepareFrame(frame platform.Frame, nativeVisualChanged bool) (platform.Frame, bool, error) {
	if frame.TextInput.Changed {
		if !state.config.DynamicTextInput {
			return platform.Frame{}, false, fmt.Errorf("windows: dynamic text input update is disabled")
		}
		if err := validateTextInput(frame.TextInput.Input); err != nil {
			return platform.Frame{}, false, err
		}
	}
	oldBaseRectangles := state.baseRectangleCount
	oldBaseLabels := state.baseTextLabelCount
	baseRectangles := state.textRectangles[:oldBaseRectangles]
	baseLabels := state.textLabels[:oldBaseLabels]

	if !frame.Update.Empty() {
		if frame.Update.First < 0 || frame.Update.Count < 0 || frame.Update.First > len(frame.Rectangles) || frame.Update.Count > len(frame.Rectangles)-frame.Update.First {
			return platform.Frame{}, false, fmt.Errorf("windows: invalid canvas rectangle update")
		}
		if frame.Update.Rebuild {
			baseRectangles = frame.Rectangles
		} else if len(frame.Rectangles) > oldBaseRectangles {
			// The scene appended instances below the native text overlay. The
			// overlay keeps its own retained storage, so take the new base and
			// let the structure-changed path below rebuild the merged list.
			if frame.Update.First > oldBaseRectangles || frame.Update.First+frame.Update.Count != len(frame.Rectangles) {
				return platform.Frame{}, false, fmt.Errorf("windows: canvas rectangle count grew outside a suffix update")
			}
			baseRectangles = frame.Rectangles
		} else {
			if len(frame.Rectangles) != oldBaseRectangles {
				return platform.Frame{}, false, fmt.Errorf("windows: canvas rectangle count shrank without rebuild")
			}
			copy(state.textRectangles[frame.Update.First:frame.Update.First+frame.Update.Count], frame.Rectangles[frame.Update.First:frame.Update.First+frame.Update.Count])
		}
	}
	if frame.TextLabels != nil {
		baseLabels = frame.TextLabels
	}

	inputChanged, inputStructureChanged, err := state.applyTextInputUpdate(frame.TextInput)
	if err != nil {
		return platform.Frame{}, false, err
	}
	state.baseRectangleCount = len(baseRectangles)
	state.baseTextLabelCount = len(baseLabels)
	extraRectangles, extraLabels := 0, 0
	if state.textEnabled {
		extraRectangles, extraLabels = textRectangleCount, 1
	}
	rectangleStructureChanged := inputStructureChanged || frame.Update.Rebuild || oldBaseRectangles != state.baseRectangleCount
	labelStructureChanged := inputStructureChanged || oldBaseLabels != state.baseTextLabelCount
	state.resizeTextStorage(state.baseRectangleCount+extraRectangles, state.baseTextLabelCount+extraLabels)
	if rectangleStructureChanged {
		copy(state.textRectangles[:state.baseRectangleCount], baseRectangles)
	}
	if frame.TextLabels != nil || labelStructureChanged {
		copy(state.textLabels[:state.baseTextLabelCount], baseLabels)
	}

	visualChanged := nativeVisualChanged || inputChanged || rectangleStructureChanged || labelStructureChanged
	if state.textEnabled && visualChanged {
		if err := state.updateTextVisuals(); err != nil {
			return platform.Frame{}, false, err
		}
	}

	merged := platform.Frame{Accessibility: frame.Accessibility}
	if rectangleStructureChanged {
		merged.Update = render.Update{Count: len(state.textRectangles), Rebuild: true}
	} else {
		merged.Update = frame.Update
		if state.textEnabled && (nativeVisualChanged || inputChanged) {
			merged.Update = mergeRenderUpdates(merged.Update, render.Update{First: state.baseRectangleCount, Count: textRectangleCount})
		}
	}
	if !merged.Update.Empty() {
		merged.Rectangles = state.textRectangles
	}
	if frame.TextLabels != nil || labelStructureChanged || state.textEnabled && (nativeVisualChanged || inputChanged) {
		merged.TextLabels = state.textLabels
	}
	dirty := !merged.Update.Empty() || merged.TextLabels != nil
	return merged, dirty, nil
}

func mergeRenderUpdates(left, right render.Update) render.Update {
	if left.Empty() {
		return right
	}
	if right.Empty() {
		return left
	}
	if left.Rebuild || right.Rebuild {
		return render.Update{Count: max(left.First+left.Count, right.First+right.Count), Rebuild: true}
	}
	first := min(left.First, right.First)
	last := max(left.First+left.Count, right.First+right.Count)
	return render.Update{First: first, Count: last - first}
}

func (state *windowState) cancelComposition() {
	if !state.imeComposing && !state.textState.Composing {
		return
	}
	state.imeComposing = false
	state.imeHasTarget = false
	state.textState.Composing = false
	state.textState.Composition = text.Range{}
	if state.window == 0 {
		return
	}
	context, _, _ := getInputContext.Call(state.window)
	if context == 0 {
		return
	}
	notifyIME.Call(context, notifyComposition, compositionCancel, 0)
	releaseInputContext.Call(state.window, context)
}

// caretBlinks reports whether a native caret belongs on screen. Terminal and
// self-painting input surfaces own both their cursor and selection.
func (state *windowState) caretBlinks() bool {
	return state.textEnabled && state.focused && !state.textInput.Terminal && !state.textInput.HideNativeOverlays
}

func (state *windowState) restartCaretTimer() error {
	state.stopCaretTimer()
	if !state.caretBlinks() || state.window == 0 {
		return nil
	}
	interval, _, _ := getCaretBlinkTime.Call()
	if interval == 0 || uint32(interval) == ^uint32(0) {
		return nil
	}
	timer, _, callErr := setTimer.Call(state.window, caretTimerID, interval, 0)
	if timer == 0 {
		return windowsCallError("start caret timer", callErr)
	}
	state.caretTimer = timer
	return nil
}

func (state *windowState) stopCaretTimer() {
	if state.caretTimer == 0 {
		return
	}
	if state.window != 0 {
		killTimer.Call(state.window, state.caretTimer)
	}
	state.caretTimer = 0
}

func (state *windowState) handleCaretTimer(timer uintptr) {
	if timer != state.caretTimer || !state.textEnabled {
		return
	}
	state.setCaretVisible(!state.caretVisible)
}

func (state *windowState) setCaretVisible(visible bool) {
	if !state.textEnabled || state.caretVisible == visible {
		return
	}
	state.caretVisible = visible
	if err := state.updateTextVisuals(); err != nil {
		state.fail(err)
		return
	}
	if err := state.renderer.apply(platform.Frame{
		Update:     render.Update{First: state.baseRectangleCount + 1, Count: 1},
		Rectangles: state.textRectangles,
	}); err != nil {
		state.fail(err)
		return
	}
	invalidateRect.Call(state.window, 0, 0)
}

func (state *windowState) handleComposition(flags uintptr) {
	// An input method that has no context to give, or a composition string it
	// cannot report, loses that one composition. Closing the window over it would
	// end the whole session for a password typed through a third-party IME.
	context, _, _ := getInputContext.Call(state.window)
	if context == 0 {
		return
	}
	defer releaseInputContext.Call(state.window, context)
	if flags&resultString != 0 {
		value, err := compositionText(context, resultString)
		state.imeComposing = false
		state.imeHasTarget = false
		if err != nil {
			return
		}
		state.handleTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: value})
		return
	}
	if flags&compositionString == 0 {
		return
	}
	value, err := compositionText(context, compositionString)
	if err != nil {
		return
	}
	state.imeTarget, state.imeHasTarget = compositionTarget(context, value)
	cursorUnits := 0
	if flags&compositionCursor != 0 {
		position, _, _ := getCompositionString.Call(context, compositionCursor, 0, 0)
		cursorUnits = int(int32(position))
		if cursorUnits < 0 {
			cursorUnits = 0
		}
	}
	cursorRunes := text.NewEditor(value).UTF16ToRuneIndex(cursorUnits)
	state.imeComposing = true
	state.handleTextEvent(platform.TextEvent{
		Kind: platform.TextSetMarked, Text: value,
		MarkedSelection: text.Range{Start: cursorRunes, End: cursorRunes},
	})
}

func (state *windowState) positionCompositionWindow() {
	if !state.textEnabled || state.window == 0 {
		return
	}
	input := &state.textInput
	context, _, _ := getInputContext.Call(state.window)
	if context == 0 {
		return
	}
	defer releaseInputContext.Call(state.window, context)
	// The composition and its candidate list go under the caret, where the text is
	// being typed; the list keeps off the line itself.
	caretX, caretY, caretHeight := state.caretCell()
	scale := state.scale
	line := rect{Left: int32(caretX * scale), Top: int32(caretY * scale), Right: int32((caretX + 1) * scale), Bottom: int32((caretY + caretHeight) * scale)}
	form := compositionForm{Style: compositionPoint, Point: point{X: line.Left, Y: line.Bottom}}
	setCompositionWindow.Call(context, uintptr(unsafe.Pointer(&form)))
	if setCandidateWindow.Find() == nil {
		candidates := candidateForm{Style: candidateExclude, Point: form.Point, Area: line}
		if !input.Placed {
			candidates.Area.Left, candidates.Area.Right = int32(input.X*scale), int32((input.X+input.Width)*scale)
		}
		setCandidateWindow.Call(context, uintptr(unsafe.Pointer(&candidates)))
	}
}

// caretCell is where the caret is in logical window pixels: its x, the top of
// its line and the height of the line.
func (state *windowState) caretCell() (x, y, height float32) {
	input := &state.textInput
	if input.Placed && input.CaretHeight > 0 {
		// The field laid its rows out itself: this is where it painted its caret.
		return input.CaretX, input.CaretY, input.CaretHeight
	}
	return input.X + state.textCaret(input.FontSize, state.textCaretIndex()) - state.textScroll, input.Y, input.Height
}

// compositionTarget is the clause the input method is converting, as runes of the
// composition string.
func compositionTarget(context uintptr, value string) (text.Range, bool) {
	length, _, _ := getCompositionString.Call(context, compositionAttr, 0, 0)
	if int32(length) <= 0 || length > 1<<16 {
		return text.Range{}, false
	}
	attrs := make([]byte, length)
	written, _, _ := getCompositionString.Call(context, compositionAttr, uintptr(unsafe.Pointer(&attrs[0])), length)
	if int32(written) <= 0 || written > length {
		return text.Range{}, false
	}
	return platform.CompositionTarget(attrs[:written], value)
}

func compositionText(context uintptr, index uintptr) (string, error) {
	lengthValue, _, _ := getCompositionString.Call(context, index, 0, 0)
	length := int(int32(lengthValue))
	if length < 0 {
		return "", fmt.Errorf("windows: read IME composition length %d", length)
	}
	if length == 0 {
		return "", nil
	}
	if length > 1<<20 || length%2 != 0 {
		return "", fmt.Errorf("windows: invalid IME composition length %d", length)
	}
	values := make([]uint16, length/2)
	writtenValue, _, _ := getCompositionString.Call(context, index, uintptr(unsafe.Pointer(&values[0])), uintptr(length))
	written := int(int32(writtenValue))
	if written < 0 || written > length || written%2 != 0 {
		return "", fmt.Errorf("windows: invalid IME composition result %d", written)
	}
	return string(utf16.Decode(values[:written/2])), nil
}

func (state *windowState) handleUTF16Character(value uint16) {
	textValue, pending, ready := decodeUTF16Character(state.pendingHighSurrogate, value)
	state.pendingHighSurrogate = pending
	if !ready {
		return
	}
	state.handleTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: textValue})
}

func decodeUTF16Character(pending, value uint16) (string, uint16, bool) {
	if value < 0x20 || value == 0x7f {
		return "", pending, false
	}
	if value >= 0xd800 && value <= 0xdbff {
		return "", value, false
	}
	if value >= 0xdc00 && value <= 0xdfff && pending != 0 {
		return string(utf16.DecodeRune(rune(pending), rune(value))), 0, true
	}
	return string(rune(value)), 0, true
}

func (state *windowState) textCommand(key uintptr) (platform.TextCommand, bool) {
	return state.textCommandWith(key, modifiers()&1 != 0)
}

// textCommandWith is textCommand for a known state of the Shift key.
func (state *windowState) textCommandWith(key uintptr, shift bool) (platform.TextCommand, bool) {
	switch key {
	case 8:
		return platform.TextDeleteBackward, true
	case 9:
		return 0, false
	case 13:
		if !state.textInput.Multiline {
			return 0, false
		}
		if shift {
			return platform.TextInsertLineBreak, true
		}
		return platform.TextInsertNewline, true
	case 37:
		if shift {
			return platform.TextMoveLeftAndModifySelection, true
		}
		return platform.TextMoveLeft, true
	case 39:
		if shift {
			return platform.TextMoveRightAndModifySelection, true
		}
		return platform.TextMoveRight, true
	case 46:
		return platform.TextDeleteForward, true
	default:
		return 0, false
	}
}
