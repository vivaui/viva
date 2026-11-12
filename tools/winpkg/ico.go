// Package winpkg builds the pieces of a Windows installer: the .ico file, the
// NSIS script and the makensis run. It uses only the standard library.
package winpkg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/internal/iconshape"
	"github.com/vivaui/viva/tools/appbundle"
)

// iconSizes are the images an .ico holds, as Explorer, the taskbar and the
// installer ask for them. Windows draws a 16, 24, 32 or 48 pixel icon at 100%, and
// 1.25, 1.5 and 1.75 times that at those display scales; with an entry of each size the
// shell shows one made for the pixels it covers instead of scaling the nearest.
var iconSizes = []int{16, 20, 24, 30, 32, 36, 40, 48, 60, 64, 72, 96, 128, 256}

// MinIconPixels is the smallest source accepted, the same as for a macOS icon.
// FullIconPixels is the source size from which every entry, up to Windows's
// largest 256, is downscaled rather than missing.
const (
	MinIconPixels  = appbundle.MinIconPixels
	FullIconPixels = 256
)

// smallIconPixels is the largest entry Windows draws the application icon small:
// the taskbar button, the Start menu, and Explorer's medium icons. Those entries
// crop the canvas margin away and carry the mark (iconshape.TileMark), as a
// status icon does, while larger entries keep the tile the application drew.
const smallIconPixels = 48

// EncodeICO builds an .ico file from a square application picture. Every entry is
// a PNG, which Windows Vista and later read. Sizes larger than the source are
// left out instead of being upscaled, and the largest size used is returned.
func EncodeICO(source image.Image) (data []byte, largest int, err error) {
	bounds := source.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return nil, 0, fmt.Errorf("winpkg: the icon must be square, not %dx%d", bounds.Dx(), bounds.Dy())
	}
	if bounds.Dx() < MinIconPixels {
		return nil, 0, fmt.Errorf("winpkg: the icon is %d px; at least %d px is required (%d or more recommended)", bounds.Dx(), MinIconPixels, FullIconPixels)
	}
	tile := iconshape.Tile(source, bounds.Dx(), 1)
	var images [][]byte
	var sizes []int
	for _, size := range iconSizes {
		if size > bounds.Dx() {
			continue
		}
		frame := appbundle.Downscale(tile, size)
		if size <= smallIconPixels {
			frame = iconshape.TileMark(source, size)
		}
		var buffer bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buffer, frame); err != nil {
			return nil, 0, err
		}
		images = append(images, buffer.Bytes())
		sizes = append(sizes, size)
		largest = size
	}

	var file bytes.Buffer
	binary.Write(&file, binary.LittleEndian, [3]uint16{0, 1, uint16(len(images))})
	offset := 6 + 16*len(images)
	for index, encoded := range images {
		dimension := byte(sizes[index])
		if sizes[index] >= 256 {
			dimension = 0 // 0 means 256 in an icon directory
		}
		file.Write([]byte{dimension, dimension, 0, 0})
		binary.Write(&file, binary.LittleEndian, [2]uint16{1, 32}) // planes, bits per pixel
		binary.Write(&file, binary.LittleEndian, [2]uint32{uint32(len(encoded)), uint32(offset)})
		offset += len(encoded)
	}
	for _, encoded := range images {
		file.Write(encoded)
	}
	return file.Bytes(), largest, nil
}

// LoadIcon reads an icon file as .ico data. A .png is converted; an .ico is
// used as it is after checking its header, and then pixels is 0 because the
// sizes inside are the file's own business. pixels is otherwise the largest
// size inside.
func LoadIcon(path string) (data []byte, pixels int, err error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("winpkg: reading the icon: %w", err)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ico":
		if err := checkICO(source); err != nil {
			return nil, 0, fmt.Errorf("winpkg: %s: %w", path, err)
		}
		return source, 0, nil
	case ".png":
		decoded, err := png.Decode(bytes.NewReader(source))
		if err != nil {
			return nil, 0, fmt.Errorf("winpkg: %s: %w", path, err)
		}
		data, pixels, err = EncodeICO(decoded)
		if err != nil {
			return nil, 0, fmt.Errorf("%w (%s)", err, path)
		}
		return data, pixels, nil
	}
	return nil, 0, fmt.Errorf("winpkg: the icon %s must be a .png or .ico file for a Windows package", path)
}

// checkICO verifies the directory of an .ico: the header, and that every entry
// lies inside the file.
func checkICO(data []byte) error {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:]) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return errors.New("is not an .ico file")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if count == 0 || len(data) < 6+16*count {
		return errors.New("is not an .ico file: the directory is truncated")
	}
	for index := 0; index < count; index++ {
		entry := data[6+16*index:]
		size, offset := int(binary.LittleEndian.Uint32(entry[8:])), int(binary.LittleEndian.Uint32(entry[12:]))
		if offset < 6+16*count || size <= 0 || offset+size > len(data) {
			return fmt.Errorf("is not a valid .ico file: image %d lies outside the file", index)
		}
	}
	return nil
}
