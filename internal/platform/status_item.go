package platform

import "math"

// PanelGap is how far a panel stays from the status item it opens beside, and
// PanelMargin how far from the edge of the usable screen area, in pixels.
const (
	PanelGap    = 4
	PanelMargin = 8
)

// PlacePanel returns the top-left corner of a width x height panel opened beside
// anchor, in a desktop whose y axis points down. The panel is centered on the
// anchor along the bar's axis and sits on the side of it that faces the middle
// of the screen: above a taskbar at the bottom, below a menu bar at the top. It
// is then kept inside area, the part of the screen windows may use, with a
// margin at the sides and at the edge away from the bar; next to the bar the gap
// to the anchor decides. An anchor with no extent (a bare click position) is
// placed the same way.
func PlacePanel(anchor ScreenRect, width, height float32, area ScreenRect) (x, y float32) {
	centerX := anchor.X + anchor.Width/2
	centerY := anchor.Y + anchor.Height/2
	x = centerX - width/2
	if centerY > area.Y+area.Height/2 {
		y = anchor.Y - PanelGap - height
	} else {
		y = anchor.Y + anchor.Height + PanelGap
	}
	x = min(x, area.X+area.Width-width-PanelMargin)
	x = max(x, area.X+PanelMargin)
	y = min(y, area.Y+area.Height-height-PanelMargin)
	y = max(y, area.Y)
	return x, y
}

// PlaceNotification returns the bottom-right origin inside a screen's working
// area. All coordinates and the margin use the same units.
func PlaceNotification(width, height, margin float32, area ScreenRect) (float32, float32) {
	return max(area.X, area.X+area.Width-width-margin), max(area.Y, area.Y+area.Height-height-margin)
}

// RestoreNotification fits a remembered top-left into the selected work area.
func RestoreNotification(x, y, width, height float32, area ScreenRect) (float32, float32) {
	return min(max(x, area.X), area.X+max(0, area.Width-width)),
		min(max(y, area.Y), area.Y+max(0, area.Height-height))
}

// Scaled returns the icon at size x size pixels, each result pixel the average
// of the source pixels it covers, weighted by their alpha so that a transparent
// edge does not bleed its hidden color into the shape. Platforms that draw a
// status icon at one small fixed size ask for it at that size rather than leave
// the scaling to a shell that does it without averaging. An invalid icon, or a
// size below one, gives the zero icon.
func (icon StatusIcon) Scaled(size int) StatusIcon {
	if size < 1 || !icon.Valid() {
		return StatusIcon{}
	}
	out := StatusIcon{Size: size, RGBA: make([]byte, size*size*4), Template: icon.Template}
	scale := float64(icon.Size) / float64(size)
	for row := 0; row < size; row++ {
		top := int(math.Floor(float64(row) * scale))
		bottom := max(top+1, int(math.Ceil(float64(row+1)*scale)))
		for column := 0; column < size; column++ {
			left := int(math.Floor(float64(column) * scale))
			right := max(left+1, int(math.Ceil(float64(column+1)*scale)))
			var red, green, blue, alpha, count uint32
			for sourceRow := top; sourceRow < min(bottom, icon.Size); sourceRow++ {
				for sourceColumn := left; sourceColumn < min(right, icon.Size); sourceColumn++ {
					pixel := icon.RGBA[(sourceRow*icon.Size+sourceColumn)*4:]
					weight := uint32(pixel[3])
					red += uint32(pixel[0]) * weight
					green += uint32(pixel[1]) * weight
					blue += uint32(pixel[2]) * weight
					alpha += weight
					count++
				}
			}
			if count == 0 || alpha == 0 {
				continue
			}
			destination := out.RGBA[(row*size+column)*4:]
			destination[0] = uint8((red + alpha/2) / alpha)
			destination[1] = uint8((green + alpha/2) / alpha)
			destination[2] = uint8((blue + alpha/2) / alpha)
			destination[3] = uint8((alpha + count/2) / count)
		}
	}
	return out
}
