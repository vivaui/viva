package platform

import (
	"encoding/binary"
	"errors"
	"net/url"
	"strings"
	"unicode/utf16"
)

// ErrNoClipboardFiles reports that the clipboard holds no files; app re-exports it.
var ErrNoClipboardFiles = errors.New("clipboard holds no files")

// MaxClipboardFiles is the most paths a clipboard read returns. A file manager
// puts the selection on the clipboard as it stands, so the cap bounds what a
// huge selection or a hostile owner can make a reader hold, not what a person
// copies. A longer list is cut at the cap.
const MaxClipboardFiles = 4096

// FilesFromURIList returns the local paths of a text/uri-list (RFC 2483), the
// form Linux file managers put copied files on the clipboard in. It also takes
// the x-special/gnome-copied-files form Nautilus, Nemo, Caja and Thunar use,
// which is the same list led by a line naming the action ("copy" or "cut").
// Comments, blank lines and anything that is not a file: URL of this computer
// are skipped, percent-escapes are decoded, and the list ends at
// MaxClipboardFiles paths.
func FilesFromURIList(data []byte) []string {
	var paths []string
	first := true
	for _, line := range strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		leading := first
		first = false
		if line == "" || line[0] == '#' {
			continue
		}
		if leading && (line == "copy" || line == "cut") {
			continue
		}
		if path, ok := pathFromFileURL(line); ok {
			paths = append(paths, path)
			if len(paths) == MaxClipboardFiles {
				break
			}
		}
	}
	return paths
}

// pathFromFileURL is the absolute local path a file: URL names. A URL with a
// host other than this computer's own (empty or localhost) is a file on
// another one, which a copy of it cannot read.
func pathFromFileURL(value string) (string, bool) {
	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Scheme, "file") {
		return "", false
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		return "", false
	}
	path := parsed.Path
	if path == "" || path[0] != '/' || strings.IndexByte(path, 0) >= 0 {
		return "", false
	}
	return path, true
}

// FilesFromDropFiles returns the paths of a Windows CF_HDROP block: a
// DROPFILES header (the offset of the list, a point, two flags) followed by
// the paths, each ended by a NUL and the list by one more. Only the wide
// (UTF-16) form is read; the ANSI form depends on a code page this package
// does not know and yields no paths, as does a block whose header or list is
// malformed. The list ends at MaxClipboardFiles paths.
func FilesFromDropFiles(data []byte) []string {
	const header = 20 // pFiles uint32, pt (two int32), fNC uint32, fWide uint32
	if len(data) < header {
		return nil
	}
	offset := int(uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24)
	wide := data[16]|data[17]|data[18]|data[19] != 0
	if !wide || offset < header || offset > len(data) || (len(data)-offset)%2 != 0 {
		return nil
	}
	units := make([]uint16, (len(data)-offset)/2)
	for index := range units {
		units[index] = uint16(data[offset+2*index]) | uint16(data[offset+2*index+1])<<8
	}
	var paths []string
	for start := 0; start < len(units) && units[start] != 0; {
		end := start
		for end < len(units) && units[end] != 0 {
			end++
		}
		if end == len(units) {
			break // an unterminated last path is not a path
		}
		paths = append(paths, utf16ToString(units[start:end]))
		if len(paths) == MaxClipboardFiles {
			break
		}
		start = end + 1
	}
	return paths
}

// DropFilesBytes builds the CF_HDROP block Windows puts on the clipboard for a
// copied selection: a DROPFILES header whose list offset is the header's own
// size, then each path as UTF-16LE, ended by a NUL, the list by one more.
func DropFilesBytes(paths []string) []byte {
	const header = 20
	data := make([]byte, header)
	binary.LittleEndian.PutUint32(data[0:4], header)
	binary.LittleEndian.PutUint32(data[16:20], 1) // fWide
	for _, path := range paths {
		for _, unit := range utf16.Encode([]rune(path)) {
			data = binary.LittleEndian.AppendUint16(data, unit)
		}
		data = binary.LittleEndian.AppendUint16(data, 0)
	}
	data = binary.LittleEndian.AppendUint16(data, 0)
	return data
}

// FileURIList builds the text/uri-list form Linux file managers put on the
// clipboard for a copied selection. Each absolute local path becomes a file:
// URL on its own CRLF-terminated line.
func FileURIList(paths []string) []byte {
	var list strings.Builder
	for _, path := range paths {
		list.WriteString((&url.URL{Scheme: "file", Path: path}).String())
		list.WriteString("\r\n")
	}
	return []byte(list.String())
}

func utf16ToString(units []uint16) string { return string(utf16.Decode(units)) }
