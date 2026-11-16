package platform

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrNoBrowser is what a backend returns when the system names no default web
// browser to open a file in.
var ErrNoBrowser = errors.New("platform: no default web browser")

// FileURL is the file URL a browser is given for an absolute local path. A
// Windows path (windows true) has its separators turned and its drive written
// as file:///C:/..., and a UNC share as file://///host/share/..., the form
// browsers on Windows resolve back to the share; the \\?\ prefix of a long
// path is dropped first. Every space, quote and percent sign comes out escaped,
// so the URL is one word on any command line.
func FileURL(path string, windows bool) string {
	if windows {
		switch {
		case strings.HasPrefix(path, `\\?\UNC\`):
			path = `\\` + path[len(`\\?\UNC\`):]
		case strings.HasPrefix(path, `\\?\`):
			path = path[len(`\\?\`):]
		}
		// C:/dir becomes /C:/dir and //host/share ///host/share, which a URL
		// with no host writes as file:///C:/dir and file://///host/share.
		path = "/" + strings.ReplaceAll(path, `\`, "/")
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// BrowserCommandLine turns the command Windows registers for a protocol, such
// as `"C:\Program Files\Browser\browser.exe" -url "%1"`, into the program to
// start and the command line to start it with for target, the way the shell
// fills it in: %1 and %L become target and %* nothing, and a command with
// neither %1 nor %L gets target appended. A program path the registry left
// unquoted though it has spaces is taken up to the first space at which exists
// reports a file. target goes in as it is, so it must be one word, as a FileURL
// is.
func BrowserCommandLine(command, target string, exists func(string) bool) (program, commandLine string, err error) {
	if target == "" || strings.ContainsAny(target, " \t\"\x00") {
		return "", "", fmt.Errorf("platform: browser target %q is not one word", target)
	}
	command = strings.TrimSpace(command)
	var rest string
	if strings.HasPrefix(command, `"`) {
		end := strings.IndexByte(command[1:], '"')
		if end < 0 {
			return "", "", fmt.Errorf("platform: browser command %q has an unterminated quote", command)
		}
		program, rest = command[1:1+end], command[2+end:]
	} else {
		program, rest = command, ""
		if space := strings.IndexByte(command, ' '); space >= 0 {
			program, rest = command[:space], command[space:]
		}
		for index := range command {
			if command[index] == ' ' && exists(command[:index]) {
				program, rest = command[:index], command[index:]
				break
			}
		}
	}
	if program == "" {
		return "", "", fmt.Errorf("platform: browser command %q names no program", command)
	}
	var line strings.Builder
	line.WriteString(`"` + program + `"`)
	placed := false
	for index := 0; index < len(rest); index++ {
		if rest[index] == '%' && index+1 < len(rest) {
			switch rest[index+1] {
			case '1', 'L', 'l':
				line.WriteString(target)
				placed = true
				index++
				continue
			case '*':
				index++
				continue
			}
		}
		line.WriteByte(rest[index])
	}
	if !placed {
		line.WriteString(" " + target)
	}
	return program, line.String(), nil
}

// DesktopEntry is what a freedesktop.org desktop entry file says about starting
// its application: the keys of its [Desktop Entry] group that a launcher needs.
type DesktopEntry struct {
	Path     string // the file the entry was read from, for %k
	Name     string
	Icon     string
	Exec     string
	Hidden   bool // the entry was deleted
	Terminal bool // the program runs in a terminal
}

// ParseDesktopEntry reads the [Desktop Entry] group of a desktop entry file,
// undoing the string escapes of the values it keeps; localized keys and other
// groups are ignored.
func ParseDesktopEntry(path string, data []byte) DesktopEntry {
	entry := DesktopEntry{Path: path}
	inGroup := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			inGroup = line == "[Desktop Entry]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !inGroup || !ok {
			continue
		}
		value = desktopString(strings.TrimSpace(value))
		switch strings.TrimSpace(key) {
		case "Name":
			entry.Name = value
		case "Icon":
			entry.Icon = value
		case "Exec":
			entry.Exec = value
		case "Hidden":
			entry.Hidden = value == "true"
		case "Terminal":
			entry.Terminal = value == "true"
		}
	}
	return entry
}

// desktopString undoes the escapes of a desktop entry string value.
func desktopString(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var out strings.Builder
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char == '\\' && index+1 < len(value) {
			index++
			switch value[index] {
			case 's':
				char = ' '
			case 'n':
				char = '\n'
			case 't':
				char = '\t'
			case 'r':
				char = '\r'
			case '\\':
				char = '\\'
			default:
				index--
			}
		}
		out.WriteByte(char)
	}
	return out.String()
}

