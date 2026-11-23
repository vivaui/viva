//go:build darwin && !ios

package darwin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const targetPAC = `function FindProxyForURL(url, host) {
    if (host === "direct.example") return "DIRECT";
    if (host === "socks.example") return "SOCKS 127.0.0.1:1080";
    return "PROXY 127.0.0.1:7890; DIRECT";
}`

func TestProxyFromPACScript(t *testing.T) {
	for _, test := range []struct{ target, want string }{
		{"http://dl.motty.page/latest.json", "http://127.0.0.1:7890"},
		{"https://api.github.com/repos/carapace-sh/carapace-bin/releases/latest", "http://127.0.0.1:7890"},
		{"https://direct.example/", ""},
		{"https://socks.example/", "socks5://127.0.0.1:1080"},
	} {
		t.Run(test.target, func(t *testing.T) {
			proxy, err := proxyFromPAC(context.Background(), test.target, targetPAC, false, 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if proxy != nil {
				got = proxy.String()
			}
			if got != test.want {
				t.Fatalf("PAC proxy = %q, want %q", got, test.want)
			}
		})
	}
}

func TestProxyFromPACURL(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		fmt.Fprint(w, targetPAC)
	}))
	defer server.Close()
	proxy, err := proxyFromPAC(context.Background(), "https://dl.motty.page/latest.json", server.URL+"/proxy.pac", true, 10*time.Second)
	if err != nil || proxy == nil || proxy.String() != "http://127.0.0.1:7890" {
		t.Fatalf("downloaded PAC proxy = %v, %v", proxy, err)
	}
	select {
	case <-requested:
	default:
		t.Fatal("the PAC URL was never downloaded")
	}
}

func TestPACScriptFailureDoesNotGoDirect(t *testing.T) {
	for _, script := range []string{
		`function FindProxyForURL( {`,
		`function FindProxyForURL(url, host) { throw new Error("broken PAC"); }`,
	} {
		proxy, err := proxyFromPAC(context.Background(), "https://example.com/", script, false, 10*time.Second)
		if proxy != nil || !errors.Is(err, ErrProxyUnavailable) {
			t.Fatalf("broken PAC returned %v, %v; want an error instead of DIRECT", proxy, err)
		}
	}
}

func stalledPAC(t *testing.T) (string, <-chan struct{}) {
	t.Helper()
	started, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return server.URL + "/proxy.pac", started
}

func TestPACDownloadHonorsCancellation(t *testing.T) {
	address, started := stalledPAC(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := proxyFromPAC(ctx, "https://example.com/", address, true, 10*time.Second)
		result <- err
	}()
	select {
	case <-started:
	case err := <-result:
		t.Fatalf("PAC lookup ended before the download: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("PAC download did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled PAC lookup returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not interrupt the PAC download")
	}
}

func TestPACDownloadHonorsDeadline(t *testing.T) {
	address, _ := stalledPAC(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	proxy, err := proxyFromPAC(ctx, "https://example.com/", address, true, 10*time.Second)
	if proxy != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired PAC lookup returned %v, %v", proxy, err)
	}
}

func TestPACDownloadTimeoutDoesNotGoDirect(t *testing.T) {
	address, _ := stalledPAC(t)
	start := time.Now()
	proxy, err := proxyFromPAC(context.Background(), "https://example.com/", address, true, 200*time.Millisecond)
	if proxy != nil || !errors.Is(err, ErrProxyUnavailable) || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("stalled PAC returned %v, %v", proxy, err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("PAC download exceeded its timeout")
	}
	// A terminated source must leave the worker's run loop usable.
	proxy, err = proxyFromPAC(context.Background(), "https://direct.example/", targetPAC, false, 10*time.Second)
	if proxy != nil || err != nil {
		t.Fatalf("PAC lookup after timeout returned %v, %v", proxy, err)
	}
}

func TestPACConcurrentLookups(t *testing.T) {
	results := make(chan error, 8)
	for i := range 8 {
		go func() {
			target := "https://dl.motty.page/"
			if i%2 == 0 {
				target = "https://direct.example/"
			}
			proxy, err := proxyFromPAC(context.Background(), target, targetPAC, false, 10*time.Second)
			if err == nil && (proxy == nil) != (i%2 == 0) {
				err = fmt.Errorf("lookup %d: proxy = %v", i, proxy)
			}
			results <- err
		}()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

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
