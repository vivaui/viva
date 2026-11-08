package app

import (
	"errors"

	"github.com/vivaui/viva/internal/filewatch"
)

// DirectoryChanges is the news a DirectoryWatch has gathered since it was last
// read.
type DirectoryChanges struct {
	// Names are the entries directly inside the directory that were created,
	// changed, renamed or removed, without repeats. The kind is not reported on
	// purpose: editors save by writing a temporary file and renaming it over the
	// original, so the sequence differs between programs and platforms. Look at
	// the entry itself to learn what it is now.
	Names []string
	// Resync means names alone no longer describe the change: the system dropped
	// events, or the directory itself was moved, unmounted or removed. Look at
	// every entry of interest again, and open a new watch if the directory was
	// replaced.
	Resync bool
}

// DirectoryWatch reports changes to the entries of one directory from the
// operating system's own notification (inotify, FSEvents, ReadDirectoryChangesW).
//
// It does not poll and keeps no timer. An open watch holds one system
// registration and one goroutine blocked on it; closing it releases both, and an
// application that opens none pays nothing. A change wakes the window thread
// through the function given at the start, which only hands the news on (for
// example Invalidator.Invalidate, whose Config.OnWake then reads it with
// Changes).
type DirectoryWatch struct {
	watch *filewatch.Watch
}

// WatchDirectory starts watching the entries directly inside dir, not those of
// its subdirectories. Watch a file's parent directory to follow the file: a
// program that replaces it by rename leaves a watch on the file itself pointing
// at the old one. The wake function is called from a background goroutine when
// news is pending, once for a burst until Changes is next read; it must not block
// or call Close. It returns ErrUnsupported on a platform without directory notification, and
// the system's error when dir cannot be watched.
func WatchDirectory(dir string, wake func()) (*DirectoryWatch, error) {
	if wake == nil {
		return nil, errors.New("app: WatchDirectory needs a wake function")
	}
	watch, err := filewatch.Open(dir, wake)
	if errors.Is(err, filewatch.ErrUnsupported) {
		return nil, ErrUnsupported
	}
	if err != nil {
		return nil, err
	}
	return &DirectoryWatch{watch: watch}, nil
}

// WatchTree is WatchDirectory for the whole tree below dir. Names are
// slash-separated paths relative to dir, and a name may be a directory that
// appeared or moved in with contents of its own: list it rather than assume it is
// empty. Any path with a component for which skip returns true (nil skips
// nothing) is never reported, and where the system needs a registration for each
// directory (Linux), a skipped directory gets none. A tree with more directories
// than the system can follow is refused with ErrTooLarge when opened and asks for
// a resync if it grows past that later.
func WatchTree(dir string, skip func(name string) bool, wake func()) (*DirectoryWatch, error) {
	if wake == nil {
		return nil, errors.New("app: WatchTree needs a wake function")
	}
	watch, err := filewatch.OpenTree(dir, skip, wake)
	if errors.Is(err, filewatch.ErrUnsupported) {
		return nil, ErrUnsupported
	}
	if errors.Is(err, filewatch.ErrTooLarge) {
		return nil, ErrWatchTooLarge
	}
	if err != nil {
		return nil, err
	}
	return &DirectoryWatch{watch: watch}, nil
}

// ErrWatchTooLarge reports a directory tree with more directories than a watch
// can follow.
var ErrWatchTooLarge = errors.New("app: the directory tree is too large to watch")

// Changes returns what changed since the previous call and forgets it. It never
// blocks on the file system and is meant for OnWake. The returned names reuse
// buf when it has room, so a caller holding one slice allocates nothing in the
// steady state. A burst of changes to one entry is reported once.
func (watch *DirectoryWatch) Changes(buf []string) DirectoryChanges {
	pending := watch.watch.Drain(buf)
	return DirectoryChanges{Names: pending.Names, Resync: pending.Resync}
}

// Close releases the registration. Unread news is discarded and the wake function
// is not called afterwards. Close may be called more than once.
func (watch *DirectoryWatch) Close() {
	watch.watch.Close()
}
