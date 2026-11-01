//go:build darwin

package darwin

import (
	"fmt"
	"os"
	"sync"
)

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
		if value := recover(); value != nil {
			// The native handling still runs, but the application's bug is not hidden.
			fmt.Fprintf(os.Stderr, "viva: reopen handler panicked: %v\n", value)
			handled = false
		}
	}()
	handler()
	return true
}
