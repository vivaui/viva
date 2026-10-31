package linux

import (
	"encoding/binary"
	"testing"
	"time"
)

func settingsOf(t *testing.T, ints map[string]int32, strings map[string]string) xsettings {
	t.Helper()
	parsed, err := parseXSettings(settingsBlob(binary.LittleEndian, ints, strings))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// resetDesktopState gives a test an empty picture of the desktop and puts back what it found.
func resetDesktopState(t *testing.T) {
	t.Helper()
	desktopState.Lock()
	type snapshot struct {
		portal                 portalValues
		portalRead, portalGone bool
		watching, haveSettings bool
		xsettings              xsettings
		resources              string
		fonts                  fontSettings
		haveFonts              bool
		files                  desktopFiles
		portalAt, filesAt      time.Time
	}
	saved := snapshot{desktopState.portal, desktopState.portalRead, desktopState.portalGone, desktopState.watching, desktopState.haveSettings,
		desktopState.xsettings, desktopState.resources, desktopState.fonts, desktopState.haveFonts, desktopState.files, desktopState.portalAt, desktopState.filesAt}
	fonts := publishedFonts.Load()
	desktopState.portal, desktopState.portalRead, desktopState.portalGone, desktopState.watching = portalValues{}, false, false, false
	desktopState.xsettings, desktopState.haveSettings, desktopState.resources = xsettings{}, false, ""
	desktopState.fonts, desktopState.haveFonts, desktopState.files = fontSettings{}, false, desktopFiles{}
	desktopState.portalAt, desktopState.filesAt = time.Time{}, time.Time{}
	publishedFonts.Store(nil)
	desktopState.Unlock()
	t.Cleanup(func() {
		desktopState.Lock()
		defer desktopState.Unlock()
		desktopState.portal, desktopState.portalRead, desktopState.portalGone, desktopState.watching = saved.portal, saved.portalRead, saved.portalGone, saved.watching
		desktopState.xsettings, desktopState.haveSettings, desktopState.resources = saved.xsettings, saved.haveSettings, saved.resources
		desktopState.fonts, desktopState.haveFonts, desktopState.files = saved.fonts, saved.haveFonts, saved.files
		desktopState.portalAt, desktopState.filesAt = saved.portalAt, saved.filesAt
		publishedFonts.Store(fonts)
	})
}

func noEnvironment(string) string { return "" }

func TestThemeNamesSayWhetherTheyAreDark(t *testing.T) {
	for name, want := range map[string]bool{
		"Adwaita": false, "Adwaita-dark": true, "Breeze-Dark": true, "Yaru-dark": true, "Adwaita:dark": true, "Breeze": false, "": false,
	} {
		if got := themeNameIsDark(name); got != want {
			t.Errorf("%q: dark = %v, want %v", name, got, want)
		}
	}
}

func TestXSettingsSayTheLookAndTheMotion(t *testing.T) {
	for _, test := range []struct {
		name        string
		ints        map[string]int32
		strings     map[string]string
		dark, known bool
		motion      tri
	}{
		{"dark theme name", nil, map[string]string{"Net/ThemeName": "Adwaita-dark"}, true, true, triUnknown},
		{"light theme name", nil, map[string]string{"Net/ThemeName": "Breeze"}, false, true, triUnknown},
		{"preference beats the name", map[string]int32{"Gtk/ApplicationPreferDarkTheme": 1}, map[string]string{"Net/ThemeName": "Adwaita"}, true, true, triUnknown},
		{"says nothing", map[string]int32{"Xft/DPI": 98304}, nil, false, false, triUnknown},
		{"animations on", map[string]int32{"Gtk/EnableAnimations": 1}, nil, false, false, triFalse},
		{"animations off", map[string]int32{"Gtk/EnableAnimations": 0}, nil, false, false, triTrue},
	} {
		settings := settingsOf(t, test.ints, test.strings)
		if dark, known := settings.darkTheme(); dark != test.dark || known != test.known {
			t.Errorf("%s: dark %v known %v, want %v %v", test.name, dark, known, test.dark, test.known)
		}
		if got := settings.reducedMotion(); got != test.motion {
			t.Errorf("%s: reduced motion %v, want %v", test.name, got, test.motion)
		}
	}
}

func TestConfigurationFilesAreReadAsTheToolkitsWriteThem(t *testing.T) {
	gtkDark := "[Settings]\ngtk-theme-name=Adwaita\ngtk-application-prefer-dark-theme=1\n"
	gtkNamedDark := "# a comment\n[Settings]\ngtk-theme-name = Materia-dark\n"
	gtkLight := "[Settings]\ngtk-theme-name=Adwaita\n"
	kdeDark := "[General]\nColorScheme=BreezeDark\n\n[KDE]\nAnimationDurationFactor=0\n"
	kdeLight := "[General]\nColorScheme=BreezeLight\n[KDE]\nAnimationDurationFactor=0.5\n"
	for _, test := range []struct {
		name        string
		got         func() (bool, bool)
		dark, known bool
	}{
		{"gtk prefer dark", func() (bool, bool) { return gtkSettingsDark(gtkDark) }, true, true},
		{"gtk dark theme name", func() (bool, bool) { return gtkSettingsDark(gtkNamedDark) }, true, true},
		{"gtk light", func() (bool, bool) { return gtkSettingsDark(gtkLight) }, false, true},
		{"gtk empty", func() (bool, bool) { return gtkSettingsDark("") }, false, false},
		{"gtk key in the wrong section", func() (bool, bool) { return gtkSettingsDark("[Other]\ngtk-application-prefer-dark-theme=1\n") }, false, false},
		{"kde dark", func() (bool, bool) { return kdeColorSchemeDark(kdeDark) }, true, true},
		{"kde light", func() (bool, bool) { return kdeColorSchemeDark(kdeLight) }, false, true},
		{"kde none", func() (bool, bool) { return kdeColorSchemeDark("[General]\n") }, false, false},
	} {
		if dark, known := test.got(); dark != test.dark || known != test.known {
			t.Errorf("%s: dark %v known %v, want %v %v", test.name, dark, known, test.dark, test.known)
		}
	}
	if kdeAnimationsOff(kdeDark) != triTrue || kdeAnimationsOff(kdeLight) != triFalse || kdeAnimationsOff("") != triUnknown {
		t.Errorf("animations: %v %v %v", kdeAnimationsOff(kdeDark), kdeAnimationsOff(kdeLight), kdeAnimationsOff(""))
	}
}

func TestADesktopWithoutAPortalIsAskedFromTheLiveSourceFirst(t *testing.T) {
	live := settingsOf(t, nil, map[string]string{"Net/ThemeName": "Breeze"})
	files := desktopFiles{GTK3: "[Settings]\ngtk-application-prefer-dark-theme=1\n", KDEGlobals: "[General]\nColorScheme=BreezeDark\n"}
	for _, test := range []struct {
		name         string
		settings     xsettings
		have         bool
		gtkTheme     string
		files        desktopFiles
		dark, known  bool
		wantsNothing bool
	}{
		{"xsettings win", live, true, "Adwaita:dark", files, false, true, false},
		{"no manager: GTK_THEME", xsettings{}, false, "Adwaita:dark", desktopFiles{}, true, true, false},
		{"no manager, no env: gtk file", xsettings{}, false, "", files, true, true, false},
		{"only kde", xsettings{}, false, "", desktopFiles{KDEGlobals: files.KDEGlobals}, true, true, false},
		{"nothing", xsettings{}, false, "", desktopFiles{}, false, false, true},
	} {
		dark, known := darkFallback(test.settings, test.have, test.gtkTheme, test.files)
		if dark != test.dark || known != test.known {
			t.Errorf("%s: dark %v known %v, want %v %v", test.name, dark, known, test.dark, test.known)
		}
	}
}

func TestThePortalWinsAndNoPreferenceFallsThrough(t *testing.T) {
	resetDesktopState(t)
	desktopState.Lock()
	desktopState.files = desktopFiles{KDEGlobals: "[General]\nColorScheme=BreezeDark\n[KDE]\nAnimationDurationFactor=0\n"}
	desktopState.Unlock()

	read := func() (dark, known bool, reduce tri) {
		desktopState.Lock()
		defer desktopState.Unlock()
		return resolveLocked(noEnvironment)
	}
	if dark, known, reduce := read(); !dark || !known || reduce != triTrue {
		t.Fatalf("files only: dark %v known %v reduce %v", dark, known, reduce)
	}
	// The portal says light and that animations are on: it outranks the files.
	if change := applyPortal(portalAppearance, portalColorScheme, portalNumber{'u', 2}, noEnvironment); change != changedAppearance {
		t.Fatalf("color scheme change = %v", change)
	}
	if change := applyPortal(portalGnomeUI, portalAnimationsOn, portalNumber{'b', 1}, noEnvironment); change != changedMotion {
		t.Fatalf("animations change = %v", change)
	}
	if dark, known, reduce := read(); dark || !known || reduce != triFalse {
		t.Fatalf("portal: dark %v known %v reduce %v", dark, known, reduce)
	}
	// Saying the same again changes nothing; "no preference" hands the question back to the files.
	if change := applyPortal(portalGnomeUI, portalAnimationsOn, portalNumber{'b', 1}, noEnvironment); change != 0 {
		t.Fatalf("repeated value changed %v", change)
	}
	applyPortal(portalAppearance, portalColorScheme, portalNumber{'u', 0}, noEnvironment)
	if dark, known, _ := read(); !dark || !known {
		t.Fatalf("no preference: dark %v known %v, want the file's dark", dark, known)
	}
	// The portal's own reduced-motion key outranks the GNOME key.
	applyPortal(portalAppearance, portalReducedMo, portalNumber{'u', 1}, noEnvironment)
	if _, _, reduce := read(); reduce != triTrue {
		t.Fatalf("reduced-motion = %v", reduce)
	}
	// A value of another kind, or for another key, is not taken.
	if change := applyPortal(portalAppearance, portalColorScheme, portalNumber{'s', 1}, noEnvironment); change != 0 {
		t.Fatalf("a string changed the color scheme: %v", change)
	}
	if change := applyPortal("org.example", "key", portalNumber{'u', 1}, noEnvironment); change != 0 {
		t.Fatalf("an unknown key changed %v", change)
	}
}

func TestPublishedSettingsReportWhatChangedAfterTheFirst(t *testing.T) {
	resetDesktopState(t)
	first := settingsOf(t, map[string]int32{"Xft/Antialias": 1}, map[string]string{"Net/ThemeName": "Adwaita", "Xft/RGBA": "none"})
	if change := publishSettings(first, "", noEnvironment); change&changedFonts != 0 {
		t.Fatalf("the first publish is the baseline, got %v", change)
	}
	if got := publishedFonts.Load(); got == nil || got.Antialias != cairoAntialiasGray {
		t.Fatalf("published fonts = %+v", got)
	}
	if change := publishSettings(first, "", noEnvironment); change != 0 {
		t.Fatalf("the same settings changed %v", change)
	}
	lcd := settingsOf(t, map[string]int32{"Xft/Antialias": 1}, map[string]string{"Net/ThemeName": "Adwaita", "Xft/RGBA": "rgb"})
	if change := publishSettings(lcd, "", noEnvironment); change != changedFonts {
		t.Fatalf("LCD text changed %v, want only the fonts", change)
	}
	if got := publishedFonts.Load(); got == nil || !got.subpixel() {
		t.Fatalf("published fonts = %+v", got)
	}
	dark := settingsOf(t, map[string]int32{"Xft/Antialias": 1, "Gtk/EnableAnimations": 0}, map[string]string{"Net/ThemeName": "Adwaita-dark", "Xft/RGBA": "rgb"})
	if change := publishSettings(dark, "", noEnvironment); change != changedAppearance|changedMotion {
		t.Fatalf("dark and no animations changed %v", change)
	}
	if platformChange := (changedAppearance | changedMotion | changedFonts).platformChange(); platformChange == 0 {
		t.Fatal("a font or motion change is not reported to applications")
	}
}

func TestFileURIsAreLocalPaths(t *testing.T) {
	for uri, want := range map[string]string{
		"file:///home/u/a%20b.txt":       "/home/u/a b.txt",
		"file://localhost/tmp/x":         "/tmp/x",
		"file:///tmp/%E4%BD%A0%E5%A5%BD": "/tmp/你好",
	} {
		if got, ok := fileURIPath(uri); !ok || got != want {
			t.Errorf("%s = %q, %v; want %q", uri, got, ok, want)
		}
	}
	for _, uri := range []string{"", "https://example.com/x", "file://other-host/x", "file://", "/plain/path"} {
		if got, ok := fileURIPath(uri); ok {
			t.Errorf("%q was taken for the local path %q", uri, got)
		}
	}
}

func TestFilterPatternsAreSplitAndChecked(t *testing.T) {
	patterns, err := splitFilterPatterns(0, "Images", "*.png; *.jpg;*.gif")
	if err != nil || len(patterns) != 3 || patterns[1] != "*.jpg" {
		t.Fatalf("patterns = %q, %v", patterns, err)
	}
	for _, bad := range [][2]string{{"x", ""}, {"x", " ; "}, {"x\ny", "*.a"}, {"x", "*.a\n*.b"}} {
		if _, err := splitFilterPatterns(2, bad[0], bad[1]); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}
