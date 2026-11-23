//go:build darwin && vivahead && !ios

package darwin

import "testing"

// A label whose name is not valid UTF-8 (a remote Linux file name, Latin-1 é here) must not
// fail the offscreen update: the window keeps drawing, the label shows the replacement.
func TestLabelWithInvalidUTF8NameUpdates(t *testing.T) {
	offscreen, err := NewOffscreen(160, 32, 1, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	label := TextLabel{Value: "caf\xe9.txt", X: 0, Y: 0, Width: 160, Height: 16, FontSize: 12}
	if err := offscreen.Set(nil, []TextLabel{label}, nil); err != nil {
		t.Fatalf("Set with invalid UTF-8 label: %v", err)
	}
	if _, _, _, err := offscreen.Render(); err != nil {
		t.Fatalf("Render after invalid UTF-8 label: %v", err)
	}
}
