//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// vivaShapedLine is how the native drawing reaches the text engine's cache: the
// shaped line for the text, retained for the caller, which releases it with
// viva_text_line_release.
//
//export vivaShapedLine
func vivaShapedLine(utf8 *C.uchar, length C.size_t, size, weight C.double) *C.viva_text_line {
	value := ""
	if length > 0 {
		value = unsafe.String((*byte)(unsafe.Pointer(utf8)), int(length))
	}
	return sharedTextEngine().retainedLine(value, float32(size), float32(weight))
}

//export vivaPointerEvent
func vivaPointerEvent(
	handle C.uintptr_t,
	view unsafe.Pointer,
	kind C.int,
	button C.int,
	modifiers C.int,
	clickCount C.int,
	x C.double,
	y C.double,
	screenX C.double,
	screenY C.double,
) {
	state := cgo.Handle(handle).Value().(*runState)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: pointer handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()

	frame, err := state.pointerHandler(PointerEvent{
		Kind:       PointerEventKind(kind),
		Button:     ui.PointerButton(button),
		Modifiers:  input.Modifiers(modifiers),
		X:          float32(x),
		Y:          float32(y),
		ScreenX:    float32(screenX),
		ScreenY:    float32(screenY),
		ClickCount: int(clickCount),
	})
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
	}
}

//export vivaViewResized
func vivaViewResized(handle C.uintptr_t, view unsafe.Pointer, width C.double, height C.double) {
	state := cgo.Handle(handle).Value().(*runState)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: resize handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()

	frame, err := state.resizeHandler(Size{Width: float32(width), Height: float32(height)})
	if err == nil {
		err = presentFrame(state, view, frame, true)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
	}
}

//export vivaFrameChanged
func vivaFrameChanged(handle C.uintptr_t, view unsafe.Pointer, x, y, width, height C.double) {
	state := cgo.Handle(handle).Value().(*runState)
	if state.frameHandler == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: frame change handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()
	state.frameHandler(ui.Rect{X: float32(x), Y: float32(y), Width: float32(width), Height: float32(height)})
}

//export vivaWindowReopenRequested
func vivaWindowReopenRequested(handle C.uintptr_t, view unsafe.Pointer) (handled C.int) {
	state := cgo.Handle(handle).Value().(*runState)
	if state.onReopen == nil {
		return 0
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: reopen handler panicked: %v", recovered)
			C.viva_view_close(view)
			handled = 1
		}
	}()
	state.onReopen()
	return 1
}

//export vivaWindowMinimized
func vivaWindowMinimized(handle C.uintptr_t, view unsafe.Pointer, minimized C.int) {
	state := cgo.Handle(handle).Value().(*runState)
	if state.minimizeHandler == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: minimize handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()
	state.minimizeHandler(minimized != 0)
}

//export vivaRedrawRequested
func vivaRedrawRequested(handle C.uintptr_t, view unsafe.Pointer) {
	state := cgo.Handle(handle).Value().(*runState)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: redraw handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()

	// A wake-up repaint is not a live resize, so the window thread need not
	// wait for the GPU to schedule it.
	frame, err := state.redrawHandler()
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
	}
}

// vivaPressAt answers what a press at a window point does (a
// platform.PressKind): whether the window moves from there.
//
//export vivaPressAt
func vivaPressAt(handle C.uintptr_t, x C.double, y C.double) C.int {
	state := cgo.Handle(handle).Value().(*runState)
	if state.pressAt == nil {
		return C.int(platform.PressClient)
	}
	return C.int(state.pressAt(float32(x), float32(y)))
}

//export vivaKeyEvent
func vivaKeyEvent(handle C.uintptr_t, view unsafe.Pointer, key C.int, modifiers C.int, repeat C.int, utf8 *C.uchar, utf8Length C.size_t) {
	deliverKey(handle, view, KeyEvent{Key: input.Key(key), Modifiers: input.Modifiers(modifiers), Repeat: repeat != 0}, utf8, utf8Length)
}

