package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/dbus"
)

func TestDirectoryPortalRequestCarriesInitialDirectory(t *testing.T) {
	for _, directory := range []string{"/tmp/My projects", "/tmp/项目/"} {
		t.Run(directory, func(t *testing.T) {
			request := directoryPortalRequest(OpenDirectoryOptions{Title: "Choose folder", InitialDirectory: directory}, "x11:2a")
			decoder := dbus.NewDecoder(encodeFileRequest(request, "tok"))
			if parent, title := decoder.String(), decoder.String(); parent != "x11:2a" || title != "Choose folder" {
				t.Fatalf("parent %q title %q", parent, title)
			}
			seenFolder, seenDirectory := false, false
			end := decoder.ArrayLength(8)
			for decoder.Pos() < end && decoder.Err() == nil {
				decoder.Align(8)
				key, signature := decoder.String(), decoder.Signature()
				switch key {
				case "directory":
					if signature != "b" || !decoder.Bool() {
						t.Fatal("directory option is wrong")
					}
					seenDirectory = true
				case "current_folder":
					if signature != "ay" {
						t.Fatalf("current_folder signature = %q", signature)
					}
					folderEnd := decoder.ArrayLength(1)
					var folder []byte
					for decoder.Pos() < folderEnd && decoder.Err() == nil {
						folder = append(folder, decoder.Byte())
					}
					if string(folder) != directory+"\x00" {
						t.Fatalf("current_folder = %q; want %q", folder, directory+"\x00")
					}
					seenFolder = true
				default:
					decoder.Skip(signature)
				}
			}
			if decoder.Err() != nil || !decoder.Done() || !seenFolder || !seenDirectory {
				t.Fatalf("request: error %v, done %v, folder %v, directory %v", decoder.Err(), decoder.Done(), seenFolder, seenDirectory)
			}
		})
	}
}
