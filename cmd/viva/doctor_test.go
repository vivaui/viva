package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVivaDependency(t *testing.T) {
	for name, test := range map[string]struct {
		goMod string
		want  vivaDependency
	}{
		"block": {"module x\n\nrequire (\n\tgithub.com/vivaui/viva v0.1.2\n\tgithub.com/other v1.0.0 // indirect\n)\n",
			vivaDependency{Version: "v0.1.2"}},
		"single line with a tool": {"module x\n\nrequire github.com/vivaui/viva v0.1.1\n\ntool github.com/vivaui/viva/cmd/viva\n",
			vivaDependency{Version: "v0.1.1", Tool: true}},
		"tool block": {"module x\n\ntool (\n\tgithub.com/vivaui/viva/cmd/viva\n)\nrequire github.com/vivaui/viva v0.1.1\n",
			vivaDependency{Version: "v0.1.1", Tool: true}},
		"replaced": {"module x\n\nrequire github.com/vivaui/viva v0.0.0\n\nreplace github.com/vivaui/viva => ../viva\n",
			vivaDependency{Version: "v0.0.0", Replaced: true}},
		"similar module is not viva": {"module x\n\nrequire github.com/vivaui/viva-extra v1.0.0\n", vivaDependency{}},
		"not required":               {"module x\n\ngo 1.26\n", vivaDependency{}},
	} {
		if got := parseVivaDependency(test.goMod); got != test.want {
			t.Errorf("%s: got %+v, want %+v", name, got, test.want)
		}
	}
}

// fakeHost answers for a machine: the commands it has and what they print.
func fakeHost(goos string, have map[string]string, captured map[string]string) doctorHost {
	return doctorHost{
		goos: goos, goarch: "arm64",
		lookPath: func(name string) (string, error) {
			if path, ok := have[name]; ok {
				return path, nil
			}
			return "", errors.New("not found")
		},
		capture: func(name string, arguments ...string) (string, error) {
			if out, ok := captured[name+" "+strings.Join(arguments, " ")]; ok {
				return out, nil
			}
			return "", errors.New("exit status 1")
		},
		getenv:   func(string) string { return "" },
		readFile: os.ReadFile,
	}
}

func results(host doctorHost, directory string) map[string]checkResult {
	byName := map[string]checkResult{}
	for _, result := range host.check(directory) {
		byName[result.name] = result
	}
	return byName
}

func TestDoctorOnAMacWithoutCommandLineTools(t *testing.T) {
	host := fakeHost("darwin", map[string]string{"go": "/usr/local/go/bin/go"},
		map[string]string{"/usr/local/go/bin/go env GOVERSION": "go1.26.1\n"})
	got := results(host, t.TempDir())
	if got["go"].level != checkOK {
		t.Errorf("go: %+v", got["go"])
	}
	if got["xcode"].level != checkFail || !strings.Contains(got["xcode"].fix, "xcode-select --install") {
		t.Errorf("a Mac without the command-line tools must fail with the fix: %+v", got["xcode"])
	}
	var out bytes.Buffer
	if err := doctor(host, t.TempDir(), &out); err == nil {
		t.Error("doctor passed on a machine that cannot build")
	}
	if !strings.Contains(out.String(), "xcode-select --install") {
		t.Errorf("the fix was not printed:\n%s", out.String())
	}
}

func TestDoctorIsHappyOnAReadyMachine(t *testing.T) {
	host := fakeHost("darwin", map[string]string{"go": "/g/go", "clang": "/usr/bin/clang", "hdiutil": "/usr/bin/hdiutil", "codesign": "/usr/bin/codesign"},
		map[string]string{"/g/go env GOVERSION": "go1.27.0", "xcode-select -p": "/Library/Developer/CommandLineTools\n"})
	var out bytes.Buffer
	if err := doctor(host, t.TempDir(), &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "FAIL") {
		t.Errorf("\n%s", out.String())
	}
}

func TestDoctorTreatsAMissingGoAsFixableUnlessDownloadingIsOff(t *testing.T) {
	host := fakeHost("windows", nil, nil)
	host.cacheDir = t.TempDir()
	if got := results(host, t.TempDir())["go"]; got.level != checkNote || !strings.Contains(got.detail, "downloads") {
		t.Errorf("no Go: %+v", got)
	}
	host.getenv = func(name string) string {
		if name == "VIVA_GO_DOWNLOAD" {
			return "0"
		}
		return ""
	}
	if got := results(host, t.TempDir())["go"]; got.level != checkFail {
		t.Errorf("no Go and no downloading must fail: %+v", got)
	}
}

func TestDoctorNotesAnOldGoThatVivaWillReplace(t *testing.T) {
	host := fakeHost("linux", map[string]string{"go": "/g/go"}, map[string]string{"/g/go env GOVERSION": "go1.22.0"})
	host.cacheDir = t.TempDir()
	if got := results(host, t.TempDir())["go"]; got.level != checkNote || !strings.Contains(got.detail, "too old") {
		t.Errorf("%+v", got)
	}
}

