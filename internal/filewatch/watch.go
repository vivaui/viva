// Package filewatch reports that entries of one directory, or of a whole
// directory tree, changed, using the
// platform's own change notification (inotify, FSEvents, ReadDirectoryChangesW).
//
// It does not poll. A Watch owns one operating-system registration and, while it
// is open, one goroutine blocked on the system's wait; closing it releases both,
// and an application that opens no Watch pays nothing. Events carry only the name
// of the entry that changed: editors replace files by writing a temporary one and
// renaming it, so the kinds the platforms report differ and none of them can be
// trusted over looking at the entry afterwards.
package filewatch

import (
	"errors"
	"strings"
	"sync"
)

// ErrUnsupported reports a platform without directory change notification.
var ErrUnsupported = errors.New("filewatch: directory watching is not supported on this platform")

// MaxDirectories bounds the directories one tree watch follows where the system
// needs a registration for each (inotify). A tree with more is refused when
// opened, and one that grows past it asks for a resync.
const MaxDirectories = 8192

// ErrTooLarge reports a tree with more directories than a watch can follow.
var ErrTooLarge = errors.New("filewatch: the directory tree is too large to watch")

// MaxPending bounds the distinct names held between two Drain calls. Past it the
// names are dropped and Pending.Resync is set, so memory stays bounded however
// large a burst is.
const MaxPending = 1024

// Pending is what changed since the last Drain.
type Pending struct {
	// Names are the entries of the directory that changed, in first-seen order and
	// without repeats.
	Names []string
	// Resync means the news was lost or is wider than any name: the event queue
	// overflowed, or the directory itself moved or went away. Look at every entry
	// of interest again, and open a new Watch if the directory was replaced.
	Resync bool
}

// Watch is one directory registration. The zero value is not usable; use Open.
type Watch struct {
	wake func()
	back backend
	// tree: names are slash-separated paths below the directory, and entries of
	// its subdirectories are reported. skip drops any path with a component it
	// names, so a tree can leave out version-control and hidden directories.
	tree bool
	skip func(name string) bool

	mu     sync.Mutex
	names  []string
	seen   map[string]struct{}
	resync bool
	closed bool
}

// backend is the platform registration behind a Watch.
type backend interface {
	close()
}

// Open starts watching the entries directly inside dir, not those of its
// subdirectories. The wake function is called from a background goroutine when
// news is pending and must only hand it on (for example Invalidator.Invalidate);
// one call can stand for many changes, and none is made once Close has returned.
func Open(dir string, wake func()) (*Watch, error) {
	return openWatch(dir, false, nil, wake)
}

// OpenTree is Open for the whole tree below dir. Names are slash-separated paths
// relative to dir. A path with a component for which skip returns true is never
// reported, and where the system needs a registration per directory, a skipped
// directory gets none.
func OpenTree(dir string, skip func(name string) bool, wake func()) (*Watch, error) {
	return openWatch(dir, true, skip, wake)
}

func openWatch(dir string, tree bool, skip func(string) bool, wake func()) (*Watch, error) {
	if wake == nil {
		return nil, errors.New("filewatch: a wake function is required")
	}
	w := &Watch{wake: wake, tree: tree, skip: skip}
	back, err := openBackend(dir, w)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	w.back = back
	w.mu.Unlock()
	return w, nil
}

// Drain returns what changed since the previous call and forgets it. It is cheap
// and never blocks on the file system; call it when the wake function has fired.
// dst is reused for the names when it has room.
func (w *Watch) Drain(dst []string) Pending {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := Pending{Names: append(dst[:0], w.names...), Resync: w.resync}
	w.names = w.names[:0]
	clear(w.seen)
	w.resync = false
	return out
}

// Close releases the registration and waits for its goroutine. Pending news is
// discarded. It is safe to call more than once.
func (w *Watch) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	back := w.back
	w.names = nil
	w.seen = nil
	w.mu.Unlock()
	if back != nil {
		back.close()
	}
}

// skipped reports whether a path has a component the watch leaves out.
func (w *Watch) skipped(name string) bool {
	if w.skip == nil {
		return false
	}
	for name != "" {
		part, rest, _ := strings.Cut(name, "/")
		if w.skip(part) {
			return true
		}
		name = rest
	}
	return false
}

// post records one changed name and wakes the owner when the batch was empty.
func (w *Watch) post(name string) {
	if w.skipped(name) {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	notify := len(w.names) == 0 && !w.resync
	if !w.resync {
		if _, dup := w.seen[name]; !dup {
			if len(w.names) >= MaxPending {
				w.names, w.resync = w.names[:0], true
				clear(w.seen)
			} else {
				if w.seen == nil {
					w.seen = make(map[string]struct{})
				}
				w.seen[name] = struct{}{}
				w.names = append(w.names, name)
			}
		}
	}
	w.mu.Unlock()
	if notify {
		w.wake()
	}
}

// postResync records that the names alone no longer describe the change.
func (w *Watch) postResync() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	notify := len(w.names) == 0 && !w.resync
	w.names, w.resync = w.names[:0], true
	clear(w.seen)
	w.mu.Unlock()
	if notify {
		w.wake()
	}
}
