package ui

// Overlay reports whether the last paint declared an open Popover, Modal, Menu,
// Select or Combobox list, for the tests of the overlays.
func (p *Page) Overlay() bool { return p.overlay }
