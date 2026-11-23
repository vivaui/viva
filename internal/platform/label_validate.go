package platform

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/gfx"
)

// The checks and comparisons of text labels that every renderer makes.

func ValidateTextLabel(index int, label TextLabel) error {
	for _, value := range []float32{label.X, label.Y, label.Width, label.Height, label.FontSize, label.Weight, label.CellWidth} {
		if !finiteFloat(value) {
			return fmt.Errorf("platform: text label %d has a non-finite metric", index)
		}
	}
	if label.Width < 0 || label.Height < 0 || len(label.Bitmap) == 0 && !ValidFontSize(label.FontSize) {
		return fmt.Errorf("platform: text label %d has invalid bounds or font size", index)
	}
	if len(label.Runs) > 0 && label.CellWidth <= 0 {
		return fmt.Errorf("platform: text label %d has invalid cell width", index)
	}
	if len(label.Bitmap) > 0 {
		if label.BitmapWidth <= 0 || label.BitmapHeight <= 0 || label.BitmapStride < label.BitmapWidth*4 {
			return fmt.Errorf("platform: bitmap label %d has invalid dimensions", index)
		}
		required := (label.BitmapHeight-1)*label.BitmapStride + label.BitmapWidth*4
		if required < 0 || len(label.Bitmap) < required {
			return fmt.Errorf("platform: bitmap label %d has a short payload", index)
		}
	}
	return nil
}

func SameTextRuns(left, right []gfx.TextRun) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func SameBitmapBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	return len(left) == 0 || &left[0] == &right[0]
}

func TightBitmap(source []byte, width, height, stride int) []byte {
	if stride == width*4 {
		return source[:width*height*4]
	}
	pixels := make([]byte, width*height*4)
	for row := 0; row < height; row++ {
		copy(pixels[row*width*4:(row+1)*width*4], source[row*stride:row*stride+width*4])
	}
	return pixels
}

func ValidateTextLabels(labels []TextLabel) error {
	for index, label := range labels {
		if err := ValidateTextLabel(index, label); err != nil {
			return err
		}
	}
	return nil
}

func finiteFloat(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

// MaxOffscreenSize is the largest side, in device pixels, of an offscreen scene.
const MaxOffscreenSize = 16384
