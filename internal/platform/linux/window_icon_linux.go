package linux

import "github.com/vivaui/viva/internal/platform"

// netWMIconSizes are the sizes offered in _NET_WM_ICON: a panel, a title bar and
// Alt+Tab each take the one nearest what they draw. A size above the source's own
// is left out, as it would only repeat pixels.
var netWMIconSizes = [...]int{16, 32, 48, 64, 128, 256}

// netWMIcon packs the icon the way _NET_WM_ICON holds it: for each size a width
// and a height, then width*height pixels as 0xAARRGGBB, rows from the top.
func netWMIcon(icon platform.WindowIcon) []uint32 {
	if !icon.Valid() {
		return nil
	}
	var data []uint32
	for _, size := range netWMIconSizes {
		if size > icon.Size {
			break
		}
		// A panel and a task switcher draw these small, so the mark fills the
		// icon rather than the margin the canvas carries for a large one.
		data = appendNetWMIcon(data, icon.TileMark(size))
	}
	if len(data) == 0 {
		// A source smaller than the smallest size is offered as it is.
		data = appendNetWMIcon(data, icon.TileMark(icon.Size))
	}
	return data
}

func appendNetWMIcon(data []uint32, icon platform.WindowIcon) []uint32 {
	data = append(data, uint32(icon.Size), uint32(icon.Size))
	for index := 0; index < icon.Size*icon.Size*4; index += 4 {
		pixel := icon.RGBA[index : index+4]
		data = append(data, uint32(pixel[3])<<24|uint32(pixel[0])<<16|uint32(pixel[1])<<8|uint32(pixel[2]))
	}
	return data
}