// Arguments is the program and arguments the entry's Exec starts to open one
// file, by the Desktop Entry Specification's quoting rules and field codes: %u
// and %U become uri, %f and %F path, %c the name, %k the entry's own file, %i
// --icon and the icon when there is one, and %% a percent sign; the deprecated
// codes are dropped. An Exec with no file code gets uri appended, since a
// browser that leaves the code out still takes a URL.
func (entry DesktopEntry) Arguments(uri, path string) ([]string, error) {
	words, err := desktopExecWords(entry.Exec)
	if err != nil {
		return nil, err
	}
	var arguments []string
	placed := false
	for _, word := range words {
		switch word {
		case "%u", "%U":
			arguments, placed = append(arguments, uri), true
			continue
		case "%f", "%F":
			arguments, placed = append(arguments, path), true
			continue
		case "%i":
			if entry.Icon != "" {
				arguments = append(arguments, "--icon", entry.Icon)
			}
			continue
		}
		var expanded strings.Builder
		for index := 0; index < len(word); index++ {
			if word[index] != '%' || index+1 == len(word) {
				expanded.WriteByte(word[index])
				continue
			}
			index++
			switch word[index] {
			case '%':
				expanded.WriteByte('%')
			case 'u', 'U':
				expanded.WriteString(uri)
				placed = true
			case 'f', 'F':
				expanded.WriteString(path)
				placed = true
			case 'c':
				expanded.WriteString(entry.Name)
			case 'k':
				expanded.WriteString(entry.Path)
			case 'i':
				expanded.WriteString(entry.Icon)
			case 'd', 'D', 'n', 'N', 'v', 'm':
			default:
				expanded.WriteByte('%')
				expanded.WriteByte(word[index])
			}
		}
		if expanded.Len() == 0 && word != "" {
			continue
		}
		arguments = append(arguments, expanded.String())
	}
	if len(arguments) == 0 {
		return nil, fmt.Errorf("platform: desktop entry %s has no program to run", entry.Path)
	}
	if !placed {
		arguments = append(arguments, uri)
	}
	return arguments, nil
}

// desktopExecWords splits an Exec value into its arguments: spaces separate
// them, and inside double quotes a backslash keeps the next quote, backquote,
// dollar sign or backslash.
func desktopExecWords(value string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord, quoted := false, false
	for index := 0; index < len(value); index++ {
		char := value[index]
		switch {
		case quoted && char == '\\' && index+1 < len(value) && strings.IndexByte("\"`$\\", value[index+1]) >= 0:
			word.WriteByte(value[index+1])
			index++
		case quoted && char == '"':
			quoted = false
		case quoted:
			word.WriteByte(char)
		case char == '"':
			quoted, inWord = true, true
		case char == ' ' || char == '\t':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteByte(char)
			inWord = true
		}
	}
	if quoted {
		return nil, fmt.Errorf("platform: desktop entry Exec %q has an unterminated quote", value)
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// DesktopEntryCandidates lists, in the order a launcher looks, the files a
// desktop file ID such as "org.example.Browser.desktop" can be: under
// applications in the user's data directory and then in each system one, as
// the ID itself and then with its dashes taken for subdirectories. dataHome and
// dataDirs are $XDG_DATA_HOME and $XDG_DATA_DIRS, with their defaults when
// unset; an ID that is not a plain .desktop file name has no candidates.
func DesktopEntryCandidates(id, dataHome, dataDirs, home string) []string {
	if !strings.HasSuffix(id, ".desktop") || strings.ContainsAny(id, "/\x00") || strings.HasPrefix(id, ".") {
		return nil
	}
	var roots []string
	if strings.HasPrefix(dataHome, "/") {
		roots = append(roots, dataHome)
	} else if strings.HasPrefix(home, "/") {
		roots = append(roots, strings.TrimSuffix(home, "/")+"/.local/share")
	}
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	for _, root := range strings.Split(dataDirs, ":") {
		if strings.HasPrefix(root, "/") {
			roots = append(roots, root)
		}
	}
	names := []string{id}
	if nested := strings.ReplaceAll(id, "-", "/"); nested != id {
		names = append(names, nested)
	}
	var candidates []string
	for _, root := range roots {
		for _, name := range names {
			candidates = append(candidates, strings.TrimSuffix(root, "/")+"/applications/"+name)
		}
	}
	return candidates
}
