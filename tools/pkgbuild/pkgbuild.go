// Package pkgbuild writes Arch Linux packages for a desktop application: a
// PKGBUILD and its .SRCINFO that install a released binary archive.
//
// The archive is the tar.gz `viva package --format tar.gz` makes: the program,
// a menu entry, an icon and document types. The package is a binary one (its
// name ends in -bin), so makepkg does not build anything; it extracts the
// archive into /opt and links the program into /usr/bin, as an AUR -bin package
// does. Nothing here runs makepkg, and nothing needs Arch to write the files.
package pkgbuild

import (
	"fmt"
	"regexp"
	"strings"
)

// Package describes one binary package.
type Package struct {
	// Executable is the program's file name in the archive. The package is
	// named after it (see PackageName).
	Executable string
	// ID is the reverse-DNS application identifier; the menu entry is
	// /usr/share/applications/<ID>.desktop.
	ID string
	// Name is the display name, used when no Comment is set.
	Name    string
	Version string
	// Arch is amd64 or arm64.
	Arch       string
	Maintainer string
	Homepage   string
	// Comment is the one-line description of the package.
	Comment string
	// Depends are Arch package names, such as vulkan-icd-loader.
	Depends []string
	// Archive is the tar.gz's file name, as it is in the PKGBUILD's
	// directory or at the end of SourceURL.
	Archive string
	// SHA256 is the archive's lowercase hexadecimal SHA-256 digest.
	SHA256 string
	// SourceURL, when set, is where makepkg downloads the archive from. Empty
	// means the archive is beside the PKGBUILD.
	SourceURL string
	// IconPixels is the icon's square size in pixels, or 0 without an icon.
	IconPixels int
}

var (
	packageNamePattern = regexp.MustCompile(`^[a-z0-9@_+][a-z0-9@._+-]+$`)
	versionPattern     = regexp.MustCompile(`^[A-Za-z0-9_+][A-Za-z0-9._+]*$`)
	idPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	digestPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// PackageName derives the Arch package name from the executable's file name:
// lowercase, with the -bin suffix a binary package has.
func PackageName(executable string) (string, error) {
	name := strings.ToLower(executable) + "-bin"
	if !packageNamePattern.MatchString(name) {
		return "", fmt.Errorf("pkgbuild: %q cannot name an Arch package: it needs lowercase letters, digits and @ . _ + -, and must start with a letter, digit, @, _ or +", executable)
	}
	return name, nil
}

// ArchName maps a Go architecture to Arch's: amd64 is x86_64 and arm64 is
// aarch64.
func ArchName(arch string) (string, error) {
	switch arch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	}
	return "", fmt.Errorf("pkgbuild: architecture %q must be amd64 or arm64", arch)
}

func (pkg Package) check() error {
	if _, err := pkg.packageNameValidated(); err != nil {
		return err
	}
	if _, err := ArchName(pkg.Arch); err != nil {
		return err
	}
	switch {
	case !versionPattern.MatchString(pkg.Version):
		return fmt.Errorf("pkgbuild: %q is not an Arch version (it cannot contain '-')", pkg.Version)
	case !idPattern.MatchString(pkg.ID):
		return fmt.Errorf("pkgbuild: %q cannot name a desktop file", pkg.ID)
	case pkg.Name == "":
		return fmt.Errorf("pkgbuild: the application needs a display name")
	case pkg.Archive == "" || strings.ContainsAny(pkg.Archive, "/\\") || strings.ContainsAny(pkg.Archive, "\r\n") || pkg.Archive == "." || pkg.Archive == "..":
		return fmt.Errorf("pkgbuild: %q is not an archive file name", pkg.Archive)
	case !digestPattern.MatchString(pkg.SHA256):
		return fmt.Errorf("pkgbuild: %q is not a lowercase SHA-256 digest", pkg.SHA256)
	case strings.ContainsAny(pkg.Maintainer, "\r\n"):
		return fmt.Errorf("pkgbuild: the maintainer must be one line")
	case strings.ContainsAny(pkg.Comment, "\r\n"):
		return fmt.Errorf("pkgbuild: the comment must be one line")
	case pkg.IconPixels < 0:
		return fmt.Errorf("pkgbuild: the icon size cannot be negative")
	}
	for _, dependency := range pkg.Depends {
		if dependency == "" || strings.ContainsAny(dependency, "\r\n") {
			return fmt.Errorf("pkgbuild: %q is not one dependency", dependency)
		}
	}
	if pkg.SourceURL != "" && (!strings.HasPrefix(pkg.SourceURL, "https://") && !strings.HasPrefix(pkg.SourceURL, "http://")) {
		return fmt.Errorf("pkgbuild: the source URL %q must be http or https", pkg.SourceURL)
	}
	return nil
}

// packageNameValidated is PackageName plus the length rule Arch enforces.
func (pkg Package) packageNameValidated() (string, error) {
	name, err := PackageName(pkg.Executable)
	if err != nil {
		return "", err
	}
	if len(name) > 255 {
		return "", fmt.Errorf("pkgbuild: the package name %q is longer than 255 characters", name)
	}
	return name, nil
}

