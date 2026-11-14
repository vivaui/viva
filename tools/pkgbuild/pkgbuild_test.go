package pkgbuild

import (
	"os/exec"
	"strings"
	"testing"
)

func sample() Package {
	return Package{
		Executable: "Fixture-Notes", ID: "dev.example.notes", Name: "Fixture Notes", Version: "2.0.1", Arch: "amd64",
		Maintainer: "Ann Lee <ann@example.com>", Homepage: "https://example.com/notes",
		Comment:    "Takes notes, doesn't lose them",
		Depends:    []string{"vulkan-icd-loader", "libxcb", "cairo"},
		Archive:    "Fixture-Notes-2.0.1-linux-amd64.tar.gz",
		SHA256:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		IconPixels: 256,
	}
}

func TestPackageName(t *testing.T) {
	name, err := PackageName("Fixture-Notes")
	if err != nil {
		t.Fatal(err)
	}
	if name != "fixture-notes-bin" {
		t.Fatalf("name = %q", name)
	}
	for _, bad := range []string{"", "-notes", ".notes", "notes app", "notes/app"} {
		if _, err := PackageName(bad); err == nil {
			t.Errorf("PackageName(%q) accepted", bad)
		}
	}
}

func TestArchName(t *testing.T) {
	cases := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}
	for input, want := range cases {
		if got, err := ArchName(input); err != nil || got != want {
			t.Errorf("ArchName(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ArchName("386"); err == nil {
		t.Error("ArchName(386) accepted")
	}
}

func TestPKGBUILD(t *testing.T) {
	data, err := sample().PKGBUILD()
	if err != nil {
		t.Fatal(err)
	}
	pkgbuild := string(data)
	for _, want := range []string{
		"pkgname='fixture-notes-bin'",
		"pkgver='2.0.1'",
		"pkgrel=1",
		`pkgdesc='Takes notes, doesn'\''t lose them'`,
		"arch=('x86_64')",
		"url='https://example.com/notes'",
		"license=('custom')",
		"depends=('vulkan-icd-loader' 'libxcb' 'cairo')",
		"provides=('Fixture-Notes')",
		"conflicts=('Fixture-Notes')",
		"options=('!strip' '!debug')",
		"_archive='Fixture-Notes-2.0.1-linux-amd64.tar.gz'",
		"_icon_size=256",
		"source=('Fixture-Notes-2.0.1-linux-amd64.tar.gz')",
		"sha256sums=('0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef')",
		`install -dm755 "$pkgdir/opt/$pkgname"`,
		`bsdtar -xf "$srcdir/$_archive" -C "$pkgdir/opt/$pkgname" --strip-components=1`,
		`ln -s "/opt/$pkgname/$_exe" "$pkgdir/usr/bin/$_exe"`,
		`$pkgdir/usr/share/applications/$_desktop.desktop`,
		`$pkgdir/usr/share/icons/hicolor/${_icon_size}x${_icon_size}/apps/$_exe.png`,
		`$pkgdir/usr/share/mime/packages/$_desktop.xml`,
		`$pkgdir/usr/share/doc/$pkgname/README.txt`,
	} {
		if !strings.Contains(pkgbuild, want) {
			t.Errorf("PKGBUILD does not contain %q:\n%s", want, pkgbuild)
		}
	}
}

func TestPKGBUILDQuotesShellValues(t *testing.T) {
	pkg := sample()
	pkg.Comment = `Ann's "notes" $HOME ` + "`x`"
	data, err := pkg.PKGBUILD()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `pkgdesc='Ann'\''s "notes" $HOME `+"`x`'") {
		t.Fatalf("description is not single-quoted safely:\n%s", data)
	}
}

func TestPKGBUILDSourceURL(t *testing.T) {
	pkg := sample()
	pkg.SourceURL = "https://example.com/downloads/Fixture-Notes-2.0.1-linux-amd64.tar.gz"
	data, err := pkg.PKGBUILD()
	if err != nil {
		t.Fatal(err)
	}
	want := "source=('Fixture-Notes-2.0.1-linux-amd64.tar.gz::https://example.com/downloads/Fixture-Notes-2.0.1-linux-amd64.tar.gz')"
	if !strings.Contains(string(data), want) {
		t.Fatalf("PKGBUILD does not contain %q:\n%s", want, data)
	}
}

func TestPKGBUILDWithoutIcon(t *testing.T) {
	pkg := sample()
	pkg.IconPixels = 0
	data, err := pkg.PKGBUILD()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `if [ "$_icon_size" -gt 0 ]; then`) {
		t.Fatalf("no icon guard:\n%s", data)
	}
}

func TestPKGBUILDSyntax(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}
	data, err := sample().PKGBUILD()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(bash, "-n")
	command.Stdin = strings.NewReader(string(data))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s\n%s", err, output, data)
	}
}

func TestSRCINFO(t *testing.T) {
	data, err := sample().SRCINFO()
	if err != nil {
		t.Fatal(err)
	}
	srcinfo := string(data)
	for _, want := range []string{
		"pkgbase = fixture-notes-bin",
		"\tpkgdesc = Takes notes, doesn't lose them",
		"\tpkgver = 2.0.1",
		"\tpkgrel = 1",
		"\turl = https://example.com/notes",
		"\tarch = x86_64",
		"\tlicense = custom",
		"\tdepends = vulkan-icd-loader",
		"\tprovides = Fixture-Notes",
		"\tconflicts = Fixture-Notes",
		"\toptions = !strip",
		"\toptions = !debug",
		"\tsource = Fixture-Notes-2.0.1-linux-amd64.tar.gz",
		"\tsha256sums = 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"pkgname = fixture-notes-bin",
	} {
		if !strings.Contains(srcinfo, want) {
			t.Errorf("SRCINFO does not contain %q:\n%s", want, srcinfo)
		}
	}
}

func TestRejectsBadPackages(t *testing.T) {
	tests := map[string]func(*Package){
		"empty name":       func(p *Package) { p.Name = "" },
		"version hyphen":   func(p *Package) { p.Version = "2.0.1-beta" },
		"empty archive":    func(p *Package) { p.Archive = "" },
		"archive path":     func(p *Package) { p.Archive = "dir/app.tar.gz" },
		"short digest":     func(p *Package) { p.SHA256 = "abc" },
		"uppercase digest": func(p *Package) { p.SHA256 = strings.ToUpper(p.SHA256) },
		"arch":             func(p *Package) { p.Arch = "386" },
		"bad id":           func(p *Package) { p.ID = "a/b" },
		"multiline":        func(p *Package) { p.Comment = "one\ntwo" },
		"bad maintainer":   func(p *Package) { p.Maintainer = "two\nlines" },
		"bad source":       func(p *Package) { p.SourceURL = "ftp://example.com/app.tar.gz" },
		"bad dependency":   func(p *Package) { p.Depends = []string{"ok", ""} },
		"negative icon":    func(p *Package) { p.IconPixels = -1 },
	}
	for name, change := range tests {
		pkg := sample()
		change(&pkg)
		if _, err := pkg.PKGBUILD(); err == nil {
			t.Errorf("%s: PKGBUILD accepted", name)
		}
		if _, err := pkg.SRCINFO(); err == nil {
			t.Errorf("%s: SRCINFO accepted", name)
		}
	}
}
