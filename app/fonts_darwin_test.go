//go:build darwin

package app

import (
	"slices"
	"testing"
)

// TestMonospaceFamiliesKeepTheSystemsOwnFacesApart guards the two ends of the
// macOS measurement with faces every install ships: the terminal faces stay in
// the list and the proportional ones stay out of it.
func TestMonospaceFamiliesKeepTheSystemsOwnFacesApart(t *testing.T) {
	families := MonospaceFamilies()
	for _, name := range []string{"Menlo", "Monaco", "Courier New"} {
		if !slices.Contains(families, name) {
			t.Errorf("%q draws on a grid but is not listed in %v", name, families)
		}
	}
	for _, name := range []string{"Helvetica", "Times New Roman", "Georgia"} {
		if slices.Contains(families, name) {
			t.Errorf("%q draws on no grid but is listed in %v", name, families)
		}
	}
}
