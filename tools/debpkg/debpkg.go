// Package debpkg writes Debian binary packages (.deb) for a desktop
// application: the program in /usr/bin, a menu entry and hicolor icons.
//
// A .deb is an ar archive of debian-binary, control.tar.gz and data.tar.gz.
// The writer uses gzip because it is in the standard library (zstd and xz are
// not); every Debian and Ubuntu release since 2007 installs it. All files are
// owned by root with one fixed time and the entries are sorted, so building the
// same input twice gives identical bytes.
package debpkg

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vivaui/viva/tools/desktopentry"
	"github.com/vivaui/viva/tools/linuxicon"
	"github.com/vivaui/viva/tools/mimeinfo"
	"github.com/vivaui/viva/tools/tarball"
)

// Package describes one application package.
type Package struct {
	// Executable is the program's file name in /usr/bin. The package is named
	// after it (see PackageName).
	Executable string
	// ID is the reverse-DNS application identifier; the menu entry is
	// /usr/share/applications/<ID>.desktop.
	ID string
	// Name is the display name in the menu.
	Name    string
	Version string
	// Arch is amd64 or arm64.
	Arch       string
	Maintainer string
	Homepage   string
	// Comment is the one-line description. A comment that is not one line is
	// an error; a longer text goes in Description.
	Comment string
	// Description is optional extended text, paragraphs separated by blank
	// lines.
	Description string
	Category    string
	Keywords    []string
	// URLSchemes are the link schemes the application opens.
	URLSchemes []string
	// FileTypes are the document types the application opens.
	FileTypes []mimeinfo.Type
	Depends   []string
	// Recommends are packages that improve the application without being
	// required to run it, such as a GPU driver where the program falls back
	// to a CPU renderer.
	Recommends []string
	// Program is the executable's bytes.
	Program []byte
	// Icons are the hicolor sizes, from linuxicon.Render; none for no icon.
	Icons []linuxicon.Rendition
}

