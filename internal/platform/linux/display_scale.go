package linux

import (
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/vivaui/viva/internal/platform"
)

// Display scale on X11.
//
// X11 has no scale of its own; a desktop publishes the one the user chose in
// several places, and toolkits read them in an order. Viva reads the same
// places, most specific first, and takes the first that says anything:
//
//  1. VIVA_SCALE, an override for Viva alone.
//  2. QT_SCALE_FACTOR, then QT_SCREEN_SCALE_FACTORS (the first screen's entry
//     when it names none): what KDE and Qt applications export, fractional.
//  3. GDK_SCALE, an integer: what GTK applications export.
//  4. The XSETTINGS manager: Xft/DPI (96 dots per inch is 1.0, 144 is 1.5) when
//     it asks for more than 96, else Gdk/WindowScalingFactor (GNOME keeps
//     Xft/DPI at 96 and scales by the factor).
//  5. Xft.dpi in the X resource database (xrdb), as a desktop without an
//     XSETTINGS manager sets it.
//  6. RandR: the monitor the window is on, when it reports a physical size that
//     makes sense. Only this source differs between monitors, so it is the only
//     one that is looked at again when the window moves.
//
// Everything is a number of device pixels per logical pixel, like the backing
// scale on macOS and the DPI over 96 on Windows. It is never below 1 (Viva does
// not shrink the interface on a dense font setting) and snaps to quarters:
// 1.25, 1.5, 1.75, 2 ... the steps Windows offers, which stops a DPI of 101
// from making every edge fall between two device pixels. The Windows backend
// treats a fractional scale the same way: layout stays in logical pixels and
// everything is rasterized and scissored at logical times scale, rounded to
// device pixels.

// scaleOrigin says which source a scale came from.
type scaleOrigin uint8

const (
	scaleDefault scaleOrigin = iota
	scaleFromEnvironment
	scaleFromXSettings
	scaleFromResource
	scaleFromRandR
)

const (
	minScale  = 1.0
	maxScale  = platform.MaxDisplayScale
	scaleStep = 0.25
	baseDPI   = 96.0
)

// scaleInputs is everything the choice reads, so that a test can supply it.
type scaleInputs struct {
	// Environment variables, empty when unset.
	VivaScale, QtScaleFactor, QtScreenScaleFactors, GdkScale string
	// Screen names the monitor the window is on, for a QT_SCREEN_SCALE_FACTORS
	// entry that names its screen.
	Screen string
	// XSettings is the XSETTINGS manager's settings, if there is one.
	XSettings xsettings
	// ResourceDPI is Xft.dpi from the resource database, 0 when absent.
	ResourceDPI float64
	// Monitor is the RandR monitor the window is on, if known.
	Monitor monitorGeometry
}

// monitorGeometry is a RandR monitor: its size in pixels and in millimeters.
type monitorGeometry struct {
	Name                string
	X, Y, Width, Height int
	WidthMM, HeightMM   int
	Primary             bool
	valid               bool
}

// chooseScale applies the order above.
func chooseScale(in scaleInputs) (float32, scaleOrigin) {
	for _, text := range []string{in.VivaScale, in.QtScaleFactor} {
		if value, ok := parseScaleFactor(text); ok {
			return snapScale(value), scaleFromEnvironment
		}
	}
	if value, ok := qtScreenScale(in.QtScreenScaleFactors, in.Screen); ok {
		return snapScale(value), scaleFromEnvironment
	}
	if value, ok := parseScaleFactor(in.GdkScale); ok {
		return snapScale(value), scaleFromEnvironment
	}
	if in.XSettings.hasDPI && in.XSettings.dpi() > baseDPI*1.125 {
		return snapScale(in.XSettings.dpi() / baseDPI), scaleFromXSettings
	}
	if in.XSettings.hasWindowScale && in.XSettings.WindowScale > 1 {
		return snapScale(float64(in.XSettings.WindowScale)), scaleFromXSettings
	}
	if in.XSettings.hasDPI {
		// A manager that says 96 means it: nothing below it is consulted.
		return 1, scaleFromXSettings
	}
	if in.ResourceDPI > 0 {
		return snapScale(in.ResourceDPI / baseDPI), scaleFromResource
	}
	if scale, ok := monitorScale(in.Monitor); ok {
		return scale, scaleFromRandR
	}
	return 1, scaleDefault
}

// parseScaleFactor reads a positive decimal number, as in GDK_SCALE=2 and
// QT_SCALE_FACTOR=1.5.
func parseScaleFactor(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0, false
	}
	return value, true
}

