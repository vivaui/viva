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
		vivaIOSKeyEvent(handle, view, 0x28, 0, 0, nil, 0)
	}
}
