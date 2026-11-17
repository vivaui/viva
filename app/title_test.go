//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"
)

type titleTestControl struct {
	fakeWindow
	titles []string
}

func (control *titleTestControl) SetTitle(title string) error {
	control.titles = append(control.titles, title)
	return nil
}

func TestSetTitleDelegatesToTheHost(t *testing.T) {
	control := &titleTestControl{}
	window := &Window{control: control}
	if err := window.SetTitle("project – main.go"); err != nil {
		t.Fatal(err)
	}
	if len(control.titles) != 1 || control.titles[0] != "project – main.go" {
		t.Fatalf("titles = %q", control.titles)
	}
	window.closed.Store(true)
	if err := window.SetTitle("closed"); err == nil || len(control.titles) != 1 {
		t.Fatal("closed window accepted a title")
	}
	var missing *Window
	if err := missing.SetTitle("nil"); err == nil {
		t.Fatal("nil window accepted a title")
	}
	unsupported := &Window{control: &fakeWindow{}}
	if err := unsupported.SetTitle("x"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported host: %v", err)
	}
}
