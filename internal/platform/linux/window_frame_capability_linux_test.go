package linux

import (
	"testing"
	"unsafe"
)

func TestSyncedFrameDoesNotRequireGTKFrameExtentsAdvertisement(t *testing.T) {
	const (
		supportedAtom = 10
		syncAtom      = 11
		extentsAtom   = 12
	)
	for _, test := range []struct {
		name  string
		atoms []uint32
		want  bool
	}{
		{"sync_only", []uint32{syncAtom}, true},
		{"sync_and_extents", []uint32{syncAtom, extentsAtom}, true},
		{"extents_only", []uint32{extentsAtom}, false},
		{"neither", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			atoms := map[string]uint32{
				"_NET_SUPPORTED":       supportedAtom,
				"_NET_WM_SYNC_REQUEST": syncAtom,
				"_GTK_FRAME_EXTENTS":   extentsAtom,
			}
			win := &window{
				screen: &xcbScreen{Root: 1},
				api: &xcbAPI{
					internAtom: func(_ uintptr, _ uint8, length uint16, name *byte) xcbAtomCookie {
						return xcbAtomCookie{Sequence: atoms[string(unsafe.Slice(name, int(length)))]}
					},
					internAtomReply: func(_ uintptr, cookie xcbAtomCookie, _ *unsafe.Pointer) *xcbAtomReply {
						return &xcbAtomReply{Atom: cookie.Sequence}
					},
					getProperty: func(_ uintptr, _ uint8, window, property, kind, _, _ uint32) xcbPropertyCookie {
						if window != 1 || property != supportedAtom || kind != xcbAtomAtom {
							t.Fatalf("capability query = window %d property %d type %d", window, property, kind)
						}
						return xcbPropertyCookie{}
					},
					getPropertyReply: func(uintptr, xcbPropertyCookie, *unsafe.Pointer) *xcbPropertyReply {
						return &xcbPropertyReply{Format: 32, Type: xcbAtomAtom}
					},
					getPropertyLength: func(*xcbPropertyReply) int32 { return int32(len(test.atoms) * 4) },
					getPropertyValue:  func(*xcbPropertyReply) unsafe.Pointer { return unsafe.Pointer(&test.atoms[0]) },
					free:              func(unsafe.Pointer) {},
				},
			}
			if got := win.supportsSyncedFrame(); got != test.want {
				t.Fatalf("supportsSyncedFrame = %v, want %v for advertised atoms %v", got, test.want, test.atoms)
			}
		})
	}
}
