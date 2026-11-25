package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// requireDiskImages skips a test that makes real disk images with hdiutil unless
// VIVA_DISK_IMAGE_TESTS is set. The disk image service of a shared macOS machine fails
// now and then whatever the code does, so these run where a change can affect how an
// image is made (the disk-image CI job), not on every change.
func requireDiskImages(t *testing.T) {
	t.Helper()
	requireMac(t)
	if os.Getenv("VIVA_DISK_IMAGE_TESTS") == "" {
		t.Skip("makes real disk images with hdiutil; set VIVA_DISK_IMAGE_TESTS=1 to run")
	}
}

func requireMac(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("packaging builds a macOS program, which needs a macOS host")
	}
	// A developer's own signing setup must not reach these runs.
	for _, name := range signingVariables {
		t.Setenv(name, "")
	}
}

var signingVariables = []string{
	"APPLE_SIGNING_IDENTITY", "APPLE_CERTIFICATE", "APPLE_CERTIFICATE_PASSWORD", "APPLE_NOTARY_PROFILE",
	"APPLE_API_KEY", "APPLE_API_KEY_ID", "APPLE_API_ISSUER", "APPLE_ID", "APPLE_PASSWORD", "APPLE_TEAM_ID",
}

func writeIcon(t *testing.T, path string, size int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / size), G: uint8(y * 255 / size), B: 90, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

const packageTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.1"
icon = "icon.png"

[mac]
category = "public.app-category.productivity"
copyright = "© 2026 Example"

