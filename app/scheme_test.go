package app

import (
	"strings"
	"testing"
)

func TestCheckSchemeURL(t *testing.T) {
	for _, test := range []struct {
		name, scheme, target string
		wantErr              bool
	}{
		{"an editor link", "vscode", "vscode://vscode-remote/ssh-remote+prod/srv/app", false},
		{"the scheme in any case", "zed", "ZED://ssh/prod/srv", false},
		{"a scheme with punctuation", "x-app.v2+", "x-app.v2+:run", false},
		{"no scheme", "", "://x", true},
		{"upper case scheme", "VSCode", "VSCode://x", true},
		{"a digit first", "1app", "1app://x", true},
		{"a space in the scheme", "my app", "my app://x", true},
		{"a web scheme", "https", "https://example.com", true},
		{"a file scheme", "file", "file:///etc/passwd", true},
		{"a script scheme", "javascript", "javascript:alert(1)", true},
		{"a link to another scheme", "vscode", "zed://file/tmp", true},
		{"a space in the link", "vscode", "vscode://file/a b", true},
		{"a newline in the link", "vscode", "vscode://file/a\nb", true},
		{"a NUL in the link", "vscode", "vscode://file/a\x00b", true},
		{"a non-ASCII link", "vscode", "vscode://file/项目", true},
		{"a link too long", "vscode", "vscode://" + strings.Repeat("a", maxSchemeURL), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSchemeURL(test.scheme, test.target); (err != nil) != test.wantErr {
				t.Fatalf("checkSchemeURL(%q, %q) = %v, want error %t", test.scheme, test.target, err, test.wantErr)
			}
		})
	}
}

func TestSchemeHandlerAvailableRefusesAReservedScheme(t *testing.T) {
	if _, err := SchemeHandlerAvailable("https"); err == nil {
		t.Fatal("https is OpenURL's, not a custom scheme")
	}
}

func BenchmarkCheckSchemeURL(b *testing.B) {
	const target = "vscode://vscode-remote/ssh-remote+prod/srv/app/some/deeper/folder"
	b.ReportAllocs()
	for b.Loop() {
		if err := checkSchemeURL("vscode", target); err != nil {
			b.Fatal(err)
		}
	}
}
