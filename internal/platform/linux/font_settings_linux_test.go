package linux

import (
	"encoding/binary"
	"image"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// The desktop's font rendering is read as a GTK application reads it: the XSETTINGS manager's
// Xft settings first, the Xft resources of the X resource database otherwise, and what neither
// says stays fontconfig's.
func TestFontSettingsFollowTheDesktop(t *testing.T) {
	manager, err := parseXSettings(settingsBlob(binary.LittleEndian,
		map[string]int32{"Xft/Antialias": 1, "Xft/Hinting": 1},
		map[string]string{"Xft/HintStyle": "hintfull", "Xft/RGBA": "bgr"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		manager   xsettings
		resources string
		want      fontSettings
	}{
		{"nothing set", xsettings{}, "", fontSettings{}},
		{"manager", manager, "Xft.rgba:\tnone\nXft.hintstyle:\thintnone\n", fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintFull, SubpixelOrder: cairoSubpixelBGR}},
		{"resources", xsettings{}, "Xft.antialias:\t1\nXft.hinting:\t1\nXft.hintstyle:\thintslight\nXft.rgba:\trgb\nXft.dpi:\t96\n",
			fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB}},
		{"grayscale", xsettings{}, "Xft.antialias: 1\nXft.rgba: none\nXft.hintstyle: hintmedium\n", fontSettings{Antialias: cairoAntialiasGray, HintStyle: cairoHintMedium}},
		{"no antialiasing", xsettings{}, "Xft.antialias: 0\nXft.rgba: rgb\n", fontSettings{Antialias: cairoAntialiasNone}},
		{"no hinting", xsettings{}, "Xft.hinting: 0\nXft.hintstyle: hintfull\n", fontSettings{HintStyle: cairoHintNone}},
		{"vertical", xsettings{}, "Xft.rgba: vbgr\n", fontSettings{Antialias: cairoAntialiasSubpixel, SubpixelOrder: cairoSubpixelVBGR}},
	} {
		if got := fontSettingsFrom(test.manager, test.resources); got != test.want {
			t.Errorf("%s: got %+v, want %+v", test.name, got, test.want)
		}
	}
	for _, test := range []struct {
		name string
		in   fontSettings
		want fontSettings
	}{
		{"subpixel", fontSettings{Antialias: cairoAntialiasSubpixel, SubpixelOrder: cairoSubpixelRGB}, fontSettings{Antialias: cairoAntialiasGray}},
		{"fontconfig default", fontSettings{}, fontSettings{Antialias: cairoAntialiasGray}},
		{"antialiasing off", fontSettings{Antialias: cairoAntialiasNone}, fontSettings{Antialias: cairoAntialiasNone}},
	} {
		if got := test.in.grayscale(); got != test.want {
			t.Errorf("%s: grayscale = %+v, want %+v", test.name, got, test.want)
		}
	}
}

// useSystemFonts replaces the desktop's font settings for the rest of the test.
func useSystemFonts(t testing.TB, settings fontSettings) {
	t.Helper()
	previous := publishedFonts.Load()
	publishedFonts.Store(&settings)
	t.Cleanup(func() { publishedFonts.Store(previous) })
}

// coloredInk counts the pixels of a dark text on white that are tinted (a channel's coverage
// differs from the others') and the darkest luminance in the image.
func coloredInk(pixels *image.RGBA) (colored int, darkest int) {
	darkest = 255
	for y := 0; y < pixels.Bounds().Dy(); y++ {
		for x := 0; x < pixels.Bounds().Dx(); x++ {
			red, green, blue, _ := rgbaAt(pixels, x, y)
			if spread := int(max(red, green, blue)) - int(min(red, green, blue)); spread > 40 {
				colored++
			}
			darkest = min(darkest, (int(red)*77+int(green)*151+int(blue)*28)>>8)
		}
	}
	return colored, darkest
}

// With LCD antialiasing on the desktop the text is blended per channel: dark text on a light
// background is dark with colored edges (a premultiplied blend of the per-channel
// coverage as color would show it pale), and without it the edges are gray.
func TestLCDTextIsBlendedPerChannelAndOnlyWhenTheDesktopAsksForIt(t *testing.T) {
	draw := func(settings fontSettings) *image.RGBA {
		useSystemFonts(t, settings)
		offscreen, err := NewOffscreen(160, 28, 1, platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
		if err != nil {
			t.Skipf("no Vulkan device: %v", err)
		}
		t.Cleanup(offscreen.Close)
		label := platform.TextLabel{Value: "Hamburgefonstiv", Width: 150, Height: 28, FontSize: 14, Color: platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}}
		return renderScene(t, offscreen, nil, []platform.TextLabel{label})
	}
	colored, darkest := coloredInk(draw(fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB}))
	if darkest > 80 || colored < 20 {
		t.Errorf("LCD text on white: darkest pixel %d, %d colored pixels; want dark stems with colored edges", darkest, colored)
	}
	colored, darkest = coloredInk(draw(fontSettings{Antialias: cairoAntialiasGray, HintStyle: cairoHintSlight}))
	if darkest > 80 || colored != 0 {
		t.Errorf("grayscale text on white: darkest pixel %d, %d colored pixels; want dark stems and gray edges", darkest, colored)
	}
}
