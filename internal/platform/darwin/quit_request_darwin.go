//go:build darwin

package darwin

import "sync"

var systemQuitRequest struct {
	sync.Mutex
	handler func()
}

func (application *nativeApplication) SetQuitRequestHandler(handler func()) {
	systemQuitRequest.Lock()
	systemQuitRequest.handler = handler
	systemQuitRequest.Unlock()
}

func deliverQuitRequest() bool {
	systemQuitRequest.Lock()
	handler := systemQuitRequest.handler
	systemQuitRequest.Unlock()
	if handler == nil {
		return false
	}
	handler()
	return true
}
