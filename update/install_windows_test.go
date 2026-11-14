//go:build windows

package update

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestInstallRejectsNonNSISFormat(t *testing.T) {
	artifact := writeWindowsTestArtifact(t, "setup.exe")
	called := false
	err := install(InstallOptions{
		Artifact: artifact,
		Asset:    Asset{Format: FormatZIP},
	}, func() { called = true })
	if !errors.Is(err, ErrManifest) {
		t.Fatalf("install error %v does not wrap ErrManifest", err)
	}
	if called {
		t.Fatal("install called quit after refusing the format")
	}
}

func TestInstallRejectsMissingArtifact(t *testing.T) {
	called := false
	err := install(InstallOptions{
		Artifact: filepath.Join(t.TempDir(), "missing.exe"),
		Asset:    Asset{Format: FormatNSIS},
	}, func() { called = true })
	if err == nil {
		t.Fatal("install accepted a missing artifact")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("install error %v does not wrap fs.ErrNotExist", err)
	}
	if called {
		t.Fatal("install called quit after refusing the artifact")
	}
}

func TestInstallRejectsNonExecutableArtifact(t *testing.T) {
	tests := []struct {
		name     string
		artifact func(*testing.T) string
	}{
		{"wrong extension", func(t *testing.T) string {
			return writeWindowsTestArtifact(t, "setup.zip")
		}},
		{"directory", func(t *testing.T) string {
			directory := filepath.Join(t.TempDir(), "setup.exe")
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatalf("create directory: %v", err)
			}
			return directory
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			err := install(InstallOptions{
				Artifact: test.artifact(t),
				Asset:    Asset{Format: FormatNSIS},
			}, func() { called = true })
			if !errors.Is(err, ErrManifest) {
				t.Fatalf("install error %v does not wrap ErrManifest", err)
			}
			if called {
				t.Fatal("install called quit after refusing the artifact")
			}
		})
	}
}

func TestPowerShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"simple", "'simple'"},
		{"/S", "'/S'"},
		{`C:\Program Files\Viva\viva.exe`, `'C:\Program Files\Viva\viva.exe'`},
		{"it's", "'it''s'"},
		{"a'b'c", "'a''b''c'"},
	}
	for _, test := range tests {
		if got := powerShellQuote(test.input); got != test.want {
			t.Errorf("powerShellQuote(%q) = %s, want %s", test.input, got, test.want)
		}
	}
}

func TestCommandLineArgument(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", `""`},
		{"simple", "simple"},
		{`C:\Program Files\Viva\viva.exe`, `"C:\Program Files\Viva\viva.exe"`},
		{`it's`, `it's`},
		{`say "hi"`, `"say \"hi\""`},
		{`C:\dir with space\`, `"C:\dir with space\\"`},
	}
	for _, test := range tests {
		if got := commandLineArgument(test.input); got != test.want {
			t.Errorf("commandLineArgument(%q) = %s, want %s", test.input, got, test.want)
		}
	}
}

func TestPowerShellArgumentList(t *testing.T) {
	if got := powerShellArgumentList(nil); got != "@()" {
		t.Errorf("powerShellArgumentList(nil) = %s, want @()", got)
	}
	arguments := []string{"--profile", `C:\Users\O'Brien\My Data`, `say "hi"`}
	want := `@('--profile','"C:\Users\O''Brien\My Data"','"say \"hi\""')`
	if got := powerShellArgumentList(arguments); got != want {
		t.Errorf("powerShellArgumentList(%q) = %s, want %s", arguments, got, want)
	}
}

func TestHelperScript(t *testing.T) {
	installer := `C:\Temp\Viva's Setup.exe`
	executable := `C:\Program Files\Viva\viva.exe`
	arguments := []string{"--profile", `C:\Users\O'Brien\My Data`}
	want := `Wait-Process -Id 4321 -ErrorAction SilentlyContinue; Start-Process -FilePath 'C:\Temp\Viva''s Setup.exe' -ArgumentList '/S' -Wait; Start-Process -FilePath 'C:\Program Files\Viva\viva.exe' -ArgumentList @('--profile','"C:\Users\O''Brien\My Data"')`
	if got := helperScript(4321, installer, executable, arguments); got != want {
		t.Errorf("helperScript() = %s, want %s", got, want)
	}
}

func TestHelperScriptWithoutArguments(t *testing.T) {
	want := `Wait-Process -Id 7 -ErrorAction SilentlyContinue; Start-Process -FilePath 'C:\setup.exe' -ArgumentList '/S' -Wait; Start-Process -FilePath 'C:\app.exe'`
	if got := helperScript(7, `C:\setup.exe`, `C:\app.exe`, nil); got != want {
		t.Errorf("helperScript() = %s, want %s", got, want)
	}
}

func TestHelperCommand(t *testing.T) {
	name, arguments := helperCommand("script")
	if name != powerShellExecutable {
		t.Errorf("helper command = %q, want %q", name, powerShellExecutable)
	}
	want := []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", "script"}
	if !slices.Equal(arguments, want) {
		t.Errorf("helper arguments = %q, want %q", arguments, want)
	}
}

func TestHelperSysProcAttr(t *testing.T) {
	attributes := helperSysProcAttr()
	if !attributes.HideWindow {
		t.Error("helper does not hide its window")
	}
	if attributes.CreationFlags&detachedProcess == 0 {
		t.Error("helper is not detached from the console")
	}
	if attributes.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Error("helper does not start in its own process group")
	}
}

