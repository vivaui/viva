//go:build linux

package linux

// Post runs fn on the event loop; it is safe from any goroutine.
func (application *linuxApplication) Post(fn func()) {
	if fn == nil {
		return
	}
	application.postMu.Lock()
	application.posts = append(application.posts, fn)
	application.postMu.Unlock()
	if application.postWake.CompareAndSwap(false, true) {
		application.post(commandPost)
	}
}

// drainPosts runs the functions accepted since the last drain, in order. A
// function that posts another is picked up by this same drain.
func (application *linuxApplication) drainPosts() {
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
