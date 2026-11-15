package ui

import "github.com/vivaui/viva/gfx"

// RowID is the identity of source row i as declared by the last paint, and false
// for a row the table did not declare (offscreen, out of range, or not yet
// painted). Scrolling does not renumber source rows: an identity keeps its source
// index when the row moves on screen. When the Source is sorted or reordered, the
// identities follow those indices, not the records the application moved.
func (s *TableState) RowID(i int) (gfx.ID, bool) {
	if i < int(s.rowFirst) || i >= int(s.rowEnd) || s.scope.IsZero() {
		return gfx.ID{}, false
	}
	return s.scope.At(i), true
}

// Rows is the scope the table named its rows in on the last paint, for a handler
// that must recover a source row from a target:
//
//	row, ok := event.Target.In(state.Rows())
//
// It is zero before the first paint, and after a paint that gave the table no row
// scope. Identities from RowID belong to this scope. Declaring the same state in
// more than one table leaves the calls describing only the last table painted.
func (s *TableState) Rows() gfx.Scope { return s.scope }
