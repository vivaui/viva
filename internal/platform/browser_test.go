package platform

import (
	"reflect"
	"strings"
	"testing"
)

func TestFileURLIsOneWordABrowserResolvesBackToThePath(t *testing.T) {
	cases := []struct {
		path    string
		windows bool
		want    string
	}{
		{"/tmp/my page #1.html", false, "file:///tmp/my%20page%20%231.html"},
		{`/tmp/"quoted" 100%.html`, false, "file:///tmp/%22quoted%22%20100%25.html"},
		{"/tmp/页面.html", false, "file:///tmp/%E9%A1%B5%E9%9D%A2.html"},
		{`C:\Users\me\my page.html`, true, "file:///C:/Users/me/my%20page.html"},
		{`\\server.local\share\dir\report.html`, true, "file://///server.local/share/dir/report.html"},
		{`\\?\C:\very\long.html`, true, "file:///C:/very/long.html"},
		{`\\?\UNC\server\share\a.html`, true, "file://///server/share/a.html"},
	}
	for _, test := range cases {
		got := FileURL(test.path, test.windows)
		if got != test.want {
			t.Errorf("FileURL(%q, %t) = %q, want %q", test.path, test.windows, got, test.want)
		}
		if strings.ContainsAny(got, " \t\"") {
			t.Errorf("FileURL(%q) = %q is more than one word", test.path, got)
		}
	}
}

func TestBrowserCommandLineFillsInTheRegisteredCommand(t *testing.T) {
	const target = "file:///C:/a%20b.html"
	installed := map[string]bool{`C:\Program Files\Browser\browser.exe`: true}
	exists := func(path string) bool { return installed[path] }
	cases := []struct {
		command, program, line string
	}{
		{`"C:\Program Files\Browser\browser.exe" -osint -url "%1"`, `C:\Program Files\Browser\browser.exe`,
			`"C:\Program Files\Browser\browser.exe" -osint -url "file:///C:/a%20b.html"`},
		{`"C:\Program Files\Browser\browser.exe" --single-argument %1`, `C:\Program Files\Browser\browser.exe`,
			`"C:\Program Files\Browser\browser.exe" --single-argument file:///C:/a%20b.html`},
		{`"C:\Program Files\Browser\browser.exe" "%L" %*`, `C:\Program Files\Browser\browser.exe`,
			`"C:\Program Files\Browser\browser.exe" "file:///C:/a%20b.html" `},
		{`"C:\Program Files\Browser\browser.exe"`, `C:\Program Files\Browser\browser.exe`,
			`"C:\Program Files\Browser\browser.exe" file:///C:/a%20b.html`},
		{`C:\Program Files\Browser\browser.exe -- "%1"`, `C:\Program Files\Browser\browser.exe`,
			`"C:\Program Files\Browser\browser.exe" -- "file:///C:/a%20b.html"`},
		{`C:\Tools\browser.exe %1`, `C:\Tools\browser.exe`, `"C:\Tools\browser.exe" file:///C:/a%20b.html`},
		{`C:\Tools\browser.exe`, `C:\Tools\browser.exe`, `"C:\Tools\browser.exe" file:///C:/a%20b.html`},
		{`"C:\Tools\browser.exe" -url %1 -- %2`, `C:\Tools\browser.exe`, `"C:\Tools\browser.exe" -url file:///C:/a%20b.html -- %2`},
	}
	for _, test := range cases {
		program, line, err := BrowserCommandLine(test.command, target, exists)
		if err != nil || program != test.program || line != test.line {
			t.Errorf("BrowserCommandLine(%q) = %q, %q, %v\nwant %q, %q", test.command, program, line, err, test.program, test.line)
		}
	}
	for _, command := range []string{`"C:\Program Files\browser.exe -url %1`, `""`, ``} {
		if _, _, err := BrowserCommandLine(command, target, exists); err == nil {
			t.Errorf("BrowserCommandLine(%q) was accepted", command)
		}
	}
	for _, unsafe := range []string{"", "a b", `a"b`} {
		if _, _, err := BrowserCommandLine(`"C:\b.exe" %1`, unsafe, exists); err == nil {
			t.Errorf("target %q was accepted", unsafe)
		}
	}
}

