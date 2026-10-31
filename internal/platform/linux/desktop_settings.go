package linux

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// What a Linux desktop says about itself, and where. There is no single place: a modern
// session answers through the settings portal (org.freedesktop.portal.Settings), an X11
// session of an older desktop through its XSETTINGS manager, and one without either only
// through the toolkits' configuration files. This file holds the decisions, with every
// input passed in so that a test can supply it; portal_unix.go and desktop_bus_linux.go
// read the inputs.

// tri is a yes or no that the desktop may not have given.
type tri uint8

const (
	triUnknown tri = iota
	triFalse
	triTrue
)

func triOf(value bool) tri {
	if value {
		return triTrue
	}
	return triFalse
}

// themeNameIsDark says whether a GTK theme's name is of the dark kind, as the usual
// ones are named: Adwaita-dark, Breeze-Dark, Yaru-dark. GTK_THEME writes the variant
// after a colon: Adwaita:dark.
func themeNameIsDark(name string) bool {
	return strings.Contains(strings.ToLower(name), "dark")
}

// darkTheme is the look XSETTINGS asks for: an application preference for the dark theme,
// else the theme's name.
func (settings xsettings) darkTheme() (dark, known bool) {
	if settings.hasPreferDark && settings.PreferDark != 0 {
		return true, true
	}
	if settings.ThemeName != "" {
		return themeNameIsDark(settings.ThemeName), true
	}
	return false, settings.hasPreferDark
}

// reducedMotion is XSETTINGS' Gtk/EnableAnimations turned off.
func (settings xsettings) reducedMotion() tri {
	if !settings.hasEnableAnimated {
		return triUnknown
	}
	return triOf(settings.EnableAnimations == 0)
}

// iniValue is the value of key in section of an INI text (a GTK settings.ini, KDE's
// kdeglobals); a key before any section belongs to the section "".
func iniValue(text, section, key string) (string, bool) {
	current := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			current = strings.Trim(line, "[]")
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if ok && current == section && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

// gtkSettingsDark reads a GTK 3 or 4 settings.ini.
func gtkSettingsDark(text string) (dark, known bool) {
	if value, ok := iniValue(text, "Settings", "gtk-application-prefer-dark-theme"); ok {
		if prefer, err := strconv.Atoi(value); err == nil && prefer != 0 || strings.EqualFold(value, "true") {
			return true, true
		}
	}
	if name, ok := iniValue(text, "Settings", "gtk-theme-name"); ok && name != "" {
		return themeNameIsDark(name), true
	}
	return false, false
}

// kdeColorSchemeDark reads the color scheme KDE records in kdeglobals: BreezeDark and the
// like are dark.
func kdeColorSchemeDark(text string) (dark, known bool) {
	if name, ok := iniValue(text, "General", "ColorScheme"); ok && name != "" {
		return themeNameIsDark(name), true
	}
	return false, false
}

// kdeAnimationsOff reads KDE's animation speed: a duration factor of 0 is "instant".
func kdeAnimationsOff(text string) tri {
	value, ok := iniValue(text, "KDE", "AnimationDurationFactor")
	if !ok {
		return triUnknown
	}
	factor, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return triUnknown
	}
	return triOf(factor <= 0)
}

// desktopFiles is the text of the configuration files the toolkits keep; empty when absent.
type desktopFiles struct {
	GTK3, GTK4, KDEGlobals string
}

// darkFallback is the look a desktop without the settings portal asks for, from the most
// live source to the most static: the XSETTINGS manager, GTK_THEME, GTK's settings.ini,
// KDE's color scheme.
func darkFallback(settings xsettings, haveSettings bool, gtkTheme string, files desktopFiles) (dark, known bool) {
	if haveSettings {
		if dark, known = settings.darkTheme(); known {
			return dark, true
		}
	}
	if gtkTheme != "" {
		return themeNameIsDark(gtkTheme), true
	}
	for _, text := range []string{files.GTK4, files.GTK3} {
		if dark, known = gtkSettingsDark(text); known {
			return dark, true
		}
	}
	return kdeColorSchemeDark(files.KDEGlobals)
}

