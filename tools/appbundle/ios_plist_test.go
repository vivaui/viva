package appbundle

import (
	"strings"
	"testing"
)

func TestIOSPlistNamesTheApplicationAndItsPlatform(t *testing.T) {
	info := IOSInfo{Name: "Notes & Co", ID: "dev.example.notes", Version: "2.0.1", Executable: "notes", MinSystem: "16.0", Simulator: true}
	data, err := info.Plist()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"dev.example.notes", "2.0.1", "<string>notes</string>", "<string>16.0</string>", "iPhoneSimulator", "Notes &amp; Co"} {
		if !strings.Contains(text, want) {
			t.Errorf("Info.plist lacks %q", want)
		}
	}
	info.Simulator = false
	data, err = info.Plist()
	if err != nil {
		t.Fatal(err)
	}
	if text := string(data); !strings.Contains(text, "iPhoneOS") || strings.Contains(text, "iPhoneSimulator") {
		t.Error("a device bundle names the simulator platform")
	}
}

func TestIOSPlistIsReproducibleAndNeedsItsFields(t *testing.T) {
	info := IOSInfo{Name: "N", ID: "a.b", Version: "1", Executable: "n", MinSystem: "16.0"}
	first, _ := info.Plist()
	second, _ := info.Plist()
	if string(first) != string(second) {
		t.Fatal("the same bundle gave different bytes")
	}
	info.ID = ""
	if _, err := info.Plist(); err == nil {
		t.Fatal("a bundle with no identifier was accepted")
	}
}
