package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const full = `# Gallery
[app]
name = "Viva Gallery"          # shown in the menu bar
id = "dev.viva.gallery"
version = "1.2.3"
executable = 'viva-gallery'
package = "./cmd/showcase"
icon = "assets/icon.png"

[build]
tags = ["vivahead", 'extra_tag',]
out = "dist"

[web]
package = "./cmd/gallery-web"

[mac]
min_system = "13.3"
category = "public.app-category.developer-tools"
copyright = "© 2026 Viva \u2014 \"quoted\""

[mac.info]
NSMicrophoneUsageDescription = "Records the demo."
NSCameraUsageDescription = "Shows the demo."
`

func TestParseReadsEveryKey(t *testing.T) {
	manifest, err := Parse("viva.toml", "/project", full)
	if err != nil {
		t.Fatal(err)
	}
	want := &Manifest{
		Dir: "/project",
		App: App{
			Name: "Viva Gallery", ID: "dev.viva.gallery", Version: "1.2.3", Executable: "viva-gallery",
			Package: "./cmd/showcase", Icon: "assets/icon.png",
		},
		Build: Build{Tags: []string{"vivahead", "extra_tag"}, Out: "dist"},
		Web:   Web{Package: "./cmd/gallery-web"},
		Mac: Mac{
			MinSystem: "13.3", Category: "public.app-category.developer-tools", Copyright: "© 2026 Viva — \"quoted\"",
			Info: []InfoEntry{
				{"NSCameraUsageDescription", "Shows the demo."},
				{"NSMicrophoneUsageDescription", "Records the demo."},
			},
		},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("manifest = %+v\nwant       %+v", manifest, want)
	}
}

func TestParseAppliesDefaults(t *testing.T) {
	manifest, err := Parse("viva.toml", "/p", "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.App.Version != DefaultVersion || manifest.App.Package != "." || manifest.Build.Out != DefaultOut || manifest.Mac.MinSystem != MacOSFloor {
		t.Fatalf("defaults = %+v", manifest)
	}
}

func TestParseReadsTheDiskImageWindow(t *testing.T) {
	manifest, err := Parse("viva.toml", "/p", "[mac.dmg]\nbackground = \"assets/dmg.png\"\nicon_size = 96\napp_position = [150, 180]\n")
	if err != nil {
		t.Fatal(err)
	}
	dmg := manifest.Mac.DMG
	if dmg.Background != "assets/dmg.png" || dmg.IconSize != 96 || dmg.App == nil || *dmg.App != (Point{150, 180}) || dmg.Applications != nil {
		t.Fatalf("dmg = %+v", dmg)
	}
}

func TestParseRejectsWithTheLineNumber(t *testing.T) {
	tables := "known tables: [app], [build], [linux], [mac], [mac.dmg], [mac.info], [web], [windows]"
	for _, test := range []struct{ name, source, want string }{
		{"unknown table", "[ap]\nname = \"x\"\n", "viva.toml:1: unknown table [ap]; " + tables},
		{"unknown empty table", "[app]\nname = \"x\"\n\n[wbe]\n", "viva.toml:4: unknown table [wbe]; " + tables},
		{"unknown key", "[app]\nnmae = \"x\"\n", "viva.toml:2: [app] nmae: is not a known key; [app] has: executable, file_types, icon, id, name, package, url_schemes, version"},
		{"unknown mac key", "[mac]\nfoo = \"x\"\n", "viva.toml:2: [mac] foo: is not a known key; [mac] has: category, copyright, dmg, entitlements, info, min_system"},
		{"key outside a table", "name = \"x\"\n", "viva.toml:1: unknown table [name]; " + tables},
		{"a table written as a value", "app = \"x\"\n", "viva.toml:1: app: must be a table ([app])"},
		{"wrong type", "[app]\nname = 3\n", "viva.toml:2: [app] name: must be a string"},
		{"a float for a string", "[app]\nname = 1.5\n", "viva.toml:2: [app] name: must be a string"},
		{"an inline table for a string", "[app]\nname = {a=1}\n", "viva.toml:2: [app] name: must be a string"},
		{"tags need an array", "[build]\ntags = \"x\"\n", "viva.toml:2: [build] tags: must be an array of strings"},
		{"tags hold strings", "[build]\ntags = [1]\n", "viva.toml:2: [build] tags: must be an array of strings"},
		{"info values are strings", "[mac.info]\nNSX = true\n", "viva.toml:2: [mac.info] NSX: must be a string"},
		{"duplicate key", "[app]\nname = \"a\"\nname = \"b\"\n", "viva.toml:3: key name is set twice in its table"},
		{"duplicate table", "[app]\n[app]\n", "viva.toml:2: table app already exists"},
		{"array of tables", "[[app]]\n", "viva.toml:1: cannot store an array table in a struct"},
		{"unterminated string", "[app]\nname = \"x\n", "viva.toml:2: basic strings cannot have new lines"},
		{"unclosed header", "[app\n", "viva.toml:1: expected ']' to close table name"},
		{"missing equals", "[app]\nname \"x\"\n", "viva.toml:2: expected '=' after key"},
		{"missing value", "[app]\nname =\n", "viva.toml:2: unexpected character"},
		{"bad escape", "[app]\nname = \"a\\qb\"\n", "viva.toml:2: invalid escape character"},
		// Well-formed TOML with a value viva cannot use: reported by key.
		{"bad id", "[app]\nid = \"notes\"\n", `viva.toml: [app] id: "notes" is not a reverse-DNS identifier such as dev.example.notes`},
		{"bad version", "[app]\nversion = \"v1\"\n", `viva.toml: [app] version: "v1" must be one to three dot-separated numbers, such as 1.2.0`},
		{"name with a slash", "[app]\nname = \"a/b\"\n", "viva.toml: [app] name:"},
		{"executable with a path", "[app]\nexecutable = \"bin/x\"\n", "viva.toml: [app] executable:"},
		{"bad tag", "[build]\ntags = [\"a b\"]\n", `viva.toml: [build] tags: tag "a b" is not a valid build tag`},
		{"below the floor", "[mac]\nmin_system = \"11.0\"\n", `viva.toml: [mac] min_system: "11.0" is older than macOS 12.0, the oldest the Go toolchain supports`},
		{"min_system shape", "[mac]\nmin_system = \"monterey\"\n", "viva.toml: [mac] min_system:"},
		{"dmg position shape", "[mac.dmg]\napp_position = [1]\n", "viva.toml: [mac.dmg] app_position: [1] must be two numbers"},
		{"dmg position type", "[mac.dmg]\napplications_position = \"1,2\"\n", "viva.toml:2: [mac.dmg] applications_position: must be an array of two integers, [x, y]"},
		{"dmg icon size", "[mac.dmg]\nicon_size = 4\n", "viva.toml: [mac.dmg] icon_size: 4 must be between 16 and 512"},
		{"info key", "[mac.info]\n\"bad key\" = \"x\"\n", "viva.toml: [mac.info] bad key: is not a valid Info.plist key"},
	} {
		_, err := Parse("viva.toml", "/p", test.source)
		if err == nil {
			t.Errorf("%s: accepted", test.name)
			continue
		}
		if !strings.HasPrefix(err.Error(), test.want) {
			t.Errorf("%s:\n got %v\nwant %s", test.name, err, test.want)
		}
	}
}

