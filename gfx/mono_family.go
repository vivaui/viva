package gfx

import (
	"slices"
	"strings"
	"sync/atomic"
)

// MaxMonoFaces is how many faces one process can register with MonoFamily and
// MonoStack, besides the two built-in ones. A registered face is never released.
const MaxMonoFaces = 4096

// MaxMonoFallbacks is how many fallback families one face holds.
const MaxMonoFallbacks = 8

// maxMonoFamilyName bounds a family name; real ones are a few words long.
const maxMonoFamilyName = 96

// monoStack is what a registered face stands for: a primary font and the families
// that draw the characters it lacks. It is immutable once registered.
type monoStack struct {
	// family is the installed primary family, "" when the primary is built in.
	family string
	// classic is set when the built-in primary is MonoClassic rather than MonoSystem.
	classic   bool
	fallbacks []string
}

// monoStacks is the registry: entry i is what MonoFace(2+i) stands for. It is
// replaced as a whole, never edited, so a reader on the render thread sees either
// the old table or the new one.
var monoStacks atomic.Pointer[[]monoStack]

// validMonoName reports whether name can be a font family: not empty, not too
// long, without control characters, and without a comma, which font lists use as
// their separator.
func validMonoName(name string) bool {
	if name == "" || len(name) > maxMonoFamilyName {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == ',' {
			return false
		}
	}
	return true
}

// MonoFamily returns the face that draws fixed-pitch text in the installed font
// family called name ("JetBrains Mono", "Menlo"). The same name always gives the
// same face. A name that is empty, too long, holds a control character or
// would be one past the limit gives MonoSystem. Whether the family is installed is
// the platform's to answer: a name it does not know draws in its fallback
// fixed-pitch font, so an application should offer only names it has listed.
func MonoFamily(name string) MonoFace {
	name = strings.TrimSpace(name)
	if !validMonoName(name) {
		return MonoSystem
	}
	return internMonoStack(name, false, nil)
}

// MonoStack returns the face that draws fixed-pitch text in base and, for the
// characters base has no glyph for, in the installed families fallbacks, tried in
// order, before the platform's own choice. base is MonoSystem, MonoClassic or a
// face from MonoFamily; fallbacks it already has are replaced. The cell of the
// face is base's: a glyph from a fallback family is placed on that grid, and the
// fallbacks never change its size. A name that is empty, too long, holds a
// control character or a comma, or repeats an earlier one (or base's own family)
// is left out, and so is every name after the first MaxMonoFallbacks. With none
// left the result is base itself. The same arguments always give the same face.
// Whether a family is installed is the platform's to answer: one it does not
// know is skipped.
func MonoStack(base MonoFace, fallbacks ...string) MonoFace {
	var family string
	var classic bool
	if base > MonoClassic {
		if current := monoStackOf(base); current != nil {
			family, classic = current.family, current.classic
		}
	} else {
		classic = base == MonoClassic
	}
	var buffer [MaxMonoFallbacks]string
	kept := buffer[:0]
	for _, name := range fallbacks {
		name = strings.TrimSpace(name)
		if !validMonoName(name) || name == family || slices.Contains(kept, name) {
			continue
		}
		if len(kept) == MaxMonoFallbacks {
			break
		}
		kept = append(kept, name)
	}
	if len(kept) == 0 && family == "" {
		if classic {
			return MonoClassic
		}
		return MonoSystem
	}
	return internMonoStack(family, classic, kept)
}

// internMonoStack returns the face of the stack, registering it when it is new. It
// keeps a copy of fallbacks, so the caller's list may live on its stack.
func internMonoStack(family string, classic bool, fallbacks []string) MonoFace {
	if current := monoStacks.Load(); current != nil {
		if face, ok := findMonoStack(*current, family, classic, fallbacks); ok {
			return face
		}
	}
	return registerMonoStack(monoStack{family: family, classic: classic, fallbacks: slices.Clone(fallbacks)})
}

func findMonoStack(stacks []monoStack, family string, classic bool, fallbacks []string) (MonoFace, bool) {
	for index := range stacks {
		if stack := &stacks[index]; stack.family == family && stack.classic == classic && slices.Equal(stack.fallbacks, fallbacks) {
			return MonoFace(2 + index), true
		}
	}
	return 0, false
}

// registerMonoStack adds a stack the registry may keep, unless another caller did
// first.
func registerMonoStack(stack monoStack) MonoFace {
	for {
		current := monoStacks.Load()
		var stacks []monoStack
		if current != nil {
			stacks = *current
		}
		if face, ok := findMonoStack(stacks, stack.family, stack.classic, stack.fallbacks); ok {
			return face
		}
		if len(stacks) >= MaxMonoFaces {
			return MonoSystem
		}
		next := append(append(make([]monoStack, 0, len(stacks)+1), stacks...), stack)
		if monoStacks.CompareAndSwap(current, &next) {
			return MonoFace(2 + len(stacks))
		}
	}
}

// monoStackOf is what a registered face stands for, or nil for a built-in face and
// for a number nothing registered.
func monoStackOf(face MonoFace) *monoStack {
	if face < 2 {
		return nil
	}
	stacks := monoStacks.Load()
	if stacks == nil || int(face-2) >= len(*stacks) {
		return nil
	}
	return &(*stacks)[face-2]
}

// MonoFamilyName is the installed family a face from MonoFamily or MonoStack
// draws in first, or "" for the built-in faces, for a stack over one of them and
// for a number nothing registered.
func MonoFamilyName(face MonoFace) string {
	if stack := monoStackOf(face); stack != nil {
		return stack.family
	}
	return ""
}

// MonoFallbacks is the families that draw what face's own font cannot, in order,
// or nil when it has none. The slice is shared: do not change it.
func MonoFallbacks(face MonoFace) []string {
	if stack := monoStackOf(face); stack != nil {
		return stack.fallbacks
	}
	return nil
}

// MonoIsClassic reports whether the font face draws in first is the built-in
// classic one: MonoClassic itself or a stack over it.
func MonoIsClassic(face MonoFace) bool {
	if face == MonoClassic {
		return true
	}
	stack := monoStackOf(face)
	return stack != nil && stack.classic
}

// MonoFaceCount is how many faces have been registered, so a platform can tell
// cheaply that it has new ones to learn.
func MonoFaceCount() int {
	if stacks := monoStacks.Load(); stacks != nil {
		return len(*stacks)
	}
	return 0
}

// validMonoFace reports whether face is one the canvas can draw: a built-in
// one or a registered face.
func validMonoFace(face MonoFace) bool {
	return face <= MonoClassic || int(face-2) < MonoFaceCount()
}
