package ui

import "github.com/vivaui/viva/gfx"

// Overlay reports whether the last paint declared an open Popover, Modal, Menu,
// Select or Combobox list, for the tests of the overlays.
func (p *Page) Overlay() bool { return p.overlay }

// Frame and View are test-only aliases: the engine types are unexported, but the
// tests that exercise layout and a View surface still name them. They are not
// part of the public API.
type Frame = frame

type View = view

// Drag, Pointer and Key are test-only: the engine's View surface is unexported,
// and these expose its host routing to the tests that cover it.
func Drag(c C, state *DragState) { drag(c, state) }

func (s *DragState) Pointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	return s.pointerEvent(event)
}

func (s *DragState) Key(event gfx.KeyEvent) (gfx.EventResult, bool) {
	return s.keyEvent(event)
}
