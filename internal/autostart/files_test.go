//go:build (darwin || linux) && !android

package autostart

import (
	"os"
	"strings"
	"testing"
)

func TestSetWritesAndRemovesTheLoginFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	entry := Entry{ID: "dev.test.app", Executable: "/Apps/Test App/test", Args: []string{"--background"}}
	if on, err := Enabled(entry.ID); on || err != nil {
		t.Fatalf("before: %v %v", on, err)
	}
	if err := Set(entry, true); err != nil {
		t.Fatal(err)
	}
	if on, err := Enabled(entry.ID); !on || err != nil {
		t.Fatalf("after enabling: %v %v", on, err)
	}
	target, _ := path(entry.ID)
	data, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(data), "--background") || !strings.Contains(string(data), "Test App") {
		t.Fatalf("file %q: %v", data, err)
	}
	if err := Set(entry, true); err != nil {
		t.Fatalf("enabling twice: %v", err)
	}
	if err := Set(entry, false); err != nil {
		t.Fatal(err)
	}
	if on, _ := Enabled(entry.ID); on {
		t.Fatal("still enabled")
	}
	if err := Set(entry, false); err != nil {
		t.Fatalf("disabling twice: %v", err)
	}
}

func TestQuoteExec(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/app":     "/usr/bin/app",
		"/opt/My App/app":  `"/opt/My App/app"`,
		`/o/a"b`:           `"/o/a\\"b"`,
		"--flag=100%":      `"--flag=100%%"`,
		"":                 `""`,
		`C:\x y\app`:       `"C:\\\\x y\\\\app"`,
		"back`tick$dollar": "\"back\\\\`tick\\\\$dollar\"",
	}
	for input, want := range cases {
		if got := quoteExec(input); got != want {
			t.Errorf("quoteExec(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestLaunchAgentEscapesTheCommand(t *testing.T) {
	text := string(launchAgent(Entry{ID: "dev.x.app", Executable: "/Apps/A&B/app", Args: []string{"--name=<x>"}}))
	for _, want := range []string{"<string>dev.x.app</string>", "/Apps/A&amp;B/app", "--name=&lt;x&gt;", "<key>RunAtLoad</key>"} {
		if !strings.Contains(text, want) {
			t.Errorf("plist lacks %q:\n%s", want, text)
		}
	}
}

func TestTheEntryShowsItsNameAndFallsBackToTheID(t *testing.T) {
	named := string(desktopEntry(Entry{ID: "dev.x.app", Name: "My App", Executable: "/bin/x"}))
	if !strings.Contains(named, "\nName=My App\n") {
		t.Fatalf("named entry:\n%s", named)
	}
	plain := string(desktopEntry(Entry{ID: "dev.x.app", Executable: "/bin/x"}))
	if !strings.Contains(plain, "\nName=dev.x.app\n") {
		t.Fatalf("plain entry:\n%s", plain)
	}
}
