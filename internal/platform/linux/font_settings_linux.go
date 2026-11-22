//go:build !android

package linux

import "sync"

var initialFontSettings struct {
	once     sync.Once
	settings fontSettings
}

// systemFonts is the desktop's font rendering. Once a window has published what its
// settings tracker reads (publishSettings) it follows the desktop as the user changes it;
// until then, and in a process that never opens a window, it is read once from the X
// server the process is connected to, nothing when there is none.
func systemFonts() fontSettings {
	if published := publishedFonts.Load(); published != nil {
		return *published
	}
	initialFontSettings.once.Do(func() {
		initialFontSettings.settings = readFontSettings()
	})
	return initialFontSettings.settings
}

func readFontSettings() fontSettings {
	api, connection, screen, err := openSystemConnection()
	if err != nil {
		return fontSettings{}
	}
	defer func() {
		api.disconnect(connection)
	}()
	tracker := &scaleTracker{api: api, connection: connection, screen: screen}
	tracker.managerAtom, _ = api.atom(connection, "_XSETTINGS_S0")
	tracker.settingsAtom, _ = api.atom(connection, "_XSETTINGS_SETTINGS")
	tracker.resourceAtom, _ = api.atom(connection, "RESOURCE_MANAGER")
	tracker.reload()
	return fontSettingsFrom(tracker.settings, tracker.resource)
}
