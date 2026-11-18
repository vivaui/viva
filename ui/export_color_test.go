package ui

import "github.com/vivaui/viva/text"

// TypeHexForTest replaces the staged hex text, as typing in the panel's field does.
func (s *ColorPickerState) TypeHexForTest(Value string) {
	s.hexEditor = text.Editor{}
	s.hexEditor.Insert(Value)
}
