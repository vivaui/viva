package dmg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writePNG writes a PNG of the pixel size tagged with the dots per metre.
func writePNG(t *testing.T, width, height int, perMetre uint32) string {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	if perMetre != 0 {
		chunk := binary.BigEndian.AppendUint32(nil, 9)
		chunk = append(chunk, "pHYs"...)
		chunk = binary.BigEndian.AppendUint32(chunk, perMetre)
		chunk = binary.BigEndian.AppendUint32(chunk, perMetre)
		chunk = append(chunk, 1)
		// The checksum is not read by the decoder under test.
		chunk = append(chunk, 0, 0, 0, 0)
		// pHYs goes right after IHDR: signature 8 + length/type/data/crc 25.
		data = append(append(append([]byte(nil), data[:33]...), chunk...), data[33:]...)
	}
	path := filepath.Join(t.TempDir(), "background.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBackgroundSizeFollowsItsResolution(t *testing.T) {
	for _, test := range []struct {
		name          string
		perMetre      uint32
		width, height int
		want          string
	}{
		{"untagged is 1x", 0, 600, 400, "600x400"},
		{"144 dpi is 2x", 5669, 1200, 800, "600x400"},
		{"odd pixels at 2x", 5669, 1201, 800, "not a whole number of points"},
		{"an odd resolution", 4000, 600, 400, "tag it 72 dpi"},
	} {
		layout, err := Resolve(Config{Background: writePNG(t, test.width, test.height, test.perMetre)})
		if test.want[0] >= '0' && test.want[0] <= '9' {
			if err != nil || fmt.Sprintf("%dx%d", layout.Width, layout.Height) != test.want {
				t.Errorf("%s: %+v, %v; want %s", test.name, layout, err, test.want)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: error = %v, want it to contain %q", test.name, err, test.want)
		}
	}
}

func TestIconsDefaultToEitherSideOfTheMiddle(t *testing.T) {
	layout, err := Resolve(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if layout.App.X >= layout.Width/2 || layout.Applications.X <= layout.Width/2 || layout.App.Y != layout.Applications.Y {
		t.Fatalf("icons at %+v and %+v in a %dx%d window", layout.App, layout.Applications, layout.Width, layout.Height)
	}
}

func TestPropertyListsAreReadByTheSystem(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plutil is a macOS tool")
	}
	data, err := encodeBinaryPlist([]plistEntry{
		plistPair("flag", plistBoolean(true)),
		plistPair("size", plistInteger(300)),
		plistPair("ratio", plistFloat(0.5)),
		plistPair("text", plistText("{{200, 400}, {600, 428}}")),
		plistPair("long text", plistText(strings.Repeat("x", 40))),
		plistPair("blob", plistBytes(bytes.Repeat([]byte{7}, 40))),
		plistPair("名称", plistText("Motty")),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "x.plist")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("plutil", "-p", path).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"flag" => 1`) && !strings.Contains(string(out), `"flag" => true`) {
		t.Errorf("missing boolean flag in:\n%s", out)
	}
	for _, want := range []string{`"size" => 300`, `"ratio" => 0.5`, `"text" => "{{200, 400}, {600, 428}}"`, `"名称" => "Motty"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
}

func TestBuiltImageHoldsTheLayoutTheFinderReads(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("disk images are made on macOS")
	}
	bundle := filepath.Join(t.TempDir(), "Sample.app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	layout, err := Resolve(Config{})
	if err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(t.TempDir(), "sample.dmg")
	if err := Build(Options{Bundle: bundle, Volume: "Sample", Image: image, Layout: layout}); err != nil {
		t.Fatal(err)
	}
	mount := t.TempDir()
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mount, image).CombinedOutput(); err != nil {
		t.Fatalf("attach: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("hdiutil", "detach", "-force", mount).Run() })

	if target, err := os.Readlink(filepath.Join(mount, "Applications")); err != nil || target != "/Applications" {
		t.Fatalf("Applications link = %q, %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(mount, backgroundFile)); err != nil {
		t.Fatalf("the background picture is not in the image: %v", err)
	}
	store, err := os.ReadFile(filepath.Join(mount, ".DS_Store"))
	if err != nil {
		t.Fatalf("the image has no .DS_Store: %v", err)
	}
	if string(store[4:8]) != "Bud1" {
		t.Fatalf(".DS_Store starts with %q", store[:8])
	}
	// Record names are UTF-16 in the file; the icon names are how the Finder
	// finds its positions.
	for _, name := range []string{"Sample.app", "Applications"} {
		var units []byte
		for _, r := range name {
			units = append(units, 0, byte(r))
		}
		if !bytes.Contains(store, append(units, "Iloc"...)) {
			t.Errorf(".DS_Store has no position for %s", name)
		}
	}
}

func TestDefaultBackgroundIsTheWindowsSizeAndHasAnArrowBetweenTheIcons(t *testing.T) {
	layout, err := Resolve(Config{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := layout.defaultBackground()
	if err != nil {
		t.Fatal(err)
	}
	width, height, err := pictureSize("default", data)
	if err != nil || width != layout.Width || height != layout.Height {
		t.Fatalf("size = %dx%d, %v; want %dx%d points", width, height, err, layout.Width, layout.Height)
	}
	picture, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	luminance := func(x, y int) uint32 {
		r, _, _, _ := picture.At(x*defaultScale, y*defaultScale).RGBA()
		return r >> 8
	}
	middle := (layout.App.X + layout.Applications.X) / 2
	if arrow, ground := luminance(middle, layout.App.Y), luminance(middle, layout.App.Y-60); arrow+60 > ground {
		t.Errorf("the middle of the arrow is %d bright against a ground of %d", arrow, ground)
	}
}

func TestNoArrowIsDrawnWhereTheIconsLeaveNoRoom(t *testing.T) {
	near := &Point{240, 185}
	layout, err := Resolve(Config{App: &Point{200, 185}, Applications: near})
	if err != nil {
		t.Fatal(err)
	}
	data, err := layout.defaultBackground()
	if err != nil {
		t.Fatal(err)
	}
	picture, _ := png.Decode(bytes.NewReader(data))
	first, last := picture.At(0, 2*185), picture.At(0, 2*185)
	for x := 0; x < picture.Bounds().Dx(); x++ {
		if picture.At(x, 2*185) != first {
			last = picture.At(x, 2*185)
			break
		}
	}
	if last != first {
		t.Fatal("an arrow was drawn between icons with no room for it")
	}
}
