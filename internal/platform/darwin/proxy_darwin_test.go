//go:build darwin

package darwin

import "testing"

// The machine this runs on may or may not have a proxy; what must hold is that
// CFNetwork answers, and that an answer is a URL or a policy this package names.
func TestSystemProxyAnswersForAURL(t *testing.T) {
	proxy, err := SystemProxy("https://example.com/")
	if err != nil {
		t.Skipf("the system's proxy policy is not readable here: %v", err)
	}
	if proxy != nil && proxy.Host == "" {
		t.Fatalf("proxy = %+v, want a host", proxy)
	}
}
