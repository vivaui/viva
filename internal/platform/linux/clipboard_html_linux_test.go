package linux

import (
	"testing"
)

// TestAClipboardHTMLOfferedAsTextHTMLBesideUTF8 pins the selection request
// service of a rich copy: TARGETS names text/html only while the pair is on
// the board, the html target hands over the fragment, and the plain targets
// beside it are untouched.
func TestAClipboardHTMLOfferedAsTextHTMLBesideUTF8(t *testing.T) {
	recorder := &selectionRecorder{}
	win := newSelectionWindowWithHTML(recorder, "alt text", "<b>rich</b> and 中文")
	request := func(target uint32) propertyWrite {
		t.Helper()
		before := len(recorder.writes)
		if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: 10, Target: target, Property: 14}); err != nil {
			t.Fatal(err)
		}
		return recorder.writes[before]
	}
	targets := request(13)
	var offered []uint32
	for offset := 0; offset+4 <= len(targets.data); offset += 4 {
		offered = append(offered, uint32(targets.data[offset])|uint32(targets.data[offset+1])<<8|uint32(targets.data[offset+2])<<16|uint32(targets.data[offset+3])<<24)
	}
	if len(offered) != 5 || offered[0] != 13 || offered[4] != win.htmlAtom {
		t.Fatalf("TARGETS = %v, want [13 .. html=17], the html named only while the pair holds", offered)
	}
	write := request(win.htmlAtom)
	if write.kind != win.htmlAtom || string(write.data) != "<b>rich</b> and 中文" {
		t.Fatalf("text/html = kind %d data %q", write.kind, write.data)
	}
	if write := request(11); string(write.data) != "alt text" {
		t.Fatalf("UTF8_STRING beside the html = %q", write.data)
	}

	// Another copy drops the flavour: TARGETS stops naming text/html, and asking
	// for it anyway is refused (the notify carries no property).
	win.clipboard, win.clipboardHTML = "typed", ""
	recorder.writes = nil
	targets = request(13)
	offered = nil
	for offset := 0; offset+4 <= len(targets.data); offset += 4 {
		offered = append(offered, uint32(targets.data[offset])|uint32(targets.data[offset+1])<<8|uint32(targets.data[offset+2])<<16|uint32(targets.data[offset+3])<<24)
	}
	for _, atom := range offered {
		if atom == win.htmlAtom {
			t.Fatal("TARGETS still names text/html after a plain copy")
		}
	}
	recorder.writes, recorder.sent = nil, nil
	if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: 10, Target: win.htmlAtom, Property: 14}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.writes) != 0 || len(recorder.sent) != 1 || recorder.sent[0].Property != xcbAtomNone {
		t.Fatalf("after the pair was replaced: writes %+v sent %+v", recorder.writes, recorder.sent)
	}
	// And losing the clipboard loses the flavour with it.
	win.ownsClipboard = true
	win.handleSelectionClear(&xcbSelectionClearEvent{Selection: 10})
	if win.clipboardHTML != "" || win.ownsClipboard {
		t.Fatal("clearing the clipboard kept the rich flavour")
	}
}

func newSelectionWindowWithHTML(recorder *selectionRecorder, text, html string) *window {
	win := newSelectionWindow(recorder, text)
	win.clipboardHTML = html
	win.htmlAtom = 17
	return win
}

// TestSetClipboardHTMLClaimsTheBoardAndRefusesBadInput covers the ownership
// path: the pair claims the CLIPBOARD selection, invalid input is refused
// before anything is claimed, and without a shell hook that can write the pair
// the X11 event loop's own serve path is what answers.
func TestSetClipboardHTMLClaimsTheBoardAndRefusesBadInput(t *testing.T) {
	recorder := &selectionRecorder{masks: map[uint32]uint32{}, incrAtom: 77}
	win := &window{api: recorder.api(), connection: 1, id: 2, clipboardAtom: 10, utf8Atom: 11, textAtom: 12, targetsAtom: 13, htmlAtom: 17, uriListAtom: 15, gnomeFilesAtom: 16}
	previousWindow, previousHooks := activeWindow.Load(), shellHooks.Load()
	activeWindow.Store(win)
	shellHooks.Store(nil)
	t.Cleanup(func() {
		activeWindow.Store(previousWindow)
		shellHooks.Store(previousHooks)
	})
	if err := SetClipboardHTML("<b>x</b>", "x"); err != nil {
		t.Fatal(err)
	}
	if !win.ownsClipboard || win.clipboard != "x" || win.clipboardHTML != "<b>x</b>" || len(win.clipboardFiles) != 0 {
		t.Fatalf("the board holds %q/%q owned=%v files=%d", win.clipboard, win.clipboardHTML, win.ownsClipboard, len(win.clipboardFiles))
	}
	for _, pair := range []struct{ name, html, alt string }{
		{"html NUL", "a\x00b", "alt"},
		{"text NUL", "<i>t</i>", "a\x00b"},
		{"html UTF-8", "bad\xff", "alt"},
		{"text UTF-8", "<i>t</i>", "bad\xff"},
	} {
		t.Run(pair.name, func(t *testing.T) {
			claims, text, html := len(recorder.ownerCalls), win.clipboard, win.clipboardHTML
			if err := SetClipboardHTML(pair.html, pair.alt); err == nil {
				t.Error("invalid rich clipboard pair was accepted")
			}
			if len(recorder.ownerCalls) != claims || !win.ownsClipboard || win.clipboard != text || win.clipboardHTML != html || len(win.clipboardFiles) != 0 {
				t.Errorf("invalid input changed the board: claims=%d text=%q html=%q", len(recorder.ownerCalls), win.clipboard, win.clipboardHTML)
			}
		})
	}
	activeWindow.Store(nil)
	if err := SetClipboardHTML("<b>x</b>", "x"); err == nil || err.Error() == "" {
		t.Fatal("a copy without a window was accepted")
	}
}

func TestSetClipboardHTMLRejectsInvalidPairBeforeDesktopHook(t *testing.T) {
	previous := shellHooks.Load()
	t.Cleanup(func() { shellHooks.Store(previous) })
	writes := 0
	var html, alt string
	shellHooks.Store(&desktopHooks{setClipboardHTML: func(nextHTML, nextAlt string) error {
		writes++
		html, alt = nextHTML, nextAlt
		return nil
	}})
	if err := SetClipboardHTML("<b>ok</b>", "ok"); err != nil || writes != 1 {
		t.Fatalf("valid pair: error=%v writes=%d", err, writes)
	}
	for _, pair := range []struct{ name, html, alt string }{
		{"html NUL", "a\x00b", "alt"},
		{"text NUL", "<i>t</i>", "a\x00b"},
		{"html UTF-8", "bad\xff", "alt"},
		{"text UTF-8", "<i>t</i>", "bad\xff"},
	} {
		t.Run(pair.name, func(t *testing.T) {
			before, previousHTML, previousAlt := writes, html, alt
			if err := SetClipboardHTML(pair.html, pair.alt); err == nil {
				t.Error("invalid rich clipboard pair was accepted")
			}
			if writes != before || html != previousHTML || alt != previousAlt {
				t.Errorf("invalid input reached the desktop: writes=%d html=%q text=%q", writes, html, alt)
			}
		})
	}
}
