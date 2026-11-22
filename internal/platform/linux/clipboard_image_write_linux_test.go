//go:build !android

package linux

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"slices"
	"testing"
)

// picturePNG is a real PNG, the shape of what SetClipboardImage is given: the
// checks read its header, so the bytes must decode as one.
func picturePNG(t *testing.T) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// newSelectionWindowWithImage is newSelectionWindow holding a picture and no
// text, the shape of an image copy.
func newSelectionWindowWithImage(recorder *selectionRecorder, picture []byte) *window {
	win := newSelectionWindow(recorder, "")
	win.clipboardImage = picture
	win.pngAtom = 19
	return win
}

func offeredTargets(t *testing.T, write propertyWrite) []uint32 {
	t.Helper()
	var offered []uint32
	for offset := 0; offset+4 <= len(write.data); offset += 4 {
		offered = append(offered, binary.LittleEndian.Uint32(write.data[offset:]))
	}
	return offered
}

// An image copy is offered as image/png and as nothing else: the CLIPBOARD holds
// a picture, so TARGETS names no text target, the png target hands over the exact
// bytes the owner wrote, and another copy drops the picture.
func TestAClipboardImageIsOfferedAsPNGAndNoText(t *testing.T) {
	picture := []byte("\x89PNG\r\n\x1a\npicture bytes")
	recorder := &selectionRecorder{}
	win := newSelectionWindowWithImage(recorder, picture)
	request := func(target uint32) propertyWrite {
		t.Helper()
		before := len(recorder.writes)
		if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: 10, Target: target, Property: 14}); err != nil {
			t.Fatal(err)
		}
		if len(recorder.writes) != before+1 {
			t.Fatalf("target %d wrote no property", target)
		}
		return recorder.writes[before]
	}
	targets := offeredTargets(t, request(win.targetsAtom))
	if !slices.Contains(targets, win.pngAtom) {
		t.Fatalf("TARGETS = %v, want image/png=%d among them", targets, win.pngAtom)
	}
	for _, atom := range []uint32{win.utf8Atom, win.textAtom, xcbAtomString} {
		if slices.Contains(targets, atom) {
			t.Fatalf("TARGETS = %v names the text target %d beside a picture", targets, atom)
		}
	}
	if write := request(win.pngAtom); write.kind != win.pngAtom || !bytes.Equal(write.data, picture) {
		t.Fatalf("image/png = kind %d data %q", write.kind, write.data)
	}

	// Another copy drops the picture: TARGETS stops naming it, and asking for it
	// anyway is refused (the notify carries no property).
	win.clipboard, win.clipboardImage = "typed", nil
	before := len(recorder.writes)
	recorder.sent = nil
	if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: 10, Target: win.pngAtom, Property: 14}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.writes) != before {
		t.Fatal("image/png was served after a copy that holds no picture")
	}
	if len(recorder.sent) != 1 || recorder.sent[0].Property != 0 {
		t.Fatalf("refusal notify = %v, want no property", recorder.sent)
	}
}

// SetClipboardImage claims the CLIPBOARD, drops every other flavour the window
// held, and refuses input that is not a PNG before it claims anything.
func TestSetClipboardImageClaimsTheClipboardAndDropsOtherFlavours(t *testing.T) {
	picture := picturePNG(t)
	recorder := &selectionRecorder{masks: map[uint32]uint32{}, incrAtom: 77}
	win := &window{
		api: recorder.api(), connection: 1, id: 2, pngAtom: 19,
		clipboardAtom: 10, utf8Atom: 11, textAtom: 12, targetsAtom: 13, uriListAtom: 15, gnomeFilesAtom: 16,
		clipboard: "old text", clipboardHTML: "<b>old</b>", clipboardFiles: []string{"old.txt"},
	}
	activeWindow.Store(win)
	t.Cleanup(func() { activeWindow.Store(nil) })

	if err := SetClipboardImage([]byte("not a PNG")); err == nil {
		t.Fatal("a non-PNG was accepted")
	}
	if len(recorder.ownerCalls) != 0 {
		t.Fatal("a refused write claimed the clipboard")
	}
	if err := SetClipboardImage(picture); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(win.clipboardImage, picture) || win.clipboard != "" || win.clipboardHTML != "" ||
		win.clipboardFiles != nil || !win.ownsClipboard {
		t.Fatalf("after the copy: image %d bytes, text %q, html %q, files %v, ownsClipboard %t",
			len(win.clipboardImage), win.clipboard, win.clipboardHTML, win.clipboardFiles, win.ownsClipboard)
	}
	if len(recorder.ownerCalls) != 1 || recorder.ownerCalls[0] != [2]uint32{2, 10} {
		t.Fatalf("ownership calls = %v, want the window taking CLIPBOARD", recorder.ownerCalls)
	}
}
