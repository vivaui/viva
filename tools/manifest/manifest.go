// Package manifest reads viva.toml, the project file the viva command uses to
// build and package an application.
//
// The file is TOML decoded strictly: an unknown table or key, a duplicate, or a
// value of the wrong type is an error that names the line, rather than being
// skipped, so a typo never silently changes what gets built.
package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// FileName is the project file viva looks for in the module directory.
const FileName = "viva.toml"

const (
	// DefaultVersion is used when [app] version is not set.
	DefaultVersion = "0.1.0"
	// DefaultOut is the directory build products go to.
	DefaultOut = "build"
	// MacOSFloor is the oldest macOS the Go toolchain supports: Go 1.26 is the
	// last release that runs on macOS 12 Monterey, and Go 1.27 requires 13.
	// Raise it when the toolchain does.
	MacOSFloor = "12.0"
)

// Manifest is a parsed viva.toml with defaults applied.
type Manifest struct {
	// Dir is the directory holding viva.toml (or the directory viva was pointed
	// at when there is none). Every relative path in the manifest is relative
	// to it, and the go tool runs in it.
	Dir string
	// Found reports whether a viva.toml was read.
	Found bool

	App   App
	Build Build
	Web   Web
	Mac   Mac
	// Windows and Linux hold the settings of those targets' packages.
	Windows Windows
	Linux   Linux
}

// App identifies the application.
type App struct {
	// Name is the display name; it also names the .app bundle.
	Name string
	// ID is the reverse-DNS bundle identifier, such as dev.example.notes.
	ID      string
	Version string
	// Executable is the file name of the built program.
	Executable string
	// Package is the main package to build, as the go tool understands it.
	Package string
	// Icon is a PNG (at least 128x128, 1024x1024 recommended) or .icns file.
	Icon string
	// URLSchemes are the link schemes the application opens, such as ssh for
	// ssh://host links. Each package format registers them with the system.
	URLSchemes []string
	// FileTypes are the document types the application opens. Each package
	// format registers them with the system, which then offers the application
	// for those files and passes their paths as arguments.
	FileTypes []FileType
}

// FileType is one kind of document the application opens.
type FileType struct {
	// Name is the description the system shows, such as "Markdown document".
	Name string
	// Extensions are the file name extensions, lower case without the dot.
	Extensions []string
	// MIME is the media type; it defaults to application/x-<id>-<first extension>,
	// which belongs to the application alone. Name a registered type, such as
	// text/markdown, for a format other programs know too.
	MIME string
	// UTI is the macOS Uniform Type Identifier, such as
	// net.daringfireball.markdown. Empty leaves LSItemContentTypes out and
	// relies on CFBundleTypeExtensions alone.
	UTI string
}

// Build holds settings shared by every target.
type Build struct {
	Tags []string
	Out  string
}

// Web holds the browser target's settings.
type Web struct {
	// Package is the js/wasm main package; the browser build has its own entry
	// point because it links a different platform backend.
	Package string
}

// Mac holds macOS settings.
type Mac struct {
	// MinSystem is the oldest macOS the application runs on. It is both the
	// deployment target the program is compiled for and LSMinimumSystemVersion.
	MinSystem string
	// Category is LSApplicationCategoryType, for example
	// public.app-category.developer-tools.
	Category  string
	Copyright string
	// Entitlements is a property list of entitlements to sign the bundle with,
	// relative to viva.toml. Signing with the hardened runtime, which notarization
	// requires, grants nothing beyond the defaults without it.
	Entitlements string
	// Info holds extra Info.plist string entries, sorted by key, such as
	// NSMicrophoneUsageDescription.
	Info []InfoEntry
	// DMG is how the disk image's window looks.
	DMG DMG
}

// DMG holds the settings of the disk image's window. Every field may be left
// unset.
type DMG struct {
	// Background is a PNG shown behind the icons, relative to viva.toml. Its
	// size in points is the window's content size: the pixel size over the scale
	// its resolution is tagged with, 144 dpi being 2x. Without one the window
	// has a plain light ground and an arrow between the icons.
	Background string
	// IconSize is the size of the icons in points.
	IconSize int
	// App and Applications place the application and the shortcut to
	// /Applications, from the top left of the window's content to the middle of
	// the icon. They default to either side of the window's middle.
	App, Applications *Point
}

// Point is a position in points.
type Point struct{ X, Y int }

