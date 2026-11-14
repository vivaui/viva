package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const linuxTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.1"
icon = "icon.png"

[linux]
category = "Development"
comment = "Takes notes"
maintainer = "Ann Lee <ann@example.com>"
homepage = "https://example.com/notes"
depends = ["libfoo1 (>= 1.2)"]
keywords = ["notes", "text"]
`

func packageLinuxFixture(t *testing.T, toml string, iconSize int) string {
	t.Helper()
	dir := fixture(t, toml)
	if iconSize > 0 {
		writeIcon(t, filepath.Join(dir, "icon.png"), iconSize)
	}
	return dir
}

func runLinuxPackage(t *testing.T, dir string, extra ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	arguments := append([]string{"package", "-C", dir, "--target", "linux", "--arch", "amd64"}, extra...)
	err := runCLI(arguments, &output, io.Discard)
	return output.String(), err
}

func readTarGz(t *testing.T, data []byte) (names []string, headers map[string]*tar.Header, contents map[string][]byte) {
	t.Helper()
	zipped, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(zipped)
	headers, contents = map[string]*tar.Header{}, map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(reader)
		names = append(names, header.Name)
		headers[header.Name], contents[header.Name] = header, body
	}
}

// arMembers splits a .deb into its members.
func arMembers(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatal("a .deb must start with the ar magic")
	}
	members := map[string][]byte{}
	var order []string
	rest := data[8:]
	for len(rest) > 0 {
		name := strings.TrimSpace(string(rest[:16]))
		var size int
		if _, err := fmt.Sscan(strings.TrimSpace(string(rest[48:58])), &size); err != nil {
			t.Fatal(err)
		}
		members[name] = rest[60 : 60+size]
		order = append(order, name)
		rest = rest[60+size+size%2:]
	}
	if strings.Join(order, " ") != "debian-binary control.tar.gz data.tar.gz" {
		t.Fatalf("members = %v", order)
	}
	return members
}

func TestPackageLinuxMakesATarball(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 512)
	output, err := runLinuxPackage(t, dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	for _, want := range []string{"packaged ", "  app      Fixture Notes 2.0.1 (dev.example.notes)", "  target   linux/amd64", "  program  ", "  icon     ", "  tar.gz   ", "  signing  none"} {
		if !strings.Contains(output, want) {
			t.Errorf("the report lacks %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "  deb ") {
		t.Errorf("a deb was reported without being asked for:\n%s", output)
	}
	archive := filepath.Join(dir, "build", "fixture-notes-2.0.1-linux-amd64.tar.gz")
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	_, headers, contents := readTarGz(t, data)
	root := "fixture-notes-2.0.1-linux-amd64/"
	for name, mode := range map[string]int64{
		root + "fixture-notes": 0o755, root + "install.sh": 0o755, root + "fixture-notes.desktop": 0o644,
		root + "fixture-notes.png": 0o644, root + "README.txt": 0o644,
	} {
		header := headers[name]
		if header == nil {
			t.Errorf("%s is missing", name)
			continue
		}
		if header.Mode != mode || header.Uid != 0 || header.Gid != 0 {
			t.Errorf("%s: mode %o owner %d:%d", name, header.Mode, header.Uid, header.Gid)
		}
	}
	if !bytes.HasPrefix(contents[root+"fixture-notes"], []byte("\x7fELF")) {
		t.Error("the program is not an ELF executable")
	}
	desktop := string(contents[root+"fixture-notes.desktop"])
	for _, want := range []string{"Name=Fixture Notes\n", "Comment=Takes notes\n", "Exec=fixture-notes\n", "Icon=fixture-notes\n", "Categories=Development;\n", "Keywords=notes;text;\n", "Terminal=false\n"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("the desktop file lacks %q:\n%s", want, desktop)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "build")); len(entries) != 1 {
		t.Errorf("build/ holds %d entries, want only the archive", len(entries))
	}
}

func TestPackageLinuxMakesADeb(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 256)
	output, err := runLinuxPackage(t, dir, "--format", "deb")
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	if !strings.Contains(output, "  deb      ") || strings.Contains(output, "  tar.gz") {
		t.Errorf("the report:\n%s", output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "build", "fixture-notes_2.0.1_amd64.deb"))
	if err != nil {
		t.Fatal(err)
	}
	members := arMembers(t, data)
	if string(members["debian-binary"]) != "2.0\n" {
		t.Errorf("debian-binary = %q", members["debian-binary"])
	}
	_, _, control := readTarGz(t, members["control.tar.gz"])
	text := string(control["./control"])
	for _, want := range []string{
		"Package: fixture-notes\n", "Version: 2.0.1\n", "Architecture: amd64\n", "Maintainer: Ann Lee <ann@example.com>\n",
		"Depends: libvulkan1, libxcb1, libcairo2, libfoo1 (>= 1.2)\n", "Homepage: https://example.com/notes\n", "Description: Takes notes\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("control lacks %q:\n%s", want, text)
		}
	}
	names, headers, contents := readTarGz(t, members["data.tar.gz"])
	if headers["./usr/bin/fixture-notes"] == nil || headers["./usr/bin/fixture-notes"].Mode != 0o755 {
		t.Errorf("the program is missing or not executable: %v", names)
	}
	if !bytes.HasPrefix(contents["./usr/bin/fixture-notes"], []byte("\x7fELF")) {
		t.Error("the program is not an ELF executable")
	}
	for _, name := range []string{
		"./usr/share/applications/dev.example.notes.desktop",
		"./usr/share/icons/hicolor/48x48/apps/fixture-notes.png", "./usr/share/icons/hicolor/64x64/apps/fixture-notes.png",
		"./usr/share/icons/hicolor/128x128/apps/fixture-notes.png", "./usr/share/icons/hicolor/256x256/apps/fixture-notes.png",
	} {
		if headers[name] == nil {
			t.Errorf("%s is missing", name)
		}
	}
}

func TestPackageLinuxBothFormatsAreReproducible(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 300)
	pack := func() (tarball, deb []byte) {
		output, err := runLinuxPackage(t, dir, "--format", "deb,tar.gz")
		if err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		if !strings.Contains(output, "  tar.gz   ") || !strings.Contains(output, "  deb      ") {
			t.Errorf("the report:\n%s", output)
		}
		tarball, err = os.ReadFile(filepath.Join(dir, "build", "fixture-notes-2.0.1-linux-amd64.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}
		deb, err = os.ReadFile(filepath.Join(dir, "build", "fixture-notes_2.0.1_amd64.deb"))
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	tarA, debA := pack()
	tarB, debB := pack()
	if !bytes.Equal(tarA, tarB) {
		t.Error("the tar.gz differs between builds")
	}
	if !bytes.Equal(debA, debB) {
		t.Error("the deb differs between builds")
	}
}

func TestPackageLinuxOutputDirectoryAndArm64(t *testing.T) {
	dir := packageLinuxFixture(t, "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n", 0)
	out := filepath.Join(t.TempDir(), "dist", "linux")
	var output bytes.Buffer
	err := runCLI([]string{"package", "-C", dir, "--target", "linux", "--arch", "arm64", "-o", out, "--format", "tar.gz,deb"}, &output, io.Discard)
	if err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	for _, name := range []string{"notes-0.1.0-linux-arm64.tar.gz", "notes_0.1.0_arm64.deb"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Error(err)
		}
	}
	if !strings.Contains(output.String(), "target   linux/arm64") || !strings.Contains(output.String(), "icon     none") {
		t.Errorf("the report:\n%s", output.String())
	}
	// Without an icon the packages hold none.
	data, _ := os.ReadFile(filepath.Join(out, "notes-0.1.0-linux-arm64.tar.gz"))
	names, _, _ := readTarGz(t, data)
	for _, name := range names {
		if strings.HasSuffix(name, ".png") {
			t.Errorf("%s in a package without an icon", name)
		}
	}
}

func TestPackageLinuxWarnsAboutASmallIcon(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 128)
	output, err := runLinuxPackage(t, dir, "--format", "deb")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "(128 px; 256 px is recommended)") {
		t.Errorf("no warning:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "build", "fixture-notes_2.0.1_amd64.deb")); err != nil {
		t.Error(err)
	}
}

func TestPackageLinuxFailures(t *testing.T) {
	for _, test := range []struct {
		name, toml string
		iconSize   int
		arguments  []string
		want       string
	}{
		{"unknown format", linuxTOML, 256, []string{"--format", "rpm"}, `cannot be made as "rpm"`},
		{"msi", linuxTOML, 256, []string{"--format", "nsis"}, `cannot be made as "nsis"`},
		{"zip flag", linuxTOML, 256, []string{"--zip"}, "--zip does not apply"},
		{"small icon", linuxTOML, 64, nil, "at least 128 px"},
		{"missing icon", linuxTOML, 0, nil, "icon.png"},
		{"icns icon", strings.Replace(linuxTOML, "icon.png", "icon.icns", 1), 0, nil, "must be a .png"},
		{"bad executable for deb", "[app]\nname = \"N\"\nid = \"dev.example.n\"\nexecutable = \"x\"\n", 0, []string{"--format", "deb"}, "cannot name a Debian package"},
		{"bad category", "[app]\nname = \"N\"\nid = \"dev.example.n\"\n[linux]\ncategory = \"Games\"\n", 0, nil, "[linux] category"},
	} {
		dir := packageLinuxFixture(t, test.toml, test.iconSize)
		_, err := runLinuxPackage(t, dir, test.arguments...)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v, want it to mention %q", test.name, err, test.want)
		}
		if entries, _ := os.ReadDir(filepath.Join(dir, "build")); len(entries) != 0 {
			t.Errorf("%s: left %d files in build/", test.name, len(entries))
		}
	}
}

func TestPackageLinuxFailureLeavesPreviousOutput(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 256)
	if _, err := runLinuxPackage(t, dir, "--format", "tar.gz,deb"); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(dir, "build")
	before := map[string][]byte{}
	entries, _ := os.ReadDir(build)
	for _, entry := range entries {
		before[entry.Name()], _ = os.ReadFile(filepath.Join(build, entry.Name()))
	}
	if len(before) != 2 {
		t.Fatalf("expected two packages, got %d", len(before))
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { broken() }\n"), 0o644)
	if _, err := runLinuxPackage(t, dir, "--format", "tar.gz,deb"); err == nil {
		t.Fatal("a program that does not compile was packaged")
	}
	entries, _ = os.ReadDir(build)
	if len(entries) != len(before) {
		t.Errorf("build/ now holds %d entries, want %d (no temporary files)", len(entries), len(before))
	}
	for name, data := range before {
		after, err := os.ReadFile(filepath.Join(build, name))
		if err != nil || !bytes.Equal(data, after) {
			t.Errorf("%s was damaged by a failed build", name)
		}
	}
}

func TestPackageLinuxRefusesAFileAsOutput(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 256)
	file := filepath.Join(dir, "out.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	_, err := runLinuxPackage(t, dir, "-o", file)
	if err == nil || !strings.Contains(err.Error(), "output directory") {
		t.Errorf("error = %v", err)
	}
}

func TestDebDepends(t *testing.T) {
	got := strings.Join(debDepends([]string{"libcairo2 (>= 1.16)", "foo"}), ", ")
	if got != "libvulkan1, libxcb1, libcairo2 (>= 1.16), foo" {
		t.Errorf("depends = %q", got)
	}
}

func TestPackageLinuxRegistersDocumentTypes(t *testing.T) {
	toml := linuxTOML[:strings.Index(linuxTOML, "[linux]")] + `