func TestDesktopEntryKeepsOnlyItsOwnGroup(t *testing.T) {
	entry := ParseDesktopEntry("/usr/share/applications/browser.desktop", []byte(`# a comment
[Desktop Entry]
Name=Web\sBrowser
Name[fr]=Navigateur
Icon = browser
Exec=browser %u
Terminal=false

[Desktop Action new-window]
Exec=browser --new-window %u
Hidden=true
`))
	want := DesktopEntry{Path: "/usr/share/applications/browser.desktop", Name: "Web Browser", Icon: "browser", Exec: "browser %u"}
	if entry != want {
		t.Fatalf("entry = %+v, want %+v", entry, want)
	}
	hidden := ParseDesktopEntry("", []byte("[Desktop Entry]\nHidden=true\nTerminal=true\n"))
	if !hidden.Hidden || !hidden.Terminal {
		t.Fatalf("hidden entry = %+v", hidden)
	}
}

func TestDesktopEntryArgumentsFollowTheSpecification(t *testing.T) {
	const uri, path = "file:///tmp/a%20b.html", "/tmp/a b.html"
	cases := []struct {
		exec string
		want []string
	}{
		{"firefox %u", []string{"firefox", uri}},
		{"/usr/bin/chromium --enable-features=X %U", []string{"/usr/bin/chromium", "--enable-features=X", uri}},
		{"viewer %f", []string{"viewer", path}},
		{"browser", []string{"browser", uri}},
		{"env BAMF_DESKTOP_FILE_HINT=/x.desktop /snap/bin/firefox %u", []string{"env", "BAMF_DESKTOP_FILE_HINT=/x.desktop", "/snap/bin/firefox", uri}},
		{"/usr/bin/flatpak run --file-forwarding org.example.Browser @@u %u @@", []string{"/usr/bin/flatpak", "run", "--file-forwarding", "org.example.Browser", "@@u", uri, "@@"}},
		{`"/opt/My Browser/browser" --name="%c" %i %m %u`, []string{"/opt/My Browser/browser", "--name=Web Browser", "--icon", "browser", uri}},
		{`sh -c "echo \"\$1\" \\\\" %k 100%% %u`, []string{"sh", "-c", `echo "$1" \\`, "/apps/browser.desktop", "100%", uri}},
		{`browser "" %u`, []string{"browser", "", uri}},
		{"browser --url=%u", []string{"browser", "--url=" + uri}},
	}
	for _, test := range cases {
		entry := DesktopEntry{Path: "/apps/browser.desktop", Name: "Web Browser", Icon: "browser", Exec: test.exec}
		got, err := entry.Arguments(uri, path)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("Exec=%s: %q, %v\nwant %q", test.exec, got, err, test.want)
		}
	}
	for _, exec := range []string{`browser "%u`, "", "%m"} {
		if got, err := (DesktopEntry{Exec: exec}).Arguments(uri, path); err == nil {
			t.Errorf("Exec=%s was accepted: %q", exec, got)
		}
	}
	if got, _ := (DesktopEntry{Exec: "browser %i %u"}).Arguments(uri, path); !reflect.DeepEqual(got, []string{"browser", uri}) {
		t.Errorf("%%i without an icon = %q", got)
	}
}

func TestDesktopEntryCandidatesFollowTheDataDirectories(t *testing.T) {
	got := DesktopEntryCandidates("kde-browser.desktop", "", "/opt/share/:relative:/usr/share", "/home/me")
	want := []string{
		"/home/me/.local/share/applications/kde-browser.desktop",
		"/home/me/.local/share/applications/kde/browser.desktop",
		"/opt/share/applications/kde-browser.desktop",
		"/opt/share/applications/kde/browser.desktop",
		"/usr/share/applications/kde-browser.desktop",
		"/usr/share/applications/kde/browser.desktop",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %q\nwant %q", got, want)
	}
	got = DesktopEntryCandidates("firefox.desktop", "/data", "", "/home/me")
	want = []string{"/data/applications/firefox.desktop", "/usr/local/share/applications/firefox.desktop", "/usr/share/applications/firefox.desktop"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults = %q\nwant %q", got, want)
	}
	for _, id := range []string{"firefox", "../evil.desktop", "a/b.desktop", ".desktop", "a\x00.desktop"} {
		if got := DesktopEntryCandidates(id, "", "", "/home/me"); got != nil {
			t.Errorf("%q has candidates %q", id, got)
		}
	}
}
