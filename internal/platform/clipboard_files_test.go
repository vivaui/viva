package platform

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestFilesFromURIList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"uri-list with CRLF and a comment", "# copied\r\nfile:///home/a/one.txt\r\nfile:///home/a/two.txt\r\n", []string{"/home/a/one.txt", "/home/a/two.txt"}},
		{"gnome copied files", "copy\nfile:///home/a/one.txt\nfile:///home/a/dir\n", []string{"/home/a/one.txt", "/home/a/dir"}},
		{"cut leads the same list", "cut\nfile:///x", []string{"/x"}},
		{"escapes are decoded", "file:///home/a/%E4%B8%AD%E6%96%87%20name%23.txt\n", []string{"/home/a/中文 name#.txt"}},
		{"localhost is this computer", "file://localhost/tmp/a\n", []string{"/tmp/a"}},
		{"another computer is not a file here", "file://server/share/a\nfile:///tmp/b\n", []string{"/tmp/b"}},
		{"other schemes are skipped", "http://example.com/a\nsmb://host/a\nfile:///tmp/b\n", []string{"/tmp/b"}},
		{"a relative or empty path is skipped", "file:relative\nfile://\nfile:///tmp/b\n", []string{"/tmp/b"}},
		{"a NUL cannot be part of a path", "file:///tmp/a%00b\nfile:///tmp/c\n", []string{"/tmp/c"}},
		{"the word copy is an action only on the first line", "file:///tmp/a\ncopy\n", []string{"/tmp/a"}},
		{"nothing", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FilesFromURIList([]byte(c.in)); !slices.Equal(got, c.want) {
				t.Fatalf("FilesFromURIList(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFilesFromURIListStopsAtTheCap(t *testing.T) {
	var list strings.Builder
	for i := 0; i < MaxClipboardFiles+10; i++ {
		fmt.Fprintf(&list, "file:///tmp/f%d\n", i)
	}
	got := FilesFromURIList([]byte(list.String()))
	if len(got) != MaxClipboardFiles || got[MaxClipboardFiles-1] != fmt.Sprintf("/tmp/f%d", MaxClipboardFiles-1) {
		t.Fatalf("got %d paths, want the first %d", len(got), MaxClipboardFiles)
	}
}

func FuzzFilesFromURIList(f *testing.F) {
	f.Add([]byte("copy\nfile:///a%20b\r\n"))
	f.Add([]byte("file://%zz\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, path := range FilesFromURIList(data) {
			if path == "" || path[0] != '/' || strings.IndexByte(path, 0) >= 0 {
				t.Fatalf("returned an unusable path %q", path)
			}
		}
	})
}

func BenchmarkFilesFromURIList(b *testing.B) {
	var list strings.Builder
	list.WriteString("copy\n")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&list, "file:///home/user/projects/viva/%%E4%%B8%%AD%%E6%%96%%87/file-%d.txt\n", i)
	}
	data := []byte(list.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if len(FilesFromURIList(data)) != 1000 {
			b.Fatal("short list")
		}
	}
}

func TestDropFilesBytesReadBackAsTheSamePaths(t *testing.T) {
	want := []string{`C:\Users\me\Documents\a file.txt`, `C:\Temp\文件夹`, `D:\x`}
	if got := FilesFromDropFiles(DropFilesBytes(want)); !slices.Equal(got, want) {
		t.Fatalf("DropFilesBytes round trip = %q, want %q", got, want)
	}
	if got := FilesFromDropFiles(DropFilesBytes(nil)); len(got) != 0 {
		t.Fatalf("an empty list read back as %q", got)
	}
}

func TestFileURIListReadsBackAsTheSamePaths(t *testing.T) {
	want := []string{"/tmp/a b.txt", "/home/u/中.txt", "/tmp/dir"}
	if got := FilesFromURIList(FileURIList(want)); !slices.Equal(got, want) {
		t.Fatalf("FileURIList round trip = %q, want %q", got, want)
	}
}
