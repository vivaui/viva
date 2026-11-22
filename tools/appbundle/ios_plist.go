package appbundle

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

// IOSInfo is what an iOS application's Info.plist records. An iOS bundle is
// flat: the executable and Info.plist sit side by side in the .app.
type IOSInfo struct {
	// Name is the display name (CFBundleName and CFBundleDisplayName).
	Name string
	// ID is the bundle identifier.
	ID string
	// Version is both the short version and the build version.
	Version string
	// Executable is the file name inside the bundle.
	Executable string
	// MinSystem is MinimumOSVersion.
	MinSystem string
	// Simulator marks a bundle for the simulator rather than for devices.
	Simulator bool
}

// Plist renders Info.plist as XML, keys in a fixed order so the same IOSInfo
// always gives the same bytes. The application fills the screen, in any
// orientation, on iPhone and iPad.
func (info IOSInfo) Plist() ([]byte, error) {
	if info.Name == "" || info.ID == "" || info.Version == "" || info.Executable == "" || info.MinSystem == "" {
		return nil, fmt.Errorf("appbundle: name, identifier, version, executable and minimum system are required")
	}
	platform := "iPhoneOS"
	if info.Simulator {
		platform = "iPhoneSimulator"
	}
	var buffer bytes.Buffer
	buffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buffer.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	buffer.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	text := func(key, value string) {
		buffer.WriteString("\t<key>" + key + "</key>\n\t<string>")
		xml.EscapeText(&buffer, []byte(value))
		buffer.WriteString("</string>\n")
	}
	list := func(key string, values ...string) {
		buffer.WriteString("\t<key>" + key + "</key>\n\t<array>\n")
		for _, value := range values {
			buffer.WriteString("\t\t<string>")
			xml.EscapeText(&buffer, []byte(value))
			buffer.WriteString("</string>\n")
		}
		buffer.WriteString("\t</array>\n")
	}
	text("CFBundleDevelopmentRegion", "en")
	text("CFBundleDisplayName", info.Name)
	text("CFBundleExecutable", info.Executable)
	text("CFBundleIdentifier", info.ID)
	text("CFBundleInfoDictionaryVersion", "6.0")
	text("CFBundleName", info.Name)
	text("CFBundlePackageType", "APPL")
	list("CFBundleSupportedPlatforms", platform)
	text("CFBundleShortVersionString", info.Version)
	text("CFBundleVersion", info.Version)
	text("MinimumOSVersion", info.MinSystem)
	buffer.WriteString("\t<key>UIDeviceFamily</key>\n\t<array>\n\t\t<integer>1</integer>\n\t\t<integer>2</integer>\n\t</array>\n")
	buffer.WriteString("\t<key>UILaunchScreen</key>\n\t<dict/>\n")
	list("UISupportedInterfaceOrientations",
		"UIInterfaceOrientationPortrait", "UIInterfaceOrientationLandscapeLeft", "UIInterfaceOrientationLandscapeRight")
	list("UISupportedInterfaceOrientations~ipad",
		"UIInterfaceOrientationPortrait", "UIInterfaceOrientationPortraitUpsideDown",
		"UIInterfaceOrientationLandscapeLeft", "UIInterfaceOrientationLandscapeRight")
	buffer.WriteString("</dict>\n</plist>\n")
	return buffer.Bytes(), nil
}
