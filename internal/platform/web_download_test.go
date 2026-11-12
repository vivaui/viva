package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFileName(t *testing.T) {
	for _, test := range []struct {
		suggested, address, want string
	}{
		{"report.pdf", "https://example.com/files/other.zip", "report.pdf"},
		{"", "https://example.com/files/a%20b.txt?x=1", "a b.txt"},
		{"", "https://example.com/", "download"},
		{"", "", "download"},
		{"../../evil.sh", "", "evil.sh"},
		{`a\b\c.txt`, "", "c.txt"},
		{`bad:name?.txt`, "", "bad_name_.txt"},
		{"..", "", "download"},
		{"CON", "", "_CON"},
		{"con.txt", "", "_con.txt"},
		{"  report.pdf  ", "", "report.pdf"},
	} {
		if got := DownloadFileName(test.suggested, test.address); got != test.want {
			t.Errorf("DownloadFileName(%q, %q) = %q, want %q", test.suggested, test.address, got, test.want)
		}
	}
}

func TestFreeDownloadPathNeverOverwrites(t *testing.T) {
	folder := t.TempDir()
	if got, want := FreeDownloadPath(folder, "report.pdf"), filepath.Join(folder, "report.pdf"); got != want {
		t.Fatalf("a free name = %q, want %q", got, want)
	}
	for _, name := range []string{"report.pdf", "report (1).pdf", "notes"} {
		if err := os.WriteFile(filepath.Join(folder, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := FreeDownloadPath(folder, "report.pdf"), filepath.Join(folder, "report (2).pdf"); got != want {
		t.Errorf("a taken name = %q, want %q", got, want)
	}
	if got, want := FreeDownloadPath(folder, "notes"), filepath.Join(folder, "notes (1)"); got != want {
		t.Errorf("a taken name without an extension = %q, want %q", got, want)
	}
}
