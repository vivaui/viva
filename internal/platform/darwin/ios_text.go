//go:build ios

package darwin

/*
#include "native.h"
*/
import "C"

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/imebridge"
)

// What the keyboard does to the text input, as ios_host.m sends it.
const (
	iosTextCommit = iota + 1
	iosTextSetMarked
	iosTextFinish
	iosTextReplace
	iosTextSelect
	iosTextDeleteBackward
	iosTextEnter
)

// vivaIOSTextOp is one thing the keyboard does to the focused text input. The ranges are
// UTF-16 units of the text the keyboard sees, which is the application's: each operation
// is read against the editor's state as it stands, and delivered as the text events the
// other platforms' text services make (deliverTextInputEvent), so the editor, the
// composition and the repaint are the same on every platform.
//
//export vivaIOSTextOp
func vivaIOSTextOp(handle C.uintptr_t, view unsafe.Pointer, operation C.int, utf8 *C.uchar, utf8Length C.size_t, first, last C.int64_t) {
	state, ok := runStateFor(handle)
	if !ok || state.closed || state.textHandler == nil {
		return
	}
	value := ""
	if utf8Length > 0 {
		value = C.GoStringN((*C.char)(unsafe.Pointer(utf8)), C.int(utf8Length))
	}
	session := &imebridge.Session{
		State: state.textState,
		Handler: func(event platform.TextEvent) (platform.TextState, error) {
			err := deliverTextInputEvent(state, view, event, true, false)
			return state.textState, err
		},
	}
	switch operation {
	case iosTextCommit:
		session.Commit(value, 1)
	case iosTextSetMarked:
		session.SetMarked(value, int(first), int(last))
	case iosTextFinish:
		session.Finish()
	case iosTextReplace:
		session.Replace(int(first), int(last), value)
	case iosTextSelect:
		session.Select(int(first), int(last))
	case iosTextDeleteBackward:
		session.Command(platform.TextDeleteBackward)
	case iosTextEnter:
		// Enter in a field of one line is the application's, as a key.
		_ = vivaIOSKeyEvent(handle, view, 0x28, 0, 0, nil, 0)
	}
}

// The text interaction's questions, asked on the main thread while UIKit lays out a
// selection: where a point is in the text, where the caret is at an index and what a range
// covers. They are the application's text geometry (platform.Config.TextGeometry); an
// application whose focused field cannot say answers false, and UIKit keeps the selection
// it has.

// vivaIOSTextIndexAt is the index of the character boundary nearest a point of the surface.
//
//export vivaIOSTextIndexAt
func vivaIOSTextIndexAt(handle C.uintptr_t, x, y C.float, index *C.int64_t) C.int {
	geometry := iosTextGeometry(handle)
	if geometry == nil {
		return 0
	}
	at, ok := geometry.IndexAt(float32(x), float32(y))
	if !ok {
		return 0
	}
	*index = C.int64_t(at)
	return 1
}

// vivaIOSTextCaretRect is the caret at an index, in the surface's points.
//
//export vivaIOSTextCaretRect
func vivaIOSTextCaretRect(handle C.uintptr_t, index C.int64_t, rect *C.float) C.int {
	geometry := iosTextGeometry(handle)
	if geometry == nil {
		return 0
	}
	caret, ok := geometry.CaretRect(int(index))
	if !ok {
		return 0
	}
	out := unsafe.Slice((*C.float)(rect), 4)
	out[0], out[1], out[2], out[3] = C.float(caret.X), C.float(caret.Y), C.float(caret.Width), C.float(caret.Height)
	return 1
}

// vivaIOSTextRangeRects fills up to capacity rectangles (x, y, width, height each) a range
// covers and returns how many.
//
//export vivaIOSTextRangeRects
func vivaIOSTextRangeRects(handle C.uintptr_t, start, end C.int64_t, rects *C.float, capacity C.int) C.int {
	geometry := iosTextGeometry(handle)
	if geometry == nil {
		return 0
	}
	covered := geometry.RangeRects(nil, int(start), int(end))
	out := unsafe.Slice((*C.float)(rects), int(capacity)*4)
	count := min(len(covered), int(capacity))
	for i := 0; i < count; i++ {
		out[i*4], out[i*4+1], out[i*4+2], out[i*4+3] = C.float(covered[i].X), C.float(covered[i].Y), C.float(covered[i].Width), C.float(covered[i].Height)
	}
	return C.int(count)
}

func iosTextGeometry(handle C.uintptr_t) platform.TextGeometry {
	state, ok := runStateFor(handle)
	if !ok || state.closed || state.textGeometry == nil {
		return nil
	}
	return state.textGeometry()
}
