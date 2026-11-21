package platform

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"testing"
)

func TestCheckClipboardPNG(t *testing.T) {
	var small bytes.Buffer
	if err := png.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := CheckClipboardPNG(small.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := CheckClipboardPNG(nil); !errors.Is(err, ErrNoClipboardImage) {
		t.Fatalf("empty = %v, want ErrNoClipboardImage", err)
	}
	big := append(append([]byte{}, pngSignature...), make([]byte, MaxClipboardImageBytes)...)
	if err := CheckClipboardPNG(big); !errors.Is(err, ErrClipboardImageTooLarge) {
		t.Fatalf("oversize = %v, want ErrClipboardImageTooLarge", err)
	}
	if err := CheckClipboardPNG([]byte("GIF89a...")); err == nil {
		t.Fatal("a GIF was accepted as PNG")
	}
}

// A PNG small enough to hand over but naming a picture too large to hold is
// refused before a platform decodes it: the encoded size says nothing about the
// memory the pixels need.
func TestCheckClipboardPNGRejectsTooManyPixels(t *testing.T) {
	if err := CheckClipboardPNG(pngHeader(1000, 1000)); err != nil {
		t.Fatalf("a 1000x1000 PNG was refused: %v", err)
	}
	if err := CheckClipboardPNG(pngHeader(20000, 20000)); !errors.Is(err, ErrClipboardImageTooLarge) {
		t.Fatalf("a 20000x20000 PNG = %v, want ErrClipboardImageTooLarge", err)
	}
}

// pngHeader builds a PNG that stops after its IHDR chunk, which is everything
// DecodeConfig reads: width and height are what the test chooses.
func pngHeader(width, height uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // colour type: RGBA
	var out bytes.Buffer
	out.Write(pngSignature)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(ihdr)))
	out.Write(length[:])
	out.WriteString("IHDR")
	out.Write(ihdr)
	crc := crc32.NewIEEE()
	crc.Write([]byte("IHDR"))
	crc.Write(ihdr)
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())
	out.Write(sum[:])
	return out.Bytes()
}

func BenchmarkCheckClipboardPNG(b *testing.B) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		b.Fatal(err)
	}
	data := encoded.Bytes()
	b.ReportAllocs()
	for b.Loop() {
		if err := CheckClipboardPNG(data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestTrimClipboardPNGDropsBlockPadding(t *testing.T) {
	var small bytes.Buffer
	if err := png.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	padded := append(append([]byte{}, small.Bytes()...), make([]byte, 100)...)
	trimmed, err := TrimClipboardPNG(padded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(trimmed, small.Bytes()) {
		t.Fatalf("trimmed to %d bytes, want %d", len(trimmed), small.Len())
	}
	if _, err := TrimClipboardPNG(small.Bytes()[:small.Len()-20]); err == nil {
		t.Fatal("a truncated PNG was accepted")
	}
}