// viva.toml is ordinary TOML, so the parts of the language it does not need are
// accepted rather than refused.
func TestParseAcceptsTheRestOfTOML(t *testing.T) {
	manifest, err := Parse("viva.toml", "/p", `[app]
name = "Notes"
id = "dev.example.notes"

[build]
tags = [
  "a",   # first
  "b",
]

[mac]
copyright = """
Line one
Line two"""
`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifest.Build.Tags, []string{"a", "b"}) || manifest.Mac.Copyright != "Line one\nLine two" {
		t.Fatalf("tags %v, copyright %q", manifest.Build.Tags, manifest.Mac.Copyright)
	}
}

func TestParseHandlesWindowsLineEndingsAndABOM(t *testing.T) {
	manifest, err := Parse("viva.toml", "/p", "\ufeff[app]\r\nname = \"Notes\"\r\n")
	if err != nil || manifest.App.Name != "Notes" {
		t.Fatalf("manifest %+v, err %v", manifest, err)
	}
}

func TestParseKeepsAHashInsideAString(t *testing.T) {
	manifest, err := Parse("viva.toml", "/p", "[mac]\ncopyright = \"C # D\"  # comment\n")
	if err != nil || manifest.Mac.Copyright != "C # D" {
		t.Fatalf("copyright %q, err %v", manifest.Mac.Copyright, err)
	}
}

func TestLoadWithoutAManifestGivesDefaults(t *testing.T) {
	manifest, err := Load(t.TempDir())
	if err != nil || manifest.Found || manifest.App.Package != "." {
		t.Fatalf("manifest %+v, err %v", manifest, err)
	}
}

func TestLoadReadsTheFileFromTheDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := Load(dir)
	if err != nil || !manifest.Found || manifest.Dir != dir || manifest.App.Name != "Notes" {
		t.Fatalf("manifest %+v, err %v", manifest, err)
	}
	if got := manifest.Path("assets/i.png"); got != filepath.Join(dir, "assets", "i.png") {
		t.Fatalf("path = %q", got)
	}
	absolute := filepath.Join(t.TempDir(), "abs", "i.png") // absolute on every platform
	if got := manifest.Path(absolute); got != absolute {
		t.Fatalf("absolute path changed: %q", got)
	}
}

func TestLoadNamesTheFileInErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("[app]\nnmae = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), filepath.Join(dir, FileName)+":2:") {
		t.Fatalf("error = %v, want it to name the file and line", err)
	}
}

