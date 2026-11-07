package keymap

import (
	"strings"

	"github.com/vivaui/viva/ui"
)

// Chord is a modifier set and one key: a printable character in Char, or a
// named key in Key. A valid chord sets exactly one of the two. Letters are
// stored in lower case, so Shift+P is Modifiers Shift with Char 'p'. The zero
// Chord is no chord at all.
type Chord struct {
	Modifiers ui.Modifiers
	Char      byte
	Key       ui.Key
}

const modifierMask = ui.ModifierShift | ui.ModifierControl | ui.ModifierAlt | ui.ModifierMeta

// namedKeys are the keys without text a chord may use: the label a sentence
// shows, the keycap a menu shows, and the name a settings file keeps. Space is
// not here; it is the character ' '.
var namedKeys = [...]struct {
	key    ui.Key
	label  string
	keycap string
	saved  string
}{
	{ui.KeyTab, "Tab", "Tab", "tab"},
	{ui.KeyPageUp, "PageUp", "PgUp", "page_up"},
	{ui.KeyPageDown, "PageDown", "PgDn", "page_down"},
	{ui.KeyLeft, "Left", "←", "left"},
	{ui.KeyRight, "Right", "→", "right"},
	{ui.KeyEnter, "Enter", "↵", "enter"},
	{ui.KeyEscape, "Esc", "Esc", "escape"},
	{ui.KeyUp, "Up", "↑", "up"},
	{ui.KeyDown, "Down", "↓", "down"},
	{ui.KeyHome, "Home", "Home", "home"},
	{ui.KeyEnd, "End", "End", "end"},
	{ui.KeyBackspace, "Backspace", "⌫", "backspace"},
	{ui.KeyDelete, "Delete", "Del", "delete"},
	{ui.KeyInsert, "Insert", "Ins", "insert"},
	{ui.KeyF1, "F1", "F1", "f1"},
	{ui.KeyF2, "F2", "F2", "f2"},
	{ui.KeyF3, "F3", "F3", "f3"},
	{ui.KeyF4, "F4", "F4", "f4"},
	{ui.KeyF5, "F5", "F5", "f5"},
	{ui.KeyF6, "F6", "F6", "f6"},
	{ui.KeyF7, "F7", "F7", "f7"},
	{ui.KeyF8, "F8", "F8", "f8"},
	{ui.KeyF9, "F9", "F9", "f9"},
	{ui.KeyF10, "F10", "F10", "f10"},
	{ui.KeyF11, "F11", "F11", "f11"},
	{ui.KeyF12, "F12", "F12", "f12"},
}

func namedKey(key ui.Key) (int, bool) {
	for index := range namedKeys {
		if namedKeys[index].key == key {
			return index, true
		}
	}
	return 0, false
}

// Valid reports whether the chord can be bound: only the four modifiers, and
// either a named key or a printable ASCII character that is not an upper-case
// letter.
func (chord Chord) Valid() bool {
	if chord.Modifiers&^modifierMask != 0 {
		return false
	}
	if chord.Key != ui.KeyUnknown {
		_, named := namedKey(chord.Key)
		return named && chord.Char == 0
	}
	return chord.Char >= ' ' && chord.Char <= '~' && !(chord.Char >= 'A' && chord.Char <= 'Z')
}

// FromEvent is the chord a key event presses: a named key, Space, or the one
// character of event.Text with the modifiers in effect. ok is false for an
// event that is not a bindable chord: a modifier on its own, text of more than
// one byte, a key a chord cannot name.
func FromEvent(event ui.KeyEvent) (chord Chord, ok bool) {
	if event.Key != ui.KeyUnknown {
		if _, named := namedKey(event.Key); named {
			chord = Chord{Modifiers: event.Modifiers, Key: event.Key}
			return chord, chord.Valid()
		}
		if event.Key == ui.KeySpace {
			return Chord{Modifiers: event.Modifiers, Char: ' '}, true
		}
	}
	if len(event.Text) != 1 {
		return Chord{}, false
	}
	char := event.Text[0]
	if char >= 'A' && char <= 'Z' {
		char += 'a' - 'A'
	}
	chord = Chord{Modifiers: event.Modifiers, Char: char}
	return chord, chord.Valid()
}

