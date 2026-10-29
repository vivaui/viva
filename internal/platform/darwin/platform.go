//go:build darwin

// Package darwin implements the native macOS application and Metal backend.
package darwin

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics -framework CoreText -framework Metal -framework QuartzCore
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type Color = platform.Color
type Config = platform.Config
type TextLabel = platform.TextLabel
type PointerEventKind = platform.PointerEventKind
type PointerEvent = platform.PointerEvent
type Size = platform.Size
type KeyEvent = platform.KeyEvent
type AccessibilityNode = platform.AccessibilityNode
type Frame = platform.Frame
type PointerHandler = platform.PointerHandler
type ResizeHandler = platform.ResizeHandler
type KeyHandler = platform.KeyHandler
type ShortcutHandler = platform.ShortcutHandler
type ScrollEvent = platform.ScrollEvent
type ScrollHandler = platform.ScrollHandler
type FileDropEvent = platform.FileDropEvent
type FileDropHandler = platform.FileDropHandler
type FileDropPhase = platform.FileDropPhase

const FileDropDrop = platform.FileDropDrop

type WindowMode = platform.WindowMode
type CloseRequestHandler = platform.CloseRequestHandler
type TextInput = platform.TextInput
type TextInputUpdate = platform.TextInputUpdate
type TextState = platform.TextState
type TextEventKind = platform.TextEventKind
type TextCommand = platform.TextCommand
type TextEvent = platform.TextEvent
type TextHandler = platform.TextHandler

const (
	PointerDown                     = platform.PointerDown
	PointerMove                     = platform.PointerMove
	PointerUp                       = platform.PointerUp
	PointerExit                     = platform.PointerExit
	WindowTitled                    = platform.WindowTitled
	WindowFrameless                 = platform.WindowFrameless
	WindowCustomTitlebar            = platform.WindowCustomTitlebar
	WindowPanel                     = platform.WindowPanel
	TextInsert                      = platform.TextInsert
	TextSetMarked                   = platform.TextSetMarked
	TextUnmark                      = platform.TextUnmark
	TextCommandEvent                = platform.TextCommandEvent
	TextSetSelection                = platform.TextSetSelection
	TextPaste                       = platform.TextPaste
	TextDeleteBackward              = platform.TextDeleteBackward
	TextDeleteForward               = platform.TextDeleteForward
	TextMoveLeft                    = platform.TextMoveLeft
	TextMoveRight                   = platform.TextMoveRight
	TextMoveLeftAndModifySelection  = platform.TextMoveLeftAndModifySelection
	TextMoveRightAndModifySelection = platform.TextMoveRightAndModifySelection
	TextInsertNewline               = platform.TextInsertNewline
	TextInsertTab                   = platform.TextInsertTab
	TextInsertLineBreak             = platform.TextInsertLineBreak
	TextUndo                        = platform.TextUndo
	TextRedo                        = platform.TextRedo
	TextDeleteLineStart             = platform.TextDeleteLineStart
	TextDeleteLineEnd               = platform.TextDeleteLineEnd
)

func EditorState(editor *text.Editor) TextState {
	return platform.EditorState(editor)
}

type runState struct {
	application      *nativeApplication
	callbackHandle   cgo.Handle
	onClosed         func()
	waker            *platform.Waker
	closed           bool
	pointerHandler   PointerHandler
	pressAt          func(x, y float32) platform.PressKind
	resizeHandler    ResizeHandler
	frameHandler     platform.FrameChangeHandler
	minimizeHandler  platform.MinimizeHandler
	onReopen         func()
	keyHandler       KeyHandler
	focusLostHandler platform.FocusLostHandler
	accessibility    platform.AccessibilityHandler
	// nodes is the accessibility tree last presented, for turning the UTF-16
	// ranges AppKit sends back into code points. It is the application's slice,
	// valid on the thread that presents it.
	nodes           []platform.AccessibilityNode
	shortcutHandler ShortcutHandler
	scrollHandler   ScrollHandler
	dropHandler     FileDropHandler
	redrawHandler   platform.RedrawHandler
	closeHandler    CloseRequestHandler
	textHandler     TextHandler
	textState       TextState
	textInput       TextInput
	textInputActive bool
	textEventRedraw bool
	rectangleLayers []uint16
	rectangleData   []C.viva_rectangle
	rectangleClips  []C.viva_clip
	err             error
}

type nativeApplication struct {
	windows  int
	firstErr error
}

type nativeWindow struct {
	token  unsafe.Pointer
	closed atomic.Bool
}

func (window *nativeWindow) Close() {
	if window != nil && !window.closed.Swap(true) {
		C.viva_application_close_window(window.token)
	}
}