// vivaRawKeyEvent is a key of a terminal that asked for releases. flags is 1
// for a repeat and 2 for a release; code is the virtual key code plus one.
//
//export vivaRawKeyEvent
func vivaRawKeyEvent(handle C.uintptr_t, view unsafe.Pointer, key C.int, modifiers C.int, flags C.int, code C.uint, utf8 *C.uchar, utf8Length C.size_t, typed *C.uchar, typedLength C.size_t) {
	event := KeyEvent{Key: input.Key(key), Modifiers: input.Modifiers(modifiers), Repeat: flags&1 != 0, Release: flags&2 != 0, Code: uint32(code)}
	if typedLength > 0 {
		event.Typed = C.GoStringN((*C.char)(unsafe.Pointer(typed)), C.int(typedLength))
	}
	deliverKey(handle, view, event, utf8, utf8Length)
}

// runStateFor resolves a callback handle, reporting false for a handle the
// window has already released; AppKit can deliver a late focus-lost callback
// while the window is closing.
func runStateFor(handle C.uintptr_t) (state *runState, ok bool) {
	defer func() {
		if recover() != nil {
			state, ok = nil, false
		}
	}()
	state = cgo.Handle(handle).Value().(*runState)
	return state, true
}

// vivaFocusLost reports that the window stopped being the keyboard window.
//
//export vivaFocusLost
func vivaFocusLost(handle C.uintptr_t, view unsafe.Pointer) {
	state, ok := runStateFor(handle)
	if !ok || state.closed {
		return
	}
	if state.focusLostHandler == nil {
		return
	}
	frame, err := state.focusLostHandler()
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
	}
}

// vivaAccessibilityAction carries a request from an accessibility client (VoiceOver
// pressing a button, a slider adjusted, a field's text set) to the application.
// It returns nonzero when the node took it.
//
//export vivaAccessibilityAction
func vivaAccessibilityAction(handle C.uintptr_t, view unsafe.Pointer, node C.uint64_t, action C.int, number C.double, utf8 *C.uchar, utf8Length C.size_t, start, end C.int) C.int {
	state, ok := runStateFor(handle)
	if !ok || state.closed || state.accessibility == nil {
		return 0
	}
	request := platform.AccessibilityRequest{Node: ui.IDFromBits(uint64(node)), Number: float64(number)}
	switch action {
	case C.VIVA_AX_PRESS:
		request.Action = platform.AccessibilityPress
	case C.VIVA_AX_FOCUS:
		request.Action = platform.AccessibilityFocus
	case C.VIVA_AX_INCREMENT:
		request.Action = platform.AccessibilityIncrement
	case C.VIVA_AX_DECREMENT:
		request.Action = platform.AccessibilityDecrement
	case C.VIVA_AX_SET_NUMBER:
		request.Action = platform.AccessibilitySetNumber
	case C.VIVA_AX_SET_TEXT:
		request.Action = platform.AccessibilitySetText
		request.Text = C.GoStringN((*C.char)(unsafe.Pointer(utf8)), C.int(utf8Length))
	case C.VIVA_AX_SET_SELECTION:
		request.Action = platform.AccessibilitySetSelection
		request.Start, request.End = int32(start), int32(end)
		if index := platform.AccessibilityIndexOf(state.nodes, request.Node); index >= 0 {
			value := state.nodes[index].Value
			request.Start, request.End = int32(platform.RuneOffset(value, int(start))), int32(platform.RuneOffset(value, int(end)))
		}
	default:
		return 0
	}
	handled, err := state.accessibility(request, func(frame Frame) error {
		state.captureFrameLayers(frame, nil)
		return presentFrame(state, view, frame, false)
	})
	if err != nil {
		state.err = err
		C.viva_view_close(view)
		return 0
	}
	if handled {
		return 1
	}
	return 0
}