[[app.file_types]]
name = "Notes document"
extensions = ["note"]

[linux]
` + linuxTOML[strings.Index(linuxTOML, "[linux]")+len("[linux]\n"):]
	dir := packageLinuxFixture(t, toml, 512)
	output, err := runLinuxPackage(t, dir, "--format", "tar.gz,deb")
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	for _, want := range []string{"  opens    .note\n", "singleinstance"} {
		if !strings.Contains(output, want) {
			t.Errorf("the report lacks %q:\n%s", want, output)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "build", "fixture-notes-2.0.1-linux-amd64.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, contents := readTarGz(t, data)
	root := "fixture-notes-2.0.1-linux-amd64/"
	if desktop := string(contents[root+"fixture-notes.desktop"]); !strings.Contains(desktop, "Exec=fixture-notes %F\n") || !strings.Contains(desktop, "MimeType=application/x-dev-example-notes-note;\n") {
		t.Errorf("desktop file:\n%s", desktop)
	}
	if types := string(contents[root+"fixture-notes.mime.xml"]); !strings.Contains(types, `<glob pattern="*.note"/>`) {
		t.Errorf("mime package:\n%s", types)
	}
}

const linuxArchTOML = linuxTOML + `arch_depends = ["libxcb>=1.16", "cairo", "extra-lib"]
`

func TestPackageLinuxMakesAPKGBUILD(t *testing.T) {
	dir := packageLinuxFixture(t, linuxArchTOML, 256)
	output, err := runLinuxPackage(t, dir, "--format", "pkgbuild")
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	for _, want := range []string{"  tar.gz   ", "  pkgbuild ", "  srcinfo  "} {
		if !strings.Contains(output, want) {
			t.Errorf("the report lacks %q:\n%s", want, output)
		}
	}
	archive := filepath.Join(dir, "build", "fixture-notes-2.0.1-linux-amd64.tar.gz")
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatalf("the tarball is not beside the PKGBUILD: %v", err)
	}
	digest := sha256.Sum256(data)
	pkgbuildBytes, err := os.ReadFile(filepath.Join(dir, "build", "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	pkgbuild := string(pkgbuildBytes)
	for _, want := range []string{
		"pkgname='fixture-notes-bin'",
		"pkgver='2.0.1'",
		"arch=('x86_64')",
		"depends=('vulkan-icd-loader' 'libxcb>=1.16' 'cairo' 'extra-lib')",
		"source=('fixture-notes-2.0.1-linux-amd64.tar.gz')",
		"sha256sums=('" + hex.EncodeToString(digest[:]) + "')",
		"_icon_size=256",
	} {
		if !strings.Contains(pkgbuild, want) {
			t.Errorf("the PKGBUILD lacks %q:\n%s", want, pkgbuild)
		}
	}
	if strings.Contains(pkgbuild, "libfoo1") {
		t.Errorf("a Debian dependency reached the Arch package:\n%s", pkgbuild)
	}
	srcinfo, err := os.ReadFile(filepath.Join(dir, "build", ".SRCINFO"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pkgbase = fixture-notes-bin", "\tarch = x86_64", "\tdepends = extra-lib", "pkgname = fixture-notes-bin"} {
		if !strings.Contains(string(srcinfo), want) {
			t.Errorf("the .SRCINFO lacks %q:\n%s", want, srcinfo)
		}
	}
	if bash, err := exec.LookPath("bash"); err == nil {
		command := exec.Command(bash, "-n")
		command.Stdin = bytes.NewReader(pkgbuildBytes)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("bash -n: %v\n%s", err, output)
		}
	}
}

func TestPackageLinuxPKGBUILDSourceURL(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 256)
	url := "https://example.com/downloads/fixture-notes-2.0.1-linux-amd64.tar.gz"
	output, err := runLinuxPackage(t, dir, "--format", "pkgbuild", "--pkgbuild-url", url)
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "build", "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	want := "source=('fixture-notes-2.0.1-linux-amd64.tar.gz::" + url + "')"
	if !strings.Contains(string(data), want) {
		t.Fatalf("the PKGBUILD lacks %q:\n%s", want, data)
	}
}

func TestPackageLinuxRefusesABadPKGBUILDURL(t *testing.T) {
	dir := packageLinuxFixture(t, linuxTOML, 256)
	output, err := runLinuxPackage(t, dir, "--format", "pkgbuild", "--pkgbuild-url", "file:///tmp/app.tar.gz")
	if err == nil {
		t.Fatalf("a file URL was accepted:\n%s", output)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "build", "PKGBUILD")); !os.IsNotExist(statErr) {
		t.Errorf("a PKGBUILD was written after the failure: %v", statErr)
	}
}
