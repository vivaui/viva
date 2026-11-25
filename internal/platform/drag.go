package platform

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrDragUnsupported reports that this target has no way to take files out of a
// window by dragging them: the application can still offer the files, but only
// by copying them (see SetClipboardFiles), which tells the user something else.
// The Web target has no drag of its own to start, and a target may lack the
// promise drag described by FilePromise.
var ErrDragUnsupported = errors.New("dragging files out of a window is unsupported")

// FilePromise is one item of a drag whose content the application produces when
// the target asks for it, instead of a path that already exists on this
// computer. Dragging a file off a remote server is this: the file is downloaded
// where the user lets go, while the target copies, so nothing is fetched when
// the drag starts.
//
// Name is one path component, unique among its siblings. A file's Open returns
// its bytes when the target reads them; a directory's List returns its children
// on demand, each named relative to it. Both callbacks may run on a goroutine
// other than the one that started the drag, and may run more than once.
type FilePromise struct {
	Name      string
	Size      int64 // -1 when unknown; a directory is 0
	Directory bool
	Open      func() (io.ReadCloser, error) // nil for a directory
	List      func() ([]FilePromise, error) // nil for a file
}

// maxFilePromiseDepth bounds how deep a promised directory tree may nest, so a
// tree that keeps producing children cannot walk the stack away.
const maxFilePromiseDepth = 64

// ValidateFilePromises checks one level of promises: names, sibling uniqueness
// and the callbacks each kind needs. It does not call List, so validating a tree
// costs nothing; WalkFilePromises validates each level as it reaches it.
func ValidateFilePromises(promises []FilePromise) error { return validateFilePromises(promises, "") }

func validateFilePromises(promises []FilePromise, at string) error {
	seen := make(map[string]struct{}, len(promises))
	for _, promise := range promises {
		if err := validateFilePromise(promise); err != nil {
			return fmt.Errorf("app: file promise %q: %w", promise.Name, err)
		}
		if _, duplicate := seen[promise.Name]; duplicate {
			return fmt.Errorf("app: two file promises in %q are named %q", at, promise.Name)
		}
		seen[promise.Name] = struct{}{}
	}
	return nil
}

func validateFilePromise(promise FilePromise) error {
	name := promise.Name
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("a name is one path component")
	}
	if promise.Directory {
		if promise.Open != nil {
			return fmt.Errorf("a directory cannot open file content")
		}
		if promise.List == nil {
			return fmt.Errorf("a directory needs the list of its children")
		}
		return nil
	}
	if promise.Open == nil {
		return fmt.Errorf("a file needs its content")
	}
	if promise.List != nil {
		return fmt.Errorf("a file cannot list children")
	}
	if promise.Size < -1 {
		return fmt.Errorf("a size below -1 is not a size")
	}
	return nil
}

// WalkFilePromises visits every promise under roots depth-first, the directory
// before its children, with path its slash-separated path from the drag root. It
// validates each level and refuses a tree with more than MaxClipboardFiles
// items, the bound one drag may carry.
func WalkFilePromises(roots []FilePromise, visit func(path string, promise FilePromise) error) error {
	count := 0
	var walk func(prefix string, promises []FilePromise, depth int) error
	walk = func(prefix string, promises []FilePromise, depth int) error {
		if depth > maxFilePromiseDepth {
			return fmt.Errorf("app: file promise %q nests deeper than %d levels", prefix, maxFilePromiseDepth)
		}
		if err := validateFilePromises(promises, prefix); err != nil {
			return err
		}
		for _, promise := range promises {
			count++
			if count > MaxClipboardFiles {
				return fmt.Errorf("app: a drag takes at most %d files", MaxClipboardFiles)
			}
			path := promise.Name
			if prefix != "" {
				path = prefix + "/" + promise.Name
			}
			if err := visit(path, promise); err != nil {
				return err
			}
			if !promise.Directory {
				continue
			}
			children, err := promise.List()
			if err != nil {
				return fmt.Errorf("app: list the children of %q: %w", path, err)
			}
			if err := walk(path, children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk("", roots, 0)
}

// DragEffect is what letting go of dragged files does to them where they land: a
// copy leaves them where they were, a move hands them to the target, and a link
// leaves a reference behind. The values are the OLE DROPEFFECT flags Windows
// reports and asks for, so one value is also the mask of the effects an
// operation allows.
type DragEffect uint32

const (
	// DragEffectNone is what a drag that ended nowhere, or that the user
	// cancelled with Escape, reports.
	DragEffectNone DragEffect = 0
	// DragEffectCopy copies the files; the ones dragged out stay where they are.
	DragEffectCopy DragEffect = 1
	// DragEffectMove moves the files to where they land.
	DragEffectMove DragEffect = 2
	// DragEffectLink puts a link to the files where they land.
	DragEffectLink DragEffect = 4

	// dragEffects is every effect named here.
	dragEffects = DragEffectCopy | DragEffectMove | DragEffectLink
)

// DragEffectKnown reports whether every effect in a mask is one this package
// names, so that an operation asking for an effect no platform understands is
// refused instead of having the bits silently dropped.
func DragEffectKnown(mask DragEffect) bool { return mask&^dragEffects == 0 }
