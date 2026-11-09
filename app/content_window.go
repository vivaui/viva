package app

import (
	"sync"

	"github.com/vivaui/viva/gfx"
)

const contentWindowPostLimit = 256

// ContentWindow is an independent native window for application-owned content.
// It uses normal system chrome on Windows, macOS and Linux. Config supplies the
// content and close policy; the helper owns UI-thread delivery and lifetime.
// It is suitable for document viewers, inspectors and detached tool panels.
type ContentWindow struct {
	window       *Window
	invalidator  Invalidator
	mu           sync.Mutex
	pending      []func()
	closed       bool
	closeRequest func(func())
}

// OpenContentWindow opens a resizable system window. Call it on the application
// thread, like OpenWindow. The helper owns Invalidator and drains Post updates
// before the supplied OnWake. Background workers never touch UI state directly.
func (application *Application) OpenContentWindow(config Config, canvas *gfx.Canvas) (*ContentWindow, error) {
	content := &ContentWindow{closeRequest: config.OnCloseRequest}
	if config.Width <= 0 {
		config.Width = 960
	}
	if config.Height <= 0 {
		config.Height = 720
	}
	onWake, onClosed := config.OnWake, config.OnClosed
	config.Invalidator = &content.invalidator
	config.OnWake = func() {
		content.mu.Lock()
		pending := content.pending
		content.pending = nil
		closed := content.closed
		content.mu.Unlock()
		if closed {
			return
		}
		for _, update := range pending {
			content.mu.Lock()
			closed = content.closed
			content.mu.Unlock()
			if closed {
				return
			}
			update()
		}
		content.mu.Lock()
		closed = content.closed
		content.mu.Unlock()
		if closed {
			return
		}
		if onWake != nil {
			onWake()
		}
	}
	config.OnClosed = func() {
		content.mu.Lock()
		content.closed = true
		content.pending = nil
		content.mu.Unlock()
		if onClosed != nil {
			onClosed()
		}
	}
	window, err := application.OpenWindow(config, canvas)
	if err != nil {
		content.mu.Lock()
		content.closed = true
		content.mu.Unlock()
		return nil, err
	}
	content.window = window
	return content, nil
}

// Post queues an update for the window thread. It rejects updates after native
// teardown or while the bounded queue is full, so a slow window cannot retain
// unbounded background results or resurrect closed content.
func (content *ContentWindow) Post(update func()) bool {
	if content == nil || update == nil {
		return false
	}
	content.mu.Lock()
	if content.closed || len(content.pending) >= contentWindowPostLimit {
		content.mu.Unlock()
		return false
	}
	content.pending = append(content.pending, update)
	content.mu.Unlock()
	content.invalidator.Invalidate()
	return true
}

func (content *ContentWindow) Invalidate() {
	if content != nil {
		content.invalidator.Invalidate()
	}
}
func (content *ContentWindow) Show() {
	if content != nil {
		content.window.Show()
	}
}
func (content *ContentWindow) Closed() bool {
	return content == nil || content.window == nil || content.window.Closed()
}

// Close force-closes the window, as application teardown does. RequestClose is
// the ordinary UI action and honors unsaved-content confirmation.
func (content *ContentWindow) Close() {
	if content != nil {
		content.window.Close()
	}
}

// RequestClose must run on the window thread; the application's close policy
// may call allow synchronously or after its own confirmation UI.
func (content *ContentWindow) RequestClose() {
	if content.Closed() {
		return
	}
	if content.closeRequest != nil {
		content.closeRequest(content.Close)
	} else {
		content.Close()
	}
}
