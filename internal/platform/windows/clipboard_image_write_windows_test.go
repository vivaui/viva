package windows

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestSetClipboardImageRoundTripsThroughTheRealClipboard writes a picture with
// SetClipboardImage and takes it back with ClipboardImagePNG, then asks the .NET
// clipboard for the same picture: that reader asks Windows for a bitmap, which
// the copy offers as CF_DIB, so it proves an application that does not know the
// registered "PNG" format still gets the picture and its colours. It replaces
// the clipboard, so it runs only on request, on a machine whose clipboard may be
// overwritten.
func TestSetClipboardImageRoundTripsThroughTheRealClipboard(t *testing.T) {
	if os.Getenv("VIVA_WINDOWS_CLIPBOARD") == "" {
		t.Skip("set VIVA_WINDOWS_CLIPBOARD=1 to overwrite the real clipboard")
	}
	fill := color.NRGBA{R: 0, G: 128, B: 0, A: 255}
	topLeft := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	bottomRight := color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	picture := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for y := range 3 {
		for x := range 4 {
			picture.SetNRGBA(x, y, fill)
		}
	}
	picture.SetNRGBA(0, 0, topLeft)
	picture.SetNRGBA(3, 2, bottomRight)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}

	if err := SetClipboardImage(encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	read, err := ClipboardImagePNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read, encoded.Bytes()) {
		t.Fatalf("read back %d bytes, wrote %d: the clipboard did not keep the copy", len(read), encoded.Len())
	}

	script := `Add-Type -AssemblyName System.Windows.Forms
$picture = [System.Windows.Forms.Clipboard]::GetImage()
if ($picture -eq $null) { 'none' } else {
  $formats = ([System.Windows.Forms.Clipboard]::GetDataObject().GetFormats() -join ',')
  "$($picture.Width)x$($picture.Height)|$formats|$($picture.GetPixel(0,0).ToArgb())|$($picture.GetPixel(3,2).ToArgb())|$($picture.GetPixel(1,1).ToArgb())"
}`
	output, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("read the clipboard with .NET: %v: %s", err, output)
	}
	fields := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(fields) != 5 {
		t.Fatalf(".NET answered %q, want a size, the formats and three pixels", output)
	}
	if fields[0] != "4x3" {
		t.Fatalf("the bitmap a .NET reader takes is %s, want 4x3", fields[0])
	}
	t.Logf("formats on the clipboard: %s", fields[1])
	argb := func(value color.NRGBA) string {
		return fmt.Sprint(int32(uint32(value.A)<<24 | uint32(value.R)<<16 | uint32(value.G)<<8 | uint32(value.B)))
	}
	for index, check := range []struct {
		position string
		want     color.NRGBA
	}{{"top left", topLeft}, {"bottom right", bottomRight}, {"middle", fill}} {
		if fields[index+2] != argb(check.want) {
			t.Fatalf(".NET %s pixel = %s, want %s (%v)", check.position, fields[index+2], argb(check.want), check.want)
		}
	}
}
