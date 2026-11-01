//go:build linux && vivagtk

package linux

import (
	"io"
	"os"
	"syscall"

	"github.com/vivaui/viva/internal/platform"
)

// Files dropped on the Wayland subsurface. GDK handles the drags that enter its own surfaces; the
// subsurface is not one of them, so the shell asks the compositor for a data device of its own on
// the seat. The compositor sends each device of a client the drag's events, with offers of its own;
// a drag over the subsurface is answered here and one over GTK's surface by GDK.

// wlDrag is the drag in progress over a subsurface.
type wlDrag struct {
	target  *waylandSurface
	offer   *wlOffer
	serial  uint32
	x, y    float64
	uris    bool
	pending map[uintptr]*wlOffer // offers the device has announced and not yet entered with
}

// wlOffer is one wl_data_offer: what the source can give.
type wlOffer struct {
	proxy   uintptr
	id      uintptr
	mimes   []string
	actions uint32
}

const uriList = "text/uri-list"

func wlDataSetup(s *wlSession) {
	if s.dataManager == 0 {
		return
	}
	c := s.c
	if wlLists.dataDevice == 0 {
		wlLists.dataOffer = wlListener(
			func(data, offer, mime uintptr) uintptr {
				if o, ok := wlObjects[data].(*wlOffer); ok {
					o.mimes = append(o.mimes, goString(mime))
				}
				return 0
			},
			func(data, offer, actions uintptr) uintptr {
				if o, ok := wlObjects[data].(*wlOffer); ok {
					o.actions = uint32(actions)
				}
				return 0
			},
			func(data, offer, action uintptr) uintptr { return 0 },
		)
		wlLists.dataDevice = wlListener(
			// data_offer: a new offer, whose mime types follow as its own events.
			func(data, device, proxy uintptr) uintptr {
				offer := &wlOffer{proxy: proxy}
				offer.id = wlRegister(offer)
				c.addListener(proxy, wlLists.dataOffer, offer.id)
				wlCurrent.drag.pending[proxy] = offer
				return 0
			},
			func(data, device, serial, surface, x, y, offer uintptr) uintptr {
				wlCurrent.dragEnter(uint32(serial), surface, wlFixed(x), wlFixed(y), offer)
				return 0
			},
			func(data, device uintptr) uintptr { wlCurrent.dragLeave(); return 0 },
			func(data, device, t, x, y uintptr) uintptr { wlCurrent.dragMotion(wlFixed(x), wlFixed(y)); return 0 },
			func(data, device uintptr) uintptr { wlCurrent.dragDrop(); return 0 },
			// selection: the clipboard's offer, not ours to keep.
			func(data, device, proxy uintptr) uintptr { wlCurrent.dropOffer(proxy); return 0 },
		)
	}
	s.drag.pending = map[uintptr]*wlOffer{}
	s.dataDevice = c.create(s.dataManager, 1, c.core["wl_data_device"], c.version(s.dataManager), 0, wlArg(s.seat))
	c.addListener(s.dataDevice, wlLists.dataDevice, 0)
}

// dropOffer destroys an offer nobody will use.
func (s *wlSession) dropOffer(proxy uintptr) {
	offer := s.drag.pending[proxy]
	delete(s.drag.pending, proxy)
	if offer != nil {
		s.c.destroy(proxy, 2)
		wlUnregister(offer.id)
	}
}

func (s *wlSession) dragEnter(serial uint32, surface uintptr, x, y float64, proxy uintptr) {
	offer := s.drag.pending[proxy]
	delete(s.drag.pending, proxy)
	target := s.surfaces[surface]
	if offer == nil {
		return
	}
	if target == nil {
		// GDK's drag, over its own surface.
		s.c.destroy(proxy, 2)
		wlUnregister(offer.id)
		return
	}
	s.drag = wlDrag{target: target, offer: offer, serial: serial, x: x, y: y, pending: s.drag.pending}
	for _, mime := range offer.mimes {
		if mime == uriList {
			s.drag.uris = true
		}
	}
	s.dragOver()
}

// dragOver asks the application about the point and tells the compositor the answer.
func (s *wlSession) dragOver() {
	drag := &s.drag
	if drag.target == nil || !drag.uris {
		return
	}
	accepted := drag.target.w.fileDrop(platform.FileDropEvent{Phase: platform.FileDropOver, X: float32(drag.x), Y: float32(drag.y)})
	c := s.c
	if accepted {
		const actionCopy = 1
		mime := wlCStr(uriList)
		c.request(drag.offer.proxy, 0, wlArg(drag.serial), wlArg(mime)) // accept
		if c.version(drag.offer.proxy) >= 3 {
			c.request(drag.offer.proxy, 4, actionCopy, actionCopy) // set_actions
		}
	} else {
		c.request(drag.offer.proxy, 0, wlArg(drag.serial), 0)
	}
	c.flush(s.display)
}

func (s *wlSession) dragMotion(x, y float64) {
	if s.drag.target != nil {
		s.drag.x, s.drag.y = x, y
		s.dragOver()
	}
}

func (s *wlSession) dragLeave() {
	drag := &s.drag
	if drag.target == nil {
		return
	}
	if drag.offer != nil {
		if drag.uris {
			drag.target.w.fileDrop(platform.FileDropEvent{Phase: platform.FileDropLeave})
		}
		s.c.destroy(drag.offer.proxy, 2)
		wlUnregister(drag.offer.id)
	}
	*drag = wlDrag{pending: drag.pending}
}

// dragDrop is the button released: the files are read from the source through a pipe, off the
// loop's thread, and handed over when they have all arrived.
func (s *wlSession) dragDrop() {
	drag := s.drag
	if drag.target == nil || drag.offer == nil || !drag.uris {
		s.dragLeave()
		return
	}
	s.drag = wlDrag{pending: drag.pending}
	var fds [2]int
	if err := syscall.Pipe2(fds[:], syscall.O_CLOEXEC); err != nil {
		s.c.destroy(drag.offer.proxy, 2)
		return
	}
	c := s.c
	c.request(drag.offer.proxy, 1, wlArg(wlCStr(uriList)), wlArg(fds[1])) // receive
	c.flush(s.display)
	syscall.Close(fds[1])
	target, offer := drag.target, drag.offer
	x, y := float32(drag.x), float32(drag.y)
	reader := os.NewFile(uintptr(fds[0]), "drop")
	go func() {
		data, _ := io.ReadAll(io.LimitReader(reader, 1<<20))
		reader.Close()
		target.w.app.enqueue(func() {
			if !target.w.closed {
				if paths := parseURIList(string(data)); len(paths) > 0 {
					target.w.fileDrop(platform.FileDropEvent{Phase: platform.FileDropDrop, X: x, Y: y, Paths: paths})
				}
			}
			if c.version(offer.proxy) >= 3 {
				c.request(offer.proxy, 3) // finish
			}
			c.destroy(offer.proxy, 2)
			wlUnregister(offer.id)
			c.flush(s.display)
		})
	}()
}
