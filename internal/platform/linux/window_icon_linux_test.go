package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestNetWMIconOffersEverySizeTheSourceCanFill(t *testing.T) {
	const side = 64
	pixels := make([]byte, side*side*4)
	for index := 0; index < len(pixels); index += 4 {
		copy(pixels[index:], []byte{0x10, 0x20, 0x30, 0x80})
	}
	data := netWMIcon(platform.WindowIcon{Size: side, RGBA: pixels})

	var sizes []uint32
	for offset := 0; offset < len(data); {
		width, height := data[offset], data[offset+1]
		if width != height {
			t.Fatalf("an image of %dx%d at element %d is not square", width, height, offset)
		}
		middle := data[offset+2+int(width*width)/2+int(width)/2]
		if middle != 0x80102030 {
			t.Fatalf("a pixel of 0x80/0x10/0x20/0x30 is packed as %#x, want 0x80102030 (alpha, red, green, blue)", middle)
		}
		if corner := data[offset+2]; corner>>24 != 0 {
			t.Fatalf("the corner of the %d px image is opaque (%#x): the square was not rounded", width, corner)
		}
		sizes = append(sizes, width)
		offset += 2 + int(width*height)
	}
	if len(sizes) != 4 || sizes[0] != 16 || sizes[3] != 64 {
		t.Fatalf("a 64 px source is offered at %v, want 16, 32, 48 and 64", sizes)
	}
}

func TestNetWMIconCarriesTheMarkInTheImagesAPanelDraws(t *testing.T) {
	// A dark mark a quarter of the side on white. The 16 px image a panel asks
	// for must carry the mark, not the canvas margin around it.
	const side = 64
	pixels := make([]byte, side*side*4)
	for index := 0; index < len(pixels); index += 4 {
		pixels[index], pixels[index+1], pixels[index+2], pixels[index+3] = 255, 255, 255, 255
	}
	for y := 24; y < 40; y++ {
		for x := 24; x < 40; x++ {
			copy(pixels[(y*side+x)*4:], []byte{16, 16, 16, 255})
		}
	}
	data := netWMIcon(platform.WindowIcon{Size: side, RGBA: pixels})
	for offset := 0; offset < len(data); {
		width := int(data[offset])
		if width == 16 {
			dark := 0
			for index := 0; index < width*width; index++ {
				pixel := data[offset+2+index]
				if pixel>>24 > 128 && (pixel>>16)&0xff < 64 {
					dark++
				}
			}
			if share := float64(dark) / float64(width*width); share < 0.6 {
				t.Errorf("the 16 px image is %.0f%% mark: the panel would show the canvas margin", share*100)
			}
			return
		}
		offset += 2 + width*width
	}
	t.Fatal("no 16 px image")
}

func TestNetWMIconOfATinySourceIsOfferedAsItIs(t *testing.T) {
	data := netWMIcon(platform.WindowIcon{Size: 8, RGBA: make([]byte, 8*8*4)})
	if len(data) != 2+64 || data[0] != 8 {
		t.Fatalf("an 8 px source gives %d elements starting %d, want one 8x8 image", len(data), data[0])
	}
	if netWMIcon(platform.WindowIcon{}) != nil {
		t.Fatal("a window without an icon must not set the property")
	}
}