func TestDoctorChecksLinuxLibraries(t *testing.T) {
	host := fakeHost("linux", map[string]string{"go": "/g/go"}, map[string]string{
		"/g/go env GOVERSION": "go1.26.0",
		"ldconfig -p":         "\tlibxcb.so.1 (libc6,x86-64) => /lib/libxcb.so.1\n",
	})
	got := results(host, t.TempDir())
	if got["libxcb.so.1"].level != checkOK {
		t.Errorf("%+v", got["libxcb.so.1"])
	}
	if vulkan := got["libvulkan.so.1"]; vulkan.level != checkNote || !strings.Contains(vulkan.detail, "CPU") || !strings.Contains(vulkan.fix, "apt install") {
		t.Errorf("%+v", vulkan)
	}
	var out bytes.Buffer
	if err := doctor(host, t.TempDir(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "optional") {
		t.Errorf("the optional GPU note is not actionable:\n%s", out.String())
	}
}

func TestDoctorChecksTheProject(t *testing.T) {
	directory := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module app\n\ngo 1.26\n\nrequire github.com/vivaui/viva v0.1.1\n")
	write("viva.toml", "[app\nname = \n")
	host := fakeHost("windows", map[string]string{"go": "/g/go"}, map[string]string{"/g/go env GOVERSION": "go1.26.0"})
	got := results(host, directory)
	if got["viva.toml"].level != checkFail {
		t.Errorf("a broken viva.toml must fail: %+v", got["viva.toml"])
	}

	write("viva.toml", "[app]\nname = \"App\"\n")
	if got := results(host, directory)["viva.toml"]; got.level != checkOK {
		t.Errorf("a valid viva.toml: %+v", got)
	}

	// The command and the library the project uses must be the same version.
	previous := version
	t.Cleanup(func() { version = previous })
	version = "v0.1.2"
	mismatch := results(host, directory)["viva lib"]
	if mismatch.level != checkWarn || !strings.Contains(mismatch.fix, "go get -tool") {
		t.Errorf("mismatch: %+v", mismatch)
	}
	write("go.mod", "module app\n\ngo 1.26\n\nrequire github.com/vivaui/viva v0.1.1\n\ntool github.com/vivaui/viva/cmd/viva\n")
	if fix := results(host, directory)["viva lib"].fix; !strings.Contains(fix, "go tool viva") {
		t.Errorf("a pinned project is told to use its tool: %q", fix)
	}
	version = "v0.1.1"
	if got := results(host, directory)["viva lib"]; got.level != checkOK {
		t.Errorf("matching versions: %+v", got)
	}
}

func TestAddWarnsWhenTheProjectUsesAnotherVivaVersion(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module app\n\nrequire github.com/vivaui/viva v0.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	previous := version
	t.Cleanup(func() { version = previous })
	version = "v0.2.0"
	var output, warnings bytes.Buffer
	if err := addComponents([]string{"--out", "ui", "button"}, &output, &warnings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warnings.String(), "v0.1.1") || !strings.Contains(warnings.String(), "v0.2.0") {
		t.Errorf("no warning about the mismatch:\n%s", warnings.String())
	}
	if _, err := os.Stat(filepath.Join(directory, "ui", "button.go")); err != nil {
		t.Errorf("the warning must not stop the copy: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	previous := version
	t.Cleanup(func() { version = previous })
	version = "v9.9.9"
	for _, arguments := range [][]string{{"version"}, {"--version"}} {
		var output bytes.Buffer
		if err := run(arguments, &output); err != nil || !strings.HasPrefix(output.String(), "viva v9.9.9 (") {
			t.Errorf("%v: %v %q", arguments, err, output.String())
		}
	}
}

func TestReleaseVersion(t *testing.T) {
	for v, want := range map[string]bool{
		"v0.1.2": true, "v1.20.3": true, "v0.2.0-rc.1": true,
		"devel": false, "": false, "v0.1.3-0.20261028045640-b491d88cbece": false,
		"v0.1.3-0.20261028045640-b491d88cbece+dirty": false, "v0.1.2+dirty": false, "v1.2": false, "0.1.2": false,
	} {
		if got := releaseVersion(v); got != want {
			t.Errorf("releaseVersion(%q) = %v, want %v", v, got, want)
		}
	}
}

// A build from a checkout has no release to match, so it must not nag.
func TestADevelopmentBuildDoesNotWarnAboutVersions(t *testing.T) {
	previous := version
	t.Cleanup(func() { version = previous })
	version = "v0.1.3-0.20261028045640-b491d88cbece+dirty"
	if got := vivaVersionCheck(vivaDependency{Version: "v0.1.2"}); got.level != checkOK {
		t.Errorf("%+v", got)
	}
}
