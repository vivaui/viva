//go:build linux

package linux

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemeHandlerAvailableReadsXdgMime(t *testing.T) {
	directory := t.TempDir()
	script := "#!/bin/sh\n[ \"$3\" = x-scheme-handler/vscode ] && echo code-url-handler.desktop\nexit 0\n"
	if err := os.WriteFile(filepath.Join(directory, "xdg-mime"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	if available, err := SchemeHandlerAvailable("vscode"); err != nil || !available {
		t.Fatalf("vscode: available = %t, err = %v", available, err)
	}
	if available, err := SchemeHandlerAvailable("zed"); err != nil || available {
		t.Fatalf("zed: available = %t, err = %v", available, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := SchemeHandlerAvailable("vscode"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("without xdg-mime: err = %v, want ErrUnsupported", err)
	}
}
