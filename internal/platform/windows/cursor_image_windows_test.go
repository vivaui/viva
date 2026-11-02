package windows

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/ui"
)

var (
	drawIconEx  = user.NewProc("DrawIconEx")
	getIconInfo = user.NewProc("GetIconInfo")
)

// The cursor a picture becomes must show the picture's colors the way a native cursor with alpha
// does, and put its hotspot where the picture says; both are read back from Windows itself.
func TestCursorHandleDrawsTheHalfTransparentPixelsAsAlphaBlended(t *testing.T) {
	// A mid gray at half alpha over white is the middle of both: 100 and 255 mixed half and half.
	image, err := ui.NewCursorImage(2, 2, 1, 0, 1, []byte{
		100, 100, 100, 128, 100, 100, 100, 128,
		100, 100, 100, 128, 100, 100, 100, 128,
	})
	if err != nil {
		t.Fatal(err)
	}
	handle, err := newCursorHandle(image, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer destroyIcon.Call(handle)

	var info iconInfo
	if ok, _, _ := getIconInfo.Call(handle, uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatal("GetIconInfo failed")
	}
	defer deleteObject.Call(info.Mask)
	defer deleteObject.Call(info.Color)
	if info.Icon != 0 || info.XHotspot != 1 || info.YHotspot != 0 {
		t.Fatalf("icon=%d hotspot=%d,%d, want a cursor with hotspot 1,0", info.Icon, info.XHotspot, info.YHotspot)
	}

	header := bitmapV4Header{
		Size: uint32(unsafe.Sizeof(bitmapV4Header{})), Width: 2, Height: -2,
		Planes: 1, Bits: 32, Compression: 3,
		RedMask: 0x00ff0000, GreenMask: 0x0000ff00, BlueMask: 0x000000ff, AlphaMask: 0xff000000,
	}
	screen, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, screen)
	var bits unsafe.Pointer
	target, _, _ := createDIBSection.Call(screen, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if target == 0 || bits == nil {
		t.Skip("no bitmap to draw into")
	}
	defer deleteObject.Call(target)
	dc, _, _ := createCompatibleDC.Call(screen)
	defer deleteDC.Call(dc)
	previous, _, _ := selectObject.Call(dc, target)
	defer selectObject.Call(dc, previous)
	pixels := unsafe.Slice((*byte)(bits), 16)
	for index := range pixels {
		pixels[index] = 255
	}
	if drawn, _, _ := drawIconEx.Call(dc, 0, 0, handle, 2, 2, 0, 0, 3); drawn == 0 { // DI_NORMAL
		t.Fatal("DrawIconEx failed")
	}
	// 100 at half alpha over 255 is 177 or 178 whichever way the half rounds.
	for channel := 0; channel < 3; channel++ {
		if got := pixels[channel]; got < 175 || got > 180 {
			t.Fatalf("channel %d over white = %d, want about 177: the picture was not read as alpha", channel, got)
		}
	}
}
