package platform

import "testing"

func TestOpenableURLTakesOnlyTheBrowsersSchemes(t *testing.T) {
	for _, test := range []struct {
		address string
		want    bool
	}{
		{"https://example.com/", true},
		{"HTTPS://EXAMPLE.COM/", true},
		{"http://example.com/page", true},
		{"HtTp://example.com", true},
		{"mailto:someone@example.com", true},
		{"MAILTO:SOMEONE@EXAMPLE.COM", true},
		{"file:///etc/passwd", false},
		{"smb://server/share", false},
		{"javascript:alert(1)", false},
		{"ftp://example.com/file", false},
		{"viva-app://page/index.html", false},
		{"https:/example.com/", false},
		{"mailto", false},
		{"", false},
		{" https://example.com/", false},
	} {
		if got := OpenableURL(test.address); got != test.want {
			t.Errorf("OpenableURL(%q) = %v, want %v", test.address, got, test.want)
		}
	}
}