// motionFallback is the preference for reduced animation from what a desktop without the
// portal offers.
func motionFallback(settings xsettings, haveSettings bool, files desktopFiles) tri {
	if haveSettings {
		if state := settings.reducedMotion(); state != triUnknown {
			return state
		}
	}
	return kdeAnimationsOff(files.KDEGlobals)
}

// portalValues is what the settings portal last said.
type portalValues struct {
	// colorScheme is org.freedesktop.appearance color-scheme: 1 dark, 2 light, 0 no preference.
	colorScheme    uint32
	hasColorScheme bool
	// reducedMotion is org.freedesktop.appearance reduced-motion (1 asks for less motion).
	reducedMotion tri
	// animations is org.gnome.desktop.interface enable-animations.
	animations tri
}

// The portal's namespaces and keys that Viva reads.
const (
	portalAppearance   = "org.freedesktop.appearance"
	portalGnomeUI      = "org.gnome.desktop.interface"
	portalColorScheme  = "color-scheme"
	portalReducedMo    = "reduced-motion"
	portalAnimationsOn = "enable-animations"
)

func (values portalValues) dark() (dark, known bool) {
	if !values.hasColorScheme {
		return false, false
	}
	switch values.colorScheme {
	case 1:
		return true, true
	case 2:
		return false, true
	}
	return false, false
}

// reduceMotion is whether the portal asks for less motion.
func (values portalValues) reduceMotion() tri {
	if values.reducedMotion != triUnknown {
		return values.reducedMotion
	}
	switch values.animations {
	case triTrue:
		return triFalse
	case triFalse:
		return triTrue
	}
	return triUnknown
}

// apply records a value the portal reported and says whether it differs from before.
func (values *portalValues) apply(namespace, key string, value portalNumber) bool {
	before := *values
	switch {
	case namespace == portalAppearance && key == portalColorScheme && value.kind == 'u':
		values.colorScheme, values.hasColorScheme = value.number, true
	case namespace == portalAppearance && key == portalReducedMo && value.kind == 'u':
		values.reducedMotion = triOf(value.number != 0)
	case namespace == portalGnomeUI && key == portalAnimationsOn && value.kind == 'b':
		values.animations = triOf(value.number != 0)
	}
	return *values != before
}

// portalNumber is a setting's value as the portal sends it: an unsigned number ('u') or a
// boolean ('b'). Other kinds are left out; 0 for kind means the value was something else.
type portalNumber struct {
	kind   byte
	number uint32
}

// desktopState is everything Viva knows of the desktop's look and motion preference, shared
// by the goroutines that ask and the event loop that hears changes.
var desktopState struct {
	sync.Mutex
	portal portalValues
	// portalRead says the portal has been asked; portalAt when.
	portalRead bool
	portalAt   time.Time
	// portalGone says the portal is permanently unsupported. Temporary bus and
	// service failures instead become stale and are asked again.
	portalGone bool
	// watching says a connection delivers the portal's changes, so the values are current.
	watching bool

	xsettings    xsettings
	haveSettings bool
	resources    string
	fonts        fontSettings
	haveFonts    bool

	files   desktopFiles
	filesAt time.Time
}

// publishedFonts is the font rendering the desktop asks for, as the last window's settings
// tracker read it. Text drawing reads it for every label, so it is not behind the lock.
var publishedFonts atomic.Pointer[fontSettings]

// stale is how long a value read without a change notification is trusted.
const stale = 5 * time.Second

// settingsChange says which of the desktop's settings differ from before.
type settingsChange uint8

const (
	changedAppearance settingsChange = 1 << iota
	changedMotion
	changedFonts
)

// platformChange is the part of a change that applications hear of as a DesktopSetting.
func (change settingsChange) platformChange() platform.DesktopSetting {
	var result platform.DesktopSetting
	if change&changedFonts != 0 {
		result |= platform.SettingFonts
	}
	if change&changedMotion != 0 {
		result |= platform.SettingMotion
	}
	return result
}

