package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// ButtonSize names the heights of a Button that Box.XSmall, Small and Large give it.
type ButtonSize uint8

const (
	ButtonSizeDefault ButtonSize = iota
	ButtonSizeXSmall
	ButtonSizeSmall
	ButtonSizeLarge
)

// ButtonExtent is the width and height a Button showing label takes at size in the page's
// theme: what the element Button(c, label) is laid out at, for a page that has to reserve
// the room of a button it paints itself or places beside something else.
//
//	room := ui.ButtonExtent(c, "Configure", ui.ButtonSizeXSmall)
//	ui.Slot(c).Size(room.Width, room.Height)
//
// It measures with the page's canvas and declares nothing.
func ButtonExtent(c C, label string, size ButtonSize) Size {
	button := components.BeginButton(c.Canvas(), *c.Theme(), gfx.ID{}, Rect{})
	switch size {
	case ButtonSizeXSmall:
		button = button.XSmall()
	case ButtonSizeSmall:
		button = button.Small()
	case ButtonSizeLarge:
		button = button.Large()
	}
	button = button.SizeToFit(label)
	return Size{Width: button.BoundsWidth(), Height: button.BoundsHeight()}
}
