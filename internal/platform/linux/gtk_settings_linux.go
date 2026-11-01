//go:build linux && vivagtk

package linux

import (
	"os"
)

// The desktop's settings as GTK reads them: the font rendering, the double click and the caret
// blink, the interface font and the look. GTK takes them from the XSETTINGS manager on X11 and from
// the settings portal or GSettings on Wayland, so one reading serves both, in the shape the XCB
// backend gets them from XSETTINGS.

func (a *gtkApplication) property(settings uintptr, name string, kind uintptr) (value gValue, ok bool) {
	text := cString(name)
	a.g.valueInit(&value, kind)
	a.g.objectGetProperty(settings, &text[0], &value)
	return value, value.gtype != 0
}

func (a *gtkApplication) intProperty(settings uintptr, name string) int32 {
	value, _ := a.property(settings, name, gTypeInt)
	defer a.g.valueUnset(&value)
	return a.g.valueInt(&value)
}

func (a *gtkApplication) boolProperty(settings uintptr, name string) int32 {
	value, _ := a.property(settings, name, gTypeBoolean)
	defer a.g.valueUnset(&value)
	return a.g.valueBool(&value)
}

func (a *gtkApplication) stringProperty(settings uintptr, name string) string {
	value, _ := a.property(settings, name, gTypeString)
	defer a.g.valueUnset(&value)
	return goString(a.g.valueString(&value))
}

// readSettings reads GTK's settings object as the desktop has it now.
func (a *gtkApplication) readSettings() xsettings {
	g := a.g
	object := g.settingsDefault()
	var s xsettings
	if object == 0 {
		return s
	}
	if v := a.intProperty(object, "gtk-xft-antialias"); v >= 0 {
		s.XftAntialias, s.hasAntialias = v, true
	}
	if v := a.intProperty(object, "gtk-xft-hinting"); v >= 0 {
		s.XftHinting, s.hasHinting = v, true
	}
	s.XftHintStyle = a.stringProperty(object, "gtk-xft-hintstyle")
	s.XftRGBA = a.stringProperty(object, "gtk-xft-rgba")
	if v := a.intProperty(object, "gtk-xft-dpi"); v > 0 {
		s.XftDPI, s.hasDPI = v, true
	}
	if v := a.intProperty(object, "gtk-cursor-theme-size"); v > 0 {
		s.CursorSize, s.hasCursorSize = v, true
	}
	if v := a.intProperty(object, "gtk-double-click-time"); v > 0 {
		s.DoubleClickTime, s.hasDoubleClick = v, true
	}
	if v := a.intProperty(object, "gtk-double-click-distance"); v > 0 {
		s.DoubleClickDistance, s.hasDoubleClickDist = v, true
	}
	s.CursorBlink, s.hasCursorBlink = a.boolProperty(object, "gtk-cursor-blink"), true
	if v := a.intProperty(object, "gtk-cursor-blink-time"); v > 0 {
		s.CursorBlinkTime, s.hasCursorBlinkTime = v, true
	}
	s.GtkFontName = a.stringProperty(object, "gtk-font-name")
	s.ThemeName = a.stringProperty(object, "gtk-theme-name")
	s.PreferDark, s.hasPreferDark = a.boolProperty(object, "gtk-application-prefer-dark-theme"), true
	s.EnableAnimations, s.hasEnableAnimated = a.boolProperty(object, "gtk-enable-animations"), true
	return s
}

// watchSettings publishes the settings as they are and again whenever one changes.
func (a *gtkApplication) watchSettings() {
	a.settings = a.readSettings()
	publishSettings(a.settings, "", os.Getenv)
	if object := a.g.settingsDefault(); object != 0 {
		a.g.connect(object, "notify", callbacks().settings, 0)
	}
}

// settingsChanged is GTK saying a setting changed: the font rendering may have, the look, the scale.
// Settings change in bursts, one notification each; the handler runs once for a burst.
func (a *gtkApplication) settingsChanged() {
	if a.settingsPending {
		return
	}
	a.settingsPending = true
	a.enqueue(func() {
		a.settingsPending = false
		next := a.readSettings()
		if next == a.settings {
			return
		}
		a.settings = next
		a.announce(publishSettings(next, "", os.Getenv))
		for _, w := range a.windows {
			w.rescale()
		}
	})
}
