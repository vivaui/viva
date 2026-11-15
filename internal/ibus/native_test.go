//go:build unix

package ibus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/dbus"
)

func TestNativeEngineLifecycle(t *testing.T) {
	if os.Getenv("VIVA_IBUS_NATIVE") != "1" {
		t.Skip("native IBus probe requires a private daemon and VIVA_IBUS_NATIVE=1")
	}
	engine := os.Getenv("VIVA_IBUS_ENGINE")
	if engine == "" {
		engine = "libpinyin"
	}
	var keycodes map[uint32]uint32
	if err := json.Unmarshal([]byte(os.Getenv("VIVA_IBUS_KEYCODES")), &keycodes); err != nil {
		t.Fatalf("native keymap: %v", err)
	}
	for _, ending := range []string{"commit", "escape", "reset", "focus-out"} {
		t.Run(ending, func(t *testing.T) {
			bus, err := Dial()
			if err != nil {
				t.Fatal(err)
			}
			defer bus.Close()
			var events []string
			var committed []string
			hasPreedit := false
			visible, endingActive, cleared := false, false, false
			wantClear := PreeditHide
			if engine == "hangul" || ending == "focus-out" {
				wantClear = PreeditUpdate
			}
			context, err := bus.NewContext("viva-native-probe", Handler{
				Commit: func(value string) {
					events = append(events, "commit:"+value)
					committed = append(committed, value)
				},
				Preedit: func(event PreeditEvent) {
					hasPreedit = hasPreedit || event.Kind == PreeditUpdate && event.Visible && event.Text != ""
					visible = event.Visible
					if endingActive && event.Kind == wantClear && !event.Visible && (event.Kind == PreeditHide || event.Text == "") {
						cleared = true
					}
					events = append(events, fmt.Sprintf("preedit:%d:%q:cursor=%d:visible=%t:mode=%d:has-mode=%t", event.Kind, event.Text, event.Cursor, event.Visible, event.Mode, event.HasMode))
				},
				Forward: func(symbol, code, state uint32) {
					events = append(events, fmt.Sprintf("forward:%#x:%d:%#x", symbol, code, state))
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer context.Close()
			if err := context.FocusIn(); err != nil {
				t.Fatal(err)
			}
			if ending == "commit" {
				var refusal *dbus.Error
				if err := context.SetEngine("viva-native-missing-engine"); !errors.As(err, &refusal) || bus.Err() != nil {
					t.Fatalf("unknown engine refusal=%v bus=%v", err, bus.Err())
				}
			}
			nativeIBusSelectEngine(t, bus, context, engine)
			reply, err := bus.conn.Call(service, context.path, contextInterface, "GetEngine", "", nil)
			if err != nil || reply.Type == dbus.TypeError {
				t.Fatalf("GetEngine: reply=%+v error=%v", reply, err)
			}
			args := reply.Args()
			signature := args.Signature()
			args.Align(8)
			typeName := args.String()
			args.Skip("a{sv}")
			actual := args.String()
			t.Logf("requested=%s actual=%s type=%s signature=%s", engine, actual, typeName, signature)
			if actual != engine {
				t.Fatalf("engine selection mismatch: %q", actual)
			}
			press := func(symbol, state uint32) {
				t.Helper()
				answered := false
				if err := context.ProcessKey(symbol, keycodes[symbol], state, func(handled bool) {
					events = append(events, fmt.Sprintf("answer:%#x:%t", symbol, handled))
					answered = true
				}); err != nil {
					t.Fatal(err)
				}
				nativeIBusDrain(t, bus, func() bool { return answered }, 2*time.Second)
			}
			keys := "nihao"
			if engine == "hangul" {
				press(0xff31, 0)
				keys = "rksk"
			}
			for _, symbol := range keys {
				press(uint32(symbol), 0)
			}
			if !hasPreedit || !visible {
				t.Fatal("the native ending has no live preedit to settle")
			}
			endingActive = true
			switch ending {
			case "commit":
				press(' ', 0)
			case "escape":
				press(0xff1b, 0)
			case "reset":
				if err := context.Reset(); err != nil {
					t.Fatal(err)
				}
			case "focus-out":
				if err := context.FocusOut(); err != nil {
					t.Fatal(err)
				}
			}
			until := time.Now().Add(150 * time.Millisecond)
			nativeIBusDrain(t, bus, func() bool { return time.Now().After(until) }, time.Second)
			for index, event := range events {
				t.Logf("engine=%s event=%d %s", engine, index, event)
			}
			want := ""
			if engine == "hangul" {
				want = "가나"
			} else if ending == "commit" {
				want = "你好"
			}
			if !cleared || visible || strings.Join(committed, "") != want {
				t.Fatalf("engine=%s ending=%s cleared=%t visible=%t committed=%q want=%q", engine, ending, cleared, visible, committed, want)
			}
		})
	}
}

func nativeIBusSelectEngine(t *testing.T, bus *Bus, context *Context, engine string) {
	t.Helper()
	if os.Getenv("VIVA_IBUS_FRONTEND") == "fcitx5" {
		if err := context.SetEngine(engine); err != nil {
			t.Fatalf("Fcitx selection roundtrip: %v", err)
		}
		nativeFcitxSelectEngine(t, engine)
		if err := bus.Dispatch(); err != nil {
			t.Fatal(err)
		}
		return
	}
	reply, err := bus.conn.Call(service, busPath, service, "GetUseGlobalEngine", "", nil)
	if err != nil {
		t.Fatalf("UseGlobalEngine: %v", err)
	}
	if reply.Signature != "b" {
		t.Fatalf("UseGlobalEngine signature: %q", reply.Signature)
	}
	args := reply.Args()
	global := args.Bool()
	if err := args.Err(); err != nil {
		t.Fatalf("UseGlobalEngine decode: %v", err)
	}
	t.Logf("daemon UseGlobalEngine=%t", global)
	err = context.SetEngine(engine)
	if !global {
		if err != nil {
			t.Fatalf("SetEngine(%q): %v", engine, err)
		}
		return
	}
	var refusal *dbus.Error
	if err != nil && (!errors.As(err, &refusal) || refusal.Name != "org.freedesktop.DBus.Error.Failed" ||
		refusal.Text != "Cannot set engines when use-global-engine is enabled.") || bus.Err() != nil {
		t.Fatalf("global mode context refusal=%v bus=%v", err, bus.Err())
	}
	t.Logf("global mode context selection refused=%t", err != nil)
	// Each case needs a fresh engine even when the daemon shares it globally.
	for _, name := range []string{"xkb:us::eng", engine} {
		var selection dbus.Encoder
		selection.String(name)
		if _, err := bus.conn.Call(service, busPath, service, "SetGlobalEngine", "s", selection.Bytes()); err != nil {
			t.Fatalf("SetGlobalEngine(%q): %v", name, err)
		}
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
}

func nativeFcitxSelectEngine(t *testing.T, engine string) {
	t.Helper()
	for _, args := range [][]string{{"--check", "-s", engine}, {"--check", "-o"}, {"--check", "-n"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		output, err := exec.CommandContext(ctx, "fcitx5-remote", args...).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("native Fcitx controller: %v: %s", err, output)
		}
		if args[1] == "-n" && strings.TrimSpace(string(output)) != engine {
			t.Fatalf("native Fcitx engine=%q, want %q", strings.TrimSpace(string(output)), engine)
		}
	}
}

func nativeIBusDrain(t *testing.T, bus *Bus, done func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("native IBus response timed out")
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

func TestNativeClientPreeditMode(t *testing.T) {
	if os.Getenv("VIVA_IBUS_NATIVE") != "1" {
		t.Skip("native client mode requires a private IBus daemon")
	}
	engine := os.Getenv("VIVA_IBUS_ENGINE")
	var keycodes map[uint32]uint32
	if err := json.Unmarshal([]byte(os.Getenv("VIVA_IBUS_KEYCODES")), &keycodes); err != nil {
		t.Fatal(err)
	}
	for _, ending := range []string{"reset", "focus-out"} {
		t.Run(ending, func(t *testing.T) {
			bus, err := Dial()
			if err != nil {
				t.Fatal(err)
			}
			defer bus.Close()
			var latest PreeditEvent
			var commits []string
			context, err := bus.NewContext("viva-client-mode-probe", Handler{
				Commit:  func(value string) { commits = append(commits, value); t.Logf("commit:%q", value) },
				Preedit: func(event PreeditEvent) { latest = event; t.Logf("preedit:%+v", event) },
			})
			if err != nil {
				t.Fatal(err)
			}
			defer context.Close()
			client, ok := any(context).(interface{ SetClientCommitPreedit(bool) error })
			if !ok {
				t.Fatal("native client mode API missing")
			}
			if err := client.SetClientCommitPreedit(true); err != nil {
				t.Fatal(err)
			}
			if err := context.FocusIn(); err != nil {
				t.Fatal(err)
			}
			nativeIBusSelectEngine(t, bus, context, engine)
			press := func(symbol uint32) {
				t.Helper()
				answered := false
				if err := context.ProcessKey(symbol, keycodes[symbol], 0, func(bool) { answered = true }); err != nil {
					t.Fatal(err)
				}
				nativeIBusDrain(t, bus, func() bool { return answered }, 2*time.Second)
			}
			keys, mode, want := "nihao", uint32(PreeditModeClear), ""
			if engine == "hangul" {
				press(0xff31)
				keys, mode, want = "rksk", PreeditModeCommit, "가"
			}
			for _, symbol := range keys {
				press(uint32(symbol))
			}
			if !latest.HasMode || latest.Mode != mode || !latest.Visible || latest.Text == "" {
				t.Fatalf("engine=%s live native mode=%+v", engine, latest)
			}
			if ending == "reset" {
				err = context.Reset()
			} else {
				err = context.FocusOut()
			}
			if err != nil {
				t.Fatal(err)
			}
			until := time.Now().Add(150 * time.Millisecond)
			nativeIBusDrain(t, bus, func() bool { return time.Now().After(until) }, time.Second)
			if strings.Join(commits, "") != want {
				t.Fatalf("client-owned %s daemon commits=%q, want=%q", ending, commits, want)
			}
		})
	}
}
