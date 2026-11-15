//go:build darwin

package darwin

/*
#include "post_darwin.h"
*/
import "C"

import "sync"

// postQueue is the work other goroutines posted to the event-loop thread. One
// application runs in a process, so one queue is enough; the lock defines the
// order concurrent callers are accepted in.
var postQueue struct {
	sync.Mutex
	pending []func()
}

// Post runs fn on the main queue. It never blocks.
func Post(fn func()) {
	if fn == nil {
		return
	}
	postQueue.Lock()
	postQueue.pending = append(postQueue.pending, fn)
	postQueue.Unlock()
	C.viva_application_post()
}

//export vivaApplicationDrain
func vivaApplicationDrain() {
	for {
		postQueue.Lock()
		if len(postQueue.pending) == 0 {
			postQueue.Unlock()
			return
		}
		fn := postQueue.pending[0]
		postQueue.pending = postQueue.pending[1:]
		postQueue.Unlock()
		fn()
	}
}

func (application *nativeApplication) Post(fn func()) { Post(fn) }