// qtScreenScale reads QT_SCREEN_SCALE_FACTORS: entries separated by semicolons,
// each a number for the next screen or name=number for a named one. The entry
// for screen wins; without a match, the first entry that names no screen, then
// the first of all.
func qtScreenScale(list, screen string) (float64, bool) {
	var first, unnamed float64
	var haveFirst, haveUnnamed bool
	for _, entry := range strings.Split(list, ";") {
		name, number, named := strings.Cut(entry, "=")
		if !named {
			name, number = "", entry
		}
		value, ok := parseScaleFactor(number)
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if named && screen != "" && name == screen {
			return value, true
		}
		if !haveFirst {
			first, haveFirst = value, true
		}
		if !named && !haveUnnamed {
			unnamed, haveUnnamed = value, true
		}
	}
	if haveUnnamed {
		return unnamed, true
	}
	return first, haveFirst
}

// snapScale rounds to the nearest quarter and keeps the result within 1 to 8.
func snapScale(value float64) float32 {
	if math.IsNaN(value) {
		return minScale
	}
	snapped := math.Round(value/scaleStep) * scaleStep
	return float32(min(max(snapped, minScale), maxScale))
}

// monitorScale derives a scale from a monitor's physical size. It trusts only
// a size that is plausible for a display and the same in both directions: X
// servers and virtual machines often report a made-up size, and a wrong guess
// would make the whole interface too large.
func monitorScale(monitor monitorGeometry) (float32, bool) {
	if !monitor.valid || monitor.Width <= 0 || monitor.Height <= 0 || monitor.WidthMM < 100 || monitor.HeightMM < 60 {
		return 0, false
	}
	across := float64(monitor.Width) * 25.4 / float64(monitor.WidthMM)
	down := float64(monitor.Height) * 25.4 / float64(monitor.HeightMM)
	if across < 40 || across > 800 || down < 40 || down > 800 || math.Abs(across-down) > 0.2*max(across, down) {
		return 0, false
	}
	return snapScale((across + down) / 2 / baseDPI), true
}

// monitorAt returns the monitor that holds the point, or the primary one, or the
// first. The window's center decides which monitor a window is on.
func monitorAt(monitors []monitorGeometry, x, y int) (monitorGeometry, bool) {
	var fallback monitorGeometry
	have := false
	for _, monitor := range monitors {
		if x >= monitor.X && x < monitor.X+monitor.Width && y >= monitor.Y && y < monitor.Y+monitor.Height {
			return monitor, true
		}
		if !have || monitor.Primary {
			fallback, have = monitor, true
		}
	}
	return fallback, have
}

// xsettings holds the settings of the XSETTINGS manager that a scale is derived
// from.
type xsettings struct {
	// WindowScale is Gdk/WindowScalingFactor.
	WindowScale    int32
	hasWindowScale bool
	// XftDPI is Xft/DPI in 1/1024 dots per inch.
	XftDPI int32
	hasDPI bool
	// CursorSize is Gtk/CursorThemeSize in pixels.
	CursorSize    int32
	hasCursorSize bool
	// The font rendering settings of the desktop (font_settings_linux.go): Xft/Antialias
	// and Xft/Hinting are 0 or 1, Xft/HintStyle and Xft/RGBA the names Xft knows.
	XftAntialias, XftHinting int32
	hasAntialias, hasHinting bool
	XftHintStyle, XftRGBA    string
	// The pointer and caret timing of the desktop: Net/DoubleClickTime in milliseconds,
	// Net/DoubleClickDistance in device pixels, Gtk/CursorBlink 0 or 1 and
	// Gtk/CursorBlinkTime the milliseconds of a whole blink.
	DoubleClickTime, DoubleClickDistance int32
	CursorBlink, CursorBlinkTime         int32
	hasDoubleClick, hasDoubleClickDist   bool
	hasCursorBlink, hasCursorBlinkTime   bool
	// GtkFontName is Gtk/FontName, the desktop's interface font as Pango writes it ("Noto Sans 10").
	GtkFontName string
	// ThemeName is Net/ThemeName, PreferDark Gtk/ApplicationPreferDarkTheme and
	// EnableAnimations Gtk/EnableAnimations: what a desktop without the settings portal
	// says of its look and of motion (desktop_settings.go).
	ThemeName                        string
	PreferDark, EnableAnimations     int32
	hasPreferDark, hasEnableAnimated bool
}

func (settings xsettings) dpi() float64 { return float64(settings.XftDPI) / 1024 }

var errXSettings = errors.New("linux: malformed XSETTINGS property")