func deliverKey(handle C.uintptr_t, view unsafe.Pointer, event KeyEvent, utf8 *C.uchar, utf8Length C.size_t) {
	state := cgo.Handle(handle).Value().(*runState)
	if state.keyHandler == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: key handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()

	chord := ""
	if utf8Length > 0 {
		chord = C.GoStringN((*C.char)(unsafe.Pointer(utf8)), C.int(utf8Length))
	}
	event.Text = chord
	frame, err := state.keyHandler(event)
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
	}
}

//export vivaShortcutEvent
func vivaShortcutEvent(handle C.uintptr_t, view unsafe.Pointer, modifiers C.int, repeat C.int, utf8 *C.uchar, utf8Length C.size_t) C.int {
	state := cgo.Handle(handle).Value().(*runState)
	if state.shortcutHandler == nil || utf8Length == 0 {
		return 0
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: shortcut handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()
	text := C.GoStringN((*C.char)(unsafe.Pointer(utf8)), C.int(utf8Length))
	frame, handled, err := state.shortcutHandler(KeyEvent{
		Modifiers: input.Modifiers(modifiers), Repeat: repeat != 0, Text: text,
	})
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
		return 1
	}
	if handled {
		return 1
	}
	return 0
}

//export vivaScrollEvent
func vivaScrollEvent(handle C.uintptr_t, view unsafe.Pointer, deltaX C.double, deltaY C.double, x C.double, y C.double, modifiers C.int, precise C.int) {
	state := cgo.Handle(handle).Value().(*runState)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: scroll handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()

	frame, err := state.scrollHandler(ScrollEvent{
		Modifiers: input.Modifiers(modifiers),
		X:         float32(x),
		Y:         float32(y),
		DeltaX:    float32(deltaX),
		DeltaY:    float32(deltaY),
		Precise:   precise != 0,
	})
	if err == nil {
		err = presentFrame(state, view, frame, true)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
	}
}

//export vivaFileDropEvent
func vivaFileDropEvent(handle C.uintptr_t, view unsafe.Pointer, phase C.int, x C.double, y C.double, paths *C.char, length C.size_t) C.int {
	state := cgo.Handle(handle).Value().(*runState)
	if state.dropHandler == nil {
		return 0
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: file drop handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()
	event := FileDropEvent{Phase: FileDropPhase(phase), X: float32(x), Y: float32(y)}
	if phase == C.int(FileDropDrop) && length > 0 {
		// The paths arrive joined by NUL, each ended by one.
		raw := C.GoBytes(unsafe.Pointer(paths), C.int(length))
		for len(raw) > 0 {
			end := 0
			for end < len(raw) && raw[end] != 0 {
				end++
			}
			if end > 0 {
				event.Paths = append(event.Paths, string(raw[:end]))
			}
			if end >= len(raw) {
				break
			}
			raw = raw[end+1:]
		}
	}
	frame, accepted, err := state.dropHandler(event)
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
		return 0
	}
	if accepted {
		return 1
	}
	return 0
}

//export vivaCloseRequested
func vivaCloseRequested(handle C.uintptr_t, view unsafe.Pointer) {
	state := cgo.Handle(handle).Value().(*runState)
	allow := func() { C.viva_view_close_confirmed(view) }
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: close handler panicked: %v", recovered)
			allow()
		}
	}()

	if state.closeHandler == nil {
		allow()
		return
	}
	state.closeHandler(allow)
}

//export vivaWindowClosed
func vivaWindowClosed(handle C.uintptr_t) {
	state := cgo.Handle(handle).Value().(*runState)
	state.closed = true
	if state.waker != nil && state.redrawHandler != nil {
		state.waker.Unbind()
	}
	state.application.windows--
	if state.application.firstErr == nil && state.err != nil {
		state.application.firstErr = state.err
	}
	if state.onClosed != nil {
		state.onClosed()
	}
	state.callbackHandle.Delete()
}

// textEventPlan says what the native layer has to redo after the application
// handled one text-system event.
type textEventPlan struct {
	// update: push the new state into the native editor, which rebuilds its
	// texture and restarts the caret.
	update bool
	// render: draw a frame even if the application's repaint changed nothing,
	// because the editor's own appearance did.
	render bool
}

