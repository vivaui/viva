package windows

import "testing"

func TestWebViewVersionParsingAndComparison(t *testing.T) {
	minimum := [4]int{96, 0, 1054, 31}
	for _, test := range []struct {
		version string
		parsed  bool
		atLeast bool
	}{
		{"96.0.1054.31", true, true},
		{"96.0.1054.32", true, true},
		{"100.0.1185.36", true, true},
		{"96.0.1054.30", true, false},
		{"95.0.1020.30", true, false},
		{" 96.0.1054.31 ", true, true},
		{"96.0.1054", true, false},
		{"", false, false},
		{"not a version", false, false},
		{"95.0.x.1", false, false},
	} {
		got, ok := parseWebViewVersion(test.version)
		if ok != test.parsed {
			t.Errorf("parseWebViewVersion(%q) ok = %v, want %v", test.version, ok, test.parsed)
			continue
		}
		if !ok {
			continue
		}
		if atLeast := webViewVersionAtLeast(got, minimum); atLeast != test.atLeast {
			t.Errorf("webViewVersionAtLeast(%q, %v) = %v, want %v", test.version, minimum, atLeast, test.atLeast)
		}
	}
}
