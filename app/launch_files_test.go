//go:build darwin || windows || linux

package app

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

type openFilesHost struct {
	fakeHost
	handler func(string)
}

func (host *openFilesHost) SetOpenFileHandler(handler func(string)) {
	host.handler = handler
}

func TestApplicationRoutesOpenedDocuments(t *testing.T) {
	host := &openFilesHost{}
	application := &Application{host: host}
	var got []string
	application.OnOpenFile(func(path string) { got = append(got, path) })
	host.handler("/tmp/notes.md")
	host.handler("/tmp/runbook.markdown")
	if want := []string{"/tmp/notes.md", "/tmp/runbook.markdown"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("opened files = %q, want %q", got, want)
	}
	application.OnOpenFile(nil)
	if host.handler != nil {
		t.Fatal("removing the document handler did not clear it")
	}
}

type openFilesPlatform struct {
	platform.Application
	handler func(string)
}

func (platform *openFilesPlatform) SetOpenFileHandler(handler func(string)) {
	platform.handler = handler
}

func TestNativeHostForwardsOpenedDocuments(t *testing.T) {
	native := &openFilesPlatform{}
	application := &Application{host: nativeHost{native: native}}
	var got []string
	application.OnOpenFile(func(path string) { got = append(got, path) })
	native.handler("/tmp/notes.md")
	if want := []string{"/tmp/notes.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("opened files = %q, want %q", got, want)
	}
}
