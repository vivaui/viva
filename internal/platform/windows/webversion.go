package windows

import (
	"strconv"
	"strings"
)

// webViewEphemeralMinimum is the oldest WebView2 runtime that offers ICoreWebView2Environment10
// and ICoreWebView2ControllerOptions, which an ephemeral page is made with: Runtime 101 (SDK
// 1.0.1210.39). The SDK build number pairs with the Edge build, and both interfaces' Applies-to
// tables give GA 1.0.1210.39, five months after Edge 96, so a gate set to 96 let a runtime in
// between (96 to 100) through and only the QueryInterface for the options refused it later.
var webViewEphemeralMinimum = [4]int{101, 0, 1210, 39}

// parseWebViewVersion reads a WebView2 runtime version, four dot-separated numbers such as
// "101.0.1210.39", as EdgeUpdate records it in "pv". Missing trailing numbers are zero. ok is false
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
