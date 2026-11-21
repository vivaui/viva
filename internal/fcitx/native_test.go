//go:build unix

package fcitx

import (
	"os"
	"testing"
	"time"
	"unicode"
)

// TestNativeComposition drives a real Fcitx 4 daemon: it asks for an input
// context, turns the Chinese engine on with its trigger key, types a pinyin
// syllable and chooses the first candidate. It runs where a Fcitx 4 daemon is
// already running and VIVA_FCITX_NATIVE=1, which the test environments set.
func TestNativeComposition(t *testing.T) {
	if os.Getenv("VIVA_FCITX_NATIVE") != "1" {
		t.Skip("native Fcitx probe requires a running daemon and VIVA_FCITX_NATIVE=1")
	}
	bus, err := Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()
	committed := 0
	var preedits []string
	pending := 0
	context, err := bus.NewContext("viva-native-probe", Handler{
		Commit: func(value string) {
			for _, r := range value {
				if unicode.Is(unicode.Han, r) {
					committed++
				}
			}
		},
		Preedit: func(event PreeditEvent) {
			if event.Visible {
				preedits = append(preedits, event.Text)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer context.Close()
	if err := context.FocusIn(); err != nil {
		t.Fatal(err)
	}
	if err := context.SetCursorLocation(100, 100, 2, 20); err != nil {
		t.Fatal(err)
	}
	clock := uint32(1000)
	send := func(keysym, keycode, state uint32) {
		t.Helper()
		clock++
		pending++
		if err := context.ProcessKey(Key{Keysym: keysym, Keycode: keycode, State: state, Time: clock}, func(bool) {
			pending--
		}); err != nil {
			t.Fatal(err)
		}
		drain(t, bus, func() bool { return pending == 0 })
	}
	// Ctrl+Space is the daemon's trigger key: it switches the context from the
	// keyboard to the Chinese engine.
	send(' ', 65, 4)
	for _, key := range []struct{ keysym, keycode uint32 }{
		{'n', 57}, {'i', 31}, {'h', 43}, {'a', 38}, {'o', 32},
	} {
		send(key.keysym, key.keycode, 0)
	}
	if len(preedits) == 0 {
		t.Fatalf("the engine committed no preedit; commits=%d", committed)
	}
	send(' ', 65, 0)
	if committed == 0 {
		t.Fatalf("choosing the first candidate committed no Chinese; preedits=%v", preedits)
	}
}

// drain lets the bus deliver what the engine sent until done holds.
func drain(t *testing.T, bus *Bus, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("the engine did not answer in time")
		}
		if err := bus.Dispatch(); err != nil {
			t.Fatal(err)
		}
		bus.Expire()
		if err := bus.Err(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
}