// Hide and Show post to the event loop like Close, and do nothing for a window
// that is closing.
func (window *nativeWindow) Hide() {
	if window != nil && !window.closed.Load() {
		C.viva_application_hide_window(window.token)
	}
}

func (window *nativeWindow) Show() {
	if window != nil && !window.closed.Load() {
		C.viva_application_show_window(window.token)
	}
}

// Frame is the content area in screen coordinates.
func (window *nativeWindow) Frame() (ui.Rect, bool) {
	if window == nil || window.closed.Load() {
		return ui.Rect{}, false
	}
	var out [4]C.double
	if C.viva_application_window_frame(window.token, &out[0]) == 0 {
		return ui.Rect{}, false
	}
	return ui.Rect{X: float32(out[0]), Y: float32(out[1]), Width: float32(out[2]), Height: float32(out[3])}, true
}

func (state *runState) setTextInput(input *TextInput) {
	if input == nil {
		state.textHandler = nil
		state.textState = TextState{}
		state.textInput = TextInput{}
		state.textInputActive = false
		return
	}
	state.textHandler = input.OnEvent
	state.textState = input.State
	state.textInput = *input
	state.textInputActive = true
}

func (state *runState) textInputAlreadyUpdated(input *TextInput) bool {
	return state.textEventRedraw && input != nil && state.textInputActive &&
		sameTextInputPresentation(state.textInput, *input)
}

func sameTextInputPresentation(left, right TextInput) bool {
	return left.Target == right.Target && left.X == right.X && left.Y == right.Y &&
		left.Width == right.Width && left.Height == right.Height &&
		left.FontSize == right.FontSize && left.Layer == right.Layer && left.Multiline == right.Multiline && left.Secure == right.Secure &&
		left.Terminal == right.Terminal && left.MonoFace == right.MonoFace && left.AltSendsEscape == right.AltSendsEscape && left.KeyReleases == right.KeyReleases && left.Color == right.Color &&
		left.State == right.State
}

// onlyMoved reports whether next is the same terminal editor as previous but
// somewhere else: the cursor moved while nothing about the text did.
func onlyMoved(previous, next TextInput) bool {
	if !previous.Terminal || !next.Terminal {
		return false
	}
	previous.X, previous.Y, previous.Width, previous.Height = next.X, next.Y, next.Width, next.Height
	return sameTextInputPresentation(previous, next)
}

