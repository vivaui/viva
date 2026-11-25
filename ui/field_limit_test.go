package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// MaxLength cuts what is typed beyond the cap, and ShowCount shows how full the
// field is at its end.
func TestMaxLengthCapsTheFieldAndShowCountReportsIt(t *testing.T) {
	var name string
	d := openPage(t, func(c ui.C) {
		ui.TextField(c, &name, "Name", "", ui.MaxLength(5), ui.ShowCount())
	})
	d.click("Name")
	d.typeText("abcdefgh")
	if name != "abcde" {
		t.Fatalf("name = %q after typing eight runes into a field capped at five, want abcde", name)
	}
	d.repaint()
	found := false
	for _, text := range d.driver.Snapshot().Texts {
		if text.Value == "5/5" {
			found = true
		}
	}
	if !found {
		t.Fatal("the field does not show its count 5/5")
	}
}

// A field with no cap counts without a total, and one with no ShowCount shows no count.
func TestCountWithoutACapAndCapWithoutACount(t *testing.T) {
	var a, b string
	d := openPage(t, func(c ui.C) {
		ui.TextField(c, &a, "Title", "", ui.ShowCount())
		ui.TextField(c, &b, "Body", "", ui.MaxLength(3))
	})
	d.click("Title")
	d.typeText("hey")
	d.click("Body")
	d.typeText("abcdef")
	d.repaint()
	var counts []string
	for _, text := range d.driver.Snapshot().Texts {
		if text.Value == "3" || text.Value == "3/3" {
			counts = append(counts, text.Value)
		}
	}
	if len(counts) != 1 || counts[0] != "3" {
		t.Fatalf("counts shown = %v, want only the uncapped field's 3", counts)
	}
	if b != "abc" {
		t.Fatalf("body = %q, want abc", b)
	}
}

// A capped, counting field repaints without allocating: the count's words are made
// when the numbers change.
func TestCountedFieldRepaintsWithoutAllocating(t *testing.T) {
	name := "abc"
	d := openPage(t, func(c ui.C) {
		ui.TextField(c, &name, "Name", "", ui.MaxLength(10), ui.ShowCount())
	})
	d.repaint()
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