[mac.info]
NSMicrophoneUsageDescription = "Dictation."
`

func TestPackageBuildsARunnableBundle(t *testing.T) {
	requireMac(t)
	dir := fixture(t, packageTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 512)

	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir, "--zip"}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	app := filepath.Join(dir, "build", "Fixture Notes.app")
	for _, want := range []string{
		"packaged ", "  app      Fixture Notes 2.0.1 (dev.example.notes)", "  target   darwin/" + runtime.GOARCH + " (macOS 12.0 or later)",
		"  program  ", "  bundle   ", "largest 512 px; 1024 px is recommended", "  zip      ", "  signing  none",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, output.String())
		}
	}

	plist, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<string>dev.example.notes</string>", "<string>fixture-notes</string>", "<string>2.0.1</string>",
		"<key>LSMinimumSystemVersion</key>\n\t<string>12.0</string>", "public.app-category.productivity",
		"© 2026 Example", "<key>NSMicrophoneUsageDescription</key>", "<key>CFBundleIconFile</key>\n\t<string>AppIcon.icns</string>",
	} {
		if !strings.Contains(string(plist), want) {
			t.Errorf("Info.plist lacks %q", want)
		}
	}
	if output, err := exec.Command("plutil", "-lint", filepath.Join(app, "Contents", "Info.plist")).CombinedOutput(); err != nil {
		t.Errorf("plutil: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(app, "Contents", "Resources", "AppIcon.icns")); err != nil {
		t.Error(err)
	}

	// The program inside the bundle is the one that was built, and it runs.
	if err := exec.Command(filepath.Join(app, "Contents", "MacOS", "fixture-notes")).Run(); err != nil {
		t.Errorf("the packaged program failed to run: %v", err)
	}

	archive := filepath.Join(dir, "build", "fixture-notes-2.0.1-darwin-"+runtime.GOARCH+".zip")
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	found := false
	for _, file := range reader.File {
		if file.Name == "Fixture Notes.app/Contents/MacOS/fixture-notes" {
			found = file.Mode()&0o111 != 0
		}
	}
	if !found {
		t.Error("the archive lacks the executable program")
	}
}

func TestPackageRebuildsInPlaceAndIsReproducible(t *testing.T) {
	requireMac(t)
	dir := fixture(t, "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n")
	pack := func() []byte {
		if err := runCLI([]string{"package", "-C", dir, "--zip"}, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "build", "notes-0.1.0-darwin-"+runtime.GOARCH+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first := pack()
	stale := filepath.Join(dir, "build", "Notes.app", "Contents", "stale")
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := pack()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a file of the previous bundle survived the rebuild")
	}
	if !bytes.Equal(first, second) {
		t.Error("packaging the same source twice gave different archives")
	}
	if _, err := os.Stat(filepath.Join(dir, "build", "Notes.app", "Contents", "Resources")); !os.IsNotExist(err) {
		t.Error("a bundle without an icon has a Resources directory")
	}
}

func TestPackageSaysWhatIsMissing(t *testing.T) {
	requireMac(t)
	for _, test := range []struct {
		name, toml string
		arguments  []string
		want       string
	}{
		{"no viva.toml", "", nil, "viva.toml not found"},
		{"no id", "[app]\nname = \"Notes\"\n", nil, "[app] id"},
		{"no name", "[app]\nid = \"dev.example.notes\"\n", nil, "[app] name"},
		{"a typo in viva.toml", "[app]\nnmae = \"x\"\n", nil, "viva.toml:2: [app] nmae"},
		{"another target", "[app]\nname = \"N\"\nid = \"dev.example.n\"\n", []string{"--target", "web"}, "viva build for the browser"},
		{"an argument", "[app]\nname = \"N\"\nid = \"dev.example.n\"\n", []string{"./cmd/x"}, "no arguments"},
		{"a missing icon", "[app]\nname = \"N\"\nid = \"dev.example.n\"\nicon = \"gone.png\"\n", nil, "gone.png"},
		{"a bad extra key", "[app]\nname = \"N\"\nid = \"dev.example.n\"\n[mac.info]\nCFBundleIdentifier = \"x\"\n", nil, "CFBundleIdentifier"},
	} {
		dir := fixture(t, test.toml)
		err := runCLI(append([]string{"package", "-C", dir}, test.arguments...), io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v, want it to mention %q", test.name, err, test.want)
		}
		if entries, _ := os.ReadDir(filepath.Join(dir, "build")); len(entries) != 0 {
			t.Errorf("%s: left %d files in build/", test.name, len(entries))
		}
	}
}

func TestPackageRejectsASmallIconBeforeBuilding(t *testing.T) {
	requireMac(t)
	dir := fixture(t, "[app]\nname = \"N\"\nid = \"dev.example.n\"\nicon = \"icon.png\"\n")
	writeIcon(t, filepath.Join(dir, "icon.png"), 64)
	err := runCLI([]string{"package", "-C", dir}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "at least 128 px") {
		t.Fatalf("a 64 px icon: %v", err)
	}
}

func TestPackageNeverDeletesWhatIsNotABundle(t *testing.T) {
	requireMac(t)
	dir := fixture(t, "[app]\nname = \"N\"\nid = \"dev.example.n\"\n")
	precious := filepath.Join(dir, "Precious.app")
	if err := os.MkdirAll(precious, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(precious, "keep.txt")
	os.WriteFile(keep, []byte("x"), 0o644)
	err := runCLI([]string{"package", "-C", dir, "-o", precious}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("the directory was deleted")
	}
}

func TestPackageFailureLeavesThePreviousBundle(t *testing.T) {
	requireMac(t)
	dir := fixture(t, "[app]\nname = \"N\"\nid = \"dev.example.n\"\n")
	if err := runCLI([]string{"package", "-C", dir}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(dir, "build", "N.app", "Contents", "Info.plist")
	before, _ := os.ReadFile(plist)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { broken() }\n"), 0o644)
	if err := runCLI([]string{"package", "-C", dir}, io.Discard, io.Discard); err == nil {
		t.Fatal("a package that does not compile was bundled")
	}
	after, err := os.ReadFile(plist)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("a failed build damaged the previous bundle")
	}
}

func TestCheckDeployment(t *testing.T) {
	for _, test := range []struct {
		required, declared string
		ok                 bool
	}{
		{"12.0", "12.0", true}, {"12.0", "13.0", true}, {"11.0", "12.0", true}, {"13.0", "12.0", false}, {"15.5", "12.0", false}, {"12.0.1", "12.0", false},
	} {
		err := checkDeployment(test.required, test.declared)
		if (err == nil) != test.ok {
			t.Errorf("program needs %s, plist says %s: %v", test.required, test.declared, err)
		}
		if err != nil && !strings.Contains(err.Error(), test.required) {
			t.Errorf("the error does not name the version the program needs: %v", err)
		}
	}
}

func TestPackageRegistersDocumentTypesInTheBundle(t *testing.T) {
	requireMac(t)
	dir := fixture(t, packageTOML+"\n[[app.file_types]]\nname = \"Notes document\"\nextensions = [\"note\"]\n")
	writeIcon(t, filepath.Join(dir, "icon.png"), 512)
	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), "  opens    .note\n") || strings.Contains(output.String(), "singleinstance") {
		t.Errorf("report:\n%s", output.String())
	}
	plist, err := os.ReadFile(filepath.Join(dir, "build", "Fixture Notes.app", "Contents", "Info.plist"))
	if err != nil || !strings.Contains(string(plist), "<key>CFBundleDocumentTypes</key>") || !strings.Contains(string(plist), "<string>note</string>") {
		t.Errorf("Info.plist: %v\n%s", err, plist)
	}
}
