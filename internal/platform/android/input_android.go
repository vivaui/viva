//go:build android

package android

/*
#include "jni_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/imebridge"
)

// Android's key codes of the keys that are not text, and its meta state.
const (
	keycodeBack        = 4
	keycodeDpadUp      = 19
	keycodeDpadDown    = 20
	keycodeDpadLeft    = 21
	keycodeDpadRight   = 22
	keycodeTab         = 61
	keycodeSpace       = 62
	keycodeEnter       = 66
	keycodeDelete      = 67 // the key that deletes backward
	keycodeEscape      = 111
	keycodeForwardDel  = 112
	keycodeMoveHome    = 122
	keycodeMoveEnd     = 123
	keycodeInsert      = 124
	keycodePageUp      = 92
	keycodePageDown    = 93
	keycodeF1          = 131
	keycodeF12         = 142
	keycodeNumpadEnter = 160
)

// keyOfCode is the key an Android key code names, for the keys that are not text.
func keyOfCode(code int32) gfx.Key {
	if code >= keycodeF1 && code <= keycodeF12 {
		return gfx.KeyF1 + gfx.Key(code-keycodeF1)
	}
	switch code {
	case keycodeDpadUp:
		return gfx.KeyUp
	case keycodeDpadDown:
		return gfx.KeyDown
	case keycodeDpadLeft:
		return gfx.KeyLeft
	case keycodeDpadRight:
		return gfx.KeyRight
	case keycodeTab:
		return gfx.KeyTab
	case keycodeSpace:
		return gfx.KeySpace
	case keycodeEnter, keycodeNumpadEnter:
		return gfx.KeyEnter
	case keycodeDelete:
		return gfx.KeyBackspace
	case keycodeEscape:
		return gfx.KeyEscape
	case keycodeForwardDel:
		return gfx.KeyDelete
	case keycodeMoveHome:
		return gfx.KeyHome
	case keycodeMoveEnd:
		return gfx.KeyEnd
	case keycodeInsert:
		return gfx.KeyInsert
	case keycodePageUp:
		return gfx.KeyPageUp
	case keycodePageDown:
		return gfx.KeyPageDown
	}
	return gfx.KeyUnknown
}

// textOperation is what the input method does to the text input.
type textOperation uint8

const (
	opCommit textOperation = iota + 1
	opSetComposing
	opFinish
	opDeleteSurrounding
	opSelect
	opMarkRange
	opAction
	opKey
)

// textInput is the text input the application has focused.
type textInput struct {
	session imebridge.Session
	input   *platform.TextInput
}

// setTextInput follows the application's wish for its text input: another one, the
// same one with other text, or none (platform.TextInputUpdate).
func (win *window) setTextInput(update platform.TextInputUpdate) {
	if !update.Changed {
		return
	}
	if update.Input == nil {
		if win.text != nil {
			win.text = nil
			C.viva_host_keyboard(0, 0, 0)
		}
		return
	}
	input := update.Input
	restart := win.text == nil || win.text.input.Target != input.Target || win.text.input.Multiline != input.Multiline || win.text.input.Secure != input.Secure
	win.text = &textInput{session: imebridge.Session{Handler: input.OnEvent, State: input.State}, input: input}
	win.pushEditor()
	if restart {
		C.viva_host_keyboard(1, boolInt(input.Multiline), boolInt(input.Secure))
	}
}

func boolInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

// pushEditor gives the host the text input's state, for the input method to read.
func (win *window) pushEditor() {
	mirror := win.text.session.Mirror()
	units := utf16Units(mirror.Text)
	var data *C.uint16_t
	if len(units) > 0 {
		data = (*C.uint16_t)(unsafe.Pointer(&units[0]))
	}
	C.viva_host_editor(data, C.int(len(units)), C.int(mirror.SelStart), C.int(mirror.SelEnd), C.int(mirror.CompStart), C.int(mirror.CompEnd))
}

// edit is one edit of the input method.
type edit struct {
	operation    textOperation
	text         string
	first, last  int32 // newCursor, the range, or what to delete before and after
	codePoints   bool
	editorAction int32
	action       imebridge.KeyAction
}

// textEdit applies an input method's edit to the application's text input, sends the state
// after it back, and shows what the application painted for it.
func (application *application) textEdit(ev event) {
	if ev.ack != nil {
		defer close(ev.ack)
	}
	win := application.window
	if win == nil || win.closed || win.text == nil {
		return
	}
	defer application.guard()
	text, session := win.text, &win.text.session
	var err error
	switch ev.edit.operation {
	case opCommit:
		err = session.Commit(ev.edit.text, int(ev.edit.first))
	case opSetComposing:
		err = session.SetComposing(ev.edit.text, int(ev.edit.first))
	case opFinish:
		err = session.Finish()
	case opDeleteSurrounding:
		err = session.DeleteSurrounding(int(ev.edit.first), int(ev.edit.last), ev.edit.codePoints)
	case opSelect:
		err = session.Select(int(ev.edit.first), int(ev.edit.last))
	case opMarkRange:
		err = session.MarkRange(int(ev.edit.first), int(ev.edit.last))
	case opKey:
		err = ev.edit.action.Apply(session)
	case opAction:
		application.editorAction(ev.edit.editorAction)
		return
	}
	if err != nil {
		application.fail(err)
		return
	}
	text.input.State = session.State
	win.pushEditor()
	if win.config.OnRedraw != nil {
		frame, err := win.config.OnRedraw()
		if err == nil {
			err = win.present(frame)
		}
		if err != nil {
			application.fail(err)
		}
	}
}

// editorAction is the action key of a keyboard (Done, Go, Search): it is Enter.
func (application *application) editorAction(int32) {
	application.keyEvent(event{keyCode: keycodeEnter})
}

// keyEvent gives the application a key of a keyboard. A key that is not text is a key
// event; a character, in a window with a text input, is typed into it; a chord with
// Control or Meta and a character is offered as a shortcut first.
func (application *application) keyEvent(ev event) {
	win := application.window
	if win == nil || win.closed || win.renderer == nil {
		return
	}
	defer application.guard()
	modifiers := modifiers(ev.modifiers)
	key := keyOfCode(ev.keyCode)
	character := ""
	if ev.unicode > 0 {
		character = string(rune(ev.unicode))
	}
	chord := modifiers&(input.ModifierControl|input.ModifierMeta) != 0 && character != ""
	if chord && win.config.OnShortcut != nil {
		frame, handled, err := win.config.OnShortcut(platform.KeyEvent{Key: key, Modifiers: modifiers, Repeat: ev.repeat, Text: character})
		if err == nil {
			err = win.present(frame)
		}
		if err != nil {
			application.fail(err)
			return
		}
		if handled {
			return
		}
	}
	if win.text != nil && !chord {
		if action, ok := imebridge.KeyEdit(key, modifiers, character, win.text.input); ok {
			application.textEdit(event{edit: edit{operation: opKey, action: action}})
			return
		}
	}
	if key == gfx.KeyUnknown && !chord {
		return
	}
	if win.config.OnKey == nil {
		return
	}
	frame, err := win.config.OnKey(platform.KeyEvent{Key: key, Modifiers: modifiers, Repeat: ev.repeat, Text: character})
	if err == nil {
		err = win.present(frame)
	}
	if err != nil {
		application.fail(err)
	}
}
