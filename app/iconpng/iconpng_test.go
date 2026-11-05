package iconpng

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestDecodeKeepsThePixelsAndRejectsWhatIsNotSquare(t *testing.T) {
	encode := func(width, height int) []byte {
		picture := image.NewNRGBA(image.Rect(0, 0, width, height))
		picture.SetNRGBA(1, 0, color.NRGBA{R: 9, G: 8, B: 7, A: 200})
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, picture); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	icon, err := Decode(encode(4, 4))
	if err != nil || icon.Size != 4 {
		t.Fatalf("a square PNG gave size %d, err %v", icon.Size, err)
	}
	if got := icon.RGBA[4:8]; got[0] != 9 || got[1] != 8 || got[2] != 7 || got[3] != 200 {
		t.Errorf("the pixel came back as %v", got)
	}
	if _, err := Decode(encode(4, 2)); err == nil {
		t.Error("a PNG that is not square was accepted")
	}
	if _, err := Decode([]byte("not a png")); err == nil {
		t.Error("garbage was accepted")
	}
}

// A master larger than the limit is shrunk so the application does not keep
// its full pixels for its whole life; a picture that fits is left alone.
func TestDecodeFitShrinksOnlyWhatExceedsTheLimit(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for index := 0; index < len(picture.Pix); index += 4 {
		picture.Pix[index], picture.Pix[index+1], picture.Pix[index+2], picture.Pix[index+3] = 10, 20, 30, 255
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	shrunk, err := DecodeFit(buffer.Bytes(), 16)
	if err != nil || shrunk.Size != 16 || len(shrunk.RGBA) != 16*16*4 {
		t.Fatalf("64 px master with limit 16 gave size %d, %d bytes, err %v", shrunk.Size, len(shrunk.RGBA), err)
	}
	if got := shrunk.RGBA[:4]; got[0] != 10 || got[1] != 20 || got[2] != 30 || got[3] != 255 {
		t.Errorf("a flat color changed when shrunk: %v", got)
	}
	whole, err := DecodeFit(buffer.Bytes(), 128)
	if err != nil || whole.Size != 64 {
		t.Fatalf("a picture within the limit gave size %d, err %v", whole.Size, err)
	}
	if _, err := DecodeFit(buffer.Bytes(), 0); err == nil {
		t.Error("a zero limit was accepted")
	}
}
