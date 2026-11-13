package platform

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// DownloadsFolder is where a download the application allowed goes when the platform cannot say
// where the user keeps downloads: "Downloads" beside the home folder, where every desktop puts it.
// The backends ask the desktop first (the known folder on Windows, the special directory on
// Linux, the search path on macOS) and use this only as the answer of last resort.
func DownloadsFolder() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "Downloads")
	}
	return os.TempDir()
}

// DownloadFileName returns the name a download is saved under. suggested is what the engine made
// of the response, usually its Content-Disposition name; when that has nothing usable, the last
// element of address is used, and when neither has one the name is "download". The result is a
// name only: no directory, nothing a file system here rejects, and never "." or "..".
func DownloadFileName(suggested, address string) string {
	name := cleanDownloadName(suggested)
	if name == "" {
		name = cleanDownloadName(downloadPath(address))
	}
	if name == "" {
		return "download"
	}
	return name
}

// downloadPath is the last element of address's path, without a scheme, a query or a fragment.
// It does not unescape, and it does not use net/url: that parser, with the inlining data the linker
// keeps for it, costs tens of kilobytes in every application that links app, for a name that is
// only the fallback when the engine suggested nothing. A percent-encoded name is left as written.
func downloadPath(address string) string {
	if scheme := strings.Index(address, "://"); scheme >= 0 {
		address = address[scheme+3:]
	}
	slash := strings.IndexByte(address, '/')
	if slash < 0 {
		return ""
	}
	name := address[slash+1:]
	if cut := strings.IndexAny(name, "?#"); cut >= 0 {
		name = name[:cut]
	}
	if end := strings.LastIndexByte(name, '/'); end >= 0 {
		name = name[end+1:]
	}
	return unescapePath(name)
}

// unescapePath decodes the percent escapes of a path, as net/url does for the parsed path but in
// the dozen lines it takes: a malformed escape is left as written rather than rejected.
func unescapePath(name string) string {
	if !strings.Contains(name, "%") {
		return name
	}
	unescaped := make([]byte, 0, len(name))
	for index := 0; index < len(name); index++ {
		if name[index] != '%' || index+2 >= len(name) {
			unescaped = append(unescaped, name[index])
			continue
		}
		high, low := hexValue(name[index+1]), hexValue(name[index+2])
		if high < 0 || low < 0 {
			unescaped = append(unescaped, name[index])
			continue
		}
		unescaped = append(unescaped, byte(high<<4|low))
		index += 2
	}
	return string(unescaped)
}

// hexValue is the value of one hexadecimal digit, or -1.
func hexValue(digit byte) int {
	switch {
	case digit >= '0' && digit <= '9':
		return int(digit - '0')
	case digit >= 'a' && digit <= 'f':
		return int(digit-'a') + 10
	case digit >= 'A' && digit <= 'F':
		return int(digit-'A') + 10
	}
	return -1
}

// cleanDownloadName turns what an engine suggests into a name this file system takes. It keeps
// the suggestion's extension and words, replaces what Windows rejects (the strictest file system
// of the three) and strips the leading and trailing dots and spaces no desktop wants.
func cleanDownloadName(name string) string {
	name = strings.TrimRight(strings.ReplaceAll(name, `\`, "/"), "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case r < ' ' || r == 0x7f:
			return -1
		case strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return ""
	}
	// Windows reserves these names with any extension and any case.
	stem := name
	if dot := strings.IndexByte(stem, '.'); dot >= 0 {
		stem = stem[:dot]
	}
	switch strings.ToUpper(stem) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return "_" + name
	}
	return name
}

// FreeDownloadPath returns folder joined with name, or with "name (1).ext" and so on: the first
// path no file has. A download never overwrites a file the user already has.
func FreeDownloadPath(folder, name string) string {
	candidate := filepath.Join(folder, name)
	if _, err := os.Lstat(candidate); err != nil {
		return candidate
	}
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for index := 1; ; index++ {
		candidate = filepath.Join(folder, fmt.Sprintf("%s (%d)%s", stem, index, extension))
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
	}
}