// Run enters the Cocoa event loop with one window.
func Run(config Config) error {
	return RunApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

// RunApplication enters one Cocoa event loop for every window opened by start.
func RunApplication(start platform.ApplicationStart) error {
	if status := C.viva_application_begin(); status != 0 {
		return nativeStatusError(status)
	}
	application := &nativeApplication{}
	defer application.SetQuitRequestHandler(nil)
	if err := start(application); err != nil {
		C.viva_application_close_all()
		if application.windows > 0 {
			_ = C.viva_application_run()
		}
		return err
	}
	status := C.viva_application_run()
	if application.firstErr != nil {
		return application.firstErr
	}
	return nativeStatusError(status)
}

func (application *nativeApplication) OpenWindow(config Config) (platform.Window, error) {
	title := C.CString(config.Title)
	defer C.free(unsafe.Pointer(title))

	if err := validateTextInput(config.TextInput); err != nil {
		return nil, err
	}

	var shaderMemory unsafe.Pointer
	var rectangleMemory, clipMemory unsafe.Pointer
	needsShaderLibrary := needsSceneShader(config)
	if needsShaderLibrary {
		shaderMemory = C.CBytes(sceneShaderLibrary)
		defer C.free(shaderMemory)
	}
	labelMemory, labelStrings, labelCount, err := makeNativeTextLabels(config.TextLabels)
	if err != nil {
		return nil, err
	}
	if labelMemory != nil {
		defer C.free(labelMemory)
	}
	for _, value := range labelStrings {
		defer C.free(value)
	}
	if len(config.Rectangles) > 0 {
		rectangleSize := C.size_t(unsafe.Sizeof(C.viva_rectangle{}))
		rectangleMemory = C.malloc(C.size_t(len(config.Rectangles)) * rectangleSize)
		if rectangleMemory == nil {
			return nil, fmt.Errorf("darwin: failed to allocate rectangle staging memory")
		}
		defer C.free(rectangleMemory)

		nativeRectangles := unsafe.Slice((*C.viva_rectangle)(rectangleMemory), len(config.Rectangles))
		for index, rectangle := range config.Rectangles {
			nativeRectangles[index] = makeNativeRectangle(rectangle)
		}

		clipSize := C.size_t(unsafe.Sizeof(C.viva_clip{}))
		clipMemory = C.malloc(C.size_t(len(config.Rectangles)) * clipSize)
		if clipMemory == nil {
			return nil, fmt.Errorf("darwin: failed to allocate rectangle clip staging memory")
		}
		defer C.free(clipMemory)
		nativeClips := unsafe.Slice((*C.viva_clip)(clipMemory), len(config.Rectangles))
		for index, rectangle := range config.Rectangles {
			nativeClips[index] = makeNativeClip(rectangle)
		}
	}

	state := &runState{application: application, onClosed: config.OnClosed, waker: config.Waker, closeHandler: config.OnCloseRequest, pressAt: config.PressAt,
		frameHandler: config.OnFrameChange, minimizeHandler: config.OnMinimize, onReopen: config.OnReopen}
	{
		state.captureRectangleLayers(config.Rectangles)
		if config.OnPointer != nil {
			state.pointerHandler = func(event PointerEvent) (Frame, error) {
				frame, err := config.OnPointer(event)
				state.captureFrameLayers(frame, err)
				return frame, err
			}
		}
		if config.OnResize != nil {
			state.resizeHandler = func(size Size) (Frame, error) {
				frame, err := config.OnResize(size)
				state.captureFrameLayers(frame, err)
				return frame, err
			}
		}
		if config.OnKey != nil {
			state.keyHandler = func(event KeyEvent) (Frame, error) {
				frame, err := config.OnKey(event)
				state.captureFrameLayers(frame, err)
				return frame, err
			}
		}
		if config.OnFocusLost != nil {
			state.focusLostHandler = func() (Frame, error) {
				frame, err := config.OnFocusLost()
				state.captureFrameLayers(frame, err)
				return frame, err
			}
		}
		state.accessibility, state.nodes = config.OnAccessibility, config.Accessibility
		if config.OnShortcut != nil {
			state.shortcutHandler = func(event KeyEvent) (Frame, bool, error) {
				frame, handled, err := config.OnShortcut(event)
				state.captureFrameLayers(frame, err)
				return frame, handled, err
			}
		}
		if config.OnScroll != nil {
			state.scrollHandler = func(event ScrollEvent) (Frame, error) {
				frame, err := config.OnScroll(event)
				state.captureFrameLayers(frame, err)
				return frame, err
			}
		}
		if config.OnFileDrop != nil {
			state.dropHandler = func(event FileDropEvent) (Frame, bool, error) {
				frame, accepted, err := config.OnFileDrop(event)
				state.captureFrameLayers(frame, err)
				return frame, accepted, err
			}
		}
		if config.OnRedraw != nil {
			state.redrawHandler = func() (Frame, error) {
				frame, err := config.OnRedraw()
				state.captureFrameLayers(frame, err)
				return frame, err
			}
		}
		if config.TextInput != nil {
			state.setTextInput(config.TextInput)
		}
		state.callbackHandle = cgo.NewHandle(state)
	}

	shaderLength := 0
	if needsShaderLibrary {
		shaderLength = len(sceneShaderLibrary)
	}

	var nativeText C.viva_text_input_config
	var textMemory unsafe.Pointer
	if config.TextInput != nil {
		textMemory = textBytes(config.TextInput.State.Value)
		if textMemory != nil {
			defer C.free(textMemory)
		}
		nativeText = makeNativeTextInput(*config.TextInput, textMemory)
	}

	accessibilityMemory, accessibilityStrings := makeNativeAccessibility(config.Accessibility)
	if accessibilityMemory != nil {
		defer freeAccessibility(accessibilityMemory, accessibilityStrings)
	}

	if config.Waker != nil && config.OnRedraw != nil {
		config.Waker.Bind(func() { C.viva_request_redraw(C.uintptr_t(state.callbackHandle)) })
	}
	token := C.viva_application_open_window(C.viva_window_config{
		title:                   title,
		width:                   C.int(config.Width),
		height:                  C.int(config.Height),
		min_width:               C.int(config.MinWidth),
		min_height:              C.int(config.MinHeight),
		clear_red:               C.double(config.ClearColor.Red),
		clear_green:             C.double(config.ClearColor.Green),
		clear_blue:              C.double(config.ClearColor.Blue),
		clear_alpha:             C.double(config.ClearColor.Alpha),
		shader_library:          (*C.uchar)(shaderMemory),
		shader_library_length:   C.size_t(shaderLength),
		rectangles:              (*C.viva_rectangle)(rectangleMemory),
		rectangle_clips:         (*C.viva_clip)(clipMemory),
		rectangle_count:         C.size_t(len(config.Rectangles)),
		text_labels:             (*C.viva_text_label)(labelMemory),
		text_label_count:        C.size_t(labelCount),
		accessibility:           (*C.viva_accessibility_node)(accessibilityMemory),
		accessibility_count:     C.size_t(len(config.Accessibility)),
		text_input:              nativeText,
		pointer_events_enabled:  C.int(boolInt(config.OnPointer != nil)),
		resize_events_enabled:   C.int(boolInt(config.OnResize != nil)),
		frame_events_enabled:    C.int(boolInt(config.OnFrameChange != nil)),
		minimize_events_enabled: C.int(boolInt(config.OnMinimize != nil)),
		key_events_enabled:      C.int(boolInt(config.OnKey != nil || config.OnShortcut != nil)),
		scroll_events_enabled:   C.int(boolInt(config.OnScroll != nil)),
		file_drop_enabled:       C.int(boolInt(config.OnFileDrop != nil)),
		redraw_events_enabled:   C.int(boolInt(config.OnRedraw != nil)),
		dynamic_text_input:      C.int(boolInt(config.DynamicTextInput)),
		window_mode:             C.int(int(config.WindowMode)),
		titlebar_height:         C.float(config.TitlebarStrip()),
		close_intercept_enabled: C.int(boolInt(config.OnCloseRequest != nil)),
		callback_handle:         C.uintptr_t(state.callbackHandle),
		has_anchor:              C.int(boolInt(config.HasAnchor)),
		anchor_x:                C.float(config.Anchor.X),
		anchor_y:                C.float(config.Anchor.Y),
		anchor_width:            C.float(config.Anchor.Width),
		anchor_height:           C.float(config.Anchor.Height),
		has_origin:              C.int(boolInt(config.HasOrigin)),
		origin_x:                C.float(config.Origin.X),
		origin_y:                C.float(config.Origin.Y),
	})
	if token == nil {
		if config.Waker != nil && config.OnRedraw != nil {
			config.Waker.Unbind()
		}
		if !state.closed {
			state.callbackHandle.Delete()
		}
		return nil, nativeStatusError(C.viva_application_last_status())
	}
	application.windows++
	setApplicationIcon(config.Icon)
	return &nativeWindow{token: token}, nil
}

func nativeStatusError(status C.int) error {
	switch status {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("darwin: Metal is unavailable")
	case 2:
		return fmt.Errorf("darwin: failed to create Metal command queue")
	case 3:
		return fmt.Errorf("darwin: failed to load precompiled Metal shader library")
	case 4:
		return fmt.Errorf("darwin: failed to create rectangle render pipeline")
	case 5:
		return fmt.Errorf("darwin: failed to create rectangle instance buffer")
	case 6:
		return fmt.Errorf("darwin: failed to create text render pipeline")
	case 7:
		return fmt.Errorf("darwin: failed to create initial text texture")
	case 8:
		return fmt.Errorf("darwin: application loop is already running")
	default:
		return fmt.Errorf("darwin: native application failed with status %d", status)
	}
}

func (state *runState) captureFrameLayers(frame Frame, err error) {
	if err == nil && !frame.Update.Empty() {
		state.captureRectangleLayers(frame.Rectangles)
	}
}

func (state *runState) captureRectangleLayers(rectangles []render.Rectangle) {
	if cap(state.rectangleLayers) < len(rectangles) {
		state.rectangleLayers = make([]uint16, len(rectangles))
	} else {
		state.rectangleLayers = state.rectangleLayers[:len(rectangles)]
	}
	for index, rectangle := range rectangles {
		state.rectangleLayers[index] = rectangle.Layer
	}
}

//export vivaRectangleLayer
func vivaRectangleLayer(handle C.uintptr_t, index C.size_t) C.uint16_t {
	if handle == 0 {
		return 0
	}
	state := cgo.Handle(handle).Value().(*runState)
	if uint64(index) >= uint64(len(state.rectangleLayers)) {
		return 0
	}
	return C.uint16_t(state.rectangleLayers[int(index)])
}

func makeNativeAccessibility(nodes []AccessibilityNode) (unsafe.Pointer, []unsafe.Pointer) {
	if len(nodes) == 0 {
		return nil, nil
	}
	size := C.size_t(unsafe.Sizeof(C.viva_accessibility_node{}))
	memory := C.malloc(C.size_t(len(nodes)) * size)
	if memory == nil {
		panic("darwin: failed to allocate accessibility staging memory")
	}
	native := unsafe.Slice((*C.viva_accessibility_node)(memory), len(nodes))
	strings := make([]unsafe.Pointer, 0, len(nodes))
	for index, node := range nodes {
		label := C.CString(node.Label)
		value := C.CString(node.Value)
		hint := C.CString(node.Hint)
		description := C.CString(node.Description)
		strings = append(strings, unsafe.Pointer(label), unsafe.Pointer(value), unsafe.Pointer(hint), unsafe.Pointer(description))
		selectionStart, selectionEnd, textLength := nativeSelection(node)
		native[index] = C.viva_accessibility_node{
			role:         C.uint8_t(node.Role),
			x:            C.float(node.X),
			y:            C.float(node.Y),
			width:        C.float(node.Width),
			height:       C.float(node.Height),
			label:        label,
			label_length: C.size_t(len(node.Label)),
			value:        value,
			value_length: C.size_t(len(node.Value)),
			hint:         hint,
			hint_length:  C.size_t(len(node.Hint)),
			focused:      C.int(boolInt(node.Focused)),
			disabled:     C.int(boolInt(node.Disabled)),
			checked:      C.int(boolInt(node.Checked)),
			mixed:        C.int(boolInt(node.Mixed)),
			selected:     C.int(boolInt(node.Selected)),
			expanded:     C.int(boolInt(node.Expanded)),
			multiline:    C.int(boolInt(node.Multiline)),
			read_only:    C.int(boolInt(node.ReadOnly)),
			secure:       C.int(boolInt(node.Secure)),

			id:                 C.uint64_t(node.ID.Bits()),
			focusable:          C.int(boolInt(node.Focusable)),
			expandable:         C.int(boolInt(node.Expandable)),
			description:        description,
			description_length: C.size_t(len(node.Description)),
			level:              C.int(node.Level),
			pos_in_set:         C.int(node.PosInSet),
			set_size:           C.int(node.SetSize),
			live:               C.int(node.Live),
			range_set:          C.int(boolInt(node.Range.Set)),
			range_now:          C.double(node.Range.Now),
			range_min:          C.double(node.Range.Min),
			range_max:          C.double(node.Range.Max),
			range_step:         C.double(node.Range.Step),
			selection_valid:    C.int(boolInt(node.Selection.Valid)),
			selection_start:    C.int(selectionStart),
			selection_end:      C.int(selectionEnd),
			text_length:        C.int(textLength),
		}
	}
	// The native side copies labels into NSString during the update call, so
	// callers free these strings once that call has returned.
	return memory, strings
}

// nativeSelection is a node's selection in the UTF-16 units AppKit counts text
// in, and the length of its text in the same.
func nativeSelection(node AccessibilityNode) (start, end, length int) {
	if !node.Selection.Valid {
		return 0, 0, 0
	}
	return platform.UTF16Offset(node.Value, int(node.Selection.Start)), platform.UTF16Offset(node.Value, int(node.Selection.End)), platform.UTF16Offset(node.Value, -1)
}

func freeAccessibility(memory unsafe.Pointer, strings []unsafe.Pointer) {
	for _, value := range strings {
		C.free(value)
	}
	C.free(memory)
}

// native.h's VIVA_RUN_* bits. Keep these values in step with that enum.
const (
	runItalic    = 1
	runUnderline = 2
	runStrike    = 4
	runLigatures = 8
)

// textRunFlags is the native flag word of one fixed-pitch run.
func textRunFlags(run ui.TextRun) uint32 {
	var flags uint32
	if run.Italic {
		flags |= runItalic
	}
	if run.Underline {
		flags |= runUnderline
	}
	if run.Strike {
		flags |= runStrike
	}
	if run.Ligatures {
		flags |= runLigatures
	}
	return flags
}

func makeNativeTextLabels(labels []TextLabel) (unsafe.Pointer, []unsafe.Pointer, int, error) {
	if len(labels) == 0 {
		return nil, nil, 0, nil
	}
	syncMonoFamilies()
	size := C.size_t(unsafe.Sizeof(C.viva_text_label{}))
	memory := C.malloc(C.size_t(len(labels)) * size)
	if memory == nil {
		return nil, nil, 0, fmt.Errorf("darwin: failed to allocate text label staging memory")
	}
	strings := make([]unsafe.Pointer, 0, len(labels))
	native := unsafe.Slice((*C.viva_text_label)(memory), len(labels))
	fail := func(err error) (unsafe.Pointer, []unsafe.Pointer, int, error) {
		for _, value := range strings {
			C.free(value)
		}
		C.free(memory)
		return nil, nil, 0, err
	}
	validCount := 0
	for index, label := range labels {
		bitmap := len(label.Bitmap) > 0
		if label.Width <= 0 || label.Height <= 0 || !bitmap && label.FontSize <= 0 {
			continue
		}
		value := textBytes(label.Value)
		if value != nil {
			strings = append(strings, value)
		}
		var runs *C.viva_text_run
		if len(label.Runs) > 0 {
			if label.CellWidth <= 0 {
				return fail(fmt.Errorf("darwin: text label %d has runs but no cell width", index))
			}
			runMemory := C.malloc(C.size_t(len(label.Runs)) * C.size_t(unsafe.Sizeof(C.viva_text_run{})))
			if runMemory == nil {
				return fail(fmt.Errorf("darwin: failed to allocate text run staging memory"))
			}
			strings = append(strings, runMemory)
			packed := unsafe.Slice((*C.viva_text_run)(runMemory), len(label.Runs))
			for at, run := range label.Runs {
				packed[at] = C.viva_text_run{
					column: C.uint32_t(max(run.Column, 0)),
					bytes:  C.uint32_t(max(run.Bytes, 0)),
					cells:  C.uint32_t(max(run.Cells, 0)),
					flags:  C.uint32_t(textRunFlags(run)),
					weight: C.float(run.Weight),
					red:    C.float(run.Color.Red),
					green:  C.float(run.Color.Green),
					blue:   C.float(run.Color.Blue),
					alpha:  C.float(run.Color.Alpha),
				}
			}
			runs = (*C.viva_text_run)(runMemory)
		}
		var flags C.uint32_t
		if label.Mono || len(label.Runs) > 0 {
			flags |= C.VIVA_LABEL_MONO
		}
		if label.Clipped {
			flags |= C.VIVA_LABEL_CLIPPED
		}
		var bitmapMemory unsafe.Pointer
		if bitmap {
			if label.BitmapWidth <= 0 || label.BitmapHeight <= 0 || label.BitmapStride < label.BitmapWidth*4 {
				return fail(fmt.Errorf("darwin: bitmap label %d has invalid dimensions", index))
			}
			required := (label.BitmapHeight-1)*label.BitmapStride + label.BitmapWidth*4
			if required < 0 || len(label.Bitmap) < required {
				return fail(fmt.Errorf("darwin: bitmap label %d has a short payload", index))
			}
			if label.BitmapUpload {
				bitmapMemory = C.CBytes(label.Bitmap[:required])
				if bitmapMemory == nil {
					return fail(fmt.Errorf("darwin: failed to allocate bitmap staging memory"))
				}
				strings = append(strings, bitmapMemory)
			}
			flags |= C.VIVA_LABEL_BITMAP
		}
		native[validCount] = C.viva_text_label{
			utf8:           (*C.uchar)(value),
			utf8_length:    C.size_t(len(label.Value)),
			x:              C.float(label.X),
			y:              C.float(label.Y),
			width:          C.float(label.Width),
			height:         C.float(label.Height),
			font_size:      C.float(label.FontSize),
			font_weight:    C.float(label.Weight),
			red:            C.float(label.Color.Red),
			green:          C.float(label.Color.Green),
			blue:           C.float(label.Color.Blue),
			alpha:          C.float(label.Color.Alpha),
			layer:          C.uint16_t(label.Layer),
			mono_face:      C.uint16_t(label.MonoFace),
			runs:           runs,
			run_count:      C.size_t(len(label.Runs)),
			cell_width:     C.float(label.CellWidth),
			flags:          flags,
			bitmap:         (*C.uchar)(bitmapMemory),
			bitmap_length:  C.size_t(len(label.Bitmap)),
			bitmap_width:   C.uint32_t(max(label.BitmapWidth, 0)),
			bitmap_height:  C.uint32_t(max(label.BitmapHeight, 0)),
			bitmap_stride:  C.uint32_t(max(label.BitmapStride, 0)),
			bitmap_id:      C.uint64_t(label.BitmapID),
			bitmap_version: C.uint32_t(label.BitmapVersion),
			clip_x:         C.float(label.Clip.X),
			clip_y:         C.float(label.Clip.Y),
			clip_width:     C.float(label.Clip.Width),
			clip_height:    C.float(label.Clip.Height),
			origin_x:       C.float(label.OriginX),
			origin_y:       C.float(label.OriginY),
		}
		validCount++
	}
	if validCount == 0 {
		for _, value := range strings {
			C.free(value)
		}
		C.free(memory)
		return nil, nil, 0, nil
	}
	return memory, strings, validCount, nil
}

func freeNativeTextLabels(memory unsafe.Pointer, strings []unsafe.Pointer) {
	for _, value := range strings {
		C.free(value)
	}
	if memory != nil {
		C.free(memory)
	}
}

func validateTextInput(input *TextInput) error {
	if input == nil {
		return nil
	}
	if input.Width <= 0 || input.Height <= 0 || input.FontSize <= 0 {
		return fmt.Errorf("darwin: text input dimensions and font size must be positive")
	}
	if input.OnEvent == nil {
		return fmt.Errorf("darwin: text input handler is required")
	}
	return validateTextState(input.State)
}

func needsSceneShader(config Config) bool {
	return len(config.Rectangles) > 0 || len(config.TextLabels) > 0 ||
		config.OnPointer != nil || config.OnResize != nil ||
		config.OnKey != nil || config.OnShortcut != nil || config.OnRedraw != nil || config.TextInput != nil ||
		config.DynamicTextInput
}

func validateTextState(state TextState) error {
	runeCount := len([]rune(state.Value))
	if !validTextRange(state.Selection, runeCount) {
		return fmt.Errorf("darwin: invalid text selection %+v for %d runes", state.Selection, runeCount)
	}
	if state.Composing && !validTextRange(state.Composition, runeCount) {
		return fmt.Errorf("darwin: invalid text composition %+v for %d runes", state.Composition, runeCount)
	}
	return nil
}

func validTextRange(value text.Range, length int) bool {
	return value.Start >= 0 && value.End >= value.Start && value.End <= length
}

func makeNativeTextInput(input TextInput, value unsafe.Pointer) C.viva_text_input_config {
	syncMonoFamilies()
	editor := text.NewEditor(input.State.Value)
	selectedStart := editor.RuneIndexToUTF16(input.State.Selection.Start)
	selectedEnd := editor.RuneIndexToUTF16(input.State.Selection.End)
	markedStart := editor.RuneIndexToUTF16(input.State.Composition.Start)
	markedEnd := editor.RuneIndexToUTF16(input.State.Composition.End)
	return C.viva_text_input_config{
		utf8:             (*C.uchar)(value),
		utf8_length:      C.size_t(len(input.State.Value)),
		x:                C.float(input.X),
		y:                C.float(input.Y),
		width:            C.float(input.Width),
		height:           C.float(input.Height),
		font_size:        C.float(input.FontSize),
		red:              C.float(input.Color.Red),
		green:            C.float(input.Color.Green),
		blue:             C.float(input.Color.Blue),
		alpha:            C.float(input.Color.Alpha),
		selected_start:   C.size_t(selectedStart),
		selected_length:  C.size_t(selectedEnd - selectedStart),
		marked_start:     C.size_t(markedStart),
		marked_length:    C.size_t(markedEnd - markedStart),
		has_marked_text:  C.int(boolInt(input.State.Composing)),
		multiline:        C.int(boolInt(input.Multiline)),
		secure:           C.int(boolInt(input.Secure)),
		terminal:         C.int(boolInt(input.Terminal)),
		mono_face:        C.int(input.MonoFace),
		alt_sends_escape: C.int(boolInt(input.AltSendsEscape)),
		key_releases:     C.int(boolInt(input.KeyReleases)),
		navigation_keys:  C.int(boolInt(input.NavigationKeys)),
		placed:           C.int(boolInt(input.Placed)),
		layer:            C.uint16_t(input.Layer),
		caret_x:          C.float(input.CaretX),
		caret_y:          C.float(input.CaretY),
		caret_width:      C.float(input.CaretWidth),
		caret_height:     C.float(input.CaretHeight),
		enabled:          1,
	}
}

func nativeTextInputRowKeyIsRaw(placed, composing bool, key int, command bool) bool {
	return C.viva_text_input_row_key_is_raw(
		C.int(boolInt(placed)), C.int(boolInt(composing)), C.int(key), C.int(boolInt(command)),
	) != 0
}

func nativeTextInputNavigationKeyIsRaw(navigationKeys, composing bool, key int) bool {
	return C.viva_text_input_navigation_key_is_raw(
		C.int(boolInt(navigationKeys)), C.int(boolInt(composing)), C.int(key),
	) != 0
}

func nativeTextInputShouldInterpretKey(enabled, multiline bool, key int) bool {
	return C.viva_text_input_should_interpret_key(
		C.int(boolInt(enabled)), C.int(boolInt(multiline)), C.int(key),
	) != 0
}

func textBytes(value string) unsafe.Pointer {
	if value == "" {
		return nil
	}
	return C.CBytes([]byte(value))
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// fragmentBlendsPremultiplied reports whether the named Metal fragment function
// outputs premultiplied colors and so needs a One source blend factor.
func fragmentBlendsPremultiplied(name string) bool {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return C.viva_fragment_blends_premultiplied(cName) != 0
}

func nativeCaretShouldBlink(enabled bool, selectedLength int, composing, terminal bool) bool {
	return C.viva_text_input_caret_should_blink(
		C.int(boolInt(enabled)),
		C.size_t(selectedLength),
		C.int(boolInt(composing)),
		C.int(boolInt(terminal)),
	) != 0
}

// nativeTerminalKeyRoutesRaw reports whether a key typed into a terminal goes
// to the application as a raw key event rather than to the text system.
func nativeTerminalKeyRoutesRaw(composing, namedKey, control, command, alt, altSendsEscape bool) bool {
	return C.viva_terminal_key_routes_raw(
		C.int(boolInt(composing)), C.int(boolInt(namedKey)), C.int(boolInt(control)), C.int(boolInt(command)),
		C.int(boolInt(alt)), C.int(boolInt(altSendsEscape)),
	) != 0
}

// nativeTerminalKeyCode names the key a character stands for in a terminal, or
// KeyUnknown for characters that are text.
func nativeTerminalKeyCode(character uint16) input.Key {
	return input.Key(C.viva_terminal_key_code(C.ushort(character)))
}

type compositeCommand struct {
	Layer   uint16
	Kind    uint16
	First   uint32
	Count   uint32
	Clipped bool
	Clip    scene.Rect
}

func planComposite(rectangles []render.Rectangle, labels []TextLabel) ([]compositeCommand, error) {
	nativeRectangles := make([]C.viva_rectangle, len(rectangles))
	nativeClips := make([]C.viva_clip, len(rectangles))
	for index, rectangle := range rectangles {
		nativeRectangles[index] = C.viva_rectangle{reserved0: C.float(rectangle.Layer)}
		// The clip fields are sent as they are, so it is the native plan that
		// ignores a Clip on an instance that is not Clipped.
		nativeClips[index] = C.viva_clip{
			x: C.float(rectangle.Clip.X), y: C.float(rectangle.Clip.Y),
			width: C.float(rectangle.Clip.Width), height: C.float(rectangle.Clip.Height),
			clipped: C.uint32_t(boolInt(rectangle.Clipped)),
		}
	}
	textLayers := make([]C.uint16_t, len(labels))
	for index, label := range labels {
		textLayers[index] = C.uint16_t(label.Layer)
	}
	nativeCommands := make([]C.viva_composite_command, len(rectangles)+len(labels))
	var rectanglePointer *C.viva_rectangle
	var clipPointer *C.viva_clip
	if len(nativeRectangles) > 0 {
		rectanglePointer = &nativeRectangles[0]
		clipPointer = &nativeClips[0]
	}
	var textPointer *C.uint16_t
	if len(textLayers) > 0 {
		textPointer = &textLayers[0]
	}
	var commandPointer *C.viva_composite_command
	if len(nativeCommands) > 0 {
		commandPointer = &nativeCommands[0]
	}
	var count C.size_t
	status := C.viva_plan_composite(
		rectanglePointer,
		clipPointer,
		C.size_t(len(nativeRectangles)),
		textPointer,
		C.size_t(len(textLayers)),
		commandPointer,
		C.size_t(len(nativeCommands)),
		&count,
	)
	if status != 0 {
		return nil, fmt.Errorf("darwin: failed to plan composite commands with status %d", status)
	}
	commands := make([]compositeCommand, int(count))
	for index := range commands {
		commands[index] = compositeCommand{
			Layer: uint16(nativeCommands[index].layer),
			Kind:  uint16(nativeCommands[index].kind),
			First: uint32(nativeCommands[index].first),
			Count: uint32(nativeCommands[index].count),
		}
		if nativeCommands[index].clipped != 0 {
			commands[index].Clipped = true
			commands[index].Clip = scene.Rect{
				X:      float32(nativeCommands[index].clip_x),
				Y:      float32(nativeCommands[index].clip_y),
				Width:  float32(nativeCommands[index].clip_width),
				Height: float32(nativeCommands[index].clip_height),
			}
		}
	}
	return commands, nil
}

func compositeABI() (rectangleSize, rectangleLayerOffset, textSize, textLayerOffset, commandSize uintptr) {
	return unsafe.Sizeof(C.viva_rectangle{}),
		unsafe.Offsetof(C.viva_rectangle{}.reserved0),
		unsafe.Sizeof(C.viva_text_label{}),
		unsafe.Offsetof(C.viva_text_label{}.layer),
		unsafe.Sizeof(C.viva_composite_command{})
}

// labelABI reports the layout of the label structures shared with native code.
func labelABI() (labelSize, runsOffset, runSize uintptr) {
	return unsafe.Sizeof(C.viva_text_label{}),
		unsafe.Offsetof(C.viva_text_label{}.runs),
		unsafe.Sizeof(C.viva_text_run{})
}

// planLabelReuse runs the native plan that decides which old label texture each
// new label takes over: source[i] is the old index or -1.
func planLabelReuse(old, next []uint64) ([]int64, error) {
	source := make([]int64, len(next))
	var oldPointer, nextPointer *C.uint64_t
	var sourcePointer *C.int64_t
	if len(old) > 0 {
		oldPointer = (*C.uint64_t)(unsafe.Pointer(&old[0]))
	}
	if len(next) > 0 {
		nextPointer = (*C.uint64_t)(unsafe.Pointer(&next[0]))
		sourcePointer = (*C.int64_t)(unsafe.Pointer(&source[0]))
	}
	if status := C.viva_plan_label_reuse(oldPointer, C.size_t(len(old)), nextPointer, C.size_t(len(next)), sourcePointer); status != 0 {
		return nil, fmt.Errorf("darwin: label reuse planning failed with status %d", status)
	}
	return source, nil
}