// planTextEvent decides how much work a text event costs. An ordinary field
// always redraws: typing, moving the caret and selecting all change what its
// editor shows. A terminal's editor shows only the composition being typed
// through an input method, so an event that leaves that as it was (plain typed
// text, which goes straight to the program, or a paste) changes nothing on
// screen. Redrawing for it would submit a frame, and rebuild a texture, per
// keystroke only to show the same picture.
func planTextEvent(terminal bool, previous, next TextState) textEventPlan {
	if !platform.TextInputPresentationChanged(terminal, previous, next) {
		return textEventPlan{}
	}
	return textEventPlan{update: true, render: true}
}

func presentFrame(state *runState, view unsafe.Pointer, frame Frame, syncRender bool) error {
	if err := presentTextInput(state, view, frame.TextInput); err != nil {
		return err
	}
	if err := updateRectangles(state, view, frame.Update, frame.Rectangles, false); err != nil {
		return err
	}
	if frame.TextLabels != nil {
		if err := updateLabels(view, frame.TextLabels); err != nil {
			return err
		}
	}
	if frame.Accessibility != nil {
		if state != nil {
			state.nodes = frame.Accessibility
		}
		if err := presentAccessibility(view, frame.Accessibility); err != nil {
			return err
		}
	}
	if !frame.Update.Empty() || frame.TextLabels != nil || frame.Accessibility != nil ||
		frame.TextInput.Changed || state != nil && state.textEventRedraw {
		if syncRender {
			C.viva_view_render_sync(view)
		} else {
			C.viva_view_render(view)
		}
	}
	return nil
}

func presentTextInput(state *runState, view unsafe.Pointer, update TextInputUpdate) error {
	if !update.Changed {
		return nil
	}
	if state == nil {
		return fmt.Errorf("darwin: text input update without callback state")
	}
	if update.Input != nil && (update.Input.Width <= 0 || update.Input.Height <= 0 || update.Input.FontSize <= 0) {
		update.Input = nil
	}
	if err := validateTextInput(update.Input); err != nil {
		return err
	}
	if state.textInputAlreadyUpdated(update.Input) {
		state.setTextInput(update.Input)
		return nil
	}
	if update.Input != nil && state.textInputActive && onlyMoved(state.textInput, *update.Input) {
		// A terminal's cursor moved: follow it without cancelling a composition.
		moved := *update.Input
		if status := C.viva_view_move_text_input(view, C.float(moved.X), C.float(moved.Y), C.float(moved.Width), C.float(moved.Height)); status != 0 {
			return fmt.Errorf("darwin: native text input move failed with status %d", status)
		}
		state.setTextInput(update.Input)
		return nil
	}

	var native C.viva_text_input_config
	var memory unsafe.Pointer
	if update.Input != nil {
		memory = textBytes(update.Input.State.Value)
		if memory != nil {
			defer C.free(memory)
		}
		native = makeNativeTextInput(*update.Input, memory)
	}
	previousHandler := state.textHandler
	previousState := state.textState
	previousInput := state.textInput
	previousActive := state.textInputActive
	state.setTextInput(update.Input)
	status := C.viva_view_update_text_input(view, native)
	if status != 0 {
		state.textHandler = previousHandler
		state.textState = previousState
		state.textInput = previousInput
		state.textInputActive = previousActive
		return fmt.Errorf("darwin: native text input update failed with status %d", status)
	}
	return nil
}

func presentAccessibility(view unsafe.Pointer, nodes []AccessibilityNode) error {
	memory, strings := makeNativeAccessibility(nodes)
	if memory == nil {
		return nil
	}
	defer freeAccessibility(memory, strings)
	status := C.viva_view_update_accessibility(view, (*C.viva_accessibility_node)(memory), C.size_t(len(nodes)))
	if status != 0 {
		return fmt.Errorf("darwin: accessibility update failed with status %d", status)
	}
	return nil
}

