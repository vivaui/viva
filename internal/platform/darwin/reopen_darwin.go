//go:build darwin

package darwin

import "sync"

var applicationReopen struct {
	sync.Mutex
	handler func()
}

func (application *nativeApplication) SetReopenHandler(handler func()) {
	applicationReopen.Lock()
	applicationReopen.handler = handler
	applicationReopen.Unlock()
}

func deliverApplicationReopen() (handled bool) {
	applicationReopen.Lock()
	handler := applicationReopen.handler
	applicationReopen.Unlock()
	if handler == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			handled = false
		}
	}()
	handler()
	return true
}
