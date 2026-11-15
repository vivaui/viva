// Package proxy reads the desktop's proxy settings. It is a package of its own
// rather than part of app so that an application which never asks for a proxy
// does not link net/http and its TLS stack: SystemProxy is the only entry, and
// the environment answer is net/http's own.
package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

// ErrUnavailable reports that the desktop's proxy settings could not
// be read: a macOS system that routes through a PAC script, or a Windows
// WinHTTP session that refused the request. An application that gets it either
// goes direct or keeps its own resolver; Viva never reports a proxy it did not
// read.
var ErrUnavailable = errors.New("app: the system proxy settings cannot be read")

// SystemProxy reports the proxy the desktop's settings select for target, nil
// when the request goes direct. It is for HTTP clients: the update downloader,
// a WebView-less API call, an MCP bridge.
//
// An explicit proxy in the environment (HTTP_PROXY, HTTPS_PROXY, ALL_PROXY and
// their lowercase forms, with NO_PROXY) wins, as it does for net/http. Without
// one, macOS asks CFNetwork, Windows asks WinHTTP's automatic proxy (PAC, WPAD
// and the static per-user settings), and Linux reads the environment only:
// desktops that keep their proxy in GSettings do not export it, and this
// package does not start a D-Bus or GIO session to find it.
//
// A macOS system whose policy is a PAC script returns
// ErrUnavailable: evaluating the script is asynchronous and needs a
// run loop, and silently going direct would bypass the user's policy.
//
// The call may block while the system evaluates its policy. It starts no
// goroutine and keeps nothing after it returns.
func SystemProxy(target *url.URL) (*url.URL, error) {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, errors.New("app: SystemProxy needs an http or https URL")
	}
	if environmentProxyConfigured() {
		request := &http.Request{URL: target}
		return http.ProxyFromEnvironment(request)
	}
	proxy, err := nativeSystemProxy(target)
	if errors.Is(err, errSystemProxyUnavailable) {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return proxy, err
}

// errSystemProxyUnavailable is what a platform reports for a policy it cannot
// evaluate; SystemProxy turns it into ErrUnavailable.
var errSystemProxyUnavailable = errors.New("app: the system proxy settings cannot be read")

// environmentProxyConfigured reports whether any proxy variable is set at all.
// Without one, a nil answer from net/http means "direct", and the system is
// asked instead; with one, NO_PROXY's answer is the last word.
func environmentProxyConfigured() bool {
	for _, name := range []string{
		"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy",
	} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}
