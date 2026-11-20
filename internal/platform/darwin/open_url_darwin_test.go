//go:build darwin

package darwin

import (
	"reflect"
	"testing"
)

func TestLinksThatArriveBeforeTheHandlerAreDeliveredWhenItIsSet(t *testing.T) {
	openURLs.Lock()
	openURLs.handler, openURLs.pending = nil, nil
	openURLs.Unlock()
	deliverOpenURL("ssh://early@host/")
	var got []string
	application := &nativeApplication{}
	application.SetOpenURLHandler(func(url string) { got = append(got, url) })
	deliverOpenURL("ssh://late@host:22/")
	if want := []string{"ssh://early@host/", "ssh://late@host:22/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	application.SetOpenURLHandler(nil)
}

// The system hands a document the application was opened with to the URL
// callback as a file URL; it has to reach the document handler as a path, while
// a registered scheme still goes to the link handler.
func TestFileURLsReachTheDocumentHandler(t *testing.T) {
	openFiles.Lock()
	openFiles.handler, openFiles.pending = nil, nil
	openFiles.Unlock()
	openURLs.Lock()
	openURLs.handler, openURLs.pending = nil, nil
	openURLs.Unlock()
	application := &nativeApplication{}
	var files, links []string
	application.SetOpenFileHandler(func(path string) { files = append(files, path) })
	application.SetOpenURLHandler(func(url string) { links = append(links, url) })
	t.Cleanup(func() {
		application.SetOpenFileHandler(nil)
		application.SetOpenURLHandler(nil)
	})

	routeOpenURL("file:///Users/me/My%20Notes.md")
	routeOpenURL("ssh://root@example.com:2222")
	routeOpenURL("file://localhost/Users/me/runbook.markdown")

	if want := []string{"/Users/me/My Notes.md", "/Users/me/runbook.markdown"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("documents = %q, want %q", files, want)
	}
	if want := []string{"ssh://root@example.com:2222"}; !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %q, want %q", links, want)
	}
}

func TestFileURLPathNeedsALocalFile(t *testing.T) {
	for _, test := range []struct {
		url  string
		path string
		ok   bool
	}{
		{"file:///tmp/notes.md", "/tmp/notes.md", true},
		{"file://localhost/tmp/notes.md", "/tmp/notes.md", true},
		{"file:///tmp/with%20space.md", "/tmp/with space.md", true},
		{"file://server/share/notes.md", "", false},
		{"file:///", "", false},
		{"ssh://host", "", false},
		{"notes.md", "", false},
	} {
		path, ok := fileURLPath(test.url)
		if path != test.path || ok != test.ok {
			t.Fatalf("fileURLPath(%q) = %q, %v; want %q, %v", test.url, path, ok, test.path, test.ok)
		}
	}
}
