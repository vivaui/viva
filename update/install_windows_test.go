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
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
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
	script := payloadHelperScript(1234, `C:\Temp\O'Brien\payload`, `C:\Apps\Motty`, `C:\Apps\Motty\motty.exe`, "v1.2.3", []string{"--flag", "a b"})
	for _, want := range []string{
		"$parent = 1234",
		`$payload = 'C:\Temp\O''Brien\payload'`,
		`$dir = 'C:\Apps\Motty'`,
		`$exe = 'C:\Apps\Motty\motty.exe'`,
		"$backup = Join-Path $dir",
		"function Restore",
		"$created = New-Object System.Collections.ArrayList",
		"Get-Process -Id $parent",
		"Move-Item -LiteralPath $dest -Destination $bak -Force",
		"Copy-Item -LiteralPath $_.FullName -Destination $dest -Force",
		"Start-Process -FilePath $exe -PassThru",
		"$p.HasExited -and $p.ExitCode -ne 0",
		"function Launch {",
		"Start-Process -FilePath $exe -ArgumentList",
		"if ($p.HasExited -and $p.ExitCode -ne 0) { Restore; Launch; Discard; exit 1 }",
		"} catch { Restore; Launch; Discard; exit 1 }",
		"if ($alive) { Discard; exit 1 }",
		"function Discard { Remove-Item -LiteralPath $payload -Recurse -Force",
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

// An application deletes the payload ApplyPatch made once Install has called
// quit, as it may on every platform; the helper reads the payload only after the
// application has exited, so it must read a copy of its own.
func TestInstallPayloadHandsTheHelperACopyTheCallerMayDelete(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	payload := t.TempDir()
	want := copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, filepath.Base(executable)))
	if err := os.MkdirAll(filepath.Join(payload, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payload, "lib", "new.dll"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	var started string
	previous := startPayload
	startPayload = func(_ int, staged, _, _, _ string, _ []string) error {
		started = staged
		return nil
	}
	t.Cleanup(func() { startPayload = previous })

	quit := func() {
		if err := os.RemoveAll(payload); err != nil {
			t.Errorf("delete the payload after quit: %v", err)
		}
	}
	if err := installPayload(InstallOptions{Payload: payload}, quit); err != nil {
		t.Fatal(err)
	}
	if started == "" {
		t.Fatal("installPayload did not start the helper")
	}
	t.Cleanup(func() { _ = os.RemoveAll(started) })
	if strings.EqualFold(filepath.Clean(started), filepath.Clean(payload)) {
		t.Fatal("the helper was handed the caller's payload, which is gone before the helper reads it")
	}
	got, err := os.ReadFile(filepath.Join(started, filepath.Base(executable)))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("the helper's copy lacks the program: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(started, "lib", "new.dll")); err != nil || string(data) != "new" {
		t.Fatalf("the helper's copy lacks lib\\new.dll: %q, %v", data, err)
	}
}

func TestInstallPayloadDeletesItsCopyWhenTheHelperDoesNotStart(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	payload := t.TempDir()
	copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, filepath.Base(executable)))
	var started string
	previous := startPayload
	startPayload = func(_ int, staged, _, _, _ string, _ []string) error {
		started = staged
		return errors.New("no helper")
	}
	t.Cleanup(func() { startPayload = previous })

	called := false
	if err := installPayload(InstallOptions{Payload: payload}, func() { called = true }); err == nil {
		t.Fatal("installPayload reported success without a helper")
	}
	if called {
		t.Fatal("installPayload called quit although the helper did not start")
	}
	if _, err := os.Stat(started); !os.IsNotExist(err) {
		_ = os.RemoveAll(started)
		t.Errorf("the copy for a helper that never started was left behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(payload, filepath.Base(executable))); err != nil {
		t.Errorf("the caller's payload was touched: %v", err)
	}
}

// A helper that cannot read its payload must start the old version again: the
// application has already quit, and nothing else would bring it back.
func TestPayloadHelperRelaunchesTheOldVersionWithoutAPayload(t *testing.T) {
	install := t.TempDir()
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	original := copyWindowsFile(t, testExecutable, filepath.Join(install, "app.exe"))
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("VIVA_UPDATE_START_MARKER", marker)
	missing := filepath.Join(t.TempDir(), "gone")
	script := payloadHelperScript(999999, missing, install, filepath.Join(install, "app.exe"), "", []string{"-test.run=^TestPayloadHelperProcess$"})
	runPayloadHelper(t, script, 1)
	waitForMarker(t, marker)
	got, err := os.ReadFile(filepath.Join(install, "app.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Error("app.exe changed although there was nothing to install")
	}
}

// waitForMarker waits, for a bounded time, until the program the helper started
// wrote marker: Launch starts it without waiting for it.
func waitForMarker(t *testing.T, marker string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if data, err := os.ReadFile(marker); err == nil && string(data) == "started" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the old version was not started again")
		}
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
	name, arguments := helperCommand(script)
	command := exec.Command(name, arguments...)
	command.SysProcAttr = helperSysProcAttr()
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
	// A backup the previous update could not delete, because the old program or
	// one of its loaded DLLs was still held, must not stay in the install
	// directory for good.
	stale := filepath.Join(install, ".viva-update-previous")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "app.exe"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), "", nil)
	runPayloadHelper(t, script, 0)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("a previous update's backup survived the next update: %v", err)
	}
	if left, err := filepath.Glob(filepath.Join(install, ".viva-update-*")); err != nil || len(left) != 0 {
		t.Errorf("backups left in the install directory: %v, %v", left, err)
	}
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

