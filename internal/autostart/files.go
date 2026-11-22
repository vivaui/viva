//go:build (darwin || linux) && !ios && !android

package autostart

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// path is the file the system reads at login for an id.
func path(id string) (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "LaunchAgents", id+".plist"), nil
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" || !filepath.IsAbs(config) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		config = filepath.Join(home, ".config")
	}
	return filepath.Join(config, "autostart", id+".desktop"), nil
}

func enable(entry Entry) error {
	target, err := path(entry.ID)
	if err != nil {
		return err
	}
	var content []byte
	if runtime.GOOS == "darwin" {
		content = launchAgent(entry)
	} else {
		content = desktopEntry(entry)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, content, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, target)
}

func disable(id string) error {
	target, err := path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func enabled(id string) (bool, error) {
	target, err := path(id)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(target)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func launchAgent(entry Entry) []byte {
	var buffer bytes.Buffer
	text := func(indent, value string) {
		buffer.WriteString(indent + "<string>")
		xml.EscapeText(&buffer, []byte(value))
		buffer.WriteString("</string>\n")
	}
	buffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buffer.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	buffer.WriteString("<plist version=\"1.0\">\n<dict>\n\t<key>Label</key>\n")
	text("\t", entry.ID)
	buffer.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	text("\t\t", entry.Executable)
	for _, arg := range entry.Args {
		text("\t\t", arg)
	}
	buffer.WriteString("\t</array>\n\t<key>RunAtLoad</key>\n\t<true/>\n\t<key>ProcessType</key>\n\t<string>Interactive</string>\n</dict>\n</plist>\n")
	return buffer.Bytes()
}

// desktopEntry follows the Desktop Entry Specification's quoting for Exec.
func desktopEntry(entry Entry) []byte {
	var command []string
	for _, word := range append([]string{entry.Executable}, entry.Args...) {
		command = append(command, quoteExec(word))
	}
	name := entry.Name
	if name == "" {
		name = entry.ID
	}
	name = strings.ReplaceAll(name, "\n", " ")
	return []byte("[Desktop Entry]\nType=Application\nName=" + name + "\nExec=" + strings.Join(command, " ") +
		"\nTerminal=false\nX-GNOME-Autostart-enabled=true\n")
}

// quoteExec quotes one word of an Exec line. Inside quotes a backslash, quote,
// backtick and dollar need a backslash, and each backslash is doubled again by
// the key's own string escaping.
func quoteExec(word string) string {
	if word != "" && !strings.ContainsAny(word, " \t\"'\\><~|&;$*?#()`%") {
		return word
	}
	var builder strings.Builder
	builder.WriteByte('"')
	for _, r := range word {
		switch r {
		case '"', '`', '$':
			builder.WriteString(`\\`)
			builder.WriteRune(r)
		case '\\':
			builder.WriteString(`\\\\`)
		case '%':
			builder.WriteString("%%")
		default:
			builder.WriteRune(r)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}
