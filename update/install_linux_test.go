//go:build linux

package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile writes body to path with an exact mode, defeating the umask.
func writeTestFile(t *testing.T, path string, mode os.FileMode, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// extractFixture unpacks a tar.gz fixture and returns its destination.
func extractFixture(t *testing.T, entries []archiveEntry) string {
	t.Helper()
	destination := t.TempDir()
	if err := extractTarGz(makeTarGz(t, entries), destination); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	return destination
}

func TestInstallLinuxRefusesNonTarGz(t *testing.T) {
	for _, format := range []string{FormatZIP, FormatNSIS, ""} {
		err := install(InstallOptions{Asset: Asset{Format: format}}, func() {
			t.Error("quit was called for a refused format")
		})
		if !errors.Is(err, ErrManifest) {
			t.Errorf("format %q: error %v does not wrap ErrManifest", format, err)
		}
	}
}

func TestInstallLinuxRefusesAppImage(t *testing.T) {
	t.Setenv("APPIMAGE", "/tmp/Viva.AppImage")
	err := install(InstallOptions{Asset: Asset{Format: FormatTarGz}}, func() {
		t.Error("quit was called for an AppImage")
	})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error %v does not wrap ErrUnsupported", err)
	}
}

func TestLinuxDpkgDetection(t *testing.T) {
	originalLookPath, originalRunCommand := lookPath, runCommand
	t.Cleanup(func() {
		lookPath, runCommand = originalLookPath, originalRunCommand
	})
	const executable = "/usr/local/bin/viva"
	notFound := errors.New("executable file not found")
	tests := []struct {
		name   string
		lookup func(string) (string, error)
		run    func(string, ...string) ([]byte, error)
		want   bool
	}{
		{
			name:   "owned by dpkg",
			lookup: func(string) (string, error) { return "/usr/bin/dpkg-query", nil },
			run: func(name string, args ...string) ([]byte, error) {
				if name != "/usr/bin/dpkg-query" {
					t.Errorf("command = %q, want the resolved dpkg-query", name)
				}
				if len(args) != 2 || args[0] != "-S" || args[1] != executable {
					t.Errorf("args = %q, want -S %s", args, executable)
				}
				return []byte("viva: " + executable + "\n"), nil
			},
			want: true,
		},
		{
			name:   "not owned by dpkg",
			lookup: func(string) (string, error) { return "/usr/bin/dpkg-query", nil },
			run: func(string, ...string) ([]byte, error) {
				return []byte("other: /usr/bin/other\n"), nil
			},
			want: false,
		},
		{
			name:   "query exits nonzero",
			lookup: func(string) (string, error) { return "/usr/bin/dpkg-query", nil },
			run: func(string, ...string) ([]byte, error) {
				return []byte("no path record for " + executable + "\n"), errors.New("exit status 1")
			},
			want: false,
		},
		{
			name:   "dpkg-query is missing",
			lookup: func(string) (string, error) { return "", notFound },
			run: func(string, ...string) ([]byte, error) {
				t.Error("dpkg-query ran although it was not found")
				return nil, notFound
			},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookPath = test.lookup
			runCommand = test.run
			if got := managedByDpkg(executable); got != test.want {
				t.Errorf("managedByDpkg = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFindProgramUsesTheOnlyExecutable(t *testing.T) {
	root := extractFixture(t, []archiveEntry{
		{name: "viva-linux-amd64", mode: 0o755, body: "binary"},
		{name: "LICENSE", mode: 0o644, body: "license"},
	})
	got, err := findProgram(root, "viva")
	if err != nil {
		t.Fatalf("findProgram: %v", err)
	}
	if want := filepath.Join(root, "viva-linux-amd64"); got != want {
		t.Errorf("findProgram = %q, want %q", got, want)
	}
}

func TestFindProgramPrefersTheRunningName(t *testing.T) {
	root := extractFixture(t, []archiveEntry{
		{name: "viva", mode: 0o755, body: "preferred"},
		{name: "helper", mode: 0o755, body: "helper"},
	})
	got, err := findProgram(root, "viva")
	if err != nil {
		t.Fatalf("findProgram: %v", err)
	}
	if want := filepath.Join(root, "viva"); got != want {
		t.Errorf("findProgram = %q, want %q", got, want)
	}
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "preferred" {
		t.Errorf("body = %q, want the file named viva", body)
	}
}

func TestFindProgramRefusesAmbiguousExecutables(t *testing.T) {
	root := extractFixture(t, []archiveEntry{
		{name: "one", mode: 0o755, body: "one"},
		{name: "two", mode: 0o755, body: "two"},
	})
	if _, err := findProgram(root, "viva"); err == nil {
		t.Fatal("findProgram accepted two executable files")
	}
}

func TestFindProgramRefusesMissingExecutable(t *testing.T) {
	root := extractFixture(t, []archiveEntry{
		{name: "LICENSE", mode: 0o644, body: "license"},
	})
	if _, err := findProgram(root, "viva"); err == nil {
		t.Fatal("findProgram accepted an archive without an executable")
	}
}

func TestLinuxHelperScriptWaitsThenSwaps(t *testing.T) {
	script := helperScript(replacementPlan{
		ParentPID:   4242,
		Executable:  "/opt/My App/app",
		Replacement: "/opt/My App/.viva-update-1/app",
		Backup:      "/opt/My App/app.old",
		Staging:     "/opt/My App/.viva-update-1",
		Args:        []string{"--flag", "a b"},
	})
	for _, want := range []string{
		"parent=4242",
		`exe='/opt/My App/app'`,
		`while kill -0 "$parent" 2>/dev/null; do`,
		`mv "$exe" "$backup"`,
		`mv "$staged" "$exe"`,
		`nohup "$exe" '--flag' 'a b'`,
		`rm -f "$backup"`,
		`rm -rf "$staging"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script does not contain %q:\n%s", want, script)
		}
	}
}

func TestLinuxHelperScriptQuotesASingleQuote(t *testing.T) {
	if got, want := shellQuote("it's"), `'it'\''s'`; got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}

func TestLinuxHelperScriptParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.sh")
	if err := os.WriteFile(path, []byte(helperScript(replacementPlan{
		ParentPID: 1, Executable: "/bin/true", Replacement: "/bin/false",
		Backup: "/tmp/old", Staging: "/tmp/staging",
	})), 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("sh", "-n", path).CombinedOutput()
	if err != nil {
		t.Fatalf("sh -n: %v\n%s", err, output)
	}
}