var (
	packageNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`)
	versionPattern     = regexp.MustCompile(`^[0-9][A-Za-z0-9.+~-]*$`)
	idPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// PackageName derives the Debian package name from the executable's file name:
// lowercase, underscores turned into hyphens. A name Debian does not allow (at
// least two characters from a-z, 0-9, + - .; starting with a letter or digit)
// is an error.
func PackageName(executable string) (string, error) {
	name := strings.ToLower(strings.ReplaceAll(executable, "_", "-"))
	if !packageNamePattern.MatchString(name) {
		return "", fmt.Errorf("debpkg: %q cannot name a Debian package: it needs two or more of a-z, 0-9, + - . and must start with a letter or digit; set [app] executable", executable)
	}
	return name, nil
}

// FileName is the conventional file name, <package>_<version>_<arch>.deb.
func (pkg Package) FileName() (string, error) {
	name, err := PackageName(pkg.Executable)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s_%s.deb", name, pkg.Version, pkg.Arch), nil
}

func (pkg Package) check() error {
	if _, err := PackageName(pkg.Executable); err != nil {
		return err
	}
	switch {
	case !versionPattern.MatchString(pkg.Version):
		return fmt.Errorf("debpkg: %q is not a Debian version (it must start with a digit)", pkg.Version)
	case pkg.Arch != "amd64" && pkg.Arch != "arm64":
		return fmt.Errorf("debpkg: architecture %q must be amd64 or arm64", pkg.Arch)
	case !idPattern.MatchString(pkg.ID):
		return fmt.Errorf("debpkg: %q cannot name a desktop file", pkg.ID)
	case pkg.Name == "":
		return errors.New("debpkg: the application needs a display name")
	case len(pkg.Program) == 0:
		return errors.New("debpkg: the program is empty")
	case pkg.Maintainer == "" || strings.ContainsAny(pkg.Maintainer, "\r\n"):
		return errors.New("debpkg: the maintainer must be one line")
	case strings.ContainsAny(pkg.Comment, "\r\n"):
		return errors.New("debpkg: the comment must be one line")
	}
	if pkg.Homepage != "" {
		if address, err := url.Parse(pkg.Homepage); err != nil || address.Host == "" || strings.ContainsAny(pkg.Homepage, " \r\n\t") {
			return fmt.Errorf("debpkg: %q is not a web address", pkg.Homepage)
		}
	}
	for _, dependencies := range [][]string{pkg.Depends, pkg.Recommends} {
		for _, dependency := range dependencies {
			if dependency == "" || strings.ContainsAny(dependency, ",\r\n") {
				return fmt.Errorf("debpkg: %q is not one dependency", dependency)
			}
		}
	}
	for _, icon := range pkg.Icons {
		if icon.Pixels <= 0 || len(icon.PNG) == 0 {
			return errors.New("debpkg: an icon is empty")
		}
	}
	return nil
}

// DesktopEntry is the menu entry the package installs.
func (pkg Package) DesktopEntry() ([]byte, error) {
	entry := desktopentry.Entry{
		Name: pkg.Name, Comment: pkg.Comment, Exec: pkg.Executable,
		Categories: []string{pkg.Category}, Keywords: pkg.Keywords, StartupWMClass: pkg.Executable,
		URLSchemes: pkg.URLSchemes, MimeTypes: mimeinfo.Types(pkg.FileTypes),
	}
	if len(pkg.Icons) > 0 {
		entry.Icon = pkg.Executable
	}
	return entry.Bytes()
}

// installedKiB is the Installed-Size field: the files' sizes in KiB, each
// rounded up as dpkg's own tools do per file.
func installedKiB(files []tarball.File) int64 {
	var total int64
	for _, file := range files {
		total += (int64(len(file.Data)) + 1023) / 1024
	}
	return total
}

// Control is the control file.
func (pkg Package) Control(files []tarball.File) ([]byte, error) {
	if err := pkg.check(); err != nil {
		return nil, err
	}
	name, _ := PackageName(pkg.Executable)
	var builder strings.Builder
	field := func(key, value string) { builder.WriteString(key + ": " + value + "\n") }
	field("Package", name)
	field("Version", pkg.Version)
	field("Architecture", pkg.Arch)
	field("Maintainer", pkg.Maintainer)
	field("Installed-Size", fmt.Sprint(installedKiB(files)))
	if len(pkg.Depends) > 0 {
		field("Depends", strings.Join(pkg.Depends, ", "))
	}
	if len(pkg.Recommends) > 0 {
		field("Recommends", strings.Join(pkg.Recommends, ", "))
	}
	field("Section", "utils")
	field("Priority", "optional")
	if pkg.Homepage != "" {
		field("Homepage", pkg.Homepage)
	}
	summary := pkg.Comment
	if summary == "" {
		summary = pkg.Name
	}
	field("Description", summary+foldDescription(pkg.Description))
	return []byte(builder.String()), nil
}

// foldDescription formats the extended description of a control file: every
// line starts with a space and an empty line is a space and a dot.
func foldDescription(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return ""
	}
	var builder strings.Builder
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			line = "."
		}
		builder.WriteString("\n " + line)
	}
	return builder.String()
}

// Files are the files the package installs, with paths relative to /.
func (pkg Package) Files() ([]tarball.File, error) {
	desktop, err := pkg.DesktopEntry()
	if err != nil {
		return nil, err
	}
	files := []tarball.File{
		{Path: "usr/bin/" + pkg.Executable, Data: pkg.Program, Mode: 0o755},
		{Path: "usr/share/applications/" + pkg.ID + ".desktop", Data: desktop, Mode: 0o644},
	}
	if len(pkg.FileTypes) > 0 {
		types, err := mimeinfo.Package(pkg.FileTypes)
		if err != nil {
			return nil, err
		}
		files = append(files, tarball.File{Path: "usr/share/mime/packages/" + pkg.ID + ".xml", Data: types, Mode: 0o644})
	}
	for _, icon := range pkg.Icons {
		files = append(files, tarball.File{
			Path: fmt.Sprintf("usr/share/icons/hicolor/%dx%d/apps/%s.png", icon.Pixels, icon.Pixels, pkg.Executable),
			Data: icon.PNG, Mode: 0o644,
		})
	}
	return files, nil
}

// MD5Sums is the md5sums file: one line per file, sorted by path.
func MD5Sums(files []tarball.File) []byte {
	sorted := append([]tarball.File(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	var builder strings.Builder
	for _, file := range sorted {
		sum := md5.Sum(file.Data)
		builder.WriteString(hex.EncodeToString(sum[:]) + "  " + file.Path + "\n")
	}
	return []byte(builder.String())
}

// Write writes the .deb.
func (pkg Package) Write(output io.Writer) error {
	if err := pkg.check(); err != nil {
		return err
	}
	files, err := pkg.Files()
	if err != nil {
		return err
	}
	control, err := pkg.Control(files)
	if err != nil {
		return err
	}
	modified := tarball.SourceDate()

	var controlTar bytes.Buffer
	if err := tarball.WriteTar(&controlTar, []tarball.File{
		{Path: "control", Data: control, Mode: 0o644},
		{Path: "md5sums", Data: MD5Sums(files), Mode: 0o644},
	}, ".", modified); err != nil {
		return err
	}
	var dataTar bytes.Buffer
	if err := tarball.WriteTar(&dataTar, files, ".", modified); err != nil {
		return err
	}
	controlGz, err := tarball.Gzip(controlTar.Bytes())
	if err != nil {
		return err
	}
	dataGz, err := tarball.Gzip(dataTar.Bytes())
	if err != nil {
		return err
	}
	return writeAr(output, modified, []member{
		{"debian-binary", []byte("2.0\n")},
		{"control.tar.gz", controlGz},
		{"data.tar.gz", dataGz},
	})
}

type member struct {
	name string
	data []byte
}

// writeAr writes the common ar format: the "!<arch>\n" magic, then per member a
// 60-byte header (name 16, mtime 12, uid 6, gid 6, mode 8, size 10, "`\n"), the
// data and a newline when the data has an odd length.
func writeAr(output io.Writer, modified time.Time, members []member) error {
	if _, err := io.WriteString(output, "!<arch>\n"); err != nil {
		return err
	}
	for _, item := range members {
		if len(item.name) > 15 {
			return fmt.Errorf("debpkg: member name %q is too long", item.name)
		}
		header := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8s%-10d`\n", item.name, modified.Unix(), 0, 0, "100644", len(item.data))
		if _, err := io.WriteString(output, header); err != nil {
			return err
		}
		if _, err := output.Write(item.data); err != nil {
			return err
		}
		if len(item.data)%2 == 1 {
			if _, err := io.WriteString(output, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
