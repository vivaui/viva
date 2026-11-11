package winpkg

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// ExtraFile is a file a package ships beside the program: a native runtime, a
// sidecar helper or a licence. Source is the file on disk; Destination is where
// it goes under the installation directory (and inside the zip), using forward
// slashes. The same list feeds the installer and the archive.
type ExtraFile struct {
	Source      string
	Destination string
}

// extraSegment allows an ordinary path segment: letters, digits, and internal
// spaces, dots, underscores and dashes. It cannot begin or end in a space or a
// dot, so a name can never collapse into ".", ".." or a Windows-trimmed name.
var extraSegment = `[A-Za-z0-9](?:[A-Za-z0-9._ -]*[A-Za-z0-9_-])?`

var extraDestination = regexp.MustCompile(`^` + extraSegment + `(?:/` + extraSegment + `)*$`)

// CheckExtraFiles validates a package's extra files and checks that every
// source exists as an ordinary file.
func CheckExtraFiles(files []ExtraFile) error {
	if _, err := checkedExtraFiles(files); err != nil {
		return err
	}
	for _, file := range files {
		info, err := os.Stat(file.Source)
		if err != nil {
			return fmt.Errorf("extra file %s: %w", file.Destination, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("extra file %s: %s is not a non-empty file", file.Destination, file.Source)
		}
	}
	return nil
}

// checkedExtraFiles validates the list and returns it sorted by destination, so
// the installer script and the zip do not depend on the order the caller gave.
func checkedExtraFiles(files []ExtraFile) ([]ExtraFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	checked := append([]ExtraFile(nil), files...)
	sort.Slice(checked, func(a, b int) bool { return checked[a].Destination < checked[b].Destination })
	seen := make(map[string]bool, len(checked))
	for _, file := range checked {
		if file.Source == "" {
			return nil, fmt.Errorf("extra file %s has no source", file.Destination)
		}
		if strings.IndexFunc(file.Source, func(r rune) bool { return r < ' ' || r == 0x7f }) >= 0 {
			return nil, fmt.Errorf("extra file source %q cannot contain a newline or other control character", file.Source)
		}
		if !extraDestination.MatchString(file.Destination) {
			return nil, fmt.Errorf("extra file destination %q must be a relative path of letters, digits, spaces, dots, underscores and dashes", file.Destination)
		}
		if seen[file.Destination] {
			return nil, fmt.Errorf("extra file destination %q is given twice", file.Destination)
		}
		seen[file.Destination] = true
	}
	return checked, nil
}

// extraDirs returns the parent directories of the destinations, deepest first,
// so an uninstaller can remove them once their files are gone.
func extraDirs(files []ExtraFile) []string {
	seen := make(map[string]bool)
	var directories []string
	for _, file := range files {
		for directory := path.Dir(file.Destination); directory != "."; directory = path.Dir(directory) {
			if !seen[directory] {
				seen[directory] = true
				directories = append(directories, directory)
			}
		}
	}
	sort.Slice(directories, func(a, b int) bool {
		aDepth, bDepth := strings.Count(directories[a], "/"), strings.Count(directories[b], "/")
		if aDepth != bDepth {
			return aDepth > bDepth
		}
		return directories[a] > directories[b]
	})
	return directories
}
