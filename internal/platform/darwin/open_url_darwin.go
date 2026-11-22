//go:build darwin && !ios

package darwin

import (
	"net/url"
	"strings"
	"sync"
)

// Links the system opens the application with arrive on the event-loop thread,
// possibly before the application has said what to do with them (a launch by
// link): those wait here until it has.
var openURLs struct {
	sync.Mutex
	handler func(string)
	pending []string
}

const maxPendingURLs = 32

func deliverOpenURL(url string) {
	openURLs.Lock()
	handler := openURLs.handler
	if handler == nil {
		if len(openURLs.pending) < maxPendingURLs {
			openURLs.pending = append(openURLs.pending, url)
		}
		openURLs.Unlock()
		return
	}
	openURLs.Unlock()
	handler(url)
}

// routeOpenURL sends what the system handed over as a URL to the handler that
// owns it. AppKit delivers a document the application was opened with through
// the URL callback as a file URL, so file URLs become the paths the open-file
// handler expects; only other schemes reach the link handler.
func routeOpenURL(text string) {
	if path, ok := fileURLPath(text); ok {
		deliverOpenFile(path)
		return
	}
	deliverOpenURL(text)
}

// fileURLPath is the filesystem path a file URL names. A URL with a host other
// than localhost names a file on another machine, which this system cannot open
// by path.
func fileURLPath(text string) (string, bool) {
	parsed, err := url.Parse(text)
	if err != nil || !strings.EqualFold(parsed.Scheme, "file") || parsed.Opaque != "" {
		return "", false
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		return "", false
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return "", false
	}
	return parsed.Path, true
}

// SetOpenURLHandler sets who receives the links the system opens this
// application with, and delivers any that arrived first. It runs on the
// event-loop thread.
func (application *nativeApplication) SetOpenURLHandler(handler func(string)) {
	openURLs.Lock()
	openURLs.handler = handler
	pending := openURLs.pending
	openURLs.pending = nil
	openURLs.Unlock()
	if handler != nil {
		for _, url := range pending {
			handler(url)
		}
	}
}
