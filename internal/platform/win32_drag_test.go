package platform

import "testing"

func TestDragEffectOfReportsOnlyTheEffectsTheSourceOffered(t *testing.T) {
	for _, test := range []struct {
		allowed  DragEffect
		reported DragEffect
		want     DragEffect
	}{
		{DragEffectCopy | DragEffectMove, DragEffectCopy, DragEffectCopy},
		{DragEffectCopy | DragEffectMove, DragEffectMove, DragEffectMove},
		{DragEffectCopy, DragEffectMove, DragEffectNone},                  // a target cannot take what was not offered
		{DragEffectCopy | DragEffectMove, DragEffectNone, DragEffectNone}, // the drag ended nowhere
		{DragEffectNone, DragEffectCopy, DragEffectNone},
		{DragEffectCopy | DragEffectLink, DragEffectCopy | DragEffectLink, DragEffectCopy},
		{DragEffectMove | DragEffectLink, DragEffectCopy | DragEffectMove | DragEffectLink, DragEffectLink},
		{DragEffectCopy | DragEffectMove, DragEffectCopy | DragEffectMove, DragEffectCopy},
	} {
		if got := DragEffectOf(test.allowed, test.reported); got != test.want {
			t.Errorf("DragEffectOf(%d, %d) = %d, want %d", test.allowed, test.reported, got, test.want)
		}
	}
}

func TestADragOffersAtLeastOneKnownEffect(t *testing.T) {
	for _, test := range []struct {
		mask  DragEffect
		known bool
	}{
		{DragEffectCopy, true},
		{DragEffectCopy | DragEffectMove | DragEffectLink, true},
		{DragEffectNone, true},
		{8, false}, // a bit no platform understands
		{1 | 16, false},
	} {
		if got := DragEffectKnown(test.mask); got != test.known {
			t.Errorf("DragEffectKnown(%d) = %v, want %v", test.mask, got, test.known)
		}
	}
}
