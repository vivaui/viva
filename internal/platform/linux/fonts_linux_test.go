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

func TestFamilyMeasuresMonoRejectsProportionalAndMissingFamilies(t *testing.T) {
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
	for _, proportional := range []string{"DejaVu Sans", "Ubuntu", "Sans"} {
		if proportional == "Sans" || slices.Contains(all, proportional) {
			if familyMeasuresMono(services, proportional) {
				t.Errorf("familyMeasuresMono(%q) = true, want false", proportional)
			}
		}
	}
	if familyMeasuresMono(services, "VivaNonExistentFontFamily12345") {
		t.Error("familyMeasuresMono(nonexistent) = true, want false")
	}
}
