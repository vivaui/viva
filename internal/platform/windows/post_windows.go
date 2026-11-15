//go:build windows

package windows

// Post runs fn on the event-loop thread; it is safe from any goroutine.
func (application *windowsApplication) Post(fn func()) {
	if fn == nil {
		return
	}
	application.postMu.Lock()
	application.posts = append(application.posts, fn)
	application.postMu.Unlock()
	if application.postWake.CompareAndSwap(false, true) {
		postThreadMessage.Call(uintptr(application.thread), wmPost, 0, 0)
	}
}

// drainPosts runs the functions accepted since the last drain, in order. A
// function that posts another is picked up by this same drain.
func (application *windowsApplication) drainPosts() {
	application.postWake.Store(false)
	for {
		application.postMu.Lock()
		if len(application.posts) == 0 {
			application.postMu.Unlock()
			return
		}
		fn := application.posts[0]
		application.posts = application.posts[1:]
		application.postMu.Unlock()
		fn()
	}
}
