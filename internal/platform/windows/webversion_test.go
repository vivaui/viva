package windows

import (
	"fmt"
	"testing"
)

func TestWebViewVersionParsingAndComparison(t *testing.T) {
	// The table judges the gate by the gate's own constant: a minimum set too low turns the boundary
	// rows red instead of leaving them green.
	minimum := webViewEphemeralMinimum
	below := minimum
	if below[3] > 0 {
		below[3]--
	} else {
		below[2]--
	}
	above := minimum
	above[3]++
	format := func(version [4]int) string {
		return fmt.Sprintf("%d.%d.%d.%d", version[0], version[1], version[2], version[3])
	}
	for _, test := range []struct {
		version string
		parsed  bool
		atLeast bool
	}{
		// The boundary of webViewEphemeralMinimum: its own value is accepted, the one under it is not.
		{format(minimum), true, true},
		{format(below), true, false},
		{format(above), true, true},
		// The gate is Runtime 101 (SDK 1.0.1210.39). Runtimes 96 through 100 install and then cannot
		// make an ephemeral page, so they must be refused rather than pass and fail the options later.
		{"96.0.1054.31", true, false},
		{"96.0.1054.32", true, false},
		{"100.0.1185.39", true, false},
		{"101.0.1210.39", true, true},
		{"101.0.1210.40", true, true},
		{"102.0.1245.33", true, true},
		{"130.0.2849.68", true, true},
		{" 101.0.1210.39 ", true, true},
		{"101.0.1210", true, false},
		{"", false, false},
		{"not a version", false, false},
		{"101.0.x.39", false, false},
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

func TestInstallVersionFromFolder(t *testing.T) {
	// EdgeUpdate's EBWebView value names ...\Application\<version>, and the version in that folder is
	// the one the runtime in it reports. The parser must survive the separator shapes Windows and a
	// test on another operating system produce, and must refuse a folder whose last component is not
	// a version so the gate falls back to the runtime itself.
	for _, test := range []struct {
		folder  string
		version string
		ok      bool
	}{
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\101.0.1210.39`, "101.0.1210.39", true},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\101.0.1210.39\`, "101.0.1210.39", true},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\101.0.1210.39/`, "101.0.1210.39", true},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\101.0.1210.39\/`, "101.0.1210.39", true},
		{`/opt/edgewebview/Application/130.0.2849.68`, "130.0.2849.68", true},
		{`\\server\share\Application\102.0.1245.33`, "102.0.1245.33", true},
		{"101.0.1210.39", "101.0.1210.39", true},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application`, "", false},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\`, "", false},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\EBWebView`, "", false},
		{`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\not a version`, "", false},
		{"/", "", false},
		{"", "", false},
	} {
		version, ok := installVersionFromFolder(test.folder)
		if ok != test.ok || version != test.version {
			t.Errorf("installVersionFromFolder(%q) = %q, %v, want %q, %v", test.folder, version, ok, test.version, test.ok)
		}
	}
}
