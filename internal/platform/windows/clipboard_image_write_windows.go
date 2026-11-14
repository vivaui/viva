package windows

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"image/png"

	"github.com/vivaui/viva/internal/platform"
)

// SetClipboardImagePNG publishes PNG and CF_DIBV5 so both image editors and
// native applications can paste it. The clipboard owns each successful block.
func SetClipboardImagePNG(data []byte) error {
	dib, err := clipboardPNGToDIBV5(data)
	if err != nil {
		return err
	}
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	format := registeredPNGFormat()
	if format == 0 {
		return fmt.Errorf("windows: register PNG clipboard format failed")
	}
	owner := uintptr(0)
	if state := active.Load(); state != nil {
		owner = state.window
	}
	if opened, _, callErr := openClipboard.Call(owner); opened == 0 {
		return windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	if emptied, _, callErr := emptyClipboard.Call(); emptied == 0 {
		return windowsCallError("empty clipboard", callErr)
	}
	if err := setClipboardBlock(format, data); err != nil {
		return err
	}
	// A usable PNG remains if the optional native representation cannot be installed.
	_ = setClipboardBlock(clipboardDIBV5, dib)
	return nil
}

func clipboardPNGToDIBV5(data []byte) ([]byte, error) {
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	const bytesPerPixel = 4
	const headerSize = 124
	if config.Width <= 0 || config.Height <= 0 || config.Width > (platform.MaxClipboardRawBytes-headerSize)/bytesPerPixel/config.Height {
		return nil, platform.ErrClipboardImageTooLarge
	}
	picture, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	pixels := config.Width * config.Height * bytesPerPixel
	result := make([]byte, headerSize+pixels)
	binary.LittleEndian.PutUint32(result[0:], headerSize)
	binary.LittleEndian.PutUint32(result[4:], uint32(config.Width))
	binary.LittleEndian.PutUint32(result[8:], uint32(-int32(config.Height))) // top-down rows
	binary.LittleEndian.PutUint16(result[12:], 1)
	binary.LittleEndian.PutUint16(result[14:], 32)
	binary.LittleEndian.PutUint32(result[16:], 3) // BI_BITFIELDS
	binary.LittleEndian.PutUint32(result[20:], uint32(pixels))
	binary.LittleEndian.PutUint32(result[40:], 0x00ff0000)
	binary.LittleEndian.PutUint32(result[44:], 0x0000ff00)
	binary.LittleEndian.PutUint32(result[48:], 0x000000ff)
	binary.LittleEndian.PutUint32(result[52:], 0xff000000)
	binary.LittleEndian.PutUint32(result[56:], 0x73524742) // LCS_sRGB
	for y := 0; y < config.Height; y++ {
		for x := 0; x < config.Width; x++ {
			pixel := color.NRGBAModel.Convert(picture.At(x, y)).(color.NRGBA)
			offset := headerSize + (y*config.Width+x)*bytesPerPixel
			copy(result[offset:offset+bytesPerPixel], []byte{pixel.B, pixel.G, pixel.R, pixel.A})
		}
	}
	return result, nil
}