// resolveLocked is the look and motion preference as they stand. desktopState is locked.
func resolveLocked(read func(name string) string) (dark, darkKnown bool, reduce tri) {
	if dark, darkKnown = desktopState.portal.dark(); !darkKnown {
		dark, darkKnown = darkFallback(desktopState.xsettings, desktopState.haveSettings, read("GTK_THEME"), desktopState.files)
	}
	if reduce = desktopState.portal.reduceMotion(); reduce == triUnknown {
		reduce = motionFallback(desktopState.xsettings, desktopState.haveSettings, desktopState.files)
	}
	return dark, darkKnown, reduce
}

type resolved struct {
	dark, darkKnown bool
	reduce          tri
	fonts           fontSettings
}

// currentLocked is the resolved settings. desktopState is locked.
func currentLocked(read func(name string) string) resolved {
	var result resolved
	result.dark, result.darkKnown, result.reduce = resolveLocked(read)
	if desktopState.haveFonts {
		result.fonts = desktopState.fonts
	}
	return result
}

func diff(before, after resolved) settingsChange {
	var change settingsChange
	if before.dark != after.dark || before.darkKnown != after.darkKnown {
		change |= changedAppearance
	}
	if before.reduce != after.reduce {
		change |= changedMotion
	}
	if before.fonts != after.fonts {
		change |= changedFonts
	}
	return change
}

// publishSettings records the XSETTINGS and resource database a window's tracker read, and
// says what that changed. The first time only sets the baseline.
func publishSettings(settings xsettings, resources string, read func(string) string) settingsChange {
	desktopState.Lock()
	defer desktopState.Unlock()
	first := !desktopState.haveSettings
	before := currentLocked(read)
	desktopState.xsettings, desktopState.resources, desktopState.haveSettings = settings, resources, true
	desktopState.fonts, desktopState.haveFonts = fontSettingsFrom(settings, resources), true
	fonts := desktopState.fonts
	publishedFonts.Store(&fonts)
	change := diff(before, currentLocked(read))
	if first {
		// Nothing was drawn with other font settings yet: this is where they start.
		change &^= changedFonts
	}
	return change
}

// applyPortal records a value the portal reported and says what that changed.
func applyPortal(namespace, key string, value portalNumber, read func(string) string) settingsChange {
	desktopState.Lock()
	defer desktopState.Unlock()
	before := currentLocked(read)
	if !desktopState.portal.apply(namespace, key, value) {
		return 0
	}
	return diff(before, currentLocked(read))
}

// replacePortal records what the portal said when it was asked and says what that changed.
func replacePortal(values portalValues, read func(string) string) settingsChange {
	desktopState.Lock()
	defer desktopState.Unlock()
	before := currentLocked(read)
	desktopState.portal, desktopState.portalRead, desktopState.portalAt = values, true, time.Now()
	return diff(before, currentLocked(read))
}

// fileURIPath is the local path of a file: URI, as the portal's file chooser returns them.
func fileURIPath(uri string) (string, bool) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" || (parsed.Host != "" && parsed.Host != "localhost") || parsed.Path == "" {
		return "", false
	}
	return parsed.Path, true
}

// splitFilterPatterns is the glob patterns of a file filter, written separated by semicolons.
func splitFilterPatterns(index int, name, pattern string) ([]string, error) {
	if strings.ContainsAny(name, "\x00\n\r") || strings.ContainsAny(pattern, "\x00\n\r") {
		return nil, filterError(index, "contains a control character")
	}
	patterns := strings.FieldsFunc(pattern, func(value rune) bool { return value == ';' })
	if len(patterns) == 0 {
		return nil, filterError(index, "has no patterns")
	}
	for position := range patterns {
		patterns[position] = strings.TrimSpace(patterns[position])
		if patterns[position] == "" {
			return nil, filterError(index, "has an empty pattern")
		}
	}
	return patterns, nil
}

func filterError(index int, what string) error {
	return fmt.Errorf("linux: file filter %d %s", index, what)
}