func TestCheckWritable(t *testing.T) {
	directory := t.TempDir()
	if err := checkWritable(directory); err != nil {
		t.Fatalf("checkWritable(%s) = %v, want nil", directory, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("checkWritable left %d entries behind", len(entries))
	}

	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := checkWritable(file); !errors.Is(err, ErrNotWritable) {
		t.Errorf("checkWritable(file) = %v, want ErrNotWritable", err)
	}
	if err := checkWritable(filepath.Join(directory, "missing")); !errors.Is(err, ErrNotWritable) {
		t.Errorf("checkWritable(missing) = %v, want ErrNotWritable", err)
	}
}

func writeWindowsTestArtifact(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return path
}

func TestWinTrustStructLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("the 64-bit layout only")
	}
	if got, want := unsafe.Sizeof(winTrustFileInfo{}), uintptr(32); got != want {
		t.Errorf("WINTRUST_FILE_INFO is %d bytes, want %d", got, want)
	}
	if got, want := unsafe.Sizeof(winTrustData{}), uintptr(88); got != want {
		t.Errorf("WINTRUST_DATA is %d bytes, want %d", got, want)
	}
	for name, check := range map[string]struct{ got, want uintptr }{
		"pFile":         {unsafe.Offsetof(winTrustData{}.pFile), 40},
		"dwStateAction": {unsafe.Offsetof(winTrustData{}.stateAction), 48},
		"hWVTStateData": {unsafe.Offsetof(winTrustData{}.stateData), 56},
		"dwProvFlags":   {unsafe.Offsetof(winTrustData{}.providerFlags), 72},
		"dwUIContext":   {unsafe.Offsetof(winTrustData{}.uiContext), 76},
	} {
		if check.got != check.want {
			t.Errorf("%s offset = %d, want %d", name, check.got, check.want)
		}
	}
}

func TestPayloadHelperScriptKeepsABackupAndRestores(t *testing.T) {
	script := payloadHelperScript(1234, `C:\Temp\O'Brien\payload`, `C:\Apps\Motty`, `C:\Apps\Motty\motty.exe`, []string{"--flag", "a b"})
	for _, want := range []string{
		"$parent = 1234",
		`$payload = 'C:\Temp\O''Brien\payload'`,
		`$dir = 'C:\Apps\Motty'`,
		`$exe = 'C:\Apps\Motty\motty.exe'`,
		"$backup = Join-Path $env:TEMP",
		"function Restore",
		"$created = New-Object System.Collections.ArrayList",
		"Get-Process -Id $parent",
		"Copy-Item -LiteralPath $_.FullName -Destination $dest -Force",
		"Start-Process -FilePath $exe -PassThru",
		"$p.HasExited -and $p.ExitCode -ne 0",
		"Restore; Start-Process -FilePath $exe",
		"Remove-Item -Recurse -Force $payload",
		"exit 0",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the helper script lacks %q:\n%s", want, script)
		}
	}
}