// parseXSettings decodes the _XSETTINGS_SETTINGS property of the manager (the
// XSETTINGS specification): a byte order, a serial and a count, then settings
// each with a type, a name, the serial of its last change and a value. Settings
// other than the ones used are skipped; a malformed blob is an error and no
// partial result.
func parseXSettings(blob []byte) (xsettings, error) {
	var settings xsettings
	if len(blob) < 12 {
		return settings, errXSettings
	}
	var order binary.ByteOrder
	switch blob[0] {
	case 0:
		order = binary.LittleEndian
	case 1:
		order = binary.BigEndian
	default:
		return settings, errXSettings
	}
	count := order.Uint32(blob[8:12])
	offset := 12
	pad := func(n int) int { return (n + 3) &^ 3 }
	for index := uint32(0); index < count; index++ {
		if offset+4 > len(blob) {
			return xsettings{}, errXSettings
		}
		kind := blob[offset]
		nameLength := int(order.Uint16(blob[offset+2 : offset+4]))
		offset += 4
		if offset+pad(nameLength)+4 > len(blob) {
			return xsettings{}, errXSettings
		}
		name := string(blob[offset : offset+nameLength])
		offset += pad(nameLength) + 4 // the name, then the serial of the last change
		switch kind {
		case 0: // integer
			if offset+4 > len(blob) {
				return xsettings{}, errXSettings
			}
			value := int32(order.Uint32(blob[offset : offset+4]))
			offset += 4
			switch name {
			case "Gdk/WindowScalingFactor":
				settings.WindowScale, settings.hasWindowScale = value, true
			case "Xft/DPI":
				settings.XftDPI, settings.hasDPI = value, true
			case "Gtk/CursorThemeSize":
				settings.CursorSize, settings.hasCursorSize = value, true
			case "Xft/Antialias":
				settings.XftAntialias, settings.hasAntialias = value, true
			case "Xft/Hinting":
				settings.XftHinting, settings.hasHinting = value, true
			case "Net/DoubleClickTime":
				settings.DoubleClickTime, settings.hasDoubleClick = value, true
			case "Net/DoubleClickDistance":
				settings.DoubleClickDistance, settings.hasDoubleClickDist = value, true
			case "Gtk/CursorBlink":
				settings.CursorBlink, settings.hasCursorBlink = value, true
			case "Gtk/CursorBlinkTime":
				settings.CursorBlinkTime, settings.hasCursorBlinkTime = value, true
			case "Gtk/ApplicationPreferDarkTheme":
				settings.PreferDark, settings.hasPreferDark = value, true
			case "Gtk/EnableAnimations":
				settings.EnableAnimations, settings.hasEnableAnimated = value, true
			}
		case 1: // string
			if offset+4 > len(blob) {
				return xsettings{}, errXSettings
			}
			length := int(order.Uint32(blob[offset : offset+4]))
			offset += 4
			if length < 0 || offset+pad(length) > len(blob) {
				return xsettings{}, errXSettings
			}
			switch name {
			case "Xft/HintStyle":
				settings.XftHintStyle = string(blob[offset : offset+length])
			case "Xft/RGBA":
				settings.XftRGBA = string(blob[offset : offset+length])
			case "Gtk/FontName":
				settings.GtkFontName = string(blob[offset : offset+length])
			case "Net/ThemeName":
				settings.ThemeName = string(blob[offset : offset+length])
			}
			offset += pad(length)
		case 2: // color: four 16-bit values
			offset += 8
			if offset > len(blob) {
				return xsettings{}, errXSettings
			}
		default:
			return xsettings{}, errXSettings
		}
	}
	return settings, nil
}

// resourceDPI finds Xft.dpi in an X resource database (the RESOURCE_MANAGER
// property of the root window), 0 when it is not set or not a number.
func resourceDPI(database string) float64 {
	result := 0.0
	for _, line := range strings.Split(database, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "Xft.dpi" {
			continue
		}
		if number, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && number > 0 && !math.IsInf(number, 0) {
			result = number
		}
	}
	return result
}

// resourceCursorSize finds Xcursor.size in an X resource database, 0 when absent.
func resourceCursorSize(database string) int {
	result := 0
	for _, line := range strings.Split(database, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "Xcursor.size" {
			continue
		}
		if number, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && number > 0 && number <= 512 {
			result = number
		}
	}
	return result
}

// deviceLength is a logical length in device pixels, rounded to the nearest.
func deviceLength(logical int, scale float32) uint32 {
	return uint32(max(1, int(math.Floor(float64(logical)*float64(scale)+0.5))))
}
