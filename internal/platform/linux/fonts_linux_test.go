//go:build !android

package linux

import (
	"slices"
	"testing"
)

func TestMonospaceFamiliesIncludeFixedPitchAndExcludeProportional(t *testing.T) {
	mono := MonospaceFamilies()
	if len(mono) == 0 {
		t.Skip("no fixed-pitch family installed")
	}
	all := FontFamilies()
	for _, name := range mono {
		if !slices.Contains(all, name) {
			t.Errorf("MonospaceFamilies() listed %q which is missing from FontFamilies()", name)
		}
	}
	for _, proportional := range []string{"DejaVu Sans", "DejaVu Serif", "Ubuntu", "Ubuntu Sans"} {
		if slices.Contains(all, proportional) && slices.Contains(mono, proportional) {
			t.Errorf("proportional family %q was listed by MonospaceFamilies()", proportional)
		}
	}
}

func TestFamilyMeasuresMonoRejectsProportionalFamilies(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Skipf("no Pango on this machine: %v", err)
	}
	defer services.close()

	mono := MonospaceFamilies()
	if len(mono) == 0 {
		t.Skip("no fixed-pitch family installed")
	}
	if !familyMeasuresMono(services, mono[0]) {
		t.Fatalf("familyMeasuresMono(%q) = false, want true", mono[0])
	}
	all := FontFamilies()
	for _, proportional := range []string{"DejaVu Sans", "Ubuntu"} {
		// An unknown name is not tested: fontconfig answers it with the
		// default font, which is monospace on some systems.
		if slices.Contains(all, proportional) && familyMeasuresMono(services, proportional) {
			t.Errorf("familyMeasuresMono(%q) = true, want false", proportional)
		}
	}
}