// Windows holds settings of the Windows installer. See manifest_installer.go.
type Windows struct {
	// Publisher is shown in Apps & features and the installer; it defaults to
	// the application name.
	Publisher string
	// Scope is "user" (the default: installs under the user's profile, no
	// administrator rights) or "machine" (Program Files, asks for them).
	Scope string
	// URL is the application's web page, shown in Apps & features.
	URL string
}

// Linux holds settings of the Linux packages. See manifest_deb.go.
type Linux struct {
	// Category is the freedesktop main category of the menu entry, such as
	// Utility or Development; it defaults to Utility.
	Category string
	// Comment is the one-line description in the menu entry and the package.
	Comment string
	// Maintainer is "Name <address>" for the .deb; it defaults to the app name.
	Maintainer string
	// Homepage is the application's web page.
	Homepage string
	// Depends are the .deb's dependencies, as in Debian's Depends field.
	Depends []string
	// ArchDepends are the Arch packages the PKGBUILD depends on. They are
	// written separately because Debian and Arch name the same library
	// differently, and a Debian name in an Arch package cannot be resolved.
	ArchDepends []string
	// Keywords help the desktop search find the menu entry.
	Keywords []string
}

// InfoEntry is one extra Info.plist string entry.
type InfoEntry struct {
	Key, Value string
}

// Load reads dir/viva.toml. A directory without one yields the defaults with
// Found false, so commands that do not need a manifest still work.
func Load(dir string) (*Manifest, error) {
	path := filepath.Join(dir, FileName)
	source, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults(dir), nil
	}
	if err != nil {
		return nil, err
	}
	manifest, err := Parse(path, dir, string(source))
	if err != nil {
		return nil, err
	}
	manifest.Found = true
	return manifest, nil
}

func defaults(dir string) *Manifest {
	return &Manifest{
		Dir:   dir,
		App:   App{Version: DefaultVersion, Package: "."},
		Build: Build{Out: DefaultOut},
		Mac:   Mac{MinSystem: MacOSFloor},
	}
}

var (
	idPattern         = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
	versionPattern    = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`)
	executablePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	tagPattern        = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)
	schemePattern     = regexp.MustCompile(`^[a-z][a-z0-9+.-]{0,31}$`)
	extensionPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_+-]{0,15}$`)
	mimePattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]{0,126}/[a-z0-9][a-z0-9!#$&^_.+-]{0,126}$`)
	utiPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*(\.[A-Za-z0-9][A-Za-z0-9-]*)+$`)
	plistKeyPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]*$`)
)

// document mirrors viva.toml for decoding.
type document struct {
	App struct {
		Name       string   `toml:"name"`
		ID         string   `toml:"id"`
		Version    string   `toml:"version"`
		Executable string   `toml:"executable"`
		Package    string   `toml:"package"`
		Icon       string   `toml:"icon"`
		URLSchemes []string `toml:"url_schemes"`
		FileTypes  []struct {
			Name       string   `toml:"name"`
			Extensions []string `toml:"extensions"`
			MIME       string   `toml:"mime"`
			UTI        string   `toml:"uti"`
		} `toml:"file_types"`
	} `toml:"app"`
	Build struct {
		Tags []string `toml:"tags"`
		Out  string   `toml:"out"`
	} `toml:"build"`
	Web struct {
		Package string `toml:"package"`
	} `toml:"web"`
	Mac struct {
		MinSystem    string            `toml:"min_system"`
		Category     string            `toml:"category"`
		Copyright    string            `toml:"copyright"`
		Entitlements string            `toml:"entitlements"`
		Info         map[string]string `toml:"info"`
		DMG          struct {
			Background   string `toml:"background"`
			IconSize     int    `toml:"icon_size"`
			App          []int  `toml:"app_position"`
			Applications []int  `toml:"applications_position"`
		} `toml:"dmg"`
	} `toml:"mac"`
	Windows struct {
		Publisher string `toml:"publisher"`
		Scope     string `toml:"scope"`
		URL       string `toml:"url"`
	} `toml:"windows"`
	Linux struct {
		Category    string   `toml:"category"`
		Comment     string   `toml:"comment"`
		Maintainer  string   `toml:"maintainer"`
		Homepage    string   `toml:"homepage"`
		Depends     []string `toml:"depends"`
		ArchDepends []string `toml:"arch_depends"`
		Keywords    []string `toml:"keywords"`
	} `toml:"linux"`
}

