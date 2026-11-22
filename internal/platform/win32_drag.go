package platform

// DragEffectOf is what a drag of files reports once it has ended: the effect the
// target took, when the source allowed that one, and none otherwise. Files that
// went nowhere are none, as are files a target claimed to take by an effect the
// source never offered.
//
// Windows documents one effect for a drop, but a target that answers with more
// is taken as taking the first of them the source allows, in the order
// DropEffect prefers: a copy, then a link, then a move.
func DragEffectOf(allowed, reported DragEffect) DragEffect {
	taken := reported & allowed
	switch {
	case taken&DragEffectCopy != 0:
		return DragEffectCopy
	case taken&DragEffectLink != 0:
		return DragEffectLink
	case taken&DragEffectMove != 0:
		return DragEffectMove
	}
	return DragEffectNone
}
