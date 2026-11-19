package debpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/tools/linuxicon"
	"github.com/vivaui/viva/tools/mimeinfo"
)

func sample() Package {
	return Package{
		Executable: "fixture-notes", ID: "dev.example.notes", Name: "Fixture Notes", Version: "2.0.1", Arch: "amd64",
		Maintainer: "Ann Lee <ann@example.com>", Homepage: "https://example.com/notes", Comment: "Takes notes",
		Description: "First paragraph.\n\nSecond paragraph.", Category: "Utility", Keywords: []string{"notes"},
		Depends: []string{"libc6 (>= 2.31)", "libvulkan1 | libvulkan-dev"}, Recommends: []string{"mesa-vulkan-drivers"}, Program: bytes.Repeat([]byte("p"), 3000),
		Icons: []linuxicon.Rendition{{Pixels: 48, PNG: []byte("png48")}, {Pixels: 256, PNG: []byte("png256")}},
	}
}

type arMember struct {
	name       string
	mtime, uid string
	mode       string
	size       int
	data       []byte
}

// parseAr reads an ar archive strictly.
func parseAr(t *testing.T, data []byte) []arMember {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatalf("magic = %q", data[:8])
	}
	rest := data[8:]
	var members []arMember
	for len(rest) > 0 {
		if len(rest) < 60 || string(rest[58:60]) != "`\n" {
			t.Fatalf("bad member header %q", rest[:min(len(rest), 60)])
		}
		header := string(rest[:60])
		size, err := strconv.Atoi(strings.TrimSpace(header[48:58]))
		if err != nil {
			t.Fatal(err)
		}
		member := arMember{
			name: strings.TrimSpace(header[0:16]), mtime: strings.TrimSpace(header[16:28]), uid: strings.TrimSpace(header[28:34]) + "/" + strings.TrimSpace(header[34:40]),
			mode: strings.TrimSpace(header[40:48]), size: size,
		}
		if len(rest) < 60+size {
			t.Fatal("truncated member")
		}
		member.data = rest[60 : 60+size]
		rest = rest[60+size:]
		if size%2 == 1 {
			if len(rest) == 0 || rest[0] != '\n' {
				t.Fatal("odd member is not padded")
			}
			rest = rest[1:]
		}
		members = append(members, member)
	}
	return members
}

func untar(t *testing.T, compressed []byte) (names []string, headers map[string]*tar.Header, contents map[string][]byte) {
	t.Helper()
	zipped, err := gzip.NewReader(bytes.NewReader(compressed))
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
		data, _ := io.ReadAll(reader)
		names = append(names, header.Name)
		headers[header.Name], contents[header.Name] = header, data
	}
}

func build(t *testing.T, pkg Package) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := pkg.Write(&buffer); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestArStructure(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	members := parseAr(t, build(t, sample()))
	if len(members) != 3 || members[0].name != "debian-binary" || members[1].name != "control.tar.gz" || members[2].name != "data.tar.gz" {
		t.Fatalf("members = %+v", members)
	}
	if string(members[0].data) != "2.0\n" || members[0].size != 4 {
		t.Errorf("debian-binary = %q", members[0].data)
	}
	for _, member := range members {
		if member.mtime != "1700000000" || member.uid != "0/0" || member.mode != "100644" {
			t.Errorf("%s: header %+v", member.name, member)
		}
	}
}

func TestArPadsOddMembers(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeAr(&buffer, time.Unix(5, 0), []member{{"a", []byte("odd")}, {"b", []byte("even")}, {"c", nil}}); err != nil {
		t.Fatal(err)
	}
	members := parseAr(t, buffer.Bytes())
	if len(members) != 3 || string(members[0].data) != "odd" || string(members[1].data) != "even" || members[2].size != 0 {
		t.Errorf("%+v", members)
	}
	if want := 8 + (60 + 3 + 1) + (60 + 4) + 60; buffer.Len() != want {
		t.Errorf("length %d, want %d", buffer.Len(), want)
	}
	if err := writeAr(io.Discard, time.Unix(0, 0), []member{{"a-name-that-is-too-long", nil}}); err == nil {
		t.Error("a long member name was accepted")
	}
}

