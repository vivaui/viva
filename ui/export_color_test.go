package ui

import "github.com/vivaui/viva/text"

// TypeHexForTest replaces the staged hex text, as typing in the panel's field does.
func (s *ColorPickerState) TypeHexForTest(value string) {
	s.hexEditor = text.Editor{}
	s.hexEditor.Insert(value)
}