//export vivaTextInput
func vivaTextInput(
	handle C.uintptr_t,
	view unsafe.Pointer,
	kind C.int,
	utf8 *C.uchar,
	utf8Length C.size_t,
	selectedStart C.int64_t,
	selectedLength C.int64_t,
	replacementStart C.int64_t,
	replacementLength C.int64_t,
	command C.int,
) {
	state := cgo.Handle(handle).Value().(*runState)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("darwin: AppKit callback", recovered)
			state.err = fmt.Errorf("darwin: text input handler panicked: %v", recovered)
			C.viva_view_close(view)
		}
	}()

	if state.textHandler == nil {
		state.err = fmt.Errorf("darwin: native text event without a handler")
		C.viva_view_close(view)
		return
	}

	value := ""
	if utf8Length > 0 {
		value = C.GoStringN((*C.char)(unsafe.Pointer(utf8)), C.int(utf8Length))
	}
	event := TextEvent{
		Kind:    TextEventKind(kind),
		Text:    value,
		Command: TextCommand(command),
	}
	if event.Kind == TextSetSelection {
		event.Selection = utf16Range(
			state.textState.Value,
			int(selectedStart),
			int(selectedLength),
		)
	}
	if event.Kind == TextSetMarked {
		event.MarkedSelection = utf16Range(value, int(selectedStart), int(selectedLength))
	}
	if replacementStart >= 0 {
		event.HasReplacement = true
		event.Replacement = utf16Range(
			state.textState.Value,
			int(replacementStart),
			int(replacementLength),
		)
	}

	previous := state.textState
	next, err := state.textHandler(event)
	if err == nil {
		err = validateTextState(next)
	}
	plan := planTextEvent(state.textInputActive && state.textInput.Terminal, previous, next)
	next = next.WithoutRedraw() // a one-shot request, not editor state
	if err == nil && plan.update {
		err = updateText(view, next, state.redrawHandler == nil)
	}
	if err != nil {
		state.err = err
		C.viva_view_close(view)
		return
	}
	state.textState = next
	if state.textInputActive {
		state.textInput.State = next
	}
	if state.redrawHandler != nil && plan.render {
		state.textEventRedraw = plan.render
		frame, redrawErr := state.redrawHandler()
		if redrawErr == nil {
			redrawErr = presentFrame(state, view, frame, false)
		}
		state.textEventRedraw = false
		if redrawErr != nil {
			state.err = redrawErr
			C.viva_view_close(view)
		}
	}
}

func utf16Range(value string, start, length int) text.Range {
	editor := text.NewEditor(value)
	if start < 0 {
		start = 0
	}
	if length < 0 {
		length = 0
	}
	return text.Range{
		Start: editor.UTF16ToRuneIndex(start),
		End:   editor.UTF16ToRuneIndex(start + length),
	}
}

func updateText(view unsafe.Pointer, state TextState, render bool) error {
	editor := text.NewEditor(state.Value)
	selectedStart := editor.RuneIndexToUTF16(state.Selection.Start)
	selectedEnd := editor.RuneIndexToUTF16(state.Selection.End)
	markedStart := editor.RuneIndexToUTF16(state.Composition.Start)
	markedEnd := editor.RuneIndexToUTF16(state.Composition.End)

	var data *C.uchar
	if state.Value != "" {
		data = (*C.uchar)(unsafe.Pointer(unsafe.StringData(state.Value)))
	}
	status := C.viva_view_update_text(
		view,
		data,
		C.size_t(len(state.Value)),
		C.size_t(selectedStart),
		C.size_t(selectedEnd-selectedStart),
		C.size_t(markedStart),
		C.size_t(markedEnd-markedStart),
		C.int(boolInt(state.Composing)),
		C.int(boolInt(render)),
	)
	if status != 0 {
		return fmt.Errorf("darwin: native text update failed with status %d", status)
	}
	return nil
}

