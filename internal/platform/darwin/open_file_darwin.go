//go:build darwin

package darwin

import "sync"

// Files the system opens the application with arrive on the event-loop thread,
// possibly before the application has said what to do with them (the document
// that launched it): those wait here until it has.
var openFiles struct {
	sync.Mutex
	handler func(string)
	pending []string
}

const maxPendingFiles = 32

func deliverOpenFile(path string) {
	openFiles.Lock()
	handler := openFiles.handler
	if handler == nil {
		if len(openFiles.pending) < maxPendingFiles {
			openFiles.pending = append(openFiles.pending, path)
		}
		openFiles.Unlock()
		return
	}
	openFiles.Unlock()
	handler(path)
}

// SetOpenFileHandler sets who receives the documents the system opens this
// application with, and delivers any that arrived first. It runs on the
// event-loop thread.
func (application *nativeApplication) SetOpenFileHandler(handler func(string)) {
	openFiles.Lock()
	openFiles.handler = handler
	pending := openFiles.pending
	openFiles.pending = nil
	openFiles.Unlock()
	if handler != nil {
		for _, path := range pending {
			handler(path)
		}
	}
}
