package linux

import (
	"bytes"
	"testing"
	"unsafe"
)

// titleProperty is one changeProperty call a title makes: the property and its bytes.
type titleProperty struct {
	property uint32
	data     []byte
}

func newTitleTestWindow(t *testing.T, written *[]titleProperty) *window {
	t.Helper()
	atoms := map[string]uint32{"UTF8_STRING": 40, "_NET_WM_NAME": 41}
	return &window{
		id:          7,
		connection:  1,
		commandAtom: 50,
		api: &xcbAPI{
			free: func(unsafe.Pointer) {},
			internAtom: func(_ uintptr, _ uint8, length uint16, name *byte) xcbAtomCookie {
				return xcbAtomCookie{Sequence: atoms[string(unsafe.Slice(name, int(length)))]}
			},
			internAtomReply: func(_ uintptr, cookie xcbAtomCookie, _ *unsafe.Pointer) *xcbAtomReply {
				return &xcbAtomReply{Atom: cookie.Sequence}
			},
			changeProperty: func(_ uintptr, _ uint8, _ uint32, property uint32, _ uint32, _ uint8, length uint32, data unsafe.Pointer) xcbVoidCookie {
				var value []byte
				if length > 0 {
					value = bytes.Clone(unsafe.Slice((*byte)(data), int(length)))
				}
				*written = append(*written, titleProperty{property: property, data: value})
				return xcbVoidCookie{}
			},
			sendEvent: func(uintptr, uint8, uint32, uint32, *byte) xcbVoidCookie { return xcbVoidCookie{} },
			flush:     func(uintptr) int32 { return 1 },
		},
	}
}

// The title is posted to the loop, which writes WM_NAME (Latin-1, what older managers read)
// and _NET_WM_NAME (UTF-8). The loop, not the caller, touches the window.
func TestSetTitleWritesBothNamesOnTheEventLoop(t *testing.T) {
	var written []titleProperty
	win := newTitleTestWindow(t, &written)
	window := &nativeWindow{state: &linuxWindowState{win: win}}
	if err := window.SetTitle("Motty – 设置"); err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Fatal("SetTitle wrote the window from the caller's goroutine")
	}
	if err := window.state.command(commandSetTitle, nil); err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("title wrote %d properties, want WM_NAME and _NET_WM_NAME", len(written))
	}
	if want := latin1("Motty – 设置"); !bytes.Equal(written[0].data, want) || written[0].property != xcbAtomWMName {
		t.Fatalf("WM_NAME = %q, want %q", written[0].data, want)
	}
	if want := []byte("Motty – 设置"); !bytes.Equal(written[1].data, want) || written[1].property != 41 {
		t.Fatalf("_NET_WM_NAME = %q, want %q", written[1].data, want)
	}
}

// A later title replaces one the loop has not applied yet, and an empty title clears both names.
func TestSetTitleAppliesTheLatestTitleAndEmptyClears(t *testing.T) {
	var written []titleProperty
	win := newTitleTestWindow(t, &written)
	window := &nativeWindow{state: &linuxWindowState{win: win}}
	for _, title := range []string{"first", "second"} {
		if err := window.SetTitle(title); err != nil {
			t.Fatal(err)
		}
	}
	if err := window.state.command(commandSetTitle, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written[0].data, []byte("second")) {
		t.Fatalf("applied %q, want the latest title", written[0].data)
	}
	written = written[:0]
	if err := window.SetTitle(""); err != nil {
		t.Fatal(err)
	}
	if err := window.state.command(commandSetTitle, nil); err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 || len(written[0].data) != 0 || len(written[1].data) != 0 {
		t.Fatalf("empty title wrote %v, want both names cleared", written)
	}
}
