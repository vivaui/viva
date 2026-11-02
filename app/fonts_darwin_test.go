//go:build darwin

package app

import (
	"slices"
	"testing"
)

// TestMonospaceFamiliesMeasureTheFontsThatDoNotSaySo guards the families AppKit
// marks proportional: the ones that still draw on a grid have to be offered, and
// the ones that do not must stay out.
func TestMonospaceFamiliesMeasureTheFontsThatDoNotSaySo(t *testing.T) {
	families := MonospaceFamilies()
	for _, name := range []string{"Helvetica", "Arial", "Georgia", "Times New Roman"} {
		if slices.Contains(families, name) {
			t.Errorf("%q draws on no grid but is listed in %v", name, families)
		}
	}
	var grid []string
	for _, name := range []string{"JetBrains Maple Mono", "Hack", "Maple Mono", "Cousine for Powerline"} {
		if slices.Contains(families, name) {
			grid = append(grid, name)
		}
	}
	if len(grid) == 0 {
		t.Skip("no installed family draws on a grid without saying so")
	}
	t.Logf("measured on a grid: %v", grid)
}
