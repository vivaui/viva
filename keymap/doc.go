// Package keymap names keyboard shortcuts and keeps a user-editable table of
// them.
//
// A Chord is a set of modifiers and one key: a printable ASCII character, or
// one of the named keys that type nothing (Tab, the arrows, the function keys).
// Chord.FromEvent turns the ui.KeyEvent a window delivers into one, and a chord
// writes itself three ways: Label for a sentence ("Cmd+Shift+P"), Keycaps for
// the symbols a menu paints (⌘ ⇧ P on macOS, Ctrl Shift P elsewhere) and Name
// for a settings file ("page_up"), which Parse reads back.
//
// A Table holds one chord per action, for an application whose actions are a
// small dense enum. Bind refuses a chord another action holds and says which,
// which is what a rebinding dialog reports; Lookup finds the action a pressed
// chord runs. The package knows nothing of what an action does, which chords
// are free for a terminal or a text field, or how a table is saved: those stay
// with the application.
//
// The package is not linked into an application that does not import it.
package keymap