func updateRectangles(state *runState, view unsafe.Pointer, update render.Update, rectangles []render.Rectangle, render bool) error {
	if update.Empty() {
		return nil
	}

	first := update.First
	count := update.Count
	selected := rectangles
	rebuild := C.int(0)
	if update.Rebuild {
		first = 0
		count = len(rectangles)
		rebuild = 1
	} else {
		if first < 0 || count < 0 || first > len(rectangles) || count > len(rectangles)-first {
			return fmt.Errorf("darwin: invalid rectangle update range first=%d count=%d total=%d", first, count, len(rectangles))
		}
		selected = rectangles[first : first+count]
	}

	renderFlag := C.int(0)
	if render {
		renderFlag = 1
	}
	if state != nil && cap(state.rectangleData) > 4096 && len(selected)*4 < cap(state.rectangleData) {
		state.rectangleData, state.rectangleClips = nil, nil
	}
	var pointer *C.viva_rectangle
	if len(selected) == 1 {
		native := makeNativeRectangle(selected[0])
		clip := makeNativeClip(selected[0])
		return rectangleUpdateStatus(C.viva_view_update_rectangles(
			view,
			&native,
			&clip,
			C.size_t(len(rectangles)),
			C.size_t(first),
			C.size_t(count),
			rebuild,
			renderFlag,
		))
	}

	// Reuse the window's staging buffers: a partial update of more than one
	// instance used to allocate both arrays on every frame, which is a steady
	// stream of garbage for an animation. The buffers are dropped when a large
	// full rebuild is followed by a small update, so they do not become a
	// permanent high-water mark.
	var native []C.viva_rectangle
	var clips []C.viva_clip
	if state != nil {
		if cap(state.rectangleData) < len(selected) {
			state.rectangleData = make([]C.viva_rectangle, len(selected))
			state.rectangleClips = make([]C.viva_clip, len(selected))
		} else {
			state.rectangleData = state.rectangleData[:len(selected)]
			state.rectangleClips = state.rectangleClips[:len(selected)]
		}
		native, clips = state.rectangleData, state.rectangleClips
	} else {
		native = make([]C.viva_rectangle, len(selected))
		clips = make([]C.viva_clip, len(selected))
	}
	for index, rectangle := range selected {
		native[index] = makeNativeRectangle(rectangle)
		clips[index] = makeNativeClip(rectangle)
	}
	var clipPointer *C.viva_clip
	if len(native) > 0 {
		pointer = &native[0]
		clipPointer = &clips[0]
	}
	return rectangleUpdateStatus(C.viva_view_update_rectangles(
		view,
		pointer,
		clipPointer,
		C.size_t(len(rectangles)),
		C.size_t(first),
		C.size_t(count),
		rebuild,
		renderFlag,
	))
}

func updateLabels(view unsafe.Pointer, labels []TextLabel) error {
	var memory unsafe.Pointer
	var strings []unsafe.Pointer
	var count int
	if len(labels) > 0 {
		var err error
		memory, strings, count, err = makeNativeTextLabels(labels)
		if err != nil {
			return err
		}
		defer freeNativeTextLabels(memory, strings)
	}
	status := C.viva_view_update_labels(view, (*C.viva_text_label)(memory), C.size_t(count))
	switch status {
	case 0:
		return nil
	case 2:
		return fmt.Errorf("darwin: failed to rebuild a text label")
	case 3:
		return fmt.Errorf("darwin: failed to rebuild text label textures")
	default:
		return fmt.Errorf("darwin: label update failed with status %d", status)
	}
}

