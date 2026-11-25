package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// devToolsWebView is a page whose platform can open developer tools, answering with err.
type devToolsWebView struct {
	testWebView
	shown int
	err   error
}

func (view *devToolsWebView) ShowDevTools() error {
	view.shown++
	return view.err
}

func TestShowDevToolsReachesAPlatformThatHasThem(t *testing.T) {
	native := &devToolsWebView{}
	view := &WebView{native: native}
	if err := view.ShowDevTools(); err != nil || native.shown != 1 {
		t.Fatalf("ShowDevTools = %v after %d calls", err, native.shown)
	}
}

func TestShowDevToolsIsUnsupportedWithoutThem(t *testing.T) {
	view := &WebView{native: &testWebView{}}
	if err := view.ShowDevTools(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("a platform without developer tools gave %v", err)
	}
	native := &devToolsWebView{err: fmt.Errorf("%w: no inspector", platform.ErrWebViewUnsupported)}
	view = &WebView{native: native}
	if err := view.ShowDevTools(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("an engine that cannot open them gave %v", err)
	}
	failure := errors.New("closed")
	native = &devToolsWebView{err: failure}
	view = &WebView{native: native}
	if err := view.ShowDevTools(); !errors.Is(err, failure) || errors.Is(err, ErrUnsupported) {
		t.Fatalf("a platform failure became %v", err)
	}
}
