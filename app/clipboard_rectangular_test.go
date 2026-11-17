//go:build darwin || windows || linux

package app

import (
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func assertClipboardTextFormat(t *testing.T, want string, rectangular bool) {
	t.Helper()
	value, column, err := ClipboardTextWithFormat()
	if err != nil || value != want || column != rectangular {
		t.Fatalf("clipboard snapshot = %q, rectangular=%v, err=%v; want %q, rectangular=%v", value, column, err, want, rectangular)
	}
}

func TestDriverRectangularClipboardCopyCutAndPaste(t *testing.T) {
	editor := text.NewEditor("primary")
	const copied = "\u00e9\r\n\u6f22\r\n"
	calls := 0
	var events []TextEvent
	config := Config{Title: "column clipboard", Width: 300, Height: 120}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 220, Height: 30}, "editor", gfx.HitOptions{Role: gfx.RoleTextField, Multiline: true})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{State: TextStateOf(editor), CopyOverride: true, CopyRectangular: true, CopyText: func() string {
			calls++
			return copied
		}, OnEvent: func(event TextEvent) (TextState, error) {
			events = append(events, event)
			return TextStateOf(editor), nil
		}}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if err := driver.Resize(301, 120); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("paint eagerly evaluated copy text %d times", calls)
	}
	for index, action := range []rune{'c', 'x', 'v'} {
		if handled, err := driver.ShortcutChord(gfx.ModifierControl, action); !handled || err != nil {
			t.Fatalf("shortcut %c: handled=%v err=%v", action, handled, err)
		}
		assertClipboardTextFormat(t, copied, true)
		wantCalls := index + 1
		if action == 'v' {
			wantCalls = 2
		}
		if calls != wantCalls {
			t.Fatalf("shortcut %c evaluated copy text %d times, want %d", action, calls, wantCalls)
		}
	}
	if len(events) != 2 || events[0].Kind != TextPaste || events[0].Text != "" || events[0].Rectangular ||
		events[1].Kind != TextPaste || events[1].Text != copied || !events[1].Rectangular {
		t.Fatalf("copy/cut/paste events = %+v", events)
	}
	if editor.String() != "primary" {
		t.Fatal("clipboard metadata changed the application-owned editor")
	}
	if err := SetClipboardText("stream"); err != nil {
		t.Fatal(err)
	}
	assertClipboardTextFormat(t, "stream", false)
	if handled, err := driver.ShortcutChord(gfx.ModifierControl, 'v'); !handled || err != nil {
		t.Fatalf("plain paste: handled=%v err=%v", handled, err)
	}
	if event := events[len(events)-1]; event.Kind != TextPaste || event.Text != "stream" || event.Rectangular {
		t.Fatalf("plain paste borrowed a stale column marker: %+v", event)
	}
}

func TestDriverRectangularClipboardReplacementsClearMetadata(t *testing.T) {
	for _, kind := range []string{"plain", "html", "image", "files"} {
		t.Run(kind, func(t *testing.T) {
			driver := newFilesDriver(t)
			if err := SetRectangularClipboardText("column\n"); err != nil {
				t.Fatal(err)
			}
			assertClipboardTextFormat(t, "column\n", true)
			want := "plain"
			switch kind {
			case "plain":
				driver.SetClipboard(want)
			case "html":
				want = "rich"
				driver.SetClipboardHTML("<b>rich</b>", want)
			case "image":
				want = ""
				if err := driver.SetClipboardImage(onePixelPNG(t)); err != nil {
					t.Fatal(err)
				}
			case "files":
				want = "file.txt"
				driver.SetClipboardFiles([]string{filepath.Join(t.TempDir(), want)})
			}
			assertClipboardTextFormat(t, want, false)
		})
	}
}