// makeNativeRectangle packs one render primitive into the native instance
// layout: float4 bounds, float4 color, float4 style, float4 shade1, float4
// shade2. A shadow keeps its blur in the third style value and its offset and
// spread in shade1 (see the Metal vertex shader). Triangles reuse the slot as bounds=(p0, p1), style=(p2.x, layer, p2.y,
// edgeMask+1), color, shade1 and shade2 being the colors of p0, p1 and p2. Zero
// continues to identify rectangles while values 1..8 identify triangles and
// their three independently feathered boundary edges.
func makeNativeRectangle(rectangle render.Rectangle) C.viva_rectangle {
	if rectangle.Kind == scene.KindTriangle {
		colors := rectangle.VertexColors()
		return C.viva_rectangle{
			shade1_red: C.float(colors[1].Red), shade1_green: C.float(colors[1].Green), shade1_blue: C.float(colors[1].Blue), shade1_alpha: C.float(colors[1].Alpha),
			shade2_red: C.float(colors[2].Red), shade2_green: C.float(colors[2].Green), shade2_blue: C.float(colors[2].Blue), shade2_alpha: C.float(colors[2].Alpha),
			x:             C.float(rectangle.Points[0].X),
			y:             C.float(rectangle.Points[0].Y),
			width:         C.float(rectangle.Points[1].X),
			height:        C.float(rectangle.Points[1].Y),
			red:           C.float(rectangle.Color.Red),
			green:         C.float(rectangle.Color.Green),
			blue:          C.float(rectangle.Color.Blue),
			alpha:         C.float(rectangle.Color.Alpha),
			corner_radius: C.float(rectangle.Points[2].X),
			reserved0:     C.float(rectangle.Layer),
			reserved1:     C.float(rectangle.Points[2].Y),
			reserved2:     C.float(rectangle.EdgeMask + 1),
		}
	}
	native := C.viva_rectangle{
		x:             C.float(rectangle.Bounds.X),
		y:             C.float(rectangle.Bounds.Y),
		width:         C.float(rectangle.Bounds.Width),
		height:        C.float(rectangle.Bounds.Height),
		red:           C.float(rectangle.Color.Red),
		green:         C.float(rectangle.Color.Green),
		blue:          C.float(rectangle.Color.Blue),
		alpha:         C.float(rectangle.Color.Alpha),
		corner_radius: C.float(rectangle.Radius),
		reserved0:     C.float(rectangle.Layer),
	}
	if rectangle.Kind == scene.KindShadow && rectangle.Shadow.Blur > 0 {
		// A shadow's casting rectangle takes the slot's bounds and radius; its blur
		// is reserved1 and its offset and spread are shade1's first three values.
		native.reserved1 = C.float(rectangle.Shadow.Blur)
		native.shade1_red = C.float(rectangle.Shadow.OffsetX)
		native.shade1_green = C.float(rectangle.Shadow.OffsetY)
		native.shade1_blue = C.float(rectangle.Shadow.Spread)
	}
	return native
}

// makeNativeClip packs the region an instance may paint. An instance no clip
// cuts is sent with clipped 0 and a zero region, so a stale Clip never splits a
// batch.
func makeNativeClip(rectangle render.Rectangle) C.viva_clip {
	if !rectangle.Clipped {
		return C.viva_clip{}
	}
	return C.viva_clip{
		x:       C.float(rectangle.Clip.X),
		y:       C.float(rectangle.Clip.Y),
		width:   C.float(rectangle.Clip.Width),
		height:  C.float(rectangle.Clip.Height),
		clipped: 1,
	}
}

func rectangleUpdateStatus(status C.int) error {
	switch status {
	case 0:
		return nil
	case 2:
		return fmt.Errorf("darwin: native renderer rejected rectangle update range")
	case 3:
		return fmt.Errorf("darwin: failed to rebuild rectangle instance buffer")
	default:
		return fmt.Errorf("darwin: rectangle update failed with status %d", status)
	}
}

//export vivaOpenURL
func vivaOpenURL(url *C.char) {
	deliverOpenURL(C.GoString(url))
}

//export vivaApplicationQuitRequested
func vivaApplicationQuitRequested() C.int {
	if deliverQuitRequest() {
		return 1
	}
	return 0
}

//export vivaApplicationReopenRequested
func vivaApplicationReopenRequested() C.int {
	if deliverApplicationReopen() {
		return 1
	}
	return 0
}
