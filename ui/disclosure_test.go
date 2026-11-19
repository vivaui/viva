package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestPageAccordionOpensOneRowAtATime(t *testing.T) {
	state := ui.NewAccordionState(2)
	d := openPage(t, func(c ui.C) {
		ui.Accordion(c, state, []ui.AccordionItem{
			{Title: "Shipping", Content: func() { ui.Button(c, "Shipping details") }},
			{Title: "Returns", Content: func() { ui.Button(c, "Return details") }},
		})
	})
	if hit := d.hit("Shipping"); hit.Expanded || !hit.Expandable {
		t.Fatalf("closed header expanded=%v expandable=%v", hit.Expanded, hit.Expandable)
	}
	d.click("Shipping")
	if !state.Open[0] {
		t.Fatal("clicking Shipping did not open it")
	}
	if hit := d.hit("Shipping"); !hit.Expanded {
		t.Fatal("the open header does not report itself expanded")
	}
	if !d.has("Shipping details") {
		t.Fatal("open content was not declared")
	}
	d.click("Returns")
	if state.Open[0] || !state.Open[1] {
		t.Fatalf("open = %v, want only Returns", state.Open)
	}
	if d.has("Shipping details") || !d.has("Return details") {
		t.Fatal("switching rows did not switch the content")
	}
}

func TestPageAccordionMultipleKeepsBothOpen(t *testing.T) {
	state := ui.NewAccordionState(2)
	state.Multiple = true
	d := openPage(t, func(c ui.C) {
		ui.Accordion(c, state, []ui.AccordionItem{
			{Title: "Shipping", Content: func() { ui.Button(c, "Shipping details") }},
			{Title: "Returns", Content: func() { ui.Button(c, "Return details") }},
		})
	})
	d.click("Shipping")
	d.click("Returns")
	if !state.Open[0] || !state.Open[1] {
		t.Fatalf("open = %v, want both rows", state.Open)
	}
	d.click("Shipping")
	if state.Open[0] || !state.Open[1] {
		t.Fatalf("open = %v, want only Returns after closing Shipping", state.Open)
	}
}

func TestPageCollapsibleFlipsItsValue(t *testing.T) {
	state := ui.CollapsibleState{}
	d := openPage(t, func(c ui.C) {
		ui.Collapsible(c, &state, "Advanced", func() {
			ui.Button(c, "Advanced settings")
		})
	})
	hit := d.hit("Advanced")
	if hit.Role != gfx.RoleButton || hit.Expanded || !hit.Expandable {
		t.Fatalf("header = %+v, want a collapsed disclosure button", hit)
	}
	d.click("Advanced")
	if !state.Open || !d.has("Advanced settings") {
		t.Fatal("clicking the header did not open the content")
	}
	d.click("Advanced")
	if state.Open || d.has("Advanced settings") {
		t.Fatal("the second click did not close the content")
	}
}

func TestPageAccordionDisabledStaysInert(t *testing.T) {
	state := ui.NewAccordionState(1)
	state.Disabled = true
	d := openPage(t, func(c ui.C) {
		ui.Accordion(c, state, []ui.AccordionItem{
			{Title: "Shipping", Content: func() { ui.Button(c, "Shipping details") }},
		})
	})
	if hit := d.hit("Shipping"); !hit.Disabled {
		t.Fatal("the disabled accordion header is not disabled")
	}
	d.click("Shipping")
	if state.Open[0] || d.has("Shipping details") {
		t.Fatal("a disabled accordion opened")
	}
}

func BenchmarkPageAccordion(b *testing.B) {
	state := ui.NewAccordionState(3)
	state.Open[0] = true
	items := []ui.AccordionItem{
		{Title: "Shipping", Content: func() {}},
		{Title: "Returns", Content: func() {}},
		{Title: "Warranty", Content: func() {}},
	}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, func(c ui.C) {
			ui.Accordion(c, state, items)
		})
	}
}
