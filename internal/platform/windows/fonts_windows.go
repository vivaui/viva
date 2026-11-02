package windows

import (
	"github.com/vivaui/viva/internal/platform"
	"sort"
	"sync"
	"syscall"
	"unsafe"
)

// enumLogFont is the front of ENUMLOGFONTEXW: a LOGFONTW, whose face name is the
// family.
type enumLogFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

// newTextMetric is the front of NEWTEXTMETRICEXW: a TEXTMETRICW up to its pitch
// byte.
type newTextMetric struct {
	Height, Ascent, Descent, InternalLeading, ExternalLeading int32
	AveCharWidth, MaxCharWidth, Weight, Overhang              int32
	DigitizedAspectX, DigitizedAspectY                        int32
	FirstChar, LastChar, DefaultChar, BreakChar               uint16
	Italic, Underlined, StruckOut, PitchAndFamily             byte
}

const (
	defaultCharset = 1
	// tmpfFixedPitch is set for fonts that are NOT fixed-pitch: the name is a
	// historical accident.
	tmpfFixedPitch = 1
	// vectorFonts excludes raster fonts, which cannot be scaled to a size.
	rasterFontType = 1
)

var (
	gdi                 = systemDLL("gdi32.dll")
	enumFontFamiliesEx  = gdi.NewProc("EnumFontFamiliesExW")
	fontEnumMu          sync.Mutex
	fontEnumFound       map[string]struct{}
	fontEnumCallbackPtr uintptr
	fontEnumMono        bool
)

func fontEnumCallback(logFont *enumLogFont, metric *newTextMetric, fontType uint32, _ uintptr, mono bool) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("font enumeration", recovered)
			result = 1
		}
	}()
	if logFont == nil || metric == nil || fontType&rasterFontType != 0 || mono && metric.PitchAndFamily&tmpfFixedPitch != 0 {
		return 1
	}
	units := 0
	for units < len(logFont.FaceName) && logFont.FaceName[units] != 0 {
		units++
	}
	if units >= len(logFont.FaceName)-1 {
		// The face name field holds 31 characters, and the system cuts a longer
		// name to fit: a name that fills it may be a cut one, which names no family.
		return 1
	}
	name := syscall.UTF16ToString(logFont.FaceName[:])
	if name != "" && name[0] != '@' { // @-names are the vertical writing variants
		fontEnumFound[name] = struct{}{}
	}
	return 1
}

// MonospaceFamilies lists the installed fixed-pitch font families, sorted.
func MonospaceFamilies() []string { return fontFamilies(true) }

// FontFamilies lists every installed font family, sorted. A family whose name fills
// the 31 characters of a LOGFONT is left out (see fontEnumCallback).
func FontFamilies() []string { return fontFamilies(false) }

// fontFamilies enumerates the installed families, only the fixed-pitch ones when
// monospace is set. Raster fonts and the vertical-writing variants are left out
// either way.
func fontFamilies(mono bool) []string {
	fontEnumMu.Lock()
	defer fontEnumMu.Unlock()
	if fontEnumCallbackPtr == 0 {
		fontEnumCallbackPtr = syscall.NewCallback(func(logFont *enumLogFont, metric *newTextMetric, fontType uint32, extra uintptr) uintptr {
			return fontEnumCallback(logFont, metric, fontType, extra, fontEnumMono)
		})
	}
	fontEnumMono = mono
	fontEnumFound = make(map[string]struct{})
	defer func() { fontEnumFound = nil }()
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return nil
	}
	defer releaseDC.Call(0, dc)
	var filter enumLogFont
	filter.CharSet = defaultCharset
	enumFontFamiliesEx.Call(dc, uintptr(unsafe.Pointer(&filter)), fontEnumCallbackPtr, 0, 0)
	names := make([]string, 0, len(fontEnumFound))
	for name := range fontEnumFound {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return lowerASCII(names[i]) < lowerASCII(names[j]) })
	return names
}

func lowerASCII(value string) string {
	bytes := []byte(value)
	for index, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[index] = b + 'a' - 'A'
		}
	}
	return string(bytes)
}
