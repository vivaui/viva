//go:build darwin

package darwin

import (
	"reflect"
	"testing"
)

func TestFilesThatArriveBeforeTheHandlerAreDeliveredWhenItIsSet(t *testing.T) {
	openFiles.Lock()
	openFiles.handler, openFiles.pending = nil, nil
	openFiles.Unlock()
	deliverOpenFile("/tmp/early.md")
	var got []string
	application := &nativeApplication{}
	application.SetOpenFileHandler(func(path string) { got = append(got, path) })
	deliverOpenFile("/tmp/late.markdown")
	if want := []string{"/tmp/early.md", "/tmp/late.markdown"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	application.SetOpenFileHandler(nil)
}