// PKGBUILD is the package description makepkg reads.
func (pkg Package) PKGBUILD() ([]byte, error) {
	if err := pkg.check(); err != nil {
		return nil, err
	}
	name, _ := pkg.packageNameValidated()
	arch, _ := ArchName(pkg.Arch)
	maintainer := pkg.Maintainer
	if maintainer == "" {
		maintainer = pkg.Name
	}
	description := pkg.Comment
	if description == "" {
		description = pkg.Name
	}
	source := pkg.Archive
	if pkg.SourceURL != "" {
		source = pkg.Archive + "::" + pkg.SourceURL
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Maintainer: %s\n", oneLine(maintainer))
	fmt.Fprintf(&b, "pkgname=%s\n", quote(name))
	fmt.Fprintf(&b, "pkgver=%s\n", quote(pkg.Version))
	fmt.Fprintf(&b, "pkgrel=1\n")
	fmt.Fprintf(&b, "pkgdesc=%s\n", quote(description))
	fmt.Fprintf(&b, "arch=(%s)\n", quote(arch))
	if pkg.Homepage != "" {
		fmt.Fprintf(&b, "url=%s\n", quote(pkg.Homepage))
	}
	fmt.Fprintf(&b, "license=(%s)\n", quote("custom"))
	if len(pkg.Depends) > 0 {
		fmt.Fprintf(&b, "depends=(%s)\n", quotedList(pkg.Depends))
	}
	fmt.Fprintf(&b, "provides=(%s)\n", quote(pkg.Executable))
	fmt.Fprintf(&b, "conflicts=(%s)\n", quote(pkg.Executable))
	fmt.Fprintf(&b, "options=(%s %s)\n", quote("!strip"), quote("!debug"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "_archive=%s\n", quote(pkg.Archive))
	fmt.Fprintf(&b, "_exe=%s\n", quote(pkg.Executable))
	fmt.Fprintf(&b, "_desktop=%s\n", quote(pkg.ID))
	fmt.Fprintf(&b, "_icon_size=%d\n", pkg.IconPixels)
	b.WriteString("\n")
	fmt.Fprintf(&b, "source=(%s)\n", quote(source))
	fmt.Fprintf(&b, "sha256sums=(%s)\n", quote(pkg.SHA256))
	b.WriteString(`
package() {
	install -dm755 "$pkgdir/opt/$pkgname"
	bsdtar -xf "$srcdir/$_archive" -C "$pkgdir/opt/$pkgname" --strip-components=1
	rm -f "$pkgdir/opt/$pkgname/install.sh"

	install -dm755 "$pkgdir/usr/bin"
	ln -s "/opt/$pkgname/$_exe" "$pkgdir/usr/bin/$_exe"

	install -Dm644 "$pkgdir/opt/$pkgname/$_exe.desktop" "$pkgdir/usr/share/applications/$_desktop.desktop"
	if [ "$_icon_size" -gt 0 ]; then
		install -Dm644 "$pkgdir/opt/$pkgname/$_exe.png" \
			"$pkgdir/usr/share/icons/hicolor/${_icon_size}x${_icon_size}/apps/$_exe.png"
	fi
	if [ -f "$pkgdir/opt/$pkgname/$_exe.mime.xml" ]; then
		install -Dm644 "$pkgdir/opt/$pkgname/$_exe.mime.xml" "$pkgdir/usr/share/mime/packages/$_desktop.xml"
	fi
	install -Dm644 "$pkgdir/opt/$pkgname/README.txt" "$pkgdir/usr/share/doc/$pkgname/README.txt"
}
`)
	return []byte(b.String()), nil
}

// SRCINFO is the package description AUR reads. It repeats the PKGBUILD's
// fields in makepkg's plain key and value format.
func (pkg Package) SRCINFO() ([]byte, error) {
	if err := pkg.check(); err != nil {
		return nil, err
	}
	name, _ := pkg.packageNameValidated()
	arch, _ := ArchName(pkg.Arch)
	description := pkg.Comment
	if description == "" {
		description = pkg.Name
	}
	source := pkg.Archive
	if pkg.SourceURL != "" {
		source = pkg.Archive + "::" + pkg.SourceURL
	}
	var b strings.Builder
	field := func(key, value string) { b.WriteString("\t" + key + " = " + value + "\n") }
	b.WriteString("pkgbase = " + name + "\n")
	field("pkgdesc", oneLine(description))
	field("pkgver", pkg.Version)
	field("pkgrel", "1")
	if pkg.Homepage != "" {
		field("url", pkg.Homepage)
	}
	field("arch", arch)
	field("license", "custom")
	for _, dependency := range pkg.Depends {
		field("depends", dependency)
	}
	field("provides", pkg.Executable)
	field("conflicts", pkg.Executable)
	field("options", "!strip")
	field("options", "!debug")
	field("source", source)
	field("sha256sums", pkg.SHA256)
	b.WriteString("\npkgname = " + name + "\n")
	return []byte(b.String()), nil
}

// quote is a single-quoted shell word, safe for any value the checks accept.
func quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = quote(value)
	}
	return strings.Join(quoted, " ")
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
