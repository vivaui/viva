package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/vivaui/viva/tools/appbundle"
	"github.com/vivaui/viva/tools/manifest"
	"github.com/vivaui/viva/tools/mimeinfo"
	"github.com/vivaui/viva/tools/winpkg"
)

func macFileTypes(kinds []manifest.FileType) []appbundle.FileType {
	result := make([]appbundle.FileType, len(kinds))
	for index, kind := range kinds {
		result[index] = appbundle.FileType{Name: kind.Name, Extensions: kind.Extensions, UTI: kind.UTI}
	}
	return result
}

func linuxFileTypes(kinds []manifest.FileType) []mimeinfo.Type {
	result := make([]mimeinfo.Type, len(kinds))
	for index, kind := range kinds {
		result[index] = mimeinfo.Type{MIME: kind.MIME, Name: kind.Name, Extensions: kind.Extensions}
	}
	return result
}

func windowsFileTypes(kinds []manifest.FileType) []winpkg.FileType {
	result := make([]winpkg.FileType, len(kinds))
	for index, kind := range kinds {
		result[index] = winpkg.FileType{Name: kind.Name, Extensions: kind.Extensions}
	}
	return result
}

// reportOpening says which documents and links the package registers, and for
// Windows and Linux how they reach the program: macOS hands them to the
// running program, the other systems start a program with them as arguments.
func reportOpening(output io.Writer, app manifest.App, startsProgram bool) {
	if len(app.FileTypes) == 0 && len(app.URLSchemes) == 0 {
		return
	}
	var parts []string
	for _, kind := range app.FileTypes {
		parts = append(parts, "."+strings.Join(kind.Extensions, " ."))
	}
	for _, scheme := range app.URLSchemes {
		parts = append(parts, scheme+"://")
	}
	fmt.Fprintf(output, "  opens    %s\n", strings.Join(parts, " "))
	if startsProgram {
		fmt.Fprintln(output, "           each opening starts a new process with the path or link as its argument; import github.com/vivaui/viva/app/singleinstance to hand it to the running copy")
	}
}
