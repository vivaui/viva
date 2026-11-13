//go:build windows

package windows

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	// SHGetFileInfoW flags: ask for the 32 px icon, and treat the path as a
	// file name whose extension decides the type even when it does not exist.
	shgfiIcon              = 0x00000100
	shgfiUseFileAttributes = 0x00000010
	fileAttributeNormal    = 0x00000080

	dibRGBColors = 0
	biRGB        = 0
)

// shFileInfo mirrors SHFILEINFOW.
type shFileInfo struct {
	Icon        uintptr
	IconIndex   int32
	Attributes  uint32
	DisplayName [260]uint16
	TypeName    [80]uint16
}

// gdiBitmap mirrors BITMAP from wingdi.h.
type gdiBitmap struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       uintptr
}

// monoBitmapInfo carries the two-entry palette a one-bit DIB needs.
type monoBitmapInfo struct {
	Header bitmapInfoHeader
	Colors [2]uint32
}

var shGetFileInfoW = shell.NewProc("SHGetFileInfoW")
var getFileIconInfo = user.NewProc("GetIconInfo")
var getObjectW = graphics.NewProc("GetObjectW")
var getDIBits = graphics.NewProc("GetDIBits")

// FileIconBGRA draws the icon Windows associates with path's extension into a
// pixels x pixels buffer of premultiplied BGRA, top row first. The path need not
// exist: with SHGFI_USEFILEATTRIBUTES the shell answers from the extension, so a
// remote file can be shown with the icon of its type. It may be called from any
// goroutine.
func FileIconBGRA(path string, pixels int) ([]byte, error) {
	if pixels < 1 || pixels > 512 {
		return nil, fmt.Errorf("windows: file icon size %d is outside 1..512", pixels)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("windows: file icon path: %w", err)
	}
	var file shFileInfo
	found, _, callErr := shGetFileInfoW.Call(
		uintptr(unsafe.Pointer(name)), fileAttributeNormal,
		uintptr(unsafe.Pointer(&file)), unsafe.Sizeof(file),
		shgfiIcon|shgfiUseFileAttributes,
	)
	if found == 0 || file.Icon == 0 {
		return nil, windowsCallError("read the file icon", callErr)
	}
	defer destroyIcon.Call(file.Icon)

	var info iconInfo
	if ok, _, callErr := getFileIconInfo.Call(file.Icon, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return nil, windowsCallError("read the file icon mask", callErr)
	}
	if info.Mask != 0 {
		defer deleteObject.Call(info.Mask)
	}
	if info.Color == 0 {
		return nil, errors.New("windows: file icon has no color bitmap")
	}
	defer deleteObject.Call(info.Color)

	var bitmap gdiBitmap
	if got, _, callErr := getObjectW.Call(info.Color, unsafe.Sizeof(bitmap), uintptr(unsafe.Pointer(&bitmap))); got == 0 || bitmap.Width <= 0 || bitmap.Height <= 0 {
		return nil, windowsCallError("measure the file icon", callErr)
	}
	width, height := int(bitmap.Width), int(bitmap.Height)

	screen, _, _ := getDC.Call(0)
	if screen == 0 {
		return nil, errors.New("windows: no screen device context for the file icon")
	}
	defer releaseDC.Call(0, screen)

	header := bitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(width), Height: -int32(height),
		Planes: 1, Bits: 32, Compression: biRGB,
	}
	colorInfo := bitmapInfo{Header: header}
	color := make([]byte, width*height*4)
	if got, _, callErr := getDIBits.Call(screen, info.Color, 0, uintptr(height),
		uintptr(unsafe.Pointer(&color[0])), uintptr(unsafe.Pointer(&colorInfo)), dibRGBColors); got == 0 {
		return nil, windowsCallError("read the file icon pixels", callErr)
	}

	hasAlpha := false
	for index := 3; index < len(color); index += 4 {
		if color[index] != 0 {
			hasAlpha = true
			break
		}
	}
	var mask []byte
	maskStride := (width + 31) / 32 * 4
	if !hasAlpha && info.Mask != 0 {
		maskHeader := bitmapInfoHeader{
			Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(width), Height: -int32(height),
			Planes: 1, Bits: 1, Compression: biRGB,
		}
		maskInfo := monoBitmapInfo{Header: maskHeader}
		mask = make([]byte, maskStride*height)
		if got, _, callErr := getDIBits.Call(screen, info.Mask, 0, uintptr(height),
			uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&maskInfo)), dibRGBColors); got == 0 {
			return nil, windowsCallError("read the file icon mask", callErr)
		}
	}

	result := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := (y*width + x) * 4
			blue, green, red := color[index], color[index+1], color[index+2]
			alpha := byte(255)
			switch {
			case hasAlpha:
				alpha = color[index+3]
				blue = byte(uint16(blue) * uint16(alpha) / 255)
				green = byte(uint16(green) * uint16(alpha) / 255)
				red = byte(uint16(red) * uint16(alpha) / 255)
			case mask != nil && mask[y*maskStride+x/8]&(0x80>>uint(x%8)) != 0:
				blue, green, red, alpha = 0, 0, 0, 0
			}
			result[index], result[index+1], result[index+2], result[index+3] = blue, green, red, alpha
		}
	}
	if width == pixels && height == pixels {
		return result, nil
	}
	scaled := make([]byte, pixels*pixels*4)
	for y := 0; y < pixels; y++ {
		sourceY := y * height / pixels
		for x := 0; x < pixels; x++ {
			sourceX := x * width / pixels
			copy(scaled[(y*pixels+x)*4:], result[(sourceY*width+sourceX)*4:(sourceY*width+sourceX)*4+4])
		}
	}
	return scaled, nil
}