func TestDriverRectangularClipboardRefusalsLeaveTextUntouched(t *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "empty override"
		if secure {
			name = "secure"
		}
		t.Run(name, func(t *testing.T) {
			calls, mutations := 0, 0
			config := Config{Title: "refused column copy", Width: 300, Height: 120}
			config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
				canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 220, Height: 30}, "editor", gfx.HitOptions{Role: gfx.RoleTextField, Secure: secure})
			}
			config.TextInput = func(TextInputContext) *TextInput {
				return &TextInput{State: TextState{Value: "private"}, CopyOverride: true, CopyRectangular: true, CopyText: func() string {
					calls++
					if secure {
						return "secret"
					}
					return ""
				}, OnEvent: func(TextEvent) (TextState, error) {
					mutations++
					return TextState{Value: "private"}, nil
				}}
			}
			driver, err := NewDriver(config, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Click(20, 20); err != nil {
				t.Fatal(err)
			}
			driver.SetClipboard("sentinel")
			for _, action := range []rune{'c', 'x'} {
				if handled, err := driver.ShortcutChord(gfx.ModifierControl, action); !handled || err != nil {
					t.Fatalf("refused shortcut %c: handled=%v err=%v", action, handled, err)
				}
				assertClipboardTextFormat(t, "sentinel", false)
			}
			wantCalls := 2
			if secure {
				wantCalls = 0
			}
			if mutations != 0 || calls != wantCalls {
				t.Fatalf("refused copy/cut: mutations=%d calls=%d, want 0/%d", mutations, calls, wantCalls)
			}
			if err := SetRectangularClipboardText("invalid\x00column"); err == nil {
				t.Fatal("NUL-bearing column text was accepted")
			}
			assertClipboardTextFormat(t, "sentinel", false)
		})
	}
}

func TestDriverRectangularClipboardCallbackHandoffDoesNotCutTheReplacement(t *testing.T) {
	for _, mode := range []string{"same cached input", "replacement", "metadata changed"} {
		for _, action := range []rune{'c', 'x'} {
			t.Run(mode+"/"+string(action), func(t *testing.T) {
				incoming := text.NewEditor("abc")
				incoming.SetSelection(text.Range{Start: 1, End: 2})
				oldEvents := 0
				current := &TextInput{State: TextState{Value: "outgoing"}, CopyOverride: true, CopyRectangular: true,
					OnEvent: func(TextEvent) (TextState, error) {
						oldEvents++
						return TextState{Value: "outgoing"}, nil
					}}
				var driver *Driver
				current.CopyText = func() string {
					if mode == "replacement" {
						current = &TextInput{Editor: incoming}
					} else if mode == "metadata changed" {
						current.CopyRectangular = false
					}
					if err := driver.Resize(301, 120); err != nil {
						t.Fatal(err)
					}
					return "rows\n"
				}
				config := Config{Title: "column callback handoff", Width: 300, Height: 120}
				config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
					canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 220, Height: 30}, "editor", gfx.HitOptions{Role: gfx.RoleTextField, Multiline: true})
				}
				config.TextInput = func(TextInputContext) *TextInput { return current }
				var err error
				driver, err = NewDriver(config, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer driver.Close()
				if err := driver.Click(20, 20); err != nil {
					t.Fatal(err)
				}
				driver.SetClipboard("sentinel")
				if handled, err := driver.ShortcutChord(gfx.ModifierControl, action); !handled || err != nil {
					t.Fatalf("shortcut %c: handled=%v err=%v", action, handled, err)
				}
				if incoming.String() != "abc" || incoming.Selection() != (text.Range{Start: 1, End: 2}) {
					t.Fatalf("old clipboard callback edited the replacement: text=%q selection=%+v", incoming.String(), incoming.Selection())
				}
				if mode != "same cached input" {
					assertClipboardTextFormat(t, "sentinel", false)
					if oldEvents != 0 {
						t.Fatalf("retired callback received %d edits", oldEvents)
					}
				} else {
					assertClipboardTextFormat(t, "rows\n", true)
					wantEvents := 0
					if action == 'x' {
						wantEvents = 1
					}
					if oldEvents != wantEvents {
						t.Fatalf("same cached owner received %d edits, want %d", oldEvents, wantEvents)
					}
				}
			})
		}
	}
}
