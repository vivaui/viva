//go:build darwin

package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestBundleRootFor(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		wantBundle string
		wantErr    bool
	}{
		{
			name:       "bundle with MacOS directory",
			executable: "My App.app/Contents/MacOS/My App",
			wantBundle: "My App.app",
		},
		{
			name:       "executable nested below MacOS",
			executable: "My App.app/Contents/MacOS/Helpers/My App",
			wantBundle: "My App.app",
		},
		{
			name:       "directory that is not a bundle",
			executable: "My App/Contents/MacOS/My App",
			wantErr:    true,
		},
		{
			name:       "bundle without MacOS directory",
			executable: "My App.app/Contents/Resources/helper",
			wantErr:    true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			executable := filepath.Join(root, filepath.FromSlash(test.executable))
			if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := bundleRootFor(executable)
			if test.wantErr {
				if err == nil {
					t.Fatalf("bundleRootFor(%q) = %q, want an error", executable, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("bundleRootFor(%q): %v", executable, err)
			}
			want := filepath.Join(evalSymlinks(t, root), filepath.FromSlash(test.wantBundle))
			if got != want {
				t.Errorf("bundleRootFor(%q) = %q, want %q", executable, got, want)
			}
		})
	}
}

// evalSymlinks returns the resolved path, as bundleRootFor reports it.
func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestInstallStagedBundle(t *testing.T) {
	t.Run("one bundle", func(t *testing.T) {
		staging := t.TempDir()
		want := filepath.Join(staging, "My App.app")
		if err := os.MkdirAll(filepath.Join(want, "Contents", "MacOS"), 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := stagedBundle(staging)
		if err != nil {
			t.Fatalf("stagedBundle: %v", err)
		}
		if got != want {
			t.Errorf("stagedBundle = %q, want %q", got, want)
		}
	})
	t.Run("no bundle", func(t *testing.T) {
		if _, err := stagedBundle(t.TempDir()); !errors.Is(err, ErrManifest) {
			t.Errorf("error %v does not wrap ErrManifest", err)
		}
	})
	t.Run("two bundles", func(t *testing.T) {
		staging := t.TempDir()
		for _, name := range []string{"One.app", "Two.app"} {
			if err := os.Mkdir(filepath.Join(staging, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := stagedBundle(staging); !errors.Is(err, ErrManifest) {
			t.Errorf("error %v does not wrap ErrManifest", err)
		}
	})
	t.Run("file with app suffix", func(t *testing.T) {
		staging := t.TempDir()
		if err := os.WriteFile(filepath.Join(staging, "Fake.app"), []byte("no"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := stagedBundle(staging); !errors.Is(err, ErrManifest) {
			t.Errorf("error %v does not wrap ErrManifest", err)
		}
	})
}

const fixtureInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>dev.viva.gallery</string>
	<key>CFBundleShortVersionString</key>
	<string>1.2.3</string>
</dict>
</plist>
`

// writeInfoPlist builds a minimal application with body as its Info.plist.
func writeInfoPlist(t *testing.T, body string) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), "My App.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestInstallReadBundleInfo(t *testing.T) {
	t.Run("identifier and version", func(t *testing.T) {
		info, err := readBundleInfo(writeInfoPlist(t, fixtureInfoPlist))
		if err != nil {
			t.Fatalf("readBundleInfo: %v", err)
		}
		if info.Identifier != "dev.viva.gallery" {
			t.Errorf("Identifier = %q", info.Identifier)
		}
		if info.Version != "1.2.3" {
			t.Errorf("Version = %q", info.Version)
		}
	})
	t.Run("nested entries are ignored", func(t *testing.T) {
		const nested = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>dev.viva.gallery</string>
	<key>CFBundleDocumentTypes</key>
	<dict>
		<key>CFBundleShortVersionString</key>
		<string>9.9.9</string>
	</dict>
	<key>CFBundleShortVersionString</key>
	<string>1.2.3</string>
</dict>
</plist>
`
		info, err := readBundleInfo(writeInfoPlist(t, nested))
		if err != nil {
			t.Fatalf("readBundleInfo: %v", err)
		}
		if info.Version != "1.2.3" {
			t.Errorf("Version = %q, want the top-level entry", info.Version)
		}
	})
	t.Run("missing plist", func(t *testing.T) {
		if _, err := readBundleInfo(t.TempDir()); !errors.Is(err, ErrManifest) {
			t.Errorf("error %v does not wrap ErrManifest", err)
		}
	})
	t.Run("version is optional", func(t *testing.T) {
		const noVersion = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>dev.viva.gallery</string>
</dict>
</plist>
`
		info, err := readBundleInfo(writeInfoPlist(t, noVersion))
		if err != nil {
			t.Fatalf("readBundleInfo: %v", err)
		}
		if info.Identifier != "dev.viva.gallery" || info.Version != "" {
			t.Errorf("info = %+v", info)
		}
	})
	t.Run("malformed plist", func(t *testing.T) {
		app := writeInfoPlist(t, "<plist><dict><key>CFBundleIdentifier</key><string>broken</plist>")
		if _, err := readBundleInfo(app); !errors.Is(err, ErrManifest) {
			t.Errorf("error %v does not wrap ErrManifest", err)
		}
	})
	t.Run("binary plist", func(t *testing.T) {
		if _, err := exec.LookPath("plutil"); err != nil {
			t.Skip("plutil is a macOS tool")
		}
		app := writeInfoPlist(t, fixtureInfoPlist)
		path := filepath.Join(app, "Contents", "Info.plist")
		if output, err := exec.Command("plutil", "-convert", "binary1", path).CombinedOutput(); err != nil {
			t.Fatalf("plutil: %v\n%s", err, output)
		}
		info, err := readBundleInfo(app)
		if err != nil {
			t.Fatalf("readBundleInfo: %v", err)
		}
		if info.Identifier != "dev.viva.gallery" || info.Version != "1.2.3" {
			t.Errorf("info = %+v", info)
		}
	})
}

func TestInstallHelperScript(t *testing.T) {
	script := helperScript(replacementPlan{
		ParentPID: 4242,
		Bundle:    "/Applications/My App.app",
		Staged:    "/Applications/.viva-update-abc/My App.app",
		Staging:   "/Applications/.viva-update-abc",
		Args:      []string{"--profile", "a b", "it's"},
	})
	wants := []string{
		"#!/bin/sh",
		"parent=4242",
		"bundle='/Applications/My App.app'",
		"staged='/Applications/.viva-update-abc/My App.app'",
		"backup='/Applications/My App.app.old'",
		"staging='/Applications/.viva-update-abc'",
		`while kill -0 "$parent" 2>/dev/null; do`,
		`[ "$waited" -ge 60 ]`,
		`sleep 1`,
		`mv "$bundle" "$backup"`,
		`mv "$staged" "$bundle"`,
		`mv "$backup" "$bundle"`,
		`open -n "$bundle" --args '--profile' 'a b' 'it'\''s'`,
		`rm -rf "$backup"`,
	}
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Errorf("helper script lacks %q", want)
		}
	}
}

func TestInstallHelperScriptWithoutArguments(t *testing.T) {
	script := helperScript(replacementPlan{
		ParentPID: 1,
		Bundle:    "/Applications/My App.app",
		Staged:    "/Applications/.viva-update-abc/My App.app",
		Staging:   "/Applications/.viva-update-abc",
	})
	if !strings.Contains(script, `if ! open -n "$bundle"; then`) {
		t.Error("helper script does not launch the bundle without arguments")
	}
	if strings.Contains(script, "--args") {
		t.Error("helper script passes --args without arguments")
	}
}

// TestInstallHelperScriptParses checks the generated shell syntax only: the
// script is never run, because it would replace the test bundle.
func TestInstallHelperScriptParses(t *testing.T) {
	dir := t.TempDir()
	script := helperScript(replacementPlan{
		ParentPID: 4242,
		Bundle:    filepath.Join(dir, "My App.app"),
		Staged:    filepath.Join(dir, "stage", "My App.app"),
		Staging:   filepath.Join(dir, "stage"),
		Args:      []string{"a b", "it's"},
	})
	path := filepath.Join(dir, "helper.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("/bin/sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("the helper script does not parse: %v\n%s", err, output)
	}
}

func TestInstallWritesHelperScript(t *testing.T) {
	dir := t.TempDir()
	path, err := writeHelperScript(dir, "#!/bin/sh\nexit 0\n")
	if err != nil {
		t.Fatalf("writeHelperScript: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("mode = %v, want 0700", info.Mode().Perm())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "#!/bin/sh\nexit 0\n" {
		t.Errorf("body = %q", body)
	}
}

func TestInstallRefusesANonZipAsset(t *testing.T) {
	called := false
	err := install(InstallOptions{
		Version: "1.2.3",
		Asset:   Asset{Format: FormatNSIS},
	}, func() { called = true })
	if !errors.Is(err, ErrManifest) {
		t.Fatalf("error %v does not wrap ErrManifest", err)
	}
	if called {
		t.Error("quit was called after a failed install")
	}
}

func TestInstallRejectsATranslocatedBundle(t *testing.T) {
	translocated := "/private/var/folders/xy/abc/T/AppTranslocation/1234/d/My App.app"
	if err := checkBundleLocation(translocated); !errors.Is(err, ErrNotWritable) {
		t.Errorf("error %v does not wrap ErrNotWritable", err)
	}
	if err := checkBundleLocation("/Applications/My App.app"); err != nil {
		t.Errorf("an installed bundle was refused: %v", err)
	}
}

func TestInstallCodesignCommand(t *testing.T) {
	command := codesignVerifyCommand("/usr/bin/codesign", "/Applications/My App.app")
	want := []string{"/usr/bin/codesign", "--verify", "--deep", "--strict", "/Applications/My App.app"}
	if !slices.Equal(command.Args, want) {
		t.Errorf("arguments = %q, want %q", command.Args, want)
	}
}

func TestInstallRunCommandError(t *testing.T) {
	err := runCommand(exec.Command("/bin/sh", "-c", "echo 'the signature is invalid' >&2; exit 3"))
	if err == nil {
		t.Fatal("runCommand accepted a failing command")
	}
	if !strings.Contains(err.Error(), "the signature is invalid") {
		t.Errorf("error %q does not carry the command output", err)
	}
}

func TestInstallShellQuote(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"", "''"},
		{"plain", "'plain'"},
		{"a b", "'a b'"},
		{"it's", `'it'\''s'`},
	}
	for _, test := range tests {
		if got := shellQuote(test.text); got != test.want {
			t.Errorf("shellQuote(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

// TestInstallStagesAnAppliedPatch follows the documented order: ApplyPatch
// rebuilds the bundle in a directory named viva-payload-*, and Install has to
// find that copy as an application bundle.
func TestInstallStagesAnAppliedPatch(t *testing.T) {
	oldRoot := filepath.Join(t.TempDir(), "My App.app")
	program := []byte("program")
	if err := os.MkdirAll(filepath.Join(oldRoot, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "Contents", "MacOS", "My App"), program, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPlist := []byte(strings.Replace(fixtureInfoPlist, "1.2.3", "1.0.0", 1))
	if err := os.WriteFile(filepath.Join(oldRoot, "Contents", "Info.plist"), oldPlist, 0o644); err != nil {
		t.Fatal(err)
	}
	newPlist := []byte(strings.Replace(fixtureInfoPlist, "1.2.3", "1.1.0", 1))
	manifest := PatchManifest{
		Schema: PatchSchema, App: "dev.viva.gallery", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: runtime.GOOS, Arch: runtime.GOARCH, Payload: "app",
		Files: []PatchFile{
			fileManifest("Contents/MacOS/My App", program, "same", "", 0o755),
			fileManifest("Contents/Info.plist", newPlist, "new", "files/0001", 0o644),
		},
	}
	patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": newPlist})

	original := locateInstalledPayload
	defer func() { locateInstalledPayload = original }()
	locateInstalledPayload = func() (string, string, error) { return oldRoot, "app", nil }

	payload, err := ApplyPatch(patchPath, "dev.viva.gallery", "1.0.0", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(payload)
	staging := t.TempDir()
	staged, err := stagePayload(payload, staging, filepath.Base(oldRoot))
	if err != nil {
		t.Fatalf("stagePayload: %v", err)
	}
	if want := filepath.Join(staging, "My App.app"); staged != want {
		t.Errorf("staged = %q, want %q", staged, want)
	}
	info, err := readBundleInfo(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Identifier != "dev.viva.gallery" || info.Version != "1.1.0" {
		t.Errorf("staged bundle = %+v, want dev.viva.gallery 1.1.0", info)
	}
	if got, err := os.ReadFile(filepath.Join(staged, "Contents", "MacOS", "My App")); err != nil || !slices.Equal(got, program) {
		t.Errorf("staged program = %q, %v", got, err)
	}
}
