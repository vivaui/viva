package windows

import (
	"strconv"
	"strings"
)

// webViewEphemeralMinimum is the oldest WebView2 runtime that offers ICoreWebView2Environment10
// and ICoreWebView2ControllerOptions, which an ephemeral page is made with: Edge 96. The SDK that
// exposes them is 1.0.1210, and the runtime that first ships them is 96.0.1054.31.
var webViewEphemeralMinimum = [4]int{96, 0, 1054, 31}

// parseWebViewVersion reads a WebView2 runtime version, four dot-separated numbers such as
// "96.0.1054.31", as EdgeUpdate records it in "pv". Missing trailing numbers are zero. ok is false
// when the text is not a version, and the caller must then try the runtime rather than judge it.
func parseWebViewVersion(version string) (numbers [4]int, ok bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return numbers, false
	}
	parts := strings.Split(version, ".")
	if len(parts) > len(numbers) {
		parts = parts[:len(numbers)]
	}
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return [4]int{}, false
		}
		numbers[i] = number
	}
	return numbers, true
}

// webViewVersionAtLeast reports whether version is at least want.
func webViewVersionAtLeast(version, want [4]int) bool {
	for i := range want {
		if version[i] != want[i] {
			return version[i] > want[i]
		}
	}
	return true
}
