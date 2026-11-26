package app

import (
	"fmt"

	"github.com/vivaui/viva/gfx"
)

// ContentWindow is an independent native window for application-owned content.
// It uses normal system chrome on Windows, macOS and Linux. Config supplies the
// content and close policy; the helper owns UI-thread delivery and lifetime.
// It is suitable for document viewers, inspectors and detached tool panels.
type ContentWindow struct {
	window         *Window
	invalidator    Invalidator
	closeRequest   func(func())
	concealed      bool
	restoreVisible bool
}

// OpenContentWindow opens a resizable system window. Call it on the application
// thread, like OpenWindow. The helper owns the window's Invalidator, whose Post
// updates run before the supplied OnWake. Background workers never touch UI
// state directly.
func (application *Application) OpenContentWindow(config Config, canvas *gfx.Canvas) (*ContentWindow, error) {
	content := &ContentWindow{closeRequest: config.OnCloseRequest}
	if config.Width <= 0 {
		config.Width = 960
	}
	if config.Height <= 0 {
		config.Height = 720
	}
	config.Invalidator = &content.invalidator
	window, err := application.OpenWindow(config, canvas)
	if err != nil {
		return nil, err
	}
	content.window = window
	return content, nil
}

// Post queues an update for the window thread, as Invalidator.Post does: it is
// refused after the window closes or while the bounded queue is full, so a slow
// window cannot retain unbounded background results or resurrect closed content.
func (content *ContentWindow) Post(update func()) bool {
	if content == nil {
		return false
	}
	return content.invalidator.Post(update)
}

func (content *ContentWindow) Invalidate() {
	if content != nil {
		content.invalidator.Invalidate()
	}
}
func (content *ContentWindow) Show() {
	if content != nil && !content.concealed {
		content.window.Show()
	}
}

// SetConcealed temporarily hides the native window and suppresses Show requests.
// Restoring visibility preserves closed windows and unsaved editor contents.
// Call on the UI thread, for example when locking all of an application's windows.
func (content *ContentWindow) SetConcealed(conceal bool) {
	if content.Closed() || content.concealed == conceal {
		return
	}
	content.concealed = conceal
	if conceal {
		content.restoreVisible = !content.window.Hidden()
		content.window.Hide()
	} else if content.restoreVisible {
		content.restoreVisible = false
		content.window.Show()
	}
}

// SetSize asks the window manager for a new content size, as Window.SetSize
// does. Config.OnResize reports the size actually granted.
func (content *ContentWindow) SetSize(width, height int) error {
	if content == nil || content.window == nil {
		return fmt.Errorf("app: content window is closed")
	}
	return content.window.SetSize(width, height)
}
func (content *ContentWindow) Closed() bool {
	return content == nil || content.window == nil || content.window.Closed()
}

// Frame reports where the content area is on screen, as Window.Frame does: a
// drag that ends over another of the application's windows finds it by comparing
// its pointer with their frames. The second result is false once the window is
// closed.
func (content *ContentWindow) Frame() (gfx.Rect, bool) {
	if content == nil || content.window == nil {
		return gfx.Rect{}, false
	}
	return content.window.Frame()
}

// Minimized reports whether the window manager has the window minimized, as
// Window.Minimized does. A minimized window covers no screen point.
func (content *ContentWindow) Minimized() bool {
	return content != nil && content.window.Minimized()
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