// Label is the chord as a sentence, modifiers first: "Cmd+Shift+P",
// "Ctrl+Tab", "Alt+Space". The names are the same on every platform; a menu
// that wants the platform's symbols uses Keycaps. An invalid chord has no label.
func (chord Chord) Label() string {
	if !chord.Valid() {
		return ""
	}
	var label strings.Builder
	if chord.Modifiers&ui.ModifierMeta != 0 {
		label.WriteString("Cmd+")
	}
	if chord.Modifiers&ui.ModifierControl != 0 {
		label.WriteString("Ctrl+")
	}
	if chord.Modifiers&ui.ModifierAlt != 0 {
		label.WriteString("Alt+")
	}
	if chord.Modifiers&ui.ModifierShift != 0 {
		label.WriteString("Shift+")
	}
	if index, ok := namedKey(chord.Key); ok {
		label.WriteString(namedKeys[index].label)
		return label.String()
	}
	if chord.Char == ' ' {
		label.WriteString("Space")
		return label.String()
	}
	label.WriteByte(upper(chord.Char))
	return label.String()
}

// keycapText is the text of each character, made once: converting a byte to a
// string allocates, and keycaps are painted on every menu.
var keycapText = func() [256]string {
	var table [256]string
	for code := range table {
		table[code] = string(rune(code))
	}
	return table
}()

// Keycaps appends the chord split into the keycaps a menu shows, one per
// modifier and then the key, and returns the extended slice. Where the platform
// has symbols for the modifiers it uses them, and the order is the platform's:
// ⌃ ⌥ ⇧ ⌘ on macOS, Ctrl Alt Shift Win elsewhere. An invalid chord appends
// nothing. With a dst that has room the call does not allocate.
func (chord Chord) Keycaps(dst []string) []string {
	if !chord.Valid() {
		return dst
	}
	if chord.Modifiers&ui.ModifierControl != 0 {
		dst = append(dst, modifierKeycap(ui.ModifierControl))
	}
	if chord.Modifiers&ui.ModifierAlt != 0 {
		dst = append(dst, modifierKeycap(ui.ModifierAlt))
	}
	if chord.Modifiers&ui.ModifierShift != 0 {
		dst = append(dst, modifierKeycap(ui.ModifierShift))
	}
	if chord.Modifiers&ui.ModifierMeta != 0 {
		dst = append(dst, modifierKeycap(ui.ModifierMeta))
	}
	if index, ok := namedKey(chord.Key); ok {
		return append(dst, namedKeys[index].keycap)
	}
	if chord.Char == ' ' {
		return append(dst, "Space")
	}
	return append(dst, keycapText[upper(chord.Char)])
}

// Name is the chord's key as a settings file writes it, without the modifiers:
// the character, or the name of a named key ("page_up", "f5"). An invalid chord
// has no name. Parse reads it back.
func (chord Chord) Name() string {
	if !chord.Valid() {
		return ""
	}
	if index, ok := namedKey(chord.Key); ok {
		return namedKeys[index].saved
	}
	return string([]byte{chord.Char})
}

// Parse is the inverse of Name: the chord with the given modifiers and the key
// the name stands for. An upper-case letter is read as its lower-case form. ok
// is false for a name that is neither a named key nor one character, and for a
// result that is not a valid chord.
func Parse(modifiers ui.Modifiers, name string) (Chord, bool) {
	for index := range namedKeys {
		if namedKeys[index].saved == name {
			chord := Chord{Modifiers: modifiers, Key: namedKeys[index].key}
			return chord, chord.Valid()
		}
	}
	if len(name) != 1 {
		return Chord{}, false
	}
	char := name[0]
	if char >= 'A' && char <= 'Z' {
		char += 'a' - 'A'
	}
	chord := Chord{Modifiers: modifiers, Char: char}
	return chord, chord.Valid()
}

func upper(char byte) byte {
	if char >= 'a' && char <= 'z' {
		return char - ('a' - 'A')
	}
	return char
}
