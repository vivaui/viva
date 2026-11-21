package ui_test

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A named Progress is a progress bar to assistive technology: a read-only range
// from 0 to 100 at the fraction, never a keyboard stop.
func TestNamedProgressIsAProgressBar(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(10).Children(func() {
			ui.Progress(c, 0.64).Label("Uploading")
		})
	})
	snapshot := d.driver.Snapshot()
	var hit gfx.Hit
	for _, h := range snapshot.Hits {
		if h.Role == gfx.RoleProgressBar {
			hit = h
		}
	}
	if hit.Role != gfx.RoleProgressBar {
		t.Fatalf("no progressbar hit in %+v", snapshot.Hits)
	}
	if !hit.Range.Set || math.Abs(hit.Range.Now-64) > 0.01 || hit.Range.Min != 0 || hit.Range.Max != 100 {
		t.Errorf("range = %+v, want 64 of 0..100", hit.Range)
	}
	if hit.Focusable {
		t.Errorf("progress bar is a keyboard stop: %+v", hit)
	}
	if hit.Label != "Uploading" {
		t.Errorf("label = %q, want Uploading", hit.Label)
	}

	nodes := d.driver.Accessibility()
	found := false
	for _, node := range nodes {
		if node.ID != hit.ID {
			continue
		}
		found = true
		if node.Role != gfx.RoleProgressBar {
			t.Errorf("accessible role = %v, want progressbar", node.Role)
		}
		if !node.Range.Set || math.Abs(node.Range.Now-64) > 0.01 {
			t.Errorf("accessible range = %+v, want 64 of 0..100", node.Range)
		}
		if node.Label != "Uploading" {
			t.Errorf("accessible label = %q", node.Label)
		}
	}
	if !found {
		t.Error("the progress bar is missing from the accessibility tree")
	}
}

// An indeterminate bar reports no number, and an unnamed bar declares no node:
// a bare progress bar would otherwise take the pointer from the row holding it.
func TestUnnamedAndIndeterminateProgress(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(10).Children(func() {
			ui.Progress(c, 0.3)
			ui.Progress(c, 0).Label("Working").Indeterminate()
		})
	})
	snapshot := d.driver.Snapshot()
	bars := 0
	for _, hit := range snapshot.Hits {
		if hit.Role != gfx.RoleProgressBar {
			continue
		}
		bars++
		if hit.Label != "Working" {
			t.Errorf("unnamed progress declared a bar: %+v", hit)
		}
		if hit.Range.Set {
			t.Errorf("indeterminate bar reported a number: %+v", hit.Range)
		}
	}
	if bars != 1 {
		t.Errorf("%d progress bars declared, want only the named one", bars)
	}
}
