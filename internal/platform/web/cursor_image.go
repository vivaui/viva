package web

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strconv"

	"github.com/vivaui/viva/ui"
)

// cursorImageCSS is the CSS cursor value that shows the picture: a PNG data URL with the hotspot,
// and the arrow to fall back on for a browser that refuses the image. A picture made for more
// than one image pixel per CSS pixel is offered as an image-set at that resolution, and its
// hotspot is in CSS pixels.
func cursorImageCSS(cursor *ui.CursorImage) (string, error) {
	width, height := cursor.Size()
	source := cursor.Pixels()
	straight := image.NewNRGBA(image.Rect(0, 0, width, height))
	for index := 0; index < len(source); index += 4 {
		straight.Pix[index], straight.Pix[index+1], straight.Pix[index+2], straight.Pix[index+3] = source[index+2], source[index+1], source[index], source[index+3]
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, straight); err != nil {
		return "", fmt.Errorf("web: encode cursor image: %w", err)
	}
	url := `url("data:image/png;base64,` + base64.StdEncoding.EncodeToString(encoded.Bytes()) + `")`
	hotX, hotY := cursor.Hotspot()
	scale := cursor.Scale()
	if scale == 1 {
		return url + " " + strconv.Itoa(hotX) + " " + strconv.Itoa(hotY) + ", default", nil
	}
	return "image-set(" + url + " " + strconv.FormatFloat(float64(scale), 'f', -1, 32) + "x) " +
		strconv.FormatFloat(float64(hotX)/float64(scale), 'f', -1, 64) + " " +
		strconv.FormatFloat(float64(hotY)/float64(scale), 'f', -1, 64) + ", default", nil
}
