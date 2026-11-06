package ui

import (
	"math"
	"testing"
)

func TestGaussianBlurBitmapSpreadsAnImpulseSymmetrically(t *testing.T) {
	pixels := make([]byte, 9*4)
	center := 4 * 4
	pixels[center+0] = 128
	pixels[center+2] = 255
	pixels[center+3] = 255
	source := Bitmap{Pix: pixels, Width: 9, Height: 1, Stride: 9 * 4, ID: 7, Version: 3}

	blurred := GaussianBlurBitmap(source, 1)
	if blurred.Width != 9 || blurred.Height != 1 || blurred.Stride != 36 || blurred.ID != 0 || blurred.Version != 0 {
		t.Fatalf("blurred bitmap = %+v", blurred)
	}
	if &blurred.Pix[0] == &source.Pix[0] {
		t.Fatal("blur reused source storage")
	}
	if blurred.Pix[center+3] >= 255 || blurred.Pix[center+3] <= blurred.Pix[center-4+3] {
		t.Fatalf("alpha around impulse = %v", alphaRow(blurred))
	}
	for distance := 1; distance <= 4; distance++ {
		left := blurred.Pix[(4-distance)*4+3]
		right := blurred.Pix[(4+distance)*4+3]
		if left != right {
			t.Fatalf("distance %d: left alpha %d, right alpha %d", distance, left, right)
		}
	}
	if blurred.Pix[center+2] < blurred.Pix[center+0] {
		t.Fatalf("premultiplied channels changed order: B=%d R=%d", blurred.Pix[center], blurred.Pix[center+2])
	}
}

func TestGaussianBlurBitmapRespectsStrideAndLeavesSourceUntouched(t *testing.T) {
	pixels := []byte{
		0, 0, 0, 0, 0, 0, 255, 255, 9, 9, 9, 9,
		0, 255, 0, 255, 0, 0, 0, 0, 8, 8, 8, 8,
	}
	before := append([]byte(nil), pixels...)
	source := Bitmap{Pix: pixels, Width: 2, Height: 2, Stride: 12}
	blurred := GaussianBlurBitmap(source, 0.8)

	if blurred.Stride != 8 || len(blurred.Pix) != 16 {
		t.Fatalf("blurred storage = stride %d len %d", blurred.Stride, len(blurred.Pix))
	}
	for index := range pixels {
		if pixels[index] != before[index] {
			t.Fatalf("source byte %d changed from %d to %d", index, before[index], pixels[index])
		}
	}
}

func TestGaussianBlurBitmapRejectsInvalidStorageAndNoOpKeepsIdentity(t *testing.T) {
	invalid := Bitmap{Pix: make([]byte, 4), Width: 2, Height: 2, Stride: 8}
	if got := GaussianBlurBitmap(invalid, 2); got.Width != 0 || len(got.Pix) != 0 {
		t.Fatalf("invalid bitmap accepted: %+v", got)
	}
	source := Bitmap{Pix: make([]byte, 16), Width: 2, Height: 2, Stride: 8, ID: 11, Version: 4}
	for _, sigma := range []float32{0, float32(math.NaN()), float32(math.Inf(1))} {
		got := GaussianBlurBitmap(source, sigma)
		if got.ID != source.ID || got.Version != source.Version || &got.Pix[0] != &source.Pix[0] {
			t.Fatalf("no-op blur %v copied or changed identity: %+v", sigma, got)
		}
	}
}

func BenchmarkGaussianBlurBitmap128x24(b *testing.B) {
	source := Bitmap{Pix: make([]byte, 128*24*4), Width: 128, Height: 24, Stride: 128 * 4}
	for index := 0; index < len(source.Pix); index += 16 {
		source.Pix[index+0] = 120
		source.Pix[index+1] = 120
		source.Pix[index+2] = 120
		source.Pix[index+3] = 220
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = GaussianBlurBitmap(source, 2.5)
	}
}

func alphaRow(bitmap Bitmap) []byte {
	alpha := make([]byte, bitmap.Width)
	for x := range bitmap.Width {
		alpha[x] = bitmap.Pix[x*4+3]
	}
	return alpha
}