func TestInstallPayloadNeedsTheProgramInThePayload(t *testing.T) {
	payload := t.TempDir()
	called := false
	err := installPayload(InstallOptions{Payload: payload}, func() { called = true })
	if !errors.Is(err, ErrManifest) {
		t.Fatalf("err = %v, want ErrManifest", err)
	}
	if called {
		t.Fatal("installPayload called quit after refusing the payload")
	}
}

func TestInstallPayloadRefusesAFile(t *testing.T) {
	file := writeWindowsTestArtifact(t, "payload")
	called := false
	err := installPayload(InstallOptions{Payload: file}, func() { called = true })
	if !errors.Is(err, ErrManifest) {
		t.Fatalf("err = %v, want ErrManifest", err)
	}
	if called {
		t.Fatal("installPayload called quit after refusing the payload")
	}
}

func systemExecutable(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s is not available: %v", path, err)
	}
	return path
}

func copyWindowsFile(t *testing.T, source, destination string) []byte {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return data
}

func runPayloadHelper(t *testing.T, script string, wantExit int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.ps1")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	output, err := command.CombinedOutput()
	if wantExit == 0 {
		if err != nil {
			t.Fatalf("helper: %v\n%s\n%s", err, output, script)
		}
		return
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != wantExit {
		t.Fatalf("helper exit = %v, want %d\n%s\n%s", err, wantExit, output, script)
	}
}

func TestPayloadHelperReplacesTheApplication(t *testing.T) {
	payload := filepath.Join(t.TempDir(), "payload with spaces")
	install := filepath.Join(t.TempDir(), "app with spaces")
	copyWindowsFile(t, systemExecutable(t, "cmd.exe"), filepath.Join(install, "app.exe"))
	copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, "app.exe"))
	if err := os.WriteFile(filepath.Join(install, "old.dll"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payload, "new.dll"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), nil)
	runPayloadHelper(t, script, 0)
	got, err := os.ReadFile(filepath.Join(install, "app.exe"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(systemExecutable(t, "whoami.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("app.exe was not replaced")
	}
	if data, err := os.ReadFile(filepath.Join(install, "new.dll")); err != nil || string(data) != "new" {
		t.Errorf("new.dll = %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(install, "old.dll")); err != nil || string(data) != "old" {
		t.Errorf("old.dll = %q, %v", data, err)
	}
}

func TestPayloadHelperRollsBackAFailedStart(t *testing.T) {
	payload := t.TempDir()
	install := t.TempDir()
	original := copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(install, "app.exe"))
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyWindowsFile(t, testExecutable, filepath.Join(payload, "app.exe"))
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("VIVA_UPDATE_START_MARKER", marker)
	if err := os.WriteFile(filepath.Join(payload, "new.dll"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), []string{"-test.run=^TestPayloadHelperProcess$"})
	runPayloadHelper(t, script, 1)
	if data, err := os.ReadFile(marker); err != nil || string(data) != "started" {
		t.Fatalf("updated application did not start: marker=%q, %v", data, err)
	}
	got, err := os.ReadFile(filepath.Join(install, "app.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Error("app.exe was not restored")
	}
	if _, err := os.Stat(filepath.Join(install, "new.dll")); !os.IsNotExist(err) {
		t.Errorf("new.dll survived the rollback: %v", err)
	}
}

func TestPayloadHelperProcess(t *testing.T) {
	marker := os.Getenv("VIVA_UPDATE_START_MARKER")
	if marker == "" {
		return
	}
	if err := os.WriteFile(marker, []byte("started"), 0o600); err != nil {
		os.Exit(2)
	}
	os.Exit(1)
}
