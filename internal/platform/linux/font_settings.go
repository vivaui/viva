package linux

import (
	"strconv"
	"strings"
)

// Cairo's font options as the desktop sets them. A GTK application draws its
// text with the antialiasing, hinting and subpixel order of the session's XSETTINGS
// (Xft/Antialias, Xft/Hinting, Xft/HintStyle, Xft/RGBA) or, without a settings manager,
// of the Xft resources in the X resource database; Qt and Xft applications read the same
// resources. Pango alone only asks fontconfig, so a desktop that sets LCD text or full
// hinting would show Viva's text in another rendering than its neighbours'.

// The values of cairo_antialias_t, cairo_hint_style_t and cairo_subpixel_order_t.
const (
	cairoAntialiasDefault  = 0
	cairoAntialiasNone     = 1
	cairoAntialiasGray     = 2
	cairoAntialiasSubpixel = 3
	cairoHintDefault       = 0
	cairoHintNone          = 1
	cairoHintSlight        = 2
	cairoHintMedium        = 3
	cairoHintFull          = 4
	cairoSubpixelRGB       = 1
	cairoSubpixelBGR       = 2
	cairoSubpixelVRGB      = 3
	cairoSubpixelVBGR      = 4
	cairoHintMetricsOff    = 1
	cairoHintMetricsOn     = 2
)

// textHintMetrics is the cairo_hint_metrics_t layout and drawing share. Metrics are not
// hinted (nor are glyph positions rounded: Pango 1.44 does that on its own), so a line is as
// wide at a display scale as the scale times its width at scale 1: layout does not change
// when a window moves to another monitor, and the width layout measured is the width drawn.
// Hinted metrics round every advance to the device pixel, which changed a line's width by up
// to 17 percent between scales 1 and 1.5 on the test machine.
const textHintMetrics = cairoHintMetricsOff

// fontSettings are the font options the desktop asks for; a zero field leaves the choice to
// fontconfig, as it is without any setting.
type fontSettings struct {
	Antialias, HintStyle, SubpixelOrder int32
	// Family is the desktop's interface font as Pango writes it ("Noto Sans 10"), from
	// XSETTINGS Gtk/FontName; empty leaves the choice to fontconfig's sans alias.
	Family string
}

// subpixel reports whether the text is drawn with LCD antialiasing.
func (settings fontSettings) subpixel() bool { return settings.Antialias == cairoAntialiasSubpixel }

// grayscale is the same settings without LCD antialiasing: for text that cannot be blended per
// channel. A default left to fontconfig is made explicit, because fontconfig may choose LCD
// antialiasing; only a desktop that turned antialiasing off keeps it off.
func (settings fontSettings) grayscale() fontSettings {
	if settings.Antialias != cairoAntialiasNone {
		settings.Antialias, settings.SubpixelOrder = cairoAntialiasGray, 0
	}
	return settings
}

// fontSettingsFrom decides as GTK does: the XSETTINGS manager's value when it has one, the X
// resource database's otherwise.
func fontSettingsFrom(manager xsettings, resources string) fontSettings {
	antialias, hasAntialias := int32(manager.XftAntialias), manager.hasAntialias
	hinting, hasHinting := int32(manager.XftHinting), manager.hasHinting
	style, rgba := manager.XftHintStyle, manager.XftRGBA
	for _, line := range strings.Split(resources, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Xft.antialias":
			if number, err := strconv.Atoi(value); err == nil && !hasAntialias {
				antialias, hasAntialias = int32(number), true
			}
		case "Xft.hinting":
			if number, err := strconv.Atoi(value); err == nil && !hasHinting {
				hinting, hasHinting = int32(number), true
			}
		case "Xft.hintstyle":
			if style == "" {
				style = value
			}
		case "Xft.rgba":
			if rgba == "" {
				rgba = value
			}
		}
	}
	settings := fontSettings{Family: manager.GtkFontName}
	switch rgba {
	case "rgb":
		settings.SubpixelOrder = cairoSubpixelRGB
	case "bgr":
		settings.SubpixelOrder = cairoSubpixelBGR
	case "vrgb":
		settings.SubpixelOrder = cairoSubpixelVRGB
	case "vbgr":
		settings.SubpixelOrder = cairoSubpixelVBGR
	}
	switch {
	case hasAntialias && antialias == 0:
		settings.Antialias, settings.SubpixelOrder = cairoAntialiasNone, 0
	case settings.SubpixelOrder != 0:
		settings.Antialias = cairoAntialiasSubpixel
	case hasAntialias:
		settings.Antialias = cairoAntialiasGray
	}
	switch {
	case hasHinting && hinting == 0:
		settings.HintStyle = cairoHintNone
	case style == "hintnone":
		settings.HintStyle = cairoHintNone
	case style == "hintslight":
		settings.HintStyle = cairoHintSlight
	case style == "hintmedium":
		settings.HintStyle = cairoHintMedium
	case style == "hintfull":
		settings.HintStyle = cairoHintFull
	}
	return settings
}
