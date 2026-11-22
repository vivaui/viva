//go:build windows

package platform

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// picturePNG encodes a picture whose channels and edges a round trip has to keep
// apart: a grid of colours that differ in every channel and in every direction.
func picturePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.SetNRGBA(x, y, color.NRGBA{R: uint8(30 + 50*x), G: uint8(10 + 60*y), B: 200, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestPNGToDIBRoundTripsThroughDIBToPNG converts a PNG to the bitmap CF_DIB
// carries and reads it back with the DIB reader, the two halves of one clipboard
// format: a picture that survives both is what a paste of the copy shows.
func TestPNGToDIBRoundTripsThroughDIBToPNG(t *testing.T) {
	encoded := picturePNG(t, 5, 3)
	dib, err := PNGToDIB(encoded)
	if err != nil {
		t.Fatal(err)
	}
	read, err := DIBToPNG(dib)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(read))
	if err != nil {
		t.Fatal(err)
	}
	if bounds := got.Bounds(); bounds.Dx() != 5 || bounds.Dy() != 3 {
		t.Fatalf("size = %dx%d, want 5x3 (rows upside down?)", bounds.Dx(), bounds.Dy())
	}
	want, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	for y := range 3 {
		for x := range 5 {
			expected := color.NRGBAModel.Convert(want.At(x, y)).(color.NRGBA)
			if actual := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA); actual != expected {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, actual, expected)
			}
		}
	}
}

// TestPNGToDIBCompositesTransparencyOnWhite pins the one thing the bitmap
// flavour cannot carry: a transparent pixel reaches a CF_DIB reader as the white
// its background is, not as black and not as a colour without alpha.
func TestPNGToDIBCompositesTransparencyOnWhite(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	picture.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 0})
	picture.SetNRGBA(1, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	dib, err := PNGToDIB(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	read, err := DIBToPNG(dib)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(read))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA); got != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("fully transparent pixel = %v, want white", got)
	}
	// (10,20,30) at half alpha over white: 10*128+255*127 rounded, and so on.
	if got := color.NRGBAModel.Convert(decoded.At(1, 0)).(color.NRGBA); got != (color.NRGBA{R: 132, G: 137, B: 142, A: 255}) {
		t.Fatalf("half-transparent pixel = %v, want the colour layered on white", got)
	}
}

// TestPNGToDIBRefusesWhatIsNotAPicture keeps the write path from putting bytes
// on the clipboard that no reader can open.
func TestPNGToDIBRefusesWhatIsNotAPicture(t *testing.T) {
	if _, err := PNGToDIB([]byte("GIF89a, not a PNG")); err == nil {
		t.Fatal("bytes that are not a PNG were converted")
	}
	truncated := picturePNG(t, 4, 4)
	if _, err := PNGToDIB(truncated[:len(truncated)/2]); err == nil {
		t.Fatal("a truncated PNG was converted")
	}
	if _, err := PNGToDIB(nil); err == nil {
		t.Fatal("an empty image was converted")
	}
}
