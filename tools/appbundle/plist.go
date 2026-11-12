// Package appbundle assembles a macOS application bundle (.app) around an
// already built program: Info.plist, the icon, and the folder layout. It signs
// nothing; an unsigned bundle runs on the machine that built it and is blocked
// by Gatekeeper elsewhere until it is signed and notarized.
package appbundle

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Read only the top-level executable, not similarly named nested dictionary keys.
func plistExecutable(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	var key string
	var executable string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("appbundle: invalid Info.plist: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			if len(stack) == 2 && stack[0] == "plist" && stack[1] == "dict" {
				if element.Name.Local == "key" {
					if err := decoder.DecodeElement(&key, &element); err != nil {
						return "", err
					}
					continue
				}
				if key == "CFBundleExecutable" {
					if executable != "" {
						return "", fmt.Errorf("appbundle: duplicate CFBundleExecutable")
					}
					if element.Name.Local != "string" {
						return "", fmt.Errorf("appbundle: CFBundleExecutable must be a string")
					}
					var name string
					if err := decoder.DecodeElement(&name, &element); err != nil {
						return "", err
					}
					if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
						return "", fmt.Errorf("appbundle: invalid CFBundleExecutable %q", name)
					}
					executable = name
					key = ""
					continue
				}
				key = ""
			}
			stack = append(stack, element.Name.Local)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if executable != "" {
		return executable, nil
	}
	return "", fmt.Errorf("appbundle: Info.plist has no CFBundleExecutable")
}

// KeyValue is one extra Info.plist string entry.
type KeyValue struct {
	Key, Value string
}

// Info is what Info.plist records.
type Info struct {
	// Name is the display name (CFBundleName and CFBundleDisplayName).
	Name string
	// ID is the bundle identifier.
	ID string
	// Version is both the short version and the build version.
	Version string
	// Executable is the file name inside Contents/MacOS.
	Executable string
	// MinSystem is LSMinimumSystemVersion.
	MinSystem string
	Category  string
	Copyright string
	// IconFile is the file name inside Contents/Resources; empty for no icon.
	IconFile string
	// URLSchemes are the link schemes the application opens (CFBundleURLTypes).
	URLSchemes []string
	// FileTypes are the document types the application opens
	// (CFBundleDocumentTypes).
	FileTypes []FileType
	// Extra entries are strings added after the ones above.
	Extra []KeyValue
}

// FileType is one kind of document: a description, its extensions without the
// dot, and optionally the Uniform Type Identifier macOS uses for it.
type FileType struct {
	Name       string
	Extensions []string
	UTI        string
}

// managedKeys are the keys Plist writes itself; Extra may not set them, so a
// project file cannot quietly contradict its own [app] and [mac] tables.
var managedKeys = map[string]bool{
	"CFBundleDevelopmentRegion": true, "CFBundleDocumentTypes": true, "CFBundleDisplayName": true, "CFBundleExecutable": true,
	"CFBundleIconFile": true, "CFBundleIdentifier": true, "CFBundleInfoDictionaryVersion": true,
	"CFBundleName": true, "CFBundlePackageType": true, "CFBundleURLTypes": true, "CFBundleShortVersionString": true,
	"CFBundleVersion": true, "LSApplicationCategoryType": true, "LSMinimumSystemVersion": true,
	"NSHighResolutionCapable": true, "NSHumanReadableCopyright": true, "NSPrincipalClass": true,
}