// uninstallEntry creates a per-user Add or Remove Programs entry the way the
// NSIS installer writes one and removes it when the test ends.
func uninstallEntry(t *testing.T, name, location string) registry.Key {
	t.Helper()
	path := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + name
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		t.Skipf("the per-user registry is not writable: %v", err)
	}
	t.Cleanup(func() {
		_ = key.Close()
		_ = registry.DeleteKey(registry.CURRENT_USER, path)
	})
	if err := key.SetStringValue("DisplayVersion", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := key.SetStringValue("InstallLocation", location); err != nil {
		t.Fatal(err)
	}
	if err := key.SetDWordValue("EstimatedSize", 1); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestPayloadHelperBringsTheInstallersRegistrationUpToDate(t *testing.T) {
	payload := t.TempDir()
	install := t.TempDir()
	copyWindowsFile(t, systemExecutable(t, "cmd.exe"), filepath.Join(install, "app.exe"))
	copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, "app.exe"))
	ours := uninstallEntry(t, "viva-update-test-ours", install+`\`)
	other := uninstallEntry(t, "viva-update-test-other", filepath.Join(t.TempDir(), "elsewhere"))

	runPayloadHelper(t, payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), "v2.0.0", nil), 0)

	if version, _, err := ours.GetStringValue("DisplayVersion"); err != nil || version != "2.0.0" {
		t.Errorf("DisplayVersion = %q, %v; want the installed version 2.0.0", version, err)
	}
	if size, _, err := ours.GetIntegerValue("EstimatedSize"); err != nil || size <= 1 {
		t.Errorf("EstimatedSize = %d, %v; want the size of the new files", size, err)
	}
	if version, _, err := other.GetStringValue("DisplayVersion"); err != nil || version != "1.0.0" {
		t.Errorf("an entry for another installation changed: %q, %v", version, err)
	}
}

func TestPayloadHelperWithoutAVersionLeavesTheRegistrationAlone(t *testing.T) {
	payload := t.TempDir()
	install := t.TempDir()
	copyWindowsFile(t, systemExecutable(t, "cmd.exe"), filepath.Join(install, "app.exe"))
	copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, "app.exe"))
	ours := uninstallEntry(t, "viva-update-test-unversioned", install)

	runPayloadHelper(t, payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), "", nil), 0)

	if version, _, err := ours.GetStringValue("DisplayVersion"); err != nil || version != "1.0.0" {
		t.Errorf("DisplayVersion = %q, %v; want it untouched without a version", version, err)
	}
}

func TestStartPayloadHelperInstallsWithoutOutputPipes(t *testing.T) {
	payload, install := t.TempDir(), t.TempDir()
	executable := filepath.Join(install, "app.exe")
	copyWindowsFile(t, systemExecutable(t, "cmd.exe"), executable)
	want := copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, "app.exe"))
	if err := startPayloadHelper(999999, payload, install, executable, "", nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(payload); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the detached helper did not finish installing the payload")
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the detached helper did not install the new executable")
	}
}

func TestPayloadHelperReplacesARunningExecutable(t *testing.T) {
	payload := t.TempDir()
	install := t.TempDir()
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyWindowsFile(t, testExecutable, filepath.Join(install, "app.exe"))
	copyWindowsFile(t, systemExecutable(t, "whoami.exe"), filepath.Join(payload, "app.exe"))

	ready := filepath.Join(t.TempDir(), "running")
	command := exec.Command(filepath.Join(install, "app.exe"), "-test.run=^TestPayloadHelperRunningProcess$")
	command.Env = append(os.Environ(), "VIVA_UPDATE_RUNNING_MARKER="+ready)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the old application did not start")
		}
	}

	script := payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), "", nil)
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
		t.Error("the running app.exe was not replaced")
	}
}

func TestPayloadHelperRunningProcess(t *testing.T) {
	marker := os.Getenv("VIVA_UPDATE_RUNNING_MARKER")
	if marker == "" {
		return
	}
	if err := os.WriteFile(marker, []byte("running"), 0o600); err != nil {
		os.Exit(2)
	}
	time.Sleep(time.Minute)
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
	script := payloadHelperScript(999999, payload, install, filepath.Join(install, "app.exe"), "", []string{"-test.run=^TestPayloadHelperProcess$"})
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
	if _, err := os.Stat(payload); !os.IsNotExist(err) {
		t.Errorf("the helper's payload survived a failed update: %v", err)
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
