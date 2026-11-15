package proxy

import (
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestSystemProxyNeedsAnHTTPURL(t *testing.T) {
	for _, raw := range []string{"ftp://example.com", "file:///tmp/x", "example.com"} {
		target, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if _, err := SystemProxy(target); err == nil {
			t.Errorf("SystemProxy(%q) was accepted", raw)
		}
	}
	if _, err := SystemProxy(nil); err == nil {
		t.Fatal("SystemProxy(nil) was accepted")
	}
}

// TestSystemProxyReadsTheEnvironmentInAFreshProcess runs the assertion in a
// child: net/http caches the environment the first time it is asked, so a
// table of t.Setenv calls would test the cache, not the proxy.
func TestSystemProxyReadsTheEnvironmentInAFreshProcess(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("a wasm test cannot start a child process")
	}
	if os.Getenv("VIVA_PROXY_CHILD") == "1" {
		target, err := url.Parse("http://example.com/path")
		if err != nil {
			t.Fatal(err)
		}
		proxy, err := SystemProxy(target)
		if err != nil {
			t.Fatal(err)
		}
		if proxy == nil || proxy.Host != "127.0.0.1:3128" {
			t.Fatalf("proxy = %+v, want the environment's", proxy)
		}
		return
	}
	var environment []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToLower(name) {
		case "http_proxy", "https_proxy", "all_proxy", "no_proxy":
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment,
		"VIVA_PROXY_CHILD=1",
		"HTTP_PROXY=http://127.0.0.1:3128",
		"NO_PROXY=",
	)
	command := exec.Command(os.Args[0], "-test.run=TestSystemProxyReadsTheEnvironmentInAFreshProcess")
	command.Env = environment
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
