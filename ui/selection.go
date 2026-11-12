package ui

import "iter"

// Selection is the rows chosen in a List that lets the user choose several, held by
// their keys (ListState.Key), or their indices when the list has none. Its zero
// value is empty. The keys of items gone from the list stay until the application
// takes them out.
//
//	type model struct {
//		files  []file
//		chosen ui.Selection // their keys
//		list   ui.ListState
//	}
//
//	m.list.Key = func(i int) int { return m.files[i].id }
//	m.list.Selection = &m.chosen
type Selection struct {
	keys map[int]struct{}
}

// Has reports whether the row of key k is chosen.
func (s *Selection) Has(k int) bool {
	_, ok := s.keys[k]
	return ok
}

// Len returns how many rows are chosen.
func (s *Selection) Len() int { return len(s.keys) }

// Add chooses the row of key k.
func (s *Selection) Add(k int) {
	if s.keys == nil {
		s.keys = map[int]struct{}{}
	}
	s.keys[k] = struct{}{}
}

// Remove takes the row of key k out of the choice.
func (s *Selection) Remove(k int) { delete(s.keys, k) }

// Clear chooses no row.
func (s *Selection) Clear() { clear(s.keys) }

// All returns the keys of the rows chosen, in no order.
func (s *Selection) All() iter.Seq[int] {
	return func(yield func(int) bool) {
		for k := range s.keys {
			if !yield(k) {
				return
			}
		}
	}
}

// set chooses the row of key k, or takes it out, and reports whether that changed
// the choice.
func (s *Selection) set(k int, on bool) bool {
	if s.Has(k) == on {
		return false
	}
	if on {
		s.Add(k)
	} else {
		s.Remove(k)
	}
	return true
}