// Plist renders Info.plist as XML. Keys are written in a fixed order, extras
// sorted by key, so the same Info always produces the same bytes.
func (info Info) Plist() ([]byte, error) {
	if info.Name == "" || info.ID == "" || info.Version == "" || info.Executable == "" {
		return nil, fmt.Errorf("appbundle: name, identifier, version and executable are required")
	}
	seen := map[string]bool{}
	for _, extra := range info.Extra {
		switch {
		case managedKeys[extra.Key]:
			return nil, fmt.Errorf("appbundle: Info.plist key %s is set from [app] and [mac]; remove it from [mac.info]", extra.Key)
		case seen[extra.Key]:
			return nil, fmt.Errorf("appbundle: Info.plist key %s is given twice", extra.Key)
		}
		seen[extra.Key] = true
	}

	var buffer bytes.Buffer
	buffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buffer.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	buffer.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	text := func(key, value string) {
		if value == "" {
			return
		}
		buffer.WriteString("\t<key>")
		xml.EscapeText(&buffer, []byte(key))
		buffer.WriteString("</key>\n\t<string>")
		xml.EscapeText(&buffer, []byte(value))
		buffer.WriteString("</string>\n")
	}
	flag := func(key string, value bool) {
		buffer.WriteString("\t<key>" + key + "</key>\n")
		if value {
			buffer.WriteString("\t<true/>\n")
		} else {
			buffer.WriteString("\t<false/>\n")
		}
	}
	text("CFBundleDevelopmentRegion", "en")
	text("CFBundleDisplayName", info.Name)
	text("CFBundleExecutable", info.Executable)
	text("CFBundleIconFile", info.IconFile)
	text("CFBundleIdentifier", info.ID)
	text("CFBundleInfoDictionaryVersion", "6.0")
	text("CFBundleName", info.Name)
	text("CFBundlePackageType", "APPL")
	text("CFBundleShortVersionString", info.Version)
	text("CFBundleVersion", info.Version)
	text("LSApplicationCategoryType", info.Category)
	text("LSMinimumSystemVersion", info.MinSystem)
	flag("NSHighResolutionCapable", true)
	text("NSHumanReadableCopyright", info.Copyright)
	text("NSPrincipalClass", "NSApplication")
	if len(info.URLSchemes) > 0 {
		buffer.WriteString("\t<key>CFBundleURLTypes</key>\n\t<array>\n\t\t<dict>\n")
		buffer.WriteString("\t\t\t<key>CFBundleURLName</key>\n\t\t\t<string>")
		xml.EscapeText(&buffer, []byte(info.ID))
		buffer.WriteString("</string>\n\t\t\t<key>CFBundleURLSchemes</key>\n\t\t\t<array>\n")
		for _, scheme := range info.URLSchemes {
			buffer.WriteString("\t\t\t\t<string>")
			xml.EscapeText(&buffer, []byte(scheme))
			buffer.WriteString("</string>\n")
		}
		buffer.WriteString("\t\t\t</array>\n\t\t</dict>\n\t</array>\n")
	}
	if len(info.FileTypes) > 0 {
		escape := func(text string) string {
			var escaped bytes.Buffer
			xml.EscapeText(&escaped, []byte(text))
			return escaped.String()
		}
		buffer.WriteString("\t<key>CFBundleDocumentTypes</key>\n\t<array>\n")
		for _, kind := range info.FileTypes {
			buffer.WriteString("\t\t<dict>\n\t\t\t<key>CFBundleTypeName</key>\n\t\t\t<string>" + escape(kind.Name) + "</string>\n")
			buffer.WriteString("\t\t\t<key>CFBundleTypeExtensions</key>\n\t\t\t<array>\n")
			for _, extension := range kind.Extensions {
				buffer.WriteString("\t\t\t\t<string>" + escape(extension) + "</string>\n")
			}
			buffer.WriteString("\t\t\t</array>\n\t\t\t<key>CFBundleTypeRole</key>\n\t\t\t<string>Editor</string>\n")
			if kind.UTI != "" {
				buffer.WriteString("\t\t\t<key>LSItemContentTypes</key>\n\t\t\t<array>\n")
				buffer.WriteString("\t\t\t\t<string>" + escape(kind.UTI) + "</string>\n")
				buffer.WriteString("\t\t\t</array>\n")
			}
			buffer.WriteString("\t\t\t<key>LSHandlerRank</key>\n\t\t\t<string>Alternate</string>\n\t\t</dict>\n")
		}
		buffer.WriteString("\t</array>\n")
	}
	extras := append([]KeyValue(nil), info.Extra...)
	sort.Slice(extras, func(i, j int) bool { return extras[i].Key < extras[j].Key })
	for _, extra := range extras {
		text(extra.Key, extra.Value)
	}
	buffer.WriteString("</dict>\n</plist>\n")
	return buffer.Bytes(), nil
}
