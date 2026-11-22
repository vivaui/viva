package windows

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestFittedWindowsClustersPreserveGraphemeEdges(t *testing.T) {
	// DirectWrite split the decomposed letter, used two UTF-16 units for the
	// supplementary letter, and joined two Arabic letters in one ligature.
	const value = "a\u0301\U00010437\u0644\u0627z"
	keep := gfx.FittedMonoCaret{Offset: -1, Leading: -2, Trailing: -3}
	got, ok := appendFittedClusterCarets([]gfx.FittedMonoCaret{keep}, value, []fittedCluster{
		{end: 1, leading: 4, trailing: 12},
		{end: 2, leading: 12, trailing: 12},
		{end: 4, leading: 12, trailing: 20},
		{end: 6, leading: 38, trailing: 20},
		{end: 7, leading: 38, trailing: 46},
	}, 4, 2)
	want := []gfx.FittedMonoCaret{
		keep,
		{Offset: 0, Leading: 0, Trailing: 0},
		{Offset: 3, Leading: 16, Trailing: 16},
		{Offset: 7, Leading: 68, Trailing: 32},
		{Offset: 9, Leading: 50, Trailing: 50},
		{Offset: 11, Leading: 68, Trailing: 32},
		{Offset: len(value), Leading: 84, Trailing: 84},
	}
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("fitted carets = %+v, %t; want %+v", got, ok, want)
	}
}

func TestFittedWindowsClustersRejectIncompleteLayout(t *testing.T) {
	before := []gfx.FittedMonoCaret{{Offset: -1}}
	got, ok := appendFittedClusterCarets(before, "a\U00010437", []fittedCluster{
		{end: 1, leading: 0, trailing: 8},
		{end: 2, leading: 8, trailing: 16},
	}, 0, 1)
	if ok || !slices.Equal(got, before) {
		t.Fatalf("truncated UTF-16 layout changed destination: %+v, %t", got, ok)
	}
}
