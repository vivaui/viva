//go:build linux

package linux

// Post runs fn on the event loop; it is safe from any goroutine.
func (application *linuxApplication) Post(fn func()) {
	if fn == nil {
		return
	}
	application.postMu.Lock()
	application.posts = append(application.posts, fn)
	// postWake means a drain is already on its way to a window. Coalescing is
	// only safe once that is true: with no window to carry the wake the message
	// is dropped, and a flag left set would swallow every later post.
	if !application.postWake.Load() && application.deliver(commandPost) {
		application.postWake.Store(true)
	}
	application.postMu.Unlock()
}

// wakePosts asks the loop to drain work accepted before a window could carry
// the wake. OpenWindow calls it once the new window is live and polled.
func (application *linuxApplication) wakePosts() {
	application.postMu.Lock()
	defer application.postMu.Unlock()
	if len(application.posts) == 0 || application.postWake.Load() {
		return
	}
	if application.deliver(commandPost) {
		application.postWake.Store(true)
	}
}

// rearmPosts sends the wake again after the live windows changed. A wake that
// was handed to a window which has since closed went with that window, so the
// flag is cleared and the wake goes to whichever windows remain.
func (application *linuxApplication) rearmPosts() {
	application.postMu.Lock()
	defer application.postMu.Unlock()
	application.postWake.Store(false)
	if len(application.posts) == 0 {
		return
	}
	if application.deliver(commandPost) {
		application.postWake.Store(true)
	}
}

// drainPosts runs the functions accepted since the last drain, in order. A
// function that posts another is picked up by this same drain.
func (application *linuxApplication) drainPosts() {
	application.postMu.Lock()
	application.postWake.Store(false)
	application.postMu.Unlock()
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
