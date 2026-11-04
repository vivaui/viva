//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vivaui/viva/internal/dbus"
)

const revealCommandTimeout = 5 * time.Second

// RevealPath selects an item through the freedesktop FileManager1 method. The
// reply, not a successful bus send, confirms selection. Desktops without that
// interface still get their containing directory through the standard opener.
func RevealPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if info.IsDir() {
		directory = path
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return revealThroughDesktop(uri, directory, showFileManagerItem, exec.LookPath, runRevealCommand)
}

func showFileManagerItem(uri string) error {
	conn, err := dbus.DialSession()
	if err != nil {
		return fmt.Errorf("linux: connect to session bus: %w", err)
	}
	defer conn.Close()
	var body dbus.Encoder
	array := body.BeginArray(4) // array of file URIs, each aligned as a D-Bus string
	body.String(uri)
	body.EndArray(array)
	body.String("") // startup ID: none
	_, err = conn.Call("org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1", "ShowItems", "ass", body.Bytes())
	if err != nil {
		return fmt.Errorf("linux: FileManager1.ShowItems: %w", err)
	}
	return nil
}

func revealThroughDesktop(uri, directory string, show func(string) error,
	look func(string) (string, error), run func(string, ...string) error) error {
	if err := show(uri); err == nil {
		return nil
	} else {
		failures := []error{err}
		for _, candidate := range [...]struct {
			name string
			args []string
		}{
			{"xdg-open", []string{directory}},
			{"gio", []string{"open", directory}},
		} {
			command, err := look(candidate.name)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if err := run(command, candidate.args...); err == nil {
				return nil
			} else {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	}
}

func runRevealCommand(program string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), revealCommandTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, program, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	if detail := strings.TrimSpace(string(output)); detail != "" {
		return fmt.Errorf("linux: reveal with %s: %w: %s", filepath.Base(program), err, detail)
	}
	return fmt.Errorf("linux: reveal with %s: %w", filepath.Base(program), err)
}
