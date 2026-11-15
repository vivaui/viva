//go:build windows

package windows

import "testing"

func TestSystemProxyAnswersForAURL(t *testing.T) {
	proxy, err := SystemProxy("https://example.com/")
	if err != nil {
		t.Fatalf("a normal session must answer, direct or through a proxy: %v", err)
	}
	if proxy != nil && proxy.Host == "" {
		t.Fatalf("proxy = %+v, want a host", proxy)
	}
}
