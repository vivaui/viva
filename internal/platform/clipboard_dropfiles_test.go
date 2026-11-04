package platform

import (
	"encoding/binary"
	"slices"
	"testing"
	"unicode/utf16"
)

// dropFiles builds a CF_HDROP block the way Explorer does.
func dropFiles(wide bool, offset int, paths ...string) []byte {
	block := make([]byte, 20)
	binary.LittleEndian.PutUint32(block[0:], uint32(offset))
	if wide {
		binary.LittleEndian.PutUint32(block[16:], 1)
	}
	for len(block) < offset {
		block = append(block, 0)
	}
	for _, path := range paths {
		for _, unit := range utf16.Encode([]rune(path)) {
			block = binary.LittleEndian.AppendUint16(block, unit)
		}
		block = binary.LittleEndian.AppendUint16(block, 0)
	}
	return binary.LittleEndian.AppendUint16(block, 0)
}

func TestFilesFromDropFiles(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []string
	}{
		{"two paths", dropFiles(true, 20, `C:\Users\a\one.txt`, `C:\Users\a\中文 dir`), []string{`C:\Users\a\one.txt`, `C:\Users\a\中文 dir`}},
		{"an offset past the header", dropFiles(true, 24, `D:\x`), []string{`D:\x`}},
		{"the ANSI form is not read", dropFiles(false, 20, `C:\a`), nil},
		{"a block shorter than the header", make([]byte, 19), nil},
		{"an offset beyond the block", func() []byte { b := dropFiles(true, 20, `C:\a`); b[0] = 0xff; return b }(), nil},
		{"an offset inside the header", func() []byte { b := dropFiles(true, 20, `C:\a`); b[0] = 4; return b }(), nil},
		{"an empty list", dropFiles(true, 20), nil},
		{"a path with no terminator ends the list", func() []byte {
			b := dropFiles(true, 20, `C:\a`)
			return b[:len(b)-4] // cut the last path's NUL and the list's
		}(), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FilesFromDropFiles(c.in); !slices.Equal(got, c.want) {
				t.Fatalf("FilesFromDropFiles = %q, want %q", got, c.want)
			}
		})
	}
}

func FuzzFilesFromDropFiles(f *testing.F) {
	f.Add(dropFiles(true, 20, `C:\a`, `C:\b`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if got := FilesFromDropFiles(data); len(got) > MaxClipboardFiles {
			t.Fatalf("%d paths past the cap", len(got))
		}
	})
}

func BenchmarkFilesFromDropFiles(b *testing.B) {
	paths := make([]string, 1000)
	for i := range paths {
		paths[i] = `C:\Users\user\projects\viva\中文\file-` + string(rune('a'+i%26)) + `.txt`
	}
	data := dropFiles(true, 20, paths...)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if len(FilesFromDropFiles(data)) != 1000 {
			b.Fatal("short list")
		}
	}
}
