package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/tools/appbundle"
	"github.com/vivaui/viva/tools/manifest"
)

// fixture writes a tiny module with a native main package and a browser one,
// plus the given viva.toml (none when empty), and returns its directory.
func fixture(t *testing.T, toml string) string {
	t.Helper()
	// A developer's own signing setup must not reach any packaging run.
	for _, name := range append(signingVariables, windowsSigningVariables...) {
		t.Setenv(name, "")
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.com/fixture\n\ngo 1.26\n",
		"main.go":     "package main\n\nfunc main() {}\n",
		"web/main.go": "package main\n\nfunc main() {}\n",
	}
	if toml != "" {
		files["viva.toml"] = toml
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const notesTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.1"

[web]
package = "./web"

[build]
tags = ["alpha"]
`

func loadPlan(t *testing.T, dir string, shared buildFlags, pkg string) buildPlan {
	t.Helper()
	shared.dir = dir
	if shared.target == "" {
		shared.target = runtime.GOOS
	}
	if shared.arch == "" {
		shared.arch = runtime.GOARCH
	}
	plan, err := newPlan(shared, pkg)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestLdflagsAreStrippedAndReproducible(t *testing.T) {
	for _, test := range []struct {
		target string
		debug  bool
		want   string
	}{
		{"linux", false, "-s -w -buildid="},
		{"darwin", false, "-s -w -buildid="},
		{"web", false, "-s -w -buildid="},
		{"windows", false, "-s -w -buildid= -H=windowsgui"},
		{"linux", true, "-buildid="},
		{"windows", true, "-buildid="},
	} {
		plan := buildPlan{target: test.target, debug: test.debug}
		if got := plan.ldflags(); got != test.want {
			t.Errorf("%s debug=%v: ldflags %q, want %q", test.target, test.debug, got, test.want)
		}
	}
}

func TestBuildArguments(t *testing.T) {
	dir := fixture(t, notesTOML)
	plan := loadPlan(t, dir, buildFlags{target: "linux", arch: "amd64", tags: "beta, alpha,,gamma"}, "./cmd/other")
	want := []string{"build", "-trimpath", "-ldflags", "-s -w -buildid=", "-tags", "alpha,beta,gamma", "-o", "/out/x", "./cmd/other"}
	if got := plan.arguments("/out/x"); !slices.Equal(got, want) {
		t.Errorf("arguments = %q\nwant        %q", got, want)
	}
	plain := loadPlan(t, fixture(t, ""), buildFlags{target: "linux"}, "")
	if got := plain.arguments("/out/x"); slices.Contains(got, "-tags") {
		t.Errorf("-tags given without tags: %q", got)
	}
}

func TestBuildEnvironment(t *testing.T) {
	dir := fixture(t, notesTOML)
	base := []string{"HOME=/h", "CGO_CFLAGS=-O2 -g", "GOOS=plan9"}

	web := loadPlan(t, dir, buildFlags{target: "web"}, "")
	environment := web.environment(base)
	if got := lastEnvironment(environment, "GOOS"); got != "js" || lastEnvironment(environment, "GOARCH") != "wasm" {
		t.Errorf("web environment: GOOS %q GOARCH %q", got, lastEnvironment(environment, "GOARCH"))
	}
	if lastEnvironment(environment, "CGO_ENABLED") != "" {
		t.Error("the web build changed CGO_ENABLED")
	}

	linux := loadPlan(t, dir, buildFlags{target: "linux", arch: "arm64"}, "")
	environment = linux.environment(base)
	if lastEnvironment(environment, "GOOS") != "linux" || lastEnvironment(environment, "GOARCH") != "arm64" {
		t.Errorf("linux environment: %v", environment)
	}
	if strings.Contains(strings.Join(environment, " "), "mmacosx") {
		t.Error("a Linux build got a macOS deployment target")
	}
	if lastEnvironment(environment, "CGO_CFLAGS") != "-O2 -g" {
		t.Errorf("the caller's CGO_CFLAGS was changed: %q", lastEnvironment(environment, "CGO_CFLAGS"))
	}
	if got := lastEnvironment(environment, "CGO_ENABLED"); got != "0" {
		t.Errorf("CGO_ENABLED = %q: a Linux program must not link the C library of the machine that built it", got)
	}
	// A caller with cgo on is still packaged without it: the package has to be
	// the same whichever machine built it.
	withCgo := append(append([]string(nil), base...), "CGO_ENABLED=1")
	if got := lastEnvironment(linux.environment(withCgo), "CGO_ENABLED"); got != "0" {
		t.Errorf("CGO_ENABLED = %q with the caller's CGO_ENABLED=1", got)
	}

	windows := loadPlan(t, dir, buildFlags{target: "windows", arch: "amd64"}, "")
	environment = windows.environment(base)
	if got := lastEnvironment(environment, "CGO_ENABLED"); got != "0" {
		t.Errorf("CGO_ENABLED = %q: a Windows program must not link the C library of the machine that built it", got)
	}

	if runtime.GOOS == "darwin" {
		mac := loadPlan(t, dir, buildFlags{target: "darwin", arch: "amd64"}, "")
		mac.manifest.Mac.MinSystem = "13.3"
		environment = mac.environment(base)
		if got := lastEnvironment(environment, "CGO_CFLAGS"); got != "-O2 -g -mmacosx-version-min=13.3" {
			t.Errorf("CGO_CFLAGS = %q", got)
		}
		if got := lastEnvironment(environment, "CGO_LDFLAGS"); got != "-mmacosx-version-min=13.3" {
			t.Errorf("CGO_LDFLAGS = %q", got)
		}
		if got := lastEnvironment(environment, "CGO_ENABLED"); got != "1" {
			t.Errorf("CGO_ENABLED = %q: Go turns cgo off for a build of another architecture, and the Metal bridge needs it", got)
		}
	}
}

func TestNewPlanChoosesThePackageTagsAndName(t *testing.T) {
	dir := fixture(t, notesTOML)

	plan := loadPlan(t, dir, buildFlags{target: "linux", tags: "beta"}, "")
	if plan.pkg != "." || !slices.Equal(plan.tags, []string{"alpha", "beta"}) || plan.executable != "fixture-notes" {
		t.Errorf("native plan = %+v", plan)
	}
	web := loadPlan(t, dir, buildFlags{target: "web"}, "")
	if web.pkg != "./web" {
		t.Errorf("web package = %q, want [web] package", web.pkg)
	}
	override := loadPlan(t, dir, buildFlags{target: "linux"}, "./web")
	if override.pkg != "./web" {
		t.Errorf("a package argument was ignored: %q", override.pkg)
	}

	// Without a viva.toml the name comes from the package being built.
	bare := fixture(t, "")
	if plan := loadPlan(t, bare, buildFlags{target: "linux"}, "./cmd/Image Viewer"); plan.executable != "image-viewer" {
		t.Errorf("executable from a package path = %q", plan.executable)
	}
	if plan := loadPlan(t, bare, buildFlags{target: "linux"}, ""); plan.executable != manifest.Slug(filepath.Base(bare)) {
		t.Errorf("executable from the directory = %q", plan.executable)
	}
}

func TestNewPlanRejectsWhatCannotBeBuilt(t *testing.T) {
	dir := fixture(t, "")
	for _, test := range []struct {
		name   string
		shared buildFlags
		want   string
	}{
		{"unknown target", buildFlags{dir: dir, target: "plan9", arch: "arm64"}, `unknown target "plan9"`},
		{"unknown arch", buildFlags{dir: dir, target: "linux", arch: "riscv"}, `unknown architecture "riscv"`},
		{"web without an entry point", buildFlags{dir: dir, target: "web", arch: "arm64"}, "[web] package"},
	} {
		if _, err := newPlan(test.shared, ""); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v", test.name, err)
		}
	}
	if runtime.GOOS != "darwin" {
		if _, err := newPlan(buildFlags{dir: dir, target: "darwin", arch: runtime.GOARCH}, ""); err == nil || !strings.Contains(err.Error(), "macOS host") {
			t.Errorf("darwin from %s: %v", runtime.GOOS, err)
		}
	}
	// A web build ignores the architecture rather than rejecting it.
	web := fixture(t, "[web]\npackage = \"./web\"\n")
	if _, err := newPlan(buildFlags{dir: web, target: "web", arch: "riscv"}, ""); err != nil {
		t.Errorf("web with an irrelevant arch: %v", err)
	}
}

func TestOutputPaths(t *testing.T) {
	dir := fixture(t, notesTOML)
	host := func(target string) buildPlan {
		return loadPlan(t, dir, buildFlags{target: target}, "")
	}
	absolute := func(path string) string {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		return abs
	}

	// Defaults land in [build] out, named for the executable, with the target in
	// the name when it is not the machine's own.
	native := loadPlan(t, dir, buildFlags{target: runtime.GOOS, arch: runtime.GOARCH}, "")
	nativeName := "fixture-notes"
	if runtime.GOOS == "windows" {
		nativeName += ".exe"
	}
	if err := native.outputPath(""); err != nil || native.out != absolute(filepath.Join(dir, "build", nativeName)) {
		t.Errorf("host default = %q, %v", native.out, err)
	}
	cross := loadPlan(t, dir, buildFlags{target: "windows", arch: "amd64"}, "")
	windowsName := "fixture-notes-windows-amd64.exe"
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		windowsName = "fixture-notes.exe"
	}
	if err := cross.outputPath(""); err != nil || filepath.Base(cross.out) != windowsName {
		t.Errorf("windows default = %q, %v", cross.out, err)
	}
	// An architecture other than the host's always puts the target in the name.
	other := "amd64"
	if runtime.GOARCH == "amd64" {
		other = "arm64"
	}
	linux := loadPlan(t, dir, buildFlags{target: "linux", arch: other}, "")
	if err := linux.outputPath(""); err != nil || filepath.Base(linux.out) != "fixture-notes-linux-"+other {
		t.Errorf("linux default = %q, %v", linux.out, err)
	}
	web := host("web")
	if err := web.outputPath(""); err != nil || web.out != absolute(filepath.Join(dir, "build", "web")) {
		t.Errorf("web default = %q, %v", web.out, err)
	}

	// -o names a file, or a directory that exists to put the program in.
	file := filepath.Join(t.TempDir(), "program")
	linux = loadPlan(t, dir, buildFlags{target: "linux", arch: other}, "")
	if err := linux.outputPath(file); err != nil || linux.out != file {
		t.Errorf("-o file = %q, %v", linux.out, err)
	}
	directory := t.TempDir()
	if err := linux.outputPath(directory); err != nil || linux.out != filepath.Join(directory, "fixture-notes-linux-"+other) {
		t.Errorf("-o directory = %q, %v", linux.out, err)
	}
	if err := web.outputPath(directory); err != nil || web.out != directory {
		t.Errorf("web -o = %q, %v", web.out, err)
	}
}

func TestFormatSizeAndNotes(t *testing.T) {
	for size, want := range map[int64]string{
		0: "0 B (0.00 MiB)", 999: "999 B (0.00 MiB)", 1000: "1,000 B (0.00 MiB)",
		3340578: "3,340,578 B (3.19 MiB)", 1 << 30: "1,073,741,824 B (1024.00 MiB)",
	} {
		if got := formatSize(size); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", size, got, want)
		}
	}
	release, debug := buildPlan{}, buildPlan{debug: true}
	for _, test := range []struct {
		plan buildPlan
		size int64
		want string
	}{
		{release, 1 * mebibyte, ", stripped"},
		{release, 40 * mebibyte, ", stripped"},
		{debug, 9 * mebibyte, ", with symbols (not the release size)"},
	} {
		if got := sizeNote(test.plan, test.size); !strings.HasPrefix(got, test.want) {
			t.Errorf("sizeNote(debug=%v, %d) = %q, want %q", test.plan.debug, test.size, got, test.want)
		}
	}
}

// The tests below run the go tool on a one-file module.

func TestBuildProducesAStrippedReproducibleProgram(t *testing.T) {
	dir := fixture(t, notesTOML)
	build := func(extra ...string) (string, string) {
		out := filepath.Join(t.TempDir(), "program")
		var output bytes.Buffer
		arguments := append([]string{"build", "-C", dir, "-o", out}, extra...)
		if err := runCLI(arguments, &output, io.Discard); err != nil {
			t.Fatalf("viva %v: %v", arguments, err)
		}
		return out, output.String()
	}

	first, report := build()
	second, _ := build()
	a, errA := os.ReadFile(first)
	b, errB := os.ReadFile(second)
	if errA != nil || errB != nil || !bytes.Equal(a, b) {
		t.Fatal("two builds of the same source differ")
	}
	info, _ := os.Stat(first)
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Error("the program is not executable")
	}
	if runtime.GOOS == "windows" && !bytes.HasPrefix(a, []byte("MZ")) {
		t.Error("the program is not a Windows executable")
	}
	for _, want := range []string{"built ", "  target   " + runtime.GOOS + "/" + runtime.GOARCH, "  tags     alpha", "  size     ", ", stripped"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q:\n%s", want, report)
		}
	}

	debug, debugReport := build("--debug")
	debugInfo, _ := os.Stat(debug)
	if debugInfo.Size() <= info.Size() {
		t.Errorf("a debug build (%d B) is not larger than the release build (%d B)", debugInfo.Size(), info.Size())
	}
	if !strings.Contains(debugReport, "with symbols") {
		t.Errorf("the debug report does not say so:\n%s", debugReport)
	}
}

func TestBuildReportsCompilerErrorsAndFails(t *testing.T) {
	dir := fixture(t, "")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { undefinedName() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errorOutput bytes.Buffer
	err := runCLI([]string{"build", "-C", dir, "-o", filepath.Join(t.TempDir(), "p")}, io.Discard, &errorOutput)
	if err == nil || !strings.Contains(errorOutput.String(), "undefinedName") {
		t.Fatalf("err %v, compiler output %q", err, errorOutput.String())
	}
}

func TestBuildCrossCompilesForWindowsAndLinux(t *testing.T) {
	dir := fixture(t, "")
	for _, test := range []struct {
		target, arch string
		magic        string
	}{
		{"linux", "amd64", "\x7fELF"},
		{"windows", "arm64", "MZ"},
	} {
		out := t.TempDir()
		var output bytes.Buffer
		arguments := []string{"build", "-C", dir, "--target", test.target, "--arch", test.arch, "-o", out}
		if err := runCLI(arguments, &output, io.Discard); err != nil {
			t.Fatalf("viva %v: %v", arguments, err)
		}
		name := manifest.Slug(filepath.Base(dir))
		if test.target != runtime.GOOS || test.arch != runtime.GOARCH {
			name += "-" + test.target + "-" + test.arch
		}
		if test.target == "windows" {
			name += ".exe"
		}
		entries, _ := os.ReadDir(out)
		if len(entries) != 1 || entries[0].Name() != name {
			t.Fatalf("%s: output directory holds %v", test.target, entries)
		}
		data, _ := os.ReadFile(filepath.Join(out, entries[0].Name()))
		if !strings.HasPrefix(string(data), test.magic) {
			t.Errorf("%s/%s: the file does not start with %q", test.target, test.arch, test.magic)
		}
	}
}

func TestBuildWebWritesTheLoaderAndAPage(t *testing.T) {
	dir := fixture(t, "[app]\nname = \"A <b> & \\\"C\\\"\"\nid = \"dev.example.a\"\n\n[web]\npackage = \"./web\"\n")
	out := filepath.Join(t.TempDir(), "site")
	var output bytes.Buffer
	if err := runCLI([]string{"build", "-C", dir, "--target", "web", "-o", out}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a-b-c.wasm", "wasm_exec.js", "index.html"} {
		if info, err := os.Stat(filepath.Join(out, name)); err != nil || info.Size() == 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
	wasm, _ := os.ReadFile(filepath.Join(out, "a-b-c.wasm"))
	if !bytes.HasPrefix(wasm, []byte("\x00asm")) {
		t.Error("the program is not WebAssembly")
	}
	page, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !strings.Contains(string(page), `fetch("a-b-c.wasm")`) {
		t.Errorf("the page does not fetch the program:\n%s", page)
	}
	if strings.Contains(string(page), "<b>") || !strings.Contains(string(page), "&lt;b&gt;") {
		t.Errorf("the title was not escaped:\n%s", page)
	}
	for _, want := range []string{"  files    a-b-c.wasm, wasm_exec.js, index.html", "  gzip     "} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, output.String())
		}
	}
}

// The deployment target is what keeps a program from starting on an older macOS,
// and it only exists for programs with C in them, as the Metal bridge has.
func TestMacBuildsAreCompiledForMinSystem(t *testing.T) {
	requireMac(t)
	for _, test := range []struct{ toml, want string }{
		{"", "12.0"},
		{"[mac]\nmin_system = \"13.3\"\n", "13.3"},
		{"[mac]\nmin_system = \"14.0\"\n", "14.0"},
	} {
		dir := fixture(t, test.toml)
		source := "package main\n\n/*\nstatic int answer(void) { return 42; }\n*/\nimport \"C\"\n\nfunc main() { _ = C.answer() }\n"
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "program")
		if err := runCLI([]string{"build", "-C", dir, "-o", out}, io.Discard, io.Discard); err != nil {
			t.Fatalf("min_system %q: %v", test.want, err)
		}
		got, err := appbundle.RequiredMacOS(out)
		if err != nil || got != test.want {
			t.Errorf("a program built for min_system %s records macOS %q (%v)", test.want, got, err)
		}
	}
}

// Go turns cgo off when GOARCH differs from the host's; the C in a Mac program
// must still be built.
func TestMacBuildsForTheOtherArchitecture(t *testing.T) {
	requireMac(t)
	other := "amd64"
	if runtime.GOARCH == "amd64" {
		other = "arm64"
	}
	dir := fixture(t, "")
	source := "package main\n\n/*\nstatic int answer(void) { return 42; }\n*/\nimport \"C\"\n\nfunc main() { _ = C.answer() }\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "program")
	if err := runCLI([]string{"build", "-C", dir, "--arch", other, "-o", out}, io.Discard, io.Discard); err != nil {
		t.Fatalf("building for darwin/%s: %v", other, err)
	}
	if output, err := exec.Command("file", out).Output(); err != nil || !strings.Contains(string(output), map[string]string{"amd64": "x86_64", "arm64": "arm64"}[other]) {
		t.Errorf("file says %q (%v)", output, err)
	}
}
