package linux

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// browserQueryTimeout bounds asking xdg-settings or xdg-mime for the default
// browser, which are shell scripts that can hang on a broken session.
const browserQueryTimeout = 5 * time.Second

// OpenInBrowser opens the file at path in the default browser: the desktop
// entry xdg-settings names as the default web browser, or the one xdg-mime
// gives https links to, is started with the file's URL by its Exec line,
// without a shell, and reaped in the background once it exits. Without a
// default browser it returns platform.ErrNoBrowser.
func OpenInBrowser(path string) error {
	id, err := defaultBrowserID()
	if err != nil {
		return err
	}
	entry, err := readDesktopEntry(id)
	if err != nil {
		return err
	}
	if entry.Hidden {
		return fmt.Errorf("linux: open in browser: %s is hidden: %w", id, platform.ErrNoBrowser)
	}
	if entry.Terminal {
		return unsupported(fmt.Sprintf("the default browser %s runs in a terminal", id))
	}
	arguments, err := entry.Arguments(platform.FileURL(path, false), path)
	if err != nil {
		return fmt.Errorf("linux: open in browser: %w", err)
	}
	if err := platform.StartReaped(exec.Command(arguments[0], arguments[1:]...)); err != nil {
		return fmt.Errorf("linux: launch %s: %w", filepath.Base(arguments[0]), err)
	}
	return nil
}

// defaultBrowserID is the desktop file ID of the default browser.
func defaultBrowserID() (string, error) {
	queries := []desktopCommand{
		{name: "xdg-settings", arguments: []string{"get", "default-web-browser"}},
		{name: "xdg-mime", arguments: []string{"query", "default", "x-scheme-handler/https"}},
	}
	asked := false
	for _, query := range queries {
		program, err := exec.LookPath(query.name)
		if err != nil {
			continue
		}
		asked = true
		ctx, cancel := context.WithTimeout(context.Background(), browserQueryTimeout)
		output, err := exec.CommandContext(ctx, program, query.arguments...).Output()
		cancel()
		if err != nil {
			continue
		}
		id, _, _ := strings.Cut(strings.TrimSpace(string(output)), ";")
		if id = strings.TrimSpace(id); id != "" {
			return id, nil
		}
	}
	if !asked {
		return "", unsupported("opening a file in the browser requires xdg-settings or xdg-mime")
	}
	return "", fmt.Errorf("linux: open in browser: %w", platform.ErrNoBrowser)
}

func readDesktopEntry(id string) (platform.DesktopEntry, error) {
	home, _ := os.UserHomeDir()
	for _, candidate := range platform.DesktopEntryCandidates(id, os.Getenv("XDG_DATA_HOME"), os.Getenv("XDG_DATA_DIRS"), home) {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return platform.ParseDesktopEntry(candidate, data), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return platform.DesktopEntry{}, fmt.Errorf("linux: read %s: %w", candidate, err)
		}
	}
	return platform.DesktopEntry{}, fmt.Errorf("linux: open in browser: the default browser %q has no desktop entry: %w", id, platform.ErrNoBrowser)
}
