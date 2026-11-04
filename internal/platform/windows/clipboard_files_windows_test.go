package windows

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// TestClipboardFilesFromExplorerCopy reads files another program (PowerShell's
// Set-Clipboard -Path, which leaves a CF_HDROP list as Explorer does) put on
// the real clipboard. It replaces the clipboard, so it runs only on request, on
// a machine whose clipboard may be overwritten.
func TestClipboardFilesFromExplorerCopy(t *testing.T) {
	if os.Getenv("VIVA_WINDOWS_CLIPBOARD") == "" {
		t.Skip("set VIVA_WINDOWS_CLIPBOARD=1 to overwrite the real clipboard")
	}
	dir := t.TempDir()
	first, second := filepath.Join(dir, "中文 one.txt"), filepath.Join(dir, "folder")
	if err := os.WriteFile(first, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(second, 0o700); err != nil {
		t.Fatal(err)
	}
	script := `[Console]::OutputEncoding = [Text.Encoding]::UTF8; Set-Clipboard -Path @($env:VIVA_COPY_1, $env:VIVA_COPY_2)`
	command := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
	command.Env = append(os.Environ(), "VIVA_COPY_1="+first, "VIVA_COPY_2="+second)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("copy files: %v: %s", err, output)
	}
	got, err := ClipboardFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{first, second}) {
		t.Fatalf("files = %q, want %q", got, []string{first, second})
	}
	if err := SetClipboardText("not files"); err != nil {
		t.Fatal(err)
	}
	if _, err := ClipboardFiles(); !errors.Is(err, platform.ErrNoClipboardFiles) {
		t.Fatalf("after text replaced the files: %v, want ErrNoClipboardFiles", err)
	}
}
