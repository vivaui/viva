package windows

import (
	"unicode/utf16"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// An input method asks the application about its text with WM_IME_REQUEST: where a
// character of the composition is, so that the candidate list stands beside it, and
// for the text around the selection to reconvert it, the key that makes the
// input method take converted text back to its reading.

const (
	imrReconvertString = 0x0004
	imrQueryCharacter  = 0x0006

	// reconvertLimit is how many UTF-16 units of the selection's line are handed to
	// an input method; it reconverts a phrase, not a page.
	reconvertLimit = 256
)

// reconvertString is RECONVERTSTRING: the header, then the string at StrOffset
// bytes from its start. Offsets into the string are bytes; lengths are characters.
type reconvertString struct {
	Size, Version                 uint32
	StrLen, StrOffset             uint32
	CompStrLen, CompStrOffset     uint32
	TargetStrLen, TargetStrOffset uint32
}

// characterPosition is IMECHARPOSITION.
type characterPosition struct {
	Size, CharPos uint32
	Point         point
	LineHeight    uint32
	Document      rect
}

// handleIMERequest answers an input method's request and reports whether it was
// one the window answers.
func (state *windowState) handleIMERequest(command, parameter uintptr) (uintptr, bool) {
	if !state.textEnabled || state.window == 0 {
		return 0, false
	}
	switch command {
	case imrReconvertString:
		if state.textInput.Terminal {
			return 0, false
		}
		return state.reconvert(parameter), true
	case imrQueryCharacter:
		return state.queryCharacterPosition(parameter), true
	}
	return 0, false
}

// reconvert hands the input method the selected text and its line (a RECONVERTSTRING)
// or, given no buffer, the size the buffer must have. It answers 0 when nothing is
// selected, which asks the input method to do without.
func (state *windowState) reconvert(parameter uintptr) uintptr {
	selection := state.textState.Selection
	if selection.Start > selection.End {
		selection.Start, selection.End = selection.End, selection.Start
	}
	units, start, length, ok := platform.ReconvertContext(state.textState.Value, selection, reconvertLimit)
	if !ok {
		return 0
	}
	header := unsafe.Sizeof(reconvertString{})
	size := header + uintptr(len(units)+1)*2
	if parameter == 0 {
		return size
	}
	request := (*reconvertString)(pointer(parameter))
	if uintptr(request.Size) < size {
		return 0
	}
	*request = reconvertString{
		Size: uint32(size), StrLen: uint32(len(units)), StrOffset: uint32(header),
		CompStrLen: uint32(length), CompStrOffset: uint32(start * 2),
		TargetStrLen: uint32(length), TargetStrOffset: uint32(start * 2),
	}
	destination := unsafe.Slice((*uint16)(pointer(parameter+header)), len(units)+1)
	copy(destination, units)
	destination[len(units)] = 0
	return size
}

// queryCharacterPosition tells the input method where a character of its
// composition is on the screen, so the candidate list can stand under it.
func (state *windowState) queryCharacterPosition(parameter uintptr) uintptr {
	request := (*characterPosition)(pointer(parameter))
	if parameter == 0 || uintptr(request.Size) < unsafe.Sizeof(characterPosition{}) {
		return 0
	}
	input := &state.textInput
	runes := []rune(state.textState.Value)
	anchor := state.textState.Selection.End
	composition := state.textState.Composition
	index := anchor
	if state.textState.Composing && validTextRange(composition, len(runes)) {
		index = composition.Start
		for units := 0; index < composition.End && units < int(request.CharPos); index++ {
			units += utf16.RuneLen(runes[index])
		}
	}
	x, y, height := state.caretCell()
	if !input.Placed {
		x = state.alignedCaretX(index)
	}
	scale := state.scale
	origin := point{X: int32(x * scale), Y: int32(y * scale)}
	clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&origin)))
	document := point{X: int32(input.X * scale), Y: int32(input.Y * scale)}
	clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&document)))
	request.Point = origin
	request.LineHeight = uint32(height * scale)
	request.Document = rect{Left: document.X, Top: document.Y, Right: document.X + int32(input.Width*scale), Bottom: document.Y + int32(input.Height*scale)}
	return 1
}