func TestDataArchive(t *testing.T) {
	members := parseAr(t, build(t, sample()))
	names, headers, contents := untar(t, members[2].data)
	want := []string{
		"./", "./usr/", "./usr/bin/", "./usr/bin/fixture-notes", "./usr/share/", "./usr/share/applications/",
		"./usr/share/applications/dev.example.notes.desktop", "./usr/share/icons/", "./usr/share/icons/hicolor/",
		"./usr/share/icons/hicolor/256x256/", "./usr/share/icons/hicolor/256x256/apps/", "./usr/share/icons/hicolor/256x256/apps/fixture-notes.png",
		"./usr/share/icons/hicolor/48x48/", "./usr/share/icons/hicolor/48x48/apps/", "./usr/share/icons/hicolor/48x48/apps/fixture-notes.png",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Errorf("entries\n%s\nwant\n%s", strings.Join(names, "\n"), strings.Join(want, "\n"))
	}
	for name, header := range headers {
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "root" || header.Gname != "root" {
			t.Errorf("%s is owned by %d:%d", name, header.Uid, header.Gid)
		}
		wantMode := int64(0o644)
		if header.Typeflag == tar.TypeDir || name == "./usr/bin/fixture-notes" {
			wantMode = 0o755
		}
		if header.Mode != wantMode {
			t.Errorf("%s: mode %o, want %o", name, header.Mode, wantMode)
		}
	}
	desktop := string(contents["./usr/share/applications/dev.example.notes.desktop"])
	for _, line := range []string{"Exec=fixture-notes", "Icon=fixture-notes", "Categories=Utility;", "Terminal=false", "StartupWMClass=fixture-notes", "Comment=Takes notes"} {
		if !strings.Contains(desktop, line+"\n") {
			t.Errorf("the desktop file lacks %q:\n%s", line, desktop)
		}
	}
}

func TestControlFile(t *testing.T) {
	members := parseAr(t, build(t, sample()))
	names, headers, contents := untar(t, members[1].data)
	if strings.Join(names, " ") != "./ ./control ./md5sums" {
		t.Errorf("control archive = %v", names)
	}
	if headers["./control"].Mode != 0o644 || headers["./control"].Uid != 0 {
		t.Errorf("control header %+v", headers["./control"])
	}
	// 3000 B program = 3 KiB, plus 1 KiB for each of the three smaller files.
	want := "Package: fixture-notes\nVersion: 2.0.1\nArchitecture: amd64\nMaintainer: Ann Lee <ann@example.com>\nInstalled-Size: 6\n" +
		"Depends: libc6 (>= 2.31), libvulkan1 | libvulkan-dev\nRecommends: mesa-vulkan-drivers\nSection: utils\nPriority: optional\nHomepage: https://example.com/notes\n" +
		"Description: Takes notes\n First paragraph.\n .\n Second paragraph.\n"
	if got := string(contents["./control"]); got != want {
		t.Errorf("control\n%q\nwant\n%q", got, want)
	}
}

func TestControlWithoutOptionalFields(t *testing.T) {
	pkg := sample()
	pkg.Depends, pkg.Recommends, pkg.Homepage, pkg.Description, pkg.Comment = nil, nil, "", "", ""
	files, _ := pkg.Files()
	control, err := pkg.Control(files)
	if err != nil {
		t.Fatal(err)
	}
	text := string(control)
	if strings.Contains(text, "Depends:") || strings.Contains(text, "Recommends:") || strings.Contains(text, "Homepage:") || !strings.HasSuffix(text, "Description: Fixture Notes\n") {
		t.Errorf("control:\n%s", text)
	}
}