// knownKeys lists the keys of each table, for error messages.
var knownKeys = map[string][]string{
	"app":            {"executable", "file_types", "icon", "id", "name", "package", "url_schemes", "version"},
	"app.file_types": {"extensions", "mime", "name", "uti"},
	"build":          {"out", "tags"},
	"web":            {"package"},
	"mac":            {"category", "copyright", "dmg", "entitlements", "info", "min_system"},
	"mac.dmg":        {"app_position", "applications_position", "background", "icon_size"},
	"windows":        {"publisher", "scope", "url"},
	"linux":          {"arch_depends", "category", "comment", "depends", "homepage", "keywords", "maintainer"},
}

// Parse decodes the text of a viva.toml. name labels errors and dir becomes
// Manifest.Dir.
func Parse(name, dir, source string) (*Manifest, error) {
	source = strings.TrimPrefix(source, "\ufeff")
	var doc document
	// Decode over the defaults, so a key that is not set keeps its default.
	base := defaults(dir)
	doc.App.Version, doc.App.Package = base.App.Version, base.App.Package
	doc.Build.Out = base.Build.Out
	doc.Mac.MinSystem = base.Mac.MinSystem

	decoder := toml.NewDecoder(strings.NewReader(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, decodeError(name, err)
	}

	manifest := base
	manifest.App = App{
		Name: doc.App.Name, ID: doc.App.ID, Version: orDefault(doc.App.Version, base.App.Version),
		Executable: doc.App.Executable, Package: orDefault(doc.App.Package, base.App.Package), Icon: doc.App.Icon,
		URLSchemes: doc.App.URLSchemes,
	}
	for _, kind := range doc.App.FileTypes {
		manifest.App.FileTypes = append(manifest.App.FileTypes, FileType{Name: kind.Name, Extensions: kind.Extensions, MIME: kind.MIME, UTI: kind.UTI})
	}
	manifest.Build = Build{Tags: doc.Build.Tags, Out: orDefault(doc.Build.Out, base.Build.Out)}
	manifest.Web = Web{Package: doc.Web.Package}
	manifest.Mac = Mac{
		MinSystem: orDefault(doc.Mac.MinSystem, base.Mac.MinSystem),
		Category:  doc.Mac.Category, Copyright: doc.Mac.Copyright, Entitlements: doc.Mac.Entitlements,
	}
	manifest.Mac.DMG.Background, manifest.Mac.DMG.IconSize = doc.Mac.DMG.Background, doc.Mac.DMG.IconSize
	var err error
	if manifest.Mac.DMG.App, err = dmgPoint("app_position", doc.Mac.DMG.App); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if manifest.Mac.DMG.Applications, err = dmgPoint("applications_position", doc.Mac.DMG.Applications); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for key, value := range doc.Mac.Info {
		manifest.Mac.Info = append(manifest.Mac.Info, InfoEntry{Key: key, Value: value})
	}
	manifest.Windows = Windows{Publisher: doc.Windows.Publisher, Scope: doc.Windows.Scope, URL: doc.Windows.URL}
	manifest.Linux = Linux{
		Category: doc.Linux.Category, Comment: doc.Linux.Comment, Maintainer: doc.Linux.Maintainer,
		Homepage: doc.Linux.Homepage, Depends: doc.Linux.Depends, ArchDepends: doc.Linux.ArchDepends,
		Keywords: doc.Linux.Keywords,
	}
	sort.Slice(manifest.Mac.Info, func(i, j int) bool { return manifest.Mac.Info[i].Key < manifest.Mac.Info[j].Key })

	if err := manifest.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return manifest, nil
}

// dmgPoint reads a position given as [x, y]; nil is unset.
func dmgPoint(key string, values []int) (*Point, error) {
	if values == nil {
		return nil, nil
	}
	if len(values) != 2 || values[0] < 0 || values[1] < 0 {
		return nil, fmt.Errorf("[mac.dmg] %s: %v must be two numbers [x, y] of points from the top left of the window", key, values)
	}
	return &Point{values[0], values[1]}, nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// describeKey writes a key path the way the file does: [app] name.
func describeKey(path []string) string {
	if len(path) < 2 {
		return strings.Join(path, ".")
	}
	return "[" + strings.Join(path[:len(path)-1], ".") + "] " + path[len(path)-1]
}

// expectedKind says what value a key takes, for the type-mismatch message.
func expectedKind(path []string) string {
	switch joined := strings.Join(path, "."); joined {
	case "app", "build", "web", "mac", "mac.dmg", "mac.info", "windows", "linux":
		return "a table ([" + joined + "])"
	case "app.file_types":
		return "an array of tables ([[app.file_types]])"
	case "app.file_types.extensions":
		return "an array of strings"
	case "build.tags", "linux.depends", "linux.keywords", "app.url_schemes":
		return "an array of strings"
	case "mac.dmg.icon_size":
		return "an integer"
	case "mac.dmg.app_position", "mac.dmg.applications_position":
		return "an array of two integers, [x, y]"
	}
	return "a string"
}

// decodeError turns the TOML library's error into one that names the file, the
// line and, for a mistake in a key, what the choices are.
func decodeError(name string, err error) error {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) && len(strict.Errors) > 0 {
		first := strict.Errors[0]
		line, _ := first.Position()
		key := first.Key()
		if len(key) == 1 {
			return fmt.Errorf("%s:%d: unknown table [%s]; known tables: [app], [build], [linux], [mac], [mac.dmg], [mac.info], [web], [windows]", name, line, key[0])
		}
		table := strings.Join(key[:len(key)-1], ".")
		if keys, known := knownKeys[table]; known {
			return fmt.Errorf("%s:%d: [%s] %s: is not a known key; [%s] has: %s", name, line, table, key[len(key)-1], table, strings.Join(keys, ", "))
		}
		return fmt.Errorf("%s:%d: unknown table [%s]; known tables: [app], [build], [linux], [mac], [mac.dmg], [mac.info], [web], [windows]", name, line, table)
	}
	var decode *toml.DecodeError
	if errors.As(err, &decode) {
		line, _ := decode.Position()
		key := decode.Key()
		message := decode.Error()
		switch {
		case strings.Contains(message, "already defined"):
			return fmt.Errorf("%s:%d: key %s is set twice in its table", name, line, strings.Join(key, "."))
		case strings.Contains(message, "cannot decode"):
			return fmt.Errorf("%s:%d: %s: must be %s", name, line, describeKey(key), expectedKind(key))
		}
		return fmt.Errorf("%s:%d: %s", name, line, strings.TrimPrefix(message, "toml: "))
	}
	return fmt.Errorf("%s: %w", name, err)
}

// validate checks the values the file sets. The library reports where a key is
// misspelled or mistyped; a value that is well-formed TOML but not a valid name,
// identifier or version is reported by its key instead.
func (manifest *Manifest) validate() error {
	fail := func(table, key, format string, arguments ...any) error {
		return fmt.Errorf("[%s] %s: %s", table, key, fmt.Sprintf(format, arguments...))
	}
	app := manifest.App
	if app.Name != "" && (strings.ContainsAny(app.Name, "/\\:") || strings.IndexFunc(app.Name, func(r rune) bool { return r < ' ' || r == 0x7f }) >= 0) {
		return fail("app", "name", "%q cannot contain /, \\, : or control characters: it names the .app bundle", app.Name)
	}
	if app.ID != "" && !idPattern.MatchString(app.ID) {
		return fail("app", "id", "%q is not a reverse-DNS identifier such as dev.example.notes", app.ID)
	}
	if !versionPattern.MatchString(app.Version) {
		return fail("app", "version", "%q must be one to three dot-separated numbers, such as 1.2.0", app.Version)
	}
	if app.Executable != "" && !executablePattern.MatchString(app.Executable) {
		return fail("app", "executable", "%q must be a plain file name (letters, digits, . _ -)", app.Executable)
	}
	for _, scheme := range app.URLSchemes {
		if !schemePattern.MatchString(scheme) {
			return fail("app", "url_schemes", "%q is not a link scheme: lower-case letters, digits, + - . and a letter first", scheme)
		}
		switch scheme {
		case "http", "https", "file", "javascript", "data", "about":
			return fail("app", "url_schemes", "%q belongs to the system's browser and cannot be claimed", scheme)
		}
	}
	if err := app.validateFileTypes(); err != nil {
		return err
	}
	for _, tag := range manifest.Build.Tags {
		if !tagPattern.MatchString(tag) {
			return fail("build", "tags", "tag %q is not a valid build tag", tag)
		}
	}
	mac := manifest.Mac
	if !versionPattern.MatchString(mac.MinSystem) {
		return fail("mac", "min_system", "%q must look like 12.0 or 13.3", mac.MinSystem)
	}
	if CompareVersions(mac.MinSystem, MacOSFloor) < 0 {
		return fail("mac", "min_system", "%q is older than macOS %s, the oldest the Go toolchain supports", mac.MinSystem, MacOSFloor)
	}
	if size := mac.DMG.IconSize; size != 0 && (size < 16 || size > 512) {
		return fail("mac.dmg", "icon_size", "%d must be between 16 and 512 points, the range the Finder shows", size)
	}
	for _, entry := range mac.Info {
		if !plistKeyPattern.MatchString(entry.Key) {
			return fail("mac.info", entry.Key, "is not a valid Info.plist key")
		}
	}
	if err := manifest.validateWindows(); err != nil {
		return err
	}
	return manifest.validateLinux()
}

// validateFileTypes checks the document types and fills in the media type of
// those that name none. An extension may belong to one type only: the system
// could not tell which to open it with.
func (app *App) validateFileTypes() error {
	fail := func(index int, key, format string, arguments ...any) error {
		return fmt.Errorf("[[app.file_types]] #%d %s: %s", index+1, key, fmt.Sprintf(format, arguments...))
	}
	extensions, mimes, utis := map[string]int{}, map[string]int{}, map[string]int{}
	for index := range app.FileTypes {
		kind := &app.FileTypes[index]
		if kind.Name == "" || strings.IndexFunc(kind.Name, func(r rune) bool { return r < ' ' || r == 0x7f }) >= 0 {
			return fail(index, "name", "is required and must be one line of text, such as \"Markdown document\"")
		}
		if len(kind.Extensions) == 0 {
			return fail(index, "extensions", "needs at least one extension, such as [\"md\"]")
		}
		for _, extension := range kind.Extensions {
			if !extensionPattern.MatchString(extension) {
				return fail(index, "extensions", "%q must be lower-case letters and digits without the dot", extension)
			}
			if other, taken := extensions[extension]; taken {
				return fail(index, "extensions", "%q is already claimed by type #%d", extension, other+1)
			}
			extensions[extension] = index
		}
		if kind.MIME == "" {
			if app.ID == "" {
				return fail(index, "mime", "has no default without [app] id; set one or name the type")
			}
			kind.MIME = "application/x-" + strings.ToLower(strings.ReplaceAll(app.ID, ".", "-")) + "-" + kind.Extensions[0]
		}
		if !mimePattern.MatchString(kind.MIME) {
			return fail(index, "mime", "%q is not a media type such as text/markdown", kind.MIME)
		}
		if kind.UTI != "" && !utiPattern.MatchString(kind.UTI) {
			return fail(index, "uti", "%q is not a Uniform Type Identifier such as net.daringfireball.markdown", kind.UTI)
		}
		if other, taken := utis[kind.UTI]; taken {
			return fail(index, "uti", "%q is already the type of #%d", kind.UTI, other+1)
		}
		if kind.UTI != "" {
			utis[kind.UTI] = index
		}
		if other, taken := mimes[kind.MIME]; taken {
			return fail(index, "mime", "%q is already the type of #%d", kind.MIME, other+1)
		}
		mimes[kind.MIME] = index
	}
	return nil
}

// CompareVersions orders two dotted numeric versions; missing parts are zero.
func CompareVersions(left, right string) int {
	l, r := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < max(len(l), len(r)); index++ {
		a, b := versionPart(l, index), versionPart(r, index)
		if a != b {
			if a < b {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	number, _ := strconv.Atoi(parts[index])
	return number
}

// RequirePackaging reports what is missing for a packaged application.
func (manifest *Manifest) RequirePackaging() error {
	if !manifest.Found {
		return fmt.Errorf("%s not found in %s; packaging needs an application name and identifier (see viva help package)", FileName, manifest.Dir)
	}
	var missing []string
	if manifest.App.Name == "" {
		missing = append(missing, "[app] name")
	}
	if manifest.App.ID == "" {
		missing = append(missing, "[app] id")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: packaging needs %s", FileName, strings.Join(missing, " and "))
	}
	return nil
}

// ExecutableName is the file name of the built program: [app] executable, else
// the application name reduced to lowercase ASCII words joined by hyphens, else
// the last part of the identifier, else "app".
func (manifest *Manifest) ExecutableName() string {
	if manifest.App.Executable != "" {
		return manifest.App.Executable
	}
	if slug := Slug(manifest.App.Name); slug != "" {
		return slug
	}
	if id := manifest.App.ID; id != "" {
		if slug := Slug(id[strings.LastIndexByte(id, '.')+1:]); slug != "" {
			return slug
		}
	}
	return "app"
}

// Slug lowercases text and keeps ASCII letters and digits, joining the words
// with single hyphens: "Viva Gallery" becomes "viva-gallery".
func Slug(text string) string {
	var builder strings.Builder
	pending := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pending && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			pending = false
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			if pending && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			pending = false
			builder.WriteRune(r + 'a' - 'A')
		default:
			pending = true
		}
	}
	return builder.String()
}

// Path resolves a manifest path against the manifest directory.
func (manifest *Manifest) Path(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(manifest.Dir, path)
}