func TestRequirePackaging(t *testing.T) {
	if err := defaults("/p").RequirePackaging(); err == nil || !strings.Contains(err.Error(), "viva.toml not found") {
		t.Fatalf("no manifest: %v", err)
	}
	manifest, _ := Parse("viva.toml", "/p", "[app]\nversion = \"1.0\"\n")
	manifest.Found = true
	if err := manifest.RequirePackaging(); err == nil || !strings.Contains(err.Error(), "[app] name and [app] id") {
		t.Fatalf("missing name and id: %v", err)
	}
	manifest, _ = Parse("viva.toml", "/p", "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n")
	manifest.Found = true
	if err := manifest.RequirePackaging(); err != nil {
		t.Fatal(err)
	}
}

func TestExecutableName(t *testing.T) {
	for _, test := range []struct{ name, id, executable, want string }{
		{"Viva Gallery", "dev.viva.gallery", "", "viva-gallery"},
		{"  Notes!! 2  ", "dev.example.notes", "", "notes-2"},
		{"笔记", "dev.example.notes", "", "notes"},
		{"笔记", "dev.example.笔记", "", "app"},
		{"", "", "", "app"},
		{"Viva Gallery", "dev.viva.gallery", "gallery", "gallery"},
	} {
		manifest := &Manifest{App: App{Name: test.name, ID: test.id, Executable: test.executable}}
		if got := manifest.ExecutableName(); got != test.want {
			t.Errorf("name %q id %q executable %q: got %q, want %q", test.name, test.id, test.executable, got, test.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"12.0", "12.0", 0}, {"12", "12.0.0", 0}, {"12.1", "12.0", 1}, {"11.9", "12.0", -1}, {"13", "12.9.9", 1}, {"12.0.1", "12.0", 1},
	} {
		if got := CompareVersions(test.left, test.right); got != test.want {
			t.Errorf("compare(%s, %s) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestParseReadsAndChecksURLSchemes(t *testing.T) {
	manifest, err := Parse("viva.toml", "/p", "[app]\nname = \"N\"\nid = \"dev.x.n\"\nurl_schemes = [\"ssh\", \"telnet\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifest.App.URLSchemes, []string{"ssh", "telnet"}) {
		t.Fatalf("schemes = %q", manifest.App.URLSchemes)
	}
	for _, bad := range []string{`"SSH"`, `"has space"`, `"https"`, `""`, `"1x"`} {
		if _, err := Parse("viva.toml", "/p", "[app]\nname = \"N\"\nid = \"dev.x.n\"\nurl_schemes = ["+bad+"]\n"); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestFileTypes(t *testing.T) {
	loaded, err := Parse("viva.toml", "/p", "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n\n[[app.file_types]]\nname = \"Notes document\"\nextensions = [\"note\", \"nt\"]\n\n[[app.file_types]]\nname = \"Markdown\"\nextensions = [\"md\"]\nmime = \"text/markdown\"\nuti = \"net.daringfireball.markdown\"\n")
	if err != nil {
		t.Fatal(err)
	}
	kinds := loaded.App.FileTypes
	if len(kinds) != 2 || kinds[0].MIME != "application/x-dev-example-notes-note" || kinds[1].MIME != "text/markdown" || kinds[1].UTI != "net.daringfireball.markdown" || len(kinds[0].Extensions) != 2 {
		t.Fatalf("file types %+v", kinds)
	}
}

func TestFileTypeErrors(t *testing.T) {
	head := "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n"
	for _, test := range []struct{ name, source, want string }{
		{"no name", head + "[[app.file_types]]\nextensions = [\"md\"]\n", "#1 name"},
		{"no extensions", head + "[[app.file_types]]\nname = \"x\"\n", "#1 extensions"},
		{"dot", head + "[[app.file_types]]\nname = \"x\"\nextensions = [\".md\"]\n", "#1 extensions"},
		{"upper case", head + "[[app.file_types]]\nname = \"x\"\nextensions = [\"MD\"]\n", "#1 extensions"},
		{"claimed twice", head + "[[app.file_types]]\nname = \"x\"\nextensions = [\"md\"]\n[[app.file_types]]\nname = \"y\"\nextensions = [\"md\"]\n", "#2 extensions"},
		{"bad mime", head + "[[app.file_types]]\nname = \"x\"\nextensions = [\"md\"]\nmime = \"markdown\"\n", "#1 mime"},
		{"bad uti", head + "[[app.file_types]]\nname = \"x\"\nextensions = [\"md\"]\nuti = \"markdown\"\n", "#1 uti"},
		{"same mime", head + "[[app.file_types]]\nname = \"x\"\nextensions = [\"a\"]\nmime = \"text/x\"\n[[app.file_types]]\nname = \"y\"\nextensions = [\"b\"]\nmime = \"text/x\"\n", "#2 mime"},
		{"no id", "[app]\nname = \"Notes\"\n[[app.file_types]]\nname = \"x\"\nextensions = [\"md\"]\n", "#1 mime"},
		{"unknown key", head + "[[app.file_types]]\nname = \"x\"\nextension = [\"md\"]\n", "[app.file_types] extension: is not a known key"},
	} {
		if _, err := Parse("viva.toml", "/p", test.source); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: got %v, want %q", test.name, err, test.want)
		}
	}
}