func TestFoldDescription(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", ""}, {"  \n", ""}, {"one", "\n one"}, {"a\r\nb", "\n a\n b"}, {"a\n\n\nb ", "\n a\n .\n .\n b"},
	} {
		if got := foldDescription(test.in); got != test.want {
			t.Errorf("foldDescription(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestMD5Sums(t *testing.T) {
	members := parseAr(t, build(t, sample()))
	_, _, control := untar(t, members[1].data)
	_, _, data := untar(t, members[2].data)
	lines := strings.Split(strings.TrimSuffix(string(control["./md5sums"]), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("md5sums has %d lines:\n%s", len(lines), control["./md5sums"])
	}
	previous := ""
	for _, line := range lines {
		sum, path, ok := strings.Cut(line, "  ")
		if !ok || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || path <= previous {
			t.Errorf("bad or unsorted line %q", line)
		}
		previous = path
		content := data["./"+path]
		digest := md5.Sum(content)
		if hex.EncodeToString(digest[:]) != sum {
			t.Errorf("%s: wrong sum", path)
		}
	}
}

func TestReproducible(t *testing.T) {
	if !bytes.Equal(build(t, sample()), build(t, sample())) {
		t.Error("two builds differ")
	}
	a := build(t, sample())
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	b := build(t, sample())
	if bytes.Equal(a, b) {
		t.Error("SOURCE_DATE_EPOCH changed nothing")
	}
	if !bytes.Equal(b, build(t, sample())) {
		t.Error("builds with SOURCE_DATE_EPOCH differ")
	}
}

func TestPackageName(t *testing.T) {
	for _, test := range []struct {
		in, want string
		ok       bool
	}{
		{"fixture-notes", "fixture-notes", true}, {"Viva_Gallery", "viva-gallery", true}, {"app2.1", "app2.1", true}, {"c++", "c++", true},
		{"a", "", false}, {"-x", "", false}, {"has space", "", false}, {"", "", false}, {"ü-app", "", false}, {"ab", "ab", true},
	} {
		got, err := PackageName(test.in)
		if (err == nil) != test.ok || got != test.want {
			t.Errorf("PackageName(%q) = %q, %v", test.in, got, err)
		}
	}
	name, err := sample().FileName()
	if err != nil || name != "fixture-notes_2.0.1_amd64.deb" {
		t.Errorf("FileName = %q, %v", name, err)
	}
}

func TestRejectsBadPackages(t *testing.T) {
	for name, change := range map[string]func(*Package){
		"executable": func(p *Package) { p.Executable = "x" },
		"version":    func(p *Package) { p.Version = "v1" },
		"arch":       func(p *Package) { p.Arch = "i386" },
		"id":         func(p *Package) { p.ID = "dev/example" },
		"name":       func(p *Package) { p.Name = "" },
		"program":    func(p *Package) { p.Program = nil },
		"maintainer": func(p *Package) { p.Maintainer = "" },
		"newline":    func(p *Package) { p.Maintainer = "a <b@c>\nInjected: yes" },
		"comment":    func(p *Package) { p.Comment = "a\nb" },
		"homepage":   func(p *Package) { p.Homepage = "not a url" },
		"depends":    func(p *Package) { p.Depends = []string{"a, b"} },
		"recommends": func(p *Package) { p.Recommends = []string{"a, b"} },
		"icon":       func(p *Package) { p.Icons = []linuxicon.Rendition{{Pixels: 48}} },
	} {
		pkg := sample()
		change(&pkg)
		if err := pkg.Write(io.Discard); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestWithoutIcons(t *testing.T) {
	pkg := sample()
	pkg.Icons = nil
	members := parseAr(t, build(t, pkg))
	names, _, contents := untar(t, members[2].data)
	for _, name := range names {
		if strings.Contains(name, "icons") {
			t.Errorf("unexpected %s", name)
		}
	}
	if strings.Contains(string(contents["./usr/share/applications/dev.example.notes.desktop"]), "Icon=") {
		t.Error("the entry names an icon")
	}
}

func TestFileTypes(t *testing.T) {
	pkg := sample()
	pkg.FileTypes = []mimeinfo.Type{{MIME: "text/markdown", Name: "Markdown", Extensions: []string{"md"}}}
	members := parseAr(t, build(t, pkg))
	_, _, contents := untar(t, members[2].data)
	desktop := string(contents["./usr/share/applications/dev.example.notes.desktop"])
	for _, want := range []string{"Exec=fixture-notes %F\n", "MimeType=text/markdown;\n"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("the desktop file lacks %q:\n%s", want, desktop)
		}
	}
	if types := string(contents["./usr/share/mime/packages/dev.example.notes.xml"]); !strings.Contains(types, `<mime-type type="text/markdown">`) {
		t.Errorf("mime package:\n%s", types)
	}
}
